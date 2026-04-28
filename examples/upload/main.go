// Package main 演示文件上传 / 下载 / 列表。
//
// AI 学习要点：
//   - 用 http.MaxBytesReader 限制请求体大小（防 DoS）
//   - 用 ParseMultipartForm 解析 multipart/form-data
//   - 校验扩展名 + Content-Type，防止上传可执行内容
//   - 用 filepath.Base 清洗文件名 + 拒绝 .. 防 path-traversal
//   - 用 http.ServeFile 让 Go 标准库处理 Range 请求 + ETag
//
// 测试：
//
//	go run ./examples/upload
//	curl -F "file=@README.md" http://localhost:8080/upload
//	curl http://localhost:8080/files
//	curl -O http://localhost:8080/files/<filename>
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	igo "github.com/igo/igo"
	"github.com/igo/igo/core"
)

// 用 var 而非 const，便于测试覆盖（t.TempDir）
var (
	uploadDir           = "./uploads"
	maxUploadSize int64 = 10 << 20 // 10 MB
)

// allowedExt 白名单扩展名（生产应根据业务调整）
var allowedExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".pdf": true, ".txt": true, ".md": true, ".csv": true, ".json": true,
}

// setupApp 构造好路由的 App，便于测试复用
func setupApp() *igo.App {
	app := igo.Simple()
	app.Post("/upload", uploadHandler)
	app.Get("/files", listHandler)
	app.Get("/files/:name", downloadHandler)
	app.Delete("/files/:name", deleteHandler)
	return app
}

func main() {
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		panic(err)
	}
	app := setupApp()
	app.PrintRoutes()
	app.Run(":8080")
}

// uploadHandler 接收 multipart/form-data，字段名为 "file"
func uploadHandler(c *core.Context) {
	// 限制请求体最大字节数（防 OOM/DoS）
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	if err := c.Request.ParseMultipartForm(maxUploadSize); err != nil {
		c.BadRequestWrap(err, "failed to parse multipart form (file too large?)")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.BadRequestWrap(err, "missing 'file' field")
		return
	}
	defer file.Close()

	// 校验扩展名
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExt[ext] {
		c.BadRequest("file type not allowed: " + ext)
		return
	}

	// 生成安全文件名（时间戳 + filepath.Base 清洗 + 仅保留扩展名前部分的 base）
	cleanBase := filepath.Base(header.Filename)
	safeName := fmt.Sprintf("%d-%s", time.Now().UnixNano(), cleanBase)

	dst, err := os.Create(filepath.Join(uploadDir, safeName))
	if err != nil {
		c.InternalErrorWrap(err, "failed to create destination file", nil)
		return
	}
	defer dst.Close()

	n, err := io.Copy(dst, file)
	if err != nil {
		c.InternalErrorWrap(err, "failed to write file", nil)
		return
	}

	c.Created(core.H{
		"filename":     safeName,
		"size":         n,
		"originalName": header.Filename,
		"contentType":  header.Header.Get("Content-Type"),
		"url":          "/files/" + safeName,
	})
}

// listHandler 返回上传目录下的文件列表
func listHandler(c *core.Context) {
	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		c.InternalErrorWrap(err, "failed to read upload directory", nil)
		return
	}

	files := make([]core.H, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, core.H{
			"name":     e.Name(),
			"size":     info.Size(),
			"modTime":  info.ModTime(),
			"url":      "/files/" + e.Name(),
		})
	}
	c.Success(files)
}

// downloadHandler 返回文件流，依赖 http.ServeFile 处理 Range/ETag/MIME
func downloadHandler(c *core.Context) {
	name, err := safeFilename(c.Param("name"))
	if err != nil {
		c.BadRequestWrap(err, "invalid filename")
		return
	}

	path := filepath.Join(uploadDir, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		c.NotFoundWrap(err, "file not found")
		return
	}

	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeFile(c.Writer, c.Request, path)
}

// deleteHandler 删除指定文件
func deleteHandler(c *core.Context) {
	name, err := safeFilename(c.Param("name"))
	if err != nil {
		c.BadRequestWrap(err, "invalid filename")
		return
	}

	path := filepath.Join(uploadDir, name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			c.NotFoundWrap(err, "file not found")
			return
		}
		c.InternalErrorWrap(err, "failed to delete file", core.H{"name": name})
		return
	}
	c.NoContent()
}

// safeFilename 校验文件名不含路径分隔符 / .. 等危险字符
// 这是 path-traversal 攻击的核心防御
func safeFilename(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("filename is empty")
	}
	if strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("filename contains path separator")
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("filename contains '..'")
	}
	if filepath.Base(name) != name {
		return "", fmt.Errorf("filename is not a basename")
	}
	return name, nil
}
