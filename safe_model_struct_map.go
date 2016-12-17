package gorm

import (
	"reflect"
)

func (s *safeModelStructsMap) set(key reflect.Type, value *ModelStruct) {
	s.l.Lock()
	defer s.l.Unlock()
	s.m[key] = value
}

func (s *safeModelStructsMap) get(key reflect.Type) *ModelStruct {
	s.l.RLock()
	defer s.l.RUnlock()
	return s.m[key]
}

//for listing in debug mode
func (s *safeModelStructsMap) getMap() map[reflect.Type]*ModelStruct {
	s.l.RLock()
	defer s.l.RUnlock()
	return s.m
}
