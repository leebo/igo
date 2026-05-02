package core

import (
	"os"
	"strings"
)

// Mode 表示 igo 应用的运行环境模式。
//
// 通过环境变量 IGO_ENV 控制:
//   - "prd" / "prod" / "production"  -> ModePrd
//   - "test" / "testing"             -> ModeTest
//   - 未设置 / 其他值                 -> ModeDev (默认)
//
// 不同 mode 影响 igo.Simple() 的中间件默认值、/_ai/* 端点暴露、
// 错误响应详细度等行为。具体见 CLAUDE.md「环境模式」章节。
type Mode string

const (
	ModeDev  Mode = "dev"
	ModeTest Mode = "test"
	ModePrd  Mode = "prd"
)

// DetectMode 从 IGO_ENV 读取并归一化为 Mode。未设置或无法识别时返回 ModeDev。
func DetectMode() Mode {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("IGO_ENV"))) {
	case "prd", "prod", "production":
		return ModePrd
	case "test", "testing":
		return ModeTest
	default:
		return ModeDev
	}
}

func (m Mode) IsDev() bool  { return m == ModeDev }
func (m Mode) IsTest() bool { return m == ModeTest }
func (m Mode) IsPrd() bool  { return m == ModePrd }
