package gorm

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

//implementation of JoinTableHandlerInterface
// SourceForeignKeys return source foreign keys
func (s *JoinTableHandler) SourceForeignKeys() []JoinTableForeignKey {
	return s.Source.ForeignKeys
}

//implementation of JoinTableHandlerInterface
// DestinationForeignKeys return destination foreign keys
func (s *JoinTableHandler) DestinationForeignKeys() []JoinTableForeignKey {
	return s.Destination.ForeignKeys
}

//implementation of JoinTableHandlerInterface
// Setup initialize a default join table handler
func (s *JoinTableHandler) Setup(
	relationship *Relationship,
	source reflect.Type,
	destination reflect.Type) {

	s.Source = JoinTableSource{ModelType: source}
	for idx, dbName := range relationship.ForeignFieldNames {
		s.Source.ForeignKeys = append(s.Source.ForeignKeys, JoinTableForeignKey{
			DBName:            relationship.ForeignDBNames[idx],
			AssociationDBName: dbName,
		})
	}

	s.Destination = JoinTableSource{ModelType: destination}
	for idx, dbName := range relationship.AssociationForeignFieldNames {
		s.Destination.ForeignKeys = append(s.Destination.ForeignKeys, JoinTableForeignKey{
			DBName:            relationship.AssociationForeignDBNames[idx],
			AssociationDBName: dbName,
		})
	}
}

//implementation of JoinTableHandlerInterface
// Table return join table's table name
func (s JoinTableHandler) Table(db *DBCon) string {
	return s.TableName
}

func (s JoinTableHandler) getSearchMap(con *DBCon, sources ...interface{}) map[string]interface{} {
	values := map[string]interface{}{}

	for _, source := range sources {
		scope := con.NewScope(source)
		modelType := scope.GetModelStruct().ModelType

		if s.Source.ModelType == modelType {
			for _, foreignKey := range s.Source.ForeignKeys {
				if field, ok := scope.FieldByName(foreignKey.AssociationDBName); ok {
					values[foreignKey.DBName] = field.Value.Interface()
				}
			}
		} else if s.Destination.ModelType == modelType {
			for _, foreignKey := range s.Destination.ForeignKeys {
				if field, ok := scope.FieldByName(foreignKey.AssociationDBName); ok {
					values[foreignKey.DBName] = field.Value.Interface()
				}
			}
		}
	}
	return values
}

//implementation of JoinTableHandlerInterface
// Add create relationship in join table for source and destination
func (s JoinTableHandler) Add(handler JoinTableHandlerInterface, con *DBCon, source interface{}, destination interface{}) error {
	scope := con.NewScope("")
	searchMap := s.getSearchMap(con, source, destination)

	var assignColumns, binVars, conditions []string
	var values []interface{}
	//because we're using it in a for, we're getting it once
	dialect := scope.con.parent.dialect
	for key, value := range searchMap {
		assignColumns = append(assignColumns, Quote(key, dialect))
		binVars = append(binVars, `?`)
		conditions = append(conditions, fmt.Sprintf("%v = ?", Quote(key, dialect)))
		values = append(values, value)
	}

	for _, value := range values {
		values = append(values, value)
	}

	quotedTable := Quote(handler.Table(con), dialect)
	sql := fmt.Sprintf(
		"INSERT INTO %v (%v) SELECT %v %v WHERE NOT EXISTS (SELECT * FROM %v WHERE %v)",
		quotedTable,
		strings.Join(assignColumns, ","),
		strings.Join(binVars, ","),
		con.parent.dialect.SelectFromDummyTable(),
		quotedTable,
		strings.Join(conditions, " AND "),
	)

	return con.Exec(sql, values...).Error
}

//implementation of JoinTableHandlerInterface
func (s *JoinTableHandler) SetTable(name string) {
	s.TableName = name
}

//implementation of JoinTableHandlerInterface
// Delete delete relationship in join table for sources
func (s JoinTableHandler) Delete(handler JoinTableHandlerInterface, con *DBCon, sources ...interface{}) error {
	var (
		scope      = con.NewScope(nil)
		conditions []string
		values     []interface{}
		//because we're using it in a for, we're getting it once
		scopeDialect = scope.con.parent.dialect
	)

	for key, value := range s.getSearchMap(con, sources...) {
		conditions = append(conditions, fmt.Sprintf("%v = ?", Quote(key, scopeDialect)))
		values = append(values, value)
	}

	return con.Table(handler.Table(con)).Where(strings.Join(conditions, " AND "), values...).Delete("").Error
}

//implementation of JoinTableHandlerInterface
// JoinWith query with `Join` conditions
func (s JoinTableHandler) JoinWith(handler JoinTableHandlerInterface, con *DBCon, source interface{}) *DBCon {
	var (
		scope           = con.NewScope(source)
		tableName       = handler.Table(con)
		dialect         = scope.con.parent.dialect
		quotedTableName = Quote(tableName, dialect)
		joinConditions  []string
		values          []interface{}
	)

	if s.Source.ModelType == scope.GetModelStruct().ModelType {
		destinationTableName := con.NewScope(reflect.New(s.Destination.ModelType).Interface()).QuotedTableName()
		for _, foreignKey := range s.Destination.ForeignKeys {
			joinConditions = append(joinConditions,
				fmt.Sprintf(
					"%v.%v = %v.%v",
					quotedTableName,
					Quote(foreignKey.DBName, dialect),
					destinationTableName,
					Quote(foreignKey.AssociationDBName, dialect)))
		}

		var foreignDBNames StrSlice
		var foreignFieldNames StrSlice

		for _, foreignKey := range s.Source.ForeignKeys {
			foreignDBNames = append(foreignDBNames, foreignKey.DBName)
			if field, ok := scope.FieldByName(foreignKey.AssociationDBName); ok {
				foreignFieldNames.add(field.GetStructName())
			}
		}

		foreignFieldValues := getColumnAsArray(foreignFieldNames, scope.Value)

		var condString string
		if len(foreignFieldValues) > 0 {
			var quotedForeignDBNames []string
			for _, dbName := range foreignDBNames {
				quotedForeignDBNames = append(quotedForeignDBNames, tableName+"."+dbName)
			}

			condString = fmt.Sprintf(
				"%v IN (%v)",
				toQueryCondition(quotedForeignDBNames, dialect),
				toQueryMarks(foreignFieldValues),
			)

			keys := getColumnAsArray(foreignFieldNames, scope.Value)
			values = append(values, toQueryValues(keys))
		} else {
			condString = fmt.Sprint("1 <> 1")
		}

		return con.Joins(
			fmt.Sprintf("INNER JOIN %v ON %v",
				quotedTableName,
				strings.Join(joinConditions, " AND "))).
			Where(condString, toQueryValues(foreignFieldValues)...)
	}

	con.Error = errors.New("JOIN : wrong source type for join table handler")
	return con
}
