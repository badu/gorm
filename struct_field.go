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
	RELATION_CHECK    uint8 = 14
)

func NewStructField(fromStruct reflect.StructField) (*StructField, error) {
	result := &StructField{
		StructName:  fromStruct.Name,
		Names:       []string{fromStruct.Name},
		tagSettings: TagSettings{Uint8Map: make(map[uint8]interface{}), l: new(sync.RWMutex)},
	}
	//field should process itself the tag settings
	err := result.parseTagSettings(fromStruct.Tag)

	if fromStruct.Anonymous {
		result.setFlag(IS_EMBED_OR_ANON)
	}

	// Even it is ignored, also possible to decode db value into the field
	if result.HasSetting(COLUMN) {
		result.DBName = result.GetStrSetting(COLUMN)
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
			}
		}
	}
	if !result.IsIgnored() && !result.IsScanner() && !result.IsTime() && !result.IsEmbedOrAnon() {
		if result.IsSlice() {
			result.setFlag(RELATION_CHECK) //marker for later processing of relationships
		} else if result.IsStruct() {
			result.setFlag(RELATION_CHECK) //marker for later processing of relationships
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

func (field *StructField) WillCheckRelations() bool {
	return field.flags&(1<<RELATION_CHECK) != 0
}

func (field *StructField) IsEmbedOrAnon() bool {
	return field.flags&(1<<IS_EMBED_OR_ANON) != 0
}

func (field *StructField) IsAutoIncrement() bool {
	return field.flags&(1<<IS_AUTOINCREMENT) != 0
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

func (field *StructField) UnsetCheckRelations() {
	field.unsetFlag(RELATION_CHECK)
}

func (field *StructField) SetIsBlank() {
	field.setFlag(IS_BLANK)
}

func (field *StructField) SetIsForeignKey() {
	field.setFlag(IS_FOREIGNKEY)
}

func (field *StructField) SetHasRelations() {
	field.setFlag(HAS_RELATIONS)
}

func (field *StructField) LinkPoly(withField *StructField, tableName string) {
	field.setFlag(IS_FOREIGNKEY)
	withField.tagSettings.set(POLYMORPHIC_TYPE, field.StructName)
	withField.tagSettings.set(POLYMORPHIC_DBNAME, field.DBName)
	// if Dog has multiple set of toys set name of the set (instead of default 'dogs')
	if !withField.HasSetting(POLYMORPHIC_VALUE) {
		withField.tagSettings.set(POLYMORPHIC_VALUE, tableName)
	}
}

func (field *StructField) UnsetTagSetting(named uint8) {
	field.tagSettings.unset(named)
}

//gets a key (for code readability)
func (field *StructField) HasSetting(named uint8) bool {
	return field.tagSettings.has(named)
}

func (field *StructField) GetStrSetting(named uint8) string {
	value, ok := field.tagSettings.get(named)
	if !ok {
		//Doesn't exist
		return ""
	}
	strValue, ok := value.(string)
	if !ok {
		//Can't convert to string
		return ""
	}
	return strValue
}

func (field *StructField) SetTagSetting(named uint8, value interface{}) {
	field.tagSettings.set(named, value)
}

func (field *StructField) RelKind() uint8 {
	kind, ok := field.tagSettings.get(RELATION_KIND)
	if !ok {
		return 0
	}
	return kind.(uint8)
}

func (field *StructField) JoinHandler() JoinTableHandlerInterface {
	iHandler, ok := field.tagSettings.get(JOIN_TABLE_HANDLER)
	if !ok {
		//doesn't has one
		return nil
	}
	handler, ok := iHandler.(JoinTableHandlerInterface)
	if !ok {
		//can't convert
		return nil
	}
	return handler
}

func (field *StructField) GetSliceSetting(named uint8) StrSlice {
	value, ok := field.tagSettings.get(named)
	if !ok {
		//doesn't exist
		//reverseTagSettingsMap()
		//fmt.Printf("ERROR : SLICE %q NOT EXISTS!\n", cachedReverseTagSettingsMap[named])
		//_, file, line, _ := runtime.Caller(1)
		//fmt.Printf("Called from %s %d\n", file, line)
		return nil
	}
	slice, ok := value.(StrSlice)
	if !ok {
		//can't convert to slice
		return nil
	}
	return slice
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

		additionalType = ""
		sqlType        = ""
	)

	//TODO : @Badu - we have the scanner field info in StructField
	// Get scanner's real value
	fieldValue = getScannerValue(fieldValue)

	if field.tagSettings.has(SIZE) {
		// Default Size
		val, ok := field.tagSettings.get(SIZE)
		if ok {
			size = val.(int)
		}
	} else {
		size = 255
	}

	if field.tagSettings.has(NOT_NULL) {
		additionalType = field.GetStrSetting(NOT_NULL)
	}

	if field.tagSettings.has(UNIQUE) {
		if additionalType != "" {
			additionalType += " "
		}
		additionalType += field.GetStrSetting(UNIQUE)
	}

	// Default type from tag setting
	if field.tagSettings.has(DEFAULT) {
		if additionalType != "" {
			additionalType += " "
		}
		additionalType += "DEFAULT " + field.GetStrSetting(DEFAULT)
	}

	if field.HasSetting(TYPE) {
		sqlType = field.GetStrSetting(TYPE)
	}
	return fieldValue, sqlType, size, strings.TrimSpace(additionalType)
}

//Function collects information from tags named `sql:""` and `gorm:""`
func (field *StructField) parseTagSettings(tag reflect.StructTag) error {
	for _, str := range []string{tag.Get(TAG_SQL), tag.Get(TAG_GORM)} {
		tags := strings.Split(str, ";")

		for _, value := range tags {
			v := strings.Split(value, ":")
			if len(v) > 0 {
				k := strings.TrimSpace(strings.ToUpper(v[0]))

				//set some flags directly
				switch k {
				case "":
				//avoid empty keys : original gorm didn't mind creating them
				case ignored:
					//we don't store this in tagSettings, mark only flag
					field.setFlag(IS_IGNORED)
				case primary_key:
					//we don't store this in tagSettings, mark only flag
					field.setFlag(IS_PRIMARYKEY)
				case auto_increment:
					//we don't store this in tagSettings, mark only flag
					field.setFlag(IS_AUTOINCREMENT)
				case embedded:
					//we don't store this in tagSettings, mark only flag
					field.setFlag(IS_EMBED_OR_ANON)
				default:
					//other settings are kept in the map
					uint8Key, ok := tagSettingMap[k]
					if ok {
						var storedValue interface{}
						if len(v) >= 2 {
							storedValue = strings.Join(v[1:], ":")
						} else {
							storedValue = k
						}

						switch k {
						case default_str:
							field.setFlag(HAS_DEFAULT_VALUE)
						case many_to_many:
							field.tagSettings.set(RELATION_KIND, MANY_TO_MANY)
						case size:
							storedValue, _ = strconv.Atoi(v[1])
						case association_foreign_key, foreignkey:
							var strSlice StrSlice
							if len(v) >= 2 {
								strSlice = append(strSlice, v[1:]...)
							} else {
								strSlice = append(strSlice, k)
							}
							storedValue = strSlice
						}
						field.tagSettings.set(uint8Key, storedValue)
					} else {
						errMsg := fmt.Sprintf(key_not_found_err, k, str)
						fmt.Printf("\n\nERROR : %v\n\n", errMsg)
						return errors.New(errMsg)
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

func (field *StructField) setFlag(value uint8) {
	field.flags = field.flags | (1 << value)
}

func (field *StructField) unsetFlag(value uint8) {
	field.flags = field.flags & ^(1 << value)
}

func (field *StructField) clone() *StructField {
	clone := &StructField{
		flags:       field.flags,
		DBName:      field.DBName,
		Names:       field.Names,
		tagSettings: field.tagSettings.clone(),
		StructName:  field.StructName,
		Type:        field.Type,
	}

	return clone
}

func (field *StructField) cloneWithValue(value reflect.Value) *StructField {
	clone := &StructField{
		flags:       field.flags,
		DBName:      field.DBName,
		Names:       field.Names,
		tagSettings: field.tagSettings.clone(),
		StructName:  field.StructName,
		Value:       value,
		Type:        field.Type,
	}
	//check if the value is blank
	clone.checkIsBlank()
	return clone
}

//implementation of Stringer
func (field StructField) String() string {
	var collector Collector
	namesNo := len(field.Names)
	if namesNo == 1 {
		collector.add("%s %q [%d %s]\n", "Name:", field.DBName, namesNo, "name")
	} else {
		collector.add("%s %q [%d %s]\n", "Name:", field.DBName, namesNo, "names")
	}

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
	collector.add("\n")

	if field.tagSettings.len() > 0 {
		collector.add("%s\n%s", "Tags:", field.tagSettings)
		if field.HasRelations() && field.RelKind() == 0 {
			collector.add("ERROR : Has relations but invalid relation kind\n")
		}
	}
	if field.Type != nil {
		collector.add("%s = %s\n", "Type:", field.Type.String())
	}
	collector.add("%s = %s\n", "Value:", field.Value.String())
	return collector.String()
}
