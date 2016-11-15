package gorm

import (
	"strings"
	"reflect"
	"database/sql/driver"
)

//============================================
// Slice of strings for better reading
//============================================
type StrSlice []string

//shorter than append, better reading
func (s *StrSlice) add(str string) {
	*s = append(*s, str)
}

func (s *StrSlice) len() int {
	return len(*s)
}

// getRIndex get right index from string slice
func (s StrSlice) rIndex(str string) int {
	for i := s.len() - 1; i >= 0; i-- {
		if s[i] == str {
			return i
		}
	}
	return -1
}

//inserts in the slice
func (s *StrSlice) insertAt(index int, name string) {
	*s = append((*s)[:index], append([]string{name}, (*s)[index:]...)...)
}

func (s *StrSlice) commaLoad(target string) {
	*s = strings.Split(target, ",")
}

func (s StrSlice) slice() []string {
	return s
}


// getValueFromFields return given fields's value
func (s StrSlice) getValueFromFields(value reflect.Value) []interface{} {
	var results []interface{}
	// If value is a nil pointer, Indirect returns a zero Value!
	// Therefor we need to check for a zero value,
	// as FieldByName could panic
	if indirectValue := reflect.Indirect(value); indirectValue.IsValid() {
		for _, fieldName := range s {
			if fieldValue := indirectValue.FieldByName(fieldName); fieldValue.IsValid() {
				result := fieldValue.Interface()
				if r, ok := result.(driver.Valuer); ok {
					result, _ = r.Value()
				}
				results = append(results, result)
			}
		}
	}
	return results
}