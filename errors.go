package gorm

import (
	"strings"
)

type (
	errorsInterface interface {
		GetErrors() []error
	}
	// Errors contains all happened errors
	GormErrors struct {
		errors []error
	}
)
// GetErrors get all happened errors
func (errs GormErrors) GetErrors() []error {
	return errs.errors
}

// Add add an error
func (errs *GormErrors) Add(err error) {
	if errors, ok := err.(errorsInterface); ok {
		for _, err := range errors.GetErrors() {
			errs.Add(err)
		}
	} else {
		for _, e := range errs.errors {
			if err == e {
				return
			}
		}
		errs.errors = append(errs.errors, err)
	}
}

// Error format happened errors
func (errs GormErrors) Error() string {
	var errors = []string{}
	for _, e := range errs.errors {
		errors = append(errors, e.Error())
	}
	return strings.Join(errors, "; ")
}
