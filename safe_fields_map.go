package gorm

import (
	"errors"
	"fmt"
	"sync"
)

type (
	fieldsMap struct {
		aliases map[string]*StructField
		locker  *sync.RWMutex
		fields  StructFields
	}
)

func (s *fieldsMap) Add(field *StructField) error {
	if s.locker == nil {
		return errors.New("fieldsMap ERROR !!! NOT INITED!")
	}
	s.locker.Lock()
	defer s.locker.Unlock()
	_, hasGetName := s.aliases[field.GetName()]
	_, hasDBName := s.aliases[field.DBName]
	if hasGetName || hasDBName {
		//replace in slice, even if we shouldn't (it's not correct to have this behavior)
		for index, existingField := range s.fields {
			if existingField.GetName() == field.GetName() || existingField.DBName == field.DBName {
				s.fields[index] = field
				break
			}
		}

	} else {
		s.fields.add(field)
	}
	s.aliases[field.GetName()] = field
	s.aliases[field.DBName] = field
	return nil
}

func (s *fieldsMap) Get(key string) (*StructField, bool) {
	if s.locker == nil {
		fmt.Errorf("fieldsMap ERROR : not inited")
		return nil, false
	}
	s.locker.RLock()
	defer s.locker.RUnlock()
	//If the requested key doesn't exist, we get the value type's zero value - avoiding that
	val, ok := s.aliases[key]
	return val, ok
}

func (s fieldsMap) Fields() StructFields {
	if s.locker == nil {
		fmt.Errorf("fieldsMap ERROR : not inited")
		return nil
	}
	return s.fields
}

func (s fieldsMap) PrimaryFields() StructFields {
	if s.locker == nil {
		fmt.Errorf("fieldsMap ERROR : not inited")
		return nil
	}
	s.locker.RLock()
	defer s.locker.RUnlock()
	var result StructFields
	for _, field := range s.fields {
		if field.IsPrimaryKey() {
			result.add(field)
		}
	}
	return result
}
