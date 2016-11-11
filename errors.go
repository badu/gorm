package gorm

import (
	"strings"
	"errors"
)

type (
	errorsInterface interface {
		GetErrors() []error
	}
	// Errors contains all happened errors
	GormErrors []error
)
var (
	// ErrRecordNotFound record not found error, happens when haven't find any matched data when looking up with a struct
	ErrRecordNotFound = errors.New("record not found")

	// ErrInvalidSQL invalid SQL error, happens when you passed invalid SQL
	ErrInvalidSQL = errors.New("invalid SQL")

	// ErrInvalidTransaction invalid transaction when you are trying to `Commit` or `Rollback`
	ErrInvalidTransaction = errors.New("no valid transaction")

	// ErrCantStartTransaction can't start transaction when you are trying to start one with `Begin`
	ErrCantStartTransaction = errors.New("can't start transaction")

	// ErrUnaddressable unaddressable value
	ErrUnaddressable = errors.New("using unaddressable value")
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
