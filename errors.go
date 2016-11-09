package gorm

import (
	"strings"
)

type (
	errorsInterface interface {
		GetErrors() []error
	}
	// Errors contains all happened errors
	GormErrors []error

)
// GetErrors get all happened errors
func (errs GormErrors) GetErrors() []error {
	return errs
}

// Add add an error
func (errs GormErrors) Add(newErrors ...error) GormErrors {
	for _, err := range newErrors {
		if errors, ok := err.(GormErrors); ok {
			errs = errs.Add(errors...)
		} else {
			ok = true
			for _, e := range errs {
				if err == e {
					ok = false
				}
			}
			if ok {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

// Error format happened errors
func (errs GormErrors) Error() string {
	var errors = []string{}
	for _, e := range errs {
		errors = append(errors, e.Error())
	}
	return strings.Join(errors, "; ")
}
