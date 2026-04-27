package core

import (
	"net/http"
	"path"
	"strings"
)

// Router 是 igo 的路由管理器
type Router struct {
	routes         []*Route
	namedRoutes    map[string]*Route
	notFound       HandlerFunc
	globalMiddlewares []MiddlewareFunc
}

// Route 代表一个路由
type Route struct {
	Method      string
	Path        string
	Handler     HandlerFunc
	Middlewares []MiddlewareFunc
	Name        string
}

// NewRouter 创建新的路由实例
func NewRouter() *Router {
	return &Router{
		routes:         make([]*Route, 0),
		namedRoutes:    make(map[string]*Route),
		globalMiddlewares: make([]MiddlewareFunc, 0),
	}
}

// Use 注册全局中间件
func (r *Router) Use(middleware MiddlewareFunc) {
	r.globalMiddlewares = append(r.globalMiddlewares, middleware)
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
	// 规范化路径
	p = path.Clean(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	route := &Route{
		Method:      method,
		Path:        p,
		Handler:     handler,
		Middlewares: middlewares,
	}
	r.routes = append(r.routes, route)
}

// Resources 注册 RESTful 资源路由
func (r *Router) Resources(basePath string, h ResourceHandler, middlewares ...MiddlewareFunc) {
	basePath = path.Clean(basePath)

	// List - GET /users
	r.GET(basePath, wrapResourceHandler(h.List), middlewares...)

	// Create - POST /users
	r.POST(basePath, wrapResourceHandler(h.Create), middlewares...)

	// Show - GET /users/:id
	r.GET(path.Join(basePath, ":id"), wrapResourceHandler(h.Show), middlewares...)

	// Update - PUT /users/:id
	r.PUT(path.Join(basePath, ":id"), wrapResourceHandler(h.Update), middlewares...)

	// Delete - DELETE /users/:id
	r.DELETE(path.Join(basePath, ":id"), wrapResourceHandler(h.Delete), middlewares...)
}

// SetNotFound 设置 404 处理器
func (r *Router) SetNotFound(handler HandlerFunc) {
	r.notFound = handler
}

// ServeHTTP 实现 http.Handler 接口
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := newContext(w, req)

	// 构建全局中间件链
	ctx.handlers = append(ctx.handlers, r.globalMiddlewares...)

	// 查找匹配的路由
	for _, route := range r.routes {
		if route.Method == req.Method && route.Path == req.URL.Path {
			r.handleRoute(ctx, route)
			return
		}
	}

	// 尝试匹配带参数的路由
	for _, route := range r.routes {
		if route.Method == req.Method && matchPath(route.Path, req.URL.Path) {
			ctx.Params = extractParams(route.Path, req.URL.Path)
			r.handleRoute(ctx, route)
			return
		}
	}

	// 404 - 只执行全局中间件
	if r.notFound != nil {
		ctx.handlers = append(ctx.handlers, r.notFound)
	} else {
		// 添加 404 处理器
		ctx.handlers = append(ctx.handlers, func(c *Context) {
			c.JSON(http.StatusNotFound, H{
				"error": H{
					"code":    "NOT_FOUND",
					"message": "The requested resource was not found",
				},
			})
		})
	}

	r.runWithRecovery(ctx)
}

// handleRoute 处理路由，执行中间件链
func (r *Router) handleRoute(ctx *Context, route *Route) {
	// 添加路由特定中间件 + 处理器
	ctx.handlers = append(ctx.handlers, route.Middlewares...)
	ctx.handlers = append(ctx.handlers, route.Handler)

	// 执行链
	r.runWithRecovery(ctx)
}

// runWithRecovery 安全执行处理器链
func (r *Router) runWithRecovery(ctx *Context) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Writer.Header().Set("Content-Type", "application/json")
			ctx.JSON(http.StatusInternalServerError, H{
				"error": H{
					"code":    "INTERNAL_ERROR",
					"message": "Internal server error",
				},
			})
		}
	}()

	ctx.Next()
}
