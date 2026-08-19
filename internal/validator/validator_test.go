// internal/validator/validator_test.go
package validator

import (
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{

		{
			name:     "standard email",
			email:    "user@example.com",
			expected: true,
		},
		{
			name:     "email with subdomain",
			email:    "user@mail.example.com",
			expected: true,
		},
		{
			name:     "email with dots in local part",
			email:    "first.last@example.com",
			expected: true,
		},
		{
			name:     "email with hyphens",
			email:    "user-name@example-domain.com",
			expected: true,
		},
		{
			name:     "email with underscores",
			email:    "user_name@example.com",
			expected: true,
		},
		{
			name:     "email with plus sign",
			email:    "user+tag@example.com",
			expected: true,
		},
		{
			name:     "email with numbers",
			email:    "user123@example123.com",
			expected: true,
		},
		{
			name:     "minimal valid email",
			email:    "a@b.co",
			expected: true,
		},

		{
			name:     "too short - empty",
			email:    "",
			expected: false,
		},
		{
			name:     "too short - 2 chars",
			email:    "a@",
			expected: false,
		},

		{
			name:     "too long - 255 chars",
			email:    "user" + string(make([]byte, 250)) + "@example.com",
			expected: false,
		},

		{
			name:     "no @ symbol",
			email:    "userexample.com",
			expected: false,
		},
		{
			name:     "multiple @ symbols",
			email:    "user@@example.com",
			expected: false,
		},
		{
			name:     "@ at start",
			email:    "@example.com",
			expected: false,
		},
		{
			name:     "@ at end",
			email:    "user@",
			expected: false,
		},

		{
			name:     "no domain",
			email:    "user@",
			expected: false,
		},
		{
			name:     "domain without TLD",
			email:    "user@example",
			expected: false,
		},
		{
			name:     "domain with single char TLD",
			email:    "user@example.c",
			expected: false,
		},

		{
			name:     "local part too long - 65 chars",
			email:    string(make([]byte, 65)) + "@example.com",
			expected: false,
		},

		{
			name:     "double dots in local part",
			email:    "user..name@example.com",
			expected: false,
		},

		{
			name:     "local part starts with dot",
			email:    ".user@example.com",
			expected: false,
		},

		{
			name:     "local part ends with dot",
			email:    "user.@example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fl := &mockFieldLevel{
				fieldValue: tt.email,
			}

			result := validateEmail(fl)
			assert.Equal(t, tt.expected, result, "validateEmail(%q)", tt.email)
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password any
		expected bool
	}{

		{
			name:     "lowercase letters only",
			password: "abcdefgh",
			expected: true,
		},
		{
			name:     "uppercase letters only",
			password: "ABCDEFGH",
			expected: true,
		},
		{
			name:     "mixed case letters",
			password: "AbCdEfGh",
			expected: true,
		},
		{
			name:     "letters with numbers",
			password: "password123",
			expected: true,
		},
		{
			name:     "letters with special chars",
			password: "password!@#",
			expected: true,
		},
		{
			name:     "complex password",
			password: "Str0ng!Pass#2024",
			expected: true,
		},
		{
			name:     "single letter",
			password: "a",
			expected: true,
		},
		{
			name:     "letter with spaces",
			password: "pass word",
			expected: true,
		},

		{
			name:     "numbers only",
			password: "12345678",
			expected: false,
		},
		{
			name:     "special chars only",
			password: "!@#$%^&*",
			expected: false,
		},
		{
			name:     "numbers and special chars",
			password: "123!@#",
			expected: false,
		},
		{
			name:     "spaces only",
			password: "        ",
			expected: false,
		},

		{
			name:     "empty string",
			password: "",
			expected: false,
		},

		{
			name:     "nil pointer",
			password: (*string)(nil),
			expected: false,
		},
		{
			name: "pointer to valid password",
			password: func() *string {
				s := "password123"
				return &s
			}(),
			expected: true,
		},
		{
			name: "pointer to invalid password",
			password: func() *string {
				s := "12345678"
				return &s
			}(),
			expected: false,
		},

		{
			name:     "integer type",
			password: 12345,
			expected: false,
		},
		{
			name:     "boolean type",
			password: true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fl := &mockFieldLevel{
				fieldValue: tt.password,
			}

			result := validatePassword(fl)
			assert.Equal(t, tt.expected, result, "validatePassword(%v)", tt.password)
		})
	}
}

func TestValidator_ValidateStruct(t *testing.T) {
	v := NewValidator()

	t.Run("valid struct with strict_email", func(t *testing.T) {
		type TestStruct struct {
			Email string `validate:"required,strict_email"`
		}

		s := TestStruct{Email: "user@example.com"}
		err := v.ValidateStruct(s)
		assert.NoError(t, err)
	})

	t.Run("invalid email with strict_email", func(t *testing.T) {
		type TestStruct struct {
			Email string `validate:"required,strict_email"`
		}

		s := TestStruct{Email: "invalid-email"}
		err := v.ValidateStruct(s)
		require.Error(t, err)

		validationErrs, ok := err.(validator.ValidationErrors)
		require.True(t, ok, "Error should be ValidationErrors")
		require.Len(t, validationErrs, 1)

		fe := validationErrs[0]
		assert.Equal(t, "Email", fe.Field())
		assert.Equal(t, "strict_email", fe.Tag())
	})

	t.Run("valid struct with strong_password", func(t *testing.T) {
		type TestStruct struct {
			Password string `validate:"required,strong_password"`
		}

		s := TestStruct{Password: "StrongPass123"}
		err := v.ValidateStruct(s)
		assert.NoError(t, err)
	})

	t.Run("invalid password with strong_password", func(t *testing.T) {
		type TestStruct struct {
			Password string `validate:"required,strong_password"`
		}

		s := TestStruct{Password: "12345678"}
		err := v.ValidateStruct(s)
		require.Error(t, err)

		validationErrs, ok := err.(validator.ValidationErrors)
		require.True(t, ok)
		require.Len(t, validationErrs, 1)

		fe := validationErrs[0]
		assert.Equal(t, "Password", fe.Field())
		assert.Equal(t, "strong_password", fe.Tag())
	})

	t.Run("struct with multiple validations", func(t *testing.T) {
		type TestStruct struct {
			Email    string `validate:"required,strict_email"`
			Password string `validate:"required,min=8,max=72,strong_password"`
		}

		s := TestStruct{
			Email:    "user@example.com",
			Password: "StrongPass123",
		}
		err := v.ValidateStruct(s)
		assert.NoError(t, err)
	})

	t.Run("struct with multiple validation errors", func(t *testing.T) {
		type TestStruct struct {
			Email    string `validate:"required,strict_email"`
			Password string `validate:"required,min=8,max=72,strong_password"`
		}

		s := TestStruct{
			Email:    "invalid",
			Password: "123",
		}
		err := v.ValidateStruct(s)
		require.Error(t, err)

		validationErrs, ok := err.(validator.ValidationErrors)
		require.True(t, ok)
		assert.GreaterOrEqual(t, len(validationErrs), 2)
	})

	t.Run("struct with missing required field", func(t *testing.T) {
		type TestStruct struct {
			Email string `validate:"required,strict_email"`
		}

		s := TestStruct{Email: ""}
		err := v.ValidateStruct(s)
		require.Error(t, err)

		validationErrs, ok := err.(validator.ValidationErrors)
		require.True(t, ok)
		require.Len(t, validationErrs, 1)

		fe := validationErrs[0]
		assert.Equal(t, "Email", fe.Field())
		assert.Equal(t, "required", fe.Tag())
	})
}

func TestNewValidator(t *testing.T) {
	v := NewValidator()

	require.NotNil(t, v, "NewValidator should return non-nil Validator")
	require.NotNil(t, v.validate, "Validator.validate should be non-nil")

	type TestStruct struct {
		Email    string `validate:"strict_email"`
		Password string `validate:"strong_password"`
	}

	s := TestStruct{
		Email:    "user@example.com",
		Password: "password123",
	}

	err := v.ValidateStruct(s)
	assert.NoError(t, err, "Custom validators should be registered")
}

type mockFieldLevel struct {
	fieldValue any
}

func (m *mockFieldLevel) Field() reflect.Value {
	return reflect.ValueOf(m.fieldValue)
}

func (m *mockFieldLevel) Top() reflect.Value {
	return reflect.Value{}
}

func (m *mockFieldLevel) Parent() reflect.Value {
	return reflect.Value{}
}

func (m *mockFieldLevel) FieldName() string {
	return ""
}

func (m *mockFieldLevel) StructFieldName() string {
	return ""
}

func (m *mockFieldLevel) Param() string {
	return ""
}

func (m *mockFieldLevel) GetTag() string {
	return ""
}

func (m *mockFieldLevel) ExtractType(field reflect.Value) (reflect.Value, reflect.Kind, bool) {
	return field, field.Kind(), false
}

func (m *mockFieldLevel) GetStructFieldOK() (reflect.Value, reflect.Kind, bool) {
	return reflect.Value{}, reflect.Invalid, false
}

func (m *mockFieldLevel) GetStructFieldOK2() (reflect.Value, reflect.Kind, bool, bool) {
	return reflect.Value{}, reflect.Invalid, false, false
}

func (m *mockFieldLevel) GetStructFieldOKAdvanced(field reflect.Value, path string) (reflect.Value, reflect.Kind, bool) {
	return reflect.Value{}, reflect.Invalid, false
}

func (m *mockFieldLevel) GetStructFieldOKAdvanced2(field reflect.Value, path string) (reflect.Value, reflect.Kind, bool, bool) {
	return reflect.Value{}, reflect.Invalid, false, false
}
