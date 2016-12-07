package gorm

import (
	"errors"
	"fmt"
	"reflect"
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

// handleRelationPreload to preload has one, has many and belongs to associations
func handleRelationPreload(scope *Scope, field *StructField, conditions []interface{}) {
	var (
		indirectScopeValue = IndirectValue(scope.Value)

		dialect     = scope.con.parent.dialect
		query       = ""
		primaryKeys [][]interface{}

		ForeignDBNames               = field.GetSliceSetting(FOREIGN_DB_NAMES)
		ForeignFieldNames            = field.GetSliceSetting(FOREIGN_FIELD_NAMES)
		AssociationForeignFieldNames = field.GetSliceSetting(ASSOCIATION_FOREIGN_FIELD_NAMES)
		AssociationForeignDBNames    = field.GetSliceSetting(ASSOCIATION_FOREIGN_DB_NAMES)
		FieldNames                   StrSlice
		DBNames                      StrSlice
	)

	// get relations's primary keys
	if field.RelKind() == BELONGS_TO {
		FieldNames = ForeignFieldNames
	} else {
		FieldNames = AssociationForeignFieldNames
	}

	primaryKeys = getColumnAsArray(FieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}

	// preload conditions
	preloadDB, preloadConditions := generatePreloadDBWithConditions(newCon(scope.con), conditions)

	values := toQueryValues(primaryKeys)

	// find relations
	if field.RelKind() == BELONGS_TO {
		DBNames = AssociationForeignDBNames
	} else {
		DBNames = ForeignDBNames
	}

	query = fmt.Sprintf(
		"%v IN (%v)",
		toQueryCondition(DBNames, dialect),
		toQueryMarks(primaryKeys))

	if field.HasSetting(POLYMORPHIC_TYPE) {
		query += fmt.Sprintf(" AND %v = ?", Quote(field.GetStrSetting(POLYMORPHIC_DBNAME), dialect))
		values = append(values, field.GetStrSetting(POLYMORPHIC_VALUE))
	}

	results, resultsValue := field.makeSlice()
	scope.Err(preloadDB.Where(query, values...).Find(results, preloadConditions...).Error)
	// assign find results

	switch field.RelKind() {
	case HAS_ONE:
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
	case HAS_MANY:
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
	case BELONGS_TO:
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
		sourceKeys         = []string{}

		ForeignFieldNames = field.GetSliceSetting(FOREIGN_FIELD_NAMES)
		joinTableHandler  = field.JoinHandler()
	)

	for _, key := range joinTableHandler.SourceForeignKeys() {
		sourceKeys = append(sourceKeys, key.DBName)
	}

	// preload conditions
	preloadDB, preloadConditions := generatePreloadDBWithConditions(newCon(scope.con), conditions)

	// generate query with join table
	newScope := scope.NewScope(reflect.New(fieldType).Interface())

	preloadDB = preloadDB.Table(newScope.TableName()).Model(newScope.Value).Select("*")
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
			fields = scope.NewScope(elem.Addr().Interface()).Fields()
		)

		// register foreign keys in join tables
		var joinTableFields StructFields
		for _, sourceKey := range sourceKeys {
			joinTableFields.add(
				&StructField{
					DBName: sourceKey,
					Value:  reflect.New(foreignKeyType).Elem(),
					flags:  0 | (1 << IS_NORMAL),
				})
		}

		scope.scan(rows, columns, append(fields, joinTableFields...))

		var foreignKeys = make([]interface{}, len(sourceKeys))
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
