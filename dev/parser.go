// Package dev 提供 igo 开发模式专用工具:文件监听、增量重启、
// 编译错误结构化解析、AI 自省状态(/_ai/dev)。
//
// 所有 dev/ 包内代码只在 IGO_ENV=dev 或通过 cmd/igo dev watcher 启动时使用,
// 不应被 prd 路径上的代码导入。
package dev

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ErrorType 是已识别的 Go 编译错误大类,未识别的归 ErrorTypeUnknown。
type ErrorType string

const (
	ErrorTypeUndefined     ErrorType = "undefined_symbol"
	ErrorTypeMissingImport ErrorType = "missing_import"
	ErrorTypeTypeMismatch  ErrorType = "type_mismatch"
	ErrorTypeSyntax        ErrorType = "syntax"
	ErrorTypeUnknown       ErrorType = "unknown"
)

// StructuredError 是把 go build stderr 解析后的单条错误。
type StructuredError struct {
	File       string    `json:"file"`
	Line       int       `json:"line"`
	Col        int       `json:"col,omitempty"`
	Type       ErrorType `json:"type"`
	Symbol     string    `json:"symbol,omitempty"`
	Message    string    `json:"message"`
	Suggestion string    `json:"suggestion,omitempty"`
	Raw        string    `json:"raw"`
}

// 形如 ./handlers/user.go:42:10: undefined: foo
//      handlers/user.go:42: syntax error: ...
var locRe = regexp.MustCompile(`^(?:\./)?([^:\s]+\.go):(\d+)(?::(\d+))?:\s*(.+)$`)

var (
	undefinedRe = regexp.MustCompile(`^undefined:\s*(\S+)`)

	// importPatterns: any one match means "user is missing or misusing an import".
	// Each pattern is intentionally narrow so we don't false-positive on type errors.
	importPatterns = []*regexp.Regexp{
		regexp.MustCompile(`imported and not used`),
		regexp.MustCompile(`cannot find package`),
		regexp.MustCompile(`no required module`),
		regexp.MustCompile(`cannot find module`),
		regexp.MustCompile(`missing go\.sum entry`),
	}

	// typeMismatchRe is anchored: "cannot use" alone is too broad (matches
	// "cannot use unexported identifier" etc.); we require the canonical
	// "cannot use X as Y" or one of the explicit type-error phrasings.
	typeMismatchRe = regexp.MustCompile(`^cannot use .+ as |type \S+ is not assignable|cannot convert|incompatible types|mismatched types`)

	syntaxRe = regexp.MustCompile(`^syntax error|^expected |unexpected\b`)
)

func matchesAny(s string, patterns []*regexp.Regexp) bool {
	for _, p := range patterns {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}

// ParseBuildErrors 把 go build stderr 解析为结构化错误列表。
// 不能识别行号/文件的行会被丢弃(典型的 "# package/path" 头部行、
// "go: ..." 模块解析说明等)。
func ParseBuildErrors(stderr string) []StructuredError {
	var out []StructuredError
	for _, raw := range strings.Split(stderr, "\n") {
		raw = strings.TrimRight(raw, "\r ")
		if raw == "" {
			continue
		}
		m := locRe.FindStringSubmatch(strings.TrimLeft(raw, "\t "))
		if m == nil {
			continue
		}
		line, _ := strconv.Atoi(m[2])
		col := 0
		if m[3] != "" {
			col, _ = strconv.Atoi(m[3])
		}
		msg := strings.TrimSpace(m[4])
		se := StructuredError{
			File:    filepath.ToSlash(m[1]),
			Line:    line,
			Col:     col,
			Message: msg,
			Raw:     raw,
		}
		classify(&se)
		out = append(out, se)
	}
	return out
}

func classify(se *StructuredError) {
	msg := se.Message
	switch {
	case undefinedRe.MatchString(msg):
		m := undefinedRe.FindStringSubmatch(msg)
		se.Type = ErrorTypeUndefined
		se.Symbol = m[1]
		se.Suggestion = "Define `" + se.Symbol + "`, fix typo, or check imports."
	case matchesAny(msg, importPatterns):
		se.Type = ErrorTypeMissingImport
		se.Suggestion = "Run `go mod tidy` or add the missing import."
	case typeMismatchRe.MatchString(msg):
		se.Type = ErrorTypeTypeMismatch
		se.Suggestion = "Adjust types or add an explicit conversion."
	case syntaxRe.MatchString(msg):
		se.Type = ErrorTypeSyntax
		se.Suggestion = "Check for missing braces, parentheses, or semicolons."
	default:
		se.Type = ErrorTypeUnknown
	}
}
