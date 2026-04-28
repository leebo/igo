// Package repositories 实现数据访问层
//
// igo:summary: 数据访问层 (Repository)
// igo:description: 封装所有数据库操作，向上层提供服务
// igo:tag: repositories
// igo:ai-hint: Repository 层只负责数据读写，不包含业务逻辑
package repositories

import (
	"context"

	"github.com/leebo/igo/examples/full/models"
	"github.com/leebo/igo/plugin/database"
	"gorm.io/gorm"
)

// UserRepository 用户数据访问层
//
// igo:summary: UserRepository
// igo:description: 提供用户数据的 CRUD 操作
// igo:ai-hint: 组合了 BaseRepository，提供 List, GetByID, Create, Update, Delete 方法
type UserRepository struct {
	*database.BaseRepository[models.User]
}

// NewUserRepository 创建 UserRepository 实例
//
// igo:summary: 创建 UserRepository
// igo:param:db:*gorm.DB:数据库连接
// igo:return:*UserRepository:新实例
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		BaseRepository: database.NewRepository[models.User](db, "users"),
	}
}

// FindByEmail 根据邮箱查找用户
//
// igo:summary: 根据邮箱查找用户
// igo:param:email:string:用户邮箱
// igo:return:*models.User:用户指针，未找到返回 nil
// igo:response:200:models.User:用户信息
// igo:response:404:User not found
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	result := r.DB.WithContext(ctx).Where("email = ?", email).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

// ListByName 根据名称模糊搜索用户
//
// igo:summary: 根据名称搜索用户
// igo:param:name:string:用户名称（模糊匹配）
// igo:param:page:int:页码
// igo:param:size:int:每页数量
// igo:return:[]models.User:用户列表
// igo:return:int64:总数
func (r *UserRepository) ListByName(ctx context.Context, name string, page, size int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	query := r.DB.WithContext(ctx).Model(&models.User{})
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	query.Count(&total)
	offset := (page - 1) * size
	result := query.Offset(offset).Limit(size).Find(&users)

	return users, total, result.Error
}
