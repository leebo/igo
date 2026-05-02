package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	errorspkg "github.com/leebo/igo/core/errors"
	routepkg "github.com/leebo/igo/core/route"
	"github.com/leebo/igo/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type aiRuntimeResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name" validate:"required|min:2"`
}

func aiRouteGlobalMiddleware(c *Context) {
	c.Next()
}

func aiRouteMiddleware(c *Context) {
	c.Next()
}

func TestAppRegistriesAreIsolated(t *testing.T) {
	app1 := New()
	app2 := New()

	app1.Get("/one", func(c *Context) { c.Success(H{"app": 1}) })
	app1.RegisterSchema(aiRuntimeResponse{})
	app2.Get("/two", func(c *Context) { c.Success(H{"app": 2}) })

	require.Len(t, app1.Routes(), 1)
	require.Len(t, app2.Routes(), 1)
	assert.Equal(t, "/one", app1.Routes()[0].Path)
	assert.Equal(t, "/two", app2.Routes()[0].Path)

	assert.True(t, hasSchema(app1.Schemas(), "aiRuntimeResponse"))
	assert.False(t, hasSchema(app2.Schemas(), "aiRuntimeResponse"))
}

func TestRegisterAIRoutesExposeMetadata(t *testing.T) {
	app := New()
	app.Use(aiRouteGlobalMiddleware)
	app.RegisterSchema(aiRuntimeResponse{})
	app.Get("/users/:id", func(c *Context) {
		c.Success(aiRuntimeResponse{ID: c.ParamInt64("id"), Name: "Ada"})
	}, aiRouteMiddleware)
	app.RegisterAIRoutes()

	var routes []routepkg.RouteConfig
	getJSON(t, app, "/_ai/routes", &routes)
	userRoute := findRoute(routes, http.MethodGet, "/users/:id")
	require.NotNil(t, userRoute)
	assert.NotEmpty(t, userRoute.HandlerName)
	assert.NotEmpty(t, userRoute.FilePath)
	assert.NotZero(t, userRoute.LineNumber)
	require.Len(t, userRoute.Params, 1)
	assert.Equal(t, "id", userRoute.Params[0].Name)
	assert.Equal(t, "path", userRoute.Params[0].In)
	assert.True(t, containsSuffix(userRoute.Middlewares, "aiRouteMiddleware"))

	var middlewarePayload struct {
		Global []struct {
			Order int    `json:"order"`
			Name  string `json:"name"`
		} `json:"global"`
		Routes []struct {
			Method      string `json:"method"`
			Path        string `json:"path"`
			Middlewares []struct {
				Order int    `json:"order"`
				Name  string `json:"name"`
			} `json:"middlewares"`
		} `json:"routes"`
	}
	getJSON(t, app, "/_ai/middlewares", &middlewarePayload)
	require.Len(t, middlewarePayload.Global, 1)
	assert.True(t, strings.HasSuffix(middlewarePayload.Global[0].Name, "aiRouteGlobalMiddleware"))

	var schemas []types.TypeSchema
	getJSON(t, app, "/_ai/schemas", &schemas)
	assert.True(t, hasSchemaPtrs(schemas, "aiRuntimeResponse"))

	var errorCodes []errorspkg.ErrorCodeInfo
	getJSON(t, app, "/_ai/errors", &errorCodes)
	require.NotEmpty(t, errorCodes)
	assert.NotEmpty(t, errorCodes[0].Code)

	var openapi struct {
		OpenAPI    string `json:"openapi"`
		Components struct {
			Schemas map[string]any `json:"schemas"`
		} `json:"components"`
	}
	getJSON(t, app, "/_ai/openapi", &openapi)
	assert.Equal(t, "3.0.0", openapi.OpenAPI)
	assert.Contains(t, openapi.Components.Schemas, "aiRuntimeResponse")

	var conventions map[string]any
	getJSON(t, app, "/_ai/conventions", &conventions)
	assert.NotNil(t, conventions["workflow"])
	assert.NotNil(t, conventions["endpoints"])
}

func getJSON(t *testing.T, app *App, target string, out any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), out), w.Body.String())
}

func findRoute(routes []routepkg.RouteConfig, method, path string) *routepkg.RouteConfig {
	for i := range routes {
		if routes[i].Method == method && routes[i].Path == path {
			return &routes[i]
		}
	}
	return nil
}

func hasSchema(schemas []*types.TypeSchema, name string) bool {
	for _, schema := range schemas {
		if schema.Name == name {
			return true
		}
	}
	return false
}

func hasSchemaPtrs(schemas []types.TypeSchema, name string) bool {
	for _, schema := range schemas {
		if schema.Name == name {
			return true
		}
	}
	return false
}

func containsSuffix(values []string, suffix string) bool {
	for _, value := range values {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}
