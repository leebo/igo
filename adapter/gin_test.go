package adapter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/leebo/igo/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddlewareBasic(t *testing.T) {
	app := core.New()
	middlewareCalled := false

	app.Use(Middleware(func(gc *GinContext) {
		middlewareCalled = true
		gc.Set("test-key", "test-value")
		gc.Next()
	}))

	app.Get("/test", func(c *core.Context) {
		c.Success(core.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	assert.True(t, middlewareCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddlewareAbort(t *testing.T) {
	app := core.New()
	handlerCalled := false

	app.Use(Middleware(func(gc *GinContext) {
		gc.AbortWithStatusJSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}))

	app.Get("/test", func(c *core.Context) {
		handlerCalled = true
		c.Success(core.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	assert.False(t, handlerCalled)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "unauthorized")
}

func TestMiddlewareSetAndGet(t *testing.T) {
	app := core.New()

	app.Use(Middleware(func(gc *GinContext) {
		gc.Set("user-id", "12345")
		gc.Next()
	}))

	app.Use(Middleware(func(gc *GinContext) {
		userID, _ := gc.Get("user-id")
		assert.Equal(t, "12345", userID)
		gc.Next()
	}))

	app.Get("/test", func(c *core.Context) {
		c.Success(core.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)
}

func TestMiddlewareQuery(t *testing.T) {
	app := core.New()

	app.Get("/test", Middleware(func(gc *GinContext) {
		query := gc.Query("name")
		assert.Equal(t, "test", query)
		gc.JSON(http.StatusOK, map[string]string{"name": query})
	}))

	req := httptest.NewRequest(http.MethodGet, "/test?name=test", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)
}

func TestMiddlewareParam(t *testing.T) {
	app := core.New()

	app.Get("/user/:id", Middleware(func(gc *GinContext) {
		id := gc.Param("id")
		assert.Equal(t, "123", id)
		gc.JSON(http.StatusOK, map[string]string{"id": id})
	}))

	req := httptest.NewRequest(http.MethodGet, "/user/123", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)
}

func TestMultipleMiddlewares(t *testing.T) {
	app := core.New()
	callOrder := []string{}

	app.Use(Middleware(func(gc *GinContext) {
		callOrder = append(callOrder, "m1")
		gc.Next()
	}))

	app.Use(Middleware(func(gc *GinContext) {
		callOrder = append(callOrder, "m2")
		gc.Next()
	}))

	app.Get("/test", func(c *core.Context) {
		callOrder = append(callOrder, "handler")
		c.Success(core.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	assert.Equal(t, []string{"m1", "m2", "handler"}, callOrder)
}

func TestNewGinEngine(t *testing.T) {
	ge := NewGinEngine()
	assert.NotNil(t, ge)
}

func TestNewGinEngineWithMode(t *testing.T) {
	ge := NewGinEngineWithMode(gin.TestMode)
	assert.NotNil(t, ge)
}

func TestMount(t *testing.T) {
	app := core.New()
	ge := gin.New()

	ge.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"msg": "pong"})
	})

	Mount(app, "/api", ge)

	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "pong")
}

func TestGinContextRequestResponseHelpers(t *testing.T) {
	app := core.New()
	app.Post("/users/:id", Middleware(func(gc *GinContext) {
		assert.Equal(t, http.MethodPost, gc.Method())
		assert.Equal(t, "/users/42", gc.Path())
		assert.Equal(t, "/users/42", gc.FullPath())
		assert.Equal(t, "42", gc.Param("id"))
		assert.Equal(t, "debug", gc.Query("mode"))
		assert.Equal(t, "trace-1", gc.GetHeader("X-Trace-ID"))
		assert.False(t, gc.IsAborted())
		assert.False(t, gc.IsWritten())

		var body struct {
			Name string `json:"name"`
		}
		require.NoError(t, gc.BindJSON(&body))
		assert.Equal(t, "Ada", body.Name)

		gc.Header("X-Seen", "yes")
		gc.JSON(http.StatusAccepted, map[string]any{"id": gc.Param("id"), "name": body.Name})
		assert.True(t, gc.IsWritten())
	}))

	req := httptest.NewRequest(http.MethodPost, "/users/42?mode=debug", strings.NewReader(`{"name":"Ada"}`))
	req.Header.Set("X-Trace-ID", "trace-1")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "yes", w.Header().Get("X-Seen"))
	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	assert.Equal(t, "42", payload["id"])
	assert.Equal(t, "Ada", payload["name"])
}

func TestGinContextBindQueryAndStatus(t *testing.T) {
	app := core.New()
	app.Get("/search", Middleware(func(gc *GinContext) {
		var query struct {
			Page int `json:"page"`
		}
		require.NoError(t, gc.BindQuery(&query))
		assert.Equal(t, 2, query.Page)

		var query2 struct {
			Page int `json:"page"`
		}
		require.NoError(t, gc.ShouldBindQuery(&query2))
		assert.Equal(t, 2, query2.Page)

		gc.Abort()
		gc.AbortWithStatus(http.StatusNoContent)
	}))

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/search?page=2", nil))
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestMiddlewaresConvertsAll(t *testing.T) {
	middlewares := Middlewares(
		func(gc *GinContext) { gc.Set("one", true); gc.Next() },
		func(gc *GinContext) { gc.Set("two", true); gc.Next() },
	)
	require.Len(t, middlewares, 2)

	app := core.New()
	for _, middleware := range middlewares {
		app.Use(middleware)
	}
	app.Get("/test", func(c *core.Context) {
		one, _ := c.GinContextData["one"].(bool)
		two, _ := c.GinContextData["two"].(bool)
		c.Success(core.H{"one": one, "two": two})
	})

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"one":true`)
	assert.Contains(t, w.Body.String(), `"two":true`)
}

func TestGinResponseWriter(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := &GinResponseWriter{ResponseWriter: recorder, statusCode: http.StatusOK}

	assert.Equal(t, http.StatusOK, writer.Status())
	assert.True(t, writer.Written())
	writer.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, writer.Status())
	assert.Nil(t, writer.CloseNotify())
	writer.Flush()
}
