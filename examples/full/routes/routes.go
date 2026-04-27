// Package routes 定义路由注册
//
// igo:summary: 路由注册
// igo:description: 集中管理所有路由的注册，便于维护和查找
// igo:tag: routes
package routes

import (
	"github.com/igo/igo"
	"github.com/igo/igo/examples/full/handlers"
	fullMiddleware "github.com/igo/igo/examples/full/middleware"
	"github.com/igo/igo/plugin/auth"
	"github.com/igo/igo/core"
	igoMiddleware "github.com/igo/igo/middleware"
)

// RegisterRoutes 注册所有路由
//
// igo:summary: Register routes
// igo:description: 在 App 上注册所有路由和全局中间件
// igo:param:app:*igo.App:igo 应用实例
// igo:param:userHandler:*handlers.UserHandler:用户处理器
// igo:param:jwtClient:*auth.Client:JWT 客户端
// igo:ai-hint: 路由分组推荐：公开接口放一个 Group，需要认证的放另一个 Group
func RegisterRoutes(app *igo.App, userHandler *handlers.UserHandler, jwtClient *auth.Client) {
	// 全局中间件
	app.Use(igoMiddleware.Logger())
	app.Use(igoMiddleware.Recovery())
	app.Use(igoMiddleware.CORS())

	// 健康检查
	app.Get("/health", handlers.NewHealth().Check)

	// API v1 路由组
	app.Group("/api/v1", func(v1 *igo.App) {
		// 公开路由
		v1.Post("/auth/login", login)
		v1.Post("/auth/register", register)

		// 需要认证的路由
		v1.Use(fullMiddleware.Auth(jwtClient))

		v1.Get("/users", userHandler.List)
		v1.Get("/users/:id", userHandler.Get)
		v1.Post("/users", userHandler.Create)
		v1.Put("/users/:id", userHandler.Update)
		v1.Delete("/users/:id", userHandler.Delete)
	})
}

// login 处理登录请求
//
// igo:summary: User login
// igo:description: 用户登录，返回 JWT token
// igo:request-body:LoginRequest:登录请求
// igo:response:200:Token response:JWT token
// igo:response:401:Invalid credentials
func login(c *core.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request")
		return
	}

	if req.Username == "" || req.Password == "" {
		c.Unauthorized("invalid credentials")
		return
	}

	// TODO: 验证用户凭证，生成 token
	// 这里简化处理，实际应该查询数据库验证
	c.Success(core.H{
		"message": "login successful",
	})
}

// register 处理注册请求
//
// igo:summary: User registration
// igo:description: 用户注册
// igo:request-body:RegisterRequest:注册请求
// igo:response:201:User:创建的用户信息
// igo:response:400:Invalid request
func register(c *core.Context) {
	// 注册逻辑在 UserHandler.Create 中处理
	c.BadRequest("use /api/v1/users POST to register")
}

// RegisterRoutesWithServices 注册路由（使用 services）
//
// igo:summary: Register routes with services
// igo:description: 便捷构造函数，同时创建 handler 和注册路由
// igo:param:app:*igo.App:igo 应用实例
// igo:param:userService:*services.UserService:用户服务
// igo:param:jwtClient:*auth.Client:JWT 客户端
func RegisterRoutesWithServices(app *igo.App, userHandler *handlers.UserHandler, jwtClient *auth.Client) {
	RegisterRoutes(app, userHandler, jwtClient)
}
