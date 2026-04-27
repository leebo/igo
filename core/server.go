package core

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// App 是 igo 应用的实例
type App struct {
	Router  *Router
	prefix string
	server *http.Server
}

// New 创建一个新的 igo 应用
func New() *App {
	return &App{
		Router: NewRouter(),
	}
}

// Use 注册全局中间件
func (a *App) Use(middleware MiddlewareFunc) {
	a.Router.Use(middleware)
}

// Get 注册 GET 请求
func (a *App) Get(path string, handler HandlerFunc) {
	a.Router.GET(a.prefix+path, handler)
}

// Post 注册 POST 请求
func (a *App) Post(path string, handler HandlerFunc) {
	a.Router.POST(a.prefix+path, handler)
}

// Put 注册 PUT 请求
func (a *App) Put(path string, handler HandlerFunc) {
	a.Router.PUT(a.prefix+path, handler)
}

// Delete 注册 DELETE 请求
func (a *App) Delete(path string, handler HandlerFunc) {
	a.Router.DELETE(a.prefix+path, handler)
}

// Patch 注册 PATCH 请求
func (a *App) Patch(path string, handler HandlerFunc) {
	a.Router.PATCH(a.prefix+path, handler)
}

// Options 注册 OPTIONS 请求
func (a *App) Options(path string, handler HandlerFunc) {
	a.Router.OPTIONS(a.prefix+path, handler)
}

// Head 注册 HEAD 请求
func (a *App) Head(path string, handler HandlerFunc) {
	a.Router.HEAD(a.prefix+path, handler)
}

// Group 创建路由分组
func (a *App) Group(prefix string, fn func(*App), middlewares ...MiddlewareFunc) {
	subApp := &App{
		Router: a.Router,
		prefix: a.prefix + prefix,
	}
	fn(subApp)
}

// Resources 注册 RESTful 资源路由
func (a *App) Resources(path string, h ResourceHandler, middlewares ...MiddlewareFunc) {
	a.Router.Resources(a.prefix+path, h, middlewares...)
}

// SetNotFound 设置 404 处理器
func (a *App) SetNotFound(handler HandlerFunc) {
	a.Router.SetNotFound(handler)
}

// Run 启动服务器
func (a *App) Run(addr string) error {
	a.server = &http.Server{
		Addr:         addr,
		Handler:      a.Router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 启动服务器
	go func() {
		log.Printf("🚀 Server starting on %s", addr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")
	return a.server.Close()
}
