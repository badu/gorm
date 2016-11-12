package gorm

import (
	"sync"
)

type (
	fieldsMap struct {
		m map[string]*StructField
		l *sync.RWMutex
	}
)

func (s *fieldsMap) Set(key string, value *StructField) {
	s.l.Lock()
	defer s.l.Unlock()
	s.m[key] = value
}

func (s *fieldsMap) Get(key string) (*StructField, bool) {
	s.l.RLock()
	defer s.l.RUnlock()
	//If the requested key doesn't exist, we get the value type's zero value - avoiding that
	val, ok := s.m[key]
	return val, ok
}
