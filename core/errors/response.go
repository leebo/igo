package errors

import "fmt"

// ErrorResponse AI 友好的错误响应
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	Field       string         `json:"field,omitempty"`
	Suggestions []string       `json:"suggestions,omitempty"`
	Context     *ErrorContext  `json:"context,omitempty"`
	Details     []ErrorDetail  `json:"details,omitempty"` // 多字段错误时使用
	Metadata    map[string]any `json:"metadata,omitempty"`
	CallChain   []CallFrame    `json:"callChain,omitempty"` // 调用链
	RootCause   *ErrorDetail   `json:"rootCause,omitempty"` // 根本原因
	TraceID     string         `json:"traceId,omitempty"`   // 由 RequestID 中间件写入，便于关联日志
}

// NewErrorResponse 从 StructuredError 创建错误响应
func NewErrorResponse(err *StructuredError) ErrorResponse {
	if err == nil {
		return ErrorResponse{Error: ErrorDetail{
			Code:    CodeInternalError,
			Message: "unknown error",
		}}
	}

	detail := ErrorDetail{
		Code:    err.Code,
		Message: err.Message,
		Field:   err.Field,
	}

	if err.Suggestions != nil {
		detail.Suggestions = err.Suggestions
	}

	if err.FilePath != "" || err.Line != 0 {
		detail.Context = &ErrorContext{
			FilePath:    err.FilePath,
			Line:        err.Line,
			Suggestions: err.Suggestions,
		}
	}

	if err.Metadata != nil {
		detail.Metadata = err.Metadata
	}

	if len(err.CallChain) > 0 {
		detail.CallChain = err.CallChain
	}

	// 转换根因
	if err.RootCause != nil {
		detail.RootCause = &ErrorDetail{
			Code:    err.RootCause.Code,
			Message: err.RootCause.Message,
			Field:   err.RootCause.Field,
		}
		if len(err.RootCause.CallChain) > 0 {
			detail.RootCause.CallChain = err.RootCause.CallChain
		}
	}

	return ErrorResponse{Error: detail}
}

// NewErrorResponseFromValidationErrors 从多个验证错误创建响应
func NewErrorResponseFromValidationErrors(errs ValidationErrors) ErrorResponse {
	if len(errs.Errors) == 0 {
		return ErrorResponse{Error: ErrorDetail{
			Code:    CodeValidation,
			Message: "validation failed",
		}}
	}

	if len(errs.Errors) == 1 {
		return NewErrorResponse(&errs.Errors[0])
	}

	// 多个错误时，使用 details 数组
	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:    CodeValidation,
			Message: "multiple validation errors",
			Details: make([]ErrorDetail, len(errs.Errors)),
		},
	}

	for i, err := range errs.Errors {
		detail := ErrorDetail{
			Code:    err.Code,
			Message: err.Message,
			Field:   err.Field,
		}
		if err.Suggestions != nil {
			detail.Suggestions = err.Suggestions
		}
		resp.Error.Details[i] = detail
	}

	return resp
}

// SimpleErrorResponse 创建简单错误响应（用于非验证错误）
func SimpleErrorResponse(code, message string) ErrorResponse {
	return ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
}

// WithSuggestions 添加建议到错误响应
func (r *ErrorResponse) WithSuggestions(suggestions ...string) *ErrorResponse {
	r.Error.Suggestions = suggestions
	return r
}

// WithTraceID 返回填入 trace ID 的副本（通常由 RequestID 中间件生成）。
// 空字符串会被忽略并返回原 response。
func (r ErrorResponse) WithTraceID(traceID string) ErrorResponse {
	if traceID == "" {
		return r
	}
	r.Error.TraceID = traceID
	return r
}

// WithContext 添加上下文到错误响应
func (r *ErrorResponse) WithContext(ctx *ErrorContext) *ErrorResponse {
	r.Error.Context = ctx
	return r
}

// String 返回错误的标准字符串格式
func (r *ErrorResponse) String() string {
	return fmt.Sprintf("[%s] %s", r.Error.Code, r.Error.Message)
}

// ValidationErrorHint 根据验证规则返回 AI 友好的提示
var ValidationErrorHint = map[string][]string{
	"required": {
		"确保字段有值",
		"检查字段是否正确绑定到请求",
	},
	"email": {
		"邮箱格式应为 user@example.com",
		"确保没有多余空格或特殊字符",
	},
	"min": {
		"值太小，增加长度或数值",
		"检查是否满足最小值要求",
	},
	"max": {
		"值太大，减少长度或数值",
		"检查是否超过最大值限制",
	},
	"gte": {
		"值小于最小允许值",
		"确保值大于等于最小值",
	},
	"lte": {
		"值大于最大允许值",
		"确保值小于等于最大值",
	},
	"enum": {
		"值不在允许的选项列表中",
		"确保使用预定义的选项之一",
	},
	"eqfield": {
		"两个字段的值不相等",
		"确保要比较的字段有相同的值",
	},
	"uuid": {
		"格式应为 xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
		"可以使用 uuidgen 生成",
	},
	"url": {
		"格式应为有效的 URL，如 https://example.com",
		"确保包含协议前缀",
	},
	"json": {
		"格式应为有效的 JSON",
		"检查引号和括号是否匹配",
	},
}

// GetHintForRule 获取规则对应的提示
func GetHintForRule(rule string) []string {
	if hints, ok := ValidationErrorHint[rule]; ok {
		return hints
	}
	return []string{"检查字段格式是否正确"}
}
