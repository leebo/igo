// Package schema 从统一的 RouteConfig/TypeSchema 模型生成 OpenAPI 3.0 规范。
package schema

import (
	"strconv"
	"strings"

	routepkg "github.com/leebo/igo/core/route"
	"github.com/leebo/igo/types"
)

// Generator OpenAPI 规范生成器。
type Generator struct {
	routes  []*routepkg.RouteConfig
	schemas map[string]*types.TypeSchema
	spec    *OpenAPISpec
}

// NewRouteGenerator 从统一的 RouteConfig 列表创建生成器。
func NewRouteGenerator(routes []*routepkg.RouteConfig, schemas ...*types.TypeSchema) *Generator {
	g := &Generator{
		routes:  routes,
		schemas: make(map[string]*types.TypeSchema),
		spec:    newSpec(),
	}
	for _, schema := range schemas {
		if schema != nil && schema.Name != "" {
			g.schemas[schema.Name] = schema
		}
	}
	return g
}

func newSpec() *OpenAPISpec {
	return &OpenAPISpec{
		OpenAPI: "3.0.0",
		Info: &Info{
			Title:       "igo API",
			Version:     "1.0.0",
			Description: "AI-friendly API documentation",
		},
		Paths: make(map[string]*PathItem),
	}
}

// Generate 生成 OpenAPI 规范。
func (g *Generator) Generate() *OpenAPISpec {
	for _, schema := range g.schemas {
		g.addComponentSchema(schema)
	}
	for _, route := range g.routes {
		g.addRouteConfig(route)
	}
	g.addSecuritySchemesIfUsed()
	return g.spec
}

// addSecuritySchemesIfUsed 当任何路由的中间件名暗示需要鉴权时，自动注册一个
// "bearerAuth" 安全方案。AI 生成 OpenAPI 客户端会知道补 Authorization 头。
//
// 启发式：中间件名（不区分大小写）含有 "auth"、"jwt"、"bearer"、"token"、"requireauth"。
func (g *Generator) addSecuritySchemesIfUsed() {
	used := false
	for _, route := range g.routes {
		if routeNeedsAuth(route) {
			used = true
			break
		}
	}
	if !used {
		return
	}
	if g.spec.Components == nil {
		g.spec.Components = &Components{Schemas: make(map[string]*Schema)}
	}
	if g.spec.Components.SecuritySchemes == nil {
		g.spec.Components.SecuritySchemes = make(map[string]*SecurityScheme)
	}
	g.spec.Components.SecuritySchemes["bearerAuth"] = &SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
		Description:  "JWT bearer token in Authorization header",
	}
	// 给"看起来需要鉴权"的 Operation 挂上 security 引用
	for _, route := range g.routes {
		if !routeNeedsAuth(route) {
			continue
		}
		op := g.lookupOperation(route)
		if op == nil {
			continue
		}
		op.Security = []map[string][]string{{"bearerAuth": {}}}
	}
}

func routeNeedsAuth(route *routepkg.RouteConfig) bool {
	if route == nil {
		return false
	}
	for _, mw := range route.Middlewares {
		l := strings.ToLower(mw)
		if strings.Contains(l, "auth") || strings.Contains(l, "jwt") ||
			strings.Contains(l, "bearer") || strings.Contains(l, "token") ||
			strings.Contains(l, "requireauth") {
			return true
		}
	}
	return false
}

func (g *Generator) lookupOperation(route *routepkg.RouteConfig) *Operation {
	pathItem := g.spec.Paths[openAPIPath(route.Path)]
	if pathItem == nil {
		return nil
	}
	switch route.Method {
	case "GET":
		return pathItem.GET
	case "POST":
		return pathItem.POST
	case "PUT":
		return pathItem.PUT
	case "DELETE":
		return pathItem.DELETE
	case "PATCH":
		return pathItem.PATCH
	case "OPTIONS":
		return pathItem.OPTIONS
	case "HEAD":
		return pathItem.HEAD
	}
	return nil
}

func (g *Generator) addComponentSchema(typeSchema *types.TypeSchema) {
	if typeSchema == nil || typeSchema.Name == "" {
		return
	}
	if g.spec.Components == nil {
		g.spec.Components = &Components{Schemas: make(map[string]*Schema)}
	}
	required := make([]string, 0)
	properties := make(map[string]*Schema, len(typeSchema.Fields))
	for _, field := range typeSchema.Fields {
		if field.JSONName == "" {
			continue
		}
		properties[field.JSONName] = g.schemaForField(field)
		if field.Required {
			required = append(required, field.JSONName)
		}
	}
	g.spec.Components.Schemas[typeSchema.Name] = &Schema{
		Type:        "object",
		Description: typeSchema.Description,
		Properties:  properties,
		Required:    required,
	}
}

func (g *Generator) addRouteConfig(route *routepkg.RouteConfig) {
	if route == nil {
		return
	}
	pathKey := openAPIPath(route.Path)
	pathItem := g.spec.Paths[pathKey]
	if pathItem == nil {
		pathItem = &PathItem{}
		g.spec.Paths[pathKey] = pathItem
	}
	g.setOperation(pathItem, route.Method, g.buildOperationConfig(route))
}

func (g *Generator) buildOperationConfig(route *routepkg.RouteConfig) *Operation {
	op := &Operation{
		Tags:        route.Tags,
		Summary:     route.Summary,
		Description: route.Description,
		OperationID: operationID(route),
		Parameters:  make([]*Parameter, 0),
		Responses:   make(map[string]*Response),
		Deprecated:  route.Deprecated,
	}

	for _, param := range route.Params {
		op.Parameters = append(op.Parameters, &Parameter{
			Name:        param.Name,
			In:          param.In,
			Required:    param.Required || param.In == "path",
			Description: param.Description,
			Schema:      schemaForParam(param),
		})
	}

	if route.RequestBody != nil {
		contentType := route.RequestBody.ContentType
		if contentType == "" {
			contentType = "application/json"
		}
		op.RequestBody = &RequestBody{
			Description: route.RequestBody.Description,
			Required:    route.RequestBody.Required,
			Content: map[string]*MediaType{
				contentType: {
					Schema:  g.schemaForTypeName(route.RequestBody.TypeName),
					Example: route.RequestBody.Example,
				},
			},
		}
	}

	for _, resp := range route.Responses {
		response := &Response{Description: responseDescription(resp)}
		if resp.StatusCode != 204 && resp.TypeName != "" {
			response.Content = map[string]*MediaType{
				"application/json": {
					Schema:  g.schemaForTypeName(resp.TypeName),
					Example: resp.Example,
				},
			}
		}
		op.Responses[strconv.Itoa(resp.StatusCode)] = response
	}
	if len(op.Responses) == 0 {
		op.Responses["200"] = &Response{Description: "success"}
	}
	return op
}

func (g *Generator) setOperation(pathItem *PathItem, method string, operation *Operation) {
	switch method {
	case "GET":
		pathItem.GET = operation
	case "POST":
		pathItem.POST = operation
	case "PUT":
		pathItem.PUT = operation
	case "DELETE":
		pathItem.DELETE = operation
	case "PATCH":
		pathItem.PATCH = operation
	case "OPTIONS":
		pathItem.OPTIONS = operation
	case "HEAD":
		pathItem.HEAD = operation
	}
}

func (g *Generator) schemaForField(field types.FieldSchema) *Schema {
	s := &Schema{
		Type:        openAPIType(field.Type),
		Description: field.Description,
		Example:     field.Example,
		Enum:        field.Enum,
	}
	if field.GoType == "int64" {
		s.Format = "int64"
	}
	if strings.HasPrefix(field.GoType, "[]") {
		s.Type = "array"
		s.Items = g.schemaForTypeName(strings.TrimPrefix(field.GoType, "[]"))
	}
	applyBounds(s, field.Min, field.Max, field.GTE, field.LTE, field.Len)
	return s
}

func (g *Generator) schemaForTypeName(typeName string) *Schema {
	typeName = strings.TrimSpace(typeName)
	typeName = strings.TrimPrefix(typeName, "*")
	switch typeName {
	case "", "error", "any", "interface{}", "H", "core.H", "igo.H", "map[string]any", "map[string]interface{}":
		return &Schema{Type: "object"}
	case "string":
		return &Schema{Type: "string"}
	case "bool":
		return &Schema{Type: "boolean"}
	case "int", "int8", "int16", "int32", "uint", "uint8", "uint16", "uint32":
		return &Schema{Type: "integer"}
	case "int64", "uint64":
		return &Schema{Type: "integer", Format: "int64"}
	case "float32", "float64":
		return &Schema{Type: "number"}
	}
	if strings.HasPrefix(typeName, "[]") {
		return &Schema{Type: "array", Items: g.schemaForTypeName(strings.TrimPrefix(typeName, "[]"))}
	}
	if idx := strings.Index(typeName, "["); idx > 0 {
		typeName = typeName[:idx]
	}
	if strings.Contains(typeName, "{") || strings.Contains(typeName, "(") || strings.HasPrefix(typeName, "map[") {
		return &Schema{Type: "object"}
	}
	refName := typeName
	if idx := strings.LastIndex(refName, "."); idx >= 0 {
		refName = refName[idx+1:]
	}
	if _, ok := g.schemas[refName]; !ok {
		return &Schema{Type: "object"}
	}
	return &Schema{Ref: "#/components/schemas/" + refName}
}

// openAPIPath 把 igo 风格的 :param 转成 OpenAPI 风格的 {param}。
func openAPIPath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func openAPIType(t string) string {
	switch t {
	case "integer", "int", "int64":
		return "integer"
	case "number", "float", "float64":
		return "number"
	case "boolean", "bool":
		return "boolean"
	case "array":
		return "array"
	case "object":
		return "object"
	default:
		return "string"
	}
}

func schemaForParam(param routepkg.ParamDefinition) *Schema {
	s := &Schema{
		Type: openAPIType(param.Type),
		Enum: param.Enum,
	}
	applyBounds(s, param.Min, param.Max, param.GTE, param.LTE, param.Len)
	return s
}

func applyBounds(s *Schema, min, max, gte, lte, exactLen string) {
	if min != "" {
		if s.Type == "string" || s.Type == "array" {
			if n, err := strconv.Atoi(min); err == nil {
				s.MinLength = &n
			}
		} else {
			s.Minimum = numericValue(min)
		}
	}
	if max != "" {
		if s.Type == "string" || s.Type == "array" {
			if n, err := strconv.Atoi(max); err == nil {
				s.MaxLength = &n
			}
		} else {
			s.Maximum = numericValue(max)
		}
	}
	if gte != "" {
		s.Minimum = numericValue(gte)
	}
	if lte != "" {
		s.Maximum = numericValue(lte)
	}
	if exactLen != "" {
		if n, err := strconv.Atoi(exactLen); err == nil {
			s.MinLength = &n
			s.MaxLength = &n
		}
	}
}

func numericValue(s string) any {
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

func responseDescription(resp routepkg.ResponseDefinition) string {
	if resp.Description != "" {
		return resp.Description
	}
	return "response"
}

func operationID(route *routepkg.RouteConfig) string {
	if route.HandlerName != "" && route.HandlerName != "inline" {
		return route.HandlerName
	}
	return strings.ToLower(route.Method) + strings.ReplaceAll(openAPIPath(route.Path), "/", "_")
}
