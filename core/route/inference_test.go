package route

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInferenceEngineInferFromFunction(t *testing.T) {
	engine := NewInferenceEngine(nil)

	cfg := engine.InferFromFunction("handlers.GetUser", "GET", "/api/v1/users/:id")

	assert.Equal(t, "GET", cfg.Method)
	assert.Equal(t, "/api/v1/users/:id", cfg.Path)
	assert.Equal(t, []string{"users"}, cfg.Tags)
	assert.Equal(t, "Get user by ID", cfg.Summary)
	require.Len(t, cfg.Params, 1)
	assert.Equal(t, ParamDefinition{Name: "id", In: "path", Type: "int", Required: true}, cfg.Params[0])
}

func TestInferenceOptionsDisableIndividualFields(t *testing.T) {
	engine := NewInferenceEngine(&InferenceOptions{})

	cfg := engine.InferFromFunction("GetUser", "GET", "/users/:id")

	assert.Empty(t, cfg.Tags)
	assert.Empty(t, cfg.Params)
	assert.Empty(t, cfg.Summary)
}

func TestInferTypeFromName(t *testing.T) {
	engine := NewInferenceEngine(nil)

	tests := map[string]string{
		"userID":      "int",
		"page":        "int",
		"email":       "string",
		"enabled":     "bool",
		"price":       "float",
		"unknownSlug": "string",
	}

	for name, want := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, want, engine.inferTypeFromName(name))
		})
	}
}

func TestInferSummaryFromNameAndPath(t *testing.T) {
	engine := NewInferenceEngine(nil)

	tests := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{"listUsers", "GET", "/users", "List users"},
		{"createUser", "POST", "/users", "Create user"},
		{"updateUser", "PUT", "/users/:id", "Update user by ID"},
		{"deleteUser", "DELETE", "/users/:id", "Delete user by ID"},
		{"Index", "GET", "/audit-logs", "List audit logs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, engine.inferSummaryFromName(tt.name, tt.method, tt.path))
		})
	}
}

func TestInferenceHelpers(t *testing.T) {
	assert.Equal(t, []string{"h", "t", "t", "p", "server", "i", "d"}, splitCamelCase("HTTPServerID"))
	assert.Equal(t, "user", singularize("users"))
	assert.Equal(t, "category", singularize("categories"))
	assert.Equal(t, "box", singularize("boxes"))
	assert.Equal(t, "audit logs", resourceNameFromPath("/api/audit-logs/:id"))
	assert.Equal(t, "List", getVerbForMethod("GET", "list"))
	assert.Equal(t, "Create", getVerbForMethod("POST", "anything"))
}

func TestMergeWithInference(t *testing.T) {
	inferred := &RouteConfig{
		Method:  "GET",
		Path:    "/users/:id",
		Summary: "Get user by ID",
		Tags:    []string{"users"},
		Params:  []ParamDefinition{{Name: "id"}},
	}
	explicit := &RouteConfig{
		Summary:     "Fetch user",
		Description: "custom",
		Tags:        []string{"accounts"},
		RequestBody: &RequestBodyDefinition{TypeName: "UserRequest"},
		Responses:   []ResponseDefinition{{StatusCode: 200, TypeName: "UserResponse"}},
		Deprecated:  true,
	}

	merged := MergeWithInference(inferred, explicit)

	assert.Equal(t, "GET", merged.Method)
	assert.Equal(t, "/users/:id", merged.Path)
	assert.Equal(t, "Fetch user", merged.Summary)
	assert.Equal(t, "custom", merged.Description)
	assert.Equal(t, []string{"accounts"}, merged.Tags)
	assert.Equal(t, inferred.Params, merged.Params)
	require.NotNil(t, merged.RequestBody)
	assert.True(t, merged.Deprecated)
	assert.Same(t, inferred, MergeWithInference(inferred, nil))
	assert.Same(t, explicit, MergeWithInference(nil, explicit))
}
