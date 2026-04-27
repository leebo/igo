package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContext_Query(t *testing.T) {
	app := New()

	app.Get("/search", func(c *Context) {
		q := c.Query("q")
		c.Success(H{"query": q})
	})

	req := httptest.NewRequest(http.MethodGet, "/search?q=hello", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	var resp H
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	if data["query"] != "hello" {
		t.Errorf("expected query 'hello', got '%v'", data["query"])
	}
}

func TestContext_QueryInt(t *testing.T) {
	app := New()

	app.Get("/page", func(c *Context) {
		page := c.QueryInt("page", 1)
		size := c.QueryInt("size", 20)
		c.Success(H{"page": page, "size": size})
	})

	tests := []struct {
		url      string
		wantPage int
		wantSize int
	}{
		{"/page", 1, 20},
		{"/page?page=2", 2, 20},
		{"/page?page=3&size=50", 3, 50},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, req)

			var resp H
			json.Unmarshal(w.Body.Bytes(), &resp)

			data := resp["data"].(map[string]interface{})
			if int(data["page"].(float64)) != tt.wantPage {
				t.Errorf("expected page %d, got %v", tt.wantPage, data["page"])
			}
			if int(data["size"].(float64)) != tt.wantSize {
				t.Errorf("expected size %d, got %v", tt.wantSize, data["size"])
			}
		})
	}
}

func TestContext_QueryDefault(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Context) {
		val := c.QueryDefault("name", "default")
		c.Success(H{"name": val})
	})

	// Test with default
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	var resp H
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["name"] != "default" {
		t.Errorf("expected 'default', got '%v'", data["name"])
	}

	// Test with value
	req = httptest.NewRequest(http.MethodGet, "/test?name=custom", nil)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &resp)
	data = resp["data"].(map[string]interface{})
	if data["name"] != "custom" {
		t.Errorf("expected 'custom', got '%v'", data["name"])
	}
}

func TestContext_Param(t *testing.T) {
	app := New()

	app.Get("/users/:id/posts/:postId", func(c *Context) {
		id := c.Param("id")
		postId := c.Param("postId")
		c.Success(H{"userId": id, "postId": postId})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123/posts/456", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	var resp H
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	if data["userId"] != "123" {
		t.Errorf("expected id '123', got '%v'", data["userId"])
	}
	if data["postId"] != "456" {
		t.Errorf("expected postId '456', got '%v'", data["postId"])
	}
}

func TestContext_BindJSON(t *testing.T) {
	app := New()

	app.Post("/data", func(c *Context) {
		var data struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}
		if err := c.BindJSON(&data); err != nil {
			c.BadRequest("invalid json")
			return
		}
		c.Success(H{"name": data.Name, "value": data.Value})
	})

	body := `{"name":"test","value":42}`
	req := httptest.NewRequest(http.MethodPost, "/data", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	var resp H
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	if data["name"] != "test" {
		t.Errorf("expected name 'test', got '%v'", data["name"])
	}
	if int(data["value"].(float64)) != 42 {
		t.Errorf("expected value 42, got %v", data["value"])
	}
}

func TestContext_BindJSON_Invalid(t *testing.T) {
	app := New()

	app.Post("/data", func(c *Context) {
		var data struct {
			Name string `json:"name"`
		}
		if err := c.BindJSON(&data); err != nil {
			c.BadRequest("invalid json")
			return
		}
		c.Success(H{"name": data.Name})
	})

	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPost, "/data", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestContext_BindQuery(t *testing.T) {
	app := New()

	app.Get("/search", func(c *Context) {
		var params struct {
			Query string `json:"q"`
			Limit int    `json:"limit"`
		}
		if err := c.BindQuery(&params); err != nil {
			c.BadRequest("invalid query")
			return
		}
		c.Success(H{"q": params.Query, "limit": params.Limit})
	})

	req := httptest.NewRequest(http.MethodGet, "/search?q=test&limit=10", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	var resp H
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	if data["q"] != "test" {
		t.Errorf("expected q 'test', got '%v'", data["q"])
	}
	if int(data["limit"].(float64)) != 10 {
		t.Errorf("expected limit 10, got %v", data["limit"])
	}
}

func TestContext_Success(t *testing.T) {
	app := New()

	app.Get("/ok", func(c *Context) {
		c.Success(H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp H
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["data"] == nil {
		t.Error("expected 'data' field in response")
	}
}

func TestContext_Created(t *testing.T) {
	app := New()

	app.Post("/resource", func(c *Context) {
		c.Created(H{"id": 1})
	})

	req := httptest.NewRequest(http.MethodPost, "/resource", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestContext_NoContent(t *testing.T) {
	app := New()

	app.Delete("/resource", func(c *Context) {
		c.NoContent()
	})

	req := httptest.NewRequest(http.MethodDelete, "/resource", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestContext_BadRequest(t *testing.T) {
	app := New()

	app.Post("/data", func(c *Context) {
		c.BadRequest("invalid input")
	})

	req := httptest.NewRequest(http.MethodPost, "/data", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp H
	json.Unmarshal(w.Body.Bytes(), &resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "BAD_REQUEST" {
		t.Errorf("expected code 'BAD_REQUEST', got '%v'", errObj["code"])
	}
}

func TestContext_NotFound(t *testing.T) {
	app := New()

	app.Get("/missing", func(c *Context) {
		c.NotFound("resource not found")
	})

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestContext_Unauthorized(t *testing.T) {
	app := New()

	app.Get("/secure", func(c *Context) {
		c.Unauthorized("token required")
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestContext_Forbidden(t *testing.T) {
	app := New()

	app.Get("/forbidden", func(c *Context) {
		c.Forbidden("access denied")
	})

	req := httptest.NewRequest(http.MethodGet, "/forbidden", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestContext_InternalError(t *testing.T) {
	app := New()

	app.Get("/error", func(c *Context) {
		c.InternalError("something went wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestContext_ValidationError(t *testing.T) {
	app := New()

	app.Post("/validate", func(c *Context) {
		c.ValidationError(ErrBodyRequired)
	})

	req := httptest.NewRequest(http.MethodPost, "/validate", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", w.Code)
	}
}

func TestContext_Header(t *testing.T) {
	app := New()

	app.Get("/headers", func(c *Context) {
		c.Header("X-Custom", "value")
		c.Header("X-Another", "another")
		c.Success(H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/headers", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Header().Get("X-Custom") != "value" {
		t.Errorf("expected header 'value', got '%s'", w.Header().Get("X-Custom"))
	}
	if w.Header().Get("X-Another") != "another" {
		t.Errorf("expected header 'another', got '%s'", w.Header().Get("X-Another"))
	}
}

func TestContext_Status(t *testing.T) {
	app := New()

	app.Get("/custom", func(c *Context) {
		c.Status(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodGet, "/custom", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}
}

func TestContext_JSON(t *testing.T) {
	app := New()

	app.Get("/json", func(c *Context) {
		c.JSON(http.StatusOK, H{"json": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/json", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected 'application/json', got '%s'", w.Header().Get("Content-Type"))
	}
}

func TestContext_Error(t *testing.T) {
	app := New()

	app.Get("/error", func(c *Context) {
		c.Error(http.StatusBadGateway, "BAD_GATEWAY", "upstream error")
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", w.Code)
	}

	var resp H
	json.Unmarshal(w.Body.Bytes(), &resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "BAD_GATEWAY" {
		t.Errorf("expected code 'BAD_GATEWAY', got '%v'", errObj["code"])
	}
	if errObj["message"] != "upstream error" {
		t.Errorf("expected message 'upstream error', got '%v'", errObj["message"])
	}
}
