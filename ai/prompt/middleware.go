package prompt

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/igo/igo/core"
)

// AIOptimized 返回一个 AI 优化的中间件
// 自动为错误响应添加 AI 友好的提示信息
func AIOptimized() core.MiddlewareFunc {
	return func(c *core.Context) {
		c.Next()

		// 只处理错误响应
		if c.StatusCode() < 400 {
			return
		}

		// 获取原始响应
		body := getResponseBody(c)
		if body == nil {
			return
		}

		// 解析原始错误
		var errResp map[string]interface{}
		if err := json.Unmarshal(body, &errResp); err != nil {
			return
		}

		// 提取错误信息
		code := extractErrorCode(errResp)
		message := extractErrorMessage(errResp)

		// 构建 AI 友好的错误响应
		aiResp := FormatError(c.StatusCode(), code, message, nil)

		// 写回响应
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.WriteHeader(c.StatusCode())
		json.NewEncoder(c.Writer).Encode(aiResp)
	}
}

// responseCapture 捕获响应体
type responseCapture struct {
	body   *bytes.Buffer
	status int
}

func (r *responseCapture) Status() int { return r.status }
func (r *responseCapture) Write(b []byte) (int, error) {
	r.body.Write(b)
	return len(b), nil
}
func (r *responseCapture) WriteHeader(status int) {
	r.status = status
}

var jsonContentTypeRe = regexp.MustCompile(`application/json`)

// getResponseBody 获取响应体
func getResponseBody(c *core.Context) []byte {
	// 检查是否是 JSON 响应
	contentType := c.Writer.Header().Get("Content-Type")
	if !jsonContentTypeRe.MatchString(contentType) {
		return nil
	}
	return nil
}

// extractErrorCode 从错误响应中提取错误码
func extractErrorCode(resp map[string]interface{}) string {
	if errObj, ok := resp["error"].(map[string]interface{}); ok {
		if code, ok := errObj["code"].(string); ok {
			return code
		}
	}
	return "UNKNOWN_ERROR"
}

// extractErrorMessage 从错误响应中提取错误消息
func extractErrorMessage(resp map[string]interface{}) string {
	if errObj, ok := resp["error"].(map[string]interface{}); ok {
		if msg, ok := errObj["message"].(string); ok {
			return msg
		}
	}
	return "Unknown error"
}

// WithAIHints 返回带有 AI 提示的中间件
// 在响应头中添加 AI 调试提示
func WithAIHints() core.MiddlewareFunc {
	return func(c *core.Context) {
		c.Next()

		if c.StatusCode() >= 400 {
			code := extractErrorCodeFromStatus(c.StatusCode())
			if hint, ok := AIHint[code]; ok {
				c.Header("X-AI-Hint", hint)
			}
		}
	}
}

// extractErrorCodeFromStatus 根据状态码提取错误码
func extractErrorCodeFromStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusUnprocessableEntity:
		return "VALIDATION_FAILED"
	default:
		return "INTERNAL_ERROR"
	}
}

// AIDebugMiddleware AI 调试中间件
// 在开发环境下提供更详细的错误信息
func AIDebugMiddleware(env string) core.MiddlewareFunc {
	return func(c *core.Context) {
		if env != "development" {
			c.Next()
			return
		}

		c.Next()

		// 开发环境添加额外调试信息
		if c.StatusCode() >= 400 {
			c.Header("X-Debug-Request-ID", c.Request.Header.Get("X-Request-ID"))
			c.Header("X-Debug-Method", c.Request.Method)
			c.Header("X-Debug-Path", c.Request.URL.Path)

			code := extractErrorCodeFromStatus(c.StatusCode())
			if hints, ok := AIHintCodeSuggestions[code]; ok {
				c.Header("X-AI-Suggestions", strings.Join(hints, "; "))
			}
		}
	}
}

// AIHintCodeSuggestions 错误码对应的建议
var AIHintCodeSuggestions = map[string][]string{
	"BAD_REQUEST": {
		"检查请求体格式是否为有效 JSON",
		"确认 Content-Type 为 application/json",
		"检查字段名称是否与 struct tag 一致",
	},
	"VALIDATION_FAILED": {
		"检查 validate tag 规则",
		"确认必填字段有值",
		"检查字段格式(email, min, max 等)",
	},
	"UNAUTHORIZED": {
		"添加 Authorization 请求头",
		"格式: Authorization: Bearer <token>",
		"检查 token 是否过期",
	},
	"FORBIDDEN": {
		"检查用户权限配置",
		"确认角色是否足够",
	},
	"NOT_FOUND": {
		"确认资源 ID 存在",
		"检查路径参数是否正确",
	},
}

// LogError 记录错误并返回 AI 友好的错误消息
func LogError(err error, c *core.Context) {
	hint := BuildAIHintFromError(err, c)
	log.Printf("[AI-HINT] %s | Path: %s | Error: %v",
		hint, c.Request.URL.Path, err)
}
