package gorm

import (
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

func (modelStruct *ModelStruct) HasColumn(column string) bool {
	if modelStruct.fields.l == nil {
		return false
	}
	field, ok := modelStruct.fields.Get(column)
	if ok {
		if field.hasFlag(IS_NORMAL) {
			return true
		}
	}
	return false
}

func (modelStruct *ModelStruct) Create(reflectType reflect.Type, scope *Scope) {
	modelStruct.ModelType = reflectType
	modelStruct.fields = fieldsMap{m: make(map[string]*StructField), l: new(sync.RWMutex)}

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
				fmt.Printf("ERROR processing tags : %v\n", err)
				//TODO : @Badu - catch this error - we might fail processing tags
			}
			// is ignored field
			if !field.IsIgnored() {
				if field.HasSetting(PRIMARY_KEY) {
					modelStruct.addPrimaryKey(field)
				}
				fieldValue := field.checkInterfaces()
				if !field.IsScanner() && !field.IsTime() {
					if field.IsEmbedOrAnon() {
						// is embedded struct
						for _, subField := range scope.New(fieldValue).GetModelStruct().StructFields {
							subField = subField.clone()
							subField.Names = append([]string{fieldStruct.Name}, subField.Names...)
							if prefix := field.GetSetting(EMBEDDED_PREFIX); prefix != "" {
								subField.DBName = prefix + subField.DBName
							}
							if subField.IsPrimaryKey() {
								modelStruct.addPrimaryKey(subField)
							}
							modelStruct.addField(subField)
						}
						continue
					} else {
						if field.IsSlice() {
							//marker for later processing of relationships
							field.SetHasRelations()
						} else if field.IsStruct() {
							//marker for later processing of relationships
							field.SetHasRelations()
						} else {
							field.SetIsNormal()
						}
					}
				}
			}

			modelStruct.addField(field)
		}
	}

	if modelStruct.PrimaryFields.len() == 0 {
		//TODO : @Badu - a boiler plate string. Get rid of it!
		if field, ok := modelStruct.FieldByName("id"); ok {
			field.setFlag(IS_PRIMARYKEY)
			modelStruct.addPrimaryKey(field)
		}
	}
}

func (modelStruct *ModelStruct) FieldByName(column string) (*StructField, bool) {
	field, ok := modelStruct.fields.Get(column)
	if !ok {
		//couldn't find it in "fields" map
		for _, field := range modelStruct.StructFields {
			if field.DBName == NamesMap.ToDBName(column) {
				return field, true
			}
		}
		//TODO : @Badu - error : oops, field not found!!!
		//fmt.Printf("Oops column %q not found in %q or in NamesMap %q\n", column, modelStruct.ModelType.Name(), NamesMap.ToDBName(column))
		return nil, false
	}
	return field, ok
}

func (modelStruct *ModelStruct) cloneFieldsToScope(indirectScopeValue reflect.Value) *StructFields {
	var result StructFields
	for _, structField := range modelStruct.StructFields {
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

func (modelStruct *ModelStruct) addPrimaryKey(field *StructField) {
	//fmt.Printf("Add field %q (%q) to model struct %q\n", field.GetName(), field.DBName, modelStruct.ModelType.Name())
	modelStruct.fields.Set(field.GetName(), field)
	modelStruct.fields.Set(field.DBName, field)
	modelStruct.PrimaryFields.add(field)
}

func (modelStruct *ModelStruct) addField(field *StructField) {
	//fmt.Printf("Add field %q (%q) to model struct %q\n", field.GetName(), field.DBName, modelStruct.ModelType.Name())
	modelStruct.fields.Set(field.GetName(), field)
	modelStruct.fields.Set(field.DBName, field)
	modelStruct.StructFields.add(field)
}

func (modelStruct *ModelStruct) processRelations(scope *Scope) {
	for _, field := range modelStruct.StructFields {
		if field.HasRelations() {
			relationship := &Relationship{}
			toScope := scope.New(reflect.New(field.Struct.Type).Interface())
			//ATTN : order matters, since it can be both slice and struct
			if field.IsSlice() {
				elemType := field.getTrueType()
				if elemType.Kind() == reflect.Struct {
					if field.HasSetting(MANY2MANY) {
						relationship.ManyToMany(field, modelStruct, toScope)
					} else {
						relationship.HasMany(field, modelStruct, scope, toScope)
					}
				} else {
					field.SetIsNormal()
				}
			} else if field.IsStruct() {
				toModelStruct := toScope.GetModelStruct()
				relationship.HasOne(field, modelStruct, toModelStruct, scope)
				relationship.BelongTo(field, modelStruct, toModelStruct, scope, toScope)
			}
		}
	}
}
