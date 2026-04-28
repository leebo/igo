package route

import (
	"sort"
	"sync"
)

// Registry 路由元数据注册表
type Registry struct {
	mu     sync.RWMutex
	routes map[string]*RouteConfig // key: "GET /users/:id"
	byName map[string]*RouteConfig // key: route name
}

// NewRegistry 创建新的注册表
func NewRegistry() *Registry {
	return &Registry{
		routes: make(map[string]*RouteConfig),
		byName: make(map[string]*RouteConfig),
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
	copyCfg := *cfg
	r.routes[key] = &copyCfg

	if cfg.HandlerName != "" {
		r.byName[cfg.HandlerName] = &copyCfg
	}
}

// GetRoute 获取路由配置
func (r *Registry) GetRoute(method, path string) *RouteConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cfg := r.routes[routeKey(method, path)]
	if cfg == nil {
		return nil
	}
	copyCfg := *cfg
	return &copyCfg
}

// GetRouteByName 通过名称获取路由配置
func (r *Registry) GetRouteByName(name string) *RouteConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cfg := r.byName[name]
	if cfg == nil {
		return nil
	}
	copyCfg := *cfg
	return &copyCfg
}

// ListRoutes 列出所有路由配置
func (r *Registry) ListRoutes() []*RouteConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RouteConfig, 0, len(r.routes))
	for _, cfg := range r.routes {
		copyCfg := *cfg
		result = append(result, &copyCfg)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path != result[j].Path {
			return result[i].Path < result[j].Path
		}
		return result[i].Method < result[j].Method
	})
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
				copyCfg := *cfg
				result = append(result, &copyCfg)
				break
			}
		}
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
}

// DefaultRegistry 是兼容旧代码的全局路由元数据注册表。
//
// 新代码不应依赖它：每个 App 都有自己的 Registry，避免多 App/测试相互污染。
var DefaultRegistry = NewRegistry()
