package gorm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
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
	IS_AUTOINCREMENT  uint8 = 12
	IS_POINTER        uint8 = 13
	IS_OMITTED        uint8 = 14
	IS_INCLUDED       uint8 = 15
)

func NewStructField(fromStruct reflect.StructField) (*StructField, error) {
	result := &StructField{
		StructName:  fromStruct.Name,
		Names:       []string{fromStruct.Name},
		tagSettings: TagSettings{Uint8Map: make(map[uint8]string), l: new(sync.RWMutex)},
	}
	//field should process itself the tag settings
	err := result.parseTagSettings(fromStruct.Tag)

	if fromStruct.Anonymous {
		result.setFlag(IS_EMBED_OR_ANON)
	}

	// Even it is ignored, also possible to decode db value into the field
	if value := result.tagSettings.get(COLUMN); value != "" {
		result.DBName = value
	} else {
		result.DBName = NamesMap.ToDBName(fromStruct.Name)
	}

	//keeping the type for later usage
	result.Type = fromStruct.Type

	//dereference it, it's a pointer
	for result.Type.Kind() == reflect.Ptr {
		result.setFlag(IS_POINTER)
		result.Type = result.Type.Elem()
	}

	//create a value of it, to be returned
	result.Value = reflect.New(result.Type)

	if !result.IsIgnored() {
		//checking implements scanner or time
		_, isScanner := result.Value.Interface().(sql.Scanner)
		_, isTime := result.Value.Interface().(*time.Time)
		if isScanner {
			// is scanner
			result.setFlag(IS_NORMAL)
			result.setFlag(IS_SCANNER)
		} else if isTime {
			// is time
			result.setFlag(IS_NORMAL)
			result.setFlag(IS_TIME)
		}
	}

	//ATTN : order matters, since it can be both slice and struct
	if result.Type.Kind() == reflect.Slice {
		//mark it as slice
		result.setFlag(IS_SLICE)

		for result.Type.Kind() == reflect.Slice || result.Type.Kind() == reflect.Ptr {
			if result.Type.Kind() == reflect.Ptr {
				result.setFlag(IS_POINTER)
			}
			//getting rid of slices and slices of pointers
			result.Type = result.Type.Elem()
		}
		//it's a slice of structs
		if result.Type.Kind() == reflect.Struct {
			//mark it as struct
			result.setFlag(IS_STRUCT)
		}
	} else if result.Type.Kind() == reflect.Struct {
		//mark it as struct
		result.setFlag(IS_STRUCT)
		if !result.IsIgnored() && result.IsScanner() {
			for i := 0; i < result.Type.NumField(); i++ {
				result.parseTagSettings(result.Type.Field(i).Tag)
				/**
				tag := result.Type.Field(i).Tag
				for _, str := range []string{tag.Get(TAG_SQL), tag.Get(TAG_GORM)} {
					err := result.tagSettings.loadFromTags(result, str)
					if err != nil {
						return nil, err
					}
				}
				**/
			}
		}
	}

	if !result.IsIgnored() && !result.IsScanner() && !result.IsTime() && !result.IsEmbedOrAnon() {
		if result.IsSlice() {
			result.setFlag(HAS_RELATIONS) //marker for later processing of relationships
		} else if result.IsStruct() {
			result.setFlag(HAS_RELATIONS) //marker for later processing of relationships
		} else {
			result.setFlag(IS_NORMAL)
		}
	}

	return result, err
}

func (field *StructField) ptrToLoad() reflect.Value {
	return reflect.New(reflect.PtrTo(field.Value.Type()))
}

func (field *StructField) makeSlice() (interface{}, reflect.Value) {
	basicType := field.Type
	if field.IsPointer() {
		basicType = reflect.PtrTo(field.Type)
	}
	sliceType := reflect.SliceOf(basicType)
	slice := reflect.New(sliceType)
	slice.Elem().Set(reflect.MakeSlice(sliceType, 0, 0))
	return slice.Interface(), IndirectValue(slice.Interface())
}

func (field *StructField) Interface() interface{} {
	return reflect.New(field.Type).Interface()
}

func (field *StructField) IsPrimaryKey() bool {
	return field.flags&(1<<IS_PRIMARYKEY) != 0
}

func (field *StructField) IsNormal() bool {
	return field.flags&(1<<IS_NORMAL) != 0
}

func (field *StructField) IsPointer() bool {
	return field.flags&(1<<IS_POINTER) != 0
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

func (field *StructField) IsEmbedOrAnon() bool {
	return field.flags&(1<<IS_EMBED_OR_ANON) != 0
}

func (field *StructField) IsAutoIncrement() bool {
	return field.flags&(1<<IS_AUTOINCREMENT) != 0
}

//TODO : @Badu - make field aware of "it's include or not"
func (field *StructField) IsOmmited() bool {
	return field.flags&(1<<IS_OMITTED) != 0
}

//TODO : @Badu - make field aware of "it's include or not"
func (field *StructField) IsIncluded() bool {
	return field.flags&(1<<IS_INCLUDED) != 0
}

func (field *StructField) UnsetIsAutoIncrement() {
	field.unsetFlag(IS_AUTOINCREMENT)
}

// Set set a value to the field
func (field *StructField) SetIsAutoIncrement() {
	field.setFlag(IS_AUTOINCREMENT)
}

func (field *StructField) SetIsPrimaryKey() {
	field.setFlag(IS_PRIMARYKEY)
}

func (field *StructField) UnsetIsPrimaryKey() {
	field.unsetFlag(IS_PRIMARYKEY)
}

func (field *StructField) SetIsNormal() {
	field.setFlag(IS_NORMAL)
}

func (field *StructField) UnsetIsBlank() {
	field.unsetFlag(IS_BLANK)
}

func (field *StructField) SetIsBlank() {
	field.setFlag(IS_BLANK)
}

func (field *StructField) SetIsForeignKey() {
	field.setFlag(IS_FOREIGNKEY)
}

//gets a key (for code readability)
func (field *StructField) HasSetting(named uint8) bool {
	return field.tagSettings.has(named)
}

func (field *StructField) GetSetting(named uint8) string {
	return field.tagSettings.get(named)
}

func (field *StructField) Set(value interface{}) error {
	var (
		err        error
		fieldValue = field.Value
	)

	if !fieldValue.IsValid() {
		//TODO : @Badu - make errors more explicit : which field...
		return errors.New("StructField : field value not valid")
	}

	if !fieldValue.CanAddr() {
		return ErrUnaddressable
	}

	reflectValue, ok := value.(reflect.Value)
	if !ok {
		//couldn't cast - reflecting it
		reflectValue = reflect.ValueOf(value)
	}

	if reflectValue.IsValid() {
		if reflectValue.Type().ConvertibleTo(fieldValue.Type()) {
			//we set it
			fieldValue.Set(reflectValue.Convert(fieldValue.Type()))
		} else {
			//we're working with a pointer?
			if fieldValue.Kind() == reflect.Ptr {
				//it's a pointer
				if fieldValue.IsNil() {
					//and it's NIL : we have to build it
					fieldValue.Set(reflect.New(field.Type))
				}
				//we dereference it
				fieldValue = fieldValue.Elem()
			}

			scanner, isScanner := fieldValue.Addr().Interface().(sql.Scanner)
			if isScanner {
				//implements Scanner - we pass it over
				err = scanner.Scan(reflectValue.Interface())

			} else if reflectValue.Type().ConvertibleTo(fieldValue.Type()) {
				//last attempt to set it
				fieldValue.Set(reflectValue.Convert(fieldValue.Type()))
			} else {
				//Oops
				//TODO : @Badu - make errors more explicit
				err = fmt.Errorf("could not convert argument of field %s from %s to %s", field.StructName, reflectValue.Type(), fieldValue.Type())
			}
		}
		//then we check if the value is blank
		field.checkIsBlank()
	} else {
		//set is blank
		field.setFlag(IS_BLANK)
		//it's not valid
		field.Value.Set(reflect.Zero(field.Value.Type()))
	}

	return err
}

// ParseFieldStructForDialect parse field struct for dialect
func (field *StructField) ParseFieldStructForDialect() (reflect.Value, string, int, string) {
	var (
		size       = 0
		fieldValue = reflect.Indirect(reflect.New(field.Type))
	)

	// Get scanner's real value
	fieldValue = getScannerValue(fieldValue)

	if field.tagSettings.has(SIZE) {
		// Default Size
		size, _ = strconv.Atoi(field.tagSettings.get(SIZE))
	} else {
		size = 255
	}

	// Default type from tag setting
	additionalType := ""

	if field.tagSettings.has(NOT_NULL) {
		additionalType = field.tagSettings.get(NOT_NULL)
	}

	if field.tagSettings.has(UNIQUE) {
		if additionalType != "" {
			additionalType += " "
		}
		additionalType += field.tagSettings.get(UNIQUE)
	}

	if field.tagSettings.has(DEFAULT) {
		if additionalType != "" {
			additionalType += " "
		}
		additionalType += "DEFAULT " + field.tagSettings.get(DEFAULT)
	}

	return fieldValue, field.tagSettings.get(TYPE), size, strings.TrimSpace(additionalType)
}

func (field *StructField) clone() *StructField {
	clone := &StructField{
		flags:        field.flags,
		DBName:       field.DBName,
		Names:        field.Names,
		tagSettings:  field.tagSettings.clone(),
		StructName:   field.StructName,
		Relationship: field.Relationship,
		Type:         field.Type,
	}

	return clone
}

func (field *StructField) cloneWithValue(value reflect.Value) *StructField {
	clone := &StructField{
		flags:        field.flags,
		DBName:       field.DBName,
		Names:        field.Names,
		tagSettings:  field.tagSettings.clone(),
		StructName:   field.StructName,
		Relationship: field.Relationship,
		Value:        value,
		Type:         field.Type,
	}
	//check if the value is blank
	clone.checkIsBlank()
	return clone
}

//Function collects information from tags named `sql:""` and `gorm:""`
func (field *StructField) parseTagSettings(tag reflect.StructTag) error {
	for _, str := range []string{tag.Get(TAG_SQL), tag.Get(TAG_GORM)} {
		tags := strings.Split(str, ";")

		for _, value := range tags {
			v := strings.Split(value, ":")
			if len(v) > 0 {
				k := strings.TrimSpace(strings.ToUpper(v[0]))
				//avoid empty keys : original gorm didn't mind creating them
				if k != "" {
					//set some flags directly
					switch k {
					case ignored:
						field.setFlag(IS_IGNORED)
					case primary_key:
						field.setFlag(IS_PRIMARYKEY)
					case auto_increment:
						field.setFlag(IS_AUTOINCREMENT)
					case embedded:
						field.setFlag(IS_EMBED_OR_ANON)
					default:
						if k == default_str {
							field.setFlag(HAS_DEFAULT_VALUE)
						}
						//other settings are kept in the map
						uint8Key, ok := tagSettingMap[k]
						if ok {
							if len(v) >= 2 {
								field.tagSettings.set(uint8Key, strings.Join(v[1:], ":"))
							} else {
								field.tagSettings.set(uint8Key, k)
							}
						} else {
							return errors.New(fmt.Sprintf(key_not_found_err, k, str))
						}
					}

				}
			}

		}
		if field.IsAutoIncrement() && !field.IsPrimaryKey() {
			field.setFlag(HAS_DEFAULT_VALUE)
		}
	}
	return nil
}

//TODO : @Badu - seems expensive to be called everytime. Maybe a good solution would be to
//change isBlank = true by default and modify the code to change it to false only when we have a value
//to make this less expensive
func (field *StructField) checkIsBlank() {
	if reflect.DeepEqual(field.Value.Interface(), reflect.Zero(field.Value.Type()).Interface()) {
		field.setFlag(IS_BLANK)
	} else {
		field.unsetFlag(IS_BLANK)
	}
}

func (field *StructField) getForeignKeys() StrSlice {
	var result StrSlice
	if field.tagSettings.has(FOREIGNKEY) {
		result.commaLoad(field.tagSettings.get(FOREIGNKEY))
	}
	return result
}

func (field *StructField) getAssocForeignKeys() StrSlice {
	var result StrSlice
	if field.tagSettings.has(ASSOCIATIONFOREIGNKEY) {
		result.commaLoad(field.tagSettings.get(ASSOCIATIONFOREIGNKEY))
	}
	return result
}

func (field *StructField) setFlag(value uint8) {
	field.flags = field.flags | (1 << value)
}

func (field *StructField) unsetFlag(value uint8) {
	field.flags = field.flags & ^(1 << value)
}

//implementation of Stringer
func (field StructField) String() string {
	var collector Collector
	collector.add("%s = %q\n", "Name", field.DBName)
	for _, n := range field.Names {
		collector.add("\t%s = %q\n", "names", n)
	}

	collector.add("Flags:")
	if field.flags&(1<<IS_PRIMARYKEY) != 0 {
		collector.add(" PrimaryKey")
	}
	if field.flags&(1<<IS_NORMAL) != 0 {
		collector.add(" IsNormal")
	}
	if field.flags&(1<<IS_IGNORED) != 0 {
		collector.add(" IsIgnored")
	}
	if field.flags&(1<<IS_SCANNER) != 0 {
		collector.add(" IsScanner")
	}
	if field.flags&(1<<IS_TIME) != 0 {
		collector.add(" IsTime")
	}
	if field.flags&(1<<HAS_DEFAULT_VALUE) != 0 {
		collector.add(" HasDefaultValue")
	}
	if field.flags&(1<<IS_FOREIGNKEY) != 0 {
		collector.add(" IsForeignKey")
	}
	if field.flags&(1<<IS_BLANK) != 0 {
		collector.add(" IsBlank")
	}
	if field.flags&(1<<IS_SLICE) != 0 {
		collector.add(" IsSlice")
	}
	if field.flags&(1<<IS_STRUCT) != 0 {
		collector.add(" IsStruct")
	}
	if field.flags&(1<<HAS_RELATIONS) != 0 {
		collector.add(" HasRelations")
	}
	if field.flags&(1<<IS_EMBED_OR_ANON) != 0 {
		collector.add(" IsEmbedAnon")
	}
	if field.flags&(1<<IS_AUTOINCREMENT) != 0 {
		collector.add(" IsAutoincrement")
	}
	if field.flags&(1<<IS_POINTER) != 0 {
		collector.add(" IsPointer")
	}
	if field.flags&(1<<IS_OMITTED) != 0 {
		collector.add(" IsOmmited")
	}
	if field.flags&(1<<IS_INCLUDED) != 0 {
		collector.add(" HasIncluded")
	}
	collector.add("\n")

	if field.tagSettings.len() > 0 {
		collector.add("%s = %q\n", "Tags:", field.tagSettings)
	}
	if field.Type != nil {
		collector.add("%s = %s\n", "Type:", field.Type.String())
	}
	collector.add("%s = %s\n", "Value:", field.Value.String())
	if field.Relationship != nil {
		collector.add("%s\n%s", "Relationship:", field.Relationship)
	}
	return collector.String()
}
