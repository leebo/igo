package adapter

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/leebo/igo/core"
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

	if !middlewareCalled {
		t.Error("middleware was not called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
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

	if handlerCalled {
		t.Error("handler should not be called after Abort")
	}

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "unauthorized") {
		t.Errorf("expected body to contain 'unauthorized', got %s", body)
	}
}

func TestMiddlewareSetAndGet(t *testing.T) {
	app := core.New()

	app.Use(Middleware(func(gc *GinContext) {
		gc.Set("user-id", "12345")
		gc.Next()
	}))

	app.Use(Middleware(func(gc *GinContext) {
		userID, _ := gc.Get("user-id")
		if userID != "12345" {
			t.Errorf("expected user-id to be 12345, got %v", userID)
		}
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
		if query != "test" {
			t.Errorf("expected query 'test', got '%s'", query)
		}
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
		if id != "123" {
			t.Errorf("expected param '123', got '%s'", id)
		}
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

	expected := []string{"m1", "m2", "handler"}
	for i, e := range expected {
		if callOrder[i] != e {
			t.Errorf("callOrder[%d] = %s, expected %s", i, callOrder[i], e)
		}
	}
}

func TestNewGinEngine(t *testing.T) {
	ge := NewGinEngine()
	if ge == nil {
		t.Error("expected non-nil GinEngine")
	}
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "pong") {
		t.Errorf("expected body to contain 'pong', got %s", body)
	}
}
