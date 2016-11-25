package gorm

import (
	"errors"
	"fmt"
	"strings"
)

const (
	poly_field_not_found_warn string = "rel : polymorphic field %q not found on model struct %q"
	fk_field_not_found_warn string = "rel [%q]: foreign key field %q not found on model struct %q"
	afk_field_not_found_warn string = "rel [%q]: association foreign key field %q not found on model struct %q"
	length_err               string = "rel [%q]: invalid foreign keys, should have same length"
	poly_type                string = "Type"
)

func (r *Relationship) Poly(field *StructField, toModel *ModelStruct, fromScope, toScope *Scope) string {
	modelName := ""
	if field.HasSetting(POLYMORPHIC) {
		polyName := field.GetSetting(POLYMORPHIC)
		polyValue := field.GetSetting(POLYMORPHIC_VALUE)
		polyFieldName := polyName + poly_type
		// Dog has many toys, tag polymorphic is Owner, then associationType is Owner
		// Toy use OwnerID, OwnerType ('dogs') as foreign key
		if polymorphicType, ok := toModel.FieldByName(polyFieldName); ok {
			modelName = polyName
			r.PolymorphicType = polymorphicType.GetName()
			r.PolymorphicDBName = polymorphicType.DBName
			// if Dog has multiple set of toys set name of the set (instead of default 'dogs')
			if polyValue != "" {
				r.PolymorphicValue = polyValue
			} else {
				r.PolymorphicValue = fromScope.TableName()
			}
			polymorphicType.SetIsForeignKey()
		} else {
			errMsg := fmt.Sprintf(poly_field_not_found_warn, polyFieldName, toModel.ModelType.Name())
			toScope.Warn(errMsg)
		}
	}
	return modelName
}

//creates a many to many relationship, even if we don't have tags
func (r *Relationship) ManyToMany(field *StructField,
	fromModel *ModelStruct,
	fromScope, toScope *Scope) {

	var foreignKeys, associationForeignKeys StrSlice

	r.Kind = MANY_TO_MANY
	elemType := field.Type
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
			r.ForeignFieldNames.add(fkField.DBName)
			// join table foreign keys for source
			r.ForeignDBNames.add(modelName + "_" + fkField.DBName)
		} else {
			errMsg := fmt.Sprintf(fk_field_not_found_warn, "Many2Many", fk, fromModel.ModelType.Name())
			toScope.Warn(errMsg)
		}
	}

	for _, fk := range associationForeignKeys {
		if fkField, ok := toScope.FieldByName(fk); ok {
			// association foreign keys (db names)
			r.AssociationForeignFieldNames.add(fkField.DBName)
			// join table foreign keys for association
			r.AssociationForeignDBNames.add(elemName + "_" + fkField.DBName)
		} else {
			errMsg := fmt.Sprintf(afk_field_not_found_warn, "Many2Many", fk, toScope.GetModelStruct().ModelType.Name())
			toScope.Warn(errMsg)
		}
	}

	joinTableHandler := JoinTableHandler{TableName: referencedTable}
	joinTableHandler.Setup(r, modelType, elemType)
	r.JoinTableHandler = &joinTableHandler
	field.Relationship = r
}

func (r *Relationship) HasMany(field *StructField,
	fromModel, toModel *ModelStruct,
	fromScope, toScope *Scope) {

	// User has many comments, associationType is User, comment use UserID as foreign key
	modelName := NamesMap.ToDBName(fromModel.ModelType.Name())
	r.Kind = HAS_MANY

	if polyName := r.Poly(field, toModel, fromScope, toScope); polyName != "" {
		modelName = polyName
	}

	foreignKeys, associationForeignKeys := r.collectFKsAndAFKs(field, fromModel, fromScope, modelName)

	for idx, foreignKey := range foreignKeys {
		if foreignField, ok := toModel.FieldByName(foreignKey); ok {
			if associationField, ok := fromModel.FieldByName(associationForeignKeys[idx]); ok {
				// source foreign keys
				foreignField.SetIsForeignKey()
				r.AssociationForeignFieldNames.add(associationField.GetName())
				r.AssociationForeignDBNames.add(associationField.DBName)

				// association foreign keys
				r.ForeignFieldNames.add(foreignField.GetName())
				r.ForeignDBNames.add(foreignField.DBName)
			} else {
				errMsg := fmt.Sprintf(afk_field_not_found_warn, "HasMany", associationForeignKeys[idx], fromModel.ModelType.Name())
				toScope.Warn(errMsg)
			}
		} else {
			errMsg := fmt.Sprintf(fk_field_not_found_warn, "HasMany", foreignKey, fromModel.ModelType.Name())
			toScope.Warn(errMsg)
		}
	}

	if r.ForeignFieldNames.len() != 0 {
		field.Relationship = r
	}
}

func (r *Relationship) HasOne(field *StructField,
	fromModel, toModel *ModelStruct,
	fromScope, toScope *Scope) bool {

	modelName := NamesMap.ToDBName(fromModel.ModelType.Name())

	if polyName := r.Poly(field, toModel, fromScope, toScope); polyName != "" {
		modelName = polyName
	}

	foreignKeys, associationForeignKeys := r.collectFKsAndAFKs(field, fromModel, fromScope, modelName)

	for idx, foreignKey := range foreignKeys {
		if foreignField, ok := toModel.FieldByName(foreignKey); ok {
			if scopeField, ok := fromModel.FieldByName(associationForeignKeys[idx]); ok {
				foreignField.SetIsForeignKey()
				// source foreign keys
				r.AssociationForeignFieldNames.add(scopeField.GetName())
				r.AssociationForeignDBNames.add(scopeField.DBName)

				// association foreign keys
				r.ForeignFieldNames.add(foreignField.GetName())
				r.ForeignDBNames.add(foreignField.DBName)
			} else {
				errMsg := fmt.Sprintf(afk_field_not_found_warn, "HasOne fromModel", associationForeignKeys[idx], fromModel.ModelType.Name())
				toScope.Warn(errMsg)
			}
		} else {
			errMsg := fmt.Sprintf(fk_field_not_found_warn, "HasOne toModel", foreignKey, fromModel.ModelType.Name())
			toScope.Warn(errMsg)
		}
	}

	if r.ForeignFieldNames.len() != 0 {
		r.Kind = HAS_ONE
		field.Relationship = r
		return true
	}
	return false
}

func (r *Relationship) BelongTo(field *StructField,
	fromModel, toModel *ModelStruct,
	fromScope, toScope *Scope) bool {

	foreignKeys, associationForeignKeys := r.collectFKsAndAFKs(field, toModel, fromScope, "")

	for idx, foreignKey := range foreignKeys {
		if foreignField, ok := fromModel.FieldByName(foreignKey); ok {
			if associationField, ok := toModel.FieldByName(associationForeignKeys[idx]); ok {
				foreignField.SetIsForeignKey()

				// association foreign keys
				r.AssociationForeignFieldNames.add(associationField.GetName())
				r.AssociationForeignDBNames.add(associationField.DBName)

				// source foreign keys
				r.ForeignFieldNames.add(foreignField.GetName())
				r.ForeignDBNames.add(foreignField.DBName)
			} else {
				errMsg := fmt.Sprintf(afk_field_not_found_warn, "BelongTo", associationForeignKeys[idx], fromModel.ModelType.Name())
				toScope.Warn(errMsg)
			}
		} else {
			errMsg := fmt.Sprintf(fk_field_not_found_warn, "BelongTo", foreignKey, fromModel.ModelType.Name())
			toScope.Warn(errMsg)
		}
	}

	if r.ForeignFieldNames.len() != 0 {
		r.Kind = BELONGS_TO
		field.Relationship = r
		return true
	}
	return false
}

func (r *Relationship) collectFKsAndAFKs(field *StructField,
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
					errMsg := fmt.Sprintf(afk_field_not_found_warn, "collectFKsAndAFKs", fkField, model.ModelType.Name())
					scope.Warn(errMsg)
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
						errMsg := fmt.Sprintf(fk_field_not_found_warn, "collectFKsAndAFKs", afk, model.ModelType.Name())
						scope.Warn(errMsg)
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
