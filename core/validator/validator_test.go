package validator

import (
	"testing"
)

type TestUser struct {
	Name  string `validate:"required"`
	Email string `validate:"required|email"`
	Age   int    `validate:"gte:0|lte:150"`
}

func TestValidate_Required(t *testing.T) {
	user := &TestUser{Name: "", Email: "test@example.com"}
	err := Validate(user)
	if err == nil {
		t.Error("expected error for empty name")
	}
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
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%s) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestValidate_Int(t *testing.T) {
	tests := []struct {
		age    int
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
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(age=%d) error = %v, wantErr %v", tt.age, err, tt.wantErr)
			}
		})
	}
}

func TestValidate_Min(t *testing.T) {
	type MinTest struct {
		Value int `validate:"min:10"`
	}

	tests := []struct {
		val    int
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
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(min=%d) error = %v, wantErr %v", tt.val, err, tt.wantErr)
			}
		})
	}
}

func TestValidate_Max(t *testing.T) {
	type MaxTest struct {
		Value int `validate:"max:100"`
	}

	tests := []struct {
		val    int
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
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(max=%d) error = %v, wantErr %v", tt.val, err, tt.wantErr)
			}
		})
	}
}

func TestValidate_Len(t *testing.T) {
	type LenTest struct {
		Value string `validate:"len:5"`
	}

	tests := []struct {
		val    string
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
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(len=%s) error = %v, wantErr %v", tt.val, err, tt.wantErr)
			}
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
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(phone=%s) error = %v, wantErr %v", tt.phone, err, tt.wantErr)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{Field: "Email", Message: "invalid email"}
	if err.Error() != "invalid email" {
		t.Errorf("expected 'invalid email', got '%s'", err.Error())
	}
}
