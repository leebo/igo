# igo

igo 是一个 **AI 友好的 Go Web 框架**：保留 Go HTTP 开发的直接性，同时把路由、Schema、错误码、OpenAPI 和运行时自省做成 AI 工具可以稳定读取的结构化上下文。

项目地址：https://github.com/leebo/igo

## 为什么有 igo

多数 Go Web 框架主要优化人类开发体验、性能或生态完整度。AI 编程工具在接手一个项目时，真正缺的是短而准确的上下文：

- 当前项目有哪些路由？
- 每个 handler 在哪个文件、哪一行？
- 请求体、query、path 参数有哪些字段和校验规则？
- 错误响应格式和错误码是什么？
- OpenAPI 是否能直接生成，并且没有悬空 `$ref`？
- 运行中的服务能不能暴露同样的结构化元数据？

igo 的目标不是替代所有 Web 框架，而是让 AI 在改接口、写 handler、补测试、生成文档时更少猜测。

## 特性

- 极简 Handler API：`func(c *igo.Context)`
- 内置 JSON、Query、Path 绑定与校验
- 统一响应格式：`c.Success`、`c.Created`、`c.NoContent`
- AI 友好的错误响应和错误码表
- App 级路由和 Schema registry，避免多 App 测试互相污染
- 离线 AI CLI：`igo ai routes/schemas/openapi/context/prompt`
- 运行时 AI 端点：`/_ai/routes`、`/_ai/schemas`、`/_ai/openapi`
- OpenAPI 3.0 生成，包含 `components.schemas`
- Gin 适配器，方便迁移或复用 Gin 生态

## 安装

环境要求：Go 1.25+。

### 在项目中使用 igo

```bash
go get github.com/leebo/igo
```

代码中导入：

```go
import igo "github.com/leebo/igo"
```

### 安装 igo CLI

```bash
go install github.com/leebo/igo/cmd/igo@latest
```

安装后可以运行：

```bash
igo doctor .
igo ai routes .
igo ai schemas .
```

### 从源码运行

```bash
git clone https://github.com/leebo/igo.git
cd igo
go test ./...
go run ./cmd/igo doctor .
go run ./cmd/igo ai context ./examples/full --format json
```

## 快速开始

新建 `main.go`：

```go
package main

import igo "github.com/leebo/igo"

type CreateUserRequest struct {
	Name  string `json:"name" validate:"required|min:2|max:50"`
	Email string `json:"email" validate:"required|email"`
}

type UserResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	app := igo.Simple()

	app.RegisterSchema(UserResponse{})

	app.Get("/health", func(c *igo.Context) {
		c.Success(igo.H{"status": "ok"})
	})

	app.Post("/users", func(c *igo.Context) {
		req, ok := igo.BindAndValidate[CreateUserRequest](c)
		if !ok {
			return
		}

		c.Created(UserResponse{
			ID:    1,
			Name:  req.Name,
			Email: req.Email,
		})
	})

	app.RegisterAIRoutes()
	app.Run(":8080")
}
```

运行：

```bash
go run .
```

访问：

```bash
curl http://localhost:8080/health
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada","email":"ada@example.com"}'
curl http://localhost:8080/_ai/routes
curl http://localhost:8080/_ai/schemas
curl http://localhost:8080/_ai/openapi
```

## 常用写法

### JSON body 绑定和校验

```go
req, ok := igo.BindAndValidate[CreateUserRequest](c)
if !ok {
	return
}
```

绑定失败自动返回 400，校验失败自动返回 422。

### Query 参数绑定和校验

```go
type ListQuery struct {
	Page  int    `json:"page" validate:"gte:1"`
	Size  int    `json:"size" validate:"lte:100"`
	Order string `json:"order" validate:"enum:asc,desc"`
}

q, ok := igo.BindQueryAndValidate[ListQuery](c)
if !ok {
	return
}
```

### Path 参数绑定和校验

```go
type UserPath struct {
	ID int64 `json:"id" validate:"required|gte:1"`
}

p, ok := igo.BindPathAndValidate[UserPath](c)
if !ok {
	return
}
```

### 错误处理

```go
user, err := service.GetUser(c.Request.Context(), id)
if err != nil {
	c.NotFoundWrap(err, "user not found")
	return
}

if err := service.Save(c.Request.Context(), user); err != nil {
	c.InternalErrorWrap(err, "failed to save user", map[string]any{
		"user_id": id,
	})
	return
}
```

错误响应保持统一结构，并保留调用链/上下文，方便 AI 和日志系统定位。

### 路由组和中间件

```go
app.Group("/api/v1", func(v1 *igo.App) {
	v1.Get("/users", listUsers)
	v1.Post("/users", createUser)
}, authMiddleware)
```

组中间件应放在 `Group` 的第三个参数里，不建议在 group closure 内调用 `Use`。

## AI CLI

igo 提供一组离线命令，让 AI 编程工具不用扫描完整代码库也能拿到关键上下文。

```bash
igo ai context . --format md
igo ai context . --format json
igo ai routes .
igo ai schemas .
igo ai errors
igo ai openapi .
igo ai prompt . GET /users/:id
igo ai workflow
igo ai examples
```

如果未安装 CLI，也可以从源码运行：

```bash
go run ./cmd/igo ai routes ./examples/full
go run ./cmd/igo ai schemas ./examples/full
go run ./cmd/igo ai openapi ./examples/full
```

## 运行时 AI 端点

在应用中启用：

```go
app.RegisterAIRoutes()
```

可用端点：

| Endpoint | 说明 |
|---|---|
| `/_ai/routes` | 当前 App 的路由、handler、参数、响应、中间件 |
| `/_ai/schemas` | 当前 App 的请求/响应 DTO Schema |
| `/_ai/errors` | 框架错误码、HTTP 状态码、推荐 helper |
| `/_ai/info` | 框架和当前 App 概览 |
| `/_ai/openapi` | OpenAPI 3.0 JSON |
| `/_ai/conventions` | AI 编码约定和工作流 |
| `/_ai/middlewares` | 全局/路由中间件名称和注册顺序 |

这些端点和 CLI 使用同一套元数据模型，避免“离线文档”和“运行时状态”不一致。

## 与其他 Go Web 框架的区别

| 维度 | Gin / Echo / Fiber / Chi 等常见框架 | igo |
|---|---|---|
| 核心目标 | 高性能、成熟生态、通用 Web 开发 | AI 辅助编程准确率、低上下文成本、稳定元数据 |
| Handler 体验 | API 成熟，但风格和项目约定差异大 | 统一 `func(*igo.Context)`，低样板 |
| 参数绑定 | 通常依赖框架 binder 或第三方 validator | 内置 `BindAndValidate`、`BindQueryAndValidate`、`BindPathAndValidate` |
| 路由元数据 | 多数情况下需要读源码或维护 Swagger 注释 | 注册路由时自动收集到 App registry |
| Schema 输出 | 通常需要手写注释或额外生成器 | `TypeSchema` 自动提取字段、JSON 名、validate 规则 |
| OpenAPI | 常见方案是注释驱动，容易和代码漂移 | 基于 route/schema 元数据生成，包含 components |
| 错误码 | 项目自行约定，AI 需要读实现 | `igo ai errors` 和 `/_ai/errors` 可直接查询 |
| AI 上下文 | AI 往往需要搜索大量文件 | `igo ai context/routes/schemas/prompt` 提供短上下文 |
| 运行时自省 | 通常没有标准端点 | `/_ai/*` 标准化暴露 |
| 多 App 测试隔离 | 取决于项目实现 | App 自有 route/schema registry |
| 生态成熟度 | Gin/Echo/Fiber/Chi 更成熟 | igo 更轻，偏 AI-first 和可复制模式 |

### 什么时候选 igo

- 你希望 AI 能快速理解项目路由、DTO、错误码和约定。
- 你经常让 Codex、Claude Code、Cursor 这类工具改接口、补测试、生成 OpenAPI。
- 你希望 handler 模板短、绑定/校验/错误处理有统一模式。
- 你想把运行时元数据暴露给调试工具或内部平台。

### 什么时候选 Gin/Echo/Fiber/Chi

- 你最看重成熟生态、插件数量或团队既有经验。
- 你需要高度定制的 HTTP 路由行为或中间件体系。
- 项目已经深度绑定某个框架，迁移成本高。

igo 不排斥这些框架。仓库提供了 Gin adapter，可用于渐进迁移或混合架构。

## AI 方面的优势

igo 的 AI 优势不是“让模型更聪明”，而是减少模型必须猜的东西：

- **上下文短**：`igo ai context` 输出框架规则、路由和 Schema 摘要，减少 token 消耗。
- **事实稳定**：`RouteConfig` 和 `TypeSchema` 是结构化 JSON，AI 不必从自然语言 README 猜接口。
- **定位精确**：路由元数据包含 `handlerName`、`filePath`、`lineNumber`。
- **校验可见**：字段级 `validate` tag 会解析为 `required`、`enum`、`gte/lte` 等机器可读信息。
- **错误可查**：错误码和推荐 helper 通过 CLI/HTTP 暴露。
- **OpenAPI 可生成**：直接从路由和 Schema 输出 OpenAPI，减少注释漂移。
- **运行时可对齐**：AI 可以比较 `igo ai routes` 和 `/_ai/routes`，确认代码与运行状态。
- **Prompt 可聚焦**：`igo ai prompt . METHOD PATH` 能为单个 handler 生成改造上下文。

## 示例

| 目录 | 说明 |
|---|---|
| `examples/simple` | 基础路由和响应 |
| `examples/pagination` | query 绑定、分页、排序白名单 |
| `examples/ai_runtime` | `/_ai/*`、path validation、cookie、redirect、static files |
| `examples/testing` | handler 测试模式 |
| `examples/upload` | 文件上传/下载 |
| `examples/websocket` | WebSocket |
| `examples/auth_e2e` | JWT 认证端到端示例 |
| `examples/gin_adapter_demo` | Gin 兼容和迁移示例 |
| `examples/full` | 更完整的分层项目示例 |

## 开发和验证

```bash
go test ./...
go run ./cmd/igo doctor .
go run ./cmd/igo ai context ./examples/full --format json
go run ./cmd/igo ai routes ./examples/full
go run ./cmd/igo ai schemas ./examples/full
go run ./cmd/igo ai errors
go run ./cmd/igo ai openapi ./examples/full
```

## 文档入口

- `AGENTS.md`：给 Codex/Claude Code 的短规则
- `CLAUDE.md`：框架约定和常用模板
- `docs/ai.md`：AI 元数据契约
- `llms.txt` / `llms-full.txt`：LLM 工具入口
- `context7.json`：Context7 风格索引
