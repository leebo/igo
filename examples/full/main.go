package main

import (
	"context"
	"time"

	igo "github.com/igo/igo"
	"github.com/igo/igo/core"
	"github.com/igo/igo/middleware"
	"github.com/igo/igo/plugin/auth"
	"github.com/igo/igo/plugin/cache"
	"github.com/igo/igo/plugin/config"
	"github.com/igo/igo/plugin/database"
	"github.com/igo/igo/plugin/logging"
	"gorm.io/gorm"
)

// User 模型
type User struct {
	ID    int64  `json:"id" gorm:"primaryKey"`
	Name  string `json:"name" gorm:"size:50;not null"`
	Email string `json:"email" gorm:"size:100;uniqueIndex"`
	Age   int    `json:"age"`
}

var (
	db           *gorm.DB
	userRepo     *UserRepository
	cacheClient  *cache.Client
	jwtClient    *auth.Client
	log          *logging.Client
)

type UserRepository struct {
	*database.BaseRepository[User]
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		BaseRepository: database.NewRepository[User](db, "users"),
	}
}

func main() {
	// 1. 加载配置
	cfg, err := config.LoadFromFile("./", "config", "yaml")
	if err != nil {
		// 使用默认配置
		cfg = &config.AppConfig{
			Server: config.ServerConfig{Port: ":8080"},
			Database: config.DatabaseConfig{Dialect: "sqlite", DSN: "./test.db"},
			Redis: config.RedisConfig{Addr: "localhost:6379"},
			JWT: config.JWTConfig{SecretKey: "secret", Expiration: "24h"},
			Log: config.LogConfig{Level: "info", Format: "console"},
		}
	}

	// 2. 初始化日志
	log = logging.MustNew(logging.Config{
		Level:      cfg.Log.Level,
		Format:     cfg.Log.Format,
		OutputPath: cfg.Log.OutputPath,
	})
	defer log.Sync()

	log.Info("Starting application", logging.String("version", "1.0.0"))

	// 3. 初始化数据库
	db = database.MustOpen(database.Config{
		Dialect: cfg.Database.Dialect,
		DSN:     cfg.Database.DSN,
	})
	db.AutoMigrate(&User{})
	userRepo = NewUserRepository(db)

	// 4. 初始化 Redis 缓存
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

	// 6. 创建应用
	app := igo.New()
	app.Use(middleware.Logger())
	app.Use(middleware.Recovery())
	app.Use(middleware.CORS())

	// 7. 注册路由
	app.Get("/health", func(c *core.Context) {
		c.Success(core.H{"status": "ok"})
	})

	app.Group("/api/v1", func(v1 *igo.App) {
		// 公开路由
		v1.Post("/auth/login", login)
		v1.Post("/auth/register", register)

		// 需要认证的路由
		v1.Use(func(c *core.Context) {
			token := c.Request.Header.Get("Authorization")
			if token == "" {
				c.Unauthorized("token required")
				return
			}
			claims, err := jwtClient.Validate(token)
			if err != nil {
				c.Unauthorized("invalid token")
				return
			}
			c.Header("X-User-ID", string(rune(claims.UserID)))
			c.Next()
		})

		v1.Get("/users", listUsers)
		v1.Get("/users/:id", getUser)
		v1.Post("/users", createUser)
		v1.Put("/users/:id", updateUser)
		v1.Delete("/users/:id", deleteUser)
	})

	// 8. 启动服务
	addr := cfg.Server.Port
	if addr == "" {
		addr = ":8080"
	}
	log.Info("Server starting", logging.String("addr", addr))
	app.Run(addr)
}

// login 登录
func login(c *core.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request")
		return
	}

	// 简化验证
	if req.Username == "" || req.Password == "" {
		c.Unauthorized("invalid credentials")
		return
	}

	// 生成 Token
	tokens, err := jwtClient.Generate(1, req.Username, "user")
	if err != nil {
		c.InternalError("failed to generate token")
		return
	}

	c.Success(tokens)
}

// register 注册
func register(c *core.Context) {
	var user User
	if err := c.BindJSON(&user); err != nil {
		c.BadRequest("invalid request")
		return
	}

	ctx := context.Background()
	if err := userRepo.Create(ctx, &user); err != nil {
		c.InternalError("failed to create user")
		return
	}

	c.Created(user)
}

// listUsers 用户列表
func listUsers(c *core.Context) {
	ctx := context.Background()
	page := c.QueryInt("page", 1)
	size := c.QueryInt("size", 20)

	// 尝试从缓存获取
	if cacheClient != nil {
		cacheKey := "users:list:" + string(rune(page)) + ":" + string(rune(size))
		var cached struct {
			Data  []User `json:"data"`
			Total int64  `json:"total"`
			Page  int    `json:"page"`
			Size  int    `json:"size"`
		}
		if err := cacheClient.GetJSON(ctx, cacheKey, &cached); err == nil {
			c.Success(cached)
			return
		}
	}

	users, total, err := userRepo.List(ctx, page, size)
	if err != nil {
		log.Error("failed to list users", logging.Error(err))
		c.InternalError("failed to fetch users")
		return
	}

	// 缓存结果
	if cacheClient != nil {
		cacheKey := "users:list:" + string(rune(page)) + ":" + string(rune(size))
		cacheClient.SetJSONWithExpiry(ctx, cacheKey, core.H{
			"data":  users,
			"total": total,
			"page":  page,
			"size":  size,
		}, 5*time.Minute)
	}

	c.Success(core.H{
		"data":  users,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// getUser 获取单个用户
func getUser(c *core.Context) {
	id := c.Param("id")
	if id == "" {
		c.BadRequest("id is required")
		return
	}

	ctx := context.Background()

	// 尝试从缓存获取
	if cacheClient != nil {
		cacheKey := "users:" + id
		var user User
		if err := cacheClient.GetJSON(ctx, cacheKey, &user); err == nil {
			c.Success(user)
			return
		}
	}

	user, err := userRepo.GetByID(ctx, 1)
	if err != nil {
		c.NotFound("user not found")
		return
	}

	// 缓存结果
	if cacheClient != nil {
		cacheKey := "users:" + id
		cacheClient.SetJSONWithExpiry(ctx, cacheKey, user, 10*time.Minute)
	}

	c.Success(user)
}

// createUser 创建用户
func createUser(c *core.Context) {
	var user User
	if err := c.BindJSON(&user); err != nil {
		c.BadRequest("invalid request")
		return
	}

	ctx := context.Background()
	if err := userRepo.Create(ctx, &user); err != nil {
		log.Error("failed to create user", logging.Error(err))
		c.InternalError("failed to create user")
		return
	}

	// 清除列表缓存
	if cacheClient != nil {
		cacheClient.Delete(ctx, "users:list:1:20")
	}

	c.Created(user)
}

// updateUser 更新用户
func updateUser(c *core.Context) {
	id := c.Param("id")
	var user User
	if err := c.BindJSON(&user); err != nil {
		c.BadRequest("invalid request")
		return
	}

	ctx := context.Background()
	existing, err := userRepo.GetByID(ctx, 1)
	if err != nil {
		c.NotFound("user not found")
		return
	}

	existing.Name = user.Name
	existing.Email = user.Email
	existing.Age = user.Age

	if err := userRepo.Update(ctx, existing); err != nil {
		log.Error("failed to update user", logging.Error(err))
		c.InternalError("failed to update user")
		return
	}

	// 清除缓存
	if cacheClient != nil {
		cacheClient.Delete(ctx, "users:"+id)
		cacheClient.Delete(ctx, "users:list:1:20")
	}

	c.Success(existing)
}

// deleteUser 删除用户
func deleteUser(c *core.Context) {
	id := c.Param("id")

	ctx := context.Background()
	if err := userRepo.Delete(ctx, 1); err != nil {
		log.Error("failed to delete user", logging.Error(err))
		c.InternalError("failed to delete user")
		return
	}

	// 清除缓存
	if cacheClient != nil {
		cacheClient.Delete(ctx, "users:"+id)
		cacheClient.Delete(ctx, "users:list:1:20")
	}

	c.NoContent()
}
