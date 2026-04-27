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
// 适用于快速开发和小项目
func Simple() *App {
	app := core.New()
	app.Use(middleware.Recovery())
	app.Use(middleware.CORS())
	app.Use(middleware.Logger())
	return app
}
