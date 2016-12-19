package gorm

import (
	"errors"
	"fmt"
	"github.com/jinzhu/inflection"
	"go/ast"
	"reflect"
	"sync"
)

// TableName get model's table name
func (modelStruct *ModelStruct) TableName(db *DBCon) string {
	return DefaultTableNameHandler(db, modelStruct.defaultTableName)
}

func (modelStruct *ModelStruct) StructFields() StructFields {
	return modelStruct.fieldsMap.fields
}

func (modelStruct *ModelStruct) Interface() interface{} {
	return reflect.New(modelStruct.ModelType).Interface()
}

func (modelStruct *ModelStruct) HasColumn(column string) bool {
	//looking for it
	field, ok := modelStruct.fieldsMap.get(column)
	if ok {
		//TODO : @Badu only if it's normal it's declared ?
		if field.IsNormal() {
			return true
		}
	}
	return false
}

func (modelStruct *ModelStruct) FieldByName(column string, con *DBCon) (*StructField, bool) {
	field, ok := modelStruct.fieldsMap.get(column)
	if !ok {
		//couldn't find it in "fields" map
		for _, field := range modelStruct.fieldsMap.fields {
			if field.DBName == con.namesMap.toDBName(column) {
				return field, true
			}
		}
		return nil, false
	}
	return field, ok
}

func (modelStruct *ModelStruct) Create(reflectType reflect.Type, scope *Scope) {
	modelStruct.ModelType = reflectType
	modelStruct.fieldsMap = fieldsMap{
		aliases: make(map[string]*StructField),
		l:       new(sync.RWMutex),
	}

	//implements tabler?
	tabler, ok := reflect.New(reflectType).Interface().(tabler)
	// Set default table name
	if ok {
		modelStruct.defaultTableName = tabler.TableName()
	} else {
		tableName := scope.con.parent.namesMap.toDBName(reflectType.Name())
		if scope.con == nil || !scope.con.parent.singularTable {
			tableName = inflection.Plural(tableName)
		}
		modelStruct.defaultTableName = tableName
	}
	// Get all fields
	for i := 0; i < reflectType.NumField(); i++ {
		if fieldStruct := reflectType.Field(i); ast.IsExported(fieldStruct.Name) {
			field, err := NewStructField(fieldStruct, scope.con.parent.namesMap.toDBName(fieldStruct.Name))
			if err != nil {
				scope.Err(errors.New(fmt.Sprintf(err_processing_tags, modelStruct.ModelType.Name(), err)))
				return
			}

			if !field.IsIgnored() {
				//field Value is created with the new struct field
				fieldValue := field.Value.Interface()
				if !field.IsScanner() && !field.IsTime() && field.IsEmbedOrAnon() {
					// is embedded struct
					for _, subField := range newScope(scope.con, fieldValue).GetModelStruct().StructFields() {
						subField = subField.clone()
						subField.Names = append([]string{fieldStruct.Name}, subField.Names...)

						if field.HasSetting(set_embedded_prefix) {
							subField.DBName = field.GetStrSetting(set_embedded_prefix) + subField.DBName
						}

						err = modelStruct.fieldsMap.add(subField)
						if err != nil {
							scope.Err(errors.New(fmt.Sprintf(err_adding_field, modelStruct.ModelType.Name(), err)))
							return
						}
					}
					continue
				}
			}

			err = modelStruct.fieldsMap.add(field)
			if err != nil {
				scope.Err(errors.New(fmt.Sprintf(err_adding_field, modelStruct.ModelType.Name(), err)))
				return
			}
		}
	}

	if modelStruct.noOfPKs() == 0 {
		//by default we're expecting that the modelstruct has a field named id
		if field, ok := modelStruct.fieldsMap.get(field_default_id_name); ok {
			field.SetIsPrimaryKey()
		}
		//else - it's not an error : joins don't have primary key named id
	}
}

func (modelStruct *ModelStruct) PKs() StructFields {
	if modelStruct.cachedPrimaryFields == nil {
		modelStruct.cachedPrimaryFields = modelStruct.fieldsMap.primaryFields()
	}
	return modelStruct.cachedPrimaryFields
}

func (modelStruct *ModelStruct) noOfPKs() int {
	if modelStruct.cachedPrimaryFields == nil {
		modelStruct.cachedPrimaryFields = modelStruct.fieldsMap.primaryFields()
	}
	return modelStruct.cachedPrimaryFields.len()
}

func (modelStruct *ModelStruct) processRelations(scope *Scope) {
	for _, field := range modelStruct.StructFields() {
		if field.WillCheckRelations() {
			toScope := newScope(scope.con, field.Interface())
			toModelStruct := toScope.GetModelStruct()
			//ATTN : order matters, since it can be both slice and struct
			if field.IsSlice() {
				if field.IsStruct() {
					//it's a slice of structs
					if field.HasSetting(set_many2many_name) {
						//many to many
						makeManyToMany(field, modelStruct, scope, toScope)
					} else {
						//has many
						makeHasMany(field, modelStruct, toModelStruct, scope, toScope)
					}
				} else {
					//it's a slice of primitive
					field.SetIsNormal()

				}
			} else if field.IsStruct() {
				//it's a struct - attempt to check if has one
				if !makeHasOne(field, modelStruct, toModelStruct, scope, toScope) {
					//attempt to check if belongs to
					if !makeBelongTo(field, modelStruct, toModelStruct, scope, toScope) {
						//Oops, neither
						errMsg := fmt.Errorf(err_no_belong_or_hasone, modelStruct.ModelType.Name(), field.DBName, field.StructName)
						scope.Warn(errMsg)
					}
				}
			}
			field.UnsetCheckRelations()
		}
	}
}

//implementation of Stringer
func (modelStruct ModelStruct) String() string {
	var collector Collector
	collector.add("%s = %s\n", "Table name", modelStruct.defaultTableName)
	if modelStruct.ModelType == nil {
		collector.add("%s = %s\n", "UNTYPED!!!")
	} else {
		collector.add("%s = %s\n", "Type", modelStruct.ModelType.String())
	}
	collector.add("Fields:\n\n")
	for fn, f := range modelStruct.fieldsMap.fields {
		collector.add("#%d\n%s\n", fn+1, f)
	}
	return collector.String()
}
