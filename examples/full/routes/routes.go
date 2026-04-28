// Package routes 定义路由注册
//
// 标准 igo 路由组织模式（AI 应当模仿）：
//   - 公开接口和需要认证的接口拆成两个 Group
//   - 中间件作为 Group 第三参数传入，不要在 Group 闭包内 Use
//   - 同一前缀可以用多个 Group 拆分中间件需求
package routes

import (
	"github.com/igo/igo"
	"github.com/igo/igo/core"
	"github.com/igo/igo/examples/full/handlers"
	fullMiddleware "github.com/igo/igo/examples/full/middleware"
	igoMiddleware "github.com/igo/igo/middleware"
	"github.com/igo/igo/plugin/auth"
)

// RegisterRoutes 注册所有路由
func RegisterRoutes(app *igo.App, userHandler *handlers.UserHandler, jwtClient *auth.Client) {
	// 全局中间件（顺序：Recovery → CORS → Logger）
	app.Use(igoMiddleware.Recovery())
	app.Use(igoMiddleware.CORS())
	app.Use(igoMiddleware.Logger())

	// 健康检查
	app.Get("/health", handlers.NewHealth().Check)

	// AI 自省端点：/_ai/routes /_ai/middlewares /_ai/info /_ai/schemas /_ai/errors
	// 让 AI 在调试时不必读源码就能拿到运行时事实
	app.RegisterAIRoutes()

	// 公开 API（无需认证）
	app.Group("/api/v1", func(v1 *igo.App) {
		v1.Post("/auth/login", login)
		v1.Post("/auth/register", register)
	})

	// 需要认证的 API（Auth 中间件作为 Group 第三参数）
	app.Group("/api/v1", func(authed *igo.App) {
		authed.Get("/users", userHandler.List)
		authed.Get("/users/:id", userHandler.Get)
		authed.Post("/users", userHandler.Create)
		authed.Put("/users/:id", userHandler.Update)
		authed.Delete("/users/:id", userHandler.Delete)
	}, fullMiddleware.Auth(jwtClient))
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" validate:"required|min:3"`
	Password string `json:"password" validate:"required|min:6"`
}

// login 处理登录请求
func login(c *core.Context) {
	req, ok := core.BindAndValidate[LoginRequest](c)
	if !ok {
		return
	}

	// TODO: 验证用户凭证，生成 token
	if req.Username == "" || req.Password == "" {
		c.Unauthorized("invalid credentials")
		return
	}

	c.Success(core.H{"message": "login successful"})
}

// register 处理注册请求
func register(c *core.Context) {
	c.BadRequest("use /api/v1/users POST to register")
}

// RegisterRoutesWithServices 便捷别名，同 RegisterRoutes
func RegisterRoutesWithServices(app *igo.App, userHandler *handlers.UserHandler, jwtClient *auth.Client) {
	RegisterRoutes(app, userHandler, jwtClient)
}
