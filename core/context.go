package core

import (
	"encoding/json"
	"net"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/leebo/igo/core/errors"
	"github.com/leebo/igo/core/validator"
	"github.com/leebo/igo/types"
)

// registerSchemaOnce 把类型注册到当前 App 的 schema 注册表（已注册则只补 Usage）。
// 由 BindAndValidate / BindQueryAndValidate / BindPathAndValidate 自动调用，
// 让 /_ai/schemas 端点能列出所有被运行时绑定的类型，并标注其用途。
// nil registry 直接 no-op (Router 路径总是注入非 nil registry)。
func registerSchemaOnce(registry *types.TypeRegistry, v interface{}, usage string) {
	if registry == nil {
		return
	}
	t := reflect.TypeOf(v)
	if t == nil {
		return
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	name := t.Name()
	if name == "" {
		return // 匿名结构体跳过
	}
	if registry.GetType(name) != nil {
		if usage != "" {
			registry.RegisterTypeUsage(name, usage)
		}
		return
	}
	schema := types.ExtractSchemaFromType(t)
	if usage != "" {
		schema.Usage = []string{usage}
	}
	registry.RegisterType(&schema)
}

// registerResponseSchemaOnce 在 c.Success / c.Created / c.JSON 时尝试把响应负载
// 的具体类型注册为 schema（usage="response"）。仅识别命名结构体；H/map、原生
// 类型、匿名结构体、ListResponse 等参数化类型按其元素类型展开。
func registerResponseSchemaOnce(registry *types.TypeRegistry, v interface{}) {
	if registry == nil || v == nil {
		return
	}
	t := reflect.TypeOf(v)
	visited := make(map[reflect.Type]struct{})
	registerResponseSchemaType(registry, t, visited)
}

func registerResponseSchemaType(registry *types.TypeRegistry, t reflect.Type, visited map[reflect.Type]struct{}) {
	if t == nil {
		return
	}
	if _, seen := visited[t]; seen {
		return
	}
	visited[t] = struct{}{}

	switch t.Kind() {
	case reflect.Ptr:
		registerResponseSchemaType(registry, t.Elem(), visited)
		return
	case reflect.Slice, reflect.Array:
		registerResponseSchemaType(registry, t.Elem(), visited)
		return
	case reflect.Map:
		registerResponseSchemaType(registry, t.Elem(), visited)
		return
	case reflect.Interface:
		return
	case reflect.Struct:
		// fall through
	default:
		return
	}

	name := t.Name()
	if name == "" {
		return // 匿名 / 字面量 struct 不入注册表
	}
	if existing := registry.GetType(name); existing != nil {
		registry.RegisterTypeUsage(name, types.UsageResponse)
		// 仍递归内部字段，可能首次发现嵌套响应类型
	} else {
		schema := types.ExtractSchemaFromType(t)
		schema.Usage = []string{types.UsageResponse}
		registry.RegisterType(&schema)
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		registerResponseSchemaType(registry, f.Type, visited)
	}
}

// LoggerInterface 日志接口
type LoggerInterface interface {
	Printf(format string, v ...interface{})
}

var (
	globalLoggerMu sync.RWMutex
	globalLogger   LoggerInterface
)

// SetLogger 设置全局日志客户端，线程安全
func SetLogger(l LoggerInterface) {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()
	globalLogger = l
}

func getLogger() LoggerInterface {
	globalLoggerMu.RLock()
	defer globalLoggerMu.RUnlock()
	return globalLogger
}

// Context 是请求上下文
type Context struct {
	Request        *http.Request
	Writer         http.ResponseWriter
	Params         map[string]string
	QueryArgs      urlValues
	handlers       []HandlerFunc
	handlerIndex   int
	statusCode     int
	GinContextData map[string]interface{}
	errorChain     []*errors.StructuredError
	typeRegistry   *types.TypeRegistry
	values         map[string]any
}

// CtxKeyRequestID 是 RequestID 中间件写入到 Context.values 的标准键名。
// 错误响应渲染时从这里读取并填充响应体的 traceId 字段。
const CtxKeyRequestID = "request_id"

type urlValues map[string][]string

func newContext(w http.ResponseWriter, req *http.Request, typeRegistry *types.TypeRegistry) *Context {
	return &Context{
		Request:        req,
		Writer:         w,
		Params:         make(map[string]string),
		QueryArgs:      urlValues(req.URL.Query()),
		handlers:       make([]HandlerFunc, 0),
		handlerIndex:   -1,
		GinContextData: make(map[string]interface{}),
		typeRegistry:   typeRegistry,
	}
}

// Use 注册中间件
func (c *Context) Use(middleware MiddlewareFunc) {
	c.handlers = append(c.handlers, middleware)
}

// Set 在 Context 上存放任意键值，对中间件之间传递信息很有用（例如 RequestID
// 中间件把请求 ID 写到 c.Set(CtxKeyRequestID, id)）。
func (c *Context) Set(key string, value any) {
	if c.values == nil {
		c.values = make(map[string]any)
	}
	c.values[key] = value
}

// Get 读取之前 Set 的值。第二个返回值表示是否存在。
func (c *Context) Get(key string) (any, bool) {
	if c.values == nil {
		return nil, false
	}
	v, ok := c.values[key]
	return v, ok
}

// GetString 是 Get 的字符串便利方法，找不到或类型不匹配时返回 ""。
func (c *Context) GetString(key string) string {
	v, ok := c.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// TraceID 返回当前请求的 trace ID（来自 RequestID 中间件）。
// 未注册中间件或尚未生成时返回 ""，错误响应渲染会跳过 traceId 字段。
func (c *Context) TraceID() string {
	if id := c.GetString(CtxKeyRequestID); id != "" {
		return id
	}
	// fallback: 直接从请求/响应头读
	if id := c.Request.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	if c.Writer != nil {
		return c.Writer.Header().Get("X-Request-ID")
	}
	return ""
}

// Next 执行下一个处理器
func (c *Context) Next() {
	c.handlerIndex++
	if c.handlerIndex < len(c.handlers) {
		c.handlers[c.handlerIndex](c)
	}
}

// Param 获取路径参数
func (c *Context) Param(key string) string {
	return c.Params[key]
}

// ParamInt64 获取路径参数（int64）
func (c *Context) ParamInt64(key string) int64 {
	val := c.Params[key]
	if val == "" {
		return 0
	}
	i, _ := strconv.ParseInt(val, 10, 64)
	return i
}

// ParamInt 获取路径参数（int）
func (c *Context) ParamInt(key string) int {
	return int(c.ParamInt64(key))
}

// ParamInt64OrFail 获取路径参数（int64），无效则返回 400 并停止
func (c *Context) ParamInt64OrFail(key string) (int64, bool) {
	val := c.Params[key]
	if val == "" {
		c.BadRequest("missing parameter: " + key)
		return 0, false
	}
	i, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		c.BadRequestWrap(err, "invalid parameter: "+key)
		return 0, false
	}
	return i, true
}

// ParamIntOrFail 获取路径参数（int），无效则返回 400 并停止
func (c *Context) ParamIntOrFail(key string) (int, bool) {
	v, ok := c.ParamInt64OrFail(key)
	return int(v), ok
}

// ParamBool 获取路径参数（bool）
func (c *Context) ParamBool(key string) bool {
	val := strings.ToLower(c.Params[key])
	return val == "true" || val == "1" || val == "yes"
}

// Query 获取查询参数
func (c *Context) Query(key string) string {
	values := c.QueryArgs[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// QueryInt 获取查询参数（整数），不存在或解析失败返回 defaultVal
func (c *Context) QueryInt(key string, defaultVal int) int {
	val := c.Query(key)
	if val == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}

// QueryInt64 获取查询参数（int64），不存在或解析失败返回 defaultVal
func (c *Context) QueryInt64(key string, defaultVal int64) int64 {
	val := c.Query(key)
	if val == "" {
		return defaultVal
	}
	i, _ := strconv.ParseInt(val, 10, 64)
	return i
}

// QueryInt64OrFail 获取查询参数（int64），无效则返回 400 并停止
func (c *Context) QueryInt64OrFail(key string) (int64, bool) {
	val := c.Query(key)
	if val == "" {
		c.BadRequest("missing query parameter: " + key)
		return 0, false
	}
	i, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		c.BadRequestWrap(err, "invalid query parameter: "+key)
		return 0, false
	}
	return i, true
}

// QueryIntOrFail 获取查询参数（int），无效则返回 400 并停止
func (c *Context) QueryIntOrFail(key string) (int, bool) {
	v, ok := c.QueryInt64OrFail(key)
	return int(v), ok
}

// QueryDefault 获取查询参数，不存在时返回 defaultVal
func (c *Context) QueryDefault(key, defaultVal string) string {
	val := c.Query(key)
	if val == "" {
		return defaultVal
	}
	return val
}

// QueryBool 获取查询参数（bool）
func (c *Context) QueryBool(key string, defaultVal bool) bool {
	val := strings.ToLower(c.Query(key))
	if val == "" {
		return defaultVal
	}
	return val == "true" || val == "1" || val == "yes"
}

// BindJSON 将 JSON body 绑定到结构体
func (c *Context) BindJSON(v interface{}) error {
	if c.Request.Body == nil {
		return ErrBodyRequired
	}
	return json.NewDecoder(c.Request.Body).Decode(v)
}

// BindQuery 将 query 参数绑定到结构体
func (c *Context) BindQuery(v interface{}) error {
	m := make(map[string]string)
	for k, vals := range c.QueryArgs {
		if len(vals) > 0 {
			m[k] = vals[0]
		}
	}
	return bindValuesToStruct(m, v)
}

// BindPath 将 path 参数绑定到结构体
func (c *Context) BindPath(v interface{}) error {
	return bindValuesToStruct(c.Params, v)
}

// BindAndValidate 将 JSON body 绑定并验证，失败时自动返回错误响应
// 返回 (nil, false) 表示已发送错误响应，调用方应立即 return
//
// 用法：
//
//	req, ok := igo.BindAndValidate[CreateUserRequest](c)
//	if !ok { return }
func BindAndValidate[T any](c *Context) (*T, bool) {
	var req T
	registerSchemaOnce(c.typeRegistry, &req, types.UsageRequest)
	if err := c.BindJSON(&req); err != nil {
		c.BadRequestWrap(err, "invalid request body")
		return nil, false
	}
	if err := validator.Validate(&req); err != nil {
		c.ValidationError(err)
		return nil, false
	}
	return &req, true
}

// BindQueryAndValidate 把 URL 查询参数绑定到结构体并校验，失败时自动响应
//
// 用法：
//
//	q, ok := igo.BindQueryAndValidate[ListQuery](c)
//	if !ok { return }
func BindQueryAndValidate[T any](c *Context) (*T, bool) {
	var req T
	registerSchemaOnce(c.typeRegistry, &req, types.UsageQuery)
	if err := c.BindQuery(&req); err != nil {
		c.BadRequestWrap(err, "invalid query parameters")
		return nil, false
	}
	if err := validator.Validate(&req); err != nil {
		c.ValidationError(err)
		return nil, false
	}
	return &req, true
}

// BindPathAndValidate 把 :path 参数绑定到结构体并校验，失败时自动响应
//
// 用法：
//
//	p, ok := igo.BindPathAndValidate[ResourceParams](c)
//	if !ok { return }
func BindPathAndValidate[T any](c *Context) (*T, bool) {
	var req T
	registerSchemaOnce(c.typeRegistry, &req, types.UsagePath)
	if err := c.BindPath(&req); err != nil {
		c.BadRequestWrap(err, "invalid path parameters")
		return nil, false
	}
	if err := validator.Validate(&req); err != nil {
		c.ValidationError(err)
		return nil, false
	}
	return &req, true
}

// JSON 返回 JSON 响应
func (c *Context) JSON(status int, v interface{}) {
	if status >= 200 && status < 300 {
		registerResponseSchemaOnce(c.typeRegistry, v)
	}
	c.statusCode = status
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(status)
	json.NewEncoder(c.Writer).Encode(v)
}

// Success 返回成功响应（200）
func (c *Context) Success(v interface{}) {
	registerResponseSchemaOnce(c.typeRegistry, v)
	c.JSON(http.StatusOK, H{"data": v})
}

// Created 返回创建成功响应（201）
func (c *Context) Created(v interface{}) {
	registerResponseSchemaOnce(c.typeRegistry, v)
	c.JSON(http.StatusCreated, H{"data": v})
}

// NoContent 返回无内容响应（204）
func (c *Context) NoContent() {
	c.statusCode = http.StatusNoContent
	c.Writer.WriteHeader(http.StatusNoContent)
}

// Error 返回错误响应。如果 RequestID 中间件已设置 trace ID，会自动注入到响应体。
func (c *Context) Error(status int, code, message string) {
	errBody := H{
		"code":    code,
		"message": message,
	}
	if id := c.TraceID(); id != "" {
		errBody["traceId"] = id
	}
	c.JSON(status, H{"error": errBody})
}

// BadRequest 返回 400 错误
func (c *Context) BadRequest(message string) {
	c.Error(http.StatusBadRequest, "BAD_REQUEST", message)
}

// NotFound 返回 404 错误
func (c *Context) NotFound(message string) {
	c.Error(http.StatusNotFound, "NOT_FOUND", message)
}

// Unauthorized 返回 401 错误
func (c *Context) Unauthorized(message string) {
	c.Error(http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// Forbidden 返回 403 错误
func (c *Context) Forbidden(message string) {
	c.Error(http.StatusForbidden, "FORBIDDEN", message)
}

// InternalError 返回 500 错误
func (c *Context) InternalError(message string) {
	c.Error(http.StatusInternalServerError, "INTERNAL_ERROR", message)
}

// ValidationError 返回验证错误（422），包含字段信息和修复建议
// 当 err 是 *validator.ValidationError 时，会根据 rule 注入 suggestions 数组
func (c *Context) ValidationError(err error) {
	errData := H{
		"code":    "VALIDATION_FAILED",
		"message": err.Error(),
	}
	if ve, ok := err.(*validator.ValidationError); ok {
		if ve.Field != "" {
			errData["field"] = ve.Field
		}
		if ve.Rule != "" {
			errData["rule"] = ve.Rule
		}
		// 根据规则自动注入修复建议
		se := errors.NewValidationError(ve.Field, ve.Rule, ve.Message).WithSuggestionsForValidation()
		if len(se.Suggestions) > 0 {
			errData["suggestions"] = se.Suggestions
		}
	}
	if id := c.TraceID(); id != "" {
		errData["traceId"] = id
	}
	c.JSON(http.StatusUnprocessableEntity, H{"error": errData})
}

// InternalErrorWrap 返回 500 错误，带调用链，自动记录日志
func (c *Context) InternalErrorWrap(err error, message string, metadata map[string]any) {
	se := errors.NewStructuredError(errors.CodeInternalError, message).
		WithFilePath(c.callerFile(1)).
		WithLine(c.callerLine(1)).
		AddCallFrame()

	if err != nil {
		se = se.Wrap(err, message)
	}
	for k, v := range metadata {
		se.WithMetadata(k, v)
	}
	c.errorChain = append(c.errorChain, se)

	if l := getLogger(); l != nil {
		if err != nil {
			l.Printf("[ERROR] %s: %v (caller: %s:%d)", message, err, c.callerFile(1), c.callerLine(1))
		} else {
			l.Printf("[ERROR] %s (caller: %s:%d)", message, c.callerFile(1), c.callerLine(1))
		}
	}

	resp := errors.NewErrorResponse(se).WithTraceID(c.TraceID())
	c.JSON(http.StatusInternalServerError, resp)
}

// BadRequestWrap 返回 400 错误，带调用链，自动记录日志
func (c *Context) BadRequestWrap(err error, message string) {
	se := errors.NewStructuredError(errors.CodeBadRequest, message).
		WithFilePath(c.callerFile(1)).
		WithLine(c.callerLine(1)).
		AddCallFrame()

	if err != nil {
		se = se.Wrap(err, message)
	}
	c.errorChain = append(c.errorChain, se)

	if l := getLogger(); l != nil && err != nil {
		l.Printf("[WARN] %s: %v", message, err)
	}

	c.JSON(http.StatusBadRequest, errors.NewErrorResponse(se).WithTraceID(c.TraceID()))
}

// NotFoundWrap 返回 404 错误，带调用链，自动记录日志
func (c *Context) NotFoundWrap(err error, message string) {
	se := errors.NewStructuredError(errors.CodeNotFound, message).
		WithFilePath(c.callerFile(1)).
		WithLine(c.callerLine(1)).
		AddCallFrame()

	if err != nil {
		se = se.Wrap(err, message)
	}
	c.errorChain = append(c.errorChain, se)

	if l := getLogger(); l != nil && err != nil {
		l.Printf("[WARN] %s: %v", message, err)
	}

	c.JSON(http.StatusNotFound, errors.NewErrorResponse(se).WithTraceID(c.TraceID()))
}

// ValidationErrorWrap 返回验证错误，带调用链，自动记录日志
func (c *Context) ValidationErrorWrap(err error, field, message string) {
	se := errors.NewValidationError(field, "", message).
		WithFilePath(c.callerFile(1)).
		WithLine(c.callerLine(1)).
		AddCallFrame()

	if err != nil {
		se = se.Wrap(err, message)
	}
	c.errorChain = append(c.errorChain, se)

	if l := getLogger(); l != nil && err != nil {
		l.Printf("[WARN] validation failed: %s - %v", message, err)
	}

	c.JSON(http.StatusUnprocessableEntity, errors.NewErrorResponse(se).WithTraceID(c.TraceID()))
}

// GetErrorChain 获取错误链
func (c *Context) GetErrorChain() []*errors.StructuredError {
	return c.errorChain
}

// callerFile 返回调用者的文件路径，skip=1 表示直接调用者
func (c *Context) callerFile(skip int) string {
	if _, file, _, ok := runtime.Caller(skip + 1); ok {
		return file
	}
	return ""
}

// callerLine 返回调用者的行号，skip=1 表示直接调用者
func (c *Context) callerLine(skip int) int {
	if _, _, line, ok := runtime.Caller(skip + 1); ok {
		return line
	}
	return 0
}

// NotFoundIfNotFound 如果 err != nil，返回 404；返回 true 表示已发送响应
func (c *Context) NotFoundIfNotFound(err error, resourceName string) bool {
	if err != nil {
		c.NotFoundWrap(err, resourceName+" not found")
		return true
	}
	return false
}

// SuccessIfNotNil 如果 v 不为 nil 返回 200，否则返回 404
func (c *Context) SuccessIfNotNil(v interface{}, resourceName string) bool {
	if v == nil || (reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil()) {
		c.NotFound(resourceName + " not found")
		return false
	}
	c.Success(v)
	return true
}

// SuccessIfNotNilOrFail 带 err 检查的 SuccessIfNotNil
func (c *Context) SuccessIfNotNilOrFail(v interface{}, err error, resourceName string) bool {
	if err != nil {
		c.NotFoundWrap(err, resourceName+" not found")
		return true
	}
	if v == nil || (reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil()) {
		c.NotFound(resourceName + " not found")
		return true
	}
	c.Success(v)
	return true
}

// FailIfError 如果 err != nil，发送 500 响应；返回 true 表示有错误
func (c *Context) FailIfError(err error, message string) bool {
	if err != nil {
		c.InternalErrorWrap(err, message, nil)
		return true
	}
	return false
}

// FailIfErrorWithMeta 带元数据的 FailIfError
func (c *Context) FailIfErrorWithMeta(err error, message string, metadata map[string]any) bool {
	if err != nil {
		c.InternalErrorWrap(err, message, metadata)
		return true
	}
	return false
}

// Header 设置响应头
func (c *Context) Header(key, value string) {
	c.Writer.Header().Set(key, value)
}

// ClientIP 返回客户端真实 IP，按以下优先级：
//  1. X-Forwarded-For 第一个非空段（穿越代理时的真实来源）
//  2. X-Real-IP
//  3. RemoteAddr 去除 :port
//
// 注意：X-Forwarded-For 由客户端可伪造，仅在受信代理后使用
func (c *Context) ClientIP() string {
	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := c.Request.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	if host, _, err := net.SplitHostPort(c.Request.RemoteAddr); err == nil {
		return host
	}
	return c.Request.RemoteAddr
}

// Cookie 读取请求中的 Cookie 值
func (c *Context) Cookie(name string) (string, error) {
	cookie, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// SetCookie 设置响应 Cookie，缺省 SameSite=Lax
//   - maxAge < 0 表示立即过期（删除 cookie）
//   - maxAge = 0 表示会话 cookie（关闭浏览器即失效）
//   - maxAge > 0 表示秒数
func (c *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	if path == "" {
		path = "/"
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
		SameSite: http.SameSiteLaxMode,
	})
}

// Redirect 发送 HTTP 重定向，status 必须在 [300, 399] 范围内
func (c *Context) Redirect(status int, url string) {
	if status < 300 || status > 399 {
		c.InternalErrorWrap(nil, "invalid redirect status code", map[string]any{
			"status": status,
			"url":    url,
		})
		return
	}
	c.statusCode = status
	http.Redirect(c.Writer, c.Request, url, status)
}

// Status 设置响应状态码
func (c *Context) Status(status int) {
	c.statusCode = status
	c.Writer.WriteHeader(status)
}

// StatusCode 获取当前响应状态码
func (c *Context) StatusCode() int {
	return c.statusCode
}

// matchPath 简单路径匹配（支持 :param 格式）
func matchPath(pattern, path string) bool {
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")
	if len(patternParts) != len(pathParts) {
		return false
	}
	for i := range patternParts {
		if strings.HasPrefix(patternParts[i], ":") {
			continue
		}
		if patternParts[i] != pathParts[i] {
			return false
		}
	}
	return true
}

// extractParams 提取路径参数
func extractParams(pattern, path string) map[string]string {
	params := make(map[string]string)
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")
	for i := range patternParts {
		if i < len(pathParts) && strings.HasPrefix(patternParts[i], ":") {
			key := strings.TrimPrefix(patternParts[i], ":")
			params[key] = pathParts[i]
		}
	}
	return params
}

// bindValuesToStruct 将 map 值绑定到结构体
func bindValuesToStruct(values map[string]string, v interface{}) error {
	t := reflect.TypeOf(v).Elem()
	rv := reflect.ValueOf(v).Elem()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		key := strings.Split(jsonTag, ",")[0]
		if val, ok := values[key]; ok {
			if err := setStringValue(rv.Field(i), val); err != nil {
				return err
			}
		}
	}
	return nil
}

// setStringValue 根据字段类型设置值
func setStringValue(v reflect.Value, s string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetUint(i)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		v.SetFloat(f)
	case reflect.Bool:
		v.SetBool(s == "true" || s == "1")
	default:
		return ErrUnsupportedType
	}
	return nil
}

// H 是 map 的别名，用于构建 JSON 响应
type H map[string]interface{}

// SetHandlers 设置处理器链（供适配器使用）
func (c *Context) SetHandlers(handlers []HandlerFunc) {
	c.handlers = handlers
}

// GetHandlerIndex 获取当前处理器索引
func (c *Context) GetHandlerIndex() int {
	return c.handlerIndex
}

// SetHandlerIndex 设置处理器索引
func (c *Context) SetHandlerIndex(index int) {
	c.handlerIndex = index
}

// Abort 停止后续处理器执行
func (c *Context) Abort() {
	c.handlerIndex = len(c.handlers)
}

// IsAborted 检查是否已终止
func (c *Context) IsAborted() bool {
	return c.handlerIndex >= len(c.handlers)
}
