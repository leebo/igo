# igo Framework - AI 编程友好 Web 框架

## 项目概述

igo 是一个专为 AI 编程工具（Claude Code, Codex）设计的 Go Web 框架，旨在减少 AI 的 Token 消耗、提高代码生成准确率、加快开发速度。

**设计目标**:
- Token 消耗降低 40-50%
- 代码生成准确率提升 30-40%
- AI 友好度评分: 7.5/10

---

## 项目结构

```
myapp/
├── main.go                 # 应用入口
├── handlers/               # 处理函数
│   ├── user.go             # 用户相关
│   └── order.go            # 订单相关
├── middleware/             # 自定义中间件
│   └── auth.go             # 认证中间件
├── models/                 # 数据模型
│   └── user.go             # User 结构体
├── routes/                 # 路由注册（推荐）
│   └── routes.go            # 集中路由定义
├── types/                  # 类型定义
│   └── schemas/            # TypeSchema 导出
└── core/                   # igo 核心
```

---

## 核心概念

### 1. Context 请求上下文

```go
// 基础方法
c.Request        // *http.Request
c.Writer         // http.ResponseWriter
c.Params         // map[string]string - 路径参数
c.QueryArgs      // urlValues - 查询参数

// 参数获取
c.Query("name")           // 获取查询参数
c.QueryInt("name", 0)     // 获取整数参数
c.Param("id")             // 获取路径参数
c.BindJSON(&req)          // 绑定 JSON body

// 响应方法
c.Success(data)           // 200 OK
c.Created(data)           // 201 Created
c.NoContent()             // 204 No Content
c.BadRequest(message)     // 400 Bad Request
c.NotFound(message)       // 404 Not Found
c.Unauthorized(message)   // 401 Unauthorized
c.Forbidden(message)      // 403 Forbidden
c.InternalError(message)  // 500 Internal Server Error
c.ValidationError(err)    // 422 Unprocessable Entity
c.JSON(status, data)      // 自定义 JSON 响应
```

### 2. 路由注册

```go
// 基础路由
app.Get("/path", handler)      // GET
app.Post("/path", handler)      // POST
app.Put("/path", handler)       // PUT
app.Delete("/path", handler)    // DELETE
app.Patch("/path", handler)     // PATCH

// 带中间件的路由
app.Get("/path", handler, middleware.Auth())

// 路由组
app.Group("/api/v1", func(v1 *igo.App) {
    v1.Get("/users", listUsers)
    v1.Post("/users", createUser)
}, middleware.Auth())

// RESTful 资源路由
app.Resources("/users", ResourceHandler{
    List:   listUsers,
    Show:   getUser,
    Create: createUser,
    Update: updateUser,
    Delete: deleteUser,
})
```

### 3. 结构化错误

igo 提供结构化错误响应，AI 可以直接解析：

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "validation failed",
    "field": "email",
    "suggestions": ["Check email format", "Ensure field is not empty"],
    "context": {
      "filePath": "handlers/user.go",
      "line": 42
    }
  }
}
```

### 4. 验证规则

| 规则 | 示例 | 说明 |
|------|------|------|
| required | `validate:"required"` | 必填字段 |
| email | `validate:"email"` | 邮箱格式 |
| min | `validate:"min:2"` | 最小长度/值 |
| max | `validate:"max:100"` | 最大长度/值 |
| gte | `validate:"gte:0"` | 大于等于 |
| lte | `validate:"lte:100"` | 小于等于 |
| enum | `validate:"enum:red,green,blue"` | 枚举值 |
| eqfield | `validate:"eqfield:Password"` | 字段相等 |
| uuid | `validate:"uuid"` | UUID 格式 |
| url | `validate:"url"` | URL 格式 |
| regex | `validate:"regex:^[a-z]+$"` | 正则匹配 |

### 5. 中间件系统

**默认中间件顺序**（按优先级）:

```
1. Recovery (100) - Panic 恢复，核心安全
2. CORS (80) - 跨域处理
3. Auth (70) - JWT 认证
4. RateLimit (60) - 限流
5. Logger (50) - 请求日志
6. RequestID (40) - 请求 ID
```

**中间件定义示例**:

```go
// 使用中间件
app.Use(middleware.Logger())
app.Use(middleware.Recovery())
app.Use(middleware.CORS())

// 自定义中间件
app.Use(func(c *igo.Context) {
    // 处理前
    c.Next()
    // 处理后
})
```

---

## 路由元数据

### 选项式路由配置

```go
import "github.com/igo/igo/core/route"

// 使用路由选项
app.Get("/users/:id",
    getUser,
    route.WithSummary("Get user by ID"),
    route.WithDescription("Returns user details including profile"),
    route.WithPathParam("id", "int", "User ID"),
    route.WithResponse(200, "User details", "User"),
    route.WithResponse(404, "User not found", ""),
    route.WithTags("users"),
    route.WithAIHint("Check if user exists before returning"),
)
```

### 参数定义

```go
// 路径参数
route.WithPathParam("id", "int", "User ID")

// 查询参数
route.WithQueryParam("page", "int", "Page number", false)
route.WithQueryParam("size", "int", "Page size", false)

// Header 参数
route.WithHeaderParam("Authorization", "string", "Bearer token", true)
```

### 响应定义

```go
// 成功响应
route.WithSuccessResponse("User created", "User")

// 创建响应
route.WithCreatedResponse("User created successfully", "User")

// 自定义状态码
route.WithResponse(201, "Created", "User")
route.WithResponse(400, "Bad request", "Error")
route.WithResponse(404, "Not found", "Error")
route.WithResponse(422, "Validation error", "ValidationError")
```

---

## 类型 Schema

### 结构体定义

```go
type User struct {
    ID    int64  `json:"id"`
    Name  string `json:"name" validate:"required|min:2|max:100"`
    Email string `json:"email" validate:"required|email"`
    Age   int    `json:"age" validate:"gte:0|lte:150"`
    Status string `json:"status" validate:"enum:active,inactive,pending"`
}
```

### 提取 TypeSchema

```go
import "github.com/igo/igo/types"

// 提取类型 Schema
schema := types.ExtractSchema(User{})
fmt.Println(schema.Name)        // "User"
fmt.Println(schema.Fields[0].Name)  // "ID"
fmt.Println(schema.Fields[0].Type)   // "int64"
fmt.Println(schema.Fields[1].ValidateTag)  // "required|min:2|max:100"
```

---

## AI 调试功能

### 错误响应中的 AI 提示

igo 的错误响应包含 suggestions 字段，帮助 AI 快速定位问题：

```go
// 验证错误会自动添加建议
c.ValidationError(err)
// ->
// {
//   "error": {
//     "code": "VALIDATION_FAILED",
//     "message": "email format is invalid",
//     "field": "email",
//     "suggestions": [
//       "Check email format",
//       "Ensure field is not empty"
//     ]
//   }
// }
```

### AI 友好的中间件链

中间件定义包含 aiHints，AI 可以理解中间件作用：

```go
middleware.GetDefinition("Auth").AIHints
// -> ["Add X-User-ID header after successful auth"]
```

---

## Token 优化技巧

### 1. 使用路由选项而非注释

```go
// 不推荐 - AI 需要解析注释
// igo:summary: Get user by ID
app.Get("/users/:id", getUser)

// 推荐 - 结构化选项
app.Get("/users/:id", getUser,
    route.WithSummary("Get user by ID"),
    route.WithResponse(200, "User details", "User"),
)
```

### 2. 复用类型 Schema

```go
// 在 types 包中定义类型
type User struct {
    ID    int64  `json:"id"`
    Name  string `json:"name"`
}

// 在多个地方引用同一类型
app.Post("/users", createUser, route.WithRequestBody("application/json", "User"))
app.Put("/users/:id", updateUser, route.WithRequestBody("application/json", "User"))
```

### 3. 使用预定义中间件

```go
// 不推荐 - 自定义中间件需要更多上下文
app.Use(customLogger)

// 推荐 - 使用预定义中间件，AI 已知其行为
app.Use(middleware.Logger())
app.Use(middleware.Recovery())
```

### 4. 结构化错误响应

```go
// 不推荐 - 字符串错误
c.BadRequest("invalid email")

// 推荐 - AI 可解析的结构化错误
c.ValidationError(err)
// AI 可以直接从响应中提取 field、rule、suggestions
```

---

## 常见模式

### RESTful 资源处理

```go
// 定义资源处理器
h := ResourceHandler{
    List:   listUsers,
    Show:   getUser,
    Create: createUser,
    Update: updateUser,
    Delete: deleteUser,
}

// 注册资源路由
app.Resources("/users", h)

// 生成路由:
// GET    /users          -> List
// POST   /users          -> Create
// GET    /users/:id      -> Show
// PUT    /users/:id      -> Update
// DELETE /users/:id      -> Delete
```

### 带验证的请求处理

```go
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required|min:2|max:100"`
    Email string `json:"email" validate:"required|email"`
}

func createUser(c *igo.Context) {
    var req CreateUserRequest
    if err := c.BindJSON(&req); err != nil {
        c.BadRequest("Invalid request body")
        return
    }

    if err := validator.Validate(req); err != nil {
        c.ValidationError(err)
        return
    }

    // 业务逻辑
    user := createUserInDB(req)
    c.Created(user)
}
```

### 路由分组与中间件

```go
// 公开 API 分组
app.Group("/api/public", func(public *igo.App) {
    public.Get("/health", healthCheck)
    public.Post("/auth/login", login)
})

// 需要认证的 API 分组
app.Group("/api/v1", func(v1 *igo.App) {
    v1.Use(middleware.Auth())

    v1.Get("/users", listUsers)
    v1.Post("/users", createUser)
    v1.Get("/users/:id", getUser)
}, middleware.Auth())
```

---

## CLI 命令

```bash
# 安装
go get github.com/igo/igo

# 运行示例
go run examples/simple/main.go

# 运行测试
go test ./...

# 构建
go build ./...
```

---

## 扩展阅读

- [API 文档](./docs/api.md)
- [中间件开发](./docs/middleware.md)
- [验证器扩展](./docs/validator.md)
- [AI 集成示例](./examples/aidemo/main.go)
