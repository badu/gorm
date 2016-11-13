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
	//Attention : relationship.Kind <= HAS_ONE in callback_functions.go saveAfterAssociationsCallback()
	BELONGS_TO uint8 = 4
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
		relationship = association.field.Relationship
		scope        = association.scope
		field        = association.field.Value
		newDB        = scope.NewCon()
	)

	// Append new values
	association.field.Set(reflect.Zero(association.field.Value.Type()))
	association.saveAssociations(values...)

	// Belongs To
	if relationship.Kind == BELONGS_TO {
		// Set foreign key to be null when clearing value (length equals 0)
		if len(values) == 0 {
			// Set foreign key to be nil
			var foreignKeyMap = map[string]interface{}{}
			for _, foreignKey := range relationship.ForeignDBNames {
				foreignKeyMap[foreignKey] = nil
			}
			association.setErr(newDB.Model(scope.Value).UpdateColumn(foreignKeyMap).Error)
		}
	} else {
		// Polymorphic Relations
		if relationship.PolymorphicDBName != "" {
			newDB = newDB.Where(fmt.Sprintf("%v = ?", scope.Quote(relationship.PolymorphicDBName)), relationship.PolymorphicValue)
		}

		// Delete Relations except new created
		if len(values) > 0 {
			var associationForeignFieldNames, associationForeignDBNames StrSlice
			if relationship.Kind == MANY_TO_MANY {
				// if many to many relations, get association fields name from association foreign keys
				associationScope := scope.New(reflect.New(field.Type()).Interface())
				for idx, dbName := range relationship.AssociationForeignFieldNames {
					if field, ok := associationScope.FieldByName(dbName); ok {
						associationForeignFieldNames.add(field.GetName())
						associationForeignDBNames.add(relationship.AssociationForeignDBNames[idx])
					}
				}
			} else {
				// If has one/many relations, use primary keys
				for _, field := range scope.New(reflect.New(field.Type()).Interface()).PrimaryFields() {
					associationForeignFieldNames.add(field.GetName())
					associationForeignDBNames.add(field.DBName)
				}
			}

			newPrimaryKeys := scope.getColumnAsArray(associationForeignFieldNames, field.Interface())

			if len(newPrimaryKeys) > 0 {
				sql := fmt.Sprintf("%v NOT IN (%v)", scope.toQueryCondition(associationForeignDBNames), toQueryMarks(newPrimaryKeys))
				newDB = newDB.Where(sql, toQueryValues(newPrimaryKeys)...)
			}
		}

		if relationship.Kind == MANY_TO_MANY {
			// if many to many relations, delete related relations from join table
			var sourceForeignFieldNames StrSlice

			for _, dbName := range relationship.ForeignFieldNames {
				if field, ok := scope.FieldByName(dbName); ok {
					sourceForeignFieldNames.add(field.GetName())
				}
			}

			if sourcePrimaryKeys := scope.getColumnAsArray(sourceForeignFieldNames, scope.Value); len(sourcePrimaryKeys) > 0 {
				newDB = newDB.Where(fmt.Sprintf("%v IN (%v)", scope.toQueryCondition(relationship.ForeignDBNames), toQueryMarks(sourcePrimaryKeys)), toQueryValues(sourcePrimaryKeys)...)

				association.setErr(relationship.JoinTableHandler.Delete(relationship.JoinTableHandler, newDB, relationship))
			}
		} else if relationship.Kind == HAS_ONE || relationship.Kind == HAS_MANY {
			// has_one or has_many relations, set foreign key to be nil (TODO or delete them?)
			var foreignKeyMap = map[string]interface{}{}
			for idx, foreignKey := range relationship.ForeignDBNames {
				foreignKeyMap[foreignKey] = nil
				if field, ok := scope.FieldByName(relationship.AssociationForeignFieldNames[idx]); ok {
					newDB = newDB.Where(fmt.Sprintf("%v = ?", scope.Quote(foreignKey)), field.Value.Interface())
				}
			}

			fieldValue := reflect.New(association.field.Value.Type()).Interface()
			association.setErr(newDB.Model(fieldValue).UpdateColumn(foreignKeyMap).Error)
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
		relationship = association.field.Relationship
		scope        = association.scope
		field        = association.field.Value
		newDB        = scope.NewCon()
	)

	if len(values) == 0 {
		return association
	}

	var deletingResourcePrimaryFieldNames, deletingResourcePrimaryDBNames StrSlice
	for _, field := range scope.New(reflect.New(field.Type()).Interface()).PrimaryFields() {
		deletingResourcePrimaryFieldNames.add(field.GetName())
		deletingResourcePrimaryDBNames.add(field.DBName)
	}

	deletingPrimaryKeys := scope.getColumnAsArray(deletingResourcePrimaryFieldNames, values...)

	if relationship.Kind == MANY_TO_MANY {
		// source value's foreign keys
		for idx, foreignKey := range relationship.ForeignDBNames {
			if field, ok := scope.FieldByName(relationship.ForeignFieldNames[idx]); ok {
				newDB = newDB.Where(fmt.Sprintf("%v = ?", scope.Quote(foreignKey)), field.Value.Interface())
			}
		}

		// get association's foreign fields name
		var associationScope = scope.New(reflect.New(field.Type()).Interface())
		var associationForeignFieldNames StrSlice
		for _, associationDBName := range relationship.AssociationForeignFieldNames {
			if field, ok := associationScope.FieldByName(associationDBName); ok {
				associationForeignFieldNames.add(field.GetName())
			}
		}

		// association value's foreign keys
		deletingPrimaryKeys := scope.getColumnAsArray(associationForeignFieldNames, values...)
		sql := fmt.Sprintf("%v IN (%v)", scope.toQueryCondition(relationship.AssociationForeignDBNames), toQueryMarks(deletingPrimaryKeys))
		newDB = newDB.Where(sql, toQueryValues(deletingPrimaryKeys)...)

		association.setErr(relationship.JoinTableHandler.Delete(relationship.JoinTableHandler, newDB, relationship))
	} else {
		var foreignKeyMap = map[string]interface{}{}
		for _, foreignKey := range relationship.ForeignDBNames {
			foreignKeyMap[foreignKey] = nil
		}

		if relationship.Kind == BELONGS_TO {
			// find with deleting relation's foreign keys
			primaryKeys := scope.getColumnAsArray(relationship.AssociationForeignFieldNames, values...)
			newDB = newDB.Where(
				fmt.Sprintf("%v IN (%v)", scope.toQueryCondition(relationship.ForeignDBNames), toQueryMarks(primaryKeys)),
				toQueryValues(primaryKeys)...,
			)

			// set foreign key to be null if there are some records affected
			modelValue := reflect.New(scope.GetModelStruct().ModelType).Interface()
			if results := newDB.Model(modelValue).UpdateColumn(foreignKeyMap); results.Error == nil {
				if results.RowsAffected > 0 {
					scope.updatedAttrsWithValues(foreignKeyMap)
				}
			} else {
				association.setErr(results.Error)
			}
		} else if relationship.Kind == HAS_ONE || relationship.Kind == HAS_MANY {
			// find all relations
			primaryKeys := scope.getColumnAsArray(relationship.AssociationForeignFieldNames, scope.Value)
			newDB = newDB.Where(
				fmt.Sprintf("%v IN (%v)", scope.toQueryCondition(relationship.ForeignDBNames), toQueryMarks(primaryKeys)),
				toQueryValues(primaryKeys)...,
			)

			// only include those deleting relations
			newDB = newDB.Where(
				fmt.Sprintf("%v IN (%v)", scope.toQueryCondition(deletingResourcePrimaryDBNames), toQueryMarks(deletingPrimaryKeys)),
				toQueryValues(deletingPrimaryKeys)...,
			)

			// set matched relation's foreign key to be null
			fieldValue := reflect.New(association.field.Value.Type()).Interface()
			association.setErr(newDB.Model(fieldValue).UpdateColumn(foreignKeyMap).Error)
		}
	}

	// Remove deleted records from source's field
	if association.Error == nil {
		if field.Kind() == reflect.Slice {
			leftValues := reflect.Zero(field.Type())

			for i := 0; i < field.Len(); i++ {
				reflectValue := field.Index(i)
				primaryKey := scope.getColumnAsArray(deletingResourcePrimaryFieldNames, reflectValue.Interface())[0]
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

			association.field.Set(leftValues)
		} else if field.Kind() == reflect.Struct {
			primaryKey := scope.getColumnAsArray(deletingResourcePrimaryFieldNames, field.Interface())[0]
			for _, pk := range deletingPrimaryKeys {
				if equalAsString(primaryKey, pk) {
					association.field.Set(reflect.Zero(field.Type()))
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
		relationship = association.field.Relationship
		scope        = association.scope
		fieldValue   = association.field.Value.Interface()
		query        = scope.Con()
	)

	if relationship.Kind == MANY_TO_MANY {
		query = relationship.JoinTableHandler.JoinWith(relationship.JoinTableHandler, query, scope.Value)
	} else if relationship.Kind == HAS_MANY || relationship.Kind == HAS_ONE {
		primaryKeys := scope.getColumnAsArray(relationship.AssociationForeignFieldNames, scope.Value)
		query = query.Where(
			fmt.Sprintf("%v IN (%v)", scope.toQueryCondition(relationship.ForeignDBNames), toQueryMarks(primaryKeys)),
			toQueryValues(primaryKeys)...,
		)
	} else if relationship.Kind == BELONGS_TO {
		primaryKeys := scope.getColumnAsArray(relationship.ForeignFieldNames, scope.Value)
		query = query.Where(
			fmt.Sprintf("%v IN (%v)", scope.toQueryCondition(relationship.AssociationForeignDBNames), toQueryMarks(primaryKeys)),
			toQueryValues(primaryKeys)...,
		)
	}

	if relationship.PolymorphicType != "" {
		query = query.Where(
			fmt.Sprintf("%v.%v = ?", scope.New(fieldValue).QuotedTableName(), scope.Quote(relationship.PolymorphicDBName)),
			relationship.PolymorphicValue,
		)
	}

	query.Model(fieldValue).Count(&count)
	return count
}

// saveAssociations save passed values as associations
func (association *Association) saveAssociations(values ...interface{}) *Association {
	var (
		scope        = association.scope
		field        = association.field
		relationship = field.Relationship
	)

	saveAssociation := func(reflectValue reflect.Value) {
		// value has to been pointer
		if reflectValue.Kind() != reflect.Ptr {
			reflectPtr := reflect.New(reflectValue.Type())
			reflectPtr.Elem().Set(reflectValue)
			reflectValue = reflectPtr
		}

		// value has to been saved for many2many
		if relationship.Kind == MANY_TO_MANY {
			if scope.New(reflectValue.Interface()).PrimaryKeyZero() {
				association.setErr(scope.NewCon().Save(reflectValue.Interface()).Error)
			}
		}

		// Assign Fields
		var fieldType = field.Value.Type()
		var setFieldBackToValue, setSliceFieldBackToValue bool
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
			association.setErr(relationship.JoinTableHandler.Add(relationship.JoinTableHandler, scope.NewCon(), scope.Value, reflectValue.Interface()))
		} else {
			association.setErr(scope.NewCon().Select(field.GetName()).Save(scope.Value).Error)

			if setFieldBackToValue {
				reflectValue.Elem().Set(field.Value)
			} else if setSliceFieldBackToValue {
				reflectValue.Elem().Set(field.Value.Index(field.Value.Len() - 1))
			}
		}
	}

	for _, value := range values {
		reflectValue := reflect.ValueOf(value)
		indirectReflectValue := reflect.Indirect(reflectValue)
		if indirectReflectValue.Kind() == reflect.Struct {
			saveAssociation(reflectValue)
		} else if indirectReflectValue.Kind() == reflect.Slice {
			for i := 0; i < indirectReflectValue.Len(); i++ {
				saveAssociation(indirectReflectValue.Index(i))
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
