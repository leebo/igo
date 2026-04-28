// Package handlers 实现 HTTP 处理层
//
// 标准 igo handler 模式（AI 应当模仿此风格）：
//   - 路径参数用 c.ParamInt64OrFail，避免手写 strconv 样板
//   - 请求体用 core.BindAndValidate，绑定 + 校验一步到位
//   - 错误响应一律用 *Wrap 系列保留 err 调用链
//   - Handler 只做参数解析和调用 service，不写业务逻辑
package handlers

import (
	"github.com/igo/igo/core"
	"github.com/igo/igo/examples/full/models"
	"github.com/igo/igo/examples/full/services"
)

// UserHandler 用户 HTTP 处理
type UserHandler struct {
	service *services.UserService
}

// NewUserHandler 创建 UserHandler 实例
func NewUserHandler(service *services.UserService) *UserHandler {
	return &UserHandler{service: service}
}

// List 处理 GET /users 请求
func (h *UserHandler) List(c *core.Context) {
	page := c.QueryInt("page", 1)
	size := c.QueryInt("size", 20)
	name := c.Query("name")

	users, total, err := h.service.ListUsers(c.Request.Context(), page, size, name)
	if err != nil {
		c.InternalErrorWrap(err, "failed to list users", map[string]any{
			"page": page, "size": size, "name": name,
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

// Get 处理 GET /users/:id 请求
func (h *UserHandler) Get(c *core.Context) {
	id, ok := c.ParamInt64OrFail("id")
	if !ok {
		return
	}

	user, err := h.service.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.NotFoundWrap(err, "user not found")
		return
	}

	c.Success(user)
}

// Create 处理 POST /users 请求
func (h *UserHandler) Create(c *core.Context) {
	req, ok := core.BindAndValidate[models.CreateUserRequest](c)
	if !ok {
		return
	}

	user := &models.User{
		Name:  req.Name,
		Email: req.Email,
		Age:   req.Age,
	}

	created, err := h.service.CreateUser(c.Request.Context(), user)
	if err != nil {
		c.InternalErrorWrap(err, "failed to create user", map[string]any{
			"email": req.Email,
		})
		return
	}

	c.Created(created)
}

// Update 处理 PUT /users/:id 请求
func (h *UserHandler) Update(c *core.Context) {
	id, ok := c.ParamInt64OrFail("id")
	if !ok {
		return
	}

	req, ok := core.BindAndValidate[models.UpdateUserRequest](c)
	if !ok {
		return
	}

	updated, err := h.service.UpdateUser(c.Request.Context(), id, req)
	if err != nil {
		c.InternalErrorWrap(err, "failed to update user", map[string]any{
			"user_id": id,
		})
		return
	}

	c.Success(updated)
}

// Delete 处理 DELETE /users/:id 请求
func (h *UserHandler) Delete(c *core.Context) {
	id, ok := c.ParamInt64OrFail("id")
	if !ok {
		return
	}

	if err := h.service.DeleteUser(c.Request.Context(), id); err != nil {
		c.InternalErrorWrap(err, "failed to delete user", map[string]any{
			"user_id": id,
		})
		return
	}

	c.NoContent()
}
