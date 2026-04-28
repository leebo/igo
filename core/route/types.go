// Package route 提供路由元数据的统一类型 + 推断引擎 + 注册表。
//
// 路由注册时由 router.addRoute 自动调用 InferenceEngine 写入当前 App 的 Registry，
// 用户/AI 通过 app.Routes() 或 /_ai/routes 端点查询全部路由元数据。
package route

// RouteConfig 路由完整配置（统一元数据模型）
type RouteConfig struct {
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Summary     string                 `json:"summary,omitempty"`
	Description string                 `json:"description,omitempty"`
	HandlerName string                 `json:"handlerName,omitempty"`
	FilePath    string                 `json:"filePath,omitempty"`
	LineNumber  int                    `json:"lineNumber,omitempty"`
	Params      []ParamDefinition      `json:"params,omitempty"`
	RequestBody *RequestBodyDefinition `json:"requestBody,omitempty"`
	Responses   []ResponseDefinition   `json:"responses,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	AIHints     []string               `json:"aiHints,omitempty"`
	Deprecated  bool                   `json:"deprecated,omitempty"`
	Middlewares []string               `json:"middlewares,omitempty"`
}

// ParamDefinition 参数定义
type ParamDefinition struct {
	Name        string   `json:"name"`
	In          string   `json:"in"`   // "path", "query", "header", "cookie"
	Type        string   `json:"type"` // "string", "int", "bool", "array", "object"
	Required    bool     `json:"required"`
	Description string   `json:"description,omitempty"`
	Validation  []string `json:"validation,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Min         string   `json:"min,omitempty"`
	Max         string   `json:"max,omitempty"`
	GTE         string   `json:"gte,omitempty"`
	LTE         string   `json:"lte,omitempty"`
	Len         string   `json:"len,omitempty"`
	Default     string   `json:"default,omitempty"`
	Example     string   `json:"example,omitempty"`
}

// RequestBodyDefinition 请求体定义
type RequestBodyDefinition struct {
	ContentType string `json:"contentType"` // 一般为 "application/json"
	TypeName    string `json:"typeName"`    // Go 类型名
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Example     any    `json:"example,omitempty"`
}

// ResponseDefinition 响应定义
type ResponseDefinition struct {
	StatusCode  int    `json:"statusCode"`
	Description string `json:"description,omitempty"`
	TypeName    string `json:"typeName,omitempty"`
	Example     any    `json:"example,omitempty"`
}
