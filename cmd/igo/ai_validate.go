package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/leebo/igo/ai/schema"
)

// ValidateIssue 是 `igo ai validate` 的单条违规项。
type ValidateIssue struct {
	Severity string `json:"severity"` // error | warning
	Code     string `json:"code"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
}

// ValidateReport 是整个项目的元数据自洽校验报告。
// AI 客户端可以根据 ok 字段判断本次提交是否安全。
type ValidateReport struct {
	OK          bool             `json:"ok"`
	RouteCount  int              `json:"routeCount"`
	SchemaCount int              `json:"schemaCount"`
	Issues      []ValidateIssue  `json:"issues"`
	Stats       map[string]int   `json:"stats,omitempty"`
}

func runAIValidate(root string, w io.Writer) int {
	project, err := loadStaticProject(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ai validate: %v\n", err)
		return 1
	}

	report := ValidateReport{
		RouteCount:  len(project.routes),
		SchemaCount: len(project.schemas),
		Stats:       map[string]int{},
	}

	// 1. doctor 风格的元数据完整性检查
	for _, d := range checkResponseTypeNotRegistered(project) {
		report.Issues = append(report.Issues, diagToIssue(d, "error"))
	}
	for _, d := range checkInvalidValidateTag(project) {
		report.Issues = append(report.Issues, diagToIssue(d, "error"))
	}
	for _, d := range checkSchemaUnusedInRoutes(project) {
		report.Issues = append(report.Issues, diagToIssue(d, "warning"))
	}

	// 2. OpenAPI 一致性：所有 $ref 指向的 schema 必须在 components.schemas 中存在
	gen := schema.NewRouteGenerator(project.routes, project.schemas...)
	spec := gen.Generate()
	dangling := findDanglingRefs(spec)
	for _, ref := range dangling {
		report.Issues = append(report.Issues, ValidateIssue{
			Severity: "error",
			Code:     "openapi-dangling-ref",
			Message:  "OpenAPI references " + ref + " which is not present in components.schemas",
		})
	}

	// 3. 每条路由必须至少有一个 2xx response
	for _, route := range project.routes {
		has2xx := false
		for _, resp := range route.Responses {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				has2xx = true
				break
			}
		}
		if !has2xx {
			report.Issues = append(report.Issues, ValidateIssue{
				Severity: "warning",
				Code:     "route-missing-success-response",
				Message:  fmt.Sprintf("route %s %s has no 2xx response declared (missing c.Success / c.Created / c.NoContent?)", route.Method, route.Path),
				File:     route.FilePath,
				Line:     route.LineNumber,
			})
		}
	}

	// 4. 中间件名称一致性：路由声明的中间件应该是实际可解析的标识符
	for _, route := range project.routes {
		for _, mw := range route.Middlewares {
			if mw == "" {
				continue
			}
			report.Stats["middleware:"+mw]++
		}
	}

	// 排序，按 severity → code → message 稳定输出
	sort.SliceStable(report.Issues, func(i, j int) bool {
		if report.Issues[i].Severity != report.Issues[j].Severity {
			return report.Issues[i].Severity < report.Issues[j].Severity
		}
		if report.Issues[i].Code != report.Issues[j].Code {
			return report.Issues[i].Code < report.Issues[j].Code
		}
		return report.Issues[i].Message < report.Issues[j].Message
	})

	report.OK = !hasErrorSeverity(report.Issues)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "ai validate: encode: %v\n", err)
		return 1
	}
	if !report.OK {
		return 1
	}
	return 0
}

func diagToIssue(d Diagnostic, override string) ValidateIssue {
	sev := d.Severity
	if override != "" {
		sev = override
	}
	return ValidateIssue{
		Severity: sev,
		Code:     d.Rule,
		Message:  d.Message,
		File:     d.File,
		Line:     d.Line,
	}
}

func hasErrorSeverity(issues []ValidateIssue) bool {
	for _, it := range issues {
		if it.Severity == "error" {
			return true
		}
	}
	return false
}

// findDanglingRefs 扫描整个 OpenAPI spec，把所有 "$ref" 字符串收集起来，
// 然后对照 components.schemas 看哪些没有定义。
func findDanglingRefs(spec any) []string {
	defined := schemaNames(spec)
	missing := map[string]struct{}{}
	walkRefs(spec, func(ref string) {
		const prefix = "#/components/schemas/"
		if !strings.HasPrefix(ref, prefix) {
			return
		}
		name := strings.TrimPrefix(ref, prefix)
		if _, ok := defined[name]; ok {
			return
		}
		missing[name] = struct{}{}
	})
	out := make([]string, 0, len(missing))
	for n := range missing {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func schemaNames(spec any) map[string]struct{} {
	out := map[string]struct{}{}
	root, ok := spec.(map[string]any)
	if !ok {
		// 走反射路径：把 spec 转 JSON 再读回 map。生成器返回 struct，先序列化。
		raw, err := json.Marshal(spec)
		if err != nil {
			return out
		}
		_ = json.Unmarshal(raw, &root)
	}
	comp, _ := root["components"].(map[string]any)
	if comp == nil {
		return out
	}
	schemas, _ := comp["schemas"].(map[string]any)
	for name := range schemas {
		out[name] = struct{}{}
	}
	return out
}

// walkRefs 把 spec（任何嵌套的 map / slice / struct）转 JSON 再遍历，对所有 "$ref" 字符串调用 fn。
func walkRefs(spec any, fn func(string)) {
	raw, err := json.Marshal(spec)
	if err != nil {
		return
	}
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return
	}
	walkRefsValue(root, fn)
}

func walkRefsValue(v any, fn func(string)) {
	switch val := v.(type) {
	case map[string]any:
		if ref, ok := val["$ref"].(string); ok {
			fn(ref)
		}
		for _, child := range val {
			walkRefsValue(child, fn)
		}
	case []any:
		for _, child := range val {
			walkRefsValue(child, fn)
		}
	}
}
