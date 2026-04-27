package agent

import (
	"fmt"
	"strings"

	"github.com/igo/igo/ai/metadata"
	"github.com/igo/igo/core"
)

// FrameworkConventions igo 框架约定
const FrameworkConventions = `igo Framework Conventions:

路由注册:
- app.Get("/path", handler)       // GET 请求
- app.Post("/path", handler)      // POST 请求
- app.Put("/path", handler)       // PUT 请求
- app.Delete("/path", handler)    // DELETE 请求
- app.Patch("/path", handler)     // PATCH 请求

Handler 签名:
- func(c *igo.Context)
- c.Success(data)                 // 返回 200 和 {data: ...}
- c.Created(data)                 // 返回 201 和 {data: ...}
- c.NoContent()                   // 返回 204
- c.BadRequest(message)           // 返回 400
- c.NotFound(message)             // 返回 404
- c.Unauthorized(message)         // 返回 401
- c.Forbidden(message)            // 返回 403
- c.InternalError(message)        // 返回 500

参数绑定:
- c.Query("name")                // 获取查询参数
- c.QueryInt("name", default)    // 获取整数查询参数
- c.Param("id")                  // 获取路径参数
- c.BindJSON(&struct)            // 绑定 JSON body

struct 验证:
- validate:"required"             // 必填
- validate:"email"                // 邮箱格式
- validate:"min:2"                // 最小长度
- validate:"max:100"              // 最大长度

中间件注册:
- app.Use(middleware)             // 全局中间件
- app.Get("/path", handler, middleware) // 路由级别

响应格式:
- 成功: {"data": {...}}
- 错误: {"error": {"code": "...", "message": "..."}}
`

// GenerateProjectPrompt 生成项目提示词
func GenerateProjectPrompt(routes []*metadata.RouteMeta) string {
	var sb strings.Builder

	sb.WriteString("# igo 项目上下文\n\n")
	sb.WriteString(FrameworkConventions)
	sb.WriteString("\n## 当前项目路由:\n\n")

	for _, route := range routes {
		sb.WriteString(fmt.Sprintf("- %s %s", route.Method, route.Path))
		if route.Summary != "" {
			sb.WriteString(fmt.Sprintf(" - %s", route.Summary))
		}
		sb.WriteString("\n")

		// 添加参数信息
		for _, param := range route.Parameters {
			sb.WriteString(fmt.Sprintf("  - %s (%s): %s\n", param.Name, param.In, param.Description))
		}

		// 添加响应信息
		for _, resp := range route.Responses {
			sb.WriteString(fmt.Sprintf("  - %d: %s\n", resp.StatusCode, resp.Description))
		}
	}

	return sb.String()
}

// GenerateHandlerPrompt 生成处理函数的提示词
func GenerateHandlerPrompt(route *metadata.RouteMeta) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Handler: %s %s\n\n", route.Method, route.Path))

	if route.Summary != "" {
		sb.WriteString(fmt.Sprintf("## 概要\n%s\n\n", route.Summary))
	}

	if route.Description != "" {
		sb.WriteString(fmt.Sprintf("## 详细说明\n%s\n\n", route.Description))
	}

	if len(route.Parameters) > 0 {
		sb.WriteString("## 参数\n")
		for _, param := range route.Parameters {
			required := ""
			if param.Required {
				required = " (必填)"
			}
			sb.WriteString(fmt.Sprintf("- %s (%s)%s: %s [%s]\n",
				param.Name, param.In, required, param.Description, param.Type))
		}
		sb.WriteString("\n")
	}

	if len(route.Responses) > 0 {
		sb.WriteString("## 响应\n")
		for _, resp := range route.Responses {
			sb.WriteString(fmt.Sprintf("- %d: %s\n", resp.StatusCode, resp.Description))
		}
		sb.WriteString("\n")
	}

	if len(route.AIHints) > 0 {
		sb.WriteString("## AI 调试提示\n")
		for _, hint := range route.AIHints {
			sb.WriteString(fmt.Sprintf("- %s\n", hint))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// GenerateCodePrompt 生成代码生成的提示词
func GenerateCodePrompt(method, path string) string {
	return fmt.Sprintf(`生成 igo 框架的 %s %s 处理函数:

要求:
1. 使用 func(c *igo.Context) 签名
2. 使用 c.Success() 返回成功响应
3. 使用 c.BadRequest() 返回错误响应
4. 进行必要的参数验证
5. 返回结构化的响应

示例格式:
func Handler(c *igo.Context) {
    var req struct {
        // 字段定义
    }
    if err := c.BindJSON(&req); err != nil {
        c.BadRequest("Invalid request body")
        return
    }
    // 业务逻辑
    c.Success(result)
}
`, method, path)
}

// GenerateDebugPrompt 生成调试提示词
func GenerateDebugPrompt(errMsg string, route *metadata.RouteMeta) string {
	var sb strings.Builder

	sb.WriteString("# 调试提示\n\n")
	sb.WriteString(fmt.Sprintf("错误信息: %s\n\n", errMsg))

	if route != nil {
		sb.WriteString(fmt.Sprintf("当前处理: %s %s\n", route.Method, route.Path))

		if len(route.AIHints) > 0 {
			sb.WriteString("\n## 相关提示:\n")
			for _, hint := range route.AIHints {
				sb.WriteString(fmt.Sprintf("- %s\n", hint))
			}
		}
	}

	sb.WriteString("\n## 常见错误排查:\n")
	sb.WriteString("1. 检查请求 Content-Type 是否为 application/json\n")
	sb.WriteString("2. 检查 JSON body 格式是否正确\n")
	sb.WriteString("3. 检查必填字段是否提供\n")
	sb.WriteString("4. 检查参数类型是否匹配\n")

	return sb.String()
}

// BuildClaudeCodeContext 为 Claude Code 构建项目上下文
func BuildClaudeCodeContext(app *core.App) string {
	if app == nil {
		return FrameworkConventions
	}

	routes := []*metadata.RouteMeta{}

	return GenerateProjectPrompt(routes)
}
