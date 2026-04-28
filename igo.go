// Package igo 是一个 AI 友好的 Go Web 框架
//
// 核心特点:
//
//   - 声明式路由 - 让 AI 理解意图而非实现细节
//   - 内置验证 - 无需额外依赖即可完成参数验证
//   - 统一响应格式 - 自动包装 {data: ...} 格式
//   - 极简 API - 减少 AI 需要生成的代码量
//
// 示例:
//
//	app := igo.New()
//	app.Get("/health", func(c *igo.Context) {
//	    c.Success(igo.H{"status": "ok"})
//	})
//	app.Run(":8080")
//
// 使用 Simple() 快速启动（带默认中间件）：
//
//	app := igo.Simple()
//	app.Get("/health", func(c *igo.Context) {
//	    c.Success(igo.H{"status": "ok"})
//	})
//	app.Run(":8080")
package igo

import (
	"github.com/igo/igo/core"
	"github.com/igo/igo/middleware"
)

// App 是 igo 应用的实例
type App = core.App

// Context 是请求上下文
type Context = core.Context

// HandlerFunc 是处理函数类型
type HandlerFunc = core.HandlerFunc

// MiddlewareFunc 是中间件函数类型
type MiddlewareFunc = core.MiddlewareFunc

// H 是 map 别名，用于构建 JSON
type H = core.H

// ResourceHandler 是 RESTful 资源处理器
type ResourceHandler = core.ResourceHandler

// New 创建一个新的 igo 应用
func New() *App {
	return core.New()
}

// Simple 创建一个带有默认中间件的 igo 应用
// 默认中间件：Recovery、CORS、Logger
func Simple() *App {
	app := core.New()
	app.Use(middleware.Recovery())
	app.Use(middleware.CORS())
	app.Use(middleware.Logger())
	return app
}

// BindAndValidate 绑定 JSON 请求体并验证结构体，失败时自动返回错误响应
// 返回 (nil, false) 表示已发送响应，调用方应立即 return
//
// 用法：
//
//	req, ok := igo.BindAndValidate[CreateUserRequest](c)
//	if !ok { return }
func BindAndValidate[T any](c *Context) (*T, bool) {
	return core.BindAndValidate[T](c)
}

// BindQueryAndValidate 绑定 URL 查询参数并验证结构体，失败时自动返回错误响应
//
// 用法：
//
//	q, ok := igo.BindQueryAndValidate[ListQuery](c)
//	if !ok { return }
func BindQueryAndValidate[T any](c *Context) (*T, bool) {
	return core.BindQueryAndValidate[T](c)
}

// BindPathAndValidate 绑定 :path 参数并验证结构体，失败时自动返回错误响应
//
// 用法：
//
//	p, ok := igo.BindPathAndValidate[ResourceParams](c)
//	if !ok { return }
func BindPathAndValidate[T any](c *Context) (*T, bool) {
	return core.BindPathAndValidate[T](c)
}

// RegisterAppSchema 把类型 T 显式注册到指定 App 的 /_ai/schemas 输出。
//
// 用法：
//
//	igo.RegisterAppSchema[UserResponse](app)
func RegisterAppSchema[T any](app *App) {
	core.RegisterAppSchema[T](app)
}

// RegisterSchema 把类型 T 显式注册到兼容全局 schema 注册表。
//
// Deprecated: use app.RegisterSchema(UserResponse{}) or RegisterAppSchema[T](app)
// so schemas stay isolated per App.
func RegisterSchema[T any]() {
	core.RegisterSchema[T]()
}
