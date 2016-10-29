package gorm

import (
	"strings"
)

// GetErrors get all happened errors
func (errs Errors) GetErrors() []error {
	return errs.errors
}

// Add add an error
func (errs *Errors) Add(err error) {
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
func (errs Errors) Error() string {
	var errors = []string{}
	for _, e := range errs.errors {
		errors = append(errors, e.Error())
	}
	return strings.Join(errors, "; ")
}
