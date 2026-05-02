package route

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryRegistersCopiesAndQueriesRoutes(t *testing.T) {
	registry := NewRegistry()
	route := &RouteConfig{
		Method:      "GET",
		Path:        "/users/:id",
		HandlerName: "getUser",
		Tags:        []string{"users"},
		Params:      []ParamDefinition{{Name: "id", In: "path", Required: true}},
	}

	registry.RegisterRoute(route)
	route.Summary = "mutated"

	byPath := registry.GetRoute("GET", "/users/:id")
	require.NotNil(t, byPath)
	assert.Empty(t, byPath.Summary)
	assert.Equal(t, "getUser", byPath.HandlerName)

	byName := registry.GetRouteByName("getUser")
	require.NotNil(t, byName)
	assert.Equal(t, "/users/:id", byName.Path)

	byTag := registry.ListRoutesByTag("users")
	require.Len(t, byTag, 1)
	assert.Equal(t, "/users/:id", byTag[0].Path)
	assert.Equal(t, 1, registry.Count())

	registry.Clear()
	assert.Equal(t, 0, registry.Count())
	assert.Nil(t, registry.GetRoute("GET", "/users/:id"))
}

func TestRegistryListRoutesSortsByPathThenMethod(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterRoute(&RouteConfig{Method: "POST", Path: "/users"})
	registry.RegisterRoute(&RouteConfig{Method: "GET", Path: "/accounts"})
	registry.RegisterRoute(&RouteConfig{Method: "GET", Path: "/users"})
	registry.RegisterRoute(nil)

	routes := registry.ListRoutes()
	require.Len(t, routes, 3)
	assert.Equal(t, []string{
		"GET /accounts",
		"GET /users",
		"POST /users",
	}, []string{
		routes[0].Method + " " + routes[0].Path,
		routes[1].Method + " " + routes[1].Path,
		routes[2].Method + " " + routes[2].Path,
	})
}
