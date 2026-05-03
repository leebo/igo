package core

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RequestRecord 是一次 HTTP 请求的轻量快照，用于 /_ai/last-requests 端点。
//
// 设计原则：
//   - 只在 dev/test 默认开启，prd 默认关闭（保护内存与隐私）
//   - 请求体 / 响应体最多记录 maxBodyBytes（默认 2KB），超出截断并标记 truncated
//   - 不会记录任何 Set-Cookie / Authorization / Cookie 头（AI agent 也不需要）
type RequestRecord struct {
	Time         time.Time         `json:"time"`
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	Query        string            `json:"query,omitempty"`
	Status       int               `json:"status"`
	DurationMS   int64             `json:"durationMs"`
	TraceID      string            `json:"traceId,omitempty"`
	ClientIP     string            `json:"clientIp,omitempty"`
	UserAgent    string            `json:"userAgent,omitempty"`
	ReqBytes     int               `json:"reqBytes,omitempty"`
	RespBytes    int               `json:"respBytes,omitempty"`
	ReqBody      string            `json:"reqBody,omitempty"`
	ReqTruncated bool              `json:"reqTruncated,omitempty"`
	RespBody     string            `json:"respBody,omitempty"`
	RespTrunc    bool              `json:"respTruncated,omitempty"`
	ErrorCode    string            `json:"errorCode,omitempty"`
	RouteParams  map[string]string `json:"routeParams,omitempty"`
}

// RequestRecorder 是一个固定容量的请求快照环形缓冲区。
type RequestRecorder struct {
	mu            sync.RWMutex
	capacity      int
	maxBodyBytes  int
	records       []RequestRecord
	cursor        int
	full          bool
	skipBodyPaths []string
}

// NewRequestRecorder 创建容量为 capacity、单请求体最多记录 maxBodyBytes 的 recorder。
// capacity <= 0 视为禁用（所有 Record 调用 no-op）。
func NewRequestRecorder(capacity, maxBodyBytes int) *RequestRecorder {
	if capacity < 0 {
		capacity = 0
	}
	if maxBodyBytes < 0 {
		maxBodyBytes = 0
	}
	return &RequestRecorder{
		capacity:     capacity,
		maxBodyBytes: maxBodyBytes,
		records:      make([]RequestRecord, 0, capacity),
		// /_ai/* 自身请求不记录，避免 AI 调试自检端点时污染缓冲区
		skipBodyPaths: []string{"/_ai/last-requests", "/_ai/logs"},
	}
}

// Record 写入一条记录。recorder 容量为 0 或 nil 时是 no-op。
func (r *RequestRecorder) Record(rec RequestRecord) {
	if r == nil || r.capacity == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.records) < r.capacity {
		r.records = append(r.records, rec)
		return
	}
	r.records[r.cursor] = rec
	r.cursor = (r.cursor + 1) % r.capacity
	r.full = true
}

// Snapshot 返回当前缓冲区的有序副本（旧 → 新）。
func (r *RequestRecorder) Snapshot() []RequestRecord {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.full {
		out := make([]RequestRecord, len(r.records))
		copy(out, r.records)
		return out
	}
	out := make([]RequestRecord, 0, r.capacity)
	out = append(out, r.records[r.cursor:]...)
	out = append(out, r.records[:r.cursor]...)
	return out
}

// shouldSkipPath 判断某路径是否不应该记录请求/响应体（自检端点）。
func (r *RequestRecorder) shouldSkipPath(path string) bool {
	if r == nil {
		return true
	}
	for _, p := range r.skipBodyPaths {
		if path == p {
			return true
		}
	}
	return false
}

// recordResponseWriter 包装 http.ResponseWriter，用于捕获状态码与响应字节数 + 部分响应体。
type recordResponseWriter struct {
	http.ResponseWriter
	status      int
	bytesOut    int
	body        bytes.Buffer
	bodyMax     int
	bodyDropped bool
}

func (w *recordResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *recordResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.bytesOut += len(p)
	remaining := w.bodyMax - w.body.Len()
	if remaining > 0 {
		if remaining >= len(p) {
			w.body.Write(p)
		} else {
			w.body.Write(p[:remaining])
			w.bodyDropped = true
		}
	} else if w.bodyMax > 0 {
		w.bodyDropped = true
	}
	return w.ResponseWriter.Write(p)
}

// Flush 透传 http.Flusher（SSE / streaming 兼容）。
func (w *recordResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack 透传 http.Hijacker（WebSocket 升级需要）。
// 上游不支持时返回错误；recorderMiddleware 在 hijack 之后会跳过 body 抓取。
func (w *recordResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not implement http.Hijacker")
	}
	conn, brw, err := hj.Hijack()
	if err == nil {
		w.bodyDropped = false
		w.body.Reset()
	}
	return conn, brw, err
}

// recorderMiddleware 返回挂载到全局/局部都可的录制中间件。
// 仅当 recorder != nil 且 capacity > 0 才生效。
func recorderMiddleware(rec *RequestRecorder) MiddlewareFunc {
	if rec == nil || rec.capacity == 0 {
		return func(c *Context) { c.Next() }
	}
	return func(c *Context) {
		path := c.Request.URL.Path
		// /_ai/last-requests 自身完全跳过，避免轮询占满缓冲区
		if rec.shouldSkipPath(path) {
			c.Next()
			return
		}

		start := time.Now()

		// 读取请求体的前 maxBodyBytes 字节（仅 application/json）。
		reqBodyText, reqTruncated, reqBytes := captureRequestBody(c.Request, rec.maxBodyBytes)

		rw := &recordResponseWriter{ResponseWriter: c.Writer, bodyMax: rec.maxBodyBytes}
		c.Writer = rw

		c.Next()

		params := make(map[string]string, len(c.Params))
		for k, v := range c.Params {
			params[k] = v
		}

		errCode := ""
		bodyText := rw.body.String()
		if rw.status >= 400 && strings.Contains(bodyText, `"code":`) {
			errCode = extractErrorCode(bodyText)
		}

		rec.Record(RequestRecord{
			Time:         start.UTC(),
			Method:       c.Request.Method,
			Path:         path,
			Query:        c.Request.URL.RawQuery,
			Status:       statusOrOK(rw.status, c.statusCode),
			DurationMS:   time.Since(start).Milliseconds(),
			TraceID:      c.TraceID(),
			ClientIP:     c.ClientIP(),
			UserAgent:    c.Request.Header.Get("User-Agent"),
			ReqBytes:     reqBytes,
			RespBytes:    rw.bytesOut,
			ReqBody:      reqBodyText,
			ReqTruncated: reqTruncated,
			RespBody:     bodyText,
			RespTrunc:    rw.bodyDropped,
			ErrorCode:    errCode,
			RouteParams:  params,
		})
	}
}

// captureRequestBody 读取请求体的前 maxBytes 字节并把读过的内容塞回 Body 以便后续 handler 使用。
// 仅捕获 application/json / text/* / application/x-www-form-urlencoded，其它类型只统计长度。
func captureRequestBody(req *http.Request, maxBytes int) (string, bool, int) {
	if req == nil || req.Body == nil || maxBytes <= 0 {
		return "", false, 0
	}
	contentType := req.Header.Get("Content-Type")
	textual := strings.HasPrefix(contentType, "application/json") ||
		strings.HasPrefix(contentType, "text/") ||
		strings.HasPrefix(contentType, "application/x-www-form-urlencoded")

	buf, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return "", false, 0
	}
	req.Body = io.NopCloser(bytes.NewReader(buf))
	if !textual {
		return "", false, len(buf)
	}
	truncated := false
	body := buf
	if len(body) > maxBytes {
		body = body[:maxBytes]
		truncated = true
	}
	return string(body), truncated, len(buf)
}

// extractErrorCode 从响应体里粗略抽取 `"code":"XXX"` 的 XXX 部分（无依赖正则，避免冷启动）。
func extractErrorCode(body string) string {
	idx := strings.Index(body, `"code":"`)
	if idx < 0 {
		return ""
	}
	body = body[idx+len(`"code":"`):]
	end := strings.IndexByte(body, '"')
	if end < 0 {
		return ""
	}
	return body[:end]
}

func statusOrOK(captured, ctxStatus int) int {
	if captured > 0 {
		return captured
	}
	if ctxStatus > 0 {
		return ctxStatus
	}
	return http.StatusOK
}

// LogRecord 是一条结构化日志快照。
type LogRecord struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// LogRecorder 是日志环形缓冲区，注入到 LoggerInterface 上即可在 /_ai/logs 暴露最近 N 条。
type LogRecorder struct {
	mu       sync.RWMutex
	capacity int
	records  []LogRecord
	cursor   int
	full     bool

	upstream LoggerInterface // 可选：透传给底层 logger（保持原行为）
}

// NewLogRecorder 创建容量为 capacity 的 LogRecorder。
// upstream != nil 时所有 Printf 也会转发给它；nil 时只缓存到 ring。
func NewLogRecorder(capacity int, upstream LoggerInterface) *LogRecorder {
	if capacity < 0 {
		capacity = 0
	}
	return &LogRecorder{
		capacity: capacity,
		upstream: upstream,
		records:  make([]LogRecord, 0, capacity),
	}
}

// Printf 实现 LoggerInterface。会从 format 第一个 [LEVEL] 风格前缀里提取日志级别，
// 没有则归类为 info；之后把整条消息塞进 ring，并透传给 upstream。
func (l *LogRecorder) Printf(format string, args ...any) {
	if l == nil {
		return
	}
	if l.upstream != nil {
		l.upstream.Printf(format, args...)
	}
	if l.capacity == 0 {
		return
	}
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	level := "info"
	if strings.HasPrefix(msg, "[ERROR") {
		level = "error"
	} else if strings.HasPrefix(msg, "[WARN") {
		level = "warn"
	} else if strings.HasPrefix(msg, "[DEBUG") {
		level = "debug"
	}
	rec := LogRecord{Time: time.Now().UTC(), Level: level, Message: msg}

	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.records) < l.capacity {
		l.records = append(l.records, rec)
		return
	}
	l.records[l.cursor] = rec
	l.cursor = (l.cursor + 1) % l.capacity
	l.full = true
}

// Snapshot 返回 ring 中所有日志（旧 → 新）。
func (l *LogRecorder) Snapshot() []LogRecord {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if !l.full {
		out := make([]LogRecord, len(l.records))
		copy(out, l.records)
		return out
	}
	out := make([]LogRecord, 0, l.capacity)
	out = append(out, l.records[l.cursor:]...)
	out = append(out, l.records[:l.cursor]...)
	return out
}

