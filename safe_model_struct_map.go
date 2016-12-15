package gorm

import (
	"reflect"
	"sync"
)

type (
	safeModelStructsMap struct {
		m map[reflect.Type]*ModelStruct
		l *sync.RWMutex
	}
)

var (
	ModelStructsMap = &safeModelStructsMap{l: new(sync.RWMutex), m: make(map[reflect.Type]*ModelStruct)}
)

func (s *safeModelStructsMap) Set(key reflect.Type, value *ModelStruct) {
	s.l.Lock()
	defer s.l.Unlock()
	s.m[key] = value
}

func (s *safeModelStructsMap) Get(key reflect.Type) *ModelStruct {
	s.l.RLock()
	defer s.l.RUnlock()
	return s.m[key]
}

//for listing in debug mode
func (s *safeModelStructsMap) M() map[reflect.Type]*ModelStruct {
	s.l.RLock()
	defer s.l.RUnlock()
	return s.m
}
