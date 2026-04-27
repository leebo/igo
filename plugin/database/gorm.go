package database

import (
	"context"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config 数据库配置
type Config struct {
	Dialect string // mysql, postgres, sqlite
	DSN     string // 连接字符串
}

// Open 打开数据库连接
func Open(cfg Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Dialect {
	case "mysql":
		dialector = mysql.Open(cfg.DSN)
	case "postgres":
		dialector = postgres.Open(cfg.DSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", cfg.Dialect)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}

	return db, nil
}

// MustOpen 打开数据库连接，失败 panic
func MustOpen(cfg Config) *gorm.DB {
	db, err := Open(cfg)
	if err != nil {
		panic(err)
	}
	return db
}

// Repository 是通用的数据仓储接口
type Repository[T any] interface {
	Create(ctx context.Context, entity *T) error
	GetByID(ctx context.Context, id int64) (*T, error)
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, page, size int) ([]T, int64, error)
}

// BaseRepository 提供基础的 CRUD 实现
type BaseRepository[T any] struct {
	DB    *gorm.DB
	Table string
}

// NewRepository 创建新的仓储实例
func NewRepository[T any](db *gorm.DB, table string) *BaseRepository[T] {
	return &BaseRepository[T]{
		DB:    db,
		Table: table,
	}
}

// Create 创建记录
func (r *BaseRepository[T]) Create(ctx context.Context, entity *T) error {
	return r.DB.WithContext(ctx).Table(r.Table).Create(entity).Error
}

// GetByID 根据 ID 获取记录
func (r *BaseRepository[T]) GetByID(ctx context.Context, id int64) (*T, error) {
	var entity T
	err := r.DB.WithContext(ctx).Table(r.Table).First(&entity, id).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// Update 更新记录
func (r *BaseRepository[T]) Update(ctx context.Context, entity *T) error {
	return r.DB.WithContext(ctx).Table(r.Table).Save(entity).Error
}

// Delete 删除记录
func (r *BaseRepository[T]) Delete(ctx context.Context, id int64) error {
	return r.DB.WithContext(ctx).Table(r.Table).Delete(new(T), id).Error
}

// List 分页查询
func (r *BaseRepository[T]) List(ctx context.Context, page, size int) ([]T, int64, error) {
	var entities []T
	var total int64

	offset := (page - 1) * size

	query := r.DB.WithContext(ctx).Table(r.Table)
	query.Count(&total)

	err := query.Offset(offset).Limit(size).Find(&entities).Error
	if err != nil {
		return nil, 0, err
	}

	return entities, total, nil
}

// FindOne 查询单条记录
func (r *BaseRepository[T]) FindOne(ctx context.Context, query string, args ...interface{}) (*T, error) {
	var entity T
	err := r.DB.WithContext(ctx).Table(r.Table).Where(query, args...).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// Find 查询多条记录
func (r *BaseRepository[T]) Find(ctx context.Context, query string, args ...interface{}) ([]T, error) {
	var entities []T
	err := r.DB.WithContext(ctx).Table(r.Table).Where(query, args...).Find(&entities).Error
	return entities, err
}
