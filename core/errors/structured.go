package errors

import (
	"fmt"
	"runtime"
)

// CallFrame 调用栈中的一帧
type CallFrame struct {
	FunctionName string `json:"functionName"`
	FilePath    string `json:"filePath"`
	Line        int    `json:"line"`
}

// StructuredError 带有完整元数据的错误
type StructuredError struct {
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	Field       string            `json:"field,omitempty"`
	Rule        string            `json:"rule,omitempty"`
	Value       any               `json:"value,omitempty"`
	FilePath    string            `json:"filePath,omitempty"`
	Line        int               `json:"line,omitempty"`
	Suggestions []string         `json:"suggestions,omitempty"`
	Metadata    map[string]any   `json:"metadata,omitempty"`
	CallChain   []CallFrame       `json:"callChain,omitempty"`   // 调用链
	RootCause   *StructuredError  `json:"rootCause,omitempty"`    // 根本原因
}

// ValidationErrors 验证错误集合
type ValidationErrors struct {
	Errors []StructuredError `json:"errors"`
}

// ErrorContext 错误上下文 (用于传递给 AI)
type ErrorContext struct {
	FilePath     string   `json:"filePath"`
	Line         int      `json:"line"`
	FunctionName string   `json:"functionName,omitempty"`
	Suggestions  []string `json:"suggestions,omitempty"`
}

// ErrorCode 定义
const (
	CodeBadRequest      = "BAD_REQUEST"
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeForbidden       = "FORBIDDEN"
	CodeNotFound        = "NOT_FOUND"
	CodeValidation      = "VALIDATION_FAILED"
	CodeInternalError   = "INTERNAL_ERROR"
	CodeInvalidJSON     = "INVALID_JSON"
	CodeMissingField    = "MISSING_FIELD"
	CodeInvalidFormat   = "INVALID_FORMAT"
	CodeOutOfRange      = "OUT_OF_RANGE"
)

// NewStructuredError 创建结构化错误
func NewStructuredError(code, message string) *StructuredError {
	return &StructuredError{
		Code:    code,
		Message: message,
	}
}

// WithField 设置字段
func (e *StructuredError) WithField(field string) *StructuredError {
	e.Field = field
	return e
}

// WithRule 设置验证规则
func (e *StructuredError) WithRule(rule string) *StructuredError {
	e.Rule = rule
	return e
}

// WithValue 设置错误值
func (e *StructuredError) WithValue(value any) *StructuredError {
	e.Value = value
	return e
}

// WithFilePath 设置文件路径
func (e *StructuredError) WithFilePath(path string) *StructuredError {
	e.FilePath = path
	return e
}

// WithLine 设置行号
func (e *StructuredError) WithLine(line int) *StructuredError {
	e.Line = line
	return e
}

// WithSuggestions 设置建议
func (e *StructuredError) WithSuggestions(suggestions ...string) *StructuredError {
	e.Suggestions = suggestions
	return e
}

// WithMetadata 设置元数据
func (e *StructuredError) WithMetadata(key string, value any) *StructuredError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
	return e
}

// Error 实现 error 接口
func (e *StructuredError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Field, e.Message, e.Rule)
	}
	return e.Message
}

// Wrap 包装底层错误，添加调用链信息
func (e *StructuredError) Wrap(err error, message string) *StructuredError {
	if err == nil {
		return e
	}

	// 创建新的包装错误
	wrapped := &StructuredError{
		Code:    e.Code,
		Message: message,
	}

	// 如果底层错误也是 StructuredError，保留其信息
	if se, ok := err.(*StructuredError); ok {
		wrapped.RootCause = se
		wrapped.CallChain = append([]CallFrame{}, se.CallChain...)
	} else {
		wrapped.RootCause = &StructuredError{
			Message: err.Error(),
		}
	}

	// 添加当前调用帧
	if pc, file, line, ok := runtime.Caller(1); ok {
		wrapped.CallChain = append(wrapped.CallChain, CallFrame{
			FunctionName: runtime.FuncForPC(pc).Name(),
			FilePath:    file,
			Line:        line,
		})
	}

	return wrapped
}

// AddCallFrame 添加调用帧
func (e *StructuredError) AddCallFrame() *StructuredError {
	if pc, file, line, ok := runtime.Caller(1); ok {
		e.CallChain = append(e.CallChain, CallFrame{
			FunctionName: runtime.FuncForPC(pc).Name(),
			FilePath:    file,
			Line:        line,
		})
	}
	return e
}

// WithCallChain 设置调用链
func (e *StructuredError) WithCallChain(chain []CallFrame) *StructuredError {
	e.CallChain = chain
	return e
}

// WithRootCause 设置根本原因
func (e *StructuredError) WithRootCause(cause *StructuredError) *StructuredError {
	e.RootCause = cause
	return e
}

// Unwrap 返回被包装的底层错误
func (e *StructuredError) Unwrap() error {
	if e.RootCause != nil {
		return e.RootCause
	}
	return nil
}

// GetCallChain 获取完整的调用链（包含根因的调用链）
func (e *StructuredError) GetCallChain() []CallFrame {
	if e.RootCause != nil && len(e.RootCause.CallChain) > 0 {
		chain := make([]CallFrame, 0, len(e.RootCause.CallChain)+len(e.CallChain))
		chain = append(chain, e.RootCause.CallChain...)
		chain = append(chain, e.CallChain...)
		return chain
	}
	return e.CallChain
}

// NewValidationError 创建验证错误
func NewValidationError(field, rule, message string) *StructuredError {
	return &StructuredError{
		Code:    CodeValidation,
		Message: message,
		Field:   field,
		Rule:    rule,
	}
}

// WithSuggestionsForValidation 添加验证建议
func (e *StructuredError) WithSuggestionsForValidation() *StructuredError {
	switch e.Rule {
	case "required":
		e.Suggestions = []string{
			"确保字段有值",
			"检查字段是否正确绑定",
		}
	case "email":
		e.Suggestions = []string{
			"检查邮箱格式是否为 user@example.com",
			"确保没有多余空格",
		}
	case "min":
		e.Suggestions = []string{
			"增加字段值的长度或数值",
		}
	case "max":
		e.Suggestions = []string{
			"减少字段值的长度或数值",
		}
	case "enum":
		e.Suggestions = []string{
			"确保值是允许的选项之一",
			"检查拼写是否正确",
		}
	case "eqfield":
		e.Suggestions = []string{
			"确保两个字段值相同",
			"检查字段名是否正确",
		}
	default:
		e.Suggestions = []string{
			"检查字段格式是否正确",
			"查看 validate tag 定义",
		}
	}
	return e
}

// ErrorCodeFromStatus 根据 HTTP 状态码获取错误码
func ErrorCodeFromStatus(status int) string {
	switch status {
	case 400:
		return CodeBadRequest
	case 401:
		return CodeUnauthorized
	case 403:
		return CodeForbidden
	case 404:
		return CodeNotFound
	case 422:
		return CodeValidation
	default:
		return CodeInternalError
	}
}
