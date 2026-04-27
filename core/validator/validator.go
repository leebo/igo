package validator

import (
	"regexp"
	"reflect"
	"strconv"
	"strings"
)

// Validate 验证结构体
func Validate(v interface{}) error {
	t := reflect.TypeOf(v).Elem()
	rv := reflect.ValueOf(v).Elem()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := rv.Field(i)

		// 获取验证标签
		validateTag := field.Tag.Get("validate")
		if validateTag == "" || validateTag == "-" {
			continue
		}

		// 解析标签
		rules := strings.Split(validateTag, "|")
		for _, rule := range rules {
			rule = strings.TrimSpace(rule)
			if err := applyRule(fieldValue, field.Name, rule); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyRule(v reflect.Value, fieldName, rule string) error {
	// 解析规则（如 "min:2", "email", "required"）
	parts := strings.SplitN(rule, ":", 2)
	name := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	switch name {
	case "required":
		if isEmpty(v) {
			return &ValidationError{Field: fieldName, Message: fieldName + " is required"}
		}
	case "email":
		if !isEmpty(v) && !isEmail(v.String()) {
			return &ValidationError{Field: fieldName, Message: fieldName + " must be a valid email"}
		}
	case "min":
		if !isEmpty(v) && !minValue(v, args) {
			return &ValidationError{Field: fieldName, Message: fieldName + " must be at least " + args}
		}
	case "max":
		if !isEmpty(v) && !maxValue(v, args) {
			return &ValidationError{Field: fieldName, Message: fieldName + " must be at most " + args}
		}
	case "gte":
		if !isEmpty(v) && !gteValue(v, args) {
			return &ValidationError{Field: fieldName, Message: fieldName + " must be greater than or equal to " + args}
		}
	case "lte":
		if !isEmpty(v) && !lteValue(v, args) {
			return &ValidationError{Field: fieldName, Message: fieldName + " must be less than or equal to " + args}
		}
	case "gt":
		if !isEmpty(v) && !gtValue(v, args) {
			return &ValidationError{Field: fieldName, Message: fieldName + " must be greater than " + args}
		}
	case "lt":
		if !isEmpty(v) && !ltValue(v, args) {
			return &ValidationError{Field: fieldName, Message: fieldName + " must be less than " + args}
		}
	case "len":
		if !isEmpty(v) && !lenValue(v, args) {
			return &ValidationError{Field: fieldName, Message: fieldName + " length must be " + args}
		}
	case "regex":
		if !isEmpty(v) && !matchRegex(v.String(), args) {
			return &ValidationError{Field: fieldName, Message: fieldName + " format is invalid"}
		}
	case "uuid":
		if !isEmpty(v) && !isUUID(v.String()) {
			return &ValidationError{Field: fieldName, Message: fieldName + " must be a valid UUID"}
		}
	case "url":
		if !isEmpty(v) && !isURL(v.String()) {
			return &ValidationError{Field: fieldName, Message: fieldName + " must be a valid URL"}
		}
	}
	return nil
}

func isEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool()
	default:
		return false
	}
}

func isEmail(s string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(s)
}

func minValue(v reflect.Value, arg string) bool {
	switch v.Kind() {
	case reflect.String:
		i, _ := strconv.Atoi(arg)
		return len(v.String()) >= i
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, _ := strconv.ParseInt(arg, 10, 64)
		return v.Int() >= i
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, _ := strconv.ParseUint(arg, 10, 64)
		return v.Uint() >= i
	case reflect.Float32, reflect.Float64:
		f, _ := strconv.ParseFloat(arg, 64)
		return v.Float() >= f
	}
	return true
}

func maxValue(v reflect.Value, arg string) bool {
	switch v.Kind() {
	case reflect.String:
		i, _ := strconv.Atoi(arg)
		return len(v.String()) <= i
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, _ := strconv.ParseInt(arg, 10, 64)
		return v.Int() <= i
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, _ := strconv.ParseUint(arg, 10, 64)
		return v.Uint() <= i
	case reflect.Float32, reflect.Float64:
		f, _ := strconv.ParseFloat(arg, 64)
		return v.Float() <= f
	}
	return true
}

func gteValue(v reflect.Value, arg string) bool {
	return minValue(v, arg)
}

func lteValue(v reflect.Value, arg string) bool {
	return maxValue(v, arg)
}

func gtValue(v reflect.Value, arg string) bool {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, _ := strconv.ParseInt(arg, 10, 64)
		return v.Int() > i
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, _ := strconv.ParseUint(arg, 10, 64)
		return v.Uint() > i
	case reflect.Float32, reflect.Float64:
		f, _ := strconv.ParseFloat(arg, 64)
		return v.Float() > f
	}
	return true
}

func ltValue(v reflect.Value, arg string) bool {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, _ := strconv.ParseInt(arg, 10, 64)
		return v.Int() < i
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, _ := strconv.ParseUint(arg, 10, 64)
		return v.Uint() < i
	case reflect.Float32, reflect.Float64:
		f, _ := strconv.ParseFloat(arg, 64)
		return v.Float() < f
	}
	return true
}

func lenValue(v reflect.Value, arg string) bool {
	i, _ := strconv.Atoi(arg)
	switch v.Kind() {
	case reflect.String:
		return len(v.String()) == i
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == int64(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == uint64(i)
	}
	return true
}

func matchRegex(s, pattern string) bool {
	re := regexp.MustCompile(pattern)
	return re.MatchString(s)
}

func isUUID(s string) bool {
	uuidRegex := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	return uuidRegex.MatchString(s)
}

func isURL(s string) bool {
	urlRegex := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
	return urlRegex.MatchString(s)
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
