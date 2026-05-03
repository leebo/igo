package schema

// OpenAPI 3.0 类型定义

type OpenAPISpec struct {
	OpenAPI    string               `json:"openapi"`
	Info       *Info                `json:"info"`
	Paths      map[string]*PathItem `json:"paths"`
	Servers    []*Server            `json:"servers,omitempty"`
	Components *Components          `json:"components,omitempty"`
}

type Info struct {
	Title       string   `json:"title"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Contact     *Contact `json:"contact,omitempty"`
}

type Contact struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

type PathItem struct {
	Ref     string     `json:"$ref,omitempty"`
	GET     *Operation `json:"get,omitempty"`
	POST    *Operation `json:"post,omitempty"`
	PUT     *Operation `json:"put,omitempty"`
	DELETE  *Operation `json:"delete,omitempty"`
	PATCH   *Operation `json:"patch,omitempty"`
	OPTIONS *Operation `json:"options,omitempty"`
	HEAD    *Operation `json:"head,omitempty"`
}

type Operation struct {
	Tags        []string                 `json:"tags,omitempty"`
	Summary     string                   `json:"summary,omitempty"`
	Description string                   `json:"description,omitempty"`
	OperationID string                   `json:"operationId,omitempty"`
	Parameters  []*Parameter             `json:"parameters,omitempty"`
	RequestBody *RequestBody             `json:"requestBody,omitempty"`
	Responses   map[string]*Response     `json:"responses"`
	Security    []map[string][]string    `json:"security,omitempty"`
	Deprecated  bool                     `json:"deprecated,omitempty"`
}

type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"` // path, query, header, cookie
	Required    bool    `json:"required"`
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

type RequestBody struct {
	Description string                `json:"description,omitempty"`
	Required    bool                  `json:"required"`
	Content     map[string]*MediaType `json:"content"`
}

type Response struct {
	Description string                `json:"description"`
	Content     map[string]*MediaType `json:"content,omitempty"`
}

type MediaType struct {
	Schema  *Schema `json:"schema,omitempty"`
	Example any     `json:"example,omitempty"`
}

type Schema struct {
	Type        string             `json:"type,omitempty"`
	Format      string             `json:"format,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Description string             `json:"description,omitempty"`
	Example     any                `json:"example,omitempty"`
	Enum        []string           `json:"enum,omitempty"`
	Minimum     any                `json:"minimum,omitempty"`
	Maximum     any                `json:"maximum,omitempty"`
	MinLength   *int               `json:"minLength,omitempty"`
	MaxLength   *int               `json:"maxLength,omitempty"`
	Ref         string             `json:"$ref,omitempty"`
}

// Components 组件定义
type Components struct {
	Schemas         map[string]*Schema         `json:"schemas,omitempty"`
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme OpenAPI 3.0 安全方案。
//
// igo 默认输出一个名为 "bearerAuth" 的 HTTP/Bearer/JWT 方案，凡是路由的
// 中间件链里出现 Auth/JWT/Bearer 字样就会自动挂上 security: [{bearerAuth: []}]
// 让 AI 生成的客户端知道带 Authorization 头。
type SecurityScheme struct {
	Type         string `json:"type"`                   // apiKey | http | oauth2 | openIdConnect
	Scheme       string `json:"scheme,omitempty"`       // bearer | basic
	BearerFormat string `json:"bearerFormat,omitempty"` // JWT
	In           string `json:"in,omitempty"`           // apiKey 时使用
	Name         string `json:"name,omitempty"`         // apiKey 名
	Description  string `json:"description,omitempty"`
}
