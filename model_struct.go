package gorm

import (
	"errors"
	"reflect"
	"strings"
)

// TableName get model's table name
func (modelStruct *ModelStruct) TableName(db *DBCon) string {
	return DefaultTableNameHandler(db, modelStruct.defaultTableName)
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

func (modelStruct *ModelStruct) fieldByName(column string) *StructField {
	field, ok := modelStruct.fields.Get(column)
	if !ok {
		//couldn't find it in "fields" map
		for _, field := range modelStruct.StructFields {
			if field.DBName == NamesMap.ToDBName(column) {
				return field
			}
		}
		//TODO : @Badu - error : oops, field not found!!!
		//fmt.Printf("Oops column %q not found in %q or in NamesMap %q\n", column, modelStruct.ModelType.Name(), NamesMap.ToDBName(column))
		return nil
	}
	return field
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
		relationship           = &Relationship{}
		toScope                = scope.New(reflect.New(field.Struct.Type).Interface())
		elemType               = field.getTrueType()
		foreignKeys            = field.getForeignKeys()
		associationForeignKeys = field.getAssocForeignKeys()
	)

	if elemType.Kind() == reflect.Struct {
		if many2many := field.GetSetting(MANY2MANY); many2many != "" {
			relationship.Kind = MANY_TO_MANY

			// if no foreign keys defined with tag
			if foreignKeys.len() == 0 {
				for _, field := range modelStruct.PrimaryFields {
					foreignKeys.add(field.DBName)
				}
			}

			for _, foreignKey := range foreignKeys {
				if foreignField := modelStruct.fieldByName(foreignKey); foreignField != nil {
					// source foreign keys (db names)
					relationship.ForeignFieldNames.add(foreignField.DBName)
					// join table foreign keys for source
					joinTableDBName := NamesMap.ToDBName(modelStruct.ModelType.Name()) + "_" + foreignField.DBName
					relationship.ForeignDBNames.add(joinTableDBName)
				}
			}

			// if no association foreign keys defined with tag
			if associationForeignKeys.len() == 0 {
				for _, field := range toScope.PrimaryFields() {
					associationForeignKeys.add(field.DBName)
				}
			}

			for _, name := range associationForeignKeys {
				if field, ok := toScope.FieldByName(name); ok {
					// association foreign keys (db names)
					relationship.AssociationForeignFieldNames.add(field.DBName)
					// join table foreign keys for association
					joinTableDBName := NamesMap.ToDBName(elemType.Name()) + "_" + field.DBName
					relationship.AssociationForeignDBNames.add(joinTableDBName)
				}
			}

			joinTableHandler := JoinTableHandler{TableName: many2many}
			joinTableHandler.Setup(relationship, modelStruct.ModelType, elemType)
			relationship.JoinTableHandler = &joinTableHandler
			field.Relationship = relationship
		} else {
			// User has many comments, associationType is User, comment use UserID as foreign key
			var associationType = modelStruct.ModelType.Name()
			var toFields = toScope.GetModelStruct()
			relationship.Kind = HAS_MANY

			if polymorphic := field.GetSetting(POLYMORPHIC); polymorphic != "" {
				// Dog has many toys, tag polymorphic is Owner, then associationType is Owner
				// Toy use OwnerID, OwnerType ('dogs') as foreign key
				if polymorphicType := toFields.fieldByName(polymorphic + "Type"); polymorphicType != nil {
					associationType = polymorphic
					relationship.PolymorphicType = polymorphicType.GetName()
					relationship.PolymorphicDBName = polymorphicType.DBName
					// if Dog has multiple set of toys set name of the set (instead of default 'dogs')
					if value := field.GetSetting(POLYMORPHIC_VALUE); value != "" {
						relationship.PolymorphicValue = value
					} else {
						relationship.PolymorphicValue = scope.TableName()
					}
					polymorphicType.SetIsForeignKey()
				}
			}

			// if no foreign keys defined with tag
			if foreignKeys.len() == 0 {
				// if no association foreign keys defined with tag
				if associationForeignKeys.len() == 0 {
					for _, field := range modelStruct.PrimaryFields {
						foreignKeys.add(associationType + field.GetName())
						associationForeignKeys.add(field.GetName())
					}
				} else {
					// generate foreign keys from defined association foreign keys
					for _, scopeFieldName := range associationForeignKeys {
						if foreignField := modelStruct.fieldByName(scopeFieldName); foreignField != nil {
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
							if foreignField := modelStruct.fieldByName(associationForeignKey); foreignField != nil {
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
				if foreignField := toFields.fieldByName(foreignKey); foreignField != nil {
					if associationField := modelStruct.fieldByName(associationForeignKeys[idx]); associationField != nil {
						// source foreign keys
						foreignField.SetIsForeignKey()
						relationship.AssociationForeignFieldNames.add(associationField.GetName())
						relationship.AssociationForeignDBNames.add(associationField.DBName)

						// association foreign keys
						relationship.ForeignFieldNames.add(foreignField.GetName())
						relationship.ForeignDBNames.add(foreignField.DBName)
					}
				}
			}

			if relationship.ForeignFieldNames.len() != 0 {
				field.Relationship = relationship
			}
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
		if polymorphicType := toFields.fieldByName(polymorphic + "Type"); polymorphicType != nil {
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
				if foreignField := modelStruct.fieldByName(associationForeignKey); foreignField != nil {
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
					if foreignField := modelStruct.fieldByName(associationForeignKey); foreignField != nil {
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
		if foreignField := toFields.fieldByName(foreignKey); foreignField != nil {
			if scopeField := modelStruct.fieldByName(associationForeignKeys[idx]); scopeField != nil {
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
					if foreignField := toFields.fieldByName(associationForeignKey); foreignField != nil {
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
						if foreignField := toFields.fieldByName(associationForeignKey); foreignField != nil {
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
			if foreignField := modelStruct.fieldByName(foreignKey); foreignField != nil {
				if associationField := toFields.fieldByName(associationForeignKeys[idx]); associationField != nil {
					foreignField.SetIsForeignKey()

					// association foreign keys
					relationship.AssociationForeignFieldNames.add(associationField.GetName())
					relationship.AssociationForeignDBNames.add(associationField.DBName)

					// source foreign keys
					relationship.ForeignFieldNames.add(foreignField.GetName())
					relationship.ForeignDBNames.add(foreignField.DBName)
				}
			}
		}

		if relationship.ForeignFieldNames.len() != 0 {
			relationship.Kind = BELONGS_TO
			field.Relationship = relationship
		}
	}
}
