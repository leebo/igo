package validator

import (
	"reflect"
	"testing"

	errorspkg "github.com/leebo/igo/core/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestUser struct {
	Name  string `validate:"required"`
	Email string `validate:"required|email"`
	Age   int    `validate:"gte:0|lte:150"`
}

func TestValidate_Required(t *testing.T) {
	user := &TestUser{Name: "", Email: "test@example.com"}
	err := Validate(user)
	assert.Error(t, err)
}

func TestValidate_Email(t *testing.T) {
	tests := []struct {
		email   string
		wantErr bool
	}{
		{"test@example.com", false},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			user := &TestUser{Name: "Test", Email: tt.email}
			err := Validate(user)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestValidate_Int(t *testing.T) {
	tests := []struct {
		age     int
		wantErr bool
	}{
		{0, false},
		{50, false},
		{150, false},
		{-1, true},
		{151, true},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.age)), func(t *testing.T) {
			user := &TestUser{Name: "Test", Email: "test@example.com", Age: tt.age}
			err := Validate(user)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestValidate_Min(t *testing.T) {
	type MinTest struct {
		Value int `validate:"min:10"`
	}

	tests := []struct {
		val     int
		wantErr bool
	}{
		{10, false},
		{15, false},
		{9, true},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.val)), func(t *testing.T) {
			m := &MinTest{Value: tt.val}
			err := Validate(m)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestValidate_Max(t *testing.T) {
	type MaxTest struct {
		Value int `validate:"max:100"`
	}

	tests := []struct {
		val     int
		wantErr bool
	}{
		{100, false},
		{50, false},
		{101, true},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.val)), func(t *testing.T) {
			m := &MaxTest{Value: tt.val}
			err := Validate(m)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestValidate_Len(t *testing.T) {
	type LenTest struct {
		Value string `validate:"len:5"`
	}

	tests := []struct {
		val     string
		wantErr bool
	}{
		{"hello", false},
		{"hi", true},
		{"toolong", true},
	}

	for _, tt := range tests {
		t.Run(tt.val, func(t *testing.T) {
			m := &LenTest{Value: tt.val}
			err := Validate(m)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestValidate_Regex(t *testing.T) {
	type RegexTest struct {
		Phone string `validate:"regex:^1[3-9]\\d{9}$"`
	}

	tests := []struct {
		phone   string
		wantErr bool
	}{
		{"13812345678", false},
		{"12345678901", true},
		{"abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			m := &RegexTest{Phone: tt.phone}
			err := Validate(m)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{Field: "Email", Message: "invalid email"}
	assert.Equal(t, "invalid email", err.Error())
}

func TestRuleRegistryAndValidateValue(t *testing.T) {
	registry := NewRuleRegistry()
	enum := &EnumRule{}
	registry.Register(enum)

	assert.Equal(t, enum, registry.Get("enum"))
	assert.Contains(t, registry.List(), "enum")
	assert.NotNil(t, DefaultRegistry())
	assert.Equal(t, "enum", enum.Name())
	assert.Contains(t, enum.Message("Role"), "Role")

	assert.Nil(t, ValidateValue(reflect.ValueOf("admin"), []string{"enum:admin"}, "Role", registry))
	err := ValidateValue(reflect.ValueOf("guest"), []string{"enum:admin"}, "Role", registry)
	require.NotNil(t, err)
	assert.Equal(t, errorspkg.CodeValidation, err.Code)

	assert.Nil(t, ValidateValue(reflect.ValueOf("anything"), []string{"unknown"}, "Field", registry))
}

func TestEqFieldRuleAndParseValidationTag(t *testing.T) {
	rule := &EqFieldRule{}

	assert.Equal(t, "eqfield", rule.Name())
	assert.Contains(t, rule.Message("PasswordConfirm"), "PasswordConfirm")
	assert.Nil(t, rule.Validate(reflect.ValueOf("x"), map[string]string{"0": "Password"}))
	assert.Nil(t, ParseValidationTag(""))
	assert.Equal(t, []string{"required", "email"}, ParseValidationTag("required|email"))
}

func TestValidateAdditionalRules(t *testing.T) {
	type RuleTarget struct {
		Count    int     `validate:"gt:1|lt:5"`
		Score    float64 `validate:"gt:1.5|lt:9.5"`
		ID       string  `validate:"uuid"`
		Website  string  `validate:"url"`
		Password string  `validate:"required"`
		Confirm  string  `validate:"eqfield:Password"`
	}

	valid := RuleTarget{
		Count:    2,
		Score:    2.5,
		ID:       "123e4567-e89b-12d3-a456-426614174000",
		Website:  "https://example.com",
		Password: "secret",
		Confirm:  "secret",
	}
	assert.NoError(t, Validate(valid))

	tests := []RuleTarget{
		{Count: 1, Score: 2.5, ID: valid.ID, Website: valid.Website, Password: "secret", Confirm: "secret"},
		{Count: 2, Score: 10, ID: valid.ID, Website: valid.Website, Password: "secret", Confirm: "secret"},
		{Count: 2, Score: 2.5, ID: "bad", Website: valid.Website, Password: "secret", Confirm: "secret"},
		{Count: 2, Score: 2.5, ID: valid.ID, Website: "ftp://example.com", Password: "secret", Confirm: "secret"},
		{Count: 2, Score: 2.5, ID: valid.ID, Website: valid.Website, Password: "secret", Confirm: "different"},
	}

	for _, tt := range tests {
		assert.Error(t, Validate(tt))
	}

	type MissingField struct {
		Confirm string `validate:"eqfield:Password"`
	}
	assert.Error(t, Validate(MissingField{Confirm: "secret"}))
}
