package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/igo/igo/ai/schema"
	"github.com/igo/igo/core"
	errorspkg "github.com/igo/igo/core/errors"
	routepkg "github.com/igo/igo/core/route"
	"github.com/igo/igo/types"
)

type aiProjectContext struct {
	Framework string                  `json:"framework"`
	Commands  []string                `json:"commands"`
	Rules     []string                `json:"rules"`
	Routes    []*routepkg.RouteConfig `json:"routes"`
	Schemas   []*types.TypeSchema     `json:"schemas,omitempty"`
}

type staticProject struct {
	root         string
	fset         *token.FileSet
	files        []*ast.File
	handlers     map[string]*handlerFacts
	routes       []*routepkg.RouteConfig
	schemas      []*types.TypeSchema
	schemaByName map[string]*types.TypeSchema
}

type handlerFacts struct {
	Name        string
	FilePath    string
	LineNumber  int
	Params      []routepkg.ParamDefinition
	RequestBody *routepkg.RequestBodyDefinition
	Responses   []routepkg.ResponseDefinition
	AIHints     []string
}

type routeEnv struct {
	varName     string
	prefix      string
	middlewares []string
}

func runAI(args []string) int {
	if len(args) == 0 {
		printAIUsage(os.Stderr)
		return 1
	}
	cmd := args[0]
	switch cmd {
	case "errors":
		return writeJSON(os.Stdout, errorspkg.ListErrorCodes())
	case "workflow":
		return writeJSON(os.Stdout, core.AIWorkflow())
	case "examples":
		return writeAIExamples(os.Stdout)
	case "help", "-h", "--help":
		printAIUsage(os.Stdout)
		return 0
	}

	root := "."
	if len(args) > 1 && !strings.HasPrefix(args[1], "--") {
		root = args[1]
		args = append(args[:1], args[2:]...)
	}

	project, err := loadStaticProject(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ai: %v\n", err)
		return 1
	}

	switch cmd {
	case "context":
		format := flagValue(args[1:], "--format", "md")
		ctx := project.context()
		if format == "json" {
			return writeJSON(os.Stdout, ctx)
		}
		writeContextMarkdown(os.Stdout, ctx)
		return 0
	case "routes":
		return writeJSON(os.Stdout, project.routes)
	case "schemas":
		return writeJSON(os.Stdout, project.schemas)
	case "openapi":
		gen := schema.NewRouteGenerator(project.routes, project.schemas...)
		return writeJSON(os.Stdout, gen.Generate())
	case "prompt":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: igo ai prompt [path] METHOD PATH")
			return 1
		}
		method := strings.ToUpper(args[1])
		routePath := args[2]
		for _, r := range project.routes {
			if r.Method == method && r.Path == routePath {
				writeHandlerPrompt(os.Stdout, r)
				return 0
			}
		}
		fmt.Fprintf(os.Stderr, "ai: route not found: %s %s\n", method, routePath)
		return 1
	default:
		fmt.Fprintf(os.Stderr, "unknown ai command: %s\n\n", cmd)
		printAIUsage(os.Stderr)
		return 1
	}
}

func printAIUsage(w io.Writer) {
	fmt.Fprint(w, `igo ai - compact context for AI coding tools

Usage:
  igo ai context [path] [--format md|json]
  igo ai routes [path]
  igo ai schemas [path]
  igo ai errors
  igo ai openapi [path]
  igo ai prompt [path] METHOD PATH
  igo ai workflow
  igo ai examples

`)
}

func loadStaticProject(root string) (*staticProject, error) {
	files, err := collectGoFiles(root)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	p := &staticProject{
		root:         root,
		fset:         fset,
		handlers:     make(map[string]*handlerFacts),
		schemaByName: make(map[string]*types.TypeSchema),
	}
	for _, filePath := range files {
		f, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		p.files = append(p.files, f)
	}
	for _, f := range p.files {
		p.collectTypes(f)
		p.collectHandlers(f)
	}
	for _, f := range p.files {
		p.collectRoutes(f)
	}
	sortRoutes(p.routes)
	sort.Slice(p.schemas, func(i, j int) bool {
		if p.schemas[i].FilePath != p.schemas[j].FilePath {
			return p.schemas[i].FilePath < p.schemas[j].FilePath
		}
		return p.schemas[i].Name < p.schemas[j].Name
	})
	return p, nil
}

func (p *staticProject) collectTypes(f *ast.File) {
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			pos := p.fset.Position(ts.Pos())
			fields := p.collectStructFields(st)
			if len(fields) == 0 {
				continue
			}
			schema := &types.TypeSchema{
				Name:     ts.Name.Name,
				Package:  f.Name.Name,
				FilePath: pos.Filename,
				Fields:   fields,
			}
			p.schemas = append(p.schemas, schema)
			p.schemaByName[schema.Name] = schema
		}
	}
}

func (p *staticProject) collectStructFields(st *ast.StructType) []types.FieldSchema {
	fields := make([]types.FieldSchema, 0)
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		goType := exprString(field.Type)
		jsonTag, validateTag, description, defaultValue, example := fieldTags(field)
		for _, name := range field.Names {
			if !name.IsExported() {
				continue
			}
			jsonName, skip := types.JSONName(name.Name, jsonTag)
			if skip {
				continue
			}
			desc := description
			if desc == "" && field.Doc != nil {
				desc = strings.TrimSpace(field.Doc.Text())
			}
			if desc == "" && field.Comment != nil {
				desc = strings.TrimSpace(field.Comment.Text())
			}
			fields = append(fields, types.BuildFieldSchema(
				name.Name,
				jsonName,
				goType,
				validateTag,
				strings.TrimSpace(desc),
				defaultValue,
				example,
			))
		}
	}
	return fields
}

func (p *staticProject) collectHandlers(f *ast.File) {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !looksLikeContextHandler(fn.Type) {
			continue
		}
		pos := p.fset.Position(fn.Pos())
		facts := &handlerFacts{
			Name:       fn.Name.Name,
			FilePath:   pos.Filename,
			LineNumber: pos.Line,
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			p.collectHandlerCallFacts(facts, call)
			return true
		})
		p.handlers[fn.Name.Name] = facts
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			p.handlers[receiverKey(fn.Recv.List[0].Type, fn.Name.Name)] = facts
		}
	}
}

func (p *staticProject) collectHandlerCallFacts(facts *handlerFacts, call *ast.CallExpr) {
	if typ, kind, ok := bindAndValidateType(call); ok {
		switch kind {
		case "body":
			facts.RequestBody = &routepkg.RequestBodyDefinition{
				ContentType: "application/json",
				TypeName:    typ,
				Required:    true,
			}
			facts.AIHints = appendUnique(facts.AIHints, "uses BindAndValidate; return immediately when ok is false")
		case "query":
			facts.Params = mergeParams(facts.Params, p.paramsFromSchema(typ, "query"))
			facts.AIHints = appendUnique(facts.AIHints, "uses BindQueryAndValidate; return immediately when ok is false")
		case "path":
			facts.Params = mergeParams(facts.Params, p.paramsFromSchema(typ, "path"))
			facts.AIHints = appendUnique(facts.AIHints, "uses BindPathAndValidate; return immediately when ok is false")
		}
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	name := sel.Sel.Name
	switch {
	case strings.HasPrefix(name, "Query"):
		if len(call.Args) > 0 {
			if key, ok := stringLiteral(call.Args[0]); ok {
				facts.Params = mergeParam(facts.Params, routepkg.ParamDefinition{
					Name:     key,
					In:       "query",
					Type:     paramTypeFromMethod(name),
					Required: strings.Contains(name, "OrFail"),
					Default:  defaultArg(call),
				})
			}
		}
	case strings.HasPrefix(name, "Param"):
		if len(call.Args) > 0 {
			if key, ok := stringLiteral(call.Args[0]); ok {
				facts.Params = mergeParam(facts.Params, routepkg.ParamDefinition{
					Name:     key,
					In:       "path",
					Type:     paramTypeFromMethod(name),
					Required: strings.Contains(name, "OrFail"),
				})
			}
		}
	case name == "Success":
		facts.Responses = mergeResponse(facts.Responses, 200, "success", responseType(call))
	case name == "Created":
		facts.Responses = mergeResponse(facts.Responses, 201, "created", responseType(call))
	case name == "NoContent":
		facts.Responses = mergeResponse(facts.Responses, 204, "no content", "")
	case name == "JSON":
		if len(call.Args) > 0 {
			facts.Responses = mergeResponse(facts.Responses, statusCode(call.Args[0]), "response", responseType(call))
		}
	case name == "Redirect":
		if len(call.Args) > 0 {
			facts.Responses = mergeResponse(facts.Responses, statusCode(call.Args[0]), "redirect", "")
		}
	case name == "BadRequest", name == "BadRequestWrap":
		facts.Responses = mergeResponse(facts.Responses, 400, "bad request", "error")
	case name == "ValidationError", name == "ValidationErrorWrap":
		facts.Responses = mergeResponse(facts.Responses, 422, "validation failed", "error")
	case name == "Unauthorized":
		facts.Responses = mergeResponse(facts.Responses, 401, "unauthorized", "error")
	case name == "Forbidden":
		facts.Responses = mergeResponse(facts.Responses, 403, "forbidden", "error")
	case name == "NotFound", name == "NotFoundWrap":
		facts.Responses = mergeResponse(facts.Responses, 404, "not found", "error")
	case name == "InternalError", name == "InternalErrorWrap":
		facts.Responses = mergeResponse(facts.Responses, 500, "internal error", "error")
	}
}

func (p *staticProject) collectRoutes(f *ast.File) {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		envs := routeEnvsForFunc(fn)
		for _, stmt := range fn.Body.List {
			p.collectRoutesFromStmt(stmt, envs)
		}
	}
}

func routeEnvsForFunc(fn *ast.FuncDecl) []routeEnv {
	var envs []routeEnv
	if fn.Type.Params == nil {
		return envs
	}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			envs = append(envs, routeEnv{varName: name.Name})
		}
	}
	return envs
}

func (p *staticProject) collectRoutesFromStmt(stmt ast.Stmt, envs []routeEnv) {
	ast.Inspect(stmt, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if p.collectRouteCall(call, envs) {
			return false
		}
		if p.collectGroupCall(call, envs) {
			return false
		}
		return true
	})
}

func (p *staticProject) collectGroupCall(call *ast.CallExpr, envs []routeEnv) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Group" || len(call.Args) < 2 {
		return false
	}
	base, ok := stringLiteral(call.Args[0])
	if !ok {
		return false
	}
	fn, ok := call.Args[1].(*ast.FuncLit)
	if !ok || fn.Body == nil || fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		return false
	}
	if len(fn.Type.Params.List[0].Names) == 0 {
		return false
	}
	parent := findRouteEnv(envs, receiverName(sel.X))
	groupEnv := routeEnv{
		varName:     fn.Type.Params.List[0].Names[0].Name,
		prefix:      joinPaths(parent.prefix, base),
		middlewares: append([]string{}, parent.middlewares...),
	}
	for _, arg := range call.Args[2:] {
		groupEnv.middlewares = append(groupEnv.middlewares, exprString(arg))
	}
	childEnvs := append(append([]routeEnv{}, envs...), groupEnv)
	for _, stmt := range fn.Body.List {
		p.collectRoutesFromStmt(stmt, childEnvs)
	}
	return true
}

func (p *staticProject) collectRouteCall(call *ast.CallExpr, envs []routeEnv) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) < 2 {
		return false
	}
	method, ok := routeMethod(sel.Sel.Name)
	if !ok {
		return false
	}
	routePath, ok := stringLiteral(call.Args[0])
	if !ok {
		return false
	}
	env := findRouteEnv(envs, receiverName(sel.X))
	fullPath := joinPaths(env.prefix, routePath)
	handlerName, inlineFile, inlineLine := p.handlerKey(call.Args[1])
	cfg := routepkg.InferFromFunction(handlerName, method, fullPath)
	cfg.HandlerName = handlerName
	cfg.FilePath = inlineFile
	cfg.LineNumber = inlineLine
	cfg.Middlewares = append([]string{}, env.middlewares...)
	for _, arg := range call.Args[2:] {
		cfg.Middlewares = append(cfg.Middlewares, exprString(arg))
	}
	if facts := p.handlerFactsFor(call.Args[1], handlerName); facts != nil {
		cfg.FilePath = facts.FilePath
		cfg.LineNumber = facts.LineNumber
		cfg.Params = mergeParams(cfg.Params, facts.Params)
		cfg.RequestBody = facts.RequestBody
		cfg.Responses = mergeResponses(cfg.Responses, facts.Responses)
		cfg.AIHints = append(cfg.AIHints, facts.AIHints...)
	}
	p.routes = append(p.routes, cfg)
	return true
}

func (p *staticProject) handlerFactsFor(expr ast.Expr, fallback string) *handlerFacts {
	switch h := expr.(type) {
	case *ast.Ident:
		return p.handlers[h.Name]
	case *ast.SelectorExpr:
		if facts := p.handlers[selectorKey(h)]; facts != nil {
			return facts
		}
		return p.handlers[h.Sel.Name]
	case *ast.FuncLit:
		return p.handlerFactsFromFuncLit(h)
	}
	return p.handlers[fallback]
}

func (p *staticProject) handlerKey(expr ast.Expr) (name, file string, line int) {
	if fn, ok := expr.(*ast.FuncLit); ok {
		pos := p.fset.Position(fn.Pos())
		return "inline", pos.Filename, pos.Line
	}
	return handlerKey(expr), "", 0
}

func (p *staticProject) handlerFactsFromFuncLit(fn *ast.FuncLit) *handlerFacts {
	pos := p.fset.Position(fn.Pos())
	facts := &handlerFacts{
		Name:       "inline",
		FilePath:   pos.Filename,
		LineNumber: pos.Line,
	}
	if fn.Body == nil {
		return facts
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		p.collectHandlerCallFacts(facts, call)
		return true
	})
	return facts
}

func (p *staticProject) paramsFromSchema(typeName, in string) []routepkg.ParamDefinition {
	schema := p.schemaByName[strings.TrimPrefix(typeName, "*")]
	if schema == nil {
		return nil
	}
	params := make([]routepkg.ParamDefinition, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		params = append(params, routepkg.ParamDefinition{
			Name:        field.JSONName,
			In:          in,
			Type:        paramTypeFromField(field),
			Required:    field.Required || in == "path",
			Description: field.Description,
			Validation:  validationStrings(field.Rules),
			Enum:        append([]string(nil), field.Enum...),
			Min:         field.Min,
			Max:         field.Max,
			GTE:         field.GTE,
			LTE:         field.LTE,
			Len:         field.Len,
			Default:     field.Default,
		})
	}
	return params
}

func (p *staticProject) context() aiProjectContext {
	return aiProjectContext{
		Framework: "igo",
		Commands: []string{
			"go test ./...",
			"go run ./cmd/igo doctor .",
			"go run ./cmd/igo ai routes .",
			"go run ./cmd/igo ai schemas .",
			"go run ./cmd/igo ai errors",
			"go run ./cmd/igo ai openapi .",
		},
		Rules: []string{
			"Handlers parse params, call services, and send one response.",
			"Use core.BindAndValidate[T](c) for JSON body validation and return when ok is false.",
			"Use core.BindQueryAndValidate[T](c) for structured query parameters.",
			"Use core.BindPathAndValidate[T](c) for structured path parameters.",
			"Use c.ParamInt64OrFail / c.QueryInt / c.QueryDefault instead of manual parsing.",
			"Inside if err != nil, prefer *Wrap response helpers to preserve the error chain.",
			"Register pure response DTOs with app.RegisterSchema(ResponseDTO{}).",
			"Pass group middleware as Group's third argument; do not call group.Use inside the closure.",
		},
		Routes:  p.routes,
		Schemas: p.schemas,
	}
}

func writeContextMarkdown(w io.Writer, ctx aiProjectContext) {
	fmt.Fprintf(w, "# igo AI context\n\n")
	fmt.Fprintf(w, "## Rules\n")
	for _, rule := range ctx.Rules {
		fmt.Fprintf(w, "- %s\n", rule)
	}
	fmt.Fprintf(w, "\n## Commands\n")
	for _, cmd := range ctx.Commands {
		fmt.Fprintf(w, "- `%s`\n", cmd)
	}
	fmt.Fprintf(w, "\n## Routes\n")
	for _, r := range ctx.Routes {
		body := ""
		if r.RequestBody != nil {
			body = " body=" + r.RequestBody.TypeName
		}
		fmt.Fprintf(w, "- `%s %s` -> `%s`%s\n", r.Method, r.Path, r.HandlerName, body)
		if len(r.Params) > 0 {
			fmt.Fprintf(w, "  params: %s\n", compactParams(r.Params))
		}
		if len(r.Responses) > 0 {
			fmt.Fprintf(w, "  responses: %s\n", compactResponses(r.Responses))
		}
	}
	if len(ctx.Schemas) > 0 {
		fmt.Fprintf(w, "\n## Schemas\n")
		for _, schema := range ctx.Schemas {
			fmt.Fprintf(w, "- `%s` fields: %s\n", schema.Name, compactFields(schema.Fields))
		}
	}
}

func writeHandlerPrompt(w io.Writer, r *routepkg.RouteConfig) {
	fmt.Fprintf(w, "Implement or modify handler for `%s %s`.\n\n", r.Method, r.Path)
	fmt.Fprintf(w, "Handler: `%s`", r.HandlerName)
	if r.FilePath != "" {
		fmt.Fprintf(w, " in `%s:%d`", r.FilePath, r.LineNumber)
	}
	fmt.Fprintln(w)
	if len(r.Middlewares) > 0 {
		fmt.Fprintf(w, "Middlewares: %s\n", strings.Join(r.Middlewares, ", "))
	}
	if r.RequestBody != nil {
		fmt.Fprintf(w, "Request body: `%s` (%s)\n", r.RequestBody.TypeName, r.RequestBody.ContentType)
	}
	if len(r.Params) > 0 {
		fmt.Fprintf(w, "Params: %s\n", compactParams(r.Params))
	}
	if len(r.Responses) > 0 {
		fmt.Fprintf(w, "Responses: %s\n", compactResponses(r.Responses))
	}
	fmt.Fprintln(w, "\nRequired igo conventions: use BindAndValidate for JSON bodies, Param*OrFail for path IDs, *Wrap helpers inside err branches, and return after sending errors.")
}

func writeJSON(w io.Writer, v any) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "ai: encode: %v\n", err)
		return 1
	}
	return 0
}

func flagValue(args []string, name, fallback string) string {
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, name+"=") {
			return strings.TrimPrefix(arg, name+"=")
		}
	}
	return fallback
}

func routeMethod(name string) (string, bool) {
	methods := map[string]string{
		"GET": "GET", "Get": "GET",
		"POST": "POST", "Post": "POST",
		"PUT": "PUT", "Put": "PUT",
		"DELETE": "DELETE", "Delete": "DELETE",
		"PATCH": "PATCH", "Patch": "PATCH",
		"OPTIONS": "OPTIONS", "Options": "OPTIONS",
		"HEAD": "HEAD", "Head": "HEAD",
	}
	method, ok := methods[name]
	return method, ok
}

func findRouteEnv(envs []routeEnv, name string) routeEnv {
	for i := len(envs) - 1; i >= 0; i-- {
		if envs[i].varName == name {
			return envs[i]
		}
	}
	return routeEnv{varName: name}
}

func receiverName(expr ast.Expr) string {
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return exprString(expr)
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

func fieldTags(field *ast.Field) (jsonTag, validateTag, description, defaultValue, example string) {
	if field.Tag == nil {
		return "", "", "", "", ""
	}
	raw, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return "", "", "", "", ""
	}
	tag := reflect.StructTag(raw)
	description = tag.Get("description")
	if description == "" {
		description = tag.Get("doc")
	}
	return tag.Get("json"), tag.Get("validate"), description, tag.Get("default"), tag.Get("example")
}

func joinPaths(prefix, p string) string {
	if prefix == "" {
		if strings.HasPrefix(p, "/") {
			return path.Clean(p)
		}
		return "/" + path.Clean(p)
	}
	return path.Clean(strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(p, "/"))
}

func handlerKey(expr ast.Expr) string {
	switch h := expr.(type) {
	case *ast.Ident:
		return h.Name
	case *ast.SelectorExpr:
		return selectorKey(h)
	default:
		return exprString(expr)
	}
}

func selectorKey(sel *ast.SelectorExpr) string {
	if id, ok := sel.X.(*ast.Ident); ok {
		return id.Name + "." + sel.Sel.Name
	}
	return sel.Sel.Name
}

func receiverKey(expr ast.Expr, method string) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverKey(t.X, method)
	case *ast.Ident:
		return t.Name + "." + method
	case *ast.SelectorExpr:
		return t.Sel.Name + "." + method
	default:
		return method
	}
}

func bindAndValidateType(call *ast.CallExpr) (string, string, bool) {
	switch fun := call.Fun.(type) {
	case *ast.IndexExpr:
		if kind, ok := bindAndValidateKind(fun.X); ok {
			return exprString(fun.Index), kind, true
		}
	case *ast.IndexListExpr:
		if kind, ok := bindAndValidateKind(fun.X); ok && len(fun.Indices) > 0 {
			return exprString(fun.Indices[0]), kind, true
		}
	}
	return "", "", false
}

func bindAndValidateKind(expr ast.Expr) (string, bool) {
	name := ""
	switch f := expr.(type) {
	case *ast.Ident:
		name = f.Name
	case *ast.SelectorExpr:
		name = f.Sel.Name
	}
	switch name {
	case "BindAndValidate":
		return "body", true
	case "BindQueryAndValidate":
		return "query", true
	case "BindPathAndValidate":
		return "path", true
	default:
		return "", false
	}
}

func paramTypeFromMethod(name string) string {
	switch {
	case strings.Contains(name, "Int"):
		return "int"
	case strings.Contains(name, "Bool"):
		return "bool"
	default:
		return "string"
	}
}

func paramTypeFromField(field types.FieldSchema) string {
	switch field.Type {
	case "integer":
		return "int"
	case "boolean":
		return "bool"
	case "number":
		return "float"
	case "array", "object":
		return field.Type
	default:
		return "string"
	}
}

func validationStrings(rules []types.RuleSchema) []string {
	if len(rules) == 0 {
		return nil
	}
	out := make([]string, 0, len(rules))
	for _, rule := range rules {
		if len(rule.Params) == 0 {
			out = append(out, rule.Name)
			continue
		}
		out = append(out, rule.Name+":"+strings.Join(rule.Params, ","))
	}
	return out
}

func defaultArg(call *ast.CallExpr) string {
	if len(call.Args) < 2 {
		return ""
	}
	if lit, ok := call.Args[1].(*ast.BasicLit); ok {
		return lit.Value
	}
	if s, ok := stringLiteral(call.Args[1]); ok {
		return s
	}
	return ""
}

func responseType(call *ast.CallExpr) string {
	if len(call.Args) == 0 {
		return ""
	}
	idx := 0
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "JSON" && len(call.Args) > 1 {
		idx = 1
	}
	return exprTypeName(call.Args[idx])
}

func exprTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.UnaryExpr:
		return exprTypeName(e.X)
	case *ast.CompositeLit:
		return exprString(e.Type)
	case *ast.CallExpr:
		return exprString(e.Fun)
	case *ast.SelectorExpr:
		return selectorKey(e)
	default:
		return exprString(expr)
	}
}

func statusCode(expr ast.Expr) int {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.INT {
		if n, err := strconv.Atoi(lit.Value); err == nil {
			return n
		}
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		switch sel.Sel.Name {
		case "StatusOK":
			return 200
		case "StatusCreated":
			return 201
		case "StatusNoContent":
			return 204
		case "StatusMovedPermanently":
			return 301
		case "StatusFound":
			return 302
		case "StatusSeeOther":
			return 303
		case "StatusTemporaryRedirect":
			return 307
		case "StatusPermanentRedirect":
			return 308
		case "StatusBadRequest":
			return 400
		case "StatusUnauthorized":
			return 401
		case "StatusForbidden":
			return 403
		case "StatusNotFound":
			return 404
		case "StatusUnprocessableEntity":
			return 422
		case "StatusInternalServerError":
			return 500
		}
	}
	return 200
}

func exprString(expr ast.Expr) string {
	var b strings.Builder
	if err := format.Node(&b, token.NewFileSet(), expr); err != nil {
		return ""
	}
	return b.String()
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func mergeParams(base, extra []routepkg.ParamDefinition) []routepkg.ParamDefinition {
	out := append([]routepkg.ParamDefinition{}, base...)
	for _, p := range extra {
		out = mergeParam(out, p)
	}
	return out
}

func mergeParam(params []routepkg.ParamDefinition, param routepkg.ParamDefinition) []routepkg.ParamDefinition {
	for i := range params {
		if params[i].Name == param.Name && params[i].In == param.In {
			if params[i].Type == "" {
				params[i].Type = param.Type
			}
			params[i].Required = params[i].Required || param.Required
			if params[i].Default == "" {
				params[i].Default = param.Default
			}
			if params[i].Description == "" {
				params[i].Description = param.Description
			}
			if len(params[i].Validation) == 0 {
				params[i].Validation = param.Validation
			}
			if len(params[i].Enum) == 0 {
				params[i].Enum = param.Enum
			}
			if params[i].Min == "" {
				params[i].Min = param.Min
			}
			if params[i].Max == "" {
				params[i].Max = param.Max
			}
			if params[i].GTE == "" {
				params[i].GTE = param.GTE
			}
			if params[i].LTE == "" {
				params[i].LTE = param.LTE
			}
			if params[i].Len == "" {
				params[i].Len = param.Len
			}
			return params
		}
	}
	return append(params, param)
}

func mergeResponses(base, extra []routepkg.ResponseDefinition) []routepkg.ResponseDefinition {
	out := append([]routepkg.ResponseDefinition{}, base...)
	for _, r := range extra {
		out = mergeResponse(out, r.StatusCode, r.Description, r.TypeName)
	}
	return out
}

func mergeResponse(responses []routepkg.ResponseDefinition, status int, desc, typ string) []routepkg.ResponseDefinition {
	for i := range responses {
		if responses[i].StatusCode == status {
			if responses[i].Description == "" {
				responses[i].Description = desc
			}
			if responses[i].TypeName == "" {
				responses[i].TypeName = typ
			}
			return responses
		}
	}
	return append(responses, routepkg.ResponseDefinition{StatusCode: status, Description: desc, TypeName: typ})
}

func sortRoutes(routes []*routepkg.RouteConfig) {
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Method < routes[j].Method
	})
}

func compactParams(params []routepkg.ParamDefinition) string {
	parts := make([]string, 0, len(params))
	for _, p := range params {
		required := "optional"
		if p.Required {
			required = "required"
		}
		parts = append(parts, fmt.Sprintf("%s:%s:%s:%s", p.Name, p.In, p.Type, required))
	}
	return strings.Join(parts, ", ")
}

func compactResponses(responses []routepkg.ResponseDefinition) string {
	parts := make([]string, 0, len(responses))
	for _, r := range responses {
		typ := r.TypeName
		if typ == "" {
			typ = "empty"
		}
		parts = append(parts, fmt.Sprintf("%d:%s", r.StatusCode, typ))
	}
	return strings.Join(parts, ", ")
}

func compactFields(fields []types.FieldSchema) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		required := "optional"
		if field.Required {
			required = "required"
		}
		parts = append(parts, fmt.Sprintf("%s:%s:%s", field.JSONName, field.GoType, required))
	}
	return strings.Join(parts, ", ")
}

func writeAIExamples(w io.Writer) int {
	examples := []map[string]string{
		{
			"name": "query validation",
			"code": "type ListQuery struct { Page int `json:\"page\" validate:\"gte:1\"` }\nq, ok := igo.BindQueryAndValidate[ListQuery](c)\nif !ok { return }",
		},
		{
			"name": "path validation",
			"code": "type UserParams struct { ID int64 `json:\"id\" validate:\"required|gte:1\"` }\np, ok := igo.BindPathAndValidate[UserParams](c)\nif !ok { return }",
		},
		{
			"name": "response schema",
			"code": "app.RegisterSchema(UserResponse{})",
		},
		{
			"name": "runtime AI endpoints",
			"code": "app.RegisterAIRoutes()",
		},
	}
	return writeJSON(w, examples)
}
