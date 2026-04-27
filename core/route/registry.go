package route

import (
	"sync"
)

// Registry 路由元数据注册表
type Registry struct {
	mu         sync.RWMutex
	routes     map[string]*RouteConfig // key: "GET /users/:id"
	byName     map[string]*RouteConfig // key: route name
	typeSchemas map[string]*TypeSchemaInfo
}

// TypeSchemaInfo 类型 Schema 信息
type TypeSchemaInfo struct {
	TypeName string
	FilePath string
}

// NewRegistry 创建新的注册表
func NewRegistry() *Registry {
	return &Registry{
		routes:     make(map[string]*RouteConfig),
		byName:     make(map[string]*RouteConfig),
		typeSchemas: make(map[string]*TypeSchemaInfo),
	}
}

// routeKey 生成路由的唯一 key
func routeKey(method, path string) string {
	return method + " " + path
}

// RegisterRoute 注册路由配置
func (r *Registry) RegisterRoute(cfg *RouteConfig) {
	if cfg == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := routeKey(cfg.Method, cfg.Path)
	r.routes[key] = cfg

	if cfg.HandlerName != "" {
		r.byName[cfg.HandlerName] = cfg
	}
}

// GetRoute 获取路由配置
func (r *Registry) GetRoute(method, path string) *RouteConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.routes[routeKey(method, path)]
}

// GetRouteByName 通过名称获取路由配置
func (r *Registry) GetRouteByName(name string) *RouteConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.byName[name]
}

// ListRoutes 列出所有路由配置
func (r *Registry) ListRoutes() []*RouteConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RouteConfig, 0, len(r.routes))
	for _, cfg := range r.routes {
		result = append(result, cfg)
	}
	return result
}

// ListRoutesByTag 根据标签获取路由
func (r *Registry) ListRoutesByTag(tag string) []*RouteConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RouteConfig, 0)
	for _, cfg := range r.routes {
		for _, t := range cfg.Tags {
			if t == tag {
				result = append(result, cfg)
				break
			}
		}
	}
	return result
}

// RegisterTypeSchema 注册类型 Schema
func (r *Registry) RegisterTypeSchema(typeName, filePath string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.typeSchemas[typeName] = &TypeSchemaInfo{
		TypeName: typeName,
		FilePath: filePath,
	}
}

// GetTypeSchema 获取类型 Schema 信息
func (r *Registry) GetTypeSchema(typeName string) *TypeSchemaInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.typeSchemas[typeName]
}

// ListTypeSchemas 列出所有类型 Schema
func (r *Registry) ListTypeSchemas() []*TypeSchemaInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*TypeSchemaInfo, 0, len(r.typeSchemas))
	for _, info := range r.typeSchemas {
		result = append(result, info)
	}
	return result
}

// Count 返回注册表中的路由数量
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.routes)
}

// Clear 清空注册表
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.routes = make(map[string]*RouteConfig)
	r.byName = make(map[string]*RouteConfig)
	r.typeSchemas = make(map[string]*TypeSchemaInfo)
}
