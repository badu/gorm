package gorm

import (
	"errors"
	"fmt"
	"reflect"
)

const (
	//Relationship Kind constants
	MANY_TO_MANY uint8 = 1
	HAS_MANY     uint8 = 2
	HAS_ONE      uint8 = 3
	BELONGS_TO   uint8 = 4 //Attention : relationship.Kind <= HAS_ONE in callback_functions.go saveAfterAssociationsCallback()
)

// Find find out all related associations
func (association *Association) Find(value interface{}) *Association {
	association.scope.related(value, association.column)
	return association.setErr(association.scope.con.Error)
}

// Append append new associations for many2many, has_many, replace current association for has_one, belongs_to
func (association *Association) Append(values ...interface{}) *Association {
	if association.Error != nil {
		return association
	}

	if relationship := association.field.Relationship; relationship.Kind == HAS_ONE {
		return association.Replace(values...)
	}
	return association.saveAssociations(values...)
}

// Replace replace current associations with new one
func (association *Association) Replace(values ...interface{}) *Association {
	if association.Error != nil {
		return association
	}

	var (
		scope        = association.scope
		field        = association.field
		relationship = field.Relationship
		fieldValue   = field.Value
		conn         = newCon(scope.con)
		dialect      = conn.parent.dialect
	)

	// Append new values
	field.Set(reflect.Zero(field.Value.Type()))
	association.saveAssociations(values...)

	switch relationship.Kind {

	case BELONGS_TO:
		// Set foreign key to be null when clearing value (length equals 0)
		if len(values) == 0 {
			// Set foreign key to be nil
			var foreignKeyMap = map[string]interface{}{}
			for _, foreignKey := range relationship.ForeignDBNames {
				foreignKeyMap[foreignKey] = nil
			}
			association.setErr(conn.Model(scope.Value).UpdateColumn(foreignKeyMap).Error)
		}
	default:
		// Polymorphic Relations
		if relationship.PolymorphicDBName != "" {
			conn = conn.Where(
				fmt.Sprintf(
					"%v = ?",
					Quote(relationship.PolymorphicDBName, dialect),
				),
				relationship.PolymorphicValue)
		}

		switch relationship.Kind {
		case MANY_TO_MANY:

			// Delete Relations except new created
			if len(values) > 0 {
				var associationForeignFieldNames, associationForeignDBNames StrSlice

				// if many to many relations, get association fields name from association foreign keys
				associationScope := scope.NewScope(fieldValue.Interface())
				for idx, dbName := range relationship.AssociationForeignFieldNames {
					if field, ok := associationScope.FieldByName(dbName); ok {
						associationForeignFieldNames.add(field.StructName)
						associationForeignDBNames.add(relationship.AssociationForeignDBNames[idx])
					}
				}

				newPrimaryKeys := getColumnAsArray(associationForeignFieldNames, fieldValue.Interface())

				if len(newPrimaryKeys) > 0 {
					sql := fmt.Sprintf(
						"%v NOT IN (%v)",
						toQueryCondition(associationForeignDBNames, dialect),
						toQueryMarks(newPrimaryKeys),
					)
					conn = conn.Where(sql, toQueryValues(newPrimaryKeys)...)
				}
			}

			// if many to many relations, delete related relations from join table
			var sourceForeignFieldNames StrSlice

			for _, dbName := range relationship.ForeignFieldNames {
				if field, ok := scope.FieldByName(dbName); ok {
					sourceForeignFieldNames.add(field.StructName)
				}
			}

			if sourcePrimaryKeys := getColumnAsArray(sourceForeignFieldNames, scope.Value); len(sourcePrimaryKeys) > 0 {
				conn = conn.Where(
					fmt.Sprintf(
						"%v IN (%v)",
						toQueryCondition(relationship.ForeignDBNames, dialect),
						toQueryMarks(sourcePrimaryKeys),
					),
					toQueryValues(sourcePrimaryKeys)...,
				)

				association.setErr(relationship.JoinTableHandler.Delete(relationship.JoinTableHandler, conn, relationship))
			}
		case HAS_ONE, HAS_MANY:
			// Delete Relations except new created
			if len(values) > 0 {
				var assocFKNames, assocDBNames StrSlice

				// If has one/many relations, use primary keys
				for _, field := range scope.NewScope(fieldValue.Interface()).PKs() {
					assocFKNames.add(field.StructName)
					assocDBNames.add(field.DBName)
				}

				newPrimaryKeys := getColumnAsArray(assocFKNames, fieldValue.Interface())

				if len(newPrimaryKeys) > 0 {
					sql := fmt.Sprintf(
						"%v NOT IN (%v)",
						toQueryCondition(assocDBNames, dialect),
						toQueryMarks(newPrimaryKeys),
					)
					conn = conn.Where(sql, toQueryValues(newPrimaryKeys)...)
				}
			}

			// has_one or has_many relations, set foreign key to be nil (TODO or delete them?)
			var foreignKeyMap = map[string]interface{}{}

			for idx, foreignKey := range relationship.ForeignDBNames {
				foreignKeyMap[foreignKey] = nil
				if field, ok := scope.FieldByName(relationship.AssociationForeignFieldNames[idx]); ok {
					conn = conn.Where(fmt.Sprintf("%v = ?", Quote(foreignKey, dialect)), field.Value.Interface())
				}
			}

			fieldValue := field.Interface()
			association.setErr(conn.Model(fieldValue).UpdateColumn(foreignKeyMap).Error)
		}
	}

	return association
}

// Delete remove relationship between source & passed arguments, but won't delete those arguments
func (association *Association) Delete(values ...interface{}) *Association {
	if association.Error != nil {
		return association
	}

	var (
		field        = association.field
		relationship = field.Relationship
		fieldValue   = field.Value
		scope        = association.scope
		conn         = newCon(scope.con)
		dialect      = conn.parent.dialect
	)

	if len(values) == 0 {
		return association
	}

	var deletingResourcePrimaryFieldNames, deletingResourcePrimaryDBNames StrSlice
	for _, field := range scope.NewScope(fieldValue.Interface()).PKs() {
		deletingResourcePrimaryFieldNames.add(field.StructName)
		deletingResourcePrimaryDBNames.add(field.DBName)
	}

	deletingPrimaryKeys := getColumnAsArray(deletingResourcePrimaryFieldNames, values...)

	switch relationship.Kind {
	case MANY_TO_MANY:
		// source value's foreign keys
		for idx, foreignKey := range relationship.ForeignDBNames {
			if field, ok := scope.FieldByName(relationship.ForeignFieldNames[idx]); ok {
				conn = conn.Where(
					fmt.Sprintf(
						"%v = ?",
						Quote(foreignKey, dialect),
					),
					field.Value.Interface())
			}
		}

		// get association's foreign fields name
		var associationScope = scope.NewScope(fieldValue.Interface())
		var associationForeignFieldNames StrSlice
		for _, associationDBName := range relationship.AssociationForeignFieldNames {
			if field, ok := associationScope.FieldByName(associationDBName); ok {
				associationForeignFieldNames.add(field.StructName)
			}
		}

		// association value's foreign keys
		deletingPrimaryKeys := getColumnAsArray(associationForeignFieldNames, values...)
		sql := fmt.Sprintf(
			"%v IN (%v)",
			toQueryCondition(relationship.AssociationForeignDBNames, dialect),
			toQueryMarks(deletingPrimaryKeys),
		)
		conn = conn.Where(sql, toQueryValues(deletingPrimaryKeys)...)

		association.setErr(relationship.JoinTableHandler.Delete(relationship.JoinTableHandler, conn, relationship))
	default:
		var foreignKeyMap = map[string]interface{}{}
		for _, foreignKey := range relationship.ForeignDBNames {
			foreignKeyMap[foreignKey] = nil
		}
		switch relationship.Kind {
		case BELONGS_TO:
			// find with deleting relation's foreign keys
			primaryKeys := getColumnAsArray(relationship.AssociationForeignFieldNames, values...)
			conn = conn.Where(
				fmt.Sprintf(
					"%v IN (%v)",
					toQueryCondition(relationship.ForeignDBNames, dialect),
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
				association.setErr(results.Error)
			}
		case HAS_ONE, HAS_MANY:
			// find all relations
			primaryKeys := getColumnAsArray(relationship.AssociationForeignFieldNames, scope.Value)
			conn = conn.Where(
				fmt.Sprintf(
					"%v IN (%v)",
					toQueryCondition(relationship.ForeignDBNames, dialect),
					toQueryMarks(primaryKeys),
				),
				toQueryValues(primaryKeys)...,
			)

			// only include those deleting relations
			conn = conn.Where(
				fmt.Sprintf(
					"%v IN (%v)",
					toQueryCondition(deletingResourcePrimaryDBNames, dialect),
					toQueryMarks(deletingPrimaryKeys),
				),
				toQueryValues(deletingPrimaryKeys)...,
			)

			// set matched relation's foreign key to be null
			fieldValue := field.Interface()
			association.setErr(conn.Model(fieldValue).UpdateColumn(foreignKeyMap).Error)
		}
	}

	// Remove deleted records from source's field
	if association.Error == nil {
		if fieldValue.Kind() == reflect.Slice {
			leftValues := reflect.Zero(fieldValue.Type())

			for i := 0; i < fieldValue.Len(); i++ {
				reflectValue := fieldValue.Index(i)
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
					field.Set(reflect.Zero(fieldValue.Type()))
					break
				}
			}
		}
	}

	return association
}

// Clear remove relationship between source & current associations, won't delete those associations
func (association *Association) Clear() *Association {
	return association.Replace()
}

// Count return the count of current associations
func (association *Association) Count() int {
	var (
		count        = 0
		field        = association.field
		relationship = field.Relationship
		fieldValue   = field.Value.Interface()
		scope        = association.scope
		conn         = scope.con
		dialect      = conn.parent.dialect
	)
	switch relationship.Kind {
	case MANY_TO_MANY:
		conn = relationship.JoinTableHandler.JoinWith(relationship.JoinTableHandler, conn, scope.Value)
	case HAS_MANY, HAS_ONE:
		primaryKeys := getColumnAsArray(relationship.AssociationForeignFieldNames, scope.Value)
		conn = conn.Where(
			fmt.Sprintf(
				"%v IN (%v)",
				toQueryCondition(relationship.ForeignDBNames, dialect),
				toQueryMarks(primaryKeys),
			),
			toQueryValues(primaryKeys)...,
		)
	case BELONGS_TO:
		primaryKeys := getColumnAsArray(relationship.ForeignFieldNames, scope.Value)
		conn = conn.Where(
			fmt.Sprintf(
				"%v IN (%v)",
				toQueryCondition(relationship.AssociationForeignDBNames, dialect),
				toQueryMarks(primaryKeys),
			),
			toQueryValues(primaryKeys)...,
		)
	}

	if relationship.PolymorphicType != "" {
		conn = conn.Where(
			fmt.Sprintf(
				"%v.%v = ?",
				QuotedTableName(scope.NewScope(fieldValue)),
				Quote(relationship.PolymorphicDBName, dialect),
			),
			relationship.PolymorphicValue,
		)
	}

	conn.Model(fieldValue).Count(&count)
	return count
}

func (association *Association) reflect(reflectValue reflect.Value) {
	var (
		scope                                         = association.scope
		field                                         = association.field
		relationship                                  = field.Relationship
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
	if relationship.Kind == MANY_TO_MANY {
		if scope.NewScope(reflectValue.Interface()).PrimaryKeyZero() {
			association.setErr(newCon(scope.con).Save(reflectValue.Interface()).Error)
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

	if relationship.Kind == MANY_TO_MANY {
		association.setErr(
			relationship.JoinTableHandler.Add(
				relationship.JoinTableHandler,
				newCon(scope.con),
				scope.Value,
				reflectValue.Interface(),
			),
		)
	} else {
		association.setErr(newCon(scope.con).Select(field.StructName).Save(scope.Value).Error)

		if setFieldBackToValue {
			reflectValue.Elem().Set(field.Value)
		} else if setSliceFieldBackToValue {
			reflectValue.Elem().Set(field.Value.Index(field.Value.Len() - 1))
		}
	}
}

// saveAssociations save passed values as associations
func (association *Association) saveAssociations(values ...interface{}) *Association {
	for _, value := range values {
		reflectValue := reflect.ValueOf(value)
		indirectReflectValue := reflect.Indirect(reflectValue)
		if indirectReflectValue.Kind() == reflect.Struct {
			association.reflect(reflectValue)
		} else if indirectReflectValue.Kind() == reflect.Slice {
			for i := 0; i < indirectReflectValue.Len(); i++ {
				association.reflect(indirectReflectValue.Index(i))
			}
		} else {
			association.setErr(errors.New("ASSOCIATION : invalid value type"))
		}
	}
	return association
}

func (association *Association) setErr(err error) *Association {
	if err != nil {
		association.Error = err
	}
	return association
}
