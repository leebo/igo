package validator

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// Validate 验证结构体，接受指针或值类型
func Validate(v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("validator: expected struct, got %s", rv.Kind())
	}
	t := rv.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := rv.Field(i)

		validateTag := field.Tag.Get("validate")
		if validateTag == "" || validateTag == "-" {
			continue
		}

		rules := strings.Split(validateTag, "|")
		for _, rule := range rules {
			rule = strings.TrimSpace(rule)
			if err := applyRule(fieldValue, field.Name, rule, rv); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyRule 对单个字段应用一条验证规则
// structVal 用于 eqfield 等需要跨字段引用的规则
func applyRule(v reflect.Value, fieldName, rule string, structVal reflect.Value) error {
	parts := strings.SplitN(rule, ":", 2)
	name := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	switch name {
	case "required":
		if isEmpty(v) {
			return &ValidationError{Field: fieldName, Rule: "required", Message: fieldName + " is required"}
		}
	case "email":
		if !isEmpty(v) && !isEmail(v.String()) {
			return &ValidationError{Field: fieldName, Rule: "email", Message: fieldName + " must be a valid email"}
		}
	case "min":
		if !isEmpty(v) && !minValue(v, args) {
			return &ValidationError{Field: fieldName, Rule: "min", Message: fieldName + " must be at least " + args}
		}
	case "max":
		if !isEmpty(v) && !maxValue(v, args) {
			return &ValidationError{Field: fieldName, Rule: "max", Message: fieldName + " must be at most " + args}
		}
	case "gte":
		if !isEmpty(v) && !gteValue(v, args) {
			return &ValidationError{Field: fieldName, Rule: "gte", Message: fieldName + " must be >= " + args}
		}
	case "lte":
		if !isEmpty(v) && !lteValue(v, args) {
			return &ValidationError{Field: fieldName, Rule: "lte", Message: fieldName + " must be <= " + args}
		}
	case "gt":
		if !isEmpty(v) && !gtValue(v, args) {
			return &ValidationError{Field: fieldName, Rule: "gt", Message: fieldName + " must be > " + args}
		}
	case "lt":
		if !isEmpty(v) && !ltValue(v, args) {
			return &ValidationError{Field: fieldName, Rule: "lt", Message: fieldName + " must be < " + args}
		}
	case "len":
		if !isEmpty(v) && !lenValue(v, args) {
			return &ValidationError{Field: fieldName, Rule: "len", Message: fieldName + " length must be " + args}
		}
	case "regex":
		if !isEmpty(v) && !matchRegex(v.String(), args) {
			return &ValidationError{Field: fieldName, Rule: "regex", Message: fieldName + " format is invalid"}
		}
	case "uuid":
		if !isEmpty(v) && !isUUID(v.String()) {
			return &ValidationError{Field: fieldName, Rule: "uuid", Message: fieldName + " must be a valid UUID"}
		}
	case "url":
		if !isEmpty(v) && !isURL(v.String()) {
			return &ValidationError{Field: fieldName, Rule: "url", Message: fieldName + " must be a valid URL"}
		}
	case "enum":
		if !isEmpty(v) && v.Kind() == reflect.String {
			opts := strings.Split(args, ",")
			found := false
			for _, opt := range opts {
				if strings.TrimSpace(opt) == v.String() {
					found = true
					break
				}
			}
			if !found {
				return &ValidationError{Field: fieldName, Rule: "enum", Message: fieldName + " must be one of: " + args}
			}
		}
	case "eqfield":
		if !isEmpty(v) {
			other := structVal.FieldByName(args)
			if !other.IsValid() {
				return &ValidationError{Field: fieldName, Rule: "eqfield", Message: "eqfield: field " + args + " not found"}
			}
			if fmt.Sprintf("%v", v.Interface()) != fmt.Sprintf("%v", other.Interface()) {
				return &ValidationError{Field: fieldName, Rule: "eqfield", Message: fieldName + " must equal " + args}
			}
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
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
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

func gteValue(v reflect.Value, arg string) bool { return minValue(v, arg) }
func lteValue(v reflect.Value, arg string) bool { return maxValue(v, arg) }

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

// ValidationError 验证错误，包含字段名和规则
type ValidationError struct {
	Field   string
	Rule    string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
