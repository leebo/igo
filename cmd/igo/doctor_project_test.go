package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTempProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		full := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
	return dir
}

func TestCheckResponseTypeNotRegistered_Triggers(t *testing.T) {
	dir := writeTempProject(t, map[string]string{
		"main.go": `package x
type App struct{}
func (a *App) Get(path string, h func(c *Context))   {}
type Context struct{}
func (c *Context) Success(v interface{}) {}

func setup(app *App) {
	app.Get("/x", listX)
}

func listX(c *Context) {
	c.Success(MissingDTO{ID: 1})
}
`,
	})
	project, err := loadStaticProject(dir)
	require.NoError(t, err)
	diags := checkResponseTypeNotRegistered(project)
	require.NotEmpty(t, diags)
	assert.Equal(t, "response-type-not-registered", diags[0].Rule)
	assert.Contains(t, diags[0].Message, "MissingDTO")
}

func TestCheckResponseTypeNotRegistered_OKWhenDefined(t *testing.T) {
	dir := writeTempProject(t, map[string]string{
		"main.go": `package x
type App struct{}
func (a *App) Get(path string, h func(c *Context))   {}
type Context struct{}
func (c *Context) Success(v interface{}) {}

type UserResponse struct {
	ID int64 ` + "`json:\"id\"`" + `
}

func setup(app *App) { app.Get("/x", listX) }
func listX(c *Context) { c.Success(UserResponse{ID: 1}) }
`,
	})
	project, err := loadStaticProject(dir)
	require.NoError(t, err)
	diags := checkResponseTypeNotRegistered(project)
	assert.Empty(t, diags)
}

func TestCheckInvalidValidateTag_Triggers(t *testing.T) {
	dir := writeTempProject(t, map[string]string{
		"types.go": `package x

type CreateRequest struct {
	Name string ` + "`json:\"name\" validate:\"reqired|min:2\"`" + `
}
`,
	})
	project, err := loadStaticProject(dir)
	require.NoError(t, err)
	diags := checkInvalidValidateTag(project)
	require.NotEmpty(t, diags)
	found := false
	for _, d := range diags {
		if d.Rule == "unknown-validate-rule" {
			assert.Contains(t, d.Message, "reqired")
			found = true
		}
	}
	assert.True(t, found)
}

func TestCheckInvalidValidateTag_AllValid(t *testing.T) {
	dir := writeTempProject(t, map[string]string{
		"types.go": `package x

type CreateRequest struct {
	Name string ` + "`json:\"name\" validate:\"required|min:2|max:50\"`" + `
}
`,
	})
	project, err := loadStaticProject(dir)
	require.NoError(t, err)
	diags := checkInvalidValidateTag(project)
	assert.Empty(t, diags)
}

func TestCheckSchemaUnusedInRoutes_Triggers(t *testing.T) {
	dir := writeTempProject(t, map[string]string{
		"main.go": `package x
type App struct{}
type Context struct{}

type UnusedDTO_Request struct{ Name string }
type UsedRequest struct{ Name string }

func (a *App) Post(path string, h func(c *Context)) {}
func (c *Context) Success(v interface{})            {}

func BindAndValidate[T any](c *Context) (*T, bool)  { var z T; return &z, true }

func setup(app *App) { app.Post("/x", h) }
func h(c *Context) {
	_, ok := BindAndValidate[UsedRequest](c)
	if !ok { return }
	c.Success(nil)
}
`,
	})
	project, err := loadStaticProject(dir)
	require.NoError(t, err)
	diags := checkSchemaUnusedInRoutes(project)
	hasUnused := false
	for _, d := range diags {
		if d.Rule == "schema-unused" {
			assert.Contains(t, d.Message, "UnusedDTO_Request")
			hasUnused = true
		}
	}
	assert.True(t, hasUnused)
}
