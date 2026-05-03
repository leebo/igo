package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestRecorder_RingWraps(t *testing.T) {
	r := NewRequestRecorder(3, 0)
	for i := 0; i < 5; i++ {
		r.Record(RequestRecord{Path: "/x", Status: 200 + i})
	}
	snap := r.Snapshot()
	require.Len(t, snap, 3)
	assert.Equal(t, 202, snap[0].Status, "oldest after wrap should be the 3rd insert")
	assert.Equal(t, 204, snap[2].Status, "newest")
}

func TestRequestRecorder_ZeroCapacityIsNoOp(t *testing.T) {
	r := NewRequestRecorder(0, 0)
	r.Record(RequestRecord{Path: "/x"})
	assert.Empty(t, r.Snapshot())
}

func TestLastRequestsEndpoint_RecordsTraceAndError(t *testing.T) {
	app := New().WithMode(ModeDev)
	app.Use(func(c *Context) {
		c.Set(CtxKeyRequestID, "trace-xyz")
		c.Header("X-Request-ID", "trace-xyz")
		c.Next()
	})
	app.RegisterAIRoutes() // dev: 自动注册 /_ai/last-requests

	app.Post("/users", func(c *Context) {
		c.BadRequest("bad input")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"alice"}`))
	r.Header.Set("Content-Type", "application/json")
	app.Router.ServeHTTP(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/_ai/last-requests", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Count   int             `json:"count"`
		Records []RequestRecord `json:"records"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Count, "/_ai/last-requests itself should not be recorded")

	rec := resp.Records[0]
	assert.Equal(t, "POST", rec.Method)
	assert.Equal(t, "/users", rec.Path)
	assert.Equal(t, http.StatusBadRequest, rec.Status)
	assert.Equal(t, "trace-xyz", rec.TraceID)
	assert.Equal(t, "BAD_REQUEST", rec.ErrorCode)
	assert.Contains(t, rec.ReqBody, `"name":"alice"`)
	assert.Contains(t, rec.RespBody, `"BAD_REQUEST"`)
}

func TestLogsEndpoint_CapturesPrintf(t *testing.T) {
	app := New().WithMode(ModeDev)
	app.RegisterAIRoutes()

	app.Get("/boom", func(c *Context) {
		c.InternalErrorWrap(assertedError("kaboom"), "save failed", nil)
	})

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	require.Equal(t, http.StatusInternalServerError, w.Code)

	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/_ai/logs?level=error", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Records []LogRecord `json:"records"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	found := false
	for _, r := range resp.Records {
		if strings.Contains(r.Message, "save failed") && r.Level == "error" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected captured ERROR log, got %+v", resp.Records)
}

type recorderTestErr struct{ msg string }

func (e *recorderTestErr) Error() string { return e.msg }

func assertedError(msg string) error {
	return &recorderTestErr{msg: msg}
}
