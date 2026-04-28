// Package adapter 提供与 gin 框架的兼容适配
//
// 两种使用方式：
//
// 方式 1: 混合架构（推荐用于迁移或需要 gin 完整生态的场景）
//
//  import (
//      "github.com/gin-gonic/gin"
//      "github.com/igo/igo"
//      "github.com/igo/igo/adapter"
//  )
//
//  app := igo.New()
//  ge := adapter.NewGinEngine()
//  ge.Use(gin.Logger())
//  ge.Use(gin.Recovery())
//  ge.GET("/api/ping", func(c *gin.Context) { c.JSON(200, gin.H{"msg":"pong"}) })
//  adapter.Mount(app, "/api", ge)  // /api/* 由 gin 处理
//
//  app.GET("/web/health", func(c *igo.Context) { c.Success(igo.H{"status": "ok"}) })
//
// 方式 2: Gin 风格 API（用于用 gin 风格编写中间件）
//
//  app.Use(adapter.Middleware(func(gc *adapter.GinContext) {
//      gc.Header("X-Custom", "value")
//      gc.Next()
//  }))
package adapter

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/igo/igo/core"
)

// NewGinEngine 创建一个新的 gin Engine
func NewGinEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// NewGinEngineWithMode 创建一个指定模式的 gin Engine
func NewGinEngineWithMode(mode string) *gin.Engine {
	gin.SetMode(mode)
	return gin.New()
}

// Mount 将 gin.Engine 作为子路由挂载到 igo
// path/*subpath 的请求将由 gin 处理
//
// 示例：
//
//  app := igo.New()
//  ge := gin.New()
//  ge.Use(gin.Logger())
//  ge.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{"msg":"pong"}) })
//  adapter.Mount(app, "/api", ge)
//
//  // GET /api/ping -> gin 处理
//  // GET /web/health -> igo 处理
// Mount 将 gin.Engine 挂载到 igo 的指定前缀，支持所有 HTTP 方法
func Mount(app *core.App, path string, ginEngine *gin.Engine) {
	handler := func(c *core.Context) {
		originalPath := c.Request.URL.Path
		newPath := strings.TrimPrefix(originalPath, path)
		if newPath == "" {
			newPath = "/"
		}
		c.Request.URL.Path = newPath
		c.Request.RequestURI = newPath
		ginEngine.ServeHTTP(c.Writer, c.Request)
	}
	subpath := path + "/*subpath"
	app.Get(subpath, handler)
	app.Post(subpath, handler)
	app.Put(subpath, handler)
	app.Delete(subpath, handler)
	app.Patch(subpath, handler)
	app.Options(subpath, handler)
}

// GinContext 提供了与 gin.Context 兼容的 API
// 用于在 igo 中以 gin 的风格编写中间件
type GinContext struct {
	igoCtx  *core.Context
	data    map[string]interface{}
	written bool
}

// NewGinContext 从 igo Context 创建 GinContext
func NewGinContext(c *core.Context) *GinContext {
	return &GinContext{
		igoCtx: c,
		data:   c.GinContextData,
	}
}

func (gc *GinContext) Request() *http.Request { return gc.igoCtx.Request }

func (gc *GinContext) Writer() http.ResponseWriter {
	return &GinResponseWriter{ResponseWriter: gc.igoCtx.Writer, statusCode: 200}
}

func (gc *GinContext) Param(key string) string     { return gc.igoCtx.Param(key) }
func (gc *GinContext) Query(key string) string     { return gc.igoCtx.Query(key) }
func (gc *GinContext) GetHeader(key string) string { return gc.igoCtx.Request.Header.Get(key) }
func (gc *GinContext) FullPath() string           { return gc.igoCtx.Request.URL.Path }
func (gc *GinContext) Method() string            { return gc.igoCtx.Request.Method }
func (gc *GinContext) Path() string             { return gc.igoCtx.Request.URL.Path }

func (gc *GinContext) Set(key string, value interface{}) { gc.data[key] = value }
func (gc *GinContext) Get(key string) (interface{}, bool) {
	v, ok := gc.data[key]
	return v, ok
}

// Next 继续执行后续处理器（igo 路由）
func (gc *GinContext) Next() { gc.igoCtx.Next() }

// Abort 停止后续处理
func (gc *GinContext) Abort() {}

// IsAborted 检查是否已终止
func (gc *GinContext) IsAborted() bool   { return false }
func (gc *GinContext) IsWritten() bool   { return gc.written }

// AbortWithStatusJSON 终止并返回 JSON 错误
func (gc *GinContext) AbortWithStatusJSON(code int, obj interface{}) {
	gc.JSON(code, obj)
}

// AbortWithStatus 终止并返回状态码
func (gc *GinContext) AbortWithStatus(code int) {
	gc.igoCtx.Status(code)
}

// JSON 返回 JSON 响应
func (gc *GinContext) JSON(code int, obj interface{}) {
	gc.written = true
	gc.igoCtx.JSON(code, obj)
}

// Status 设置响应状态码
func (gc *GinContext) Status(code int) { gc.igoCtx.Status(code) }

// Header 设置响应头
func (gc *GinContext) Header(key, value string) { gc.igoCtx.Header(key, value) }

// BindJSON 绑定 JSON body
func (gc *GinContext) BindJSON(obj interface{}) error { return gc.igoCtx.BindJSON(obj) }

// ShouldBindJSON 绑定 JSON body
func (gc *GinContext) ShouldBindJSON(obj interface{}) error { return gc.igoCtx.BindJSON(obj) }

// BindQuery 绑定查询参数
func (gc *GinContext) BindQuery(obj interface{}) error { return gc.igoCtx.BindQuery(obj) }

// ShouldBindQuery 绑定查询参数
func (gc *GinContext) ShouldBindQuery(obj interface{}) error { return gc.igoCtx.BindQuery(obj) }

// GinMiddlewareFunc 是 gin 风格的中间件函数类型
type GinMiddlewareFunc func(*GinContext)

// Middleware 将 GinMiddlewareFunc 转换为 igo 中间件
func Middleware(middleware GinMiddlewareFunc) core.MiddlewareFunc {
	return func(c *core.Context) {
		gc := NewGinContext(c)
		middleware(gc)
	}
}

// Middlewares 将多个 GinMiddlewareFunc 转换为多个 igo 中间件
func Middlewares(middlewares ...GinMiddlewareFunc) []core.MiddlewareFunc {
	result := make([]core.MiddlewareFunc, len(middlewares))
	for i, m := range middlewares {
		result[i] = Middleware(m)
	}
	return result
}

// GinResponseWriter 包装 igo ResponseWriter
type GinResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *GinResponseWriter) Status() int { return w.statusCode }

func (w *GinResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *GinResponseWriter) Written() bool { return w.statusCode != 0 }

func (w *GinResponseWriter) CloseNotify() <-chan bool {
	if c, ok := w.ResponseWriter.(http.CloseNotifier); ok {
		return c.CloseNotify()
	}
	return nil
}

func (w *GinResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
