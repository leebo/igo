package route

import (
	"strings"
)

// InferenceOptions 推断选项
type InferenceOptions struct {
	EnableSummary     bool // 从函数名推断 Summary
	EnableTags        bool // 从路径前缀推断 Tags
	EnableParams      bool // 从 :param 模式提取参数
	EnableHandler     bool // 从函数位置获取 HandlerName, FilePath, LineNumber
	EnableRequestBody bool // 分析 BindJSON 调用 (需要运行时)
	EnableResponse    bool // 分析 Success/Created 调用 (需要运行时)
}

// DefaultInferenceOptions 默认推断选项
var DefaultInferenceOptions = &InferenceOptions{
	EnableSummary:     true,
	EnableTags:        true,
	EnableParams:      true,
	EnableHandler:     true,
	EnableRequestBody: false,
	EnableResponse:    false,
}

// InferenceEngine 推断引擎
type InferenceEngine struct {
	opts *InferenceOptions
}

// NewInferenceEngine 创建推断引擎
func NewInferenceEngine(opts *InferenceOptions) *InferenceEngine {
	if opts == nil {
		opts = DefaultInferenceOptions
	}
	return &InferenceEngine{opts: opts}
}

// InferFromFunction 从函数推断元数据
func (ie *InferenceEngine) InferFromFunction(handlerName, method, path string) *RouteConfig {
	cfg := &RouteConfig{
		Method: method,
		Path:   path,
	}

	// 从路径推断 Tags
	if ie.opts.EnableTags {
		cfg.Tags = ie.inferTagsFromPath(path)
	}

	// 从路径模式提取 Parameters
	if ie.opts.EnableParams {
		cfg.Params = ie.inferParamsFromPath(path)
	}

	// 从函数名推断 Summary
	if ie.opts.EnableSummary {
		cfg.Summary = ie.inferSummaryFromName(handlerName, method, path)
	}

	return cfg
}

// inferTagsFromPath 从路径推断 Tags
// /api/v1/users -> ["users"]
// /api/v1/users/:id -> ["users"]
func (ie *InferenceEngine) inferTagsFromPath(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return nil
	}

	// 最后一个非参数段作为 tag
	for i := len(parts) - 1; i >= 0; i-- {
		if !strings.HasPrefix(parts[i], ":") && parts[i] != "" {
			return []string{parts[i]}
		}
	}
	return nil
}

// inferParamsFromPath 从路径模式提取参数
// /users/:id/posts/:postId -> [{Name:"id", In:"path"}, {Name:"postId", In:"path"}]
func (ie *InferenceEngine) inferParamsFromPath(path string) []ParamDefinition {
	params := make([]ParamDefinition, 0)
	parts := strings.Split(path, "/")

	for _, part := range parts {
		if strings.HasPrefix(part, ":") || strings.HasPrefix(part, "*") {
			name := strings.TrimLeft(part, ":*")
			params = append(params, ParamDefinition{
				Name:     name,
				In:       "path",
				Type:     ie.inferTypeFromName(name),
				Required: true,
			})
		}
	}
	return params
}

// inferTypeFromName 从参数名推断类型
func (ie *InferenceEngine) inferTypeFromName(name string) string {
	lower := strings.ToLower(name)

	// 常见命名模式
	switch {
	case strings.HasSuffix(lower, "id") || strings.HasSuffix(lower, "ids"):
		return "int"
	case strings.HasSuffix(lower, "page"), strings.HasSuffix(lower, "size"),
		strings.HasSuffix(lower, "limit"), strings.HasSuffix(lower, "offset"),
		strings.HasSuffix(lower, "count"), strings.HasSuffix(lower, "total"):
		return "int"
	case strings.HasSuffix(lower, "name"), strings.HasSuffix(lower, "email"),
		strings.HasSuffix(lower, "username"), strings.HasSuffix(lower, "password"),
		strings.HasSuffix(lower, "title"), strings.HasSuffix(lower, "description"),
		strings.HasSuffix(lower, "content"), strings.HasSuffix(lower, "body"):
		return "string"
	case strings.HasSuffix(lower, "bool"), strings.HasSuffix(lower, "enabled"),
		strings.HasSuffix(lower, "active"), strings.HasSuffix(lower, "visible"):
		return "bool"
	case strings.HasSuffix(lower, "price"), strings.HasSuffix(lower, "amount"),
		strings.HasSuffix(lower, "balance"), strings.HasSuffix(lower, "lat"), strings.HasSuffix(lower, "lng"):
		return "float"
	default:
		return "string"
	}
}

// inferSummaryFromName 从函数名推断 Summary
// getUser -> "Get user by ID"
// listUsers -> "List users"
// createUser -> "Create user"
// updateUser -> "Update user"
// deleteUser -> "Delete user"
func (ie *InferenceEngine) inferSummaryFromName(handlerName, method, path string) string {
	// 清理函数名（去掉包路径前缀）
	name := handlerName
	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[idx+1:]
	}
	name = strings.TrimPrefix(name, "handle")
	name = strings.TrimPrefix(name, "Handle")
	name = strings.TrimPrefix(name, "do")
	name = strings.TrimPrefix(name, "Do")

	// 转换驼峰为空格分隔
	words := splitCamelCase(name)

	if len(words) == 0 {
		return ""
	}

	// 根据 HTTP 方法和路径决定语态
	verb := getVerbForMethod(method, words[0])

	if len(words) == 1 {
		resource := resourceNameFromPath(path)
		if resource == "" {
			resource = singularize(words[0])
		}
		switch method {
		case "GET":
			if strings.Contains(path, ":") {
				return "Get " + singularize(resource) + " by ID"
			}
			if strings.EqualFold(words[0], "list") || strings.HasSuffix(resource, "s") {
				return "List " + resource
			}
			return "Get " + resource
		case "POST":
			return "Create " + singularize(resource)
		case "PUT", "PATCH":
			return "Update " + singularize(resource)
		case "DELETE":
			return "Delete " + singularize(resource)
		default:
			return verb + " " + resource
		}
	}

	// 根据路径参数决定后缀
	hasPathParams := strings.Contains(path, ":")
	if hasPathParams {
		return verb + " " + strings.Join(words[1:], " ") + " by ID"
	}

	return verb + " " + strings.Join(words[1:], " ")
}

func resourceNameFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if part == "" || strings.HasPrefix(part, ":") || strings.HasPrefix(part, "*") {
			continue
		}
		return strings.ReplaceAll(part, "-", " ")
	}
	return ""
}

// splitCamelCase 拆分驼峰命名
func splitCamelCase(s string) []string {
	if s == "" {
		return nil
	}
	var words []string
	var current strings.Builder

	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			current.WriteRune(r + 32)
		} else {
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// getVerbForMethod 根据 HTTP 方法获取动词
func getVerbForMethod(method, firstWord string) string {
	switch method {
	case "GET":
		if strings.HasSuffix(strings.ToLower(firstWord), "list") {
			return "List"
		}
		return "Get"
	case "POST":
		return "Create"
	case "PUT", "PATCH":
		return "Update"
	case "DELETE":
		return "Delete"
	default:
		return firstWord
	}
}

// singularize 单数化 (简化版本)
func singularize(word string) string {
	if word == "" {
		return word
	}
	lower := strings.ToLower(word)
	if strings.HasSuffix(lower, "ies") && len(word) > 3 {
		return strings.TrimSuffix(word, "ies") + "y"
	}
	if strings.HasSuffix(lower, "es") && len(word) > 2 {
		return strings.TrimSuffix(word, "es")
	}
	if strings.HasSuffix(lower, "s") && len(word) > 1 {
		return strings.TrimSuffix(word, "s")
	}
	return word
}

// GlobalInferenceEngine 全局推断引擎实例
var GlobalInferenceEngine = NewInferenceEngine(DefaultInferenceOptions)

// InferFromFunction 全局推断函数
func InferFromFunction(handlerName, method, path string) *RouteConfig {
	return GlobalInferenceEngine.InferFromFunction(handlerName, method, path)
}

// MergeWithInference 合并推断配置
// explicit 显式配置优先于 inferred 推断配置
func MergeWithInference(inferred, explicit *RouteConfig) *RouteConfig {
	if explicit == nil {
		return inferred
	}
	if inferred == nil {
		return explicit
	}

	result := *inferred

	// 显式值优先
	if explicit.Summary != "" {
		result.Summary = explicit.Summary
	}
	if explicit.Description != "" {
		result.Description = explicit.Description
	}
	if explicit.HandlerName != "" {
		result.HandlerName = explicit.HandlerName
	}
	if explicit.FilePath != "" {
		result.FilePath = explicit.FilePath
	}
	if explicit.LineNumber != 0 {
		result.LineNumber = explicit.LineNumber
	}

	// Tags 合并（显式值覆盖推断值）
	if len(explicit.Tags) > 0 {
		result.Tags = explicit.Tags
	}

	// Params 合并（只有推断的）
	if len(explicit.Params) == 0 {
		result.Params = inferred.Params
	}

	// RequestBody, Responses 使用显式值
	if explicit.RequestBody != nil {
		result.RequestBody = explicit.RequestBody
	}
	if len(explicit.Responses) > 0 {
		result.Responses = explicit.Responses
	}

	// AIHints 合并
	if len(explicit.AIHints) > 0 {
		result.AIHints = explicit.AIHints
	}

	// Middlewares 合并
	if len(explicit.Middlewares) > 0 {
		result.Middlewares = explicit.Middlewares
	}

	// Deprecated
	if explicit.Deprecated {
		result.Deprecated = true
	}

	return &result
}
