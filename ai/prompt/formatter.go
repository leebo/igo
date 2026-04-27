package prompt

import (
	"fmt"
	"strings"

	"github.com/igo/igo/core"
)

// AIHint 错误对应的 AI 提示
var AIHint = map[string]string{
	"BAD_REQUEST":         "检查请求参数格式是否正确，确认 Content-Type 是 application/json",
	"UNAUTHORIZED":        "需要在请求头中添加 Authorization: Bearer <token>",
	"FORBIDDEN":           "当前用户没有权限执行此操作，检查角色或权限配置",
	"NOT_FOUND":           "请求的资源不存在，检查 ID 或路径参数是否正确",
	"VALIDATION_FAILED":   "参数验证失败，检查 validate tag 和实际输入值是否匹配",
	"INTERNAL_ERROR":      "服务器内部错误，查看服务器日志获取详细信息",
}

// ErrorContext 错误上下文信息
type ErrorContext struct {
	Field  string `json:"field,omitempty"`
	Value  any    `json:"value,omitempty"`
	Rule   string `json:"rule,omitempty"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

// AIErrorResponse AI 友好的错误响应
type AIErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	AIHint  string        `json:"ai_hint,omitempty"`
	Context *ErrorContext `json:"context,omitempty"`
}

// FormatError 将错误格式化为 AI 友好格式
func FormatError(statusCode int, code, message string, ctx *ErrorContext) AIErrorResponse {
	hint := ""
	if h, ok := AIHint[code]; ok {
		hint = h
	}
	if ctx != nil && ctx.Hint != "" {
		hint = ctx.Hint
	}

	return AIErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			AIHint:  hint,
			Context: ctx,
		},
	}
}

// BuildAIHintFromError 根据错误信息构建 AI 提示
func BuildAIHintFromError(err error, c *core.Context) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// 验证错误
	if strings.Contains(errMsg, "validation") || strings.Contains(errMsg, "validate") {
		return "参数验证失败，检查 validate tag 定义和实际输入值"
	}

	// BindJSON 错误
	if strings.Contains(errMsg, "json") || strings.Contains(errMsg, "body") {
		return "请求体 JSON 格式错误，确保 Content-Type: application/json 且 body 是有效 JSON"
	}

	// 参数错误
	if strings.Contains(errMsg, "param") || strings.Contains(errMsg, "argument") {
		return "参数缺失或类型错误，检查路由参数和查询参数"
	}

	return fmt.Sprintf("错误信息: %s", errMsg)
}

// SuggestFix 基于错误类型建议修复方案
func SuggestFix(code string) []string {
	suggestions := []string{}
	switch code {
	case "BAD_REQUEST":
		suggestions = []string{
			"检查请求体是否是有效的 JSON",
			"确认 Content-Type 是 application/json",
			"检查字段名称和类型是否匹配",
		}
	case "VALIDATION_FAILED":
		suggestions = []string{
			"查看 struct 的 validate tag 定义",
			"确保必填字段有值",
			"检查格式验证规则(如 email, min, max)",
		}
	case "UNAUTHORIZED":
		suggestions = []string{
			"在请求头添加 Authorization",
			"格式: Authorization: Bearer <access_token>",
			"检查 token 是否过期",
		}
	case "NOT_FOUND":
		suggestions = []string{
			"确认资源 ID 存在",
			"检查路径参数是否正确",
			"确认资源已被删除",
		}
	}
	return suggestions
}
