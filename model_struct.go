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
		if field.IsNormal() {
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
		l:       new(sync.RWMutex),
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
			field.SetIsPrimaryKey()
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

func (modelStruct *ModelStruct) processRelations(scope *Scope) {
	for _, field := range modelStruct.StructFields() {
		if field.HasRelations() {
			relationship := &Relationship{}
			toScope := scope.NewScope(field.Interface())
			toModelStruct := toScope.GetModelStruct()
			//ATTN : order matters, since it can be both slice and struct
			if field.IsSlice() {
				if field.IsStruct() {
					//it's a slice of structs
					if field.HasSetting(MANY2MANY) {
						//many to many
						relationship.ManyToMany(field, modelStruct, scope, toScope)
					} else {
						//has many
						relationship.HasMany(field, modelStruct, toModelStruct, scope, toScope)
					}
				} else {
					//it's a slice of primitive
					field.SetIsNormal()

				}
			} else if field.IsStruct() {
				//it's a struct - attempt to check if has one
				if !relationship.HasOne(field, modelStruct, toModelStruct, scope, toScope) {
					//attempt to check if belongs to
					if !relationship.BelongTo(field, modelStruct, toModelStruct, scope, toScope) {
						//Oops, neither
						errMsg := fmt.Errorf(no_belong_or_hasone_err, modelStruct.ModelType.Name(), field.DBName, field.StructName)
						scope.Warn(errMsg)
					}
				}
			}
		}
		//unsetting the flag, so we can use HasRelations() instead of checking for relationship == nil
		if field.Relationship == nil {
			errMsg := fmt.Errorf(bad_relationship, "ModelStruct")
			scope.Warn(errMsg)
			field.UnsetHasRelations()
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
