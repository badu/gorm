package gorm

import (
	"errors"
	"fmt"
	"github.com/jinzhu/inflection"
	"go/ast"
	"reflect"
	"strings"
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
					modelStruct.addPK(field)
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
								modelStruct.addPK(subField)
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
			modelStruct.addPK(field)
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

func (modelStruct *ModelStruct) addPK(field *StructField) {
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
			//ATTN : order matters, since it can be both slice and struct
			if field.IsSlice() {
				modelStruct.sliceRelationships(scope, field)
			} else if field.IsStruct() {
				modelStruct.structRelationships(scope, field)
			}
		}
	}
}

func (modelStruct *ModelStruct) sliceRelationships(scope *Scope, field *StructField) {
	var (
		relationship = &Relationship{}
		toScope      = scope.New(reflect.New(field.Struct.Type).Interface())
		elemType     = field.getTrueType()
	)

	if elemType.Kind() == reflect.Struct {
		if many2many := field.GetSetting(MANY2MANY); many2many != "" {
			relationship.ManyToMany(field, modelStruct, toScope)
		} else {
			relationship.HasMany(field, modelStruct, scope, toScope)
		}
	} else {
		field.SetIsNormal()
	}
}

func (modelStruct *ModelStruct) structRelationships(scope *Scope, field *StructField) {
	var (
		// user has one profile, associationType is User, profile use UserID as foreign key
		// user belongs to profile, associationType is Profile, user use ProfileID as foreign key
		associationType           = modelStruct.ModelType.Name()
		relationship              = &Relationship{}
		toScope                   = scope.New(reflect.New(field.Struct.Type).Interface())
		toFields                  = toScope.GetModelStruct()
		tagForeignKeys            = field.getForeignKeys()
		tagAssociationForeignKeys = field.getAssocForeignKeys()
	)

	if polymorphic := field.GetSetting(POLYMORPHIC); polymorphic != "" {
		// Cat has one toy, tag polymorphic is Owner, then associationType is Owner
		// Toy use OwnerID, OwnerType ('cats') as foreign key
		if polymorphicType, ok := toFields.FieldByName(polymorphic + "Type"); ok {
			associationType = polymorphic
			relationship.PolymorphicType = polymorphicType.GetName()
			relationship.PolymorphicDBName = polymorphicType.DBName
			// if Cat has several different types of toys set name for each (instead of default 'cats')
			if value := field.GetSetting(POLYMORPHIC_VALUE); value != "" {
				relationship.PolymorphicValue = value
			} else {
				relationship.PolymorphicValue = scope.TableName()
			}
			polymorphicType.SetIsForeignKey()
		}
	}

	// Has One
	var foreignKeys = tagForeignKeys
	var associationForeignKeys = tagAssociationForeignKeys
	// if no foreign keys defined with tag
	if foreignKeys.len() == 0 {
		// if no association foreign keys defined with tag
		if associationForeignKeys.len() == 0 {
			for _, primaryField := range modelStruct.PrimaryFields {
				foreignKeys.add(associationType + primaryField.GetName())
				associationForeignKeys.add(primaryField.GetName())
			}
		} else {
			// generate foreign keys form association foreign keys
			for _, associationForeignKey := range tagAssociationForeignKeys {
				if foreignField, ok := modelStruct.FieldByName(associationForeignKey); ok {
					foreignKeys.add(associationType + foreignField.GetName())
					associationForeignKeys.add(foreignField.GetName())
				}
			}
		}
	} else {
		// generate association foreign keys from foreign keys
		if associationForeignKeys.len() == 0 {
			for _, foreignKey := range foreignKeys {
				if strings.HasPrefix(foreignKey, associationType) {
					associationForeignKey := strings.TrimPrefix(foreignKey, associationType)
					//TODO : @Badu - see that code repeats everywhere in this
					if _, ok := modelStruct.FieldByName(associationForeignKey); ok {
						associationForeignKeys.add(associationForeignKey)
					}
				}
			}
			if associationForeignKeys.len() == 0 && foreignKeys.len() == 1 {
				associationForeignKeys = StrSlice{scope.PrimaryKey()}
			}
		} else if foreignKeys.len() != associationForeignKeys.len() {
			scope.Err(errors.New("invalid foreign keys, should have same length"))
			return
		}
	}

	for idx, foreignKey := range foreignKeys {
		if foreignField, ok := toFields.FieldByName(foreignKey); ok {
			if scopeField, ok := modelStruct.FieldByName(associationForeignKeys[idx]); ok {
				foreignField.SetIsForeignKey()
				// source foreign keys
				relationship.AssociationForeignFieldNames.add(scopeField.GetName())
				relationship.AssociationForeignDBNames.add(scopeField.DBName)

				// association foreign keys
				relationship.ForeignFieldNames.add(foreignField.GetName())
				relationship.ForeignDBNames.add(foreignField.DBName)
			}
		}
	}

	if relationship.ForeignFieldNames.len() != 0 {
		relationship.Kind = HAS_ONE
		field.Relationship = relationship
	} else {
		var foreignKeys = tagForeignKeys
		var associationForeignKeys = tagAssociationForeignKeys

		if foreignKeys.len() == 0 {
			// generate foreign keys & association foreign keys
			if associationForeignKeys.len() == 0 {
				for _, primaryField := range toScope.PrimaryFields() {
					foreignKeys.add(field.GetName() + primaryField.GetName())
					associationForeignKeys.add(primaryField.GetName())
				}
			} else {
				// generate foreign keys with association foreign keys
				for _, associationForeignKey := range associationForeignKeys {
					if foreignField, ok := toFields.FieldByName(associationForeignKey); ok {
						foreignKeys.add(field.GetName() + foreignField.GetName())
						associationForeignKeys.add(foreignField.GetName())
					}
				}
			}
		} else {
			// generate foreign keys & association foreign keys
			if associationForeignKeys.len() == 0 {
				for _, foreignKey := range foreignKeys {
					if strings.HasPrefix(foreignKey, field.GetName()) {
						associationForeignKey := strings.TrimPrefix(foreignKey, field.GetName())
						if _, ok := toFields.FieldByName(associationForeignKey); ok {
							associationForeignKeys.add(associationForeignKey)
						}
					}
				}
				if associationForeignKeys.len() == 0 && foreignKeys.len() == 1 {
					associationForeignKeys = StrSlice{toScope.PrimaryKey()}
				}
			} else if foreignKeys.len() != associationForeignKeys.len() {
				scope.Err(errors.New("invalid foreign keys, should have same length"))
				return
			}
		}

		for idx, foreignKey := range foreignKeys {
			if foreignField, ok := modelStruct.FieldByName(foreignKey); ok {
				if associationField, ok := toFields.FieldByName(associationForeignKeys[idx]); ok {
					foreignField.SetIsForeignKey()

					// association foreign keys
					relationship.AssociationForeignFieldNames.add(associationField.GetName())
					relationship.AssociationForeignDBNames.add(associationField.DBName)

					// source foreign keys
					relationship.ForeignFieldNames.add(foreignField.GetName())
					relationship.ForeignDBNames.add(foreignField.DBName)
				}
			}
			//TODO : @Badu - error if !ok EVERYWHERE
		}

		if relationship.ForeignFieldNames.len() != 0 {
			relationship.Kind = BELONGS_TO
			field.Relationship = relationship
		}
	}
}
