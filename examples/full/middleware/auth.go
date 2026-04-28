// Package middleware 定义中间件
//
// igo:summary: 中间件层
// igo:description: 提供可复用的请求处理逻辑，如认证、日志、限流等
// igo:tag: middleware
package middleware

import (
	"strconv"

	"github.com/leebo/igo/core"
	"github.com/leebo/igo/plugin/auth"
)

// Auth 认证中间件
//
// igo:summary: JWT 认证中间件
// igo:description: 验证请求头中的 JWT token，有效则设置 X-User-ID header
// igo:ai-hint: 认证失败返回 401，成功则调用 c.Next() 继续处理
// igo:header-param:Authorization:string:Bearer token,true
func Auth(jwtClient *auth.Client) core.MiddlewareFunc {
	return func(c *core.Context) {
		token := c.Request.Header.Get("Authorization")
		if token == "" {
			c.Unauthorized("token required")
			return
		}

		claims, err := jwtClient.Validate(token)
		if err != nil {
			c.Unauthorized("invalid token")
			return
		}

		c.Header("X-User-ID", strconv.FormatInt(claims.UserID, 10))
		c.Next()
	}
}
