# igo Framework - AI 编程友好 Web 框架

## 项目概述

igo 是一个专为 AI 编程工具（Claude Code, Codex）设计的 Go Web 框架，旨在减少 AI 的 Token 消耗、提高代码生成准确率、加快开发速度。

**设计目标**:
- Token 消耗降低 40-50%
- 代码生成准确率提升 30-40%
- AI 友好度评分: 9/10

---

## 快速开始

### 极简模式（推荐）

使用 `igo.Simple()` 自动包含默认中间件（Recovery/CORS/Logger）：

```go
package main

import igo "github.com/igo/igo"

func main() {
    app := igo.Simple()
    app.Get("/health", func(c *igo.Context) {
        c.Success(igo.H{"status": "ok"})
    })
    app.Run(":8080")
}
```

### 标准模式

```go
package main

import (
    igo "github.com/igo/igo"
    "github.com/igo/igo/middleware"
)

func main() {
    app := igo.New()
    app.Use(middleware.Recovery())
    app.Use(middleware.CORS())
    app.Use(middleware.Logger())
    app.Get("/health", healthHandler)
    app.Run(":8080")
}
```

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
├── services/               # 业务逻辑层
│   └── user.go             # 用户服务
├── repositories/           # 数据访问层
│   └── user.go             # 用户仓库
├── routes/                 # 路由注册（推荐）
│   └── routes.go            # 集中路由定义
└── config/                 # 配置
    └── config.go            # 配置加载
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

// 参数获取（推荐使用便捷方法）
c.Query("name")              // 获取查询参数
c.QueryInt("page", 1)        // 获取整数参数，带默认值
c.QueryInt64("id", 0)        // 获取 int64 参数
c.QueryBool("active", false) // 获取布尔参数
c.QueryInt64OrFail("id")     // 获取参数，无效则自动 400

c.Param("id")                // 获取路径参数
c.ParamInt64("id")           // 获取 int64 路径参数
c.ParamInt64OrFail("id")    // 获取路径参数，无效则自动 400
c.ParamBool("active")        // 获取布尔路径参数

// BindJSON 绑定请求体
c.BindJSON(&req)

// 响应方法（统一使用这三个）
c.Success(data)              // 200 OK {data: ...}
c.Created(data)              // 201 Created {data: ...}
c.NoContent()                // 204 No Content
c.JSON(status, data)          // 自定义响应

// 错误响应
c.BadRequest(message)        // 400
c.NotFound(message)          // 404
c.Unauthorized(message)     // 401
c.Forbidden(message)         // 403
c.InternalError(message)     // 500

// 带调用链的错误响应（推荐）
c.InternalErrorWrap(err, "操作失败", map[string]any{"id": id})
c.NotFoundWrap(err, "用户不存在")
c.BadRequestWrap(err, "请求无效")
c.ValidationErrorWrap(err, "email", "邮箱格式错误")

// 便捷错误检查
c.FailIfError(err, "操作失败")           // err != nil 时自动 500
c.NotFoundIfNotFound(err, "user")       // err != nil 或 v == nil 时自动 404
c.SuccessIfNotNil(user, "user")          // v == nil 时自动 404，否则返回 v
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

### 3. 结构化错误（带调用链）

igo 提供结构化错误响应，AI 可以直接解析：

```json
{
  "error": {
    "code": "INTERNAL_ERROR",
    "message": "failed to fetch user",
    "callChain": [
      {"functionName": "handlers.getUser", "filePath": "handlers/user.go", "line": 42},
      {"functionName": "core.Context.InternalErrorWrap", "filePath": "core/context.go", "line": 168}
    ],
    "rootCause": {
      "code": "NOT_FOUND",
      "message": "user with id 123 not found"
    },
    "metadata": {"user_id": 123}
  }
}
```

**使用示例**：

```go
// 之前的写法
func getUser(c *core.Context) {
    user, err := userRepo.GetByID(ctx, id)
    if err != nil {
        c.InternalError("failed to fetch user")  // 无调用链
        return
    }
    c.Success(user)
}

// 新的写法（推荐）
func getUser(c *core.Context) {
    user, err := userRepo.GetByID(ctx, id)
    if err != nil {
        c.InternalErrorWrap(err, "failed to fetch user", map[string]any{"user_id": id})
        return
    }
    c.Success(user)
}
```

**错误自动日志**：设置 `core.SetLogger(logClient)` 后，所有错误响应会自动记录日志，不需要手动 `log.Error()`。

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
// 使用预定义中间件
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

## 配置校验

igo 提供配置校验功能，启动前检查配置有效性：

```go
import "github.com/igo/igo/plugin/config"

func main() {
    cfg, err := config.LoadFromFile("./", "config", "yaml")
    if err != nil {
        log.Fatal(err)
    }

    // 启动前验证配置
    if err := cfg.Validate(); err != nil {
        log.Printf("⚠️ Config warning: %v", err)
    }

    // 生产环境使用更严格的校验
    if err := cfg.ValidateForProduction(); err != nil {
        log.Fatalf("❌ Production config error: %v", err)
    }
}
```

**校验内容**：
- `server.port` 是否设置
- `database.dialect` 和 `database.dsn` 是否设置
- `jwt.secret_key` 是否为有效值（非 placeholder）
- 生产环境额外检查：secret_key 长度、禁止使用 sqlite、log.level 不能是 debug

---

## 智能元数据推断

igo 自动从函数名和路径推断路由元数据，减少样板代码：

```go
// AI 只需要写
app.Get("/users/:id", getUser)

// 框架自动推断：
// - Summary: "Get user by ID"
// - Tags: ["users"]
// - Params: [{Name: "id", In: "path", Type: "int"}]
```

**推断规则**：
- 函数名 `getUser` + GET `/users/:id` → "Get user by ID"
- 函数名 `listUsers` + GET `/users` → "List users"
- 路径 `/api/v1/users/:id` → Tags: ["users"]
- 参数名以 `id` 结尾 → int 类型
- 参数名以 `page/size/limit` 结尾 → int 类型

**显式覆盖**：显式指定的元数据优先于推断值

```go
app.Get("/users/:id", getUser,
    route.WithSummary("Custom summary"),  // 显式优先
    route.WithTags("custom"),             // 覆盖推断的 ["users"]
)
```

---

## 标准分层架构

igo 推荐的标准项目结构，AI 可以从示例学习模式：

```
myapp/
├── main.go                  # 应用入口，初始化顺序：配置 -> 日志 -> 数据库 -> 缓存 -> 认证 -> 路由
├── config/
│   └── config.go            # 配置加载，支持 config.Validate() 校验
├── models/
│   └── user.go             # 数据模型，定义结构体和请求/响应类型
├── repositories/
│   └── user.go             # 数据访问层，封装数据库操作
├── services/
│   └── user.go             # 业务逻辑层，组合多个 Repository，缓存逻辑
├── handlers/
│   ├── user.go             # HTTP 处理层，调用 Service，返回响应
│   └── health.go           # 健康检查
├── middleware/
│   └── auth.go             # 认证中间件
└── routes/
    └── routes.go           # 路由注册，集中管理所有路由
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
