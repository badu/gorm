package gorm

import (
	"fmt"
	"github.com/jinzhu/inflection"
	"go/ast"
	"reflect"
	"sync"
)

// TableName get model's table name
func (m *ModelStruct) TableName(db *DBCon) string {
	return DefaultTableNameHandler(db, m.defaultTableName)
}

func (m *ModelStruct) StructFields() StructFields {
	return m.fieldsMap.fields
}

func (m *ModelStruct) Interface() interface{} {
	return reflect.New(m.ModelType).Interface()
}

func (m *ModelStruct) HasColumn(column string) bool {
	//looking for it
	field, ok := m.fieldsMap.get(column)
	if ok {
		//TODO : @Badu only if it's normal it's declared ?
		if field.IsNormal() {
			return true
		}
	}
	return false
}

func (m *ModelStruct) FieldByName(column string, con *DBCon) (*StructField, bool) {
	field, ok := m.fieldsMap.get(column)
	if !ok {
		//couldn't find it in "fields" map
		for _, field := range m.fieldsMap.fields {
			if field.DBName == con.namesMap.toDBName(column) {
				return field, true
			}
		}
		return nil, false
	}
	return field, ok
}

func (m *ModelStruct) Create(scope *Scope) {
	m.ModelType = scope.rType
	m.fieldsMap = fieldsMap{
		aliases: make(map[string]*StructField),
		mu:      new(sync.RWMutex),
	}

	//implements tabler?
	tabler, ok := reflect.New(scope.rType).Interface().(tabler)
	// Set default table name
	if ok {
		m.defaultTableName = tabler.TableName()
	} else {
		tableName := scope.con.parent.namesMap.toDBName(scope.rType.Name())
		if scope.con == nil || !scope.con.parent.singularTable {
			tableName = inflection.Plural(tableName)
		}
		m.defaultTableName = tableName
	}
	// Get all fields
	for i := 0; i < scope.rType.NumField(); i++ {
		if fieldStruct := scope.rType.Field(i); ast.IsExported(fieldStruct.Name) {
			field, err := NewStructField(fieldStruct, scope.con.parent.namesMap.toDBName(fieldStruct.Name))
			if err != nil {
				scope.Err(fmt.Errorf(errProcessingTags, m.ModelType.Name(), err))
				return
			}

			if !field.IsIgnored() {
				//field Value is created with the new struct field
				fieldValue := field.Value.Interface()
				if !field.IsScanner() && !field.IsTime() && field.IsEmbedOrAnon() {
					// is embedded struct
					for _, subField := range scope.con.emptyScope(fieldValue).GetModelStruct().StructFields() {
						subField = subField.clone()
						subField.Names = append([]string{fieldStruct.Name}, subField.Names...)

						if field.HasSetting(setEmbeddedPrefix) {
							subField.DBName = field.GetStrSetting(setEmbeddedPrefix) + subField.DBName
						}

						err = m.fieldsMap.add(subField)
						if err != nil {
							scope.Err(fmt.Errorf(errAddingField, m.ModelType.Name(), err))
							return
						}
					}
					continue
				}
			}

			err = m.fieldsMap.add(field)
			if err != nil {
				scope.Err(fmt.Errorf(errAddingField, m.ModelType.Name(), err))
				return
			}
		}
	}

	if m.noOfPKs() == 0 {
		//by default we're expecting that the modelstruct has a field named id
		if field, ok := m.fieldsMap.get(fieldDefaultIdName); ok {
			field.SetIsPrimaryKey()
		}
		//else - it's not an error : joins don't have primary key named id
	}
}

func (m *ModelStruct) PKs() StructFields {
	if m.cachedPrimaryFields == nil {
		m.cachedPrimaryFields = m.fieldsMap.primaryFields()
	}
	return m.cachedPrimaryFields
}

func (m *ModelStruct) noOfPKs() int {
	if m.cachedPrimaryFields == nil {
		m.cachedPrimaryFields = m.fieldsMap.primaryFields()
	}
	return m.cachedPrimaryFields.len()
}

func (m *ModelStruct) processRelations(scope *Scope) {
	for _, field := range m.StructFields() {
		if field.WillCheckRelations() {
			toScope := scope.con.emptyScope(field.Interface())
			toModelStruct := toScope.GetModelStruct()
			//ATTN : order matters, since it can be both slice and struct
			if field.IsSlice() {
				if field.IsStruct() {
					//it's a slice of structs
					if field.HasSetting(setMany2manyName) {
						//many to many
						makeManyToMany(field, m, scope, toScope)
					} else {
						//has many
						makeHasMany(field, m, toModelStruct, scope, toScope)
					}
				} else {
					//it's a slice of primitive
					field.SetIsNormal()

				}
			} else if field.IsStruct() {
				//it's a struct - attempt to check if has one
				if !makeHasOne(field, m, toModelStruct, scope, toScope) {
					//attempt to check if belongs to
					if !makeBelongTo(field, m, toModelStruct, scope, toScope) {
						//Oops, neither
						scope.Warn(fmt.Errorf(errNoBelongOrHasone, m.ModelType.Name(), field.DBName, field.StructName))
					}
				}
			}

			field.UnsetCheckRelations()
		}
	}
}

//implementation of Stringer
func (m ModelStruct) String() string {
	var collector Collector
	collector.add("%s = %s\n", "Table name", m.defaultTableName)
	if m.ModelType == nil {
		collector.add("%s = %s\n", "UNTYPED!!!")
	} else {
		collector.add("%s = %s\n", "Type", m.ModelType.String())
	}
	collector.add("Fields:\n\n")
	for fn, f := range m.fieldsMap.fields {
		collector.add("#%d\n%s\n", fn+1, f)
	}
	return collector.String()
}
