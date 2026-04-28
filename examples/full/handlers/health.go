// Package handlers 实现 HTTP 处理层
//
// igo:summary: HTTP 处理层 (Handler)
// igo:description: 处理 HTTP 请求和响应
// igo:tag: handlers
package handlers

import (
	"github.com/leebo/igo/core"
)

// Health 健康检查处理
//
// igo:summary: Health handler
// igo:description: 提供服务健康检查接口
// igo:tag: handlers
type Health struct{}

// NewHealth 创建 Health handler
func NewHealth() *Health {
	return &Health{}
}

// Check 处理 GET /health 请求
//
// igo:summary: Health check
// igo:description: 返回服务健康状态
// igo:response:200:Health status:健康状态
// igo:ai-hint: 用于 k8s readiness/liveness probe 或负载均衡健康检查
func (h *Health) Check(c *core.Context) {
	c.Success(core.H{"status": "ok"})
}
