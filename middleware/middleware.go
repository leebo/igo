package middleware

import (
	"fmt"
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
			c.StatusCode(),
			time.Since(start),
		)
	}
}

// Recovery panic 恢复中间件，仅在尚未写入响应时才写入 500
func Recovery() core.MiddlewareFunc {
	return func(c *core.Context) {
		defer func() {
			if err := recover(); err != nil {
				if c.StatusCode() == 0 {
					c.InternalError(fmt.Sprintf("Internal server error: %v", err))
				}
			}
		}()
		c.Next()
	}
}

// CORSConfig CORS 配置
type CORSConfig struct {
	AllowOrigins []string // 允许的源，默认 ["*"]
	AllowMethods []string // 允许的方法
	AllowHeaders []string // 允许的请求头
}

var defaultCORSConfig = CORSConfig{
	AllowOrigins: []string{"*"},
	AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
	AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
}

// CORS 跨域中间件（允许所有源，开发用）
// 生产环境请使用 CORSWithConfig 指定允许的源
func CORS() core.MiddlewareFunc {
	return CORSWithConfig(defaultCORSConfig)
}

// CORSWithConfig 可配置的跨域中间件
func CORSWithConfig(cfg CORSConfig) core.MiddlewareFunc {
	origins := cfg.AllowOrigins
	if len(origins) == 0 {
		origins = defaultCORSConfig.AllowOrigins
	}
	methods := cfg.AllowMethods
	if len(methods) == 0 {
		methods = defaultCORSConfig.AllowMethods
	}
	headers := cfg.AllowHeaders
	if len(headers) == 0 {
		headers = defaultCORSConfig.AllowHeaders
	}

	originStr := joinStrings(origins, ", ")
	methodStr := joinStrings(methods, ", ")
	headerStr := joinStrings(headers, ", ")

	return func(c *core.Context) {
		c.Header("Access-Control-Allow-Origin", originStr)
		c.Header("Access-Control-Allow-Methods", methodStr)
		c.Header("Access-Control-Allow-Headers", headerStr)

		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// RateLimit 限流中间件占位（未实现实际限流逻辑）
// 实际限流需配合 plugin/cache/redis 实现分布式限流
func RateLimit(_ int, _ time.Duration) core.MiddlewareFunc {
	log.Println("[igo/middleware] RateLimit: stub implementation, no actual limiting applied")
	return func(c *core.Context) {
		c.Next()
	}
}

// Auth 认证中间件示例（仅检查 Authorization header 非空）
// 生产环境请使用 plugin/auth JWT 验证
func Auth() core.MiddlewareFunc {
	return func(c *core.Context) {
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
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
