package gorm

import (
	"strings"
)

// GetErrors get all happened errors
func (e GormErrors) GetErrors() []error {
	return e
}

func (e GormErrors) Add(newErrors ...error) GormErrors {
	for _, err := range newErrors {
		if errors, ok := err.(GormErrors); ok {
			e = e.Add(errors...)
		} else {
			ok = true
			for _, e := range e {
				if err == e {
					ok = false
				}
			}
			if ok {
				e = append(e, err)
			}
		}
	}
	return e
}

// Add add an error
func (e GormErrors) Error() string {
	var errors []string
	for _, e := range e {
		errors = append(errors, e.Error())
	}
	return strings.Join(errors, "; ")
}
