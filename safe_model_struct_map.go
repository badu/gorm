package gorm

import "reflect"

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
