package igo

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type igoTestBody struct {
	Name string `json:"name" validate:"required"`
}

type igoTestQuery struct {
	Page int `json:"page" validate:"gte:1"`
}

type igoTestPath struct {
	ID int64 `json:"id" validate:"gte:1"`
}

type igoTestResponse struct {
	OK bool `json:"ok"`
}

func TestTopLevelConstructorsAndHelpers(t *testing.T) {
	app := New()
	require.NotNil(t, app)

	app.Post("/body", func(c *Context) {
		req, ok := BindAndValidate[igoTestBody](c)
		require.True(t, ok)
		c.Success(H{"name": req.Name})
	})
	app.Get("/query", func(c *Context) {
		req, ok := BindQueryAndValidate[igoTestQuery](c)
		require.True(t, ok)
		c.Success(H{"page": req.Page})
	})
	app.Get("/items/:id", func(c *Context) {
		req, ok := BindPathAndValidate[igoTestPath](c)
		require.True(t, ok)
		c.Success(H{"id": req.ID})
	})
	RegisterAppSchema[igoTestResponse](app)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/body", strings.NewReader(`{"name":"alice"}`))
	app.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/query?page=2", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/items/7", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	names := map[string]bool{}
	for _, schema := range app.Schemas() {
		names[schema.Name] = true
	}
	assert.True(t, names["igoTestBody"])
	assert.True(t, names["igoTestQuery"])
	assert.True(t, names["igoTestPath"])
	assert.True(t, names["igoTestResponse"])

	simple := Simple()
	require.NotNil(t, simple)
	// Simple 在 dev/test 下注册：Recovery + CORS + Logger + recorderMiddleware (RegisterAIRoutes 内自动启用)
	assert.Equal(t, 4, simple.Router.GlobalMiddlewareCount())
}

func TestRegisterAppSchema(t *testing.T) {
	app := New()
	RegisterAppSchema[igoTestResponse](app)
	assert.NotEmpty(t, app.Schemas())
}
