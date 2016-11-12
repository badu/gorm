package gorm

import (
	"errors"
	"strings"
)

func (relationship *Relationship) ManyToMany(field *StructField, modelStruct *ModelStruct, scope *Scope) {

	elemType := field.getTrueType()
	foreignKeys := field.getForeignKeys()
	associationForeignKeys := field.getAssocForeignKeys()
	modelType := modelStruct.ModelType
	many2many := field.GetSetting(MANY2MANY)

	relationship.Kind = MANY_TO_MANY
	// if no foreign keys defined with tag
	if foreignKeys.len() == 0 {
		for _, field := range modelStruct.PrimaryFields {
			foreignKeys.add(field.DBName)
		}
	}

	for _, foreignKey := range foreignKeys {
		if foreignField, ok := modelStruct.FieldByName(foreignKey); ok {
			// source foreign keys (db names)
			relationship.ForeignFieldNames.add(foreignField.DBName)
			// join table foreign keys for source
			joinTableDBName := NamesMap.ToDBName(modelType.Name()) + "_" + foreignField.DBName
			relationship.ForeignDBNames.add(joinTableDBName)
		}
	}

	// if no association foreign keys defined with tag
	if associationForeignKeys.len() == 0 {
		for _, field := range scope.PrimaryFields() {
			associationForeignKeys.add(field.DBName)
		}
	}

	for _, name := range associationForeignKeys {
		if field, ok := scope.FieldByName(name); ok {
			// association foreign keys (db names)
			relationship.AssociationForeignFieldNames.add(field.DBName)
			// join table foreign keys for association
			joinTableDBName := NamesMap.ToDBName(elemType.Name()) + "_" + field.DBName
			relationship.AssociationForeignDBNames.add(joinTableDBName)
		}
	}

	joinTableHandler := JoinTableHandler{TableName: many2many}
	joinTableHandler.Setup(relationship, modelType, elemType)
	relationship.JoinTableHandler = &joinTableHandler
	field.Relationship = relationship
}

func (relationship *Relationship) HasMany(field *StructField, modelStruct *ModelStruct, fromScope *Scope, toScope *Scope) {

	foreignKeys := field.getForeignKeys()
	associationForeignKeys := field.getAssocForeignKeys()
	modelType := modelStruct.ModelType

	// User has many comments, associationType is User, comment use UserID as foreign key
	var associationType = modelType.Name()
	var toModelStruct = toScope.GetModelStruct()
	relationship.Kind = HAS_MANY

	if polymorphic := field.GetSetting(POLYMORPHIC); polymorphic != "" {
		// Dog has many toys, tag polymorphic is Owner, then associationType is Owner
		// Toy use OwnerID, OwnerType ('dogs') as foreign key
		if polymorphicType, ok := toModelStruct.FieldByName(polymorphic + "Type"); ok {
			associationType = polymorphic
			relationship.PolymorphicType = polymorphicType.GetName()
			relationship.PolymorphicDBName = polymorphicType.DBName
			// if Dog has multiple set of toys set name of the set (instead of default 'dogs')
			if value := field.GetSetting(POLYMORPHIC_VALUE); value != "" {
				relationship.PolymorphicValue = value
			} else {
				relationship.PolymorphicValue = fromScope.TableName()
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
				if foreignField, ok := modelStruct.FieldByName(scopeFieldName); ok {
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
					if _, ok := modelStruct.FieldByName(associationForeignKey); ok {
						associationForeignKeys.add(associationForeignKey)
					}
				}
			}
			if associationForeignKeys.len() == 0 && foreignKeys.len() == 1 {
				associationForeignKeys = StrSlice{fromScope.PrimaryKey()}
			}
		} else if foreignKeys.len() != associationForeignKeys.len() {
			fromScope.Err(errors.New("invalid foreign keys, should have same length"))
			return
		}
	}

	for idx, foreignKey := range foreignKeys {
		if foreignField, ok := toModelStruct.FieldByName(foreignKey); ok {
			if associationField, ok := modelStruct.FieldByName(associationForeignKeys[idx]); ok {
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


func (relationship *Relationship) HasOne(field *StructField, fromModelStruct, toModelStruct *ModelStruct, fromScope *Scope) {
	associationType := fromModelStruct.ModelType.Name()
	tagForeignKeys := field.getForeignKeys()
	tagAssociationForeignKeys := field.getAssocForeignKeys()

	//check polymorphic
	if polymorphic := field.GetSetting(POLYMORPHIC); polymorphic != "" {
		// Cat has one toy, tag polymorphic is Owner, then associationType is Owner
		// Toy use OwnerID, OwnerType ('cats') as foreign key
		if polymorphicType, ok := toModelStruct.FieldByName(polymorphic + "Type"); ok {
			//has polymorphic, modify associationType from model-type.name to poly-name
			associationType = polymorphic
			relationship.PolymorphicType = polymorphicType.GetName()
			relationship.PolymorphicDBName = polymorphicType.DBName
			// if Cat has several different types of toys set name for each (instead of default 'cats')
			if value := field.GetSetting(POLYMORPHIC_VALUE); value != "" {
				relationship.PolymorphicValue = value
			} else {
				relationship.PolymorphicValue = fromScope.TableName()
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
			for _, primaryField := range fromModelStruct.PrimaryFields {
				foreignKeys.add(associationType + primaryField.GetName())
				associationForeignKeys.add(primaryField.GetName())
			}
		} else {
			// generate foreign keys form association foreign keys
			for _, associationForeignKey := range tagAssociationForeignKeys {
				if foreignField, ok := fromModelStruct.FieldByName(associationForeignKey); ok {
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
					if _, ok := fromModelStruct.FieldByName(associationForeignKey); ok {
						associationForeignKeys.add(associationForeignKey)
					}
				}
			}
			if associationForeignKeys.len() == 0 && foreignKeys.len() == 1 {
				associationForeignKeys = StrSlice{fromScope.PrimaryKey()}
			}
		} else if foreignKeys.len() != associationForeignKeys.len() {
			fromScope.Err(errors.New("invalid foreign keys, should have same length"))
			return
		}
	}

	for idx, foreignKey := range foreignKeys {
		if foreignField, ok := toModelStruct.FieldByName(foreignKey); ok {
			if scopeField, ok := fromModelStruct.FieldByName(associationForeignKeys[idx]); ok {
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
}

func (relationship *Relationship) BelongTo(field *StructField, fromModelStruct, toModelStruct *ModelStruct, fromScope, toScope *Scope) {
	tagForeignKeys := field.getForeignKeys()
	tagAssociationForeignKeys := field.getAssocForeignKeys()

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
					if foreignField, ok := toModelStruct.FieldByName(associationForeignKey); ok {
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
						if _, ok := toModelStruct.FieldByName(associationForeignKey); ok {
							associationForeignKeys.add(associationForeignKey)
						}
					}
				}
				if associationForeignKeys.len() == 0 && foreignKeys.len() == 1 {
					associationForeignKeys = StrSlice{toScope.PrimaryKey()}
				}
			} else if foreignKeys.len() != associationForeignKeys.len() {
				fromScope.Err(errors.New("invalid foreign keys, should have same length"))
				return
			}
		}

		for idx, foreignKey := range foreignKeys {
			if foreignField, ok := fromModelStruct.FieldByName(foreignKey); ok {
				if associationField, ok := toModelStruct.FieldByName(associationForeignKeys[idx]); ok {
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
