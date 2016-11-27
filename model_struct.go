package gorm

import (
	"errors"
	"fmt"
	"github.com/jinzhu/inflection"
	"go/ast"
	"reflect"
	"sync"
)

const (
	proc_tag_err            string = "ModelStruct %q processing tags error : %v"
	add_field_err           string = "ModelStruct %q add field error : %v"
	no_belong_or_hasone_err string = "%q (%q [%q]) is HAS ONE / BELONG TO missing"
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
	field, ok := modelStruct.fieldsMap.Get(column)
	if ok {
		//TODO : @Badu only if it's normal it's declared ?
		if field.hasFlag(IS_NORMAL) {
			return true
		}
	}
	return false
}

func (modelStruct *ModelStruct) FieldByName(column string) (*StructField, bool) {
	field, ok := modelStruct.fieldsMap.Get(column)
	if !ok {
		//fmt.Printf("couldn't find %q in fields map\n", column)
		//couldn't find it in "fields" map
		for _, field := range modelStruct.fieldsMap.fields {
			if field.DBName == NamesMap.ToDBName(column) {
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
		locker:  new(sync.RWMutex),
	}

	//implements tabler?
	tabler, ok := reflect.New(reflectType).Interface().(tabler)
	// Set default table name
	if ok {
		modelStruct.defaultTableName = tabler.TableName()
	} else {
		tableName := NamesMap.ToDBName(reflectType.Name())
		if scope.con == nil || !scope.con.parent.singularTable {
			tableName = inflection.Plural(tableName)
		}
		modelStruct.defaultTableName = tableName
	}

	// Get all fields
	for i := 0; i < reflectType.NumField(); i++ {
		if fieldStruct := reflectType.Field(i); ast.IsExported(fieldStruct.Name) {
			field, err := NewStructField(fieldStruct)
			if err != nil {
				scope.Err(errors.New(fmt.Sprintf(proc_tag_err, modelStruct.ModelType.Name(), err)))
				return
			}

			if !field.IsIgnored() {
				//field Value is created with the new struct field
				fieldValue := field.Value.Interface()
				if !field.IsScanner() && !field.IsTime() && field.IsEmbedOrAnon() {
					// is embedded struct
					for _, subField := range scope.NewScope(fieldValue).GetModelStruct().StructFields() {
						subField = subField.clone()
						subField.Names = append([]string{fieldStruct.Name}, subField.Names...)
						if prefix := field.GetSetting(EMBEDDED_PREFIX); prefix != "" {
							subField.DBName = prefix + subField.DBName
						}
						err = modelStruct.fieldsMap.Add(subField)
						if err != nil {
							scope.Err(errors.New(fmt.Sprintf(add_field_err, modelStruct.ModelType.Name(), err)))
							return
						}
					}
					continue
				}
			}

			err = modelStruct.fieldsMap.Add(field)
			if err != nil {
				scope.Err(errors.New(fmt.Sprintf(add_field_err, modelStruct.ModelType.Name(), err)))
				return
			}
		}
	}

	if modelStruct.noOfPKs() == 0 {
		//by default we're expecting that the modelstruct has a field named id
		if field, ok := modelStruct.fieldsMap.Get(DEFAULT_ID_NAME); ok {
			field.setFlag(IS_PRIMARYKEY)
		}
		//else - it's not an error : joins don't have primary key named id
	}
}

func (modelStruct *ModelStruct) PKs() StructFields {
	if modelStruct.cachedPrimaryFields == nil {
		modelStruct.cachedPrimaryFields = modelStruct.fieldsMap.PrimaryFields()
	}
	return modelStruct.cachedPrimaryFields
}

func (modelStruct *ModelStruct) noOfPKs() int {
	if modelStruct.cachedPrimaryFields == nil {
		modelStruct.cachedPrimaryFields = modelStruct.fieldsMap.PrimaryFields()
	}
	return modelStruct.cachedPrimaryFields.len()
}

func (modelStruct *ModelStruct) cloneFieldsToScope(indirectScopeValue reflect.Value) *StructFields {
	var result StructFields
	//Badu : can't use copy, we need to clone
	for _, structField := range modelStruct.fieldsMap.fields {
		if indirectScopeValue.Kind() == reflect.Struct {
			fieldValue := indirectScopeValue
			for _, name := range structField.Names {
				fieldValue = reflect.Indirect(fieldValue).FieldByName(name)
			}
			clonedField := structField.cloneWithValue(fieldValue)
			result.add(clonedField)
		} else {
			clonedField := structField.clone()
			clonedField.setFlag(IS_BLANK)
			result.add(clonedField)
		}
	}
	return &result
}

func (modelStruct *ModelStruct) processRelations(scope *Scope) {
	for _, field := range modelStruct.StructFields() {
		if field.HasRelations() {
			relationship := &Relationship{}
			toScope := scope.NewScope(field.Interface())
			toModelStruct := toScope.GetModelStruct()
			//ATTN : order matters, since it can be both slice and struct
			if field.IsSlice() {
				if field.Type.Kind() == reflect.Struct {
					if field.HasSetting(MANY2MANY) {
						relationship.ManyToMany(field, modelStruct, scope, toScope)
					} else {
						relationship.HasMany(field, modelStruct, toModelStruct, scope, toScope)
					}
				} else {
					field.SetIsNormal()
				}
			} else if field.IsStruct() {
				if !relationship.HasOne(field, modelStruct, toModelStruct, scope, toScope) {
					if !relationship.BelongTo(field, modelStruct, toModelStruct, scope, toScope) {
						errMsg := fmt.Errorf(no_belong_or_hasone_err, modelStruct.ModelType.Name(), field.DBName, field.GetStructName())
						scope.Warn(errMsg)
					}
				}
			}
		}
	}
}
