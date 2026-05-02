package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leebo/igo/ai/schema"
	routepkg "github.com/leebo/igo/core/route"
	"github.com/leebo/igo/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadStaticProjectRoutes(t *testing.T) {
	dir := t.TempDir()
	src := `package sample

import "github.com/leebo/igo/core"

type App struct{}
type CreateUserRequest struct {
	Name string ` + "`json:\"name\" validate:\"required|min:2\"`" + `
	Role string ` + "`json:\"role\" validate:\"enum:admin,user\" description:\"account role\"`" + `
}
type ListUsersQuery struct {
	Page int ` + "`json:\"page\" validate:\"gte:1|lte:100\"`" + `
}
type UserPathParams struct {
	ID int64 ` + "`json:\"id\" validate:\"required|gte:1\"`" + `
}

func RegisterRoutes(app *App) {
	app.Group("/api/v1", func(v1 *App) {
		v1.Get("/users", listUsers)
		v1.Get("/users/:id", getUser, authMiddleware)
		v1.Post("/users", createUser)
	}, groupMiddleware)
}

func authMiddleware(c *core.Context) {}
func groupMiddleware(c *core.Context) {}

func getUser(c *core.Context) {
	params, ok := core.BindPathAndValidate[UserPathParams](c)
	if !ok {
		return
	}
	_ = params
	verbose := c.QueryBool("verbose", false)
	_ = verbose
	c.Success(core.H{"id": params.ID})
}

func listUsers(c *core.Context) {
	q, ok := core.BindQueryAndValidate[ListUsersQuery](c)
	if !ok {
		return
	}
	_ = q
	c.Success([]core.H{{"id": 1}})
}

func createUser(c *core.Context) {
	req, ok := core.BindAndValidate[CreateUserRequest](c)
	if !ok {
		return
	}
	_ = req
	c.Created(core.H{"ok": true})
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "routes.go"), []byte(src), 0644))

	project, err := loadStaticProject(dir)
	require.NoError(t, err)
	require.Len(t, project.routes, 3)

	get := findStaticRoute(project.routes, "GET", "/api/v1/users/:id")
	assert.Equal(t, "GET", get.Method)
	assert.Equal(t, "/api/v1/users/:id", get.Path)
	assert.Equal(t, []string{"groupMiddleware", "authMiddleware"}, get.Middlewares)
	require.Len(t, get.Params, 2)
	require.NotEmpty(t, get.Responses)
	assert.Equal(t, 200, get.Responses[len(get.Responses)-1].StatusCode)

	list := findStaticRoute(project.routes, "GET", "/api/v1/users")
	require.Len(t, list.Params, 1)
	assert.Equal(t, "page", list.Params[0].Name)
	assert.Equal(t, "1", list.Params[0].GTE)
	assert.Equal(t, "100", list.Params[0].LTE)

	post := findStaticRoute(project.routes, "POST", "/api/v1/users")
	assert.Equal(t, "POST", post.Method)
	require.NotNil(t, post.RequestBody)
	assert.Equal(t, "CreateUserRequest", post.RequestBody.TypeName)
	require.NotEmpty(t, post.Responses)
	assert.Equal(t, 201, post.Responses[0].StatusCode)
	assert.True(t, hasStaticSchema(project.schemas, "CreateUserRequest"))
	assert.True(t, hasStaticSchema(project.schemas, "ListUsersQuery"))

	spec := schema.NewRouteGenerator(project.routes, project.schemas...).Generate()
	require.NotNil(t, spec.Components)
	require.NotNil(t, spec.Components.Schemas["CreateUserRequest"])
	bodyRef := spec.Paths["/api/v1/users"].POST.RequestBody.Content["application/json"].Schema.Ref
	assert.Equal(t, "#/components/schemas/CreateUserRequest", bodyRef)
}

func TestRunAINewCommands(t *testing.T) {
	var out bytes.Buffer
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	code := runAI([]string{"errors"})
	w.Close()
	os.Stdout = oldStdout
	assert.Equal(t, 0, code)
	_, err = out.ReadFrom(r)
	require.NoError(t, err)
	var payload []map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &payload), out.String())
	require.NotEmpty(t, payload)
	assert.NotEmpty(t, payload[0]["code"])
}

func findStaticRoute(routes []*routepkg.RouteConfig, method, path string) *routepkg.RouteConfig {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return route
		}
	}
	return &routepkg.RouteConfig{}
}

func hasStaticSchema(schemas []*types.TypeSchema, name string) bool {
	for _, schema := range schemas {
		if schema.Name == name {
			return true
		}
	}
	return false
}
