package types

import (
	"encoding/json"
	"path"
	"reflect"
	"sort"
	"strings"
	"sync"
)

// TypeSchema 类型完整描述，供 CLI、/_ai/schemas 和 OpenAPI 组件共用。
//
// Usage 标注此类型在路由元数据中出现的位置，可同时包含多个值。AI 客户端可据此判断
// 一个类型是请求体（"request"）、响应体（"response"）、查询参数容器（"query"）
// 还是路径参数容器（"path"）。空切片表示未被任何路由直接引用。
type TypeSchema struct {
	Name        string        `json:"name"`
	Package     string        `json:"package,omitempty"`
	FilePath    string        `json:"filePath,omitempty"`
	Fields      []FieldSchema `json:"fields"`
	Description string        `json:"description,omitempty"`
	Usage       []string      `json:"usage,omitempty"`
}

// Schema usage 常量，配合 TypeSchema.Usage 与 TypeRegistry.RegisterTypeUsage 使用。
const (
	UsageRequest  = "request"
	UsageResponse = "response"
	UsageQuery    = "query"
	UsagePath     = "path"
)

// FieldSchema 字段描述。
type FieldSchema struct {
	GoName      string       `json:"goName"`
	JSONName    string       `json:"jsonName"`
	GoType      string       `json:"goType"`
	Type        string       `json:"type"`
	Validate    string       `json:"validate,omitempty"`
	Required    bool         `json:"required"`
	Enum        []string     `json:"enum,omitempty"`
	Min         string       `json:"min,omitempty"`
	Max         string       `json:"max,omitempty"`
	GTE         string       `json:"gte,omitempty"`
	LTE         string       `json:"lte,omitempty"`
	Len         string       `json:"len,omitempty"`
	Default     string       `json:"default,omitempty"`
	Example     any          `json:"example,omitempty"`
	Description string       `json:"description,omitempty"`
	Rules       []RuleSchema `json:"rules,omitempty"`
}

// RuleSchema 结构化规则定义。
type RuleSchema struct {
	Name    string   `json:"name"`
	Params  []string `json:"params,omitempty"`
	Message string   `json:"message,omitempty"`
}

// TypeRegistry 类型注册表。
type TypeRegistry struct {
	mu    sync.RWMutex
	types map[string]*TypeSchema
}

// NewTypeRegistry 创建新的类型注册表。
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{types: make(map[string]*TypeSchema)}
}

// RegisterType 注册类型。
//
// 重复注册同名类型时，新 schema 替换旧 schema，但 Usage 会合并去重，避免在
// 不同绑定路径上注册同一类型时丢失先前已记录的用途。
func (r *TypeRegistry) RegisterType(schema *TypeSchema) {
	if r == nil || schema == nil || schema.Name == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	copySchema := cloneTypeSchema(schema)
	if existing := r.types[schema.Name]; existing != nil {
		copySchema.Usage = mergeUsages(existing.Usage, copySchema.Usage)
	}
	r.types[schema.Name] = &copySchema
}

// RegisterTypeUsage 给已注册类型追加一个或多个 usage 标签，幂等。
// 如果类型尚未注册，本调用会被忽略 —— Usage 必须挂在已知 schema 上。
func (r *TypeRegistry) RegisterTypeUsage(name string, usages ...string) {
	if r == nil || name == "" || len(usages) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.types[name]
	if !ok || existing == nil {
		return
	}
	existing.Usage = mergeUsages(existing.Usage, usages)
}

// GetType 获取类型。
func (r *TypeRegistry) GetType(name string) *TypeSchema {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	schema := r.types[name]
	if schema == nil {
		return nil
	}
	copySchema := cloneTypeSchema(schema)
	return &copySchema
}

// ListTypes 列出所有类型。
func (r *TypeRegistry) ListTypes() []*TypeSchema {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*TypeSchema, 0, len(r.types))
	for _, schema := range r.types {
		copySchema := cloneTypeSchema(schema)
		result = append(result, &copySchema)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Package != result[j].Package {
			return result[i].Package < result[j].Package
		}
		return result[i].Name < result[j].Name
	})
	return result
}

// ExportJSON 导出为 JSON。
func (r *TypeRegistry) ExportJSON() ([]byte, error) {
	return json.MarshalIndent(r.ListTypes(), "", "  ")
}

// ExtractSchema 从结构体提取完整 Schema。
func ExtractSchema(v any) TypeSchema {
	t := reflect.TypeOf(v)
	return ExtractSchemaFromType(t)
}

// ExtractSchemaFromType 从类型提取 Schema。
func ExtractSchemaFromType(t reflect.Type) TypeSchema {
	if t == nil {
		return TypeSchema{}
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return TypeSchema{}
	}

	schema := TypeSchema{
		Name:    t.Name(),
		Package: packageName(t.PkgPath()),
		Fields:  make([]FieldSchema, 0, t.NumField()),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		jsonName, skip := JSONName(field.Name, field.Tag.Get("json"))
		if skip {
			continue
		}
		fieldSchema := BuildFieldSchema(
			field.Name,
			jsonName,
			field.Type.String(),
			field.Tag.Get("validate"),
			field.Tag.Get("description"),
			field.Tag.Get("default"),
			field.Tag.Get("example"),
		)
		schema.Fields = append(schema.Fields, fieldSchema)
	}

	return schema
}

// BuildFieldSchema 从字段名、类型和 tag 构建统一字段 schema。
func BuildFieldSchema(goName, jsonName, goType, validateTag, description, defaultValue string, example any) FieldSchema {
	if jsonName == "" {
		jsonName = goName
	}
	if s, ok := example.(string); ok && s == "" {
		example = nil
	}
	field := FieldSchema{
		GoName:      goName,
		JSONName:    jsonName,
		GoType:      goType,
		Type:        JSONType(goType),
		Validate:    validateTag,
		Description: description,
		Default:     defaultValue,
		Example:     example,
	}
	field.Rules = ParseValidationRules(validateTag)
	for _, rule := range field.Rules {
		value := ""
		if len(rule.Params) > 0 {
			value = rule.Params[0]
		}
		switch rule.Name {
		case "required":
			field.Required = true
		case "enum":
			field.Enum = rule.Params
		case "min":
			field.Min = value
		case "max":
			field.Max = value
		case "gte":
			field.GTE = value
		case "lte":
			field.LTE = value
		case "len":
			field.Len = value
		}
	}
	return field
}

// JSONName 返回 JSON 字段名；第二个返回值表示应跳过该字段。
func JSONName(goName, tag string) (string, bool) {
	if tag == "-" {
		return "", true
	}
	if tag == "" {
		return goName, false
	}
	name := strings.Split(tag, ",")[0]
	if name == "-" {
		return "", true
	}
	if name == "" {
		return goName, false
	}
	return name, false
}

// ParseValidationRules 把 validate tag 解析为结构化规则。
func ParseValidationRules(validateTag string) []RuleSchema {
	if validateTag == "" || validateTag == "-" {
		return nil
	}
	rawRules := strings.Split(validateTag, "|")
	rules := make([]RuleSchema, 0, len(rawRules))
	for _, raw := range rawRules {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parts := strings.SplitN(raw, ":", 2)
		rule := RuleSchema{Name: parts[0]}
		if len(parts) == 2 && parts[1] != "" {
			if parts[0] == "enum" {
				for _, item := range strings.Split(parts[1], ",") {
					item = strings.TrimSpace(item)
					if item != "" {
						rule.Params = append(rule.Params, item)
					}
				}
			} else {
				rule.Params = []string{parts[1]}
			}
		}
		rules = append(rules, rule)
	}
	return rules
}

// JSONType 把 Go 类型映射到 JSON/OpenAPI 基础类型。
func JSONType(goType string) string {
	goType = strings.TrimPrefix(goType, "*")
	switch {
	case strings.HasPrefix(goType, "[]"), strings.HasPrefix(goType, "["):
		return "array"
	case strings.HasPrefix(goType, "map["):
		return "object"
	}
	switch goType {
	case "string":
		return "string"
	case "bool":
		return "boolean"
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return "integer"
	case "float32", "float64":
		return "number"
	default:
		return "object"
	}
}

func cloneTypeSchema(schema *TypeSchema) TypeSchema {
	copySchema := *schema
	copySchema.Fields = append([]FieldSchema(nil), schema.Fields...)
	for i := range copySchema.Fields {
		copySchema.Fields[i].Enum = append([]string(nil), schema.Fields[i].Enum...)
		copySchema.Fields[i].Rules = append([]RuleSchema(nil), schema.Fields[i].Rules...)
		for j := range copySchema.Fields[i].Rules {
			copySchema.Fields[i].Rules[j].Params = append([]string(nil), schema.Fields[i].Rules[j].Params...)
		}
	}
	copySchema.Usage = append([]string(nil), schema.Usage...)
	return copySchema
}

// mergeUsages 合并两组 usage 标签并保持稳定排序与去重。
func mergeUsages(existing, incoming []string) []string {
	if len(existing) == 0 && len(incoming) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	merged := make([]string, 0, len(existing)+len(incoming))
	for _, group := range [][]string{existing, incoming} {
		for _, u := range group {
			if u == "" {
				continue
			}
			if _, ok := seen[u]; ok {
				continue
			}
			seen[u] = struct{}{}
			merged = append(merged, u)
		}
	}
	sort.Strings(merged)
	return merged
}

func packageName(pkgPath string) string {
	if pkgPath == "" {
		return ""
	}
	return path.Base(pkgPath)
}
