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
	"strings"
	"testing"

	igo "github.com/igo/igo"
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
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		fw.Write(content)
	} else {
		mw.WriteField("foo", "bar")
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestUpload_Success(t *testing.T) {
	app := setupTestApp(t)
	req := makeUploadRequest(t, "file", "hello.txt", []byte("hello world"))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Filename     string `json:"filename"`
			Size         int64  `json:"size"`
			OriginalName string `json:"originalName"`
			URL          string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\nbody=%s", err, w.Body.String())
	}
	if resp.Data.Size != int64(len("hello world")) {
		t.Errorf("size = %d, want %d", resp.Data.Size, len("hello world"))
	}
	if resp.Data.OriginalName != "hello.txt" {
		t.Errorf("originalName = %q, want hello.txt", resp.Data.OriginalName)
	}
	// 文件应当真的写到磁盘了
	if _, err := os.Stat(filepath.Join(uploadDir, resp.Data.Filename)); err != nil {
		t.Errorf("uploaded file should exist: %v", err)
	}
}

func TestUpload_MissingFileField(t *testing.T) {
	app := setupTestApp(t)
	req := makeUploadRequest(t, "file", "", nil) // 不附带 file 字段
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "missing 'file' field") {
		t.Errorf("body should mention missing file: %s", w.Body.String())
	}
}

func TestUpload_DisallowedExtension(t *testing.T) {
	app := setupTestApp(t)
	req := makeUploadRequest(t, "file", "evil.exe", []byte("MZ"))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not allowed") {
		t.Errorf("body should mention 'not allowed': %s", w.Body.String())
	}
	// 临时目录不应有任何残留文件
	entries, _ := os.ReadDir(uploadDir)
	if len(entries) != 0 {
		t.Errorf("disallowed upload should not be saved, found %d files", len(entries))
	}
}

func TestList_EmptyAndAfterUpload(t *testing.T) {
	app := setupTestApp(t)

	// 空目录：列表应是空数组
	req := httptest.NewRequest(http.MethodGet, "/files", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Errorf("expected empty list, got %s", w.Body.String())
	}

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
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data) != 1 {
		t.Errorf("expected 1 file, got %d", len(resp.Data))
	}
}

func TestDownload_Success(t *testing.T) {
	app := setupTestApp(t)
	name := "doc.txt"
	content := []byte("download me")
	if err := os.WriteFile(filepath.Join(uploadDir, name), content, 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/files/"+name, nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), content) {
		t.Errorf("body = %q, want %q", w.Body.Bytes(), content)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, name) {
		t.Errorf("Content-Disposition should reference %s, got %q", name, cd)
	}
}

func TestDownload_NotFound(t *testing.T) {
	app := setupTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/files/nope.txt", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// TestDownload_PathTraversalRejected 验证各种路径注入手段都不会拿到 uploadDir 之外的文件
func TestDownload_PathTraversalRejected(t *testing.T) {
	app := setupTestApp(t)

	// 同时在 uploadDir 内放一个真文件，避免被"恰好不存在"误导
	os.WriteFile(filepath.Join(uploadDir, "real.txt"), []byte("real"), 0o644)

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
			if w.Code == http.StatusOK {
				t.Errorf("path traversal succeeded for %q (status 200, body=%q)", tt.path, w.Body.String())
			}
		})
	}
}

func TestDelete_Success(t *testing.T) {
	app := setupTestApp(t)
	name := "rm-me.txt"
	path := filepath.Join(uploadDir, name)
	os.WriteFile(path, []byte("x"), 0o644)

	req := httptest.NewRequest(http.MethodDelete, "/files/"+name, nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestDelete_NotFound(t *testing.T) {
	app := setupTestApp(t)
	req := httptest.NewRequest(http.MethodDelete, "/files/ghost.txt", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
