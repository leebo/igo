package core

import "net/http"

// HandlerFunc 是处理函数类型
type HandlerFunc func(*Context)

// MiddlewareFunc 是中间件函数类型（与 HandlerFunc 签名相同）
type MiddlewareFunc = HandlerFunc

// ResourceHandler 是 RESTful 资源处理器
type ResourceHandler struct {
	List   func(*Context)
	Show   func(*Context)
	Create func(*Context)
	Update func(*Context)
	Delete func(*Context)
}

// wrapResourceHandler 将 ResourceHandler 方法包装为 HandlerFunc
func wrapResourceHandler(fn func(*Context)) HandlerFunc {
	return func(c *Context) {
		fn(c)
	}
}

// ListResponse 是列表响应结构
type ListResponse[T any] struct {
	Data  []T   `json:"data"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

// ErrorCode 错误码常量
const (
	ErrCodeBadRequest    = "BAD_REQUEST"
	ErrCodeUnauthorized  = "UNAUTHORIZED"
	ErrCodeForbidden     = "FORBIDDEN"
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeValidation    = "VALIDATION_FAILED"
	ErrCodeInternalError = "INTERNAL_ERROR"
)

// HTTP 状态码常量
const (
	StatusOK                  = http.StatusOK
	StatusCreated             = http.StatusCreated
	StatusNoContent           = http.StatusNoContent
	StatusBadRequest          = http.StatusBadRequest
	StatusUnauthorized        = http.StatusUnauthorized
	StatusForbidden           = http.StatusForbidden
	StatusNotFound            = http.StatusNotFound
	StatusUnprocessableEntity = http.StatusUnprocessableEntity
	StatusInternalError       = http.StatusInternalServerError
)
