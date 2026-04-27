package main

import (
	"net/http"

	igo "github.com/igo/igo"
	"github.com/igo/igo/core"
	"github.com/igo/igo/middleware"
)

// User 模型
type User struct {
	ID    int64  `json:"id"`
	Name  string `json:"name" validate:"required|min:2|max:50"`
	Email string `json:"email" validate:"required|email"`
	Age   int    `json:"age" validate:"gte:0|lte:150"`
}

func main() {
	app := igo.New()

	// 全局中间件
	app.Use(middleware.Logger())
	app.Use(middleware.Recovery())
	app.Use(middleware.CORS())

	// GET /health
	app.Get("/health", func(c *core.Context) {
		c.Success(core.H{
			"status": "ok",
			"version": "1.0.0",
		})
	})

	// 路由组
	app.Group("/api/v1", func(v1 *igo.App) {
		// GET /api/v1/users - 用户列表
		v1.Get("/users", func(c *core.Context) {
			page := c.QueryInt("page", 1)
			size := c.QueryInt("size", 20)

			users := []User{
				{ID: 1, Name: "张三", Email: "zhangsan@example.com", Age: 25},
				{ID: 2, Name: "李四", Email: "lisi@example.com", Age: 30},
			}

			c.Success(core.H{
				"data":  users,
				"total": 2,
				"page":  page,
				"size":  size,
			})
		})

		// GET /api/v1/users/:id - 获取单个用户
		v1.Get("/users/:id", func(c *core.Context) {
			id := c.Param("id")
			c.Success(User{
				ID:    1,
				Name:  "张三",
				Email: "zhangsan@example.com",
				Age:   25,
			})
			_ = id
		})

		// POST /api/v1/users - 创建用户
		v1.Post("/users", func(c *core.Context) {
			var user User
			if err := c.BindJSON(&user); err != nil {
				c.BadRequest("Invalid request body")
				return
			}
			user.ID = 1
			c.Created(user)
		})

		// PUT /api/v1/users/:id - 更新用户
		v1.Put("/users/:id", func(c *core.Context) {
			id := c.Param("id")
			var user User
			if err := c.BindJSON(&user); err != nil {
				c.BadRequest("Invalid request body")
				return
			}
			user.ID = 1
			c.Success(user)
			_ = id
		})

		// DELETE /api/v1/users/:id - 删除用户
		v1.Delete("/users/:id", func(c *core.Context) {
			id := c.Param("id")
			c.NoContent()
			_ = id
		})
	})

	// 设置 404 处理器
	app.Use(func(c *core.Context) {
		c.Status(http.StatusNotFound)
	})

	// 启动服务
	app.Run(":8080")
}
