// Package main 演示如何为 igo handler 写测试。
//
// AI 学习要点（这是 AI 编程最大的反馈回路）：
//   - 用接口抽象 service 层，便于 mock
//   - 用 httptest.NewRequest + Recorder 模拟请求
//   - 用表驱动测试覆盖多场景（happy path + 错误分支）
//   - 断言 JSON 响应：状态码 + body 关键字段
//   - 测试中间件：单独测 c.Next() 是否被调用
//
// 运行：
//
//	go test ./examples/testing/...
//	go run ./examples/testing/         # 启动服务（端口 :8080）
package main

import (
	"errors"

	igo "github.com/leebo/igo"
	"github.com/leebo/igo/core"
)

// User 业务实体
type User struct {
	ID   int64  `json:"id"`
	Name string `json:"name" validate:"required|min:2"`
}

// UserService 是抽象接口，handler 只依赖它（不依赖具体实现）
// 这是测试可注入 mock 的关键
type UserService interface {
	GetByID(id int64) (*User, error)
	Create(name string) (*User, error)
}

// ErrUserNotFound 业务错误
var ErrUserNotFound = errors.New("user not found")

// userHandler 持有 service 接口，构造时注入
type userHandler struct {
	svc UserService
}

func newUserHandler(svc UserService) *userHandler {
	return &userHandler{svc: svc}
}

// Get 处理 GET /users/:id
func (h *userHandler) Get(c *core.Context) {
	id, ok := c.ParamInt64OrFail("id")
	if !ok {
		return
	}
	user, err := h.svc.GetByID(id)
	if err != nil {
		c.NotFoundWrap(err, "user not found")
		return
	}
	c.Success(user)
}

// CreateUserRequest 请求体
type CreateUserRequest struct {
	Name string `json:"name" validate:"required|min:2|max:50"`
}

// Create 处理 POST /users
func (h *userHandler) Create(c *core.Context) {
	req, ok := igo.BindAndValidate[CreateUserRequest](c)
	if !ok {
		return
	}
	user, err := h.svc.Create(req.Name)
	if err != nil {
		c.InternalErrorWrap(err, "failed to create user", nil)
		return
	}
	c.Created(user)
}

// realUserService 生产实现（演示用，写死内存）
type realUserService struct{}

func (s *realUserService) GetByID(id int64) (*User, error) {
	if id == 1 {
		return &User{ID: 1, Name: "Alice"}, nil
	}
	return nil, ErrUserNotFound
}

func (s *realUserService) Create(name string) (*User, error) {
	return &User{ID: 99, Name: name}, nil
}

func main() {
	app := igo.Simple()

	h := newUserHandler(&realUserService{})
	app.Get("/users/:id", h.Get)
	app.Post("/users", h.Create)

	app.PrintRoutes()
	app.Run(":8080")
}
