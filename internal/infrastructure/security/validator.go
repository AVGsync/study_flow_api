package security

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

type Validator struct {
	validator *validator.Validate
}

func NewValidator() *Validator {
	return &Validator{
		validator: validator.New(),
	}
}

func (v *Validator) ValidateStruct(s interface{}) (bool, error) {
	err := v.validator.Struct(s)
	if err == nil {
		return true, nil
	}

	if _, ok := err.(*validator.InvalidValidationError); ok {
		return false, fmt.Errorf("internal validation error")
	}

	verrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return false, fmt.Errorf("validation error")
	}

	msgs := make([]string, 0, len(verrs))
	for _, fe := range verrs {
		msgs = append(msgs, humanMessage(fe))
	}

	return false, errors.New(strings.Join(msgs, "; "))
}

func humanMessage(fe validator.FieldError) string {
	field := strings.ToLower(fe.Field())

	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("поле %s обязательно", field)
	case "min":
		return fmt.Sprintf("поле %s должно быть не короче %s символов", field, fe.Param())
	case "max":
		return fmt.Sprintf("поле %s должно быть не длиннее %s символов", field, fe.Param())
	case "email":
		return fmt.Sprintf("поле %s должно быть валидным email", field)
	default:
		return fmt.Sprintf("поле %s не прошло проверку %s", field, fe.Tag())
	}
}
