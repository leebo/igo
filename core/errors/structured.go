package errors

import "fmt"

// StructuredError 带有完整元数据的错误
type StructuredError struct {
	Code        string   `json:"code"`
	Message     string   `json:"message"`
	Field       string   `json:"field,omitempty"`
	Rule        string   `json:"rule,omitempty"`
	Value       any      `json:"value,omitempty"`
	FilePath    string   `json:"filePath,omitempty"`
	Line        int      `json:"line,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
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
