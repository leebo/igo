package types

import (
	"encoding/json"
	"reflect"
	"sync"
)

// TypeSchema 类型完整描述 (导出给 AI)
type TypeSchema struct {
	Name        string       `json:"name"`
	Package     string       `json:"package"`
	FilePath    string       `json:"filePath"`
	Fields      []FieldSchema `json:"fields"`
	Description string       `json:"description,omitempty"`
}

// FieldSchema 字段描述
type FieldSchema struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	JSONTag     string      `json:"jsonTag"`
	ValidateTag string      `json:"validateTag"`
	Required    bool        `json:"required"`
	EnumValues  []string    `json:"enumValues,omitempty"`
	Default     string      `json:"default,omitempty"`
	Example     interface{} `json:"example,omitempty"`
	Description string      `json:"description,omitempty"`
	Rules       []RuleSchema `json:"rules,omitempty"`
}

// RuleSchema 结构化规则定义
type RuleSchema struct {
	Name   string   `json:"name"`
	Params []string `json:"params"`
	Message string  `json:"message,omitempty"`
}

// TypeRegistry 类型注册表
type TypeRegistry struct {
	mu    sync.RWMutex
	types map[string]*TypeSchema
}

// GlobalTypeRegistry 全局类型注册表
var GlobalTypeRegistry = NewTypeRegistry()

// NewTypeRegistry 创建新的类型注册表
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		types: make(map[string]*TypeSchema),
	}
}

// RegisterType 注册类型
func (r *TypeRegistry) RegisterType(schema *TypeSchema) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.types[schema.Name] = schema
}

// GetType 获取类型
func (r *TypeRegistry) GetType(name string) *TypeSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.types[name]
}

// ListTypes 列出所有类型
func (r *TypeRegistry) ListTypes() []*TypeSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*TypeSchema, 0, len(r.types))
	for _, schema := range r.types {
		result = append(result, schema)
	}
	return result
}

// ExportJSON 导出为 JSON (供 AI 使用)
func (r *TypeRegistry) ExportJSON() ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return json.MarshalIndent(r.types, "", "  ")
}

// ExtractSchema 从结构体提取完整 Schema
func ExtractSchema(v interface{}) TypeSchema {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	schema := TypeSchema{
		Name:   t.Name(),
		Fields: make([]FieldSchema, 0),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldSchema := FieldSchema{
			Name:        field.Name,
			Type:        field.Type.String(),
			JSONTag:     field.Tag.Get("json"),
			ValidateTag: field.Tag.Get("validate"),
		}

		validateTag := field.Tag.Get("validate")
		fieldSchema.Required = validateTag != "" && !containsString(validateTag, "omitempty")

		schema.Fields = append(schema.Fields, fieldSchema)
	}

	return schema
}

// ExtractSchemaFromType 从类型提取 Schema
func ExtractSchemaFromType(t reflect.Type) TypeSchema {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	schema := TypeSchema{
		Name:   t.Name(),
		Fields: make([]FieldSchema, 0),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldSchema := FieldSchema{
			Name:        field.Name,
			Type:        field.Type.String(),
			JSONTag:     field.Tag.Get("json"),
			ValidateTag: field.Tag.Get("validate"),
		}

		validateTag := field.Tag.Get("validate")
		fieldSchema.Required = validateTag != "" && !containsString(validateTag, "omitempty")

		schema.Fields = append(schema.Fields, fieldSchema)
	}

	return schema
}

// containsString 检查字符串是否包含子串
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// RegisterGlobal 注册类型到全局注册表
func RegisterGlobal(schema *TypeSchema) {
	GlobalTypeRegistry.RegisterType(schema)
}

// GetGlobal 获取全局注册的类型
func GetGlobal(name string) *TypeSchema {
	return GlobalTypeRegistry.GetType(name)
}
