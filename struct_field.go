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
		field.setFlag(ff_is_embed_or_anon)
	}

	// Even it is ignored, also possible to decode db value into the field
	if field.HasSetting(set_column) {
		field.DBName = field.GetStrSetting(set_column)
	} else {
		field.DBName = toDBName
	}
	//finished with it : cleanup
	field.UnsetTagSetting(set_column)

	//keeping the type for later usage
	field.Type = fromStruct.Type

	//dereference it, it's a pointer
	for field.Type.Kind() == reflect.Ptr {
		field.setFlag(ff_is_pointer)
		field.Type = field.Type.Elem()
	}

	if !field.IsIgnored() {
		fv := reflect.New(field.Type)
		//checking implements scanner or time
		_, isScanner := fv.Interface().(sql.Scanner)
		_, isTime := fv.Interface().(*time.Time)
		if isScanner {
			// is scanner
			field.setFlag(ff_is_normal)
			field.setFlag(ff_is_scanner)
		} else if isTime {
			// is time
			field.setFlag(ff_is_normal)
			field.setFlag(ff_is_time)
		}
	}

	//ATTN : order matters (can't use switch), since it can be both slice and struct
	if field.Type.Kind() == reflect.Slice {
		//mark it as slice
		field.setFlag(ff_is_slice)

		for field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Ptr {
			if field.Type.Kind() == reflect.Ptr {
				field.setFlag(ff_is_pointer)
			}
			//getting rid of slices and slices of pointers
			field.Type = field.Type.Elem()
		}
		//it's a slice of structs
		if field.Type.Kind() == reflect.Struct {
			//mark it as struct
			field.setFlag(ff_is_struct)
		}
	} else if field.Type.Kind() == reflect.Struct {
		//mark it as struct
		field.setFlag(ff_is_struct)
		if !field.IsIgnored() && field.IsScanner() {
			for i := 0; i < field.Type.NumField(); i++ {
				field.parseTagSettings(field.Type.Field(i).Tag)
			}
		}
	}

	if !field.IsIgnored() && !field.IsScanner() && !field.IsTime() && !field.IsEmbedOrAnon() {
		if field.IsSlice() || field.IsStruct() {
			field.setFlag(ff_relation_check) //marker for later processing of relationships
		} else {
			field.setFlag(ff_is_normal)
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
	return field.flags&(1<<ff_is_primarykey) != 0
}

func (field *StructField) IsNormal() bool {
	return field.flags&(1<<ff_is_normal) != 0
}

func (field *StructField) IsPointer() bool {
	return field.flags&(1<<ff_is_pointer) != 0
}

func (field *StructField) IsIgnored() bool {
	return field.flags&(1<<ff_is_ignored) != 0
}

func (field *StructField) IsScanner() bool {
	return field.flags&(1<<ff_is_scanner) != 0
}

func (field *StructField) IsTime() bool {
	return field.flags&(1<<ff_is_time) != 0
}

func (field *StructField) HasDefaultValue() bool {
	return field.flags&(1<<ff_has_default_value) != 0
}

func (field *StructField) IsForeignKey() bool {
	return field.flags&(1<<ff_is_foreignkey) != 0
}

func (field *StructField) IsBlank() bool {
	return field.flags&(1<<ff_is_blank) != 0
}

func (field *StructField) IsSlice() bool {
	return field.flags&(1<<ff_is_slice) != 0
}

func (field *StructField) IsStruct() bool {
	return field.flags&(1<<ff_is_struct) != 0
}

func (field *StructField) HasRelations() bool {
	return field.flags&(1<<ff_has_relations) != 0
}

func (field *StructField) WillCheckRelations() bool {
	return field.flags&(1<<ff_relation_check) != 0
}

func (field *StructField) IsEmbedOrAnon() bool {
	return field.flags&(1<<ff_is_embed_or_anon) != 0
}

func (field *StructField) IsAutoIncrement() bool {
	return field.flags&(1<<ff_is_autoincrement) != 0
}

func (field *StructField) UnsetIsAutoIncrement() {
	field.unsetFlag(ff_is_autoincrement)
}

// Set set a value to the field
func (field *StructField) SetIsAutoIncrement() {
	field.setFlag(ff_is_autoincrement)
}

func (field *StructField) SetIsPrimaryKey() {
	field.setFlag(ff_is_primarykey)
}

func (field *StructField) UnsetIsPrimaryKey() {
	field.unsetFlag(ff_is_primarykey)
}

func (field *StructField) SetIsNormal() {
	field.setFlag(ff_is_normal)
}

func (field *StructField) UnsetIsBlank() {
	field.unsetFlag(ff_is_blank)
}

func (field *StructField) UnsetCheckRelations() {
	field.unsetFlag(ff_relation_check)
}

func (field *StructField) SetIsBlank() {
	field.setFlag(ff_is_blank)
}

func (field *StructField) SetIsForeignKey() {
	field.setFlag(ff_is_foreignkey)
}

func (field *StructField) SetHasRelations() {
	field.setFlag(ff_has_relations)
}

func (field *StructField) LinkPoly(withField *StructField, tableName string) {
	field.setFlag(ff_is_foreignkey)
	withField.tagSettings.set(set_polymorphic_type, field.StructName)
	withField.tagSettings.set(set_polymorphic_dbname, field.DBName)
	// if Dog has multiple set of toys set name of the set (instead of default 'dogs')
	if !withField.HasSetting(set_polymorphic_value) {
		withField.tagSettings.set(set_polymorphic_value, tableName)
	}
}

func (field *StructField) HasNotNullSetting() bool {
	return field.tagSettings.has(set_not_null)
}

func (field *StructField) UnsetTagSetting(named uint8) {
	field.tagSettings.unset(named)
}

//gets a key (for code readability)
func (field *StructField) HasSetting(named uint8) bool {
	return field.tagSettings.has(named)
}

//TODO : make methods for each setting (readable code)
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

func (field *StructField) RelationIsMany2Many() bool {
	kind, ok := field.tagSettings.get(set_relation_kind)
	if !ok {
		return false
	}
	return kind == rel_many2many
}

func (field *StructField) RelationIsHasMany() bool {
	kind, ok := field.tagSettings.get(set_relation_kind)
	if !ok {
		return false
	}
	return kind == rel_has_many
}

func (field *StructField) RelationIsHasOne() bool {
	kind, ok := field.tagSettings.get(set_relation_kind)
	if !ok {
		return false
	}
	return kind == rel_has_one
}

func (field *StructField) RelationIsBelongsTo() bool {
	kind, ok := field.tagSettings.get(set_relation_kind)
	if !ok {
		return false
	}
	return kind == rel_belongs_to
}

func (field *StructField) RelKind() uint8 {
	kind, ok := field.tagSettings.get(set_relation_kind)
	if !ok {
		return 0
	}
	return kind.(uint8)
}

func (field *StructField) JoinHandler() JoinTableHandlerInterface {
	iHandler, ok := field.tagSettings.get(set_join_table_handler)
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

func (field *StructField) GetForeignFieldNames() StrSlice {
	value, ok := field.tagSettings.get(set_foreign_field_names)
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

func (field *StructField) GetAssociationForeignFieldNames() StrSlice {
	value, ok := field.tagSettings.get(set_association_foreign_field_names)
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

func (field *StructField) GetForeignDBNames() StrSlice {
	value, ok := field.tagSettings.get(set_foreign_db_names)
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

func (field *StructField) GetAssociationDBNames() StrSlice {
	value, ok := field.tagSettings.get(set_association_foreign_db_names)
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

func (field *StructField) GetAssocFKs() StrSlice {
	value, ok := field.tagSettings.get(set_associationforeignkey)
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

func (field *StructField) GetFKs() StrSlice {
	value, ok := field.tagSettings.get(set_foreignkey)
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

func (field *StructField) Set(value interface{}) error {
	var (
		err        error
		fieldValue = field.Value
	)

	if !fieldValue.IsValid() {
		//TODO : @Badu - make errors more explicit : which field...
		return fmt.Errorf(err_struct_field_not_valid)
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
				err = fmt.Errorf(err_cannot_convert, field.StructName, reflectValue.Type(), fieldValue.Type())
			}
		}
		//then we check if the value is blank
		field.checkIsBlank()
	} else {
		//set is blank
		field.setFlag(ff_is_blank)
		//it's not valid : set empty
		field.setZeroValue()
	}

	return err
}

func (field *StructField) setZeroValue() {
	field.Value.Set(SetZero(field.Value))
}

// ParseFieldStructForDialect parse field struct for dialect
func (field *StructField) ParseFieldStructForDialect() (reflect.Value, string, int, string) {
	var (
		size int
		//TODO : maybe it's best that this would be kept into settings and retrieved via field.GetStrSetting
		//so we can delete set_not_null, set_unique,  set_default and set_type from our map
		additionalType, sqlType string
	)

	if field.tagSettings.has(set_size) {
		// Default Size
		val, ok := field.tagSettings.get(set_size)
		if ok {
			size = val.(int)
		}
	} else {
		size = 255
	}

	if field.tagSettings.has(set_not_null) {
		additionalType = field.GetStrSetting(set_not_null)
	}

	if field.tagSettings.has(set_unique) {
		if additionalType != "" {
			additionalType += " "
		}
		additionalType += field.GetStrSetting(set_unique)
	}

	// Default type from tag setting
	if field.tagSettings.has(set_default) {
		if additionalType != "" {
			additionalType += " "
		}
		additionalType += "DEFAULT " + field.GetStrSetting(set_default)
	}

	if field.HasSetting(set_type) {
		sqlType = field.GetStrSetting(set_type)
	}

	fieldValue := reflect.Indirect(reflect.New(field.Type))

	if field.IsStruct() && field.IsScanner() {
		//implements scanner
		fieldValue = fieldValue.Field(0) //Attention : returns the ONLY first field

	} else if field.IsSlice() {
		fieldValue = field.Value
	}

	return fieldValue, sqlType, size, strings.TrimSpace(additionalType)
}

//Function collects information from tags named `sql:""` and `gorm:""`
func (field *StructField) parseTagSettings(tag reflect.StructTag) error {
	for _, str := range []string{tag.Get(str_tag_sql), tag.Get(str_tag_gorm)} {
		tags := strings.Split(str, ";")

		for _, value := range tags {
			v := strings.Split(value, ":")
			if len(v) > 0 {
				k := strings.TrimSpace(strings.ToUpper(v[0]))

				//set some flags directly
				switch k {
				case "":
				//avoid empty keys : original gorm didn't mind creating them
				case tag_ignored:
					//we don't store this in tagSettings, mark only flag
					field.setFlag(ff_is_ignored)
				case tag_primary_key:
					//we don't store this in tagSettings, mark only flag
					field.setFlag(ff_is_primarykey)
				case tag_auto_increment:
					//we don't store this in tagSettings, mark only flag
					field.setFlag(ff_is_autoincrement)
				case tag_embedded:
					//we don't store this in tagSettings, mark only flag
					field.setFlag(ff_is_embed_or_anon)
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
						case tag_default_str:
							field.setFlag(ff_has_default_value)
						case tag_many_to_many:
							field.tagSettings.set(set_relation_kind, rel_many2many)
						case tag_size:
							storedValue, _ = strconv.Atoi(v[1])
						case tag_association_foreign_key, tag_foreignkey:
							var strSlice StrSlice
							if len(v) != 2 {
								return fmt.Errorf(err_missing_field_names, k, str)
							}
							keyNames := strings.Split(v[1], ",")
							strSlice = append(strSlice, keyNames...)
							storedValue = strSlice
						}
						field.tagSettings.set(uint8Key, storedValue)
					} else {
						return fmt.Errorf(err_key_not_found, k, str)
					}
				}
			}

		}
		if field.IsAutoIncrement() && !field.IsPrimaryKey() {
			field.setFlag(ff_has_default_value)
		}
	}
	return nil
}

//TODO : @Badu - seems expensive to be called everytime. Maybe a good solution would be to
//change isBlank = true by default and modify the code to change it to false only when we have a value
//to make this less expensive
func (field *StructField) checkIsBlank() {
	if IsZero(field.Value) {
		field.setFlag(ff_is_blank)
	} else {
		field.unsetFlag(ff_is_blank)
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
	if field.flags&(1<<ff_is_primarykey) != 0 {
		collector.add(" PrimaryKey")
	}
	if field.flags&(1<<ff_is_normal) != 0 {
		collector.add(" IsNormal")
	}
	if field.flags&(1<<ff_is_ignored) != 0 {
		collector.add(" IsIgnored")
	}
	if field.flags&(1<<ff_is_scanner) != 0 {
		collector.add(" IsScanner")
	}
	if field.flags&(1<<ff_is_time) != 0 {
		collector.add(" IsTime")
	}
	if field.flags&(1<<ff_has_default_value) != 0 {
		collector.add(" HasDefaultValue")
	}
	if field.flags&(1<<ff_is_foreignkey) != 0 {
		collector.add(" IsForeignKey")
	}
	if field.flags&(1<<ff_is_blank) != 0 {
		collector.add(" IsBlank")
	}
	if field.flags&(1<<ff_is_slice) != 0 {
		collector.add(" IsSlice")
	}
	if field.flags&(1<<ff_is_struct) != 0 {
		collector.add(" IsStruct")
	}
	if field.flags&(1<<ff_has_relations) != 0 {
		collector.add(" HasRelations")
	}
	if field.flags&(1<<ff_is_embed_or_anon) != 0 {
		collector.add(" IsEmbedAnon")
	}
	if field.flags&(1<<ff_is_autoincrement) != 0 {
		collector.add(" IsAutoincrement")
	}
	if field.flags&(1<<ff_is_pointer) != 0 {
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
