// Package errors 定义业务错误
//
// igo:summary: 业务错误定义
// igo:description: 定义应用中使用的业务错误码和错误处理
// igo:tag: errors
package errors

import (
	"errors"
	"net/http"

	"github.com/leebo/igo/core"
)

// 业务错误码定义
var (
	// ErrUserNotFound 用户不存在
	ErrUserNotFound = errors.New("user not found")
	// ErrEmailExists 邮箱已被使用
	ErrEmailExists = errors.New("email already exists")
	// ErrInvalidCredentials 凭证无效
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrTokenExpired Token 已过期
	ErrTokenExpired = errors.New("token expired")
	// ErrUnauthorized 未授权
	ErrUnauthorized = errors.New("unauthorized")
)

// BusinessError 业务错误响应
//
// igo:summary: Business error
// igo:description: 将业务错误转换为 HTTP 响应
// igo:ai-hint: 根据错误类型返回对应的 HTTP 状态码和消息
func BusinessError(c *core.Context, err error) {
	if errors.Is(err, ErrUserNotFound) {
		c.NotFound("user not found")
		return
	}
	if errors.Is(err, ErrEmailExists) {
		c.BadRequest("email already exists")
		return
	}
	if errors.Is(err, ErrInvalidCredentials) {
		c.Unauthorized("invalid credentials")
		return
	}
	if errors.Is(err, ErrTokenExpired) {
		c.JSON(http.StatusUnauthorized, core.H{
			"error": core.H{
				"code":    "TOKEN_EXPIRED",
				"message": "token has expired",
			},
		})
		return
	}
	c.InternalError("internal server error")
}
