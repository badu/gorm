package gorm

import (
	"errors"
	"reflect"
	"strings"
)

// TableName get model's table name
func (s *ModelStruct) TableName(db *DBCon) string {
	return DefaultTableNameHandler(db, s.defaultTableName)
}

func (s *ModelStruct) fieldByName(column string) *StructField {
	//TODO : @Badu - find a easier way to deliver this, instead of iterating over slice
	//since ModelStructs are cached, it wouldn't be a problem to have a safeMap here too, which delivers the field
	for _, field := range s.StructFields {
		if field.GetName() == column || field.DBName == column || field.DBName == NamesMap.ToDBName(column) {
			return field
		}
	}
	return nil
}

func (modelStruct *ModelStruct) sliceRelationships(scope *Scope, field *StructField, reflectType reflect.Type) {
	var (
		relationship           = &Relationship{}
		toScope                = scope.New(reflect.New(field.Struct.Type).Interface())
		foreignKeys            StrSlice
		associationForeignKeys StrSlice
		elemType               = field.Struct.Type
	)

	if foreignKey := field.GetSetting(FOREIGNKEY); foreignKey != "" {
		foreignKeys.commaLoad(foreignKey)
	}

	if foreignKey := field.GetSetting(ASSOCIATIONFOREIGNKEY); foreignKey != "" {
		associationForeignKeys.commaLoad(foreignKey)
	}

	for elemType.Kind() == reflect.Slice || elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
	}

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
					joinTableDBName := NamesMap.ToDBName(reflectType.Name()) + "_" + foreignField.DBName
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

			joinTableHandler := JoinTableHandler{}
			joinTableHandler.Setup(relationship, many2many, reflectType, elemType)
			relationship.JoinTableHandler = &joinTableHandler
			field.Relationship = relationship
		} else {
			// User has many comments, associationType is User, comment use UserID as foreign key
			var associationType = reflectType.Name()
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
					polymorphicType.IsForeignKey = true
				}
			}

			// if no foreign keys defined with tag
			if foreignKeys.len() == 0 {
				// if no association foreign keys defined with tag
				if associationForeignKeys.len() == 0 {
					for _, field := range modelStruct.PrimaryFields {
						foreignKeys.add( associationType+field.GetName())
						associationForeignKeys.add(field.GetName())
					}
				} else {
					// generate foreign keys from defined association foreign keys
					for _, scopeFieldName := range associationForeignKeys {
						if foreignField := modelStruct.fieldByName(scopeFieldName); foreignField != nil {
							foreignKeys.add(associationType+foreignField.GetName())
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
						foreignField.IsForeignKey = true
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
		field.IsNormal = true
	}
}

func (modelStruct *ModelStruct) structRelationships(scope *Scope, field *StructField, reflectType reflect.Type) {
	var (
		// user has one profile, associationType is User, profile use UserID as foreign key
		// user belongs to profile, associationType is Profile, user use ProfileID as foreign key
		associationType           = reflectType.Name()
		relationship              = &Relationship{}
		toScope                   = scope.New(reflect.New(field.Struct.Type).Interface())
		toFields                  = toScope.GetModelStruct()
		tagForeignKeys            StrSlice
		tagAssociationForeignKeys StrSlice
	)

	if foreignKey := field.GetSetting(FOREIGNKEY); foreignKey != "" {
		tagForeignKeys.commaLoad(foreignKey)
	}

	if foreignKey := field.GetSetting(ASSOCIATIONFOREIGNKEY); foreignKey != "" {
		tagAssociationForeignKeys.commaLoad(foreignKey)
	}

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
			polymorphicType.IsForeignKey = true
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
				foreignKeys.add(associationType+primaryField.GetName())
				associationForeignKeys.add(primaryField.GetName())
			}
		} else {
			// generate foreign keys form association foreign keys
			for _, associationForeignKey := range tagAssociationForeignKeys {
				if foreignField := modelStruct.fieldByName(associationForeignKey); foreignField != nil {
					foreignKeys.add(associationType+foreignField.GetName())
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
				foreignField.IsForeignKey = true
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
					foreignKeys.add(field.GetName()+primaryField.GetName())
					associationForeignKeys.add(primaryField.GetName())
				}
			} else {
				// generate foreign keys with association foreign keys
				for _, associationForeignKey := range associationForeignKeys {
					if foreignField := toFields.fieldByName(associationForeignKey); foreignField != nil {
						foreignKeys.add(field.GetName()+foreignField.GetName())
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
					foreignField.IsForeignKey = true

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
