package gorm

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

func NewStructField(fromStruct reflect.StructField, toDBName string) (*StructField, error) {
	field := &StructField{
		StructName:  fromStruct.Name,
		Names:       []string{fromStruct.Name},
		tagSettings: TagSettings{Uint8Map: make(map[uint8]interface{}), l: new(sync.RWMutex)},
	}
	//field should process itself the tag settings
	err := field.parseTagSettings(fromStruct.Tag)

	if fromStruct.Anonymous {
		field.setFlag(ffIsEmbedOrAnon)
	}

	// Even it is ignored, also possible to decode db value into the field
	if field.HasSetting(setColumn) {
		field.DBName = field.GetStrSetting(setColumn)
	} else {
		field.DBName = toDBName
	}
	//finished with it : cleanup
	field.UnsetTagSetting(setColumn)

	//keeping the type for later usage
	field.Type = fromStruct.Type

	//dereference it, it's a pointer
	for field.Type.Kind() == reflect.Ptr {
		field.setFlag(ffIsPointer)
		field.Type = field.Type.Elem()
	}

	if !field.IsIgnored() {
		fv := reflect.New(field.Type)
		//checking implements scanner or time
		_, isScanner := fv.Interface().(sql.Scanner)
		_, isTime := fv.Interface().(*time.Time)
		if isScanner {
			// is scanner
			field.setFlag(ffIsNormal)
			field.setFlag(ffIsScanner)
		} else if isTime {
			// is time
			field.setFlag(ffIsNormal)
			field.setFlag(ffIsTime)
		}
	}

	//ATTN : order matters (can't use switch), since it can be both slice and struct
	if field.Type.Kind() == reflect.Slice {
		//mark it as slice
		field.setFlag(ffIsSlice)

		for field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Ptr {
			if field.Type.Kind() == reflect.Ptr {
				field.setFlag(ffIsPointer)
			}
			//getting rid of slices and slices of pointers
			field.Type = field.Type.Elem()
		}
		//it's a slice of structs
		if field.Type.Kind() == reflect.Struct {
			//mark it as struct
			field.setFlag(ffIsStruct)
		}
	} else if field.Type.Kind() == reflect.Struct {
		//mark it as struct
		field.setFlag(ffIsStruct)
		if !field.IsIgnored() && field.IsScanner() {
			for i := 0; i < field.Type.NumField(); i++ {
				field.parseTagSettings(field.Type.Field(i).Tag)
			}
		}
	}

	if !field.IsIgnored() && !field.IsScanner() && !field.IsTime() && !field.IsEmbedOrAnon() {
		if field.IsSlice() || field.IsStruct() {
			field.setFlag(ffRelationCheck) //marker for later processing of relationships
		} else {
			field.setFlag(ffIsNormal)
		}
	}

	if !field.IsStruct() && field.IsSlice() {
		//create a slice value of it, to be returned
		_, field.Value = field.makeSlice()
	} else {
		//otherwise, create a value of it, to be returned
		field.Value = reflect.New(field.Type)
	}

	return field, err
}

func (f *StructField) ptrToLoad() reflect.Value {
	return reflect.New(reflect.PtrTo(f.Value.Type()))
}

func (f *StructField) makeSlice() (interface{}, reflect.Value) {
	basicType := f.Type
	if f.IsPointer() {
		basicType = reflect.PtrTo(f.Type)
	}
	sliceType := reflect.SliceOf(basicType)
	slice := reflect.New(sliceType)
	slice.Elem().Set(reflect.MakeSlice(sliceType, 0, 0))
	return slice.Interface(), IndirectValue(slice.Interface())
}

func (f *StructField) Interface() interface{} {
	return reflect.New(f.Type).Interface()
}

func (f *StructField) IsPrimaryKey() bool {
	return f.flags&(1<<ffIsPrimarykey) != 0
}

func (f *StructField) IsNormal() bool {
	return f.flags&(1<<ffIsNormal) != 0
}

func (f *StructField) IsPointer() bool {
	return f.flags&(1<<ffIsPointer) != 0
}

func (f *StructField) IsIgnored() bool {
	return f.flags&(1<<ffIsIgnored) != 0
}

func (f *StructField) IsScanner() bool {
	return f.flags&(1<<ffIsScanner) != 0
}

func (f *StructField) IsTime() bool {
	return f.flags&(1<<ffIsTime) != 0
}

func (f *StructField) HasDefaultValue() bool {
	return f.flags&(1<<ffHasDefaultValue) != 0
}

func (f *StructField) IsForeignKey() bool {
	return f.flags&(1<<ffIsForeignkey) != 0
}

func (f *StructField) IsBlank() bool {
	return f.flags&(1<<ffIsBlank) != 0
}

func (f *StructField) IsSlice() bool {
	return f.flags&(1<<ffIsSlice) != 0
}

func (f *StructField) IsStruct() bool {
	return f.flags&(1<<ffIsStruct) != 0
}

func (f *StructField) HasRelations() bool {
	return f.flags&(1<<ffHasRelations) != 0
}

func (f *StructField) WillCheckRelations() bool {
	return f.flags&(1<<ffRelationCheck) != 0
}

func (f *StructField) IsEmbedOrAnon() bool {
	return f.flags&(1<<ffIsEmbedOrAnon) != 0
}

func (f *StructField) IsAutoIncrement() bool {
	return f.flags&(1<<ffIsAutoincrement) != 0
}

func (f *StructField) UnsetIsAutoIncrement() {
	f.unsetFlag(ffIsAutoincrement)
}

// Set set a value to the field
func (f *StructField) SetIsAutoIncrement() {
	f.setFlag(ffIsAutoincrement)
}

func (f *StructField) SetIsPrimaryKey() {
	f.setFlag(ffIsPrimarykey)
}

func (f *StructField) UnsetIsPrimaryKey() {
	f.unsetFlag(ffIsPrimarykey)
}

func (f *StructField) SetIsNormal() {
	f.setFlag(ffIsNormal)
}

func (f *StructField) UnsetIsBlank() {
	f.unsetFlag(ffIsBlank)
}

func (f *StructField) UnsetCheckRelations() {
	f.unsetFlag(ffRelationCheck)
}

func (f *StructField) SetIsBlank() {
	f.setFlag(ffIsBlank)
}

func (f *StructField) SetIsForeignKey() {
	f.setFlag(ffIsForeignkey)
}

func (f *StructField) SetHasRelations() {
	f.setFlag(ffHasRelations)
}

func (f *StructField) LinkPoly(withField *StructField, tableName string) {
	f.setFlag(ffIsForeignkey)
	withField.tagSettings.set(setPolymorphicType, f.StructName)
	withField.tagSettings.set(setPolymorphicDbname, f.DBName)
	// if Dog has multiple set of toys set name of the set (instead of default 'dogs')
	if !withField.HasSetting(setPolymorphicValue) {
		withField.tagSettings.set(setPolymorphicValue, tableName)
	}
}

func (f *StructField) HasNotNullSetting() bool {
	return f.tagSettings.has(setNotNull)
}

func (f *StructField) UnsetTagSetting(named uint8) {
	f.tagSettings.unset(named)
}

//gets a key (for code readability)
func (f *StructField) HasSetting(named uint8) bool {
	return f.tagSettings.has(named)
}

//TODO : make methods for each setting (readable code)
func (f *StructField) GetStrSetting(named uint8) string {
	value, ok := f.tagSettings.get(named)
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

func (f *StructField) SetTagSetting(named uint8, value interface{}) {
	f.tagSettings.set(named, value)
}

func (f *StructField) RelationIsMany2Many() bool {
	kind, ok := f.tagSettings.get(setRelationKind)
	if !ok {
		return false
	}
	return kind == relMany2many
}

func (f *StructField) RelationIsHasMany() bool {
	kind, ok := f.tagSettings.get(setRelationKind)
	if !ok {
		return false
	}
	return kind == relHasMany
}

func (f *StructField) RelationIsHasOne() bool {
	kind, ok := f.tagSettings.get(setRelationKind)
	if !ok {
		return false
	}
	return kind == relHasOne
}

func (f *StructField) RelationIsBelongsTo() bool {
	kind, ok := f.tagSettings.get(setRelationKind)
	if !ok {
		return false
	}
	return kind == relBelongsTo
}

func (f *StructField) RelKind() uint8 {
	kind, ok := f.tagSettings.get(setRelationKind)
	if !ok {
		return 0
	}
	return kind.(uint8)
}

func (f *StructField) JoinHandler() JoinTableHandlerInterface {
	iHandler, ok := f.tagSettings.get(setJoinTableHandler)
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

func (f *StructField) GetForeignFieldNames() StrSlice {
	value, ok := f.tagSettings.get(setForeignFieldNames)
	if !ok {
		return nil
	}
	slice, ok := value.(StrSlice)
	if !ok {
		//can't convert to slice
		return nil
	}
	return slice
}

func (f *StructField) GetAssociationForeignFieldNames() StrSlice {
	value, ok := f.tagSettings.get(setAssociationForeignFieldNames)
	if !ok {
		return nil
	}
	slice, ok := value.(StrSlice)
	if !ok {
		//can't convert to slice
		return nil
	}
	return slice
}

func (f *StructField) GetForeignDBNames() StrSlice {
	value, ok := f.tagSettings.get(setForeignDbNames)
	if !ok {
		return nil
	}
	slice, ok := value.(StrSlice)
	if !ok {
		//can't convert to slice
		return nil
	}
	return slice
}

func (f *StructField) GetAssociationDBNames() StrSlice {
	value, ok := f.tagSettings.get(setAssociationForeignDbNames)
	if !ok {
		return nil
	}
	slice, ok := value.(StrSlice)
	if !ok {
		//can't convert to slice
		return nil
	}
	return slice
}

func (f *StructField) GetAssocFKs() StrSlice {
	value, ok := f.tagSettings.get(setAssociationforeignkey)
	if !ok {
		return nil
	}
	slice, ok := value.(StrSlice)
	if !ok {
		//can't convert to slice
		return nil
	}
	return slice
}

func (f *StructField) GetFKs() StrSlice {
	value, ok := f.tagSettings.get(setForeignkey)
	if !ok {
		return nil
	}
	slice, ok := value.(StrSlice)
	if !ok {
		//can't convert to slice
		return nil
	}
	return slice
}

func (f *StructField) Set(value interface{}) error {
	var (
		err        error
		fieldValue = f.Value
	)

	if !fieldValue.IsValid() {
		//TODO : @Badu - make errors more explicit : which field...
		return fmt.Errorf(errStructFieldNotValid)
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
					fieldValue.Set(reflect.New(f.Type))
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
				err = fmt.Errorf(errCannotConvert, f.StructName, reflectValue.Type(), fieldValue.Type())
			}
		}
		//then we check if the value is blank
		f.checkIsBlank()
	} else {
		//set is blank
		f.setFlag(ffIsBlank)
		//it's not valid : set empty
		f.setZeroValue()
	}

	return err
}

func (f *StructField) setZeroValue() {
	f.Value.Set(SetZero(f.Value))
}

// ParseFieldStructForDialect parse field struct for dialect
func (f *StructField) ParseFieldStructForDialect() (reflect.Value, string, int, string) {
	var (
		size int
		//TODO : maybe it's best that this would be kept into settings and retrieved via field.GetStrSetting
		//so we can delete set_not_null, set_unique,  set_default and set_type from our map
		additionalType, sqlType string
	)

	if f.tagSettings.has(setSize) {
		// Default Size
		val, ok := f.tagSettings.get(setSize)
		if ok {
			size = val.(int)
		}
	} else {
		size = 255
	}

	if f.tagSettings.has(setNotNull) {
		additionalType = f.GetStrSetting(setNotNull)
	}

	if f.tagSettings.has(setUnique) {
		if additionalType != "" {
			additionalType += " "
		}
		additionalType += f.GetStrSetting(setUnique)
	}

	// Default type from tag setting
	if f.tagSettings.has(setDefault) {
		if additionalType != "" {
			additionalType += " "
		}
		additionalType += "DEFAULT " + f.GetStrSetting(setDefault)
	}

	if f.HasSetting(setType) {
		sqlType = f.GetStrSetting(setType)
	}

	fieldValue := reflect.Indirect(reflect.New(f.Type))

	if f.IsStruct() && f.IsScanner() {
		//implements scanner
		fieldValue = fieldValue.Field(0) //Attention : returns the ONLY first field

	} else if f.IsSlice() {
		fieldValue = f.Value
	}

	return fieldValue, sqlType, size, strings.TrimSpace(additionalType)
}

//Function collects information from tags named `sql:""` and `gorm:""`
func (f *StructField) parseTagSettings(tag reflect.StructTag) error {
	for _, str := range []string{tag.Get(strTagSql), tag.Get(strTagGorm)} {
		tags := strings.Split(str, ";")

		for _, value := range tags {
			v := strings.Split(value, ":")
			if len(v) > 0 {
				k := strings.TrimSpace(strings.ToUpper(v[0]))

				//set some flags directly
				switch k {
				case "":
				//avoid empty keys : original gorm didn't mind creating them
				case tagTransient:
					//we don't store this in tagSettings, mark only flag
					f.setFlag(ffIsIgnored)
				case tagPrimaryKey:
					//we don't store this in tagSettings, mark only flag
					f.setFlag(ffIsPrimarykey)
				case tagAutoIncrement:
					//we don't store this in tagSettings, mark only flag
					f.setFlag(ffIsAutoincrement)
				case tagEmbedded:
					//we don't store this in tagSettings, mark only flag
					f.setFlag(ffIsEmbedOrAnon)
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
						case tagDefaultStr:
							f.setFlag(ffHasDefaultValue)
						case tagManyToMany:
							f.tagSettings.set(setRelationKind, relMany2many)
						case tagSize:
							storedValue, _ = strconv.Atoi(v[1])
						case tagAssociationForeignKey, tagForeignkey:
							var strSlice StrSlice
							if len(v) != 2 {
								return fmt.Errorf(errMissingFieldNames, k, str)
							}
							keyNames := strings.Split(v[1], ",")
							strSlice = append(strSlice, keyNames...)
							storedValue = strSlice
						}
						f.tagSettings.set(uint8Key, storedValue)
					} else {
						return fmt.Errorf(errKeyNotFound, k, str)
					}
				}
			}

		}
		if f.IsAutoIncrement() && !f.IsPrimaryKey() {
			f.setFlag(ffHasDefaultValue)
		}
	}
	return nil
}

//TODO : @Badu - seems expensive to be called everytime. Maybe a good solution would be to
//change isBlank = true by default and modify the code to change it to false only when we have a value
//to make this less expensive
func (f *StructField) checkIsBlank() {
	if IsZero(f.Value) {
		f.setFlag(ffIsBlank)
	} else {
		f.unsetFlag(ffIsBlank)
	}
}

func (f *StructField) setFlag(value uint8) {
	f.flags = f.flags | (1 << value)
}

func (f *StructField) unsetFlag(value uint8) {
	f.flags = f.flags & ^(1 << value)
}

func (f *StructField) clone() *StructField {
	clone := &StructField{
		flags:       f.flags,
		DBName:      f.DBName,
		Names:       f.Names,
		tagSettings: f.tagSettings.clone(),
		StructName:  f.StructName,
		Type:        f.Type,
	}

	return clone
}

func (f *StructField) cloneWithValue(value reflect.Value) *StructField {
	clone := &StructField{
		flags:       f.flags,
		DBName:      f.DBName,
		Names:       f.Names,
		tagSettings: f.tagSettings.clone(),
		StructName:  f.StructName,
		Value:       value,
		Type:        f.Type,
	}
	//check if the value is blank
	clone.checkIsBlank()
	return clone
}

//implementation of Stringer
func (f StructField) String() string {
	var collector Collector
	namesNo := len(f.Names)
	if namesNo == 1 {
		collector.add("%s %q [%d %s]\n", "Name:", f.DBName, namesNo, "name")
	} else {
		collector.add("%s %q [%d %s]\n", "Name:", f.DBName, namesNo, "names")
	}

	for _, n := range f.Names {
		collector.add("\t%s = %q\n", "names", n)
	}

	collector.add("Flags:")
	if f.flags&(1<<ffIsPrimarykey) != 0 {
		collector.add(" PrimaryKey")
	}
	if f.flags&(1<<ffIsNormal) != 0 {
		collector.add(" IsNormal")
	}
	if f.flags&(1<<ffIsIgnored) != 0 {
		collector.add(" IsIgnored")
	}
	if f.flags&(1<<ffIsScanner) != 0 {
		collector.add(" IsScanner")
	}
	if f.flags&(1<<ffIsTime) != 0 {
		collector.add(" IsTime")
	}
	if f.flags&(1<<ffHasDefaultValue) != 0 {
		collector.add(" HasDefaultValue")
	}
	if f.flags&(1<<ffIsForeignkey) != 0 {
		collector.add(" IsForeignKey")
	}
	if f.flags&(1<<ffIsBlank) != 0 {
		collector.add(" IsBlank")
	}
	if f.flags&(1<<ffIsSlice) != 0 {
		collector.add(" IsSlice")
	}
	if f.flags&(1<<ffIsStruct) != 0 {
		collector.add(" IsStruct")
	}
	if f.flags&(1<<ffHasRelations) != 0 {
		collector.add(" HasRelations")
	}
	if f.flags&(1<<ffIsEmbedOrAnon) != 0 {
		collector.add(" IsEmbedAnon")
	}
	if f.flags&(1<<ffIsAutoincrement) != 0 {
		collector.add(" IsAutoincrement")
	}
	if f.flags&(1<<ffIsPointer) != 0 {
		collector.add(" IsPointer")
	}
	collector.add("\n")

	if f.tagSettings.len() > 0 {
		collector.add("%s\n%s", "Tags:", f.tagSettings)
		if f.HasRelations() && f.RelKind() == 0 {
			collector.add("ERROR : Has relations but invalid relation kind\n")
		}
	}
	if f.Type != nil {
		collector.add("%s = %s\n", "Type:", f.Type.String())
	}
	collector.add("%s = %s\n", "Value:", f.Value.String())
	return collector.String()
}
