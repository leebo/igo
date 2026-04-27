// Package config 定义配置加载
//
// igo:summary: 配置加载层
// igo:description: 管理应用配置，支持从文件和环境变量加载
// igo:tag: config
package config

import (
	"github.com/igo/igo/plugin/config"
)

// LoadConfig 加载配置
//
// igo:summary: Load config
// igo:description: 从配置文件和默认配置加载应用配置
// igo:return:*config.AppConfig:应用配置
// igo:ai-hint: 先尝试从文件加载，失败则使用默认配置
func LoadConfig() *config.AppConfig {
	cfg, err := config.LoadFromFile("./", "config", "yaml")
	if err != nil {
		return DefaultConfig()
	}
	return cfg
}

// DefaultConfig 返回默认配置
//
// igo:summary: Default config
// igo:description: 返回开发环境默认配置
// igo:return:*config.AppConfig:默认配置
func DefaultConfig() *config.AppConfig {
	return &config.AppConfig{
		Server: config.ServerConfig{
			Port: ":8080",
		},
		Database: config.DatabaseConfig{
			Dialect: "sqlite",
			DSN:     "./test.db",
		},
		Redis: config.RedisConfig{
			Addr: "localhost:6379",
		},
		JWT: config.JWTConfig{
			SecretKey:  "secret",
			Expiration: "24h",
		},
		Log: config.LogConfig{
			Level:  "info",
			Format: "console",
		},
	}
}
