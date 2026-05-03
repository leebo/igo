package schema

import (
	"encoding/json"
	"testing"

	routepkg "github.com/leebo/igo/core/route"
	"github.com/leebo/igo/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateOpenAPIFromRoutesAndSchemas(t *testing.T) {
	userSchema := &types.TypeSchema{
		Name:        "UserResponse",
		Description: "user payload",
		Fields: []types.FieldSchema{
			{GoName: "ID", JSONName: "id", GoType: "int64", Type: "integer", Required: true},
			{GoName: "Name", JSONName: "name", GoType: "string", Type: "string", Required: true, Min: "2", Max: "20", Example: "alice"},
			{GoName: "Role", JSONName: "role", GoType: "string", Type: "string", Enum: []string{"admin", "user"}},
			{GoName: "Tags", JSONName: "tags", GoType: "[]string", Type: "array"},
			{GoName: "Skip", JSONName: "", GoType: "string", Type: "string"},
		},
	}
	createSchema := &types.TypeSchema{Name: "CreateUserRequest", Fields: []types.FieldSchema{
		{GoName: "Name", JSONName: "name", GoType: "string", Type: "string", Required: true, Len: "5"},
	}}
	routes := []*routepkg.RouteConfig{
		{
			Method:      "GET",
			Path:        "/users/:id",
			Summary:     "Get user",
			Description: "fetch one user",
			HandlerName: "getUser",
			Tags:        []string{"users"},
			Params: []routepkg.ParamDefinition{
				{Name: "id", In: "path", Type: "int", Required: false, Description: "user id", GTE: "1"},
				{Name: "expand", In: "query", Type: "string", Enum: []string{"profile"}},
			},
			Responses: []routepkg.ResponseDefinition{{StatusCode: 200, TypeName: "UserResponse"}},
		},
		{
			Method:      "POST",
			Path:        "/users",
			HandlerName: "inline",
			RequestBody: &routepkg.RequestBodyDefinition{TypeName: "CreateUserRequest", Required: true, Example: map[string]any{"name": "alice"}},
			Responses:   []routepkg.ResponseDefinition{{StatusCode: 201, Description: "created", TypeName: "*UserResponse"}},
		},
		{Method: "DELETE", Path: "/users/:id", Responses: []routepkg.ResponseDefinition{{StatusCode: 204}}},
		{Method: "PATCH", Path: "/users/:id"},
		{Method: "OPTIONS", Path: "/users"},
		{Method: "HEAD", Path: "/users/:id"},
		nil,
	}

	spec := NewRouteGenerator(routes, userSchema, createSchema, nil).Generate()

	require.Equal(t, "3.0.0", spec.OpenAPI)
	require.NotNil(t, spec.Components)
	user := spec.Components.Schemas["UserResponse"]
	require.NotNil(t, user)
	assert.Equal(t, "object", user.Type)
	assert.Equal(t, []string{"id", "name"}, user.Required)
	assert.Equal(t, "int64", user.Properties["id"].Format)
	assert.Equal(t, "alice", user.Properties["name"].Example)
	require.NotNil(t, user.Properties["name"].MinLength)
	require.NotNil(t, user.Properties["name"].MaxLength)
	assert.Equal(t, 2, *user.Properties["name"].MinLength)
	assert.Equal(t, 20, *user.Properties["name"].MaxLength)
	assert.Equal(t, []string{"admin", "user"}, user.Properties["role"].Enum)
	require.NotNil(t, user.Properties["tags"].Items)
	assert.Equal(t, "string", user.Properties["tags"].Items.Type)
	assert.NotContains(t, user.Properties, "")

	item := spec.Paths["/users/{id}"]
	require.NotNil(t, item)
	require.NotNil(t, item.GET)
	assert.Equal(t, "getUser", item.GET.OperationID)
	require.Len(t, item.GET.Parameters, 2)
	assert.True(t, item.GET.Parameters[0].Required)
	assert.Equal(t, int64(1), item.GET.Parameters[0].Schema.Minimum)
	assert.Equal(t, []string{"profile"}, item.GET.Parameters[1].Schema.Enum)
	assert.Equal(t, "#/components/schemas/UserResponse", item.GET.Responses["200"].Content["application/json"].Schema.Ref)

	require.NotNil(t, spec.Paths["/users"].POST)
	post := spec.Paths["/users"].POST
	assert.Equal(t, "post_users", post.OperationID)
	assert.Equal(t, "#/components/schemas/CreateUserRequest", post.RequestBody.Content["application/json"].Schema.Ref)
	assert.Equal(t, "created", post.Responses["201"].Description)
	assert.Equal(t, "#/components/schemas/UserResponse", post.Responses["201"].Content["application/json"].Schema.Ref)
	assert.Empty(t, item.DELETE.Responses["204"].Content)
	assert.Equal(t, "success", item.PATCH.Responses["200"].Description)
	assert.NotNil(t, spec.Paths["/users"].OPTIONS)
	assert.NotNil(t, item.HEAD)

	payload, err := json.Marshal(spec)
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"/users/{id}"`)
}

func TestGenerator_SecuritySchemesAddedWhenAuthMiddlewareUsed(t *testing.T) {
	routes := []*routepkg.RouteConfig{
		{
			Method:      "GET",
			Path:        "/me",
			HandlerName: "me",
			Middlewares: []string{"middleware.JWTAuth"},
			Responses:   []routepkg.ResponseDefinition{{StatusCode: 200}},
		},
		{
			Method:      "GET",
			Path:        "/health",
			HandlerName: "health",
			Responses:   []routepkg.ResponseDefinition{{StatusCode: 200}},
		},
	}
	spec := NewRouteGenerator(routes).Generate()

	require.NotNil(t, spec.Components)
	require.NotNil(t, spec.Components.SecuritySchemes)
	scheme := spec.Components.SecuritySchemes["bearerAuth"]
	require.NotNil(t, scheme)
	assert.Equal(t, "http", scheme.Type)
	assert.Equal(t, "bearer", scheme.Scheme)
	assert.Equal(t, "JWT", scheme.BearerFormat)

	// /me 应该带 security，/health 不应带
	me := spec.Paths["/me"].GET
	require.NotNil(t, me.Security)
	assert.Equal(t, []map[string][]string{{"bearerAuth": {}}}, me.Security)

	health := spec.Paths["/health"].GET
	assert.Empty(t, health.Security)
}

func TestGenerator_NoSecuritySchemesWhenNoAuth(t *testing.T) {
	routes := []*routepkg.RouteConfig{
		{Method: "GET", Path: "/health", HandlerName: "health", Responses: []routepkg.ResponseDefinition{{StatusCode: 200}}},
	}
	spec := NewRouteGenerator(routes).Generate()

	if spec.Components != nil {
		assert.Empty(t, spec.Components.SecuritySchemes)
	}
}

func TestSchemaForTypeNameFallbacksAndBounds(t *testing.T) {
	generator := NewRouteGenerator(nil, &types.TypeSchema{Name: "Known"})

	tests := map[string]*Schema{
		"":                          {Type: "object"},
		"string":                    {Type: "string"},
		"bool":                      {Type: "boolean"},
		"int64":                     {Type: "integer", Format: "int64"},
		"[]string":                  {Type: "array", Items: &Schema{Type: "string"}},
		"map[string]any":            {Type: "object"},
		"github.com/app.Known":      {Ref: "#/components/schemas/Known"},
		"Unknown":                   {Type: "object"},
		"core.H":                    {Type: "object"},
		"func(context.Context) err": {Type: "object"},
	}

	for typeName, want := range tests {
		t.Run(typeName, func(t *testing.T) {
			assert.Equal(t, want, generator.schemaForTypeName(typeName))
		})
	}

	stringSchema := &Schema{Type: "string"}
	applyBounds(stringSchema, "2", "bad", "", "", "5")
	require.NotNil(t, stringSchema.MinLength)
	require.NotNil(t, stringSchema.MaxLength)
	assert.Equal(t, 5, *stringSchema.MinLength)
	assert.Equal(t, 5, *stringSchema.MaxLength)

	numberSchema := &Schema{Type: "number"}
	applyBounds(numberSchema, "1.5", "9", "2", "8.5", "")
	assert.Equal(t, int64(2), numberSchema.Minimum)
	assert.Equal(t, 8.5, numberSchema.Maximum)
}
