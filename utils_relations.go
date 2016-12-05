package gorm

import (
	"errors"
	"fmt"
	"strings"
)

const (
	poly_field_not_found_warn string = "\nrel : polymorphic field %q not found on model struct %q"
	fk_field_not_found_warn   string = "\nrel [%q]: foreign key field %q not found on model struct %q pointed by %q [%q]"
	afk_field_not_found_warn  string = "\nrel [%q]: association foreign key field %q not found on model struct %q"
	length_err                string = "rel [%q]: invalid foreign keys, should have same length"
	poly_type                 string = "Type"
	has_no_foreign_key        string = "\nrel [%q]: field has no foreign key setting"
	has_no_association_key    string = "\nrel [%q]: field has no association key setting"
)

func makePoly(field *StructField, toModel *ModelStruct, fromScope *Scope) string {
	modelName := ""
	if field.HasSetting(POLYMORPHIC) {
		polyName := field.GetStrSetting(POLYMORPHIC)
		polyFieldName := polyName + poly_type
		// Dog has many toys, tag polymorphic is Owner, then associationType is Owner
		// Toy use OwnerID, OwnerType ('dogs') as foreign key
		if polyField, ok := toModel.FieldByName(polyFieldName); ok {
			modelName = polyName
			polyField.LinkPoly(field, fromScope.TableName())
		} else {
			errMsg := fmt.Sprintf(poly_field_not_found_warn, polyFieldName, toModel.ModelType.Name())
			fromScope.Warn(errMsg)
		}
	}
	return modelName
}

//creates a many to many relationship, even if we don't have tags
func makeManyToMany(field *StructField,
	fromModel *ModelStruct,
	fromScope, toScope *Scope) {

	var foreignKeys, associationForeignKeys StrSlice

	//rel.Kind = MANY_TO_MANY
	elemType := field.Type
	elemName := NamesMap.ToDBName(elemType.Name())
	modelType := fromModel.ModelType
	modelName := NamesMap.ToDBName(modelType.Name())
	//many to many is set (check is in ModelStruct)
	referencedTable := field.GetStrSetting(MANY2MANY_NAME)

	if !field.HasSetting(FOREIGNKEY) {
		// if no foreign keys defined with tag, we add the primary keys
		for _, pk := range fromModel.PKs() {
			foreignKeys.add(pk.DBName)
		}
	} else {
		if field.HasSetting(FOREIGNKEY) {
			foreignKeys = field.GetSliceSetting(FOREIGNKEY)
		} else {
			errMsg := fmt.Sprintf(has_no_foreign_key, "Many2Many")
			fromScope.Warn(errMsg)
		}
	}

	if !field.HasSetting(ASSOCIATIONFOREIGNKEY) {
		// if no association foreign keys defined with tag, we add the primary keys
		for _, pk := range toScope.PKs() {
			associationForeignKeys.add(pk.DBName)
		}
	} else {
		if field.HasSetting(ASSOCIATIONFOREIGNKEY) {
			associationForeignKeys = field.GetSliceSetting(ASSOCIATIONFOREIGNKEY)
		} else {
			errMsg := fmt.Sprintf(has_no_association_key, "Many2Many")
			fromScope.Warn(errMsg)
		}
	}

	var (
		ForeignFieldNames            StrSlice
		ForeignDBNames               StrSlice
		AssociationForeignFieldNames StrSlice
		AssociationForeignDBNames    StrSlice
	)

	for _, fk := range foreignKeys {
		if fkField, ok := fromModel.FieldByName(fk); ok {
			// source foreign keys (db names)
			ForeignFieldNames.add(fkField.DBName)
			// join table foreign keys for source
			ForeignDBNames.add(modelName + "_" + fkField.DBName)
		} else {
			errMsg := fmt.Sprintf(fk_field_not_found_warn, "Many2Many", fk, fromModel.ModelType.Name(), field.StructName, toScope.GetModelStruct().ModelType.Name())
			toScope.Warn(errMsg)
		}
	}

	for _, fk := range associationForeignKeys {
		if fkField, ok := toScope.FieldByName(fk); ok {
			// association foreign keys (db names)
			AssociationForeignFieldNames.add(fkField.DBName)
			// join table foreign keys for association
			AssociationForeignDBNames.add(elemName + "_" + fkField.DBName)
		} else {
			errMsg := fmt.Sprintf(afk_field_not_found_warn, "Many2Many", fk, toScope.GetModelStruct().ModelType.Name())
			toScope.Warn(errMsg)
		}
	}

	field.SetTagSetting(FOREIGN_FIELD_NAMES, ForeignFieldNames)
	field.SetTagSetting(FOREIGN_DB_NAMES, ForeignDBNames)
	field.SetTagSetting(ASSOCIATION_FOREIGN_FIELD_NAMES, AssociationForeignFieldNames)
	field.SetTagSetting(ASSOCIATION_FOREIGN_DB_NAMES, AssociationForeignDBNames)

	//we've finished with this information - removing for allocation sake
	field.UnsetTagSetting(ASSOCIATIONFOREIGNKEY)
	field.UnsetTagSetting(FOREIGNKEY)

	field.SetHasRelations()

	joinTableHandler := JoinTableHandler{TableName: referencedTable}
	joinTableHandler.Setup(field, modelType, elemType)
	field.SetTagSetting(JOIN_TABLE_HANDLER, &joinTableHandler)
}

func makeHasMany(field *StructField,
	fromModel, toModel *ModelStruct,
	fromScope, toScope *Scope) {

	// User has many comments, associationType is User, comment use UserID as foreign key
	modelName := NamesMap.ToDBName(fromModel.ModelType.Name())
	//r.Kind = HAS_MANY

	if polyName := makePoly(field, toModel, fromScope); polyName != "" {
		modelName = polyName
	}

	foreignKeys, associationForeignKeys := collectFKsAndAFKs(field, fromModel, fromScope, modelName)

	var (
		ForeignFieldNames            StrSlice
		ForeignDBNames               StrSlice
		AssociationForeignFieldNames StrSlice
		AssociationForeignDBNames    StrSlice
	)

	for idx, foreignKey := range foreignKeys {
		if foreignField, ok := toModel.FieldByName(foreignKey); ok {
			if associationField, ok := fromModel.FieldByName(associationForeignKeys[idx]); ok {
				// source foreign keys
				foreignField.SetIsForeignKey()
				AssociationForeignFieldNames.add(associationField.StructName)
				AssociationForeignDBNames.add(associationField.DBName)

				// association foreign keys
				ForeignFieldNames.add(foreignField.StructName)
				ForeignDBNames.add(foreignField.DBName)
			} else {
				errMsg := fmt.Sprintf(afk_field_not_found_warn, "HasMany", associationForeignKeys[idx], fromModel.ModelType.Name())
				toScope.Warn(errMsg)
			}
		} else {
			errMsg := fmt.Sprintf(fk_field_not_found_warn, "HasMany", foreignKey, fromModel.ModelType.Name(), field.StructName, toModel.ModelType.Name())
			toScope.Warn(errMsg)
		}
	}

	if ForeignFieldNames.len() != 0 {
		field.SetTagSetting(RELATION_KIND, HAS_MANY)
		field.SetTagSetting(FOREIGN_FIELD_NAMES, ForeignFieldNames)
		field.SetTagSetting(FOREIGN_DB_NAMES, ForeignDBNames)
		field.SetTagSetting(ASSOCIATION_FOREIGN_FIELD_NAMES, AssociationForeignFieldNames)
		field.SetTagSetting(ASSOCIATION_FOREIGN_DB_NAMES, AssociationForeignDBNames)
		//we've finished with this information - removing for allocation sake
		field.UnsetTagSetting(ASSOCIATIONFOREIGNKEY)
		field.UnsetTagSetting(FOREIGNKEY)
		field.SetHasRelations()
	}
}

func makeHasOne(field *StructField,
	fromModel, toModel *ModelStruct,
	fromScope, toScope *Scope) bool {

	modelName := NamesMap.ToDBName(fromModel.ModelType.Name())

	if polyName := makePoly(field, toModel, fromScope); polyName != "" {
		modelName = polyName
	}

	foreignKeys, associationForeignKeys := collectFKsAndAFKs(field, fromModel, fromScope, modelName)

	var (
		ForeignFieldNames            StrSlice
		ForeignDBNames               StrSlice
		AssociationForeignFieldNames StrSlice
		AssociationForeignDBNames    StrSlice
	)

	for idx, foreignKey := range foreignKeys {
		if foreignField, ok := toModel.FieldByName(foreignKey); ok {
			if assocField, ok := fromModel.FieldByName(associationForeignKeys[idx]); ok {
				foreignField.SetIsForeignKey()
				// source foreign keys
				AssociationForeignFieldNames.add(assocField.StructName)
				AssociationForeignDBNames.add(assocField.DBName)

				// association foreign keys
				ForeignFieldNames.add(foreignField.StructName)
				ForeignDBNames.add(foreignField.DBName)
			} else {
				errMsg := fmt.Sprintf(afk_field_not_found_warn, "HasOne[1]", associationForeignKeys[idx], fromModel.ModelType.Name())
				toScope.Warn(errMsg)
			}
		} else {
			errMsg := fmt.Sprintf(fk_field_not_found_warn, "HasOne[2]", foreignKey, toModel.ModelType.Name(), field.StructName, toModel.ModelType.Name())
			toScope.Warn(errMsg)
		}
	}

	if ForeignFieldNames.len() != 0 {
		field.SetTagSetting(RELATION_KIND, HAS_ONE)
		field.SetTagSetting(FOREIGN_FIELD_NAMES, ForeignFieldNames)
		field.SetTagSetting(FOREIGN_DB_NAMES, ForeignDBNames)
		field.SetTagSetting(ASSOCIATION_FOREIGN_FIELD_NAMES, AssociationForeignFieldNames)
		field.SetTagSetting(ASSOCIATION_FOREIGN_DB_NAMES, AssociationForeignDBNames)
		//we've finished with this information - removing for allocation sake
		field.UnsetTagSetting(ASSOCIATIONFOREIGNKEY)
		field.UnsetTagSetting(FOREIGNKEY)
		field.SetHasRelations()
		return true
	}
	return false
}

func makeBelongTo(field *StructField,
	fromModel, toModel *ModelStruct,
	fromScope, toScope *Scope) bool {

	foreignKeys, associationForeignKeys := collectFKsAndAFKs(field, toModel, fromScope, "")

	var (
		ForeignFieldNames            StrSlice
		ForeignDBNames               StrSlice
		AssociationForeignFieldNames StrSlice
		AssociationForeignDBNames    StrSlice
	)

	for idx, foreignKey := range foreignKeys {
		if foreignField, ok := fromModel.FieldByName(foreignKey); ok {
			if associationField, ok := toModel.FieldByName(associationForeignKeys[idx]); ok {
				foreignField.SetIsForeignKey()

				// association foreign keys
				AssociationForeignFieldNames.add(associationField.StructName)
				AssociationForeignDBNames.add(associationField.DBName)

				// source foreign keys
				ForeignFieldNames.add(foreignField.StructName)
				ForeignDBNames.add(foreignField.DBName)
			} else {
				errMsg := fmt.Sprintf(afk_field_not_found_warn, "BelongTo", associationForeignKeys[idx], fromModel.ModelType.Name())
				toScope.Warn(errMsg)
			}
		} else {
			errMsg := fmt.Sprintf(fk_field_not_found_warn, "BelongTo", foreignKey, toModel.ModelType.Name(), field.StructName, fromModel.ModelType.Name())
			toScope.Warn(errMsg)
		}
	}

	if ForeignFieldNames.len() != 0 {
		field.SetTagSetting(RELATION_KIND, BELONGS_TO)
		field.SetTagSetting(FOREIGN_FIELD_NAMES, ForeignFieldNames)
		field.SetTagSetting(FOREIGN_DB_NAMES, ForeignDBNames)
		field.SetTagSetting(ASSOCIATION_FOREIGN_FIELD_NAMES, AssociationForeignFieldNames)
		field.SetTagSetting(ASSOCIATION_FOREIGN_DB_NAMES, AssociationForeignDBNames)
		//we've finished with this information - removing for allocation sake
		field.UnsetTagSetting(ASSOCIATIONFOREIGNKEY)
		field.UnsetTagSetting(FOREIGNKEY)
		field.SetHasRelations()
		return true
	}
	return false
}

func collectFKsAndAFKs(field *StructField,
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
					foreignKeys.add(field.StructName + pk.StructName)
				} else {
					foreignKeys.add(modelName + pk.StructName)
				}
				associationForeignKeys.add(pk.StructName)
			}
		} else {
			if field.HasSetting(ASSOCIATIONFOREIGNKEY) {
				associationForeignKeys = field.GetSliceSetting(ASSOCIATIONFOREIGNKEY)
			} else {
				errMsg := fmt.Sprintf(has_no_association_key, "collectFKsAndAFKs")
				scope.Warn(errMsg)
			}
			// generate foreign keys from defined association foreign keys
			for _, afk := range associationForeignKeys {
				if fkField, ok := model.FieldByName(afk); ok {
					if modelName == "" {
						foreignKeys.add(field.StructName + fkField.StructName)
					} else {
						foreignKeys.add(modelName + fkField.StructName)
					}
					associationForeignKeys.add(fkField.StructName)
				} else {
					errMsg := fmt.Sprintf(afk_field_not_found_warn, "collectFKsAndAFKs", fkField, model.ModelType.Name())
					scope.Warn(errMsg)
				}
			}
		}
	} else {
		if field.HasSetting(FOREIGNKEY) {
			foreignKeys = field.GetSliceSetting(FOREIGNKEY)
		} else {
			errMsg := fmt.Sprintf(has_no_foreign_key, "collectFKsAndAFKs")
			scope.Warn(errMsg)
		}
		// generate association foreign keys from foreign keys
		if !field.HasSetting(ASSOCIATIONFOREIGNKEY) {
			for _, fk := range foreignKeys {
				prefix := modelName
				if modelName == "" {
					prefix = field.StructName
				}
				if strings.HasPrefix(fk, prefix) {
					afk := strings.TrimPrefix(fk, prefix)
					if _, ok := model.FieldByName(afk); ok {
						associationForeignKeys.add(afk)
					} else {
						errMsg := fmt.Sprintf(fk_field_not_found_warn, "collectFKsAndAFKs", afk, model.ModelType.Name(), field.StructName, modelName)
						scope.Warn(errMsg)
					}
				}
			}
			if associationForeignKeys.len() == 0 && foreignKeys.len() == 1 {
				associationForeignKeys = StrSlice{scope.PKName()}
			}
		} else {
			if field.HasSetting(ASSOCIATIONFOREIGNKEY) {
				associationForeignKeys = field.GetSliceSetting(ASSOCIATIONFOREIGNKEY)
			} else {
				errMsg := fmt.Sprintf(has_no_association_key, "collectFKsAndAFKs")
				scope.Warn(errMsg)
			}
			if foreignKeys.len() != associationForeignKeys.len() {
				scope.Err(errors.New(length_err))
				return nil, nil
			}
		}
	}
	return foreignKeys, associationForeignKeys
}
