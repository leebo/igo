package core

import (
	"encoding/json"
	"net/http"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/igo/igo/core/errors"
)

// LoggerInterface 日志接口
type LoggerInterface interface {
	Printf(format string, v ...interface{})
}

// globalLogger 全局日志客户端（可选，用于自动记录错误日志）
var globalLogger LoggerInterface

// SetLogger 设置全局日志客户端
// 如果设置了，InternalErrorWrap 等方法会自动记录错误到日志
func SetLogger(l LoggerInterface) {
	globalLogger = l
}

// Context 是请求上下文
type Context struct {
	Request         *http.Request
	Writer          http.ResponseWriter
	Params          map[string]string
	QueryArgs       urlValues
	handlers       []HandlerFunc
	handlerIndex   int
	statusCode     int
	// GinContextData 用于存储 gin 风格的数据（c.Set/c.Get）
	// 由 adapter 包使用
	GinContextData map[string]interface{}
	// errorChain 错误链，用于追踪错误传播
	errorChain []*errors.StructuredError
}

type urlValues map[string][]string

func newContext(w http.ResponseWriter, req *http.Request) *Context {
	return &Context{
		Request:          req,
		Writer:           w,
		Params:           make(map[string]string),
		QueryArgs:        urlValues(req.URL.Query()),
		handlers:         make([]HandlerFunc, 0),
		handlerIndex:     -1, // 初始值为 -1，Next() 会先递增再调用
		GinContextData:   make(map[string]interface{}),
	}
}

// Use 注册中间件
func (c *Context) Use(middleware MiddlewareFunc) {
	c.handlers = append(c.handlers, middleware)
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

// ParamInt64OrFail 获取路径参数（int64），无效则返回 400 错误
// 返回 (value, true) 表示成功，返回 (0, false) 表示失败并已发送响应
func (c *Context) ParamInt64OrFail(key string) (int64, bool) {
	val := c.Params[key]
	if val == "" {
		c.BadRequest("missing parameter: " + key)
		return 0, false
	}
	i, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		c.BadRequest("invalid parameter: " + key)
		return 0, false
	}
	return i, true
}

// ParamIntOrFail 获取路径参数（int），无效则返回 400 错误
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

// QueryInt 获取查询参数（整数）
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

// QueryInt64 获取查询参数（int64）
func (c *Context) QueryInt64(key string, defaultVal int64) int64 {
	val := c.Query(key)
	if val == "" {
		return defaultVal
	}
	i, _ := strconv.ParseInt(val, 10, 64)
	return i
}

// QueryInt64OrFail 获取查询参数（int64），无效则返回 400 错误
func (c *Context) QueryInt64OrFail(key string) (int64, bool) {
	val := c.Query(key)
	if val == "" {
		c.BadRequest("missing query parameter: " + key)
		return 0, false
	}
	i, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		c.BadRequest("invalid query parameter: " + key)
		return 0, false
	}
	return i, true
}

// QueryIntOrFail 获取查询参数（int），无效则返回 400 错误
func (c *Context) QueryIntOrFail(key string) (int, bool) {
	v, ok := c.QueryInt64OrFail(key)
	return int(v), ok
}

// QueryDefault 获取查询参数，带默认值
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
	decoder := json.NewDecoder(c.Request.Body)
	return decoder.Decode(v)
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

// JSON 返回 JSON 响应
func (c *Context) JSON(status int, v interface{}) {
	c.statusCode = status
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(status)
	json.NewEncoder(c.Writer).Encode(v)
}

// Success 返回成功响应（200）
func (c *Context) Success(v interface{}) {
	c.JSON(http.StatusOK, H{"data": v})
}

// Created 返回创建成功响应（201）
func (c *Context) Created(v interface{}) {
	c.JSON(http.StatusCreated, H{"data": v})
}

// NoContent 返回无内容响应（204）
func (c *Context) NoContent() {
	c.Writer.WriteHeader(http.StatusNoContent)
}

// Error 返回错误响应
func (c *Context) Error(status int, code, message string) {
	c.JSON(status, H{
		"error": H{
			"code":    code,
			"message": message,
		},
	})
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

// InternalErrorWrap 返回 500 错误，带调用链包装，自动记录日志
func (c *Context) InternalErrorWrap(err error, message string, metadata map[string]any) {
	se := errors.NewStructuredError(errors.CodeInternalError, message).
		WithFilePath(c.getCallerFilePath()).
		WithLine(c.getCallerLine()).
		AddCallFrame()

	// 如果有底层错误，包装它
	if err != nil {
		se = se.Wrap(err, message)
	}

	// 添加元数据
	for k, v := range metadata {
		se.WithMetadata(k, v)
	}

	// 记录到错误链
	c.errorChain = append(c.errorChain, se)

	// 自动记录到日志
	if globalLogger != nil {
		if err != nil {
			globalLogger.Printf("[ERROR] %s: %v (caller: %s:%d)", message, err, c.getCallerFilePath(), c.getCallerLine())
		} else {
			globalLogger.Printf("[ERROR] %s (caller: %s:%d)", message, c.getCallerFilePath(), c.getCallerLine())
		}
	}

	errResp := errors.NewErrorResponse(se)
	c.JSON(http.StatusInternalServerError, errResp)
}

// BadRequestWrap 返回 400 错误，带调用链包装，自动记录日志
func (c *Context) BadRequestWrap(err error, message string) {
	se := errors.NewStructuredError(errors.CodeBadRequest, message).
		WithFilePath(c.getCallerFilePath()).
		WithLine(c.getCallerLine()).
		AddCallFrame()

	if err != nil {
		se = se.Wrap(err, message)
	}

	c.errorChain = append(c.errorChain, se)

	// 自动记录到日志
	if globalLogger != nil && err != nil {
		globalLogger.Printf("[WARN] %s: %v", message, err)
	}

	c.JSON(http.StatusBadRequest, errors.NewErrorResponse(se))
}

// NotFoundWrap 返回 404 错误，带调用链包装，自动记录日志
func (c *Context) NotFoundWrap(err error, message string) {
	se := errors.NewStructuredError(errors.CodeNotFound, message).
		WithFilePath(c.getCallerFilePath()).
		WithLine(c.getCallerLine()).
		AddCallFrame()

	if err != nil {
		se = se.Wrap(err, message)
	}

	c.errorChain = append(c.errorChain, se)

	// 自动记录到日志
	if globalLogger != nil && err != nil {
		globalLogger.Printf("[WARN] %s: %v", message, err)
	}

	c.JSON(http.StatusNotFound, errors.NewErrorResponse(se))
}

// ValidationErrorWrap 返回验证错误，带调用链包装，自动记录日志
func (c *Context) ValidationErrorWrap(err error, field, message string) {
	se := errors.NewValidationError(field, "", message).
		WithFilePath(c.getCallerFilePath()).
		WithLine(c.getCallerLine()).
		AddCallFrame()

	if err != nil {
		se = se.Wrap(err, message)
	}

	c.errorChain = append(c.errorChain, se)

	// 自动记录到日志
	if globalLogger != nil && err != nil {
		globalLogger.Printf("[WARN] validation failed: %s - %v", message, err)
	}

	c.JSON(http.StatusUnprocessableEntity, errors.NewErrorResponse(se))
}

// GetErrorChain 获取错误链
func (c *Context) GetErrorChain() []*errors.StructuredError {
	return c.errorChain
}

// getCallerFilePath 获取调用者的文件路径
func (c *Context) getCallerFilePath() string {
	if _, file, _, ok := runtime.Caller(2); ok {
		return file
	}
	return ""
}

// getCallerLine 获取调用者的行号
func (c *Context) getCallerLine() int {
	if _, _, line, ok := runtime.Caller(2); ok {
		return line
	}
	return 0
}

// ValidationError 返回验证错误
func (c *Context) ValidationError(err error) {
	c.JSON(http.StatusUnprocessableEntity, H{
		"error": H{
			"code":    "VALIDATION_FAILED",
			"message": err.Error(),
		},
	})
}

// NotFoundIfNotFound 如果 err 是"未找到"错误，返回 404 并发送响应
// 返回 true 表示是 NotFound 错误（已发送响应），false 表示不是
func (c *Context) NotFoundIfNotFound(err error, resourceName string) bool {
	if err != nil {
		c.NotFoundWrap(err, resourceName+" not found")
		return true
	}
	return false
}

// SuccessIfNotNil 如果 v 不为 nil，返回成功响应
// 如果 v 为 nil，返回 404 并发送响应
// 返回 true 表示 v 不为 nil（已发送响应），false 表示 v 为 nil（已发送响应）
func (c *Context) SuccessIfNotNil(v interface{}, resourceName string) bool {
	if v == nil || (reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil()) {
		c.NotFound(resourceName + " not found")
		return false
	}
	c.Success(v)
	return true
}

// SuccessIfNotNilOrFail 类似 SuccessIfNotNil，但使用 Wrap 包装错误
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

// FailIfError 如果 err 不为 nil，发送 InternalErrorWrap 响应
// 返回 true 表示有错误（已发送响应），false 表示没有错误
func (c *Context) FailIfError(err error, message string) bool {
	if err != nil {
		c.InternalErrorWrap(err, message, nil)
		return true
	}
	return false
}

// FailIfErrorWithMeta 如果 err 不为 nil，发送 InternalErrorWrap 响应（带元数据）
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
			continue // 参数匹配
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
		if strings.HasPrefix(patternParts[i], ":") {
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
			fieldValue := rv.Field(i)
			if err := setStringValue(fieldValue, val); err != nil {
				return err
			}
		}
	}
	return nil
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// setStringValue 根据字段类型设置值
func setStringValue(v reflect.Value, s string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Int:
		i, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		v.SetInt(int64(i))
	case reflect.Int8:
		i, err := strconv.ParseInt(s, 10, 8)
		if err != nil {
			return err
		}
		v.SetInt(i)
	case reflect.Int16:
		i, err := strconv.ParseInt(s, 10, 16)
		if err != nil {
			return err
		}
		v.SetInt(i)
	case reflect.Int32:
		i, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return err
		}
		v.SetInt(i)
	case reflect.Int64:
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(i)
	case reflect.Uint:
		i, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetUint(i)
	case reflect.Float32:
		f, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return err
		}
		v.SetFloat(f)
	case reflect.Float64:
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
