package main

import (
	"encoding/json"
	"log"

	"github.com/igo/igo"
	"github.com/igo/igo/ai"
	"github.com/igo/igo/ai/metadata"
	"github.com/igo/igo/ai/prompt"
	"github.com/igo/igo/middleware"
)

// UserRequest 创建用户的请求结构
type UserRequest struct {
	Name  string `json:"name" validate:"required|min:2"`
	Email string `json:"email" validate:"required|email"`
}

// UserResponse 用户响应结构
type UserResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	// 创建 AI 元数据注册表
	registry := ai.NewRegistry()

	// 注册路由元数据
	registerRouteMeta(registry)

	// 创建应用
	app := igo.New()

	// 注册 AI 优化中间件
	app.Use(prompt.AIOptimized())
	app.Use(prompt.WithAIHints())
	app.Use(middleware.Logger())

	// 注册路由
	registerUserRoutes(app)

	// AI Schema 端点
	app.Get("/_ai/schema", func(c *igo.Context) {
		spec := ai.GenerateOpenAPI(registry)
		c.JSON(200, spec)
	})

	// AI 提示词端点
	app.Get("/_ai/prompt", func(c *igo.Context) {
		routes := registry.ListRoutes()
		p := ai.GenerateProjectPrompt(routes)
		c.JSON(200, igo.H{"prompt": p})
	})

	log.Println("AI-enabled igo server starting on :8080")
	log.Println("Schema endpoint: GET /_ai/schema")
	log.Println("Prompt endpoint: GET /_ai/prompt")
	app.Run(":8080")
}

// registerRouteMeta 注册路由元数据
func registerRouteMeta(registry *ai.Registry) {
	registry.RegisterRoute(&metadata.RouteMeta{
		Method:  "POST",
		Path:    "/users",
		Summary: "创建新用户",
		Parameters: []metadata.ParamMeta{
			{Name: "name", In: "body", Type: "string", Required: true, Description: "用户名"},
			{Name: "email", In: "body", Type: "string", Required: true, Description: "用户邮箱"},
		},
		Responses: []metadata.ResponseMeta{
			{StatusCode: 201, Description: "用户创建成功"},
			{StatusCode: 400, Description: "请求参数错误"},
		},
	})

	registry.RegisterRoute(&metadata.RouteMeta{
		Method:  "GET",
		Path:    "/users",
		Summary: "获取用户列表",
		Parameters: []metadata.ParamMeta{
			{Name: "page", In: "query", Type: "int", Required: false, Description: "页码"},
			{Name: "size", In: "query", Type: "int", Required: false, Description: "每页数量"},
		},
		Responses: []metadata.ResponseMeta{
			{StatusCode: 200, Description: "用户列表"},
		},
	})

	registry.RegisterRoute(&metadata.RouteMeta{
		Method:  "GET",
		Path:    "/users/:id",
		Summary: "获取用户详情",
		Parameters: []metadata.ParamMeta{
			{Name: "id", In: "path", Type: "int", Required: true, Description: "用户ID"},
		},
		Responses: []metadata.ResponseMeta{
			{StatusCode: 200, Description: "用户信息"},
			{StatusCode: 404, Description: "用户不存在"},
		},
	})
}

// registerUserRoutes 注册路由处理函数
func registerUserRoutes(app *igo.App) {
	// 创建用户
	app.Post("/users", func(c *igo.Context) {
		var req UserRequest
		if err := c.BindJSON(&req); err != nil {
			c.BadRequest("Invalid request body: " + err.Error())
			return
		}

		// 模拟创建用户
		user := UserResponse{
			ID:    1,
			Name:  req.Name,
			Email: req.Email,
		}
		c.Created(user)
	})

	// 获取用户列表
	app.Get("/users", func(c *igo.Context) {
		page := c.QueryInt("page", 1)
		size := c.QueryInt("size", 10)

		users := []UserResponse{
			{ID: 1, Name: "Alice", Email: "alice@example.com"},
			{ID: 2, Name: "Bob", Email: "bob@example.com"},
		}

		c.Success(igo.H{
			"data":  users,
			"page":  page,
			"size":  size,
			"total": len(users),
		})
	})

	// 获取单个用户
	app.Get("/users/:id", func(c *igo.Context) {
		id := c.Param("id")
		if id == "0" {
			c.NotFound("User not found")
			return
		}

		c.Success(UserResponse{
			ID:    1,
			Name:  "Test User",
			Email: "test@example.com",
		})
	})
}

// 示例: 使用 ParseHandlerDoc 解析注解
func exampleParseAnnotations() {
	doc := `// igo:summary: 创建新用户
// igo:description: 创建一个新的用户账号
// igo:param:name:body:string:用户名
// igo:param:email:body:string:用户邮箱
// igo:response:201:用户创建成功
// igo:response:400:请求参数错误
`
	meta := ai.ParseHandlerDoc(doc)
	b, _ := json.MarshalIndent(meta, "", "  ")
	log.Println(string(b))
}
