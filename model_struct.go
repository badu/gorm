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

func (s *ModelStruct) getForeignField(column string) *StructField {
	//TODO : @Badu - find a easier way to deliver this, instead of iterating over slice
	//Attention : after you get rid of ToDBName and other unnecessary fields
	for _, field := range s.StructFields {
		if field.GetName() == column || field.DBName == column || field.DBName == ToDBName(column) {
			return field
		}
	}
	return nil
}

func (modelStruct *ModelStruct) sliceRelationships(scope *Scope, field *StructField, reflectType reflect.Type) {
	var (
		relationship           = &Relationship{}
		toScope                = scope.New(reflect.New(field.Struct.Type).Interface())
		foreignKeys            []string
		associationForeignKeys []string
		elemType               = field.Struct.Type
	)

	if foreignKey := field.GetSetting(FOREIGNKEY); foreignKey != "" {
		foreignKeys = strings.Split(foreignKey, ",")
	}

	if foreignKey := field.GetSetting(ASSOCIATIONFOREIGNKEY); foreignKey != "" {
		associationForeignKeys = strings.Split(foreignKey, ",")
	}

	for elemType.Kind() == reflect.Slice || elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
	}

	if elemType.Kind() == reflect.Struct {
		if many2many := field.GetSetting(MANY2MANY); many2many != "" {
			relationship.Kind = MANY_TO_MANY

			// if no foreign keys defined with tag
			if len(foreignKeys) == 0 {
				for _, field := range modelStruct.PrimaryFields {
					foreignKeys = append(foreignKeys, field.DBName)
				}
			}

			for _, foreignKey := range foreignKeys {
				if foreignField := modelStruct.getForeignField(foreignKey); foreignField != nil {
					// source foreign keys (db names)
					relationship.ForeignFieldNames = append(relationship.ForeignFieldNames, foreignField.DBName)
					// join table foreign keys for source
					joinTableDBName := ToDBName(reflectType.Name()) + "_" + foreignField.DBName
					relationship.ForeignDBNames = append(relationship.ForeignDBNames, joinTableDBName)
				}
			}

			// if no association foreign keys defined with tag
			if len(associationForeignKeys) == 0 {
				for _, field := range toScope.PrimaryFields() {
					associationForeignKeys = append(associationForeignKeys, field.DBName)
				}
			}

			for _, name := range associationForeignKeys {
				if field, ok := toScope.FieldByName(name); ok {
					// association foreign keys (db names)
					relationship.AssociationForeignFieldNames = append(relationship.AssociationForeignFieldNames, field.DBName)
					// join table foreign keys for association
					joinTableDBName := ToDBName(elemType.Name()) + "_" + field.DBName
					relationship.AssociationForeignDBNames = append(relationship.AssociationForeignDBNames, joinTableDBName)
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
				if polymorphicType := toFields.getForeignField(polymorphic + "Type"); polymorphicType != nil {
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
			if len(foreignKeys) == 0 {
				// if no association foreign keys defined with tag
				if len(associationForeignKeys) == 0 {
					for _, field := range modelStruct.PrimaryFields {
						foreignKeys = append(foreignKeys, associationType+field.GetName())
						associationForeignKeys = append(associationForeignKeys, field.GetName())
					}
				} else {
					// generate foreign keys from defined association foreign keys
					for _, scopeFieldName := range associationForeignKeys {
						if foreignField := modelStruct.getForeignField(scopeFieldName); foreignField != nil {
							foreignKeys = append(foreignKeys, associationType+foreignField.GetName())
							associationForeignKeys = append(associationForeignKeys, foreignField.GetName())
						}
					}
				}
			} else {
				// generate association foreign keys from foreign keys
				if len(associationForeignKeys) == 0 {
					for _, foreignKey := range foreignKeys {
						if strings.HasPrefix(foreignKey, associationType) {
							associationForeignKey := strings.TrimPrefix(foreignKey, associationType)
							if foreignField := modelStruct.getForeignField(associationForeignKey); foreignField != nil {
								associationForeignKeys = append(associationForeignKeys, associationForeignKey)
							}
						}
					}
					if len(associationForeignKeys) == 0 && len(foreignKeys) == 1 {
						associationForeignKeys = []string{scope.PrimaryKey()}
					}
				} else if len(foreignKeys) != len(associationForeignKeys) {
					scope.Err(errors.New("invalid foreign keys, should have same length"))
					return
				}
			}

			for idx, foreignKey := range foreignKeys {
				if foreignField := toFields.getForeignField(foreignKey); foreignField != nil {
					if associationField := modelStruct.getForeignField(associationForeignKeys[idx]); associationField != nil {
						// source foreign keys
						foreignField.IsForeignKey = true
						relationship.AssociationForeignFieldNames = append(relationship.AssociationForeignFieldNames, associationField.GetName())
						relationship.AssociationForeignDBNames = append(relationship.AssociationForeignDBNames, associationField.DBName)

						// association foreign keys
						relationship.ForeignFieldNames = append(relationship.ForeignFieldNames, foreignField.GetName())
						relationship.ForeignDBNames = append(relationship.ForeignDBNames, foreignField.DBName)
					}
				}
			}

			if len(relationship.ForeignFieldNames) != 0 {
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
		tagForeignKeys            []string
		tagAssociationForeignKeys []string
	)

	if foreignKey := field.GetSetting(FOREIGNKEY); foreignKey != "" {
		tagForeignKeys = strings.Split(foreignKey, ",")
	}

	if foreignKey := field.GetSetting(ASSOCIATIONFOREIGNKEY); foreignKey != "" {
		tagAssociationForeignKeys = strings.Split(foreignKey, ",")
	}

	if polymorphic := field.GetSetting(POLYMORPHIC); polymorphic != "" {
		// Cat has one toy, tag polymorphic is Owner, then associationType is Owner
		// Toy use OwnerID, OwnerType ('cats') as foreign key
		if polymorphicType := toFields.getForeignField(polymorphic + "Type"); polymorphicType != nil {
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
	if len(foreignKeys) == 0 {
		// if no association foreign keys defined with tag
		if len(associationForeignKeys) == 0 {
			for _, primaryField := range modelStruct.PrimaryFields {
				foreignKeys = append(foreignKeys, associationType+primaryField.GetName())
				associationForeignKeys = append(associationForeignKeys, primaryField.GetName())
			}
		} else {
			// generate foreign keys form association foreign keys
			for _, associationForeignKey := range tagAssociationForeignKeys {
				if foreignField := modelStruct.getForeignField(associationForeignKey); foreignField != nil {
					foreignKeys = append(foreignKeys, associationType+foreignField.GetName())
					associationForeignKeys = append(associationForeignKeys, foreignField.GetName())
				}
			}
		}
	} else {
		// generate association foreign keys from foreign keys
		if len(associationForeignKeys) == 0 {
			for _, foreignKey := range foreignKeys {
				if strings.HasPrefix(foreignKey, associationType) {
					associationForeignKey := strings.TrimPrefix(foreignKey, associationType)
					if foreignField := modelStruct.getForeignField(associationForeignKey); foreignField != nil {
						associationForeignKeys = append(associationForeignKeys, associationForeignKey)
					}
				}
			}
			if len(associationForeignKeys) == 0 && len(foreignKeys) == 1 {
				associationForeignKeys = []string{scope.PrimaryKey()}
			}
		} else if len(foreignKeys) != len(associationForeignKeys) {
			scope.Err(errors.New("invalid foreign keys, should have same length"))
			return
		}
	}

	for idx, foreignKey := range foreignKeys {
		if foreignField := toFields.getForeignField(foreignKey); foreignField != nil {
			if scopeField := modelStruct.getForeignField(associationForeignKeys[idx]); scopeField != nil {
				foreignField.IsForeignKey = true
				// source foreign keys
				relationship.AssociationForeignFieldNames = append(relationship.AssociationForeignFieldNames, scopeField.GetName())
				relationship.AssociationForeignDBNames = append(relationship.AssociationForeignDBNames, scopeField.DBName)

				// association foreign keys
				relationship.ForeignFieldNames = append(relationship.ForeignFieldNames, foreignField.GetName())
				relationship.ForeignDBNames = append(relationship.ForeignDBNames, foreignField.DBName)
			}
		}
	}

	if len(relationship.ForeignFieldNames) != 0 {
		relationship.Kind = HAS_ONE
		field.Relationship = relationship
	} else {
		var foreignKeys = tagForeignKeys
		var associationForeignKeys = tagAssociationForeignKeys

		if len(foreignKeys) == 0 {
			// generate foreign keys & association foreign keys
			if len(associationForeignKeys) == 0 {
				for _, primaryField := range toScope.PrimaryFields() {
					foreignKeys = append(foreignKeys, field.GetName()+primaryField.GetName())
					associationForeignKeys = append(associationForeignKeys, primaryField.GetName())
				}
			} else {
				// generate foreign keys with association foreign keys
				for _, associationForeignKey := range associationForeignKeys {
					if foreignField := toFields.getForeignField(associationForeignKey); foreignField != nil {
						foreignKeys = append(foreignKeys, field.GetName()+foreignField.GetName())
						associationForeignKeys = append(associationForeignKeys, foreignField.GetName())
					}
				}
			}
		} else {
			// generate foreign keys & association foreign keys
			if len(associationForeignKeys) == 0 {
				for _, foreignKey := range foreignKeys {
					if strings.HasPrefix(foreignKey, field.GetName()) {
						associationForeignKey := strings.TrimPrefix(foreignKey, field.GetName())
						if foreignField := toFields.getForeignField(associationForeignKey); foreignField != nil {
							associationForeignKeys = append(associationForeignKeys, associationForeignKey)
						}
					}
				}
				if len(associationForeignKeys) == 0 && len(foreignKeys) == 1 {
					associationForeignKeys = []string{toScope.PrimaryKey()}
				}
			} else if len(foreignKeys) != len(associationForeignKeys) {
				scope.Err(errors.New("invalid foreign keys, should have same length"))
				return
			}
		}

		for idx, foreignKey := range foreignKeys {
			if foreignField := modelStruct.getForeignField(foreignKey); foreignField != nil {
				if associationField := toFields.getForeignField(associationForeignKeys[idx]); associationField != nil {
					foreignField.IsForeignKey = true

					// association foreign keys
					relationship.AssociationForeignFieldNames = append(relationship.AssociationForeignFieldNames, associationField.GetName())
					relationship.AssociationForeignDBNames = append(relationship.AssociationForeignDBNames, associationField.DBName)

					// source foreign keys
					relationship.ForeignFieldNames = append(relationship.ForeignFieldNames, foreignField.GetName())
					relationship.ForeignDBNames = append(relationship.ForeignDBNames, foreignField.DBName)
				}
			}
		}

		if len(relationship.ForeignFieldNames) != 0 {
			relationship.Kind = BELONGS_TO
			field.Relationship = relationship
		}
	}
}
