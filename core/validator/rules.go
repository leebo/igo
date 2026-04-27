package validator

import (
	"reflect"
	"strings"

	"github.com/igo/igo/core/errors"
)

// ValidationRule 验证规则接口
type ValidationRule interface {
	Name() string
	Message(fieldName string) string
	Validate(value reflect.Value, params map[string]string) *errors.StructuredError
}

// RuleRegistry 全局规则注册表
type RuleRegistry struct {
	rules map[string]ValidationRule
}

var defaultRegistry *RuleRegistry

func init() {
	defaultRegistry = NewRuleRegistry()
	defaultRegistry.Register(&EnumRule{})
	defaultRegistry.Register(&EqFieldRule{})
}

// NewRuleRegistry 创建新的规则注册表
func NewRuleRegistry() *RuleRegistry {
	return &RuleRegistry{
		rules: make(map[string]ValidationRule),
	}
}

// Register 注册验证规则
func (r *RuleRegistry) Register(rule ValidationRule) {
	r.rules[rule.Name()] = rule
}

// Get 获取验证规则
func (r *RuleRegistry) Get(name string) ValidationRule {
	return r.rules[name]
}

// List 列出所有规则名称
func (r *RuleRegistry) List() []string {
	names := make([]string, 0, len(r.rules))
	for name := range r.rules {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry 返回默认规则注册表
func DefaultRegistry() *RuleRegistry {
	return defaultRegistry
}

// EnumRule 枚举规则
type EnumRule struct{}

func (r *EnumRule) Name() string { return "enum" }
func (r *EnumRule) Message(fieldName string) string {
	return fieldName + " must be one of the allowed values"
}
func (r *EnumRule) Validate(value reflect.Value, params map[string]string) *errors.StructuredError {
	str := value.String()
	for _, v := range params {
		if str == v {
			return nil
		}
	}
	return errors.NewValidationError(r.Name(), r.Name(), r.Message("field"))
}

// EqFieldRule 字段相等规则
type EqFieldRule struct{}

func (r *EqFieldRule) Name() string { return "eqfield" }
func (r *EqFieldRule) Message(fieldName string) string {
	return fieldName + " must be equal to the specified field"
}
func (r *EqFieldRule) Validate(value reflect.Value, params map[string]string) *errors.StructuredError {
	return nil
}

// ValidateValue 验证单个值
func ValidateValue(value reflect.Value, rules []string, fieldName string, ruleRegistry *RuleRegistry) *errors.StructuredError {
	if ruleRegistry == nil {
		ruleRegistry = defaultRegistry
	}

	for _, rule := range rules {
		parts := strings.SplitN(rule, ":", 2)
		name := parts[0]

		rule := ruleRegistry.Get(name)
		if rule == nil {
			continue
		}

		params := make(map[string]string)
		if len(parts) > 1 {
			params["0"] = parts[1]
		}

		if err := rule.Validate(value, params); err != nil {
			return err
		}
	}

	return nil
}

// ParseValidationTag 解析验证 tag
func ParseValidationTag(tag string) []string {
	if tag == "" {
		return nil
	}
	return strings.Split(tag, "|")
}
