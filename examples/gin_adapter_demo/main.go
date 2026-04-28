// 示例：igo + gin 混合架构
//
// 方式 1: gin 作为子应用（使用完整 gin 生态）
// 方式 2: gin 风格 API 编写中间件
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/igo/igo"
	"github.com/igo/igo/adapter"
)

func main() {
	app := igo.New()

	// ========== 方式 1: gin 作为子应用 ==========
	// 创建 gin engine
	ginEngine := adapter.NewGinEngine()
	ginEngine.Use(gin.Logger())
	ginEngine.Use(gin.Recovery())

	// gin 路由
	ginEngine.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong", "from": "gin"})
	})

	ginEngine.POST("/echo", func(c *gin.Context) {
		var req map[string]interface{}
		c.BindJSON(&req)
		c.JSON(200, req)
	})

	// 挂载 gin 到 /api 路径
	adapter.Mount(app, "/api", ginEngine)

	// ========== 方式 2: gin 风格 API 编写中间件 ==========
	app.Use(adapter.Middleware(func(gc *adapter.GinContext) {
		gc.Header("X-Custom-Header", "Hello from gin-style middleware")
		gc.Set("request-id", "12345")
		gc.Next()
	}))

	app.Use(adapter.Middleware(func(gc *adapter.GinContext) {
		requestID, _ := gc.Get("request-id")
		gc.Header("X-Request-ID", requestID.(string))
		gc.Next()
	}))

	// ========== igo 路由 ==========
	app.Get("/health", func(c *igo.Context) {
		c.Success(igo.H{"status": "ok", "from": "igo"})
	})

	app.Get("/user/:id", func(c *igo.Context) {
		id := c.Param("id")
		c.Success(igo.H{"id": id, "name": "John Doe"})
	})

	// 启动服务器
	app.Run(":8080")
}
