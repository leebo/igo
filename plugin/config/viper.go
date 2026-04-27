package config

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config Viper 配置客户端
type Config struct {
	*viper.Viper
}

// New 创建配置客户端
func New() *Config {
	return &Config{
		Viper: viper.New(),
	}
}

// FromEnv 从环境变量加载配置
func (c *Config) FromEnv() *Config {
	c.Viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	c.Viper.AutomaticEnv()
	return c
}

// AddConfigPath 添加配置搜索路径
func (c *Config) AddConfigPath(path string) *Config {
	c.Viper.AddConfigPath(path)
	return c
}

// SetConfigName 设置配置文件名（不含扩展名）
func (c *Config) SetConfigName(name string) *Config {
	c.Viper.SetConfigName(name)
	return c
}

// SetConfigType 设置配置类型（yaml, json, toml, env）
func (c *Config) SetConfigType(typ string) *Config {
	c.Viper.SetConfigType(typ)
	return c
}

// ReadConfig 读取配置（适用于字符串）
func (c *Config) ReadConfig(content []byte) error {
	return c.Viper.ReadConfig(bytes.NewReader(content))
}

// Load 加载配置
func (c *Config) Load() error {
	return c.Viper.ReadInConfig()
}

// Get 获取值
func (c *Config) Get(key string) interface{} {
	return c.Viper.Get(key)
}

// GetString 获取字符串
func (c *Config) GetString(key string) string {
	return c.Viper.GetString(key)
}

// GetInt 获取整数
func (c *Config) GetInt(key string) int {
	return c.Viper.GetInt(key)
}

// GetInt64 获取 int64
func (c *Config) GetInt64(key string) int64 {
	return c.Viper.GetInt64(key)
}

// GetFloat 获取浮点数
func (c *Config) GetFloat(key string) float64 {
	return c.Viper.GetFloat64(key)
}

// GetBool 获取布尔值
func (c *Config) GetBool(key string) bool {
	return c.Viper.GetBool(key)
}

// GetStringSlice 获取字符串切片
func (c *Config) GetStringSlice(key string) []string {
	return c.Viper.GetStringSlice(key)
}

// GetStringMap 获取字符串 map
func (c *Config) GetStringMap(key string) map[string]interface{} {
	return c.Viper.GetStringMap(key)
}

// Set 设置值
func (c *Config) Set(key string, value interface{}) {
	c.Viper.Set(key, value)
}

// BindEnv 绑定环境变量
func (c *Config) BindEnv(key string, envVar string) error {
	return c.Viper.BindEnv(key, envVar)
}

// Unmarshal 反序列化到结构体
func (c *Config) Unmarshal(result interface{}) error {
	return c.Viper.Unmarshal(result)
}

// UnmarshalKey 反序列化指定 key 到结构体
func (c *Config) UnmarshalKey(key string, result interface{}) error {
	return c.Viper.UnmarshalKey(key, result)
}

// IsSet 检查 key 是否存在
func (c *Config) IsSet(key string) bool {
	return c.Viper.IsSet(key)
}

// AllSettings 获取所有设置
func (c *Config) AllSettings() map[string]interface{} {
	return c.Viper.AllSettings()
}

// Default 设置默认值
func (c *Config) Default(key string, value interface{}) *Config {
	c.Viper.SetDefault(key, value)
	return c
}

// AppConfig 应用配置结构
type AppConfig struct {
	Server ServerConfig `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis RedisConfig `mapstructure:"redis"`
	JWT JWTConfig `mapstructure:"jwt"`
	Log LogConfig `mapstructure:"log"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Dialect string `mapstructure:"dialect"`
	DSN     string `mapstructure:"dsn"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	SecretKey     string `mapstructure:"secret_key"`
	Expiration    string `mapstructure:"expiration"`
	RefreshExpiry string `mapstructure:"refresh_expiry"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	OutputPath string `mapstructure:"output_path"`
}

// LoadFromFile 从文件加载配置
func LoadFromFile(path, name, typ string) (*AppConfig, error) {
	v := New()
	v.AddConfigPath(path)
	v.SetConfigName(name)
	v.SetConfigType(typ)

	if err := v.Load(); err != nil {
		return nil, fmt.Errorf("load config failed: %w", err)
	}

	var cfg AppConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}

	return &cfg, nil
}

// Validate 验证配置有效性，返回第一个发现的错误
// AI 使用此方法可以在启动前发现配置问题
func (c *AppConfig) Validate() error {
	var errors []string

	// 验证 Server
	if c.Server.Port == "" {
		errors = append(errors, "server.port is required (e.g., ':8080' or '0.0.0.0:8080')")
	}

	// 验证 Database
	if c.Database.Dialect == "" {
		errors = append(errors, "database.dialect is required (e.g., 'mysql', 'postgres', 'sqlite')")
	}
	if c.Database.DSN == "" {
		errors = append(errors, "database.dsn is required (e.g., './app.db' or 'user:pass@tcp(localhost:3306)/db')")
	}

	// 验证 JWT（如果是生产环境）
	if c.JWT.SecretKey == "" {
		errors = append(errors, "jwt.secret_key is required for authentication")
	}
	if c.JWT.SecretKey == "secret" || c.JWT.SecretKey == "your-secret-key" {
		errors = append(errors, "jwt.secret_key should not be a placeholder value in production")
	}

	// 验证 Log
	if c.Log.Level == "" {
		c.Log.Level = "info" // 默认值
	}
	if c.Log.Format == "" {
		c.Log.Format = "console" // 默认值
	}

	if len(errors) > 0 {
		return fmt.Errorf("config validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

// ValidateForProduction 生产环境验证，更严格的检查
func (c *AppConfig) ValidateForProduction() error {
	var errors []string

	// 基本验证
	if err := c.Validate(); err != nil {
		return err
	}

	// 生产环境额外检查
	if c.JWT.SecretKey == "" || len(c.JWT.SecretKey) < 32 {
		errors = append(errors, "jwt.secret_key must be at least 32 characters in production")
	}

	if c.Database.Dialect == "sqlite" {
		errors = append(errors, "sqlite is not recommended for production")
	}

	if c.Log.Level == "debug" {
		errors = append(errors, "log.level 'debug' should not be used in production")
	}

	if len(errors) > 0 {
		return fmt.Errorf("production config validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}
