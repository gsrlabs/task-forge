// internal/validator/validator.go
package validator

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var emailRegex = regexp.MustCompile(`^(?P<local>[a-zA-Z0-9._%+\-]+)@(?P<domain>([a-zA-Z0-9\-]+\.)+[a-zA-Z]{2,})$`)

type Validator struct {
	validate *validator.Validate
}

func NewValidator() *Validator {
	v := validator.New()

	_ = v.RegisterValidation("strict_email", validateEmail)
	_ = v.RegisterValidation("strong_password", validatePassword)

	return &Validator{validate: v}
}

func (v *Validator) ValidateStruct(s interface{}) error {
	return v.validate.Struct(s)
}

func validateEmail(fl validator.FieldLevel) bool {
	email := fl.Field().String()

	if len(email) < 3 || len(email) > 254 {
		return false
	}

	if !emailRegex.MatchString(email) {
		return false
	}

	parts := strings.Split(email, "@")
	if len(parts[0]) > 64 {
		return false
	}

	if strings.Contains(parts[0], "..") || strings.HasPrefix(parts[0], ".") || strings.HasSuffix(parts[0], ".") {
		return false
	}

	return true
}

func validatePassword(fl validator.FieldLevel) bool {
	field := fl.Field()

	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return false
		}
		field = field.Elem()
	}

	if field.Kind() != reflect.String {
		return false
	}

	password := field.String()

	for _, r := range password {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}
