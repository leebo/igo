// Package handlers 实现 HTTP 处理层
//
// igo:summary: HTTP 处理层 (Handler)
// igo:description: 处理 HTTP 请求和响应，调用 Service 层完成业务逻辑
// igo:tag: handlers
// igo:ai-hint: Handler 层只处理请求/响应，不包含业务逻辑
package handlers

import (
	"strconv"

	"github.com/igo/igo/examples/full/models"
	"github.com/igo/igo/examples/full/services"
	"github.com/igo/igo/core"
)

// UserHandler 用户 HTTP 处理
//
// igo:summary: UserHandler
// igo:description: 处理用户相关的 HTTP 请求
// igo:tag: handlers
type UserHandler struct {
	service *services.UserService
}

// NewUserHandler 创建 UserHandler 实例
//
// igo:summary: 创建 UserHandler
// igo:param:service:*services.UserService:用户服务
// igo:return:*UserHandler:新实例
func NewUserHandler(service *services.UserService) *UserHandler {
	return &UserHandler{service: service}
}

// List 处理 GET /users 请求
//
// igo:summary: List users
// igo:description: 返回用户列表，支持分页和名称搜索
// igo:query-param:page:int:页码,false
// igo:query-param:size:int:每页数量,false
// igo:query-param:name:string:名称搜索,false
// igo:response:200:User list:用户列表
// igo:ai-hint: 调用 service.ListUsers，响应格式为 {data: [...], total: N, page: N, size: N}
func (h *UserHandler) List(c *core.Context) {
	page := c.QueryInt("page", 1)
	size := c.QueryInt("size", 20)
	name := c.Query("name")

	users, total, err := h.service.ListUsers(c.Request.Context(), page, size, name)
	if err != nil {
		c.InternalError("failed to list users")
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
//
// igo:summary: Get user by ID
// igo:description: 返回指定用户的详细信息
// igo:param:id:path:int:用户 ID
// igo:response:200:models.User:用户信息
// igo:response:404:User not found
// igo:ai-hint: 调用 service.GetUserByID，404 时使用 NotFoundWrap
func (h *UserHandler) Get(c *core.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.BadRequest("invalid user id")
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
//
// igo:summary: Create user
// igo:description: 创建新用户
// igo:request-body:CreateUserRequest:创建用户请求体
// igo:response:201:models.User:创建的用户信息
// igo:response:400:Invalid request:请求格式错误
// igo:response:422:Validation error:验证失败
// igo:ai-hint: 调用 service.CreateUser，成功时使用 Created() 返回 201
func (h *UserHandler) Create(c *core.Context) {
	var req models.CreateUserRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
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
//
// igo:summary: Update user
// igo:description: 更新指定用户的信息
// igo:param:id:path:int:用户 ID
// igo:request-body:UpdateUserRequest:更新用户请求体
// igo:response:200:models.User:更新后的用户信息
// igo:response:400:Invalid request:请求格式错误
// igo:response:404:User not found
// igo:ai-hint: 调用 service.UpdateUser，使用 InternalErrorWrap 保留原始错误
func (h *UserHandler) Update(c *core.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.BadRequest("invalid user id")
		return
	}

	var req models.UpdateUserRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	updated, err := h.service.UpdateUser(c.Request.Context(), id, &req)
	if err != nil {
		c.InternalErrorWrap(err, "failed to update user", map[string]any{
			"user_id": id,
		})
		return
	}

	c.Success(updated)
}

// Delete 处理 DELETE /users/:id 请求
//
// igo:summary: Delete user
// igo:description: 删除指定用户
// igo:param:id:path:int:用户 ID
// igo:response:204:No content
// igo:response:404:User not found
// igo:ai-hint: 调用 service.DeleteUser，成功时使用 NoContent() 返回 204
func (h *UserHandler) Delete(c *core.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.BadRequest("invalid user id")
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
