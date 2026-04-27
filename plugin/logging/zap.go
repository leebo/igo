package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config 日志配置
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, console
	OutputPath string // stdout, stderr, 或文件路径
}

// Client Zap 日志客户端
type Client struct {
	logger *zap.Logger
}

// New 创建日志客户端
func New(cfg Config) (*Client, error) {
	var level zapcore.Level
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	var config zap.Config
	if cfg.Format == "console" {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
	}

	config.Level = zap.NewAtomicLevelAt(level)

	if cfg.OutputPath == "" || cfg.OutputPath == "stdout" {
		config.OutputPaths = []string{"stdout"}
	} else if cfg.OutputPath == "stderr" {
		config.OutputPaths = []string{"stderr"}
	} else {
		config.OutputPaths = []string{cfg.OutputPath}
		config.ErrorOutputPaths = []string{cfg.OutputPath}
	}

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &Client{logger: logger}, nil
}

// MustNew 创建日志客户端，失败 panic
func MustNew(cfg Config) *Client {
	c, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return c
}

// Sugar 获取 SugarLogger（支持结构化日志）
func (c *Client) Sugar() *zap.SugaredLogger {
	return c.logger.Sugar()
}

// Sync 刷新日志
func (c *Client) Sync() error {
	return c.logger.Sync()
}

// Debug 调试日志
func (c *Client) Debug(msg string, fields ...Field) {
	c.logger.Debug(msg, fields...)
}

// Info 信息日志
func (c *Client) Info(msg string, fields ...Field) {
	c.logger.Info(msg, fields...)
}

// Warn 警告日志
func (c *Client) Warn(msg string, fields ...Field) {
	c.logger.Warn(msg, fields...)
}

// Error 错误日志
func (c *Client) Error(msg string, fields ...Field) {
	c.logger.Error(msg, fields...)
}

// Fatal 致命日志
func (c *Client) Fatal(msg string, fields ...Field) {
	c.logger.Fatal(msg, fields...)
}

// With 创建带上下文的日志
func (c *Client) With(fields ...Field) *zap.Logger {
	return c.logger.With(fields...)
}

// Field 是 zap.Field 的别名
type Field = zap.Field

// String 创建字符串字段
func String(key, val string) Field {
	return zap.String(key, val)
}

// Int 创建整数字段
func Int(key string, val int) Field {
	return zap.Int(key, val)
}

// Int64 创建 int64 字段
func Int64(key string, val int64) Field {
	return zap.Int64(key, val)
}

// Float64 创建 float64 字段
func Float64(key string, val float64) Field {
	return zap.Float64(key, val)
}

// Bool 创建布尔字段
func Bool(key string, val bool) Field {
	return zap.Bool(key, val)
}

// Error 创建错误字段
func Error(err error) Field {
	return zap.Error(err)
}

// Any 创建任意类型字段
func Any(key string, val interface{}) Field {
	return zap.Any(key, val)
}

// Strings 创建字符串切片字段
func Strings(key string, val []string) Field {
	return zap.Strings(key, val)
}

// Int64s 创建 int64 切片字段
func Int64s(key string, val []int64) Field {
	return zap.Int64s(key, val)
}

// Object 创建对象字段（需要实现 MarshalLogObject 接口）
func Object(key string, val zapcore.ObjectMarshaler) Field {
	return zap.Object(key, val)
}
