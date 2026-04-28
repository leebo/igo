package core

import (
	"net/http"
	"path"
	"reflect"
	"runtime"
	"strings"

	routepkg "github.com/leebo/igo/core/route"
	"github.com/leebo/igo/types"
)

// Router 是 igo 的路由管理器
type Router struct {
	routes            []*Route
	namedRoutes       map[string]*Route
	notFound          HandlerFunc
	globalMiddlewares []MiddlewareFunc
	routeRegistry     *routepkg.Registry
	typeRegistry      *types.TypeRegistry
}

// Route 代表一个路由
type Route struct {
	Method      string
	Path        string
	Handler     HandlerFunc
	Middlewares []MiddlewareFunc
	Name        string
	HandlerInfo *HandlerInfo
}

// HandlerInfo 处理函数信息
type HandlerInfo struct {
	Name     string
	FilePath string
	Line     int
}

// NewRouter 创建新的路由实例
func NewRouter() *Router {
	return NewRouterWithRegistries(routepkg.NewRegistry(), types.NewTypeRegistry())
}

// NewRouterWithRegistries 创建绑定到指定元数据注册表的路由实例。
func NewRouterWithRegistries(routeRegistry *routepkg.Registry, typeRegistry *types.TypeRegistry) *Router {
	if routeRegistry == nil {
		routeRegistry = routepkg.NewRegistry()
	}
	if typeRegistry == nil {
		typeRegistry = types.NewTypeRegistry()
	}
	return &Router{
		routes:            make([]*Route, 0),
		namedRoutes:       make(map[string]*Route),
		globalMiddlewares: make([]MiddlewareFunc, 0),
		routeRegistry:     routeRegistry,
		typeRegistry:      typeRegistry,
	}
}

// Use 注册全局中间件
func (r *Router) Use(middleware MiddlewareFunc) {
	r.globalMiddlewares = append(r.globalMiddlewares, middleware)
}

// Routes 返回已注册路由的副本（只读用途）
func (r *Router) Routes() []*Route {
	out := make([]*Route, len(r.routes))
	copy(out, r.routes)
	return out
}

// GlobalMiddlewareCount 返回全局中间件数量
func (r *Router) GlobalMiddlewareCount() int {
	return len(r.globalMiddlewares)
}

// GlobalMiddlewareNames 返回全局中间件函数名，按注册顺序排列。
func (r *Router) GlobalMiddlewareNames() []string {
	return handlerFuncNames(r.globalMiddlewares)
}

// GET 注册 GET 请求
func (r *Router) GET(p string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.addRoute(http.MethodGet, p, handler, middlewares...)
}

// POST 注册 POST 请求
func (r *Router) POST(p string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.addRoute(http.MethodPost, p, handler, middlewares...)
}

// PUT 注册 PUT 请求
func (r *Router) PUT(p string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.addRoute(http.MethodPut, p, handler, middlewares...)
}

// DELETE 注册 DELETE 请求
func (r *Router) DELETE(p string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.addRoute(http.MethodDelete, p, handler, middlewares...)
}

// PATCH 注册 PATCH 请求
func (r *Router) PATCH(p string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.addRoute(http.MethodPatch, p, handler, middlewares...)
}

// OPTIONS 注册 OPTIONS 请求
func (r *Router) OPTIONS(p string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.addRoute(http.MethodOptions, p, handler, middlewares...)
}

// HEAD 注册 HEAD 请求
func (r *Router) HEAD(p string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.addRoute(http.MethodHead, p, handler, middlewares...)
}

func (r *Router) addRoute(method, p string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	p = path.Clean(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	handlerName := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
	filePath, line := getFileLine(handler)

	route := &Route{
		Method:      method,
		Path:        p,
		Handler:     handler,
		Middlewares: middlewares,
		Name:        handlerName,
		HandlerInfo: &HandlerInfo{
			Name:     handlerName,
			FilePath: filePath,
			Line:     line,
		},
	}
	r.routes = append(r.routes, route)

	// 自动推断元数据并写入全局 Registry
	inferred := routepkg.GlobalInferenceEngine.InferFromFunction(handlerName, method, p)
	inferred.HandlerName = handlerName
	inferred.FilePath = filePath
	inferred.LineNumber = line
	inferred.Middlewares = handlerFuncNames(middlewares)
	r.routeRegistry.RegisterRoute(inferred)
}

// getFileLine 获取函数定义的文件和行号
func getFileLine(handler HandlerFunc) (filePath string, line int) {
	pc := reflect.ValueOf(handler).Pointer()
	if f := runtime.FuncForPC(pc); f != nil {
		filePath, line = f.FileLine(pc)
	}
	return
}

// Resources 注册 RESTful 资源路由
func (r *Router) Resources(basePath string, h ResourceHandler, middlewares ...MiddlewareFunc) {
	basePath = path.Clean(basePath)

	r.GET(basePath, wrapResourceHandler(h.List), middlewares...)
	r.POST(basePath, wrapResourceHandler(h.Create), middlewares...)
	r.GET(path.Join(basePath, ":id"), wrapResourceHandler(h.Show), middlewares...)
	r.PUT(path.Join(basePath, ":id"), wrapResourceHandler(h.Update), middlewares...)
	r.DELETE(path.Join(basePath, ":id"), wrapResourceHandler(h.Delete), middlewares...)
}

// SetNotFound 设置 404 处理器
func (r *Router) SetNotFound(handler HandlerFunc) {
	r.notFound = handler
}

// ServeHTTP 实现 http.Handler 接口
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := newContext(w, req, r.typeRegistry)
	ctx.handlers = append(ctx.handlers, r.globalMiddlewares...)

	// 精确匹配
	for _, route := range r.routes {
		if route.Method == req.Method && route.Path == req.URL.Path {
			r.handleRoute(ctx, route)
			return
		}
	}

	// 参数路由匹配
	for _, route := range r.routes {
		if route.Method == req.Method && matchPath(route.Path, req.URL.Path) {
			ctx.Params = extractParams(route.Path, req.URL.Path)
			r.handleRoute(ctx, route)
			return
		}
	}

	// 通配符路由匹配
	for _, route := range r.routes {
		if route.Method == req.Method && matchWildcardPath(route.Path, req.URL.Path) {
			ctx.Params = extractWildcardParams(route.Path, req.URL.Path)
			r.handleRoute(ctx, route)
			return
		}
	}

	// 404
	if r.notFound != nil {
		ctx.handlers = append(ctx.handlers, r.notFound)
	} else {
		ctx.handlers = append(ctx.handlers, func(c *Context) {
			c.JSON(http.StatusNotFound, H{
				"error": H{
					"code":    "NOT_FOUND",
					"message": "The requested resource was not found",
				},
			})
		})
	}
	r.runHandlers(ctx)
}

func handlerFuncNames(handlers []MiddlewareFunc) []string {
	if len(handlers) == 0 {
		return nil
	}
	names := make([]string, 0, len(handlers))
	for _, handler := range handlers {
		names = append(names, handlerFuncName(handler))
	}
	return names
}

func handlerFuncName(handler HandlerFunc) string {
	if handler == nil {
		return ""
	}
	fn := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
	if fn == nil {
		return ""
	}
	return fn.Name()
}

// handleRoute 处理路由，执行中间件链
func (r *Router) handleRoute(ctx *Context, route *Route) {
	ctx.handlers = append(ctx.handlers, route.Middlewares...)
	ctx.handlers = append(ctx.handlers, route.Handler)
	r.runHandlers(ctx)
}

// runHandlers 执行处理器链，仅在未写入响应时兜底 panic 恢复
func (r *Router) runHandlers(ctx *Context) {
	defer func() {
		if err := recover(); err != nil {
			// 仅当尚未写入响应时才兜底，避免与 Recovery 中间件双写
			if ctx.statusCode == 0 {
				ctx.Writer.Header().Set("Content-Type", "application/json")
				ctx.JSON(http.StatusInternalServerError, H{
					"error": H{
						"code":    "INTERNAL_ERROR",
						"message": "Internal server error",
					},
				})
			}
		}
	}()
	ctx.Next()
}

// matchWildcardPath 检查路径是否匹配通配符模式（/path/*subpath）
func matchWildcardPath(pattern, path string) bool {
	if !strings.Contains(pattern, "*") {
		return false
	}
	prefix := strings.Split(pattern, "*")[0]
	return strings.HasPrefix(path, prefix)
}

// extractWildcardParams 提取通配符参数
func extractWildcardParams(pattern, path string) map[string]string {
	params := make(map[string]string)
	if !strings.Contains(pattern, "*") {
		return params
	}

	prefix := strings.Split(pattern, "*")[0]
	starIdx := strings.Index(pattern, "*")
	wildcardName := strings.TrimPrefix(pattern[starIdx+1:], "*")

	if strings.HasPrefix(path, prefix) {
		wildcardValue := strings.TrimPrefix(path, prefix)
		wildcardValue = strings.TrimPrefix(wildcardValue, "/")
		params[wildcardName] = wildcardValue
	}
	return params
}
