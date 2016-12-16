package gorm

import (
	"strings"
)

// GetErrors get all happened errors
func (errs GormErrors) GetErrors() []error {
	return errs
}

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

// Add add an error
func (errs GormErrors) Error() string {
	var errors = []string{}
	for _, e := range errs {
		errors = append(errors, e.Error())
	}
	return strings.Join(errors, "; ")
}
