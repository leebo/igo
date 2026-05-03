package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runValidateAndDecode(t *testing.T, dir string) (ValidateReport, int) {
	t.Helper()
	var buf bytes.Buffer
	code := runAIValidate(dir, &buf)
	var report ValidateReport
	require.NoError(t, json.Unmarshal(buf.Bytes(), &report), buf.String())
	return report, code
}

func TestAIValidate_ReportsMissingResponseDTO(t *testing.T) {
	dir := writeTempProject(t, map[string]string{
		"main.go": `package x
type App struct{}
func (a *App) Get(path string, h func(c *Context))   {}
type Context struct{}
func (c *Context) Success(v interface{}) {}

func setup(app *App) { app.Get("/x", listX) }
func listX(c *Context) { c.Success(MissingDTO{ID: 1}) }
`,
	})
	report, code := runValidateAndDecode(t, dir)
	assert.False(t, report.OK)
	assert.Equal(t, 1, code)
	codes := []string{}
	for _, it := range report.Issues {
		codes = append(codes, it.Code)
	}
	assert.Contains(t, codes, "response-type-not-registered")
}

func TestAIValidate_OKOnCleanProject(t *testing.T) {
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
	report, code := runValidateAndDecode(t, dir)
	assert.True(t, report.OK, "issues: %+v", report.Issues)
	assert.Equal(t, 0, code)
	assert.Equal(t, 1, report.RouteCount)
	assert.Equal(t, 1, report.SchemaCount)
}

func TestAIValidate_FlagsRouteWithoutSuccessResponse(t *testing.T) {
	dir := writeTempProject(t, map[string]string{
		"main.go": `package x
type App struct{}
func (a *App) Post(path string, h func(c *Context))  {}
type Context struct{}
func (c *Context) BadRequest(s string)               {}

func setup(app *App) { app.Post("/x", listX) }
func listX(c *Context) { c.BadRequest("nope") }
`,
	})
	report, _ := runValidateAndDecode(t, dir)
	codes := []string{}
	for _, it := range report.Issues {
		codes = append(codes, it.Code)
	}
	assert.Contains(t, codes, "route-missing-success-response")
}
