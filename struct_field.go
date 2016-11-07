package gorm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"strconv"
)

func NewStructField(fieldStruct reflect.StructField) (*StructField, error) {
	result := &StructField{
		Struct: fieldStruct,
		Names:  []string{fieldStruct.Name},
	}
	//field should process itself the tag settings
	err := result.parseTagSettings()

	if result.tagSettings.has(IGNORED) {
		result.IsIgnored = true
	}

	if result.tagSettings.has(PRIMARY_KEY) {
		result.IsPrimaryKey = true
	}

	if result.tagSettings.has(DEFAULT) {
		result.HasDefaultValue = true
	}

	if result.tagSettings.has(AUTO_INCREMENT) && !result.IsPrimaryKey {
		result.HasDefaultValue = true
	}

	// Even it is ignored, also possible to decode db value into the field
	if value := result.tagSettings.get(COLUMN); value != "" {
		result.DBName = value
	} else {
		result.DBName = NamesMap.ToDBName(fieldStruct.Name)
	}

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
		tagSettings:  structField.tagSettings.clone(),
		Struct:       structField.Struct,
		Relationship: structField.Relationship,
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
		tagSettings:  structField.tagSettings.clone(),
		Struct:       structField.Struct,
		Relationship: structField.Relationship,
	}

	clone.Value = value
	//check if the value is blank
	clone.setIsBlank()
	return clone
}

//Function collects information from tags named `sql:""` and `gorm:""`
//TODO : @Badu - seems expensive to be called everytime
func (structField *StructField) parseTagSettings() error {
	structField.tagSettings = newTagSettings()
	for _, str := range []string{structField.Struct.Tag.Get("sql"), structField.Struct.Tag.Get("gorm")} {
		err := structField.tagSettings.loadFromTags(str)
		if err != nil {
			return err
		}
	}
	return nil
}

// Set set a value to the field
func (field *StructField) Set(value interface{}) error {
	var err error
	if !field.Value.IsValid() {
		//TODO : @Badu - make errors more explicit : which field...
		return errors.New("field value not valid")
	}

	if !field.Value.CanAddr() {
		return ErrUnaddressable
	}
	//type cast to value
	reflectValue, ok := value.(reflect.Value)
	if !ok {
		//couldn't cast - reflecting it
		reflectValue = reflect.ValueOf(value)
	}

	fieldValue := field.Value
	if reflectValue.IsValid() {
		if reflectValue.Type().ConvertibleTo(fieldValue.Type()) {
			//we set it
			fieldValue.Set(reflectValue.Convert(fieldValue.Type()))
		} else {
			if fieldValue.Kind() == reflect.Ptr {
				//it's a pointer
				if fieldValue.IsNil() {
					//and it's NIL : we have to build it
					fieldValue.Set(reflect.New(field.Struct.Type.Elem()))
				}
				//we dereference it
				fieldValue = fieldValue.Elem()
			}
			//#fix (chore) : if implements scanner don't attempt to convert, just pass it over
			if scanner, ok := fieldValue.Addr().Interface().(sql.Scanner); ok {
				//implements Scanner - we pass it over
				err = scanner.Scan(reflectValue.Interface())
			} else if reflectValue.Type().ConvertibleTo(fieldValue.Type()) {
				fieldValue.Set(reflectValue.Convert(fieldValue.Type()))
			} else {
				//Oops
				//TODO : @Badu - make errors more explicit
				err = fmt.Errorf("could not convert argument of field %s from %s to %s", field.GetName(), reflectValue.Type(), fieldValue.Type())
			}
		}
	} else {
		//it's not valid
		field.Value.Set(reflect.Zero(field.Value.Type()))
	}
	//TODO : @Badu - seems invalid logic : above we set it ot zero if it's not valid
	//then we check if the value is blank
	//check if the value is blank
	field.setIsBlank()
	return err
}

//TODO : @Badu - seems expensive to be called everytime. Maybe a good solution would be to
//change isBlank = true by default and modify the code to change it to false only when we have a value
//to make this less expensive
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

//checks if has such a key (for code readability)
func (structField *StructField) HasSetting(named uint8) bool {
	return structField.tagSettings.has(named)
}

//gets a key (for code readability)
func (structField *StructField) GetSetting(named uint8) string {
	return structField.tagSettings.get(named)
}

//sets a key (for code readability)
func (structField *StructField) SetSetting(named uint8, value string) {
	structField.tagSettings.set(named, value)
}

//deletes a key (for code readability)
func (structField *StructField) UnsetSetting(named uint8) {
	structField.tagSettings.unset(named)
}

func (field *StructField) makeSlice() interface{} {
	elemType := field.Struct.Type
	if elemType.Kind() == reflect.Slice {
		elemType = elemType.Elem()
	}
	sliceType := reflect.SliceOf(elemType)
	slice := reflect.New(sliceType)
	slice.Elem().Set(reflect.MakeSlice(sliceType, 0, 0))
	return slice.Interface()
}

// ParseFieldStructForDialect parse field struct for dialect
func (field *StructField) ParseFieldStructForDialect()(fieldValue reflect.Value, sqlType string, size int, additionalType string) {
	// Get redirected field type
	var reflectType = field.Struct.Type
	for reflectType.Kind() == reflect.Ptr {
		reflectType = reflectType.Elem()
	}

	// Get redirected field value
	fieldValue = reflect.Indirect(reflect.New(reflectType))

	// Get scanner's real value
	var getScannerValue func(reflect.Value)
	getScannerValue = func(value reflect.Value) {
		fieldValue = value
		if _, isScanner := reflect.New(fieldValue.Type()).Interface().(sql.Scanner); isScanner && fieldValue.Kind() == reflect.Struct {
			getScannerValue(fieldValue.Field(0))
		}
	}
	getScannerValue(fieldValue)

	// Default Size
	if num := field.GetSetting(SIZE); num != "" {
		size, _ = strconv.Atoi(num)
	} else {
		size = 255
	}

	//TODO : @Badu - what if the settings below are empty?
	// Default type from tag setting
	additionalType = field.GetSetting(NOT_NULL) + " " + field.GetSetting(UNIQUE)
	if value := field.GetSetting(DEFAULT); value != "" {
		additionalType = additionalType + " DEFAULT " + value
	}

	return fieldValue, field.GetSetting(TYPE), size, strings.TrimSpace(additionalType)
}

/**
reflect.StructField{
	// Name is the field name.
	Name string
	// PkgPath is the package path that qualifies a lower case (unexported)
	// field name. It is empty for upper case (exported) field names.
	// See https://golang.org/ref/spec#Uniqueness_of_identifiers
	PkgPath string

	Type      Type      // field type
	Tag       StructTag // field tag string
	Offset    uintptr   // offset within struct, in bytes
	Index     []int     // index sequence for Type.FieldByIndex
	Anonymous bool      // is an embedded field
}
*/

//implementation of Stringer
//TODO : fully implement it
func (structField StructField) String() string {
	result := fmt.Sprintf("%q:%q", "FieldName", structField.Struct.Name)
	if structField.Struct.PkgPath != "" {
		result += fmt.Sprintf(",%q:%q", "PkgPath", structField.Struct.PkgPath)
	}
	if structField.tagSettings.len() > 0 {
		result += fmt.Sprintf(",%q:%s", "Tags", structField.tagSettings)
	}
	return fmt.Sprint(result)
}
