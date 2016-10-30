package gorm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

func NewStructField(fieldStruct reflect.StructField) (*StructField, error) {
	result := &StructField{
		Struct: fieldStruct,
		Names:  []string{fieldStruct.Name},
	}
	//field should process itself the tag settings
	err := result.parseTagSettings()
	return result, err
}

func (structField *StructField) clone() *StructField {
	clone := &StructField{
		IsPrimaryKey:    structField.IsPrimaryKey,
		IsNormal:        structField.IsNormal,
		IsIgnored:       structField.IsIgnored,
		IsScanner:       structField.IsScanner,
		HasDefaultValue: structField.HasDefaultValue,
		IsForeignKey:    structField.IsForeignKey,

		DBName:       structField.DBName,
		Names:        structField.Names,
		TagSettings:  map[uint8]string{},
		Struct:       structField.Struct,
		Relationship: structField.Relationship,
	}

	for key, value := range structField.TagSettings {
		clone.TagSettings[key] = value
	}

	return clone
}

func (structField *StructField) cloneWithValue(value reflect.Value) *StructField {
	clone := &StructField{
		IsPrimaryKey:    structField.IsPrimaryKey,
		IsNormal:        structField.IsNormal,
		IsIgnored:       structField.IsIgnored,
		IsScanner:       structField.IsScanner,
		HasDefaultValue: structField.HasDefaultValue,
		IsForeignKey:    structField.IsForeignKey,

		DBName:       structField.DBName,
		Names:        structField.Names,
		TagSettings:  map[uint8]string{},
		Struct:       structField.Struct,
		Relationship: structField.Relationship,
	}
	for key, value := range structField.TagSettings {
		clone.TagSettings[key] = value
	}
	clone.Value = value
	//check if the value is blank
	clone.setIsBlank()
	return clone
}

//Function collects information from tags named `sql:""` and `gorm:""`
//TODO : @Badu - seems expensive to be called everytime
func (structField *StructField) parseTagSettings() error {
	structField.TagSettings = make(map[uint8]string)
	for _, str := range []string{structField.Struct.Tag.Get("sql"), structField.Struct.Tag.Get("gorm")} {
		tags := strings.Split(str, ";")
		for _, value := range tags {
			v := strings.Split(value, ":")
			k := strings.TrimSpace(strings.ToUpper(v[0]))
			uint8Key, ok := tagSettingMap[k]
			if ok {
				if len(v) >= 2 {
					structField.TagSettings[uint8Key] = strings.Join(v[1:], ":")
				} else {
					structField.TagSettings[uint8Key] = k
				}
			} else {
				fmt.Errorf("ERROR : COULDN'T FIND KEY FOR %q", k)
			}
		}
	}
	return nil
}

// Set set a value to the field
func (field *StructField) Set(value interface{}) (err error) {
	if !field.Value.IsValid() {
		return errors.New("field value not valid")
	}

	if !field.Value.CanAddr() {
		return ErrUnaddressable
	}

	reflectValue, ok := value.(reflect.Value)
	if !ok {
		reflectValue = reflect.ValueOf(value)
	}

	fieldValue := field.Value
	if reflectValue.IsValid() {
		if reflectValue.Type().ConvertibleTo(fieldValue.Type()) {
			fieldValue.Set(reflectValue.Convert(fieldValue.Type()))
		} else {
			//it's a pointer
			if fieldValue.Kind() == reflect.Ptr {
				//and it's NIL : we have to build it
				if fieldValue.IsNil() {
					fieldValue.Set(reflect.New(field.Struct.Type.Elem()))
				}
				//we dereference it
				fieldValue = fieldValue.Elem()
			}

			if reflectValue.Type().ConvertibleTo(fieldValue.Type()) {
				fieldValue.Set(reflectValue.Convert(fieldValue.Type()))
			} else if scanner, ok := fieldValue.Addr().Interface().(sql.Scanner); ok {
				err = scanner.Scan(reflectValue.Interface())
			} else {
				err = fmt.Errorf("could not convert argument of field %s from %s to %s", field.GetName(), reflectValue.Type(), fieldValue.Type())
			}
		}
	} else {
		field.Value.Set(reflect.Zero(field.Value.Type()))
	}
	//check if the value is blank
	field.setIsBlank()
	return err
}
//TODO : @Badu - seems expensive to be called everytime
func (structField *StructField) setIsBlank() {
	structField.IsBlank = reflect.DeepEqual(structField.Value.Interface(), reflect.Zero(structField.Value.Type()).Interface())
}

func (structField *StructField) GetName() string {
	return structField.Struct.Name
}
//TODO : implement it
func (structField *StructField) GetNames() []string {
	return nil
}

//TODO : @Badu - might need removal since seems unused
//seems unused
func (structField *StructField) GetTag() reflect.StructTag {
	return structField.Struct.Tag
}

//implementation of Stringer
//TODO : implement it
func (structField StructField) String() string {
	result := ""
	return fmt.Sprint(result)
}