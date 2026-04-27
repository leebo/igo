package main

import (
	"context"

	igo "github.com/igo/igo"
	"github.com/igo/igo/core"
	"github.com/igo/igo/middleware"
	"github.com/igo/igo/plugin/database"
	"gorm.io/gorm"
)

// User 模型
type User struct {
	ID    int64  `json:"id" gorm:"primaryKey"`
	Name  string `json:"name" gorm:"size:50;not null"`
	Email string `json:"email" gorm:"size:100;uniqueIndex"`
	Age   int    `json:"age"`
}

// UserRepository 用户仓储
type UserRepository struct {
	*database.BaseRepository[User]
}

// NewUserRepository 创建用户仓储
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		BaseRepository: database.NewRepository[User](db, "users"),
	}
}

var userRepo *UserRepository

func main() {
	// 连接数据库（SQLite 示例）
	db, err := database.Open(database.Config{
		Dialect: "sqlite",
		DSN:     "./test.db",
	})
	if err != nil {
		panic(err)
	}

	// 自动迁移
	db.AutoMigrate(&User{})

	// 创建仓储
	userRepo = NewUserRepository(db)

	app := igo.New()

	// 全局中间件
	app.Use(middleware.Logger())
	app.Use(middleware.Recovery())
	app.Use(middleware.CORS())

	// 路由
	app.Group("/api/v1", func(v1 *igo.App) {
		// 用户列表
		v1.Get("/users", listUsers)

		// 获取单个用户
		v1.Get("/users/:id", getUser)

		// 创建用户
		v1.Post("/users", createUser)

		// 更新用户
		v1.Put("/users/:id", updateUser)

		// 删除用户
		v1.Delete("/users/:id", deleteUser)
	})

	app.Run(":8080")
}

// listUsers 用户列表
func listUsers(c *core.Context) {
	page := c.QueryInt("page", 1)
	size := c.QueryInt("size", 20)

	ctx := context.Background()
	users, total, err := userRepo.List(ctx, page, size)
	if err != nil {
		c.InternalError("Failed to fetch users")
		return
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
	id := c.QueryInt("id", 0)
	if id == 0 {
		c.BadRequest("id is required")
		return
	}

	ctx := context.Background()
	user, err := userRepo.GetByID(ctx, int64(id))
	if err != nil {
		c.NotFound("User not found")
		return
	}

	c.Success(user)
}

// createUser 创建用户
func createUser(c *core.Context) {
	var user User
	if err := c.BindJSON(&user); err != nil {
		c.BadRequest("Invalid request body")
		return
	}

	ctx := context.Background()
	if err := userRepo.Create(ctx, &user); err != nil {
		c.InternalError("Failed to create user")
		return
	}

	c.Created(user)
}

// updateUser 更新用户
func updateUser(c *core.Context) {
	id := c.Param("id")
	if id == "" {
		c.BadRequest("id is required")
		return
	}

	var user User
	if err := c.BindJSON(&user); err != nil {
		c.BadRequest("Invalid request body")
		return
	}

	ctx := context.Background()
	existing, err := userRepo.GetByID(ctx, 1)
	if err != nil {
		c.NotFound("User not found")
		return
	}

	existing.Name = user.Name
	existing.Email = user.Email
	existing.Age = user.Age

	if err := userRepo.Update(ctx, existing); err != nil {
		c.InternalError("Failed to update user")
		return
	}

	c.Success(existing)
}

// deleteUser 删除用户
func deleteUser(c *core.Context) {
	id := c.Param("id")
	if id == "" {
		c.BadRequest("id is required")
		return
	}

	ctx := context.Background()
	if err := userRepo.Delete(ctx, 1); err != nil {
		c.InternalError("Failed to delete user")
		return
	}

	c.NoContent()
}
