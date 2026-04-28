package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Diagnostic 一条检查结果
type Diagnostic struct {
	Severity string // "error" | "warning"
	File     string
	Line     int
	Rule     string
	Message  string
}

// runDoctor 扫描指定目录下的 .go 文件，输出检查结果
// 返回值用作进程退出码（有 error 时返回 1）
func runDoctor(root string) int {
	files, err := collectGoFiles(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor: %v\n", err)
		return 1
	}

	if len(files) == 0 {
		fmt.Println("doctor: no .go files found")
		return 0
	}

	fset := token.NewFileSet()
	var diags []Diagnostic

	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			diags = append(diags, Diagnostic{
				Severity: "error",
				File:     path,
				Line:     0,
				Rule:     "parse",
				Message:  err.Error(),
			})
			continue
		}
		diags = append(diags, checkErrShouldWrap(fset, f)...)
		diags = append(diags, checkGroupInternalUse(fset, f)...)
		diags = append(diags, checkAppUseMissingNext(fset, f)...)
		diags = append(diags, checkMissingReturnAfterErrorResponse(fset, f)...)
		diags = append(diags, checkDoubleSuccessResponse(fset, f)...)
		diags = append(diags, checkJSONErrorResponse(fset, f)...)
	}

	report(diags, len(files))

	for _, d := range diags {
		if d.Severity == "error" {
			return 1
		}
	}
	return 0
}

func checkMissingReturnAfterErrorResponse(fset *token.FileSet, f *ast.File) []Diagnostic {
	var diags []Diagnostic
	errorResponses := map[string]bool{
		"BadRequest": true, "NotFound": true, "Unauthorized": true, "Forbidden": true,
		"InternalError": true, "ValidationError": true,
		"BadRequestWrap": true, "NotFoundWrap": true, "InternalErrorWrap": true,
		"ValidationErrorWrap": true,
	}

	ast.Inspect(f, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for i, stmt := range block.List {
			if !stmtCallsSelector(stmt, errorResponses) {
				continue
			}
			if i+1 < len(block.List) {
				if _, ok := block.List[i+1].(*ast.ReturnStmt); !ok {
					pos := fset.Position(stmt.Pos())
					diags = append(diags, Diagnostic{
						Severity: "warning",
						File:     pos.Filename,
						Line:     pos.Line,
						Rule:     "missing-return-after-error",
						Message:  "error response should usually be followed by return to avoid continuing the handler",
					})
				}
			}
		}
		return true
	})
	return diags
}

func checkDoubleSuccessResponse(fset *token.FileSet, f *ast.File) []Diagnostic {
	var diags []Diagnostic
	successResponses := map[string]bool{
		"JSON": true, "Success": true, "Created": true, "NoContent": true,
	}

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !looksLikeContextHandler(fn.Type) {
			continue
		}
		var count int
		var second token.Pos
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if selectorCallName(call, successResponses) != "" {
				count++
				if count == 2 {
					second = call.Pos()
				}
			}
			return true
		})
		if count > 1 {
			pos := fset.Position(second)
			diags = append(diags, Diagnostic{
				Severity: "warning",
				File:     pos.Filename,
				Line:     pos.Line,
				Rule:     "multiple-success-responses",
				Message:  "handler appears to send more than one success response; ensure control flow returns after the first response",
			})
		}
	}
	return diags
}

func checkJSONErrorResponse(fset *token.FileSet, f *ast.File) []Diagnostic {
	var diags []Diagnostic
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "JSON" || len(call.Args) < 2 {
			return true
		}
		if id, ok := call.Args[1].(*ast.Ident); ok && id.Name == "err" {
			pos := fset.Position(call.Pos())
			diags = append(diags, Diagnostic{
				Severity: "warning",
				File:     pos.Filename,
				Line:     pos.Line,
				Rule:     "json-error",
				Message:  "do not return raw err with c.JSON; use structured error helpers or *Wrap methods",
			})
		}
		return true
	})
	return diags
}

func stmtCallsSelector(stmt ast.Stmt, names map[string]bool) bool {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := expr.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	return selectorCallName(call, names) != ""
}

func selectorCallName(call *ast.CallExpr, names map[string]bool) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !names[sel.Sel.Name] {
		return ""
	}
	if _, ok := sel.X.(*ast.Ident); !ok {
		return ""
	}
	return sel.Sel.Name
}

// collectGoFiles 递归收集 .go 文件，跳过 vendor / .git / node_modules / _test.go
func collectGoFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

// checkErrShouldWrap 检测：在 if err != nil { ... } 块体内调用 c.InternalError/NotFound/BadRequest
//
// 这些调用会丢弃 err 上下文，建议改用 *Wrap 系列保留调用链。
// 只在严格的 `if err != nil { ... }` 模式下警告，避免误报。
func checkErrShouldWrap(fset *token.FileSet, f *ast.File) []Diagnostic {
	var diags []Diagnostic
	plain := map[string]string{
		"InternalError": "InternalErrorWrap",
		"NotFound":      "NotFoundWrap",
		"BadRequest":    "BadRequestWrap",
	}

	ast.Inspect(f, func(n ast.Node) bool {
		ifs, ok := n.(*ast.IfStmt)
		if !ok || !isErrNotNilCond(ifs.Cond) {
			return true
		}
		// 跳过 `if err := recover(); err != nil` 模式（panic 恢复，err 是 interface{}）
		if isRecoverInit(ifs.Init) {
			return true
		}
		ast.Inspect(ifs.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			replacement, isPlain := plain[sel.Sel.Name]
			if !isPlain {
				return true
			}
			// 排除 receiver 不像 Context 的情况：receiver 必须是简单 Ident（如 c）
			recv, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			pos := fset.Position(call.Pos())
			diags = append(diags, Diagnostic{
				Severity: "warning",
				File:     pos.Filename,
				Line:     pos.Line,
				Rule:     "should-wrap",
				Message: fmt.Sprintf(
					"%s.%s(...) inside `if err != nil` loses err context; use %s.%s(err, ...) to preserve call chain",
					recv.Name, sel.Sel.Name, recv.Name, replacement),
			})
			return true
		})
		return true
	})
	return diags
}

// isRecoverInit 判断 IfStmt 的 Init 是否是 `err := recover()` 模式
func isRecoverInit(init ast.Stmt) bool {
	as, ok := init.(*ast.AssignStmt)
	if !ok || len(as.Rhs) == 0 {
		return false
	}
	call, ok := as.Rhs[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == "recover"
}

// isErrNotNilCond 判断条件是否为 `err != nil` 形式
func isErrNotNilCond(expr ast.Expr) bool {
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return false
	}
	lhs, ok := bin.X.(*ast.Ident)
	if !ok || lhs.Name != "err" {
		return false
	}
	rhs, ok := bin.Y.(*ast.Ident)
	if !ok || rhs.Name != "nil" {
		return false
	}
	return true
}

// checkGroupInternalUse 检测：app.Group(...) 内部对 group 参数调用 .Use(...)
// 这会把中间件挂到共享的 Router 全局链而非组内，是常见 footgun
func checkGroupInternalUse(fset *token.FileSet, f *ast.File) []Diagnostic {
	var diags []Diagnostic

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Group" || len(call.Args) < 2 {
			return true
		}
		fn, ok := call.Args[1].(*ast.FuncLit)
		if !ok || fn.Body == nil || len(fn.Type.Params.List) == 0 {
			return true
		}
		if len(fn.Type.Params.List[0].Names) == 0 {
			return true
		}
		paramName := fn.Type.Params.List[0].Names[0].Name

		ast.Inspect(fn.Body, func(node ast.Node) bool {
			inner, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			isel, ok := inner.Fun.(*ast.SelectorExpr)
			if !ok || isel.Sel.Name != "Use" {
				return true
			}
			if id, ok := isel.X.(*ast.Ident); ok && id.Name == paramName {
				pos := fset.Position(inner.Pos())
				diags = append(diags, Diagnostic{
					Severity: "warning",
					File:     pos.Filename,
					Line:     pos.Line,
					Rule:     "group-use-leak",
					Message: fmt.Sprintf(
						"%s.Use(...) inside Group leaks to the global router; pass middleware as Group's third argument instead",
						paramName),
				})
			}
			return true
		})
		return true
	})
	return diags
}

// checkAppUseMissingNext 检测：app.Use(func(c *...){...}) 内联中间件没调用 c.Next() 也没短路
//
// 仅检查 .Use(...) 调用的第一个参数是 FuncLit 的情形，避免对普通 handler 误报。
func checkAppUseMissingNext(fset *token.FileSet, f *ast.File) []Diagnostic {
	var diags []Diagnostic
	terminating := map[string]bool{
		"Next": true, "JSON": true, "Success": true, "Created": true, "NoContent": true,
		"BadRequest": true, "NotFound": true, "Unauthorized": true, "Forbidden": true,
		"InternalError": true, "ValidationError": true, "Error": true, "Status": true,
		"BadRequestWrap": true, "NotFoundWrap": true, "InternalErrorWrap": true,
		"ValidationErrorWrap": true, "Abort": true, "Redirect": true,
	}

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Use" || len(call.Args) == 0 {
			return true
		}
		lit, ok := call.Args[0].(*ast.FuncLit)
		if !ok || lit.Body == nil {
			return true
		}
		if !looksLikeContextHandler(lit.Type) {
			return true
		}

		var hasTerminating bool
		ast.Inspect(lit.Body, func(node ast.Node) bool {
			if c, ok := node.(*ast.CallExpr); ok {
				if s, ok := c.Fun.(*ast.SelectorExpr); ok && terminating[s.Sel.Name] {
					hasTerminating = true
				}
			}
			return true
		})

		if !hasTerminating {
			pos := fset.Position(lit.Pos())
			diags = append(diags, Diagnostic{
				Severity: "warning",
				File:     pos.Filename,
				Line:     pos.Line,
				Rule:     "middleware-missing-next",
				Message:  "inline middleware does not call c.Next() or send a response; request will hang",
			})
		}
		return true
	})
	return diags
}

// looksLikeContextHandler 判断函数签名是否形如 func(c *Context) 或 func(c *core.Context)
func looksLikeContextHandler(ft *ast.FuncType) bool {
	if ft.Params == nil || len(ft.Params.List) != 1 {
		return false
	}
	if ft.Results != nil && len(ft.Results.List) > 0 {
		return false
	}
	star, ok := ft.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	switch t := star.X.(type) {
	case *ast.Ident:
		return t.Name == "Context"
	case *ast.SelectorExpr:
		return t.Sel.Name == "Context"
	}
	return false
}

// report 按文件分组打印诊断结果
func report(diags []Diagnostic, fileCount int) {
	if len(diags) == 0 {
		fmt.Printf("✓ doctor: %d file(s) checked, no issues found\n", fileCount)
		return
	}

	sort.Slice(diags, func(i, j int) bool {
		if diags[i].File != diags[j].File {
			return diags[i].File < diags[j].File
		}
		return diags[i].Line < diags[j].Line
	})

	var errs, warns int
	for _, d := range diags {
		mark := "⚠"
		if d.Severity == "error" {
			mark = "✗"
			errs++
		} else {
			warns++
		}
		fmt.Printf("%s %s:%d  [%s]  %s\n", mark, d.File, d.Line, d.Rule, d.Message)
	}
	fmt.Printf("\ndoctor: %d file(s) checked, %d error(s), %d warning(s)\n",
		fileCount, errs, warns)
}
