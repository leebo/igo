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
	"github.com/leebo/igo/core"
	"github.com/leebo/igo/middleware"
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

// Mode 是当前运行模式 (dev/test/prd) 的别名
type Mode = core.Mode

// 模式常量,转发自 core 包
const (
	ModeDev  = core.ModeDev
	ModeTest = core.ModeTest
	ModePrd  = core.ModePrd
)

// Simple 创建一个带有默认中间件的 igo 应用,默认值随 IGO_ENV 变化:
//
//   - dev (默认): 详细 Logger、CORS *、Recovery 含 stack、自动注册 /_ai/*
//   - test:       静默 Logger、CORS *、Recovery 含 stack、自动注册 /_ai/*
//   - prd:        结构化 Logger、严格 CORS、Recovery 不外泄 stack、不自动注册 /_ai/*
//
// 在 prd 下若用户未配置 CORS,Simple 会以「拒绝跨源」启动并打 WARN 日志。
//
// BREAKING (since env-mode change): 在 dev/test 下 Simple 会自动调用
// app.RegisterAIRoutes(),旧版本不会。如果你的应用已经定义了任何
// /_ai/* 路径,会发生路由冲突或覆盖 —— 请改用 SimpleWithoutAI() 或
// igo.New() 自行组合中间件。
func Simple() *App {
	app := core.New()
	app.Use(middleware.RecoveryFor(app.Mode))
	app.Use(middleware.CORSFor(app.Mode))
	app.Use(middleware.LoggerFor(app.Mode))
	if !app.Mode.IsPrd() {
		app.RegisterAIRoutes()
	}
	return app
}

// SimpleWithoutAI 等价于 Simple(),但**不**自动注册 /_ai/* 端点。
// 适合已经自己定义 /_ai/foo 路径、或者出于安全考虑不想暴露自省接口的用户。
// 若需要某些 AI 端点,可在返回后手动 app.RegisterAIRoutes()。
func SimpleWithoutAI() *App {
	app := core.New()
	app.Use(middleware.RecoveryFor(app.Mode))
	app.Use(middleware.CORSFor(app.Mode))
	app.Use(middleware.LoggerFor(app.Mode))
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
