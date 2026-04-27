package core

import (
	"encoding/json"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// Context 是请求上下文
type Context struct {
	Request         *http.Request
	Writer          http.ResponseWriter
	Params          map[string]string
	QueryArgs       urlValues
	handlers       []HandlerFunc
	handlerIndex   int
	statusCode     int
}

type urlValues map[string][]string

func newContext(w http.ResponseWriter, req *http.Request) *Context {
	return &Context{
		Request:     req,
		Writer:      w,
		Params:      make(map[string]string),
		QueryArgs:   urlValues(req.URL.Query()),
		handlers:    make([]HandlerFunc, 0),
		handlerIndex: -1, // 初始值为 -1，Next() 会先递增再调用
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

// QueryDefault 获取查询参数，带默认值
func (c *Context) QueryDefault(key, defaultVal string) string {
	val := c.Query(key)
	if val == "" {
		return defaultVal
	}
	return val
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

// ValidationError 返回验证错误
func (c *Context) ValidationError(err error) {
	c.JSON(http.StatusUnprocessableEntity, H{
		"error": H{
			"code":    "VALIDATION_FAILED",
			"message": err.Error(),
		},
	})
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
