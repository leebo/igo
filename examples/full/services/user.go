// Package services 实现业务逻辑层
//
// igo:summary: 业务逻辑层 (Service)
// igo:description: 封装业务规则，组合多个 Repository，协调业务操作
// igo:tag: services
// igo:ai-hint: Service 层处理业务逻辑，可以组合多个 Repository 调用
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/leebo/igo/examples/full/models"
	"github.com/leebo/igo/examples/full/repositories"
	"github.com/leebo/igo/plugin/cache"
)

// UserService 用户服务
//
// igo:summary: UserService
// igo:description: 处理用户相关的业务逻辑
// igo:ai-hint: 组合 UserRepository 和缓存，提供业务方法
type UserService struct {
	repo  *repositories.UserRepository
	cache *cache.Client
}

// NewUserService 创建 UserService 实例
//
// igo:summary: 创建 UserService
// igo:param:repo:*UserRepository:用户仓库
// igo:param:cache:*cache.Client:缓存客户端，可为 nil
// igo:return:*UserService:新实例
func NewUserService(repo *repositories.UserRepository, cacheClient *cache.Client) *UserService {
	return &UserService{
		repo:  repo,
		cache: cacheClient,
	}
}

// GetUserByID 根据 ID 获取用户
//
// igo:summary: Get user by ID
// igo:description: 先从缓存获取，缓存未命中则从数据库查询并更新缓存
// igo:param:ctx:context.Context:上下文
// igo:param:id:int64:用户 ID
// igo:return:*models.User:用户信息
// igo:return:error:错误信息
// igo:response:200:models.User:用户信息
// igo:response:404:User not found
// igo:ai-hint: 使用缓存降低数据库压力，缓存 key 格式为 "users:{id}"
func (s *UserService) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	cacheKey := fmt.Sprintf("users:%d", id)

	// 尝试从缓存获取
	if s.cache != nil {
		var user models.User
		if err := s.cache.GetJSON(ctx, cacheKey, &user); err == nil {
			return &user, nil
		}
	}

	// 从数据库获取
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	if s.cache != nil {
		s.cache.SetJSONWithExpiry(ctx, cacheKey, user, 10*time.Minute)
	}

	return user, nil
}

// ListUsers 获取用户列表
//
// igo:summary: List users
// igo:description: 返回用户列表，支持分页和名称搜索
// igo:param:ctx:context.Context:上下文
// igo:param:page:int:页码
// igo:param:size:int:每页数量
// igo:param:name:string:名称搜索（可选）
// igo:return:[]models.User:用户列表
// igo:return:int64:总数
// igo:return:error:错误信息
// igo:response:200:User list:用户列表
// igo:ai-hint: 列表结果不缓存（数据量大），按需实现
func (s *UserService) ListUsers(ctx context.Context, page, size int, name string) ([]models.User, int64, error) {
	if name != "" {
		return s.repo.ListByName(ctx, name, page, size)
	}
	return s.repo.List(ctx, page, size)
}

// CreateUser 创建用户
//
// igo:summary: Create user
// igo:description: 创建新用户，检查邮箱唯一性
// igo:param:ctx:context.Context:上下文
// igo:param:user:*models.User:用户数据
// igo:return:*models.User:创建的用户
// igo:return:error:错误信息
// igo:response:201:models.User:创建的用户
// igo:response:400:Email already exists
// igo:ai-hint: 创建前检查邮箱是否已存在
func (s *UserService) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	// 检查邮箱唯一性
	existing, _ := s.repo.FindByEmail(ctx, user.Email)
	if existing != nil {
		return nil, fmt.Errorf("email already exists")
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	// 清除列表缓存
	if s.cache != nil {
		s.cache.Delete(ctx, "users:list:1:20")
	}

	return user, nil
}

// UpdateUser 更新用户
//
// igo:summary: Update user
// igo:description: 更新用户信息，支持部分更新
// igo:param:ctx:context.Context:上下文
// igo:param:id:int64:用户 ID
// igo:param:req:*models.UpdateUserRequest:更新请求
// igo:return:*models.User:更新后的用户
// igo:return:error:错误信息
// igo:response:200:models.User:更新后的用户
// igo:response:404:User not found
// igo:ai-hint: 只更新提供的字段，使用 OmitZero 跳过零值
func (s *UserService) UpdateUser(ctx context.Context, id int64, req *models.UpdateUserRequest) (*models.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 只更新提供的字段
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		// 检查新邮箱是否被占用
		existing, _ := s.repo.FindByEmail(ctx, req.Email)
		if existing != nil && existing.ID != id {
			return nil, fmt.Errorf("email already exists")
		}
		user.Email = req.Email
	}
	if req.Age != 0 {
		user.Age = req.Age
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	// 清除缓存
	if s.cache != nil {
		s.cache.Delete(ctx, fmt.Sprintf("users:%d", id))
		s.cache.Delete(ctx, "users:list:1:20")
	}

	return user, nil
}

// DeleteUser 删除用户
//
// igo:summary: Delete user
// igo:description: 删除指定用户
// igo:param:ctx:context.Context:上下文
// igo:param:id:int64:用户 ID
// igo:return:error:错误信息
// igo:response:204:No content
// igo:response:404:User not found
// igo:ai-hint: 删除后清除相关缓存
func (s *UserService) DeleteUser(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	// 清除缓存
	if s.cache != nil {
		s.cache.Delete(ctx, fmt.Sprintf("users:%d", id))
		s.cache.Delete(ctx, "users:list:1:20")
	}

	return nil
}
