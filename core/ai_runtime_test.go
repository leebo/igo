package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	errorspkg "github.com/igo/igo/core/errors"
	routepkg "github.com/igo/igo/core/route"
	"github.com/igo/igo/types"
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

	if got := len(app1.Routes()); got != 1 {
		t.Fatalf("app1 route count = %d, want 1", got)
	}
	if got := len(app2.Routes()); got != 1 {
		t.Fatalf("app2 route count = %d, want 1", got)
	}
	if app1.Routes()[0].Path != "/one" {
		t.Fatalf("app1 route = %s, want /one", app1.Routes()[0].Path)
	}
	if app2.Routes()[0].Path != "/two" {
		t.Fatalf("app2 route = %s, want /two", app2.Routes()[0].Path)
	}

	if !hasSchema(app1.Schemas(), "aiRuntimeResponse") {
		t.Fatalf("app1 schema registry missing aiRuntimeResponse: %+v", app1.Schemas())
	}
	if hasSchema(app2.Schemas(), "aiRuntimeResponse") {
		t.Fatalf("app2 schema registry was polluted: %+v", app2.Schemas())
	}
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
	if userRoute == nil {
		t.Fatalf("/_ai/routes missing GET /users/:id: %+v", routes)
	}
	if userRoute.HandlerName == "" || userRoute.FilePath == "" || userRoute.LineNumber == 0 {
		t.Fatalf("route handler metadata incomplete: %+v", userRoute)
	}
	if len(userRoute.Params) != 1 || userRoute.Params[0].Name != "id" || userRoute.Params[0].In != "path" {
		t.Fatalf("route path params not inferred: %+v", userRoute.Params)
	}
	if !containsSuffix(userRoute.Middlewares, "aiRouteMiddleware") {
		t.Fatalf("route middleware name not recorded: %+v", userRoute.Middlewares)
	}

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
	if len(middlewarePayload.Global) != 1 || !strings.HasSuffix(middlewarePayload.Global[0].Name, "aiRouteGlobalMiddleware") {
		t.Fatalf("global middleware metadata not recorded: %+v", middlewarePayload.Global)
	}

	var schemas []types.TypeSchema
	getJSON(t, app, "/_ai/schemas", &schemas)
	if !hasSchemaPtrs(schemas, "aiRuntimeResponse") {
		t.Fatalf("/_ai/schemas missing aiRuntimeResponse: %+v", schemas)
	}

	var errorCodes []errorspkg.ErrorCodeInfo
	getJSON(t, app, "/_ai/errors", &errorCodes)
	if len(errorCodes) == 0 || errorCodes[0].Code == "" {
		t.Fatalf("/_ai/errors returned empty payload")
	}

	var openapi struct {
		OpenAPI    string `json:"openapi"`
		Components struct {
			Schemas map[string]any `json:"schemas"`
		} `json:"components"`
	}
	getJSON(t, app, "/_ai/openapi", &openapi)
	if openapi.OpenAPI != "3.0.0" {
		t.Fatalf("openapi version = %q, want 3.0.0", openapi.OpenAPI)
	}
	if _, ok := openapi.Components.Schemas["aiRuntimeResponse"]; !ok {
		t.Fatalf("openapi components missing aiRuntimeResponse: %+v", openapi.Components.Schemas)
	}

	var conventions map[string]any
	getJSON(t, app, "/_ai/conventions", &conventions)
	if conventions["workflow"] == nil || conventions["endpoints"] == nil {
		t.Fatalf("/_ai/conventions missing workflow/endpoints: %+v", conventions)
	}
}

func getJSON(t *testing.T, app *App, target string, out any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("%s status = %d, body=%s", target, w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
		t.Fatalf("%s json decode: %v\nbody=%s", target, err, w.Body.String())
	}
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
