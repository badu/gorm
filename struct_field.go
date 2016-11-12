package gorm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	//bit flags - flags are uint16, which means we can use 16 flags
	IS_PRIMARYKEY     uint8 = 0
	IS_NORMAL         uint8 = 1
	IS_IGNORED        uint8 = 2
	IS_SCANNER        uint8 = 3
	IS_TIME           uint8 = 4
	HAS_DEFAULT_VALUE uint8 = 5
	IS_FOREIGNKEY     uint8 = 6
	IS_BLANK          uint8 = 7
	IS_SLICE          uint8 = 8
	IS_STRUCT         uint8 = 9
	HAS_RELATIONS     uint8 = 10
	IS_EMBED_OR_ANON  uint8 = 11
)

func NewStructField(fieldStruct reflect.StructField) (*StructField, error) {
	result := &StructField{
		Struct: fieldStruct,
		Names:  []string{fieldStruct.Name},
	}
	//field should process itself the tag settings
	err := result.parseTagSettings()

	if result.tagSettings.has(IGNORED) {
		//result.IsIgnored = true
		result.setFlag(IS_IGNORED)
	}

	if result.tagSettings.has(PRIMARY_KEY) {
		//result.IsPrimaryKey = true
		result.setFlag(IS_PRIMARYKEY)
	}

	if result.tagSettings.has(DEFAULT) {
		//result.HasDefaultValue = true
		result.setFlag(HAS_DEFAULT_VALUE)
	}

	if result.tagSettings.has(AUTO_INCREMENT) && !result.IsPrimaryKey() {
		//result.HasDefaultValue = true
		result.setFlag(HAS_DEFAULT_VALUE)
	}

	if result.HasSetting(EMBEDDED) || fieldStruct.Anonymous {
		//result.isEmbedOrAnon = true
		result.setFlag(IS_EMBED_OR_ANON)
	}

	// Even it is ignored, also possible to decode db value into the field
	if value := result.tagSettings.get(COLUMN); value != "" {
		result.DBName = value
	} else {
		result.DBName = NamesMap.ToDBName(fieldStruct.Name)
	}
	//keeping the underlying type for later usage
	result.UnderlyingType = fieldStruct.Type

	for result.UnderlyingType.Kind() == reflect.Ptr {
		//dereference it, it's a pointer
		result.UnderlyingType = result.UnderlyingType.Elem()
	}

	if result.UnderlyingType.Kind() == reflect.Slice {
		//mark it as slice
		//result.IsSlice = true
		result.setFlag(IS_SLICE)
		//it's a slice of structs
		if result.getTrueType().Kind() == reflect.Struct {
			//mark it as struct
			//result.IsStruct = true
			result.setFlag(IS_STRUCT)
		}
	} else if result.UnderlyingType.Kind() == reflect.Struct {
		//mark it as struct
		//result.IsStruct = true
		result.setFlag(IS_STRUCT)
	}

	return result, err
}

func (field *StructField) IsPrimaryKey() bool {
	return field.flags&(1<<IS_PRIMARYKEY) != 0
}

func (field *StructField) IsNormal() bool {
	return field.flags&(1<<IS_NORMAL) != 0
}

func (field *StructField) SetIsNormal() {
	field.flags = field.flags | (1 << IS_NORMAL)
}

func (field *StructField) IsIgnored() bool {
	return field.flags&(1<<IS_IGNORED) != 0
}

func (field *StructField) IsScanner() bool {
	return field.flags&(1<<IS_SCANNER) != 0
}

func (field *StructField) IsTime() bool {
	return field.flags&(1<<IS_TIME) != 0
}

func (field *StructField) HasDefaultValue() bool {
	return field.flags&(1<<HAS_DEFAULT_VALUE) != 0
}

func (field *StructField) IsForeignKey() bool {
	return field.flags&(1<<IS_FOREIGNKEY) != 0
}

func (field *StructField) SetIsForeignKey() {
	field.flags = field.flags | (1 << IS_FOREIGNKEY)
}

func (field *StructField) IsBlank() bool {
	return field.flags&(1<<IS_BLANK) != 0
}

func (field *StructField) IsSlice() bool {
	return field.flags&(1<<IS_SLICE) != 0
}

func (field *StructField) IsStruct() bool {
	return field.flags&(1<<IS_STRUCT) != 0
}

func (field *StructField) HasRelations() bool {
	return field.flags&(1<<HAS_RELATIONS) != 0
}

func (field *StructField) SetHasRelations() {
	field.flags = field.flags | (1 << HAS_RELATIONS)
}

func (field *StructField) IsEmbedOrAnon() bool {
	return field.flags&(1<<IS_EMBED_OR_ANON) != 0
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

func (field *StructField) GetName() string {
	return field.Struct.Name
}

func (field *StructField) GetTagSetting() TagSettings {
	return field.tagSettings
}

//checks if has such a key (for code readability)
func (field *StructField) HasSetting(named uint8) bool {
	return field.tagSettings.has(named)
}

//gets a key (for code readability)
func (field *StructField) GetSetting(named uint8) string {
	return field.tagSettings.get(named)
}

//sets a key (for code readability)
func (field *StructField) SetSetting(named uint8, value string) {
	field.tagSettings.set(named, value)
}

//deletes a key (for code readability)
func (field *StructField) UnsetSetting(named uint8) {
	field.tagSettings.unset(named)
}

// ParseFieldStructForDialect parse field struct for dialect
func (field *StructField) ParseFieldStructForDialect() (
	fieldValue reflect.Value,
	sqlType string, size int,
	additionalType string) {

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
func (field StructField) String() string {
	result := fmt.Sprintf("%q:%q", "FieldName", field.Struct.Name)
	if field.Struct.PkgPath != "" {
		result += fmt.Sprintf(",%q:%q", "PkgPath", field.Struct.PkgPath)
	}
	if field.tagSettings.len() > 0 {
		result += fmt.Sprintf(",%q:%s", "Tags", field.tagSettings)
	}
	return fmt.Sprint(result)
}

////////////////////////////////////////////////////////////////////////////////
// Private methods
////////////////////////////////////////////////////////////////////////////////
func (field StructField) hasFlag(value uint8) bool {
	return field.flags&(1<<value) != 0
}

func (field *StructField) setFlag(value uint8) {
	field.flags = field.flags | (1 << value)
}

func (field *StructField) unsetFlag(value uint8) {
	field.flags = field.flags & ^(1 << value)
}

func (field *StructField) getTrueType() reflect.Type {
	trueType := field.UnderlyingType
	for trueType.Kind() == reflect.Slice || trueType.Kind() == reflect.Ptr {
		//dereference it
		trueType = trueType.Elem()
	}
	return trueType
}

func (field *StructField) checkInterfaces() interface{} {
	newValue := reflect.New(field.UnderlyingType)
	fieldValue := newValue.Interface()
	_, isScanner := fieldValue.(sql.Scanner)
	_, isTime := fieldValue.(*time.Time)
	if isScanner {
		// is scanner
		field.setFlag(IS_NORMAL)
		field.setFlag(IS_SCANNER)
		if field.UnderlyingType.Kind() == reflect.Struct {
			for i := 0; i < field.UnderlyingType.NumField(); i++ {
				tag := field.UnderlyingType.Field(i).Tag
				for _, str := range []string{tag.Get(TAG_SQL), tag.Get(TAG_GORM)} {
					err := field.tagSettings.loadFromTags(str)
					if err != nil {
						fmt.Printf("ERROR processing Scanner tags : %v\n", err)
					}
				}
			}
		}

	} else if isTime {
		// is time
		field.setFlag(IS_NORMAL)
		field.setFlag(IS_TIME)
	}
	return fieldValue
}

func (field *StructField) clone() *StructField {
	clone := &StructField{
		flags:        field.flags,
		DBName:       field.DBName,
		Names:        field.Names,
		tagSettings:  field.tagSettings.clone(),
		Struct:       field.Struct,
		Relationship: field.Relationship,
	}

	return clone
}

func (field *StructField) cloneWithValue(value reflect.Value) *StructField {
	clone := &StructField{
		flags:        field.flags,
		DBName:       field.DBName,
		Names:        field.Names,
		tagSettings:  field.tagSettings.clone(),
		Struct:       field.Struct,
		Relationship: field.Relationship,
	}

	clone.Value = value
	//check if the value is blank
	clone.setIsBlank()
	return clone
}

//Function collects information from tags named `sql:""` and `gorm:""`
func (field *StructField) parseTagSettings() error {
	field.tagSettings = TagSettings{Uint8Map: make(map[uint8]string)}
	tag := field.Struct.Tag
	for _, str := range []string{tag.Get(TAG_SQL), tag.Get(TAG_GORM)} {
		err := field.tagSettings.loadFromTags(str)
		if err != nil {
			return err
		}
	}
	return nil
}

//TODO : @Badu - seems expensive to be called everytime. Maybe a good solution would be to
//change isBlank = true by default and modify the code to change it to false only when we have a value
//to make this less expensive
func (field *StructField) setIsBlank() {
	if reflect.DeepEqual(field.Value.Interface(), reflect.Zero(field.Value.Type()).Interface()) {
		field.setFlag(IS_BLANK)
	} else {
		field.unsetFlag(IS_BLANK)
	}
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

func (field *StructField) getForeignKeys() StrSlice {
	var result StrSlice
	if foreignKey := field.GetSetting(FOREIGNKEY); foreignKey != "" {
		result.commaLoad(foreignKey)
	}
	return result
}

func (field *StructField) getAssocForeignKeys() StrSlice {
	var result StrSlice
	if foreignKey := field.GetSetting(ASSOCIATIONFOREIGNKEY); foreignKey != "" {
		result.commaLoad(foreignKey)
	}
	return result
}
