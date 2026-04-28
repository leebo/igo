package core

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/igo/igo/ai/schema"
	"github.com/igo/igo/core/errors"
	routepkg "github.com/igo/igo/core/route"
	"github.com/igo/igo/types"
)

// App 是 igo 应用的实例
type App struct {
	Router           *Router
	prefix           string
	server           *http.Server
	groupMiddlewares []MiddlewareFunc // 从父 Group 继承的中间件
	routeRegistry    *routepkg.Registry
	typeRegistry     *types.TypeRegistry
}

// New 创建一个新的 igo 应用
func New() *App {
	routeRegistry := routepkg.NewRegistry()
	typeRegistry := types.NewTypeRegistry()
	return &App{
		Router:        NewRouterWithRegistries(routeRegistry, typeRegistry),
		routeRegistry: routeRegistry,
		typeRegistry:  typeRegistry,
	}
}

// Use 注册全局中间件
func (a *App) Use(middleware MiddlewareFunc) {
	a.Router.Use(middleware)
}

// Get 注册 GET 请求，可附加路由级中间件
func (a *App) Get(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	a.Router.GET(a.prefix+path, handler, a.resolveMiddlewares(middlewares)...)
}

// Post 注册 POST 请求，可附加路由级中间件
func (a *App) Post(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	a.Router.POST(a.prefix+path, handler, a.resolveMiddlewares(middlewares)...)
}

// Put 注册 PUT 请求，可附加路由级中间件
func (a *App) Put(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	a.Router.PUT(a.prefix+path, handler, a.resolveMiddlewares(middlewares)...)
}

// Delete 注册 DELETE 请求，可附加路由级中间件
func (a *App) Delete(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	a.Router.DELETE(a.prefix+path, handler, a.resolveMiddlewares(middlewares)...)
}

// Patch 注册 PATCH 请求，可附加路由级中间件
func (a *App) Patch(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	a.Router.PATCH(a.prefix+path, handler, a.resolveMiddlewares(middlewares)...)
}

// Options 注册 OPTIONS 请求，可附加路由级中间件
func (a *App) Options(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	a.Router.OPTIONS(a.prefix+path, handler, a.resolveMiddlewares(middlewares)...)
}

// Head 注册 HEAD 请求，可附加路由级中间件
func (a *App) Head(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	a.Router.HEAD(a.prefix+path, handler, a.resolveMiddlewares(middlewares)...)
}

// resolveMiddlewares 将 group 中间件与路由级中间件合并
func (a *App) resolveMiddlewares(routeMiddlewares []MiddlewareFunc) []MiddlewareFunc {
	if len(a.groupMiddlewares) == 0 {
		return routeMiddlewares
	}
	result := make([]MiddlewareFunc, 0, len(a.groupMiddlewares)+len(routeMiddlewares))
	result = append(result, a.groupMiddlewares...)
	result = append(result, routeMiddlewares...)
	return result
}

// Group 创建路由分组，middlewares 作用于组内所有路由
func (a *App) Group(prefix string, fn func(*App), middlewares ...MiddlewareFunc) {
	combined := make([]MiddlewareFunc, 0, len(a.groupMiddlewares)+len(middlewares))
	combined = append(combined, a.groupMiddlewares...)
	combined = append(combined, middlewares...)

	subApp := &App{
		Router:           a.Router,
		prefix:           a.prefix + prefix,
		groupMiddlewares: combined,
		routeRegistry:    a.routeRegistry,
		typeRegistry:     a.typeRegistry,
	}
	fn(subApp)
}

// Resources 注册 RESTful 资源路由
func (a *App) Resources(path string, h ResourceHandler, middlewares ...MiddlewareFunc) {
	a.Router.Resources(a.prefix+path, h, a.resolveMiddlewares(middlewares)...)
}

// SetNotFound 设置 404 处理器
func (a *App) SetNotFound(handler HandlerFunc) {
	a.Router.SetNotFound(handler)
}

// Static 在指定 URL 前缀提供本地目录的静态文件服务
//
// 示例：app.Static("/static", "./public") 把 ./public 下的文件挂到 /static/* 下
//
// 内部使用通配符路由 + http.FileServer，自动处理 ETag/Range/MIME。
// 注意：dir 应该是受信任的目录，不要直接挂用户上传目录（path-traversal 风险），
// 如需暴露上传文件参考 examples/upload 的写法。
func (a *App) Static(prefix, dir string) {
	prefix = strings.TrimRight(prefix, "/")
	fileServer := http.StripPrefix(prefix, http.FileServer(http.Dir(dir)))
	handler := func(c *Context) {
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
	a.Get(prefix+"/*filepath", handler)
}

// Routes 返回当前应用所有已注册的路由元数据
// AI 在调试时可调 app.Routes() 拿到全部路由信息，无需读源码
func (a *App) Routes() []*routepkg.RouteConfig {
	return a.routeRegistry.ListRoutes()
}

// Schemas 返回当前应用显式注册和运行时绑定过的类型 Schema。
func (a *App) Schemas() []*types.TypeSchema {
	appSchemas := a.typeRegistry.ListTypes()
	legacySchemas := types.GlobalTypeRegistry.ListTypes()
	if len(legacySchemas) == 0 {
		return appSchemas
	}

	seen := make(map[string]bool, len(appSchemas))
	for _, schema := range appSchemas {
		seen[schema.Name] = true
	}
	for _, schema := range legacySchemas {
		if !seen[schema.Name] {
			appSchemas = append(appSchemas, schema)
			seen[schema.Name] = true
		}
	}
	sort.Slice(appSchemas, func(i, j int) bool {
		if appSchemas[i].Package != appSchemas[j].Package {
			return appSchemas[i].Package < appSchemas[j].Package
		}
		return appSchemas[i].Name < appSchemas[j].Name
	})
	return appSchemas
}

// PrintRoutes 将所有已注册路由按 method+path 排序打印到 stdout
// 用于启动时让人类与 AI 都能看到完整路由表
func (a *App) PrintRoutes() {
	routes := a.Router.Routes()
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Method < routes[j].Method
	})

	fmt.Printf("\n[igo] %d route(s), %d global middleware(s):\n",
		len(routes), a.Router.GlobalMiddlewareCount())
	for _, r := range routes {
		cfg := a.routeRegistry.GetRoute(r.Method, r.Path)
		summary := ""
		if cfg != nil && cfg.Summary != "" {
			summary = "  // " + cfg.Summary
		}
		fmt.Printf("  %-6s %-40s -> %s%s\n", r.Method, r.Path, r.Name, summary)
	}
	fmt.Println()
}

// RegisterAIRoutes 注册一组 AI 自省端点，方便 AI 在运行时查询应用状态
//
//	GET /_ai/routes       列出所有路由及推断元数据
//	GET /_ai/schemas      列出当前 App 的类型 Schema
//	GET /_ai/errors       列出框架错误码约定
//	GET /_ai/info         框架信息总览
//	GET /_ai/openapi      输出 OpenAPI 3.0 JSON
//	GET /_ai/conventions  输出 AI 编码约定
//	GET /_ai/middlewares  列出中间件注册顺序
func (a *App) RegisterAIRoutes() {
	a.Get("/_ai/routes", func(c *Context) {
		c.JSON(http.StatusOK, a.Routes())
	})
	a.Get("/_ai/middlewares", func(c *Context) {
		c.JSON(http.StatusOK, a.middlewareSnapshot())
	})
	a.Get("/_ai/info", func(c *Context) {
		c.JSON(http.StatusOK, H{
			"framework":       "igo",
			"routeCount":      len(a.Routes()),
			"middlewareCount": a.Router.GlobalMiddlewareCount(),
			"schemaCount":     len(a.Schemas()),
		})
	})
	a.Get("/_ai/schemas", func(c *Context) {
		c.JSON(http.StatusOK, a.Schemas())
	})
	a.Get("/_ai/errors", func(c *Context) {
		c.JSON(http.StatusOK, errors.ListErrorCodes())
	})
	a.Get("/_ai/openapi", func(c *Context) {
		gen := schema.NewRouteGenerator(a.Routes(), a.Schemas()...)
		c.JSON(http.StatusOK, gen.Generate())
	})
	a.Get("/_ai/conventions", func(c *Context) {
		c.JSON(http.StatusOK, AIConventions())
	})
}

// RegisterSchema 把类型显式注册到当前 App 的 schema 注册表。
// 用于不会经过 BindAndValidate 的类型（如纯响应类型）也能出现在 /_ai/schemas。
func (a *App) RegisterSchema(sample any) {
	registerSchemaOnce(a.typeRegistry, sample)
}

// RegisterAppSchema 把类型 T 显式注册到指定 App 的 schema 注册表。
func RegisterAppSchema[T any](app *App) {
	if app == nil {
		return
	}
	var zero T
	app.RegisterSchema(&zero)
}

// RegisterSchema 把类型 T 显式注册到兼容全局 schema 注册表。
//
// Deprecated: use app.RegisterSchema(UserResponse{}) or RegisterAppSchema[T](app)
// so schemas stay isolated per App.
func RegisterSchema[T any]() {
	var zero T
	registerSchemaOnce(types.GlobalTypeRegistry, &zero)
}

func (a *App) middlewareSnapshot() H {
	global := a.Router.GlobalMiddlewareNames()
	globalItems := make([]H, 0, len(global))
	for i, name := range global {
		globalItems = append(globalItems, H{"order": i, "name": name})
	}

	routes := a.Routes()
	routeItems := make([]H, 0, len(routes))
	for _, route := range routes {
		middlewares := make([]H, 0, len(route.Middlewares))
		for i, name := range route.Middlewares {
			middlewares = append(middlewares, H{"order": i, "name": name})
		}
		routeItems = append(routeItems, H{
			"method":      route.Method,
			"path":        route.Path,
			"middlewares": middlewares,
		})
	}

	return H{
		"globalCount": a.Router.GlobalMiddlewareCount(),
		"global":      globalItems,
		"routes":      routeItems,
	}
}

// AIConventions 返回 CLI 和运行时端点共享的轻量编码约定。
func AIConventions() H {
	return H{
		"workflow": AIWorkflow(),
		"rules": []string{
			"Use BindAndValidate[T](c) for JSON bodies and return immediately when ok is false.",
			"Use BindQueryAndValidate[T](c) for structured query input.",
			"Use BindPathAndValidate[T](c) or Param*OrFail helpers for path parameters.",
			"Use *Wrap error helpers inside err branches to preserve the error chain.",
			"Register pure response DTOs with app.RegisterSchema(ResponseDTO{}).",
			"Inspect /_ai/routes, /_ai/schemas, /_ai/errors, and /_ai/openapi before editing unfamiliar apps.",
		},
		"endpoints": []string{
			"/_ai/routes",
			"/_ai/schemas",
			"/_ai/errors",
			"/_ai/info",
			"/_ai/openapi",
			"/_ai/conventions",
			"/_ai/middlewares",
		},
	}
}

// AIWorkflow 返回 AI 编码工具可复用的短流程。
func AIWorkflow() []string {
	return []string{
		"Run `igo ai context . --format json` to get compact project facts.",
		"Inspect `igo ai routes .` and `igo ai schemas .` before changing handlers or DTOs.",
		"Use `igo ai prompt . METHOD PATH` for a route-specific implementation prompt.",
		"Validate with `go test ./...` and `igo doctor .`.",
		"When the app is running, compare CLI output with /_ai/routes, /_ai/schemas, /_ai/errors, and /_ai/openapi.",
	}
}

// Run 启动服务器，收到 SIGINT/SIGTERM 后优雅关闭（等待最多 30s）
func (a *App) Run(addr string) error {
	a.server = &http.Server{
		Addr:         addr,
		Handler:      a.Router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server starting on %s", addr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return a.server.Shutdown(ctx)
}
