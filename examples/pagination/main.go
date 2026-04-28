// Package main 演示生产级 list endpoint 的标准模板。
//
// AI 学习要点：
//   - 用 BindQuery 把整组 query 参数绑定到结构体（一行）
//   - 用 igo.BindAndValidate 复用：把 ListQuery 也校验一遍
//   - page/size 必须有默认值 + 上限（防止 size=99999 拖垮 DB）
//   - sort 必须用白名单，不能直接拼到 SQL/ORM（防注入）
//   - 返回值标准结构：{ data, total, page, size, pageCount }
//   - 过滤参数用空字符串当作"未指定"，避免 nil 检查地狱
//
// 测试：
//
//	curl 'http://localhost:8080/users?page=1&size=10'
//	curl 'http://localhost:8080/users?status=active&sort=age&order=desc'
//	curl 'http://localhost:8080/users?name=ali&page=2&size=5'
package main

import (
	"sort"
	"strings"

	igo "github.com/igo/igo"
	"github.com/igo/igo/core"
)

// =============================================================================
// 模型 + 内存数据
// =============================================================================

type User struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Age    int    `json:"age"`
	Status string `json:"status"` // active / inactive
}

var allUsers = []User{
	{1, "Alice", "alice@x.com", 28, "active"},
	{2, "Bob", "bob@x.com", 35, "active"},
	{3, "Charlie", "ch@x.com", 42, "inactive"},
	{4, "David", "d@x.com", 19, "active"},
	{5, "Eve", "eve@x.com", 56, "inactive"},
	{6, "Frank", "frank@x.com", 30, "active"},
	{7, "Grace", "grace@x.com", 24, "active"},
	{8, "Henry", "henry@x.com", 67, "inactive"},
}

// =============================================================================
// 标准查询参数结构
// =============================================================================

// ListQuery 是 list endpoint 的标准查询参数。
// 用 igo.BindQueryAndValidate[ListQuery](c) 一次绑定 + 校验
type ListQuery struct {
	// 分页（gte:0 允许 0 触发 applyDefaults，校验失败的负数会自动 422）
	Page int `json:"page" validate:"gte:0"`
	Size int `json:"size" validate:"gte:0|lte:100"`

	// 过滤（空串 = 不过滤）
	Name   string `json:"name" validate:"max:50"`
	Status string `json:"status" validate:"enum:,active,inactive"` // 允许空 + 白名单

	// 排序（具体字段名由 applyDefaults 用 allowedSortFields 二次过滤）
	Sort  string `json:"sort" validate:"max:20"`
	Order string `json:"order" validate:"enum:,asc,desc"` // 允许空 + 仅 asc/desc
}

// allowedSortFields 白名单：防止用户传任意字段做排序（防注入）
var allowedSortFields = map[string]bool{
	"id": true, "name": true, "age": true, "status": true,
}

// applyDefaults 设默认值 + 上下限
func (q *ListQuery) applyDefaults() {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Size <= 0 {
		q.Size = 20
	}
	if q.Size > 100 {
		q.Size = 100 // 上限
	}
	if q.Order == "" {
		q.Order = "asc"
	}
	q.Order = strings.ToLower(q.Order)
	if q.Order != "asc" && q.Order != "desc" {
		q.Order = "asc"
	}
	if q.Sort != "" && !allowedSortFields[q.Sort] {
		q.Sort = "" // 非法字段静默忽略，等同不排序
	}
}

// =============================================================================
// 标准响应结构
// =============================================================================

type ListResponse[T any] struct {
	Data      []T   `json:"data"`
	Total     int   `json:"total"`     // 过滤后的总数
	Page      int   `json:"page"`
	Size      int   `json:"size"`
	PageCount int   `json:"pageCount"` // 总页数，前端方便分页器渲染
}

// =============================================================================
// list endpoint
// =============================================================================

func listUsers(c *core.Context) {
	q, ok := igo.BindQueryAndValidate[ListQuery](c)
	if !ok {
		return // BindQueryAndValidate 已自动发送 400/422
	}
	q.applyDefaults()

	// 1. 过滤
	filtered := make([]User, 0, len(allUsers))
	for _, u := range allUsers {
		if q.Name != "" && !strings.Contains(strings.ToLower(u.Name), strings.ToLower(q.Name)) {
			continue
		}
		if q.Status != "" && u.Status != q.Status {
			continue
		}
		filtered = append(filtered, u)
	}

	// 2. 排序
	if q.Sort != "" {
		sort.SliceStable(filtered, func(i, j int) bool {
			less := compareUsers(filtered[i], filtered[j], q.Sort)
			if q.Order == "desc" {
				return !less
			}
			return less
		})
	}

	total := len(filtered)
	pageCount := (total + q.Size - 1) / q.Size // 向上取整

	// 3. 分页
	start := (q.Page - 1) * q.Size
	end := start + q.Size
	if start >= total {
		filtered = []User{}
	} else {
		if end > total {
			end = total
		}
		filtered = filtered[start:end]
	}

	c.Success(ListResponse[User]{
		Data:      filtered,
		Total:     total,
		Page:      q.Page,
		Size:      q.Size,
		PageCount: pageCount,
	})
}

// compareUsers 按指定字段比较两个 User，返回 a < b
func compareUsers(a, b User, field string) bool {
	switch field {
	case "id":
		return a.ID < b.ID
	case "name":
		return a.Name < b.Name
	case "age":
		return a.Age < b.Age
	case "status":
		return a.Status < b.Status
	}
	return false
}

func main() {
	app := igo.Simple()
	app.Get("/users", listUsers)
	app.PrintRoutes()
	app.Run(":8080")
}
