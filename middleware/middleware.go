package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/leebo/igo/core"
)

// corsPrdWarnOnce guards the prd-mode "no allowed origins configured" log so
// it appears at most once per process even when multiple Apps construct
// CORSFor(prd) middleware (e.g. tests, embedded apps).
var corsPrdWarnOnce sync.Once

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

// LoggerFor 根据运行模式返回合适详细度的 Logger 中间件:
//   - dev:  与 Logger() 一致 (访问日志 + raw query)
//   - test: 静默 (不打印,避免污染 test output)
//   - prd:  仅打印 method/path/status/duration,不带 raw query (避免 token/secret 泄漏)
func LoggerFor(mode core.Mode) core.MiddlewareFunc {
	switch mode {
	case core.ModeTest:
		return func(c *core.Context) { c.Next() }
	case core.ModePrd:
		return func(c *core.Context) {
			start := time.Now()
			method := c.Request.Method
			path := c.Request.URL.Path
			c.Next()
			log.Printf("[%s] %s %d %v", method, path, c.StatusCode(), time.Since(start))
		}
	default:
		return Logger()
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

// RecoveryFor 根据运行模式返回合适的 Recovery 中间件:
//   - dev/test: panic 信息直接写进响应体,便于调试
//   - prd:      响应体仅 "Internal server error",panic 详情写到日志 (避免栈泄漏)
func RecoveryFor(mode core.Mode) core.MiddlewareFunc {
	if mode.IsPrd() {
		return func(c *core.Context) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("[igo/recovery] panic: %v", err)
					if c.StatusCode() == 0 {
						c.InternalError("Internal server error")
					}
				}
			}()
			c.Next()
		}
	}
	return Recovery()
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

// CORSFor 根据运行模式返回合适的 CORS 中间件:
//   - dev/test: 允许所有源 (与 CORS() 一致)
//   - prd:      不允许任何跨源请求 (拒绝写入 Access-Control-Allow-* 头),
//               并在初始化时打 WARN 日志,提示用户应用 CORSWithConfig 显式配置
func CORSFor(mode core.Mode) core.MiddlewareFunc {
	if mode.IsPrd() {
		corsPrdWarnOnce.Do(func() {
			log.Println("[igo SECURITY] CORSFor(prd): no allowed origins configured; cross-origin requests will be rejected. " +
				"Pass an explicit middleware.CORSWithConfig(...) in prd to silence this and define your origin policy.")
		})
		return func(c *core.Context) {
			if c.Request.Method == http.MethodOptions {
				c.Status(http.StatusForbidden)
				return
			}
			c.Next()
		}
	}
	return CORS()
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

	originStr := strings.Join(origins, ", ")
	methodStr := strings.Join(methods, ", ")
	headerStr := strings.Join(headers, ", ")

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
