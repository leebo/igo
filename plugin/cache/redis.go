package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config Redis 配置
type Config struct {
	Addr     string // 地址，如 localhost:6379
	Password string // 密码
	DB       int    // 数据库号
}

// Client Redis 客户端
type Client struct {
	rdb *redis.Client
}

// New 创建 Redis 客户端
func New(cfg Config) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// MustNew 创建 Redis 客户端，失败 panic
func MustNew(cfg Config) *Client {
	c, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return c
}

// Close 关闭连接
func (c *Client) Close() error {
	return c.rdb.Close()
}

// CacheItem 带过期时间的缓存项
type CacheItem[T any] struct {
	Value     T
	ExpiredAt time.Time
}

// Get 获取缓存值
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

// GetJSON 获取 JSON 缓存并反序列化
func (c *Client) GetJSON(ctx context.Context, key string, result interface{}) error {
	data, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), result)
}

// Set 设置缓存值
func (c *Client) Set(ctx context.Context, key string, value interface{}) error {
	return c.SetWithExpiry(ctx, key, value, 0)
}

// SetWithExpiry 设置缓存值并指定过期时间
func (c *Client) SetWithExpiry(ctx context.Context, key string, value interface{}, expiry time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, data, expiry).Err()
}

// SetJSON 设置 JSON 缓存
func (c *Client) SetJSON(ctx context.Context, key string, value interface{}) error {
	return c.Set(ctx, key, value)
}

// SetJSONWithExpiry 设置 JSON 缓存并指定过期时间
func (c *Client) SetJSONWithExpiry(ctx context.Context, key string, value interface{}, expiry time.Duration) error {
	return c.SetWithExpiry(ctx, key, value, expiry)
}

// Delete 删除缓存
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Exists 检查 key 是否存在
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}

// Expire 设置过期时间
func (c *Client) Expire(ctx context.Context, key string, expiry time.Duration) error {
	return c.rdb.Expire(ctx, key, expiry).Err()
}

// TTL 获取剩余生存时间
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.rdb.TTL(ctx, key).Result()
}

// Incr 自增
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, key).Result()
}

// IncrBy 增加指定值
func (c *Client) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.rdb.IncrBy(ctx, key, value).Result()
}

// Decr 自减
func (c *Client) Decr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Decr(ctx, key).Result()
}

// CacheService 是缓存服务接口（方便 AI 生成代码）
type CacheService interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}) error
	Delete(ctx context.Context, keys ...string) error
}
