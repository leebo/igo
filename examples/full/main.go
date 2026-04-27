// Package main 是应用入口
//
// igo:summary: 应用入口
// igo:description: 初始化所有组件（数据库、缓存、认证等），注册路由，启动 HTTP 服务
// igo:ai-hint: 标准的应用初始化顺序：配置 -> 日志 -> 数据库 -> 缓存 -> 认证 -> 路由 -> 服务
package main

import (
	"time"

	igo "github.com/igo/igo"
	"github.com/igo/igo/examples/full/config"
	"github.com/igo/igo/examples/full/handlers"
	"github.com/igo/igo/examples/full/repositories"
	"github.com/igo/igo/examples/full/routes"
	"github.com/igo/igo/examples/full/services"
	"github.com/igo/igo/plugin/auth"
	"github.com/igo/igo/plugin/cache"
	"github.com/igo/igo/plugin/database"
	"github.com/igo/igo/plugin/logging"
)

var (
	userRepo    *repositories.UserRepository
	userService *services.UserService
	cacheClient *cache.Client
	jwtClient   *auth.Client
	log         *logging.Client
)

func main() {
	// 1. 加载配置
	cfg := config.LoadConfig()

	// 2. 初始化日志
	log = logging.MustNew(logging.Config{
		Level:      cfg.Log.Level,
		Format:     cfg.Log.Format,
		OutputPath: cfg.Log.OutputPath,
	})
	defer log.Sync()

	log.Info("Starting application", logging.String("version", "1.0.0"))

	// 3. 初始化数据库
	db := database.MustOpen(database.Config{
		Dialect: cfg.Database.Dialect,
		DSN:     cfg.Database.DSN,
	})

	// 初始化 Repository
	userRepo = repositories.NewUserRepository(db)

	// 4. 初始化 Redis 缓存
	var err error
	cacheClient, err = cache.New(cache.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		log.Warn("Redis connection failed, caching disabled", logging.Error(err))
		cacheClient = nil
	}

	// 5. 初始化 JWT
	jwtClient = auth.New(auth.Config{
		SecretKey:     cfg.JWT.SecretKey,
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	// 6. 初始化 Service
	userService = services.NewUserService(userRepo, cacheClient)

	// 7. 创建应用
	app := igo.New()

	// 8. 注册路由
	userHandler := handlers.NewUserHandler(userService)
	routes.RegisterRoutes(app, userHandler, jwtClient)

	// 9. 启动服务
	addr := cfg.Server.Port
	if addr == "" {
		addr = ":8080"
	}
	log.Info("Server starting", logging.String("addr", addr))
	app.Run(addr)
}
