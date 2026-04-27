// Package models 定义数据模型
//
// igo:summary: 数据模型层
// igo:description: 存放应用中所有的结构体定义，对应数据库表结构
// igo:tag: models
// igo:ai-hint: Model 层只定义数据结构，不包含业务逻辑
package models

// User 是用户模型
//
// igo:summary: User 模型
// igo:description: 包含用户的基本信息，用于数据库存储和 API 响应
// igo:ai-hint: 此结构体对应 users 表，gorm 标签用于数据库字段映射
type User struct {
	ID    int64  `json:"id" gorm:"primaryKey"`
	Name  string `json:"name" gorm:"size:50;not null"`
	Email string `json:"email" gorm:"size:100;uniqueIndex"`
	Age   int    `json:"age"`
}

// CreateUserRequest 创建用户请求
//
// igo:summary: 创建用户请求体
// igo:description: 包含创建用户所需的字段
// igo:ai-hint: 使用 validate tag 进行请求验证
type CreateUserRequest struct {
	Name  string `json:"name" validate:"required|min:2|max:50"`
	Email string `json:"email" validate:"required|email"`
	Age   int    `json:"age" validate:"gte:0|lte:150"`
}

// UpdateUserRequest 更新用户请求
//
// igo:summary: 更新用户请求体
// igo:description: 包含更新用户可修改的字段
// igo:ai-hint: 所有字段可选，不提供的字段不会更新
type UpdateUserRequest struct {
	Name  string `json:"name" validate:"omitempty|min:2|max:50"`
	Email string `json:"email" validate:"omitempty|email"`
	Age   int    `json:"age" validate:"omitempty|gte:0|lte:150"`
}
