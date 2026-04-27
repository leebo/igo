package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApp_Get(t *testing.T) {
	app := New()

	app.Get("/hello", func(c *Context) {
		c.Success(H{"message": "hello"})
	})

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestApp_Post(t *testing.T) {
	app := New()

	app.Post("/data", func(c *Context) {
		c.Success(H{"created": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/data", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestApp_Put(t *testing.T) {
	app := New()

	app.Put("/data/:id", func(c *Context) {
		id := c.Param("id")
		c.Success(H{"updated": id})
	})

	req := httptest.NewRequest(http.MethodPut, "/data/123", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestApp_Delete(t *testing.T) {
	app := New()

	app.Delete("/data/:id", func(c *Context) {
		c.NoContent()
	})

	req := httptest.NewRequest(http.MethodDelete, "/data/123", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestApp_Patch(t *testing.T) {
	app := New()

	app.Patch("/data/:id", func(c *Context) {
		c.Success(H{"patched": true})
	})

	req := httptest.NewRequest(http.MethodPatch, "/data/123", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestApp_Options(t *testing.T) {
	app := New()

	app.Options("/resource", func(c *Context) {
		c.Success(H{"options": true})
	})

	req := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestApp_Head(t *testing.T) {
	app := New()

	app.Head("/resource", func(c *Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodHead, "/resource", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestApp_Group(t *testing.T) {
	app := New()

	app.Group("/api", func(v1 *App) {
		v1.Get("/hello", func(c *Context) {
			c.Success(H{"api": "hello"})
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/hello", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestApp_GroupNested(t *testing.T) {
	app := New()

	app.Group("/v1", func(v1 *App) {
		v1.Group("/users", func(users *App) {
			users.Get("/list", func(c *Context) {
				c.Success(H{"users": []string{"a", "b"}})
			})
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/users/list", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestApp_GroupWithMiddlewares(t *testing.T) {
	app := New()

	app.Use(func(c *Context) {
		c.Header("X-Global", "global")
		c.Next()
	})

	app.Group("/v1", func(v1 *App) {
		v1.Get("/test", func(c *Context) {
			c.Success(H{"v1": true})
		})
	}, func(c *Context) {
		c.Header("X-Group", "middleware")
		c.Next()
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	// Note: Group middleware is only applied to routes registered within the group
}

func TestApp_Resources(t *testing.T) {
	app := New()

	app.Resources("/users", ResourceHandler{
		List: func(c *Context) {
			c.Success([]H{{"id": 1, "name": "张三"}})
		},
		Show: func(c *Context) {
			c.Success(H{"id": 1, "name": "张三"})
		},
		Create: func(c *Context) {
			c.Created(H{"id": 1})
		},
		Update: func(c *Context) {
			c.Success(H{"id": 1, "updated": true})
		},
		Delete: func(c *Context) {
			c.NoContent()
		},
	})

	tests := []struct {
		method string
		path   string
		code   int
	}{
		{http.MethodGet, "/users", http.StatusOK},
		{http.MethodPost, "/users", http.StatusCreated},
		{http.MethodGet, "/users/1", http.StatusOK},
		{http.MethodPut, "/users/1", http.StatusOK},
		{http.MethodDelete, "/users/1", http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, req)

			if w.Code != tt.code {
				t.Errorf("expected status %d, got %d", tt.code, w.Code)
			}
		})
	}
}

func TestApp_ResourcesWithMiddleware(t *testing.T) {
	app := New()

	app.Resources("/posts", ResourceHandler{
		List: func(c *Context) {
			c.Success([]H{{"id": 1}})
		},
		Show: func(c *Context) {},
		Create: func(c *Context) {
			c.Created(H{"id": 1})
		},
		Update: func(c *Context) {
			c.Success(H{"id": 1})
		},
		Delete: func(c *Context) {
			c.NoContent()
		},
	}, func(c *Context) {
		c.Header("X-Resource", "posts")
		c.Next()
	})

	req := httptest.NewRequest(http.MethodGet, "/posts", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Header().Get("X-Resource") != "posts" {
		t.Errorf("expected X-Resource header 'posts', got '%s'", w.Header().Get("X-Resource"))
	}
}

func TestApp_NotFound(t *testing.T) {
	app := New()

	app.Get("/exists", func(c *Context) {
		c.Success(H{"found": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestApp_SetNotFound(t *testing.T) {
	app := New()

	app.SetNotFound(func(c *Context) {
		c.JSON(http.StatusNotFound, H{"error": "custom 404"})
	})

	app.Get("/exists", func(c *Context) {
		c.Success(H{"found": true})
	})

	// Test existing route
	req := httptest.NewRequest(http.MethodGet, "/exists", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Test 404
	req = httptest.NewRequest(http.MethodGet, "/not-found", nil)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestApp_PathNormalization(t *testing.T) {
	app := New()

	// Path without leading slash should be normalized
	app.Get("test", func(c *Context) {
		c.Success(H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestApp_ChainMethods(t *testing.T) {
	app := New()

	app.Get("/chain", func(c *Context) { c.Success(H{"get": true}) })
	app.Post("/chain", func(c *Context) { c.Success(H{"post": true}) })
	app.Put("/chain", func(c *Context) { c.Success(H{"put": true}) })
	app.Delete("/chain", func(c *Context) { c.Success(H{"delete": true}) })
	app.Patch("/chain", func(c *Context) { c.Success(H{"patch": true}) })

	tests := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/chain", "get"},
		{http.MethodPost, "/chain", "post"},
		{http.MethodPut, "/chain", "put"},
		{http.MethodDelete, "/chain", "delete"},
		{http.MethodPatch, "/chain", "patch"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, req)

			var resp H
			json.Unmarshal(w.Body.Bytes(), &resp)
			data := resp["data"].(map[string]interface{})

			if data[tt.want] != true {
				t.Errorf("expected %s to be true", tt.want)
			}
		})
	}
}

func TestApp_Concurrent(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Context) {
		c.Success(H{"ok": true})
	})

	// Run multiple requests concurrently
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
		}()
	}
}
