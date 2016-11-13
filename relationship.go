package gorm

import (
	"errors"
	"fmt"
	"strings"
)

const (
	poly_field_not_found_err string = "Relationship : polymorphic field %q not found on model struct %q"
	fk_field_not_found_err   string = "Relationship : foreign key field %q not found on model struct %q"
	afk_field_not_found_err  string = "Relationship : association foreign key field %q not found on model struct %q"
	length_err               string = "Relationship : invalid foreign keys, should have same length"
	poly_type                string = "Type"
)

func (relationship *Relationship) Poly(field *StructField, toModel *ModelStruct, fromScope, toScope *Scope) string {
	modelName := ""
	if field.HasSetting(POLYMORPHIC) {
		polyName := field.GetSetting(POLYMORPHIC)
		polyValue := field.GetSetting(POLYMORPHIC_VALUE)
		polyFieldName := polyName + poly_type
		// Dog has many toys, tag polymorphic is Owner, then associationType is Owner
		// Toy use OwnerID, OwnerType ('dogs') as foreign key
		if polymorphicType, ok := toModel.FieldByName(polyFieldName); ok {
			modelName = polyName
			relationship.PolymorphicType = polymorphicType.GetName()
			relationship.PolymorphicDBName = polymorphicType.DBName
			// if Dog has multiple set of toys set name of the set (instead of default 'dogs')
			if polyValue != "" {
				relationship.PolymorphicValue = polyValue
			} else {
				relationship.PolymorphicValue = fromScope.TableName()
			}
			polymorphicType.SetIsForeignKey()
		} else {
			errMsg := fmt.Sprintf(poly_field_not_found_err, polyFieldName, toModel.ModelType.Name())
			toScope.Log(errMsg)
			//TODO : @Badu - activate below
			//fromScope.Err(errors.New(errMsg))
			//return ""
		}
	}
	return modelName
}

//creates a many to many relationship, even if we don't have tags
func (relationship *Relationship) ManyToMany(field *StructField,
	fromModel *ModelStruct,
	fromScope, toScope *Scope) {

	var foreignKeys, associationForeignKeys StrSlice

	relationship.Kind = MANY_TO_MANY
	elemType := field.getTrueType()
	elemName := NamesMap.ToDBName(elemType.Name())
	modelType := fromModel.ModelType
	modelName := NamesMap.ToDBName(modelType.Name())
	//many to many is set (check is in ModelStruct)
	referencedTable := field.GetSetting(MANY2MANY)

	if !field.HasSetting(FOREIGNKEY) {
		// if no foreign keys defined with tag, we add the primary keys
		for _, pk := range fromModel.PKs() {
			foreignKeys.add(pk.DBName)
		}
	} else {
		foreignKeys = field.getForeignKeys()
	}

	if !field.HasSetting(ASSOCIATIONFOREIGNKEY) {
		// if no association foreign keys defined with tag, we add the primary keys
		for _, pk := range toScope.PKs() {
			associationForeignKeys.add(pk.DBName)
		}
	} else {
		associationForeignKeys = field.getAssocForeignKeys()
	}

	for _, fk := range foreignKeys {
		if fkField, ok := fromModel.FieldByName(fk); ok {
			// source foreign keys (db names)
			relationship.ForeignFieldNames.add(fkField.DBName)
			// join table foreign keys for source
			relationship.ForeignDBNames.add(modelName + "_" + fkField.DBName)
		} else {
			//TODO : @Badu - activate below
			errMsg := fmt.Sprintf(fk_field_not_found_err, fk, fromModel.ModelType.Name())
			toScope.Log(errMsg)
			//fromScope.Err(errors.New(errMsg))
			//return
		}
	}

	for _, fk := range associationForeignKeys {
		if fkField, ok := toScope.FieldByName(fk); ok {
			// association foreign keys (db names)
			relationship.AssociationForeignFieldNames.add(fkField.DBName)
			// join table foreign keys for association
			relationship.AssociationForeignDBNames.add(elemName + "_" + fkField.DBName)
		} else {
			errMsg := fmt.Sprintf(afk_field_not_found_err, fk, toScope.GetModelStruct().ModelType.Name())
			toScope.Log(errMsg)
			//TODO : @Badu - activate below
			//fromScope.Err(errors.New(errMsg))
			//return
		}
	}

	joinTableHandler := JoinTableHandler{TableName: referencedTable}
	joinTableHandler.Setup(relationship, modelType, elemType)
	relationship.JoinTableHandler = &joinTableHandler
	field.Relationship = relationship
}

func (relationship *Relationship) HasMany(field *StructField,
	fromModel, toModel *ModelStruct,
	fromScope, toScope *Scope) {

	// User has many comments, associationType is User, comment use UserID as foreign key
	modelName := NamesMap.ToDBName(fromModel.ModelType.Name())
	relationship.Kind = HAS_MANY

	if polyName := relationship.Poly(field, toModel, fromScope, toScope); polyName != "" {
		modelName = polyName
	}

	foreignKeys, associationForeignKeys := relationship.collectFKsAndAFKs(field, fromModel, fromScope, modelName)

	for idx, foreignKey := range foreignKeys {
		if foreignField, ok := toModel.FieldByName(foreignKey); ok {
			if associationField, ok := fromModel.FieldByName(associationForeignKeys[idx]); ok {
				// source foreign keys
				foreignField.SetIsForeignKey()
				relationship.AssociationForeignFieldNames.add(associationField.GetName())
				relationship.AssociationForeignDBNames.add(associationField.DBName)

				// association foreign keys
				relationship.ForeignFieldNames.add(foreignField.GetName())
				relationship.ForeignDBNames.add(foreignField.DBName)
			} else {
				errMsg := fmt.Sprintf(afk_field_not_found_err, associationForeignKeys[idx], fromModel.ModelType.Name())
				toScope.Log(errMsg)
				//TODO : @Badu - activate below
				//fromScope.Err(errors.New(errMsg))
				//return
			}
		} else {
			errMsg := fmt.Sprintf(fk_field_not_found_err, foreignKey, fromModel.ModelType.Name())
			toScope.Log(errMsg)
			//TODO : @Badu - activate below
			//fromScope.Err(errors.New(errMsg))
			//return
		}
	}

	if relationship.ForeignFieldNames.len() != 0 {
		field.Relationship = relationship
	}
}

func (relationship *Relationship) HasOne(field *StructField,
	fromModel, toModel *ModelStruct,
	fromScope, toScope *Scope) bool {

	modelName := NamesMap.ToDBName(fromModel.ModelType.Name())

	if polyName := relationship.Poly(field, toModel, fromScope, toScope); polyName != "" {
		modelName = polyName
	}

	foreignKeys, associationForeignKeys := relationship.collectFKsAndAFKs(field, fromModel, fromScope, modelName)

	for idx, foreignKey := range foreignKeys {
		if foreignField, ok := toModel.FieldByName(foreignKey); ok {
			if scopeField, ok := fromModel.FieldByName(associationForeignKeys[idx]); ok {
				foreignField.SetIsForeignKey()
				// source foreign keys
				relationship.AssociationForeignFieldNames.add(scopeField.GetName())
				relationship.AssociationForeignDBNames.add(scopeField.DBName)

				// association foreign keys
				relationship.ForeignFieldNames.add(foreignField.GetName())
				relationship.ForeignDBNames.add(foreignField.DBName)
			} else {
				errMsg := fmt.Sprintf(afk_field_not_found_err, associationForeignKeys[idx], fromModel.ModelType.Name())
				toScope.Log(errMsg)
				//TODO : @Badu - activate below
				//fromScope.Err(errors.New(errMsg))
				//return true
			}
		} else {
			errMsg := fmt.Sprintf(fk_field_not_found_err, foreignKey, fromModel.ModelType.Name())
			toScope.Log(errMsg)
			//TODO : @Badu - activate below
			//fromScope.Err(errors.New(errMsg))
			//return true
		}
	}

	if relationship.ForeignFieldNames.len() != 0 {
		relationship.Kind = HAS_ONE
		field.Relationship = relationship
		return true
	}
	return false
}

func (relationship *Relationship) BelongTo(field *StructField,
	fromModel, toModel *ModelStruct,
	fromScope, toScope *Scope) bool {

	foreignKeys, associationForeignKeys := relationship.collectFKsAndAFKs(field, toModel, fromScope, "")

	for idx, foreignKey := range foreignKeys {
		if foreignField, ok := fromModel.FieldByName(foreignKey); ok {
			if associationField, ok := toModel.FieldByName(associationForeignKeys[idx]); ok {
				foreignField.SetIsForeignKey()

				// association foreign keys
				relationship.AssociationForeignFieldNames.add(associationField.GetName())
				relationship.AssociationForeignDBNames.add(associationField.DBName)

				// source foreign keys
				relationship.ForeignFieldNames.add(foreignField.GetName())
				relationship.ForeignDBNames.add(foreignField.DBName)
			} else {
				errMsg := fmt.Sprintf(afk_field_not_found_err, associationForeignKeys[idx], fromModel.ModelType.Name())
				toScope.Log(errMsg)
				//TODO : @Badu - activate below
				//fromScope.Err(errors.New(errMsg))
				//return true
			}
		} else {
			errMsg := fmt.Sprintf(fk_field_not_found_err, foreignKey, fromModel.ModelType.Name())
			toScope.Log(errMsg)
			//TODO : @Badu - activate below
			//fromScope.Err(errors.New(errMsg))
			//return true
		}
	}

	if relationship.ForeignFieldNames.len() != 0 {
		relationship.Kind = BELONGS_TO
		field.Relationship = relationship
		return true
	}
	return false
}

func (relationship *Relationship) collectFKsAndAFKs(field *StructField,
	model *ModelStruct,
	scope *Scope,
	modelName string) (StrSlice, StrSlice) {

	var foreignKeys, associationForeignKeys StrSlice

	// if no foreign keys defined with tag
	if !field.HasSetting(FOREIGNKEY) {
		// if no association foreign keys defined with tag
		if !field.HasSetting(ASSOCIATIONFOREIGNKEY) {
			for _, pk := range model.PKs() {
				if modelName == "" {
					foreignKeys.add(field.GetName() + pk.GetName())
				} else {
					foreignKeys.add(modelName + pk.GetName())
				}
				associationForeignKeys.add(pk.GetName())
			}
		} else {
			associationForeignKeys = field.getAssocForeignKeys()
			// generate foreign keys from defined association foreign keys
			for _, afk := range associationForeignKeys {
				if fkField, ok := model.FieldByName(afk); ok {
					if modelName == "" {
						foreignKeys.add(field.GetName() + fkField.GetName())
					} else {
						foreignKeys.add(modelName + fkField.GetName())
					}
					associationForeignKeys.add(fkField.GetName())
				} else {
					errMsg := fmt.Sprintf(afk_field_not_found_err, fkField, model.ModelType.Name())
					scope.Log(errMsg)
					//TODO : @Badu - activate below
					//fromScope.Err(errors.New(errMsg))
					//return nil, nil
				}
			}
		}
	} else {
		foreignKeys = field.getForeignKeys()
		// generate association foreign keys from foreign keys
		if !field.HasSetting(ASSOCIATIONFOREIGNKEY) {
			for _, fk := range foreignKeys {
				prefix := modelName
				if modelName == "" {
					prefix = field.GetName()
				}
				if strings.HasPrefix(fk, prefix) {
					afk := strings.TrimPrefix(fk, prefix)
					if _, ok := model.FieldByName(afk); ok {
						associationForeignKeys.add(afk)
					} else {
						errMsg := fmt.Sprintf(fk_field_not_found_err, afk, model.ModelType.Name())
						scope.Log(errMsg)
						//TODO : @Badu - activate below
						//fromScope.Err(errors.New(errMsg))
						//return nil, nil
					}
				}
			}
			if associationForeignKeys.len() == 0 && foreignKeys.len() == 1 {
				associationForeignKeys = StrSlice{scope.PKName()}
			}
		} else {
			associationForeignKeys = field.getAssocForeignKeys()
			if foreignKeys.len() != associationForeignKeys.len() {
				scope.Err(errors.New(length_err))
				return nil, nil
			}
		}
	}
	return foreignKeys, associationForeignKeys
}
