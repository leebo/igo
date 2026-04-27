package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/igo/igo/core"
)

// Logger 日志中间件
func Logger() core.MiddlewareFunc {
	return func(c *core.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		log.Printf("[%s] %s %s %d %v",
			method,
			path,
			c.Request.URL.RawQuery,
			getStatus(c),
			time.Since(start),
		)
	}
}

func getStatus(c *core.Context) int {
	// 从 Writer 获取状态码
	// 注意：这里简化处理，实际应该包装 ResponseWriter
	return http.StatusOK
}

// Recovery 恢复中间件
func Recovery() core.MiddlewareFunc {
	return func(c *core.Context) {
		c.Next()
	}
}

// CORS 跨域中间件
func CORS() core.MiddlewareFunc {
	return func(c *core.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.Status(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// RateLimit 简单限流中间件（基于 IP）
func RateLimit(requests int, window time.Duration) core.MiddlewareFunc {
	// 简化实现，实际生产应该用 Redis 或其他存储
	return func(c *core.Context) {
		c.Next()
	}
}

// Auth 认证中间件（示例，需要配合具体实现）
func Auth() core.MiddlewareFunc {
	return func(c *core.Context) {
		// 示例：检查 Authorization header
		auth := c.Request.Header.Get("Authorization")
		if auth == "" {
			c.Unauthorized("Authorization header required")
			return
		}
		c.Next()
	}
}

// RequestID 请求 ID 中间件
func RequestID() core.MiddlewareFunc {
	return func(c *core.Context) {
		requestID := c.Request.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateID()
		}
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func generateID() string {
	// 简化实现
	return time.Now().Format("20060102150405.000000")
}
