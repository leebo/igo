package main

import (
	"fmt"
	"sort"
	"strings"
)

// 已知的内置 / 非结构体响应类型名 -- 不应被当作"未注册"。
// 这里放的是 *type 名*，不是 description。description 由 collectHandlerCallFacts
// 写入（"success" / "created" 等），不应混入这里。
var responseTypeAllowlist = map[string]bool{
	"":      true, // 未识别到具体类型，跳过
	"H":     true,
	"map":   true,
	"nil":   true,
	"error": true, // 错误响应占位，由 BadRequest/NotFound 等写入
}

// 内置 validate 规则。与 core/validator/rules.go 保持同步；新增规则时一并更新。
var knownValidateRules = map[string]bool{
	"required":   true,
	"email":      true,
	"min":        true,
	"max":        true,
	"gte":        true,
	"lte":        true,
	"gt":         true,
	"lt":         true,
	"len":        true,
	"regex":      true,
	"uuid":       true,
	"url":        true,
	"enum":       true,
	"eqfield":    true,
	"omitempty":  true,
	"alpha":      true,
	"alphanum":   true,
	"numeric":    true,
	"datetime":   true,
	"oneof":      true,
}

// checkResponseTypeNotRegistered: 路由声明的响应类型（来自 c.Success/Created/JSON 的实参）
// 在项目里没有对应的结构体定义。可能 AI 漏写 DTO 或拼错类型名。
func checkResponseTypeNotRegistered(p *staticProject) []Diagnostic {
	var diags []Diagnostic
	for _, route := range p.routes {
		for _, resp := range route.Responses {
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				continue
			}
			name := normalizeTypeName(resp.TypeName)
			if responseTypeAllowlist[name] {
				continue
			}
			if isPrimitiveTypeName(name) {
				continue
			}
			if _, ok := p.schemaByName[name]; ok {
				continue
			}
			// 静态分析无法解析局部变量类型（如 c.Success(user) 中的 user）。
			// Go 约定类型名首字母大写，所以 lowercase 名字基本是变量，跳过避免误报。
			if name == "" || !startsUpper(name) {
				continue
			}
			diags = append(diags, Diagnostic{
				Severity: "warning",
				File:     route.FilePath,
				Line:     route.LineNumber,
				Rule:     "response-type-not-registered",
				Message: fmt.Sprintf(
					"route %s %s returns %q (status %d) but no struct type %q is defined in this project; AI clients see an unresolved schema",
					route.Method, route.Path, resp.TypeName, resp.StatusCode, name),
			})
		}
	}
	return diags
}

// checkInvalidValidateTag: 项目内任何 schema 字段的 validate tag 中出现未知规则。
// 若 AI 写错（如 'required|min=2' 用了 = 而非 :），doctor 会提示。
func checkInvalidValidateTag(p *staticProject) []Diagnostic {
	var diags []Diagnostic
	for _, s := range p.schemas {
		for _, field := range s.Fields {
			for _, rule := range field.Rules {
				if rule.Name == "" {
					continue
				}
				if knownValidateRules[rule.Name] {
					continue
				}
				diags = append(diags, Diagnostic{
					Severity: "warning",
					File:     s.FilePath,
					Line:     0,
					Rule:     "unknown-validate-rule",
					Message: fmt.Sprintf(
						"struct %s field %s uses unknown validate rule %q; check spelling or register a custom rule",
						s.Name, field.GoName, rule.Name),
				})
			}
		}
	}
	// 排序保证输出稳定
	sort.SliceStable(diags, func(i, j int) bool {
		if diags[i].File != diags[j].File {
			return diags[i].File < diags[j].File
		}
		return diags[i].Message < diags[j].Message
	})
	return diags
}

// checkSchemaUnusedInRoutes: 项目里定义的 schema 完全没被任何路由引用为
// request/response/path/query。常见原因：拼错类型名 / 遗留 DTO / 应该删除的内部类型。
//
// 为降低误报：仅当 schema 名形如 *Request/*Response/*Body/*Params/*Query 时才告警。
// 引用证据来自 staticProject.markSchemaUsage 写入的 Usage 标签。
func checkSchemaUnusedInRoutes(p *staticProject) []Diagnostic {
	var diags []Diagnostic
	referenced := collectReferencedTypes(p)
	for _, s := range p.schemas {
		if !looksLikeDTOName(s.Name) {
			continue
		}
		if len(s.Usage) > 0 {
			continue
		}
		if referenced[s.Name] {
			continue
		}
		diags = append(diags, Diagnostic{
			Severity: "warning",
			File:     s.FilePath,
			Line:     0,
			Rule:     "schema-unused",
			Message: fmt.Sprintf(
				"schema %s appears to be a DTO but is not referenced by any route's BindAndValidate, c.Success, or c.Created; remove it or wire it into a handler",
				s.Name),
		})
	}
	return diags
}

func collectReferencedTypes(p *staticProject) map[string]bool {
	ref := make(map[string]bool)
	for _, route := range p.routes {
		if route.RequestBody != nil {
			ref[normalizeTypeName(route.RequestBody.TypeName)] = true
		}
		for _, resp := range route.Responses {
			ref[normalizeTypeName(resp.TypeName)] = true
		}
	}
	return ref
}

func looksLikeDTOName(name string) bool {
	for _, suffix := range []string{"Request", "Response", "Body", "Params", "Query", "Form", "DTO", "Payload"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func normalizeTypeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "*")
	name = strings.TrimPrefix(name, "&")
	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[idx+1:]
	}
	if idx := strings.IndexByte(name, '['); idx != -1 {
		name = name[:idx]
	}
	if idx := strings.IndexByte(name, '{'); idx != -1 {
		name = name[:idx]
	}
	return name
}

func startsUpper(name string) bool {
	if name == "" {
		return false
	}
	c := name[0]
	return c >= 'A' && c <= 'Z'
}

func isPrimitiveTypeName(name string) bool {
	switch name {
	case "string", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"bool", "float32", "float64", "any", "interface", "byte", "rune":
		return true
	}
	return false
}
