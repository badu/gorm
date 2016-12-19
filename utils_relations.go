package gorm

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

func makePoly(field *StructField, toModel *ModelStruct, fromScope *Scope) string {
	modelName := ""
	if field.HasSetting(set_polymorphic) {
		polyName := field.GetStrSetting(set_polymorphic)
		polyFieldName := polyName + field_poly_type
		// Dog has many toys, tag polymorphic is Owner, then associationType is Owner
		// Toy use OwnerID, OwnerType ('dogs') as foreign key
		if polyField, ok := toModel.FieldByName(polyFieldName, fromScope.con.parent); ok {
			modelName = polyName
			polyField.LinkPoly(field, fromScope.TableName())
		} else {
			fromScope.Warn(
				fmt.Errorf(
					warn_poly_field_not_found,
					polyFieldName,
					toModel.ModelType.Name(),
				),
			)
		}
	}
	return modelName
}

//creates a many to many relationship, even if we don't have tags
func makeManyToMany(field *StructField,
	fromModel *ModelStruct,
	fromScope, toScope *Scope) {

	var (
		foreignKeys, associationForeignKeys StrSlice
		elemType                            = field.Type
		elemName                            = fromScope.con.parent.namesMap.toDBName(elemType.Name())
		modelType                           = fromModel.ModelType
		modelName                           = fromScope.con.parent.namesMap.toDBName(modelType.Name())
		referencedTable                     = field.GetStrSetting(set_many2many_name) //many to many is set (check is in ModelStruct)
	)

	if !field.HasSetting(set_foreignkey) {
		// if no foreign keys defined with tag, we add the primary keys
		for _, pk := range fromModel.PKs() {
			foreignKeys.add(pk.DBName)
		}
	} else {
		if field.HasSetting(set_foreignkey) {
			foreignKeys = field.GetFKs()
		} else {
			fromScope.Warn(fmt.Errorf(warn_has_no_foreign_key, tag_many_to_many))
		}
	}

	if !field.HasSetting(set_associationforeignkey) {
		// if no association foreign keys defined with tag, we add the primary keys
		for _, pk := range toScope.PKs() {
			associationForeignKeys.add(pk.DBName)
		}
	} else {
		if field.HasSetting(set_associationforeignkey) {
			associationForeignKeys = field.GetAssocFKs()
		} else {
			fromScope.Warn(fmt.Errorf(warn_has_no_association_key, tag_many_to_many))
		}
	}

	var (
		ForeignFieldNames            StrSlice
		ForeignDBNames               StrSlice
		AssociationForeignFieldNames StrSlice
		AssociationForeignDBNames    StrSlice
	)

	for _, fk := range foreignKeys {
		if fkField, ok := fromModel.FieldByName(fk, fromScope.con.parent); ok {
			// source foreign keys (db names)
			ForeignFieldNames.add(fkField.DBName)
			// join table foreign keys for source
			ForeignDBNames.add(modelName + "_" + fkField.DBName)
		} else {
			toScope.Warn(
				fmt.Errorf(
					warn_fk_field_not_found,
					tag_many_to_many,
					fk,
					fromModel.ModelType.Name(),
					field.StructName,
					toScope.GetModelStruct().ModelType.Name(),
				),
			)
		}
	}

	for _, fk := range associationForeignKeys {
		if fkField, ok := toScope.FieldByName(fk); ok {
			// association foreign keys (db names)
			AssociationForeignFieldNames.add(fkField.DBName)
			// join table foreign keys for association
			AssociationForeignDBNames.add(elemName + "_" + fkField.DBName)
		} else {
			toScope.Warn(
				fmt.Errorf(
					warn_afk_field_not_found,
					tag_many_to_many,
					fk,
					toScope.GetModelStruct().ModelType.Name(),
				),
			)
		}
	}

	field.SetTagSetting(set_foreign_field_names, ForeignFieldNames)
	field.SetTagSetting(set_foreign_db_names, ForeignDBNames)
	field.SetTagSetting(set_association_foreign_field_names, AssociationForeignFieldNames)
	field.SetTagSetting(set_association_foreign_db_names, AssociationForeignDBNames)

	//we've finished with this information - removing for allocation sake
	field.UnsetTagSetting(set_associationforeignkey)
	field.UnsetTagSetting(set_foreignkey)

	field.SetHasRelations()

	joinTableHandler := JoinTableHandler{TableName: referencedTable}
	joinTableHandler.Setup(field, modelType, elemType)
	field.SetTagSetting(set_join_table_handler, &joinTableHandler)
}

func makeHasMany(field *StructField,
	fromModel, toModel *ModelStruct,
	fromScope, toScope *Scope) {

	var (
		modelName = fromScope.con.parent.namesMap.toDBName(fromModel.ModelType.Name()) // User has many comments, associationType is User, comment use UserID as foreign key
	)
	//checking if we have poly, which alters modelName
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
		if foreignField, ok := toModel.FieldByName(foreignKey, fromScope.con.parent); ok {
			if associationField, ok := fromModel.FieldByName(associationForeignKeys[idx], fromScope.con.parent); ok {
				// source foreign keys
				foreignField.SetIsForeignKey()
				AssociationForeignFieldNames.add(associationField.StructName)
				AssociationForeignDBNames.add(associationField.DBName)

				// association foreign keys
				ForeignFieldNames.add(foreignField.StructName)
				ForeignDBNames.add(foreignField.DBName)
			} else {
				toScope.Warn(
					fmt.Errorf(
						warn_afk_field_not_found,
						str_hasmany,
						associationForeignKeys[idx],
						fromModel.ModelType.Name(),
					),
				)
			}
		} else {
			toScope.Warn(
				fmt.Errorf(
					warn_fk_field_not_found,
					str_hasmany,
					foreignKey,
					fromModel.ModelType.Name(),
					field.StructName,
					toModel.ModelType.Name(),
				),
			)
		}
	}

	if ForeignFieldNames.len() != 0 {
		field.SetTagSetting(set_relation_kind, rel_has_many)
		field.SetTagSetting(set_foreign_field_names, ForeignFieldNames)
		field.SetTagSetting(set_foreign_db_names, ForeignDBNames)
		field.SetTagSetting(set_association_foreign_field_names, AssociationForeignFieldNames)
		field.SetTagSetting(set_association_foreign_db_names, AssociationForeignDBNames)
		//we've finished with this information - removing for allocation sake
		field.UnsetTagSetting(set_associationforeignkey)
		field.UnsetTagSetting(set_foreignkey)
		field.SetHasRelations()
	}
}

func makeHasOne(field *StructField,
	fromModel, toModel *ModelStruct,
	fromScope, toScope *Scope) bool {

	var (
		modelName = fromScope.con.parent.namesMap.toDBName(fromModel.ModelType.Name())
	)
	//checking if we have poly, which alters modelName
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
		if foreignField, ok := toModel.FieldByName(foreignKey, fromScope.con.parent); ok {
			if assocField, ok := fromModel.FieldByName(associationForeignKeys[idx], fromScope.con.parent); ok {
				foreignField.SetIsForeignKey()
				// source foreign keys
				AssociationForeignFieldNames.add(assocField.StructName)
				AssociationForeignDBNames.add(assocField.DBName)

				// association foreign keys
				ForeignFieldNames.add(foreignField.StructName)
				ForeignDBNames.add(foreignField.DBName)
			} else {
				toScope.Warn(
					fmt.Errorf(
						warn_afk_field_not_found,
						str_hasone,
						associationForeignKeys[idx],
						fromModel.ModelType.Name(),
					),
				)
			}
		} else {
			toScope.Warn(
				fmt.Errorf(
					warn_fk_field_not_found,
					str_hasone,
					foreignKey,
					toModel.ModelType.Name(),
					field.StructName,
					toModel.ModelType.Name(),
				),
			)
		}
	}

	if ForeignFieldNames.len() != 0 {
		field.SetTagSetting(set_relation_kind, rel_has_one)
		field.SetTagSetting(set_foreign_field_names, ForeignFieldNames)
		field.SetTagSetting(set_foreign_db_names, ForeignDBNames)
		field.SetTagSetting(set_association_foreign_field_names, AssociationForeignFieldNames)
		field.SetTagSetting(set_association_foreign_db_names, AssociationForeignDBNames)
		//we've finished with this information - removing for allocation sake
		field.UnsetTagSetting(set_associationforeignkey)
		field.UnsetTagSetting(set_foreignkey)
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
		if foreignField, ok := fromModel.FieldByName(foreignKey, fromScope.con.parent); ok {
			if associationField, ok := toModel.FieldByName(associationForeignKeys[idx], fromScope.con.parent); ok {
				foreignField.SetIsForeignKey()

				// association foreign keys
				AssociationForeignFieldNames.add(associationField.StructName)
				AssociationForeignDBNames.add(associationField.DBName)

				// source foreign keys
				ForeignFieldNames.add(foreignField.StructName)
				ForeignDBNames.add(foreignField.DBName)
			} else {
				toScope.Warn(
					fmt.Errorf(
						warn_afk_field_not_found,
						str_belongsto,
						associationForeignKeys[idx],
						fromModel.ModelType.Name(),
					),
				)
			}
		} else {
			toScope.Warn(
				fmt.Errorf(
					warn_fk_field_not_found,
					str_belongsto,
					foreignKey,
					toModel.ModelType.Name(),
					field.StructName,
					fromModel.ModelType.Name(),
				),
			)
		}
	}

	if ForeignFieldNames.len() != 0 {
		field.SetTagSetting(set_relation_kind, rel_belongs_to)
		field.SetTagSetting(set_foreign_field_names, ForeignFieldNames)
		field.SetTagSetting(set_foreign_db_names, ForeignDBNames)
		field.SetTagSetting(set_association_foreign_field_names, AssociationForeignFieldNames)
		field.SetTagSetting(set_association_foreign_db_names, AssociationForeignDBNames)
		//we've finished with this information - removing for allocation sake
		field.UnsetTagSetting(set_associationforeignkey)
		field.UnsetTagSetting(set_foreignkey)
		field.SetHasRelations()
		return true
	}
	return false
}

func collectFKsAndAFKs(field *StructField,
	model *ModelStruct,
	scope *Scope,
	modelName string) (StrSlice, StrSlice) {

	var (
		foreignKeys, associationForeignKeys StrSlice
	)

	// if no foreign keys defined with tag
	if !field.HasSetting(set_foreignkey) {
		// if no association foreign keys defined with tag
		if !field.HasSetting(set_associationforeignkey) {
			for _, pk := range model.PKs() {
				if modelName == "" {
					foreignKeys.add(field.StructName + pk.StructName)
				} else {
					foreignKeys.add(modelName + pk.StructName)
				}
				associationForeignKeys.add(pk.StructName)
			}
		} else {
			if field.HasSetting(set_associationforeignkey) {
				associationForeignKeys = field.GetAssocFKs()
			} else {
				scope.Warn(fmt.Errorf(warn_has_no_association_key, str_collectfks))
			}
			// generate foreign keys from defined association foreign keys
			for _, afk := range associationForeignKeys {
				if fkField, ok := model.FieldByName(afk, scope.con.parent); ok {
					if modelName == "" {
						foreignKeys.add(field.StructName + fkField.StructName)
					} else {
						foreignKeys.add(modelName + fkField.StructName)
					}
					associationForeignKeys.add(fkField.StructName)
				} else {
					scope.Warn(
						fmt.Errorf(
							warn_afk_field_not_found,
							str_collectfks,
							fkField,
							model.ModelType.Name(),
						),
					)
				}
			}
		}
	} else {
		if field.HasSetting(set_foreignkey) {
			foreignKeys = field.GetFKs()
		} else {
			scope.Warn(fmt.Errorf(warn_has_no_foreign_key, str_collectfks))
		}
		// generate association foreign keys from foreign keys
		if !field.HasSetting(set_associationforeignkey) {
			for _, fk := range foreignKeys {
				prefix := modelName
				if modelName == "" {
					prefix = field.StructName
				}
				if strings.HasPrefix(fk, prefix) {
					afk := strings.TrimPrefix(fk, prefix)
					if _, ok := model.FieldByName(afk, scope.con.parent); ok {
						associationForeignKeys.add(afk)
					} else {
						scope.Warn(
							fmt.Errorf(
								warn_fk_field_not_found,
								str_collectfks,
								afk,
								model.ModelType.Name(),
								field.StructName,
								modelName,
							),
						)
					}
				}
			}
			if associationForeignKeys.len() == 0 && foreignKeys.len() == 1 {
				associationForeignKeys = StrSlice{scope.PKName()}
			}
		} else {
			if field.HasSetting(set_associationforeignkey) {
				associationForeignKeys = field.GetAssocFKs()
			} else {
				scope.Warn(fmt.Errorf(warn_has_no_association_key, str_collectfks))
			}
			if foreignKeys.len() != associationForeignKeys.len() {
				scope.Err(errors.New(err_fk_length_not_equal))
				return nil, nil
			}
		}
	}
	return foreignKeys, associationForeignKeys
}

// handleRelationPreload to preload has one, has many and belongs to associations
func handleRelationPreload(scope *Scope, field *StructField, conditions []interface{}) {
	var (
		indirectScopeValue = IndirectValue(scope.Value)

		query       = ""
		primaryKeys [][]interface{}

		ForeignDBNames               = field.GetForeignDBNames()
		ForeignFieldNames            = field.GetForeignFieldNames()
		AssociationForeignFieldNames = field.GetAssociationForeignFieldNames()
		AssociationForeignDBNames    = field.GetAssociationDBNames()
		FieldNames                   StrSlice
		DBNames                      StrSlice
	)

	// get relations's primary keys
	if field.RelationIsBelongsTo() {
		FieldNames = ForeignFieldNames
	} else {
		FieldNames = AssociationForeignFieldNames
	}

	primaryKeys = getColumnAsArray(FieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}

	// preload conditions
	preloadDB, preloadConditions := generatePreloadDBWithConditions(scope.con.empty(), conditions)

	values := toQueryValues(primaryKeys)

	// find relations
	if field.RelationIsBelongsTo() {
		DBNames = AssociationForeignDBNames
	} else {
		DBNames = ForeignDBNames
	}

	query = fmt.Sprintf(
		"%v IN (%v)",
		scope.toQueryCondition(DBNames),
		toQueryMarks(primaryKeys))

	if field.HasSetting(set_polymorphic_type) {
		query += fmt.Sprintf(" AND %v = ?", scope.con.quote(field.GetStrSetting(set_polymorphic_dbname)))
		values = append(values, field.GetStrSetting(set_polymorphic_value))
	}

	results, resultsValue := field.makeSlice()
	scope.Err(preloadDB.Where(query, values...).Find(results, preloadConditions...).Error)
	// assign find results

	switch field.RelKind() {
	case rel_has_one:
		switch indirectScopeValue.Kind() {
		case reflect.Slice:
			for j := 0; j < indirectScopeValue.Len(); j++ {
				for i := 0; i < resultsValue.Len(); i++ {
					result := resultsValue.Index(i)
					foreignValues := getValueFromFields(ForeignFieldNames, result)
					indirectValue := FieldValue(indirectScopeValue, j)
					if equalAsString(
						getValueFromFields(
							AssociationForeignFieldNames,
							indirectValue,
						),
						foreignValues,
					) {
						indirectValue.FieldByName(field.StructName).Set(result)
						break
					}
				}
			}
		default:
			for i := 0; i < resultsValue.Len(); i++ {
				result := resultsValue.Index(i)
				scope.Err(field.Set(result))
			}
		}
	case rel_has_many:
		switch indirectScopeValue.Kind() {
		case reflect.Slice:
			preloadMap := make(map[string][]reflect.Value)
			for i := 0; i < resultsValue.Len(); i++ {
				result := resultsValue.Index(i)
				foreignValues := getValueFromFields(ForeignFieldNames, result)
				preloadMap[toString(foreignValues)] = append(preloadMap[toString(foreignValues)], result)
			}

			for j := 0; j < indirectScopeValue.Len(); j++ {
				reflectValue := FieldValue(indirectScopeValue, j)
				objectRealValue := getValueFromFields(AssociationForeignFieldNames, reflectValue)
				f := reflectValue.FieldByName(field.StructName)
				if results, ok := preloadMap[toString(objectRealValue)]; ok {
					f.Set(reflect.Append(f, results...))
				} else {
					f.Set(reflect.MakeSlice(f.Type(), 0, 0))
				}
			}
		default:
			scope.Err(field.Set(resultsValue))

		}
	case rel_belongs_to:
		for i := 0; i < resultsValue.Len(); i++ {
			result := resultsValue.Index(i)
			if indirectScopeValue.Kind() == reflect.Slice {
				value := getValueFromFields(AssociationForeignFieldNames, result)
				for j := 0; j < indirectScopeValue.Len(); j++ {
					reflectValue := FieldValue(indirectScopeValue, j)
					if equalAsString(
						getValueFromFields(
							ForeignFieldNames,
							reflectValue,
						),
						value,
					) {
						reflectValue.FieldByName(field.StructName).Set(result)
					}
				}
			} else {
				scope.Err(field.Set(result))
			}
		}
	}

}

// handleManyToManyPreload used to preload many to many associations
func handleManyToManyPreload(scope *Scope, field *StructField, conditions []interface{}) {
	var (
		fieldType, isPtr   = field.Type, field.IsPointer()
		foreignKeyValue    interface{}
		foreignKeyType     = reflect.ValueOf(&foreignKeyValue).Type()
		linkHash           = map[string][]reflect.Value{}
		indirectScopeValue = IndirectValue(scope.Value)
		fieldsSourceMap    = map[string][]reflect.Value{}
		foreignFieldNames  = StrSlice{}
		ForeignFieldNames  = field.GetForeignFieldNames()
		joinTableHandler   = field.JoinHandler()
	)

	// preload conditions
	preloadDB, preloadConditions := generatePreloadDBWithConditions(scope.con.empty(), conditions)

	// generate query with join table
	freshScope := scope.con.emptyScope(reflect.New(fieldType).Interface())

	preloadDB = preloadDB.Table(freshScope.TableName()).Model(freshScope.Value).Select(str_everything)
	preloadDB = joinTableHandler.JoinWith(joinTableHandler, preloadDB, scope.Value)

	// preload inline conditions
	if len(preloadConditions) > 0 {
		preloadDB = preloadDB.Where(preloadConditions[0], preloadConditions[1:]...)
	}

	rows, err := preloadDB.Rows()

	if scope.Err(err) != nil {
		return
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	for rows.Next() {
		var (
			elem   = reflect.New(fieldType).Elem()
			fields = scope.con.emptyScope(elem.Addr().Interface()).Fields()
		)

		// register foreign keys in join tables
		var joinTableFields StructFields
		for _, sourceKey := range joinTableHandler.SourceForeignKeys() {
			joinTableFields.add(
				&StructField{
					DBName: sourceKey.DBName,
					Value:  reflect.New(foreignKeyType).Elem(),
					flags:  0 | (1 << ff_is_normal), //added as normal field
				})
		}

		scope.scan(rows, columns, append(fields, joinTableFields...))

		var foreignKeys = make([]interface{}, joinTableFields.len())
		// generate hashed forkey keys in join table
		for idx, joinTableField := range joinTableFields {
			if !joinTableField.Value.IsNil() {
				foreignKeys[idx] = joinTableField.Value.Elem().Interface()
			}
		}
		hashedSourceKeys := toString(foreignKeys)

		if isPtr {
			linkHash[hashedSourceKeys] = append(linkHash[hashedSourceKeys], elem.Addr())
		} else {
			linkHash[hashedSourceKeys] = append(linkHash[hashedSourceKeys], elem)
		}
	}

	// assign find results
	for _, dbName := range ForeignFieldNames {
		if field, ok := scope.FieldByName(dbName); ok {
			foreignFieldNames.add(field.StructName)
		}
	}

	switch indirectScopeValue.Kind() {
	case reflect.Slice:
		for j := 0; j < indirectScopeValue.Len(); j++ {
			reflectValue := FieldValue(indirectScopeValue, j)
			key := toString(getValueFromFields(foreignFieldNames, reflectValue))
			fieldsSourceMap[key] = append(fieldsSourceMap[key], reflectValue.FieldByName(field.StructName))
		}
	default:
		if indirectScopeValue.IsValid() {
			key := toString(getValueFromFields(foreignFieldNames, indirectScopeValue))
			fieldsSourceMap[key] = append(fieldsSourceMap[key], indirectScopeValue.FieldByName(field.StructName))
		}
	}

	for source, link := range linkHash {
		for i, field := range fieldsSourceMap[source] {
			//If not 0 this means Value is a pointer and we already added preloaded models to it
			if fieldsSourceMap[source][i].Len() != 0 {
				continue
			}
			field.Set(reflect.Append(fieldsSourceMap[source][i], link...))
		}

	}
}
