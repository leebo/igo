package schema

import (
	"strconv"

	"github.com/igo/igo/ai/metadata"
)

// Generator OpenAPI Schema 生成器
type Generator struct {
	registry *metadata.Registry
	spec    *OpenAPISpec
}

// NewGenerator 创建新的生成器
func NewGenerator(registry *metadata.Registry) *Generator {
	return &Generator{
		registry: registry,
		spec: &OpenAPISpec{
			OpenAPI: "3.0.0",
			Info: &Info{
				Title:       "igo API",
				Version:     "1.0.0",
				Description: "AI-friendly API documentation",
			},
			Paths: make(map[string]*PathItem),
		},
	}
}

// Generate 生成 OpenAPI 规范
func (g *Generator) Generate() *OpenAPISpec {
	routes := g.registry.ListRoutes()
	for _, route := range routes {
		g.addRoute(route)
	}
	return g.spec
}

// addRoute 添加路由到 OpenAPI spec
func (g *Generator) addRoute(route *metadata.RouteMeta) {
	pathItem := g.spec.Paths[route.Path]
	if pathItem == nil {
		pathItem = &PathItem{}
		g.spec.Paths[route.Path] = pathItem
	}

	operation := g.buildOperation(route)
	g.setOperation(pathItem, route.Method, operation)
}

// buildOperation 从 RouteMeta 构建 Operation
func (g *Generator) buildOperation(route *metadata.RouteMeta) *Operation {
	op := &Operation{
		Tags:        route.Tags,
		Summary:     route.Summary,
		Description: route.Description,
		Parameters:  make([]*Parameter, 0),
		Responses:   make(map[string]*Response),
		Deprecated:  route.Deprecated,
	}

	// 添加参数
	for _, param := range route.Parameters {
		op.Parameters = append(op.Parameters, &Parameter{
			Name:        param.Name,
			In:          param.In,
			Required:    param.Required,
			Description: param.Description,
			Schema: &Schema{
				Type: param.Type,
			},
		})
	}

	// 添加请求体
	if route.RequestBody != nil {
		op.RequestBody = &RequestBody{
			Description: route.RequestBody.Description,
			Required:    true,
			Content: map[string]*MediaType{
				route.RequestBody.ContentType: {
					Schema: &Schema{
						Type: route.RequestBody.Type,
					},
				},
			},
		}
	}

	// 添加响应
	for _, resp := range route.Responses {
		respKey := itoa(resp.StatusCode)
		op.Responses[respKey] = &Response{
			Description: resp.Description,
			Content: map[string]*MediaType{
				"application/json": {
					Schema: &Schema{
						Type: resp.Type,
					},
				},
			},
		}
	}

	return op
}

// setOperation 根据 HTTP 方法设置操作
func (g *Generator) setOperation(pathItem *PathItem, method string, operation *Operation) {
	switch method {
	case "GET":
		pathItem.GET = operation
	case "POST":
		pathItem.POST = operation
	case "PUT":
		pathItem.PUT = operation
	case "DELETE":
		pathItem.DELETE = operation
	case "PATCH":
		pathItem.PATCH = operation
	}
}

// itoa 将整数转换为字符串
func itoa(i int) string {
	return strconv.Itoa(i)
}
