package metadata

// ParamMeta 描述 API 参数
type ParamMeta struct {
	Name        string   `json:"name"`
	In          string   `json:"in"`           // path, query, header, cookie
	Type        string   `json:"type"`         // string, int, bool, object
	Required    bool     `json:"required"`
	Description string   `json:"description"`
	Validation  []string `json:"validation,omitempty"`
	Default     string   `json:"default,omitempty"`
}

// BodyMeta 描述请求体
type BodyMeta struct {
	ContentType string      `json:"contentType"` // application/json
	Type        string      `json:"type"`         // 请求体类型名称
	Example     interface{} `json:"example,omitempty"`
	Description string      `json:"description,omitempty"`
}

// ResponseMeta 描述 API 响应
type ResponseMeta struct {
	StatusCode  int         `json:"statusCode"`
	Description string      `json:"description"`
	Type        string      `json:"type"`         // 响应类型名称
	Example     interface{} `json:"example,omitempty"`
}

// RouteMeta 描述路由的 AI 可读元数据
type RouteMeta struct {
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	Summary     string          `json:"summary"`      // 一句话描述
	Description string          `json:"description"`  // 详细描述
	HandlerName string          `json:"handlerName"`  // 处理函数名
	FilePath    string          `json:"filePath"`     // 文件路径
	LineNumber  int             `json:"lineNumber"`   // 行号
	Parameters  []ParamMeta     `json:"parameters"`
	RequestBody *BodyMeta       `json:"requestBody,omitempty"`
	Responses   []ResponseMeta  `json:"responses"`
	Tags        []string        `json:"tags"`
	Deprecated  bool            `json:"deprecated"`
	AIHints     []string        `json:"aiHints,omitempty"` // AI 调试提示
}

// HandlerMeta 描述处理函数的元数据
type HandlerMeta struct {
	Name           string   `json:"name"`
	FilePath       string   `json:"filePath"`
	LineNumber     int      `json:"lineNumber"`
	InputType      string   `json:"inputType"`       // BindJSON 目标类型
	OutputType     string   `json:"outputType"`      // 响应类型
	MiddlewareChain []string `json:"middlewareChain"`
	ValidationUsed bool     `json:"validationUsed"`
	BindMethods    []string `json:"bindMethods"` // BindJSON, Query, Param
}

// Annotation 解析后的注解
type Annotation struct {
	Key   string
	Value string
	Raw   string
}

// Registry 全局元数据注册表
type Registry struct {
	routes   map[string]*RouteMeta  // key: "GET /users/:id"
	handlers map[string]*HandlerMeta // key: "handlers.user.GetUser"
}

// NewRegistry 创建新的注册表
func NewRegistry() *Registry {
	return &Registry{
		routes:   make(map[string]*RouteMeta),
		handlers: make(map[string]*HandlerMeta),
	}
}

// routeKey 生成路由的唯一 key
func routeKey(method, path string) string {
	return method + " " + path
}

// RegisterRoute 注册路由元数据
func (r *Registry) RegisterRoute(meta *RouteMeta) {
	r.routes[routeKey(meta.Method, meta.Path)] = meta
}

// GetRoute 获取路由元数据
func (r *Registry) GetRoute(method, path string) *RouteMeta {
	return r.routes[routeKey(method, path)]
}

// ListRoutes 列出所有路由元数据
func (r *Registry) ListRoutes() []*RouteMeta {
	result := make([]*RouteMeta, 0, len(r.routes))
	for _, meta := range r.routes {
		result = append(result, meta)
	}
	return result
}

// RegisterHandler 注册处理器元数据
func (r *Registry) RegisterHandler(meta *HandlerMeta) {
	r.handlers[meta.Name] = meta
}

// GetHandler 获取处理器元数据
func (r *Registry) GetHandler(name string) *HandlerMeta {
	return r.handlers[name]
}

// ListHandlers 列出所有处理器元数据
func (r *Registry) ListHandlers() []*HandlerMeta {
	result := make([]*HandlerMeta, 0, len(r.handlers))
	for _, meta := range r.handlers {
		result = append(result, meta)
	}
	return result
}
