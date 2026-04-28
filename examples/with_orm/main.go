// Package main 演示 igo + GORM 的标准集成模式
//
// AI 学习要点：
//   - 用 c.ParamInt64OrFail 解析 :id，避免 strconv 样板
//   - 用 core.BindAndValidate[T] 绑定 + 校验一步到位
//   - 错误一律用 *Wrap 系列保留 err 调用链
//   - 用 c.Request.Context() 而不是 context.Background()
package main

import (
	igo "github.com/igo/igo"
	"github.com/igo/igo/core"
	"github.com/igo/igo/middleware"
	"github.com/igo/igo/plugin/database"
	"gorm.io/gorm"
)

// User 模型
type User struct {
	ID    int64  `json:"id" gorm:"primaryKey"`
	Name  string `json:"name" gorm:"size:50;not null" validate:"required|min:2|max:50"`
	Email string `json:"email" gorm:"size:100;uniqueIndex" validate:"required|email"`
	Age   int    `json:"age" validate:"gte:0|lte:150"`
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
	db, err := database.Open(database.Config{
		Dialect: "sqlite",
		DSN:     "./test.db",
	})
	if err != nil {
		panic(err)
	}
	db.AutoMigrate(&User{})
	userRepo = NewUserRepository(db)

	app := igo.New()
	app.Use(middleware.Recovery())
	app.Use(middleware.CORS())
	app.Use(middleware.Logger())

	app.Group("/api/v1", func(v1 *igo.App) {
		v1.Get("/users", listUsers)
		v1.Get("/users/:id", getUser)
		v1.Post("/users", createUser)
		v1.Put("/users/:id", updateUser)
		v1.Delete("/users/:id", deleteUser)
	})

	app.Run(":8080")
}

func listUsers(c *core.Context) {
	page := c.QueryInt("page", 1)
	size := c.QueryInt("size", 20)

	users, total, err := userRepo.List(c.Request.Context(), page, size)
	if err != nil {
		c.InternalErrorWrap(err, "failed to fetch users", map[string]any{
			"page": page, "size": size,
		})
		return
	}

	c.Success(core.H{
		"data":  users,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

func getUser(c *core.Context) {
	id, ok := c.ParamInt64OrFail("id")
	if !ok {
		return
	}

	user, err := userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.NotFoundWrap(err, "user not found")
		return
	}
	c.Success(user)
}

func createUser(c *core.Context) {
	user, ok := core.BindAndValidate[User](c)
	if !ok {
		return
	}

	if err := userRepo.Create(c.Request.Context(), user); err != nil {
		c.InternalErrorWrap(err, "failed to create user", map[string]any{
			"email": user.Email,
		})
		return
	}
	c.Created(user)
}

func updateUser(c *core.Context) {
	id, ok := c.ParamInt64OrFail("id")
	if !ok {
		return
	}

	patch, ok := core.BindAndValidate[User](c)
	if !ok {
		return
	}

	existing, err := userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.NotFoundWrap(err, "user not found")
		return
	}

	existing.Name = patch.Name
	existing.Email = patch.Email
	existing.Age = patch.Age

	if err := userRepo.Update(c.Request.Context(), existing); err != nil {
		c.InternalErrorWrap(err, "failed to update user", map[string]any{
			"user_id": id,
		})
		return
	}
	c.Success(existing)
}

func deleteUser(c *core.Context) {
	id, ok := c.ParamInt64OrFail("id")
	if !ok {
		return
	}

	if err := userRepo.Delete(c.Request.Context(), id); err != nil {
		c.InternalErrorWrap(err, "failed to delete user", map[string]any{
			"user_id": id,
		})
		return
	}
	c.NoContent()
}
