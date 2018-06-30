package gorm

import (
	"fmt"
	"reflect"
)

// Find find out all related associations
func (a *Association) Find(value interface{}) *Association {
	a.scope.related(value, a.column)
	return a.setErr(a.scope.con.Error)
}

// Append append new associations for many2many, has_many, replace current association for has_one, belongs_to
func (a *Association) Append(values ...interface{}) *Association {
	if a.Error != nil {
		return a
	}

	if a.field.RelationIsHasOne() {
		return a.Replace(values...)
	}
	return a.saveAssociations(values...)
}

// Replace replace current associations with new one
func (a *Association) Replace(values ...interface{}) *Association {
	if a.Error != nil {
		return a
	}

	var (
		scope                        = a.scope
		field                        = a.field
		fieldValue                   = field.Value
		conn                         = scope.con.empty()
		ForeignDBNames               = field.GetForeignDBNames()
		AssociationForeignFieldNames = field.GetAssociationForeignFieldNames()
	)

	// Append new values
	field.setZeroValue()
	a.saveAssociations(values...)

	switch field.RelKind() {

	case relBelongsTo:
		// Set foreign key to be null when clearing value (length equals 0)
		if len(values) == 0 {
			// Set foreign key to be nil
			var foreignKeyMap = map[string]interface{}{}
			for _, foreignKey := range ForeignDBNames {
				foreignKeyMap[foreignKey] = nil
			}
			a.setErr(conn.Model(scope.Value).UpdateColumn(foreignKeyMap).Error)
		}
	default:
		// Polymorphic Relations
		if field.HasSetting(setPolymorphicDbname) {
			conn = conn.Where(
				fmt.Sprintf(
					"%v = ?",
					conn.quote(field.GetStrSetting(setPolymorphicDbname)),
				),
				field.GetStrSetting(setPolymorphicValue))
		}

		switch field.RelKind() {
		case relMany2many:

			// Delete Relations except new created
			if len(values) > 0 {
				var associationForeignFieldNames, associationForeignDBNames StrSlice
				AssociationForeignDBNames := field.GetAssociationDBNames()
				// if many to many relations, get association fields name from association foreign keys
				associationScope := conn.emptyScope(fieldValue.Interface())
				for idx, dbName := range AssociationForeignFieldNames {
					if field, ok := associationScope.FieldByName(dbName); ok {
						associationForeignFieldNames.add(field.StructName)
						associationForeignDBNames.add(AssociationForeignDBNames[idx])
					}
				}

				newPrimaryKeys := getColumnAsArray(associationForeignFieldNames, fieldValue.Interface())

				if len(newPrimaryKeys) > 0 {
					sql := fmt.Sprintf(
						"%v NOT IN (%v)",
						scope.toQueryCondition(associationForeignDBNames),
						toQueryMarks(newPrimaryKeys),
					)
					conn = conn.Where(sql, toQueryValues(newPrimaryKeys)...)
				}
			}

			// if many to many relations, delete related relations from join table
			var sourceForeignFieldNames StrSlice
			ForeignFieldNames := field.GetForeignFieldNames()
			for _, dbName := range ForeignFieldNames {
				if field, ok := scope.FieldByName(dbName); ok {
					sourceForeignFieldNames.add(field.StructName)
				}
			}

			if sourcePrimaryKeys := scope.getColumnAsArray(sourceForeignFieldNames); len(sourcePrimaryKeys) > 0 {
				conn = conn.Where(
					fmt.Sprintf(
						"%v IN (%v)",
						scope.toQueryCondition(ForeignDBNames),
						toQueryMarks(sourcePrimaryKeys),
					),
					toQueryValues(sourcePrimaryKeys)...,
				)
				joinTableHandler := field.JoinHandler()
				a.setErr(joinTableHandler.Delete(joinTableHandler, conn))
			}
		case relHasOne, relHasMany:
			// Delete Relations except new created
			if len(values) > 0 {
				var assocFKNames, assocDBNames StrSlice

				// If has one/many relations, use primary keys
				for _, field := range conn.emptyScope(fieldValue.Interface()).PKs() {
					assocFKNames.add(field.StructName)
					assocDBNames.add(field.DBName)
				}

				newPrimaryKeys := getColumnAsArray(assocFKNames, fieldValue.Interface())

				if len(newPrimaryKeys) > 0 {
					sql := fmt.Sprintf(
						"%v NOT IN (%v)",
						scope.toQueryCondition(assocDBNames),
						toQueryMarks(newPrimaryKeys),
					)
					conn = conn.Where(sql, toQueryValues(newPrimaryKeys)...)
				}
			}

			// has_one or has_many relations, set foreign key to be nil
			var foreignKeyMap = map[string]interface{}{}

			for idx, foreignKey := range ForeignDBNames {
				foreignKeyMap[foreignKey] = nil
				if field, ok := scope.FieldByName(AssociationForeignFieldNames[idx]); ok {
					conn = conn.Where(fmt.Sprintf("%v = ?", conn.quote(foreignKey)), field.Value.Interface())
				}
			}

			fieldValue := field.Interface()
			a.setErr(conn.Model(fieldValue).UpdateColumn(foreignKeyMap).Error)
		}
	}

	return a
}

// Delete remove relationship between source & passed arguments, but won't delete those arguments
func (a *Association) Delete(values ...interface{}) *Association {
	if a.Error != nil {
		return a
	}

	var (
		field                        = a.field
		fieldValue                   = field.Value
		scope                        = a.scope
		conn                         = scope.con.empty()
		ForeignDBNames               = field.GetForeignDBNames()
		AssociationForeignFieldNames = field.GetAssociationForeignFieldNames()
	)

	if len(values) == 0 {
		return a
	}

	var deletingResourcePrimaryFieldNames, deletingResourcePrimaryDBNames StrSlice
	for _, field := range conn.emptyScope(fieldValue.Interface()).PKs() {
		deletingResourcePrimaryFieldNames.add(field.StructName)
		deletingResourcePrimaryDBNames.add(field.DBName)
	}

	deletingPrimaryKeys := getColumnAsArray(deletingResourcePrimaryFieldNames, values...)

	switch field.RelKind() {
	case relMany2many:
		ForeignFieldNames := field.GetForeignFieldNames()
		AssociationForeignDBNames := field.GetAssociationDBNames()
		// source value's foreign keys
		for idx, foreignKey := range ForeignDBNames {
			if field, ok := scope.FieldByName(ForeignFieldNames[idx]); ok {
				conn = conn.Where(
					fmt.Sprintf(
						"%v = ?",
						conn.quote(foreignKey),
					),
					field.Value.Interface())
			}
		}

		// get association's foreign fields name
		var associationScope = conn.emptyScope(fieldValue.Interface())
		var associationForeignFieldNames StrSlice
		for _, associationDBName := range AssociationForeignFieldNames {
			if field, ok := associationScope.FieldByName(associationDBName); ok {
				associationForeignFieldNames.add(field.StructName)
			}
		}

		// association value's foreign keys
		deletingPrimaryKeys := getColumnAsArray(associationForeignFieldNames, values...)
		sql := fmt.Sprintf(
			"%v IN (%v)",
			scope.toQueryCondition(AssociationForeignDBNames),
			toQueryMarks(deletingPrimaryKeys),
		)
		conn = conn.Where(sql, toQueryValues(deletingPrimaryKeys)...)
		joinTableHandler := field.JoinHandler()
		a.setErr(joinTableHandler.Delete(joinTableHandler, conn))
	default:
		var foreignKeyMap = map[string]interface{}{}
		for _, foreignKey := range ForeignDBNames {
			foreignKeyMap[foreignKey] = nil
		}
		switch field.RelKind() {
		case relBelongsTo:
			// find with deleting relation's foreign keys
			primaryKeys := getColumnAsArray(AssociationForeignFieldNames, values...)
			conn = conn.Where(
				fmt.Sprintf(
					"%v IN (%v)",
					scope.toQueryCondition(ForeignDBNames),
					toQueryMarks(primaryKeys),
				),
				toQueryValues(primaryKeys)...,
			)

			// set foreign key to be null if there are some records affected
			modelValue := scope.GetModelStruct().Interface()
			if results := conn.Model(modelValue).UpdateColumn(foreignKeyMap); results.Error == nil {
				if results.RowsAffected > 0 {
					updatedAttrsWithValues(scope, foreignKeyMap)
				}
			} else {
				a.setErr(results.Error)
			}
		case relHasOne, relHasMany:
			// find all relations
			primaryKeys := scope.getColumnAsArray(AssociationForeignFieldNames)
			conn = conn.Where(
				fmt.Sprintf(
					"%v IN (%v)",
					scope.toQueryCondition(ForeignDBNames),
					toQueryMarks(primaryKeys),
				),
				toQueryValues(primaryKeys)...,
			)

			// only include those deleting relations
			conn = conn.Where(
				fmt.Sprintf(
					"%v IN (%v)",
					scope.toQueryCondition(deletingResourcePrimaryDBNames),
					toQueryMarks(deletingPrimaryKeys),
				),
				toQueryValues(deletingPrimaryKeys)...,
			)

			// set matched relation's foreign key to be null
			fieldValue := field.Interface()
			a.setErr(conn.Model(fieldValue).UpdateColumn(foreignKeyMap).Error)
		}
	}

	// Remove deleted records from source's field
	if a.Error == nil {
		if fieldValue.Kind() == reflect.Slice {
			leftValues := SetZero(fieldValue)

			for i := 0; i < fieldValue.Len(); i++ {
				reflectValue := fieldValue.Index(i)
				//TODO : check if len is ok
				primaryKey := getColumnAsArray(deletingResourcePrimaryFieldNames, reflectValue.Interface())[0]
				var isDeleted = false
				for _, pk := range deletingPrimaryKeys {
					if equalAsString(primaryKey, pk) {
						isDeleted = true
						break
					}
				}
				if !isDeleted {
					leftValues = reflect.Append(leftValues, reflectValue)
				}
			}

			field.Set(leftValues)
		} else if fieldValue.Kind() == reflect.Struct {
			primaryKey := getColumnAsArray(deletingResourcePrimaryFieldNames, fieldValue.Interface())[0]
			for _, pk := range deletingPrimaryKeys {
				if equalAsString(primaryKey, pk) {
					field.Set(SetZero(fieldValue))
					break
				}
			}
		}
	}

	return a
}

// Clear remove relationship between source & current associations, won't delete those associations
func (a *Association) Clear() *Association {
	return a.Replace()
}

// Count return the count of current associations
func (a *Association) Count() int {
	var (
		count      = 0
		field      = a.field
		scope      = a.scope
		conn       = scope.con
		fieldValue = field.Value.Interface()
	)

	switch field.RelKind() {
	case relMany2many:
		joinTableHandler := field.JoinHandler()
		conn = joinTableHandler.JoinWith(joinTableHandler, conn, scope.Value)
	case relHasMany, relHasOne:
		AssociationForeignFieldNames := field.GetAssociationForeignFieldNames()
		primaryKeys := scope.getColumnAsArray(AssociationForeignFieldNames)
		if len(primaryKeys) == 0 {
			//fix where %v IN (%v) is empty
			return 0
		}
		ForeignDBNames := field.GetForeignDBNames()
		conn = conn.Where(
			fmt.Sprintf(
				"%v IN (%v)",
				scope.toQueryCondition(ForeignDBNames),
				toQueryMarks(primaryKeys),
			),
			toQueryValues(primaryKeys)...,
		)
	case relBelongsTo:
		ForeignFieldNames := field.GetForeignFieldNames()
		primaryKeys := scope.getColumnAsArray(ForeignFieldNames)
		if len(primaryKeys) == 0 {
			//fix where %v IN (%v) is empty
			return 0
		}
		AssociationForeignDBNames := field.GetAssociationDBNames()
		conn = conn.Where(
			fmt.Sprintf(
				"%v IN (%v)",
				scope.toQueryCondition(AssociationForeignDBNames),
				toQueryMarks(primaryKeys),
			),
			toQueryValues(primaryKeys)...,
		)
	}

	if field.HasSetting(setPolymorphicType) {
		conn = conn.Where(
			fmt.Sprintf(
				"%v.%v = ?",
				conn.emptyScope(fieldValue).quotedTableName(),
				conn.quote(field.GetStrSetting(setPolymorphicDbname)),
			),
			field.GetStrSetting(setPolymorphicValue),
		)
	}
	conn.Model(fieldValue).Count(&count)
	return count
}

func (a *Association) reflect(reflectValue reflect.Value) {
	var (
		scope = a.scope
		field = a.field

		fieldType                                     = field.Value.Type()
		setFieldBackToValue, setSliceFieldBackToValue bool
	)
	// value has to been pointer
	if reflectValue.Kind() != reflect.Ptr {
		reflectPtr := reflect.New(reflectValue.Type())
		reflectPtr.Elem().Set(reflectValue)
		reflectValue = reflectPtr
	}

	// value has to been saved for many2many
	if field.RelationIsMany2Many() {
		if scope.con.emptyScope(reflectValue.Interface()).PrimaryKeyZero() {
			a.setErr(scope.con.empty().Save(reflectValue.Interface()).Error)
		}
	}

	// Assign Fields
	if reflectValue.Type().AssignableTo(fieldType) {
		field.Set(reflectValue)
	} else if reflectValue.Type().Elem().AssignableTo(fieldType) {
		// if field's type is struct, then need to set value back to argument after save
		setFieldBackToValue = true
		field.Set(reflectValue.Elem())
	} else if fieldType.Kind() == reflect.Slice {
		if reflectValue.Type().AssignableTo(fieldType.Elem()) {
			field.Set(reflect.Append(field.Value, reflectValue))
		} else if reflectValue.Type().Elem().AssignableTo(fieldType.Elem()) {
			// if field's type is slice of struct, then need to set value back to argument after save
			setSliceFieldBackToValue = true
			field.Set(reflect.Append(field.Value, reflectValue.Elem()))
		}
	}

	if field.RelationIsMany2Many() {
		joinTableHandler := field.JoinHandler()
		a.setErr(
			joinTableHandler.Add(
				joinTableHandler,
				scope.con.empty(),
				scope.Value,
				reflectValue.Interface(),
			),
		)
	} else {
		a.setErr(scope.con.empty().Select(field.StructName).Save(scope.Value).Error)

		if setFieldBackToValue {
			reflectValue.Elem().Set(field.Value)
		} else if setSliceFieldBackToValue {
			reflectValue.Elem().Set(field.Value.Index(field.Value.Len() - 1))
		}
	}
}

// saveAssociations save passed values as associations
func (a *Association) saveAssociations(values ...interface{}) *Association {
	for _, value := range values {
		reflectValue := reflect.ValueOf(value)
		indirectReflectValue := reflect.Indirect(reflectValue)
		if indirectReflectValue.Kind() == reflect.Struct {
			a.reflect(reflectValue)
		} else if indirectReflectValue.Kind() == reflect.Slice {
			for i := 0; i < indirectReflectValue.Len(); i++ {
				a.reflect(indirectReflectValue.Index(i))
			}
		} else {
			a.setErr(fmt.Errorf("ASSOCIATION : invalid value type %v", indirectReflectValue.Kind()))
		}
	}
	return a
}

func (a *Association) setErr(err error) *Association {
	if err != nil {
		a.Error = err
	}
	return a
}
