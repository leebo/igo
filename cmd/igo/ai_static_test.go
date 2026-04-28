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
	if err := os.WriteFile(filepath.Join(dir, "routes.go"), []byte(src), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	project, err := loadStaticProject(dir)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	if len(project.routes) != 3 {
		t.Fatalf("expected 3 routes, got %d: %+v", len(project.routes), project.routes)
	}

	get := findStaticRoute(project.routes, "GET", "/api/v1/users/:id")
	if get.Method != "GET" || get.Path != "/api/v1/users/:id" {
		t.Fatalf("unexpected first route: %s %s", get.Method, get.Path)
	}
	if len(get.Middlewares) != 2 || get.Middlewares[0] != "groupMiddleware" || get.Middlewares[1] != "authMiddleware" {
		t.Fatalf("middlewares not inferred: %+v", get.Middlewares)
	}
	if len(get.Params) != 2 {
		t.Fatalf("expected path and query params, got %+v", get.Params)
	}
	if len(get.Responses) == 0 || get.Responses[len(get.Responses)-1].StatusCode != 200 {
		t.Fatalf("success response not inferred: %+v", get.Responses)
	}

	list := findStaticRoute(project.routes, "GET", "/api/v1/users")
	if len(list.Params) != 1 || list.Params[0].Name != "page" || list.Params[0].GTE != "1" || list.Params[0].LTE != "100" {
		t.Fatalf("BindQueryAndValidate params not inferred: %+v", list.Params)
	}

	post := findStaticRoute(project.routes, "POST", "/api/v1/users")
	if post.Method != "POST" || post.RequestBody == nil || post.RequestBody.TypeName != "CreateUserRequest" {
		t.Fatalf("request body not inferred: %+v", post)
	}
	if len(post.Responses) == 0 || post.Responses[0].StatusCode != 201 {
		t.Fatalf("created response not inferred: %+v", post.Responses)
	}
	if !hasStaticSchema(project.schemas, "CreateUserRequest") || !hasStaticSchema(project.schemas, "ListUsersQuery") {
		t.Fatalf("schemas not collected: %+v", project.schemas)
	}

	spec := schema.NewRouteGenerator(project.routes, project.schemas...).Generate()
	if spec.Components == nil || spec.Components.Schemas["CreateUserRequest"] == nil {
		t.Fatalf("openapi components missing CreateUserRequest: %+v", spec.Components)
	}
	bodyRef := spec.Paths["/api/v1/users"].POST.RequestBody.Content["application/json"].Schema.Ref
	if bodyRef != "#/components/schemas/CreateUserRequest" {
		t.Fatalf("request body ref = %q", bodyRef)
	}
}

func TestRunAINewCommands(t *testing.T) {
	var out bytes.Buffer
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	code := runAI([]string{"errors"})
	w.Close()
	os.Stdout = oldStdout
	if code != 0 {
		t.Fatalf("runAI errors code = %d", code)
	}
	if _, err := out.ReadFrom(r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	var payload []map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("errors output is not json: %v\n%s", err, out.String())
	}
	if len(payload) == 0 || payload[0]["code"] == "" {
		t.Fatalf("errors output missing codes: %+v", payload)
	}
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
