package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	igo "github.com/leebo/igo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestApp 把 uploadDir 切到临时目录，避免污染仓库
func setupTestApp(t *testing.T) *igo.App {
	t.Helper()
	uploadDir = t.TempDir()
	return setupApp()
}

// makeUploadRequest 构造一个 multipart/form-data 请求
func makeUploadRequest(t *testing.T, fieldName, filename string, content []byte) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	if filename != "" {
		fw, err := mw.CreateFormFile(fieldName, filename)
		require.NoError(t, err)
		_, err = fw.Write(content)
		require.NoError(t, err)
	} else {
		require.NoError(t, mw.WriteField("foo", "bar"))
	}
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestUpload_Success(t *testing.T) {
	app := setupTestApp(t)
	req := makeUploadRequest(t, "file", "hello.txt", []byte("hello world"))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp struct {
		Data struct {
			Filename     string `json:"filename"`
			Size         int64  `json:"size"`
			OriginalName string `json:"originalName"`
			URL          string `json:"url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), w.Body.String())
	assert.Equal(t, int64(len("hello world")), resp.Data.Size)
	assert.Equal(t, "hello.txt", resp.Data.OriginalName)
	// 文件应当真的写到磁盘了
	_, err := os.Stat(filepath.Join(uploadDir, resp.Data.Filename))
	assert.NoError(t, err)
}

func TestUpload_MissingFileField(t *testing.T) {
	app := setupTestApp(t)
	req := makeUploadRequest(t, "file", "", nil) // 不附带 file 字段
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "missing 'file' field")
}

func TestUpload_DisallowedExtension(t *testing.T) {
	app := setupTestApp(t)
	req := makeUploadRequest(t, "file", "evil.exe", []byte("MZ"))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "not allowed")
	// 临时目录不应有任何残留文件
	entries, _ := os.ReadDir(uploadDir)
	assert.Empty(t, entries)
}

func TestList_EmptyAndAfterUpload(t *testing.T) {
	app := setupTestApp(t)

	// 空目录：列表应是空数组
	req := httptest.NewRequest(http.MethodGet, "/files", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"data":[]`)

	// 上传一个文件后，列表应包含 1 项
	app.Router.ServeHTTP(httptest.NewRecorder(), makeUploadRequest(t, "file", "a.txt", []byte("x")))
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/files", nil))

	var resp struct {
		Data []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 1)
}

func TestDownload_Success(t *testing.T) {
	app := setupTestApp(t)
	name := "doc.txt"
	content := []byte("download me")
	require.NoError(t, os.WriteFile(filepath.Join(uploadDir, name), content, 0o644))

	req := httptest.NewRequest(http.MethodGet, "/files/"+name, nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, content, w.Body.Bytes())
	assert.Contains(t, w.Header().Get("Content-Disposition"), name)
}

func TestDownload_NotFound(t *testing.T) {
	app := setupTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/files/nope.txt", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDownload_PathTraversalRejected 验证各种路径注入手段都不会拿到 uploadDir 之外的文件
func TestDownload_PathTraversalRejected(t *testing.T) {
	app := setupTestApp(t)

	// 同时在 uploadDir 内放一个真文件，避免被"恰好不存在"误导
	require.NoError(t, os.WriteFile(filepath.Join(uploadDir, "real.txt"), []byte("real"), 0o644))

	cases := []struct {
		name string
		path string // 直接拼到 URL（已 url.PathEscape 处理）
	}{
		{"with-dotdot", "with..dots"},
		{"backslash", url.PathEscape(`..\evil.txt`)},
		{"encoded-slash", url.PathEscape("../etc/passwd")}, // %2F 解码成 /
		{"url-encoded-traversal", "%2E%2E%2Fetc%2Fpasswd"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/files/"+tt.path, nil)
			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, req)
			assert.NotEqual(t, http.StatusOK, w.Code)
		})
	}
}

func TestDelete_Success(t *testing.T) {
	app := setupTestApp(t)
	name := "rm-me.txt"
	path := filepath.Join(uploadDir, name)
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	req := httptest.NewRequest(http.MethodDelete, "/files/"+name, nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		assert.Fail(t, "file should have been deleted")
	}
}

func TestDelete_NotFound(t *testing.T) {
	app := setupTestApp(t)
	req := httptest.NewRequest(http.MethodDelete, "/files/ghost.txt", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
