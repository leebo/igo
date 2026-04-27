package core

import "errors"

// 常用错误
var (
	ErrBodyRequired    = errors.New("request body is required")
	ErrUnsupportedType  = errors.New("unsupported field type")
	ErrInvalidJSON      = errors.New("invalid JSON")
)
