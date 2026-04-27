package route

// RouteOption 路由配置选项
type RouteOption func(*RouteConfig)

// RouteConfig 路由完整配置
type RouteConfig struct {
	Method       string
	Path         string
	Summary      string
	Description  string
	HandlerName  string
	FilePath     string
	LineNumber   int
	Params       []ParamDefinition
	RequestBody  *RequestBodyDefinition
	Responses    []ResponseDefinition
	Tags         []string
	AIHints      []string
	Deprecated   bool
	Middlewares  []string
}

// ParamDefinition 参数定义
type ParamDefinition struct {
	Name        string
	In          string // "path", "query", "header", "cookie"
	Type        string // "string", "int", "bool", "array", "object"
	Required    bool
	Description string
	Validation  []string
	Default     string
	Example     string
}

// RequestBodyDefinition 请求体定义
type RequestBodyDefinition struct {
	ContentType string // "application/json"
	TypeName    string // Go 类型名
	Description string
	Required    bool
	Example     any
}

// ResponseDefinition 响应定义
type ResponseDefinition struct {
	StatusCode  int
	Description string
	TypeName    string
	Example     any
}

// WithSummary 设置路由摘要
func WithSummary(s string) RouteOption {
	return func(cfg *RouteConfig) {
		cfg.Summary = s
	}
}

// WithDescription 设置路由详细描述
func WithDescription(s string) RouteOption {
	return func(cfg *RouteConfig) {
		cfg.Description = s
	}
}

// WithParam 添加参数定义
func WithParam(name, in, typ string, required bool, description string) RouteOption {
	return func(cfg *RouteConfig) {
		cfg.Params = append(cfg.Params, ParamDefinition{
			Name:        name,
			In:          in,
			Type:        typ,
			Required:    required,
			Description: description,
		})
	}
}

// WithPathParam 添加路径参数
func WithPathParam(name, typ, description string) RouteOption {
	return WithParam(name, "path", typ, true, description)
}

// WithQueryParam 添加查询参数
func WithQueryParam(name, typ, description string, required bool) RouteOption {
	return WithParam(name, "query", typ, required, description)
}

// WithHeaderParam 添加 header 参数
func WithHeaderParam(name, typ, description string, required bool) RouteOption {
	return WithParam(name, "header", typ, required, description)
}

// WithRequestBody 设置请求体
func WithRequestBody(contentType, typeName string) RouteOption {
	return func(cfg *RouteConfig) {
		cfg.RequestBody = &RequestBodyDefinition{
			ContentType: contentType,
			TypeName:    typeName,
			Required:    true,
		}
	}
}

// WithRequestBodyOptional 设置可选请求体
func WithRequestBodyOptional(contentType, typeName, description string) RouteOption {
	return func(cfg *RouteConfig) {
		cfg.RequestBody = &RequestBodyDefinition{
			ContentType: contentType,
			TypeName:    typeName,
			Description: description,
			Required:    false,
		}
	}
}

// WithResponse 添加响应定义
func WithResponse(statusCode int, description, typeName string) RouteOption {
	return func(cfg *RouteConfig) {
		cfg.Responses = append(cfg.Responses, ResponseDefinition{
			StatusCode:  statusCode,
			Description: description,
			TypeName:    typeName,
		})
	}
}

// WithSuccessResponse 添加成功响应 (200/201)
func WithSuccessResponse(description, typeName string) RouteOption {
	return WithResponse(200, description, typeName)
}

// WithCreatedResponse 添加创建成功响应 (201)
func WithCreatedResponse(description, typeName string) RouteOption {
	return WithResponse(201, description, typeName)
}

// WithTags 设置标签
func WithTags(tags ...string) RouteOption {
	return func(cfg *RouteConfig) {
		cfg.Tags = tags
	}
}

// WithAIHint 添加 AI 调试提示
func WithAIHint(hint string) RouteOption {
	return func(cfg *RouteConfig) {
		cfg.AIHints = append(cfg.AIHints, hint)
	}
}

// WithDeprecated 标记为废弃
func WithDeprecated() RouteOption {
	return func(cfg *RouteConfig) {
		cfg.Deprecated = true
	}
}

// WithMiddleware 添加中间件名称
func WithMiddleware(names ...string) RouteOption {
	return func(cfg *RouteConfig) {
		cfg.Middlewares = append(cfg.Middlewares, names...)
	}
}

// WithHandlerInfo 设置处理函数信息
func WithHandlerInfo(name, filePath string, lineNumber int) RouteOption {
	return func(cfg *RouteConfig) {
		cfg.HandlerName = name
		cfg.FilePath = filePath
		cfg.LineNumber = lineNumber
	}
}

// ApplyOptions 应用选项到配置
func ApplyOptions(opts ...RouteOption) *RouteConfig {
	cfg := &RouteConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// Merge 合并两个配置
func (c *RouteConfig) Merge(other *RouteConfig) *RouteConfig {
	if other == nil {
		return c
	}

	result := *c

	if other.Summary != "" {
		result.Summary = other.Summary
	}
	if other.Description != "" {
		result.Description = other.Description
	}
	if other.HandlerName != "" {
		result.HandlerName = other.HandlerName
	}
	if other.FilePath != "" {
		result.FilePath = other.FilePath
	}
	if other.LineNumber != 0 {
		result.LineNumber = other.LineNumber
	}

	result.Params = append(result.Params, other.Params...)
	result.Responses = append(result.Responses, other.Responses...)
	result.Tags = append(result.Tags, other.Tags...)
	result.AIHints = append(result.AIHints, other.AIHints...)
	result.Middlewares = append(result.Middlewares, other.Middlewares...)

	if other.RequestBody != nil {
		result.RequestBody = other.RequestBody
	}

	return &result
}
