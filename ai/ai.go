// Package ai 提供 AI 开发工具支持
//
// 包含功能:
//   - 元数据注册与管理
//   - OpenAPI Schema 生成
//   - AI 友好的错误提示
//   - Claude Code/Codex 提示词生成
package ai

import (
	"github.com/igo/igo/ai/agent"
	"github.com/igo/igo/ai/metadata"
	"github.com/igo/igo/ai/prompt"
	"github.com/igo/igo/ai/schema"
)

// Registry 元数据注册表
type Registry = metadata.Registry

// NewRegistry 创建新的注册表
var NewRegistry = metadata.NewRegistry

// RouteMeta 路由元数据
type RouteMeta = metadata.RouteMeta

// ParamMeta 参数元数据
type ParamMeta = metadata.ParamMeta

// ParseHandlerDoc 解析处理函数文档
var ParseHandlerDoc = metadata.ParseHandlerDoc

// GenerateOpenAPI 生成 OpenAPI 规范
func GenerateOpenAPI(registry *Registry) *schema.OpenAPISpec {
	gen := schema.NewGenerator(registry)
	return gen.Generate()
}

// GenerateProjectPrompt 生成项目提示词
func GenerateProjectPrompt(routes []*RouteMeta) string {
	return agent.GenerateProjectPrompt(routes)
}

// GenerateHandlerPrompt 生成处理函数提示词
func GenerateHandlerPrompt(route *RouteMeta) string {
	return agent.GenerateHandlerPrompt(route)
}

// FormatError 格式化 AI 错误响应
func FormatError(statusCode int, code, message string, ctx *prompt.ErrorContext) prompt.AIErrorResponse {
	return prompt.FormatError(statusCode, code, message, ctx)
}
