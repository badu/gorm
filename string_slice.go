package gorm

import "strings"

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

func (s StrSlice) asString() string {
	return strings.Join(s, ",")
}
