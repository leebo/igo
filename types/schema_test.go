package types

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type schemaTestUser struct {
	ID       int64    `json:"id" validate:"required|gte:1"`
	Name     string   `json:"name,omitempty" validate:"required|min:2|max:20" description:"display name" default:"guest" example:"alice"`
	Role     string   `json:"role" validate:"enum:admin,user"`
	Tags     []string `json:"tags"`
	Ignored  string   `json:"-"`
	private  string
	Optional *string `json:",omitempty"`
}

func TestTypeRegistryCopiesAndSortsSchemas(t *testing.T) {
	registry := NewTypeRegistry()
	registry.RegisterType(&TypeSchema{Name: "User", Package: "models", Fields: []FieldSchema{{
		GoName: "Role",
		Enum:   []string{"admin"},
		Rules:  []RuleSchema{{Name: "enum", Params: []string{"admin"}}},
	}}})
	registry.RegisterType(&TypeSchema{Name: "Account", Package: "models"})

	got := registry.GetType("User")
	require.NotNil(t, got)
	got.Fields[0].Enum[0] = "mutated"
	got.Fields[0].Rules[0].Params[0] = "mutated"

	again := registry.GetType("User")
	require.NotNil(t, again)
	assert.Equal(t, []string{"admin"}, again.Fields[0].Enum)
	assert.Equal(t, []string{"admin"}, again.Fields[0].Rules[0].Params)

	list := registry.ListTypes()
	require.Len(t, list, 2)
	assert.Equal(t, "Account", list[0].Name)
	assert.Equal(t, "User", list[1].Name)

	payload, err := registry.ExportJSON()
	require.NoError(t, err)
	assert.True(t, json.Valid(payload))
}

func TestTypeRegistryIgnoresInvalidInput(t *testing.T) {
	var nilRegistry *TypeRegistry
	assert.Nil(t, nilRegistry.GetType("User"))
	assert.Nil(t, nilRegistry.ListTypes())

	registry := NewTypeRegistry()
	registry.RegisterType(nil)
	registry.RegisterType(&TypeSchema{})
	assert.Equal(t, 0, len(registry.ListTypes()))
}

func TestExtractSchemaFromStruct(t *testing.T) {
	schema := ExtractSchema(&schemaTestUser{})

	assert.Equal(t, "schemaTestUser", schema.Name)
	assert.Equal(t, "types", schema.Package)

	require.Len(t, schema.Fields, 5)
	fields := map[string]FieldSchema{}
	for _, field := range schema.Fields {
		fields[field.JSONName] = field
	}

	id := fields["id"]
	assert.Equal(t, "ID", id.GoName)
	assert.Equal(t, "integer", id.Type)
	assert.True(t, id.Required)
	assert.Equal(t, "1", id.GTE)

	name := fields["name"]
	assert.Equal(t, "string", name.Type)
	assert.Equal(t, "display name", name.Description)
	assert.Equal(t, "guest", name.Default)
	assert.Equal(t, "alice", name.Example)
	assert.Equal(t, "2", name.Min)
	assert.Equal(t, "20", name.Max)

	role := fields["role"]
	assert.Equal(t, []string{"admin", "user"}, role.Enum)
	assert.Contains(t, fields, "Optional")
	assert.NotContains(t, fields, "Ignored")
}

func TestExtractSchemaRejectsNonStructs(t *testing.T) {
	assert.Equal(t, TypeSchema{}, ExtractSchema(nil))
	assert.Equal(t, TypeSchema{}, ExtractSchema(42))
	assert.Equal(t, TypeSchema{}, ExtractSchemaFromType(reflect.TypeOf((*int)(nil))))
}

func TestBuildFieldSchemaAndHelpers(t *testing.T) {
	field := BuildFieldSchema("Age", "", "int", "required|gte:18|lte:99|len:2", "age", "", "")

	assert.Equal(t, "Age", field.JSONName)
	assert.Equal(t, "integer", field.Type)
	assert.True(t, field.Required)
	assert.Equal(t, "18", field.GTE)
	assert.Equal(t, "99", field.LTE)
	assert.Equal(t, "2", field.Len)
	assert.Nil(t, field.Example)

	assert.Equal(t, []RuleSchema{
		{Name: "required"},
		{Name: "enum", Params: []string{"admin", "user"}},
		{Name: "regex", Params: []string{"^a:b$"}},
	}, ParseValidationRules("required|enum:admin, user|regex:^a:b$"))

	name, skip := JSONName("Field", "custom,omitempty")
	assert.Equal(t, "custom", name)
	assert.False(t, skip)

	name, skip = JSONName("Field", "-")
	assert.Empty(t, name)
	assert.True(t, skip)

	assert.Equal(t, "array", JSONType("[]string"))
	assert.Equal(t, "object", JSONType("map[string]any"))
	assert.Equal(t, "boolean", JSONType("*bool"))
	assert.Equal(t, "number", JSONType("float64"))
	assert.Equal(t, "object", JSONType("Custom"))
}

func TestGlobalRegistryCompatibility(t *testing.T) {
	old := GlobalTypeRegistry
	GlobalTypeRegistry = NewTypeRegistry()
	t.Cleanup(func() { GlobalTypeRegistry = old })

	RegisterGlobal(&TypeSchema{Name: "GlobalUser"})
	got := GetGlobal("GlobalUser")
	require.NotNil(t, got)
	assert.Equal(t, "GlobalUser", got.Name)
}
