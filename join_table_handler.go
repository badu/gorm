package gorm

import (
	"errors"
	"fmt"
	"reflect"
)

//implementation of JoinTableHandlerInterface
// SourceForeignKeys return source foreign keys
func (jth *JoinTableHandler) SourceForeignKeys() []JoinTableForeignKey {
	return jth.Source.ForeignKeys
}

//implementation of JoinTableHandlerInterface
// DestinationForeignKeys return destination foreign keys
func (jth *JoinTableHandler) DestinationForeignKeys() []JoinTableForeignKey {
	return jth.Destination.ForeignKeys
}

//implementation of JoinTableHandlerInterface
// Setup initialize a default join table handler
func (jth *JoinTableHandler) Setup(
	field *StructField,
	source reflect.Type,
	destination reflect.Type) {

	var (
		ForeignDBNames               = field.GetSliceSetting(FOREIGN_DB_NAMES)
		ForeignFieldNames            = field.GetSliceSetting(FOREIGN_FIELD_NAMES)
		AssociationForeignFieldNames = field.GetSliceSetting(ASSOCIATION_FOREIGN_FIELD_NAMES)
		AssociationForeignDBNames    = field.GetSliceSetting(ASSOCIATION_FOREIGN_DB_NAMES)
	)

	jth.Source = JoinTableSource{ModelType: source}
	for idx, dbName := range ForeignFieldNames {
		jth.Source.ForeignKeys = append(jth.Source.ForeignKeys,
			JoinTableForeignKey{
				DBName:            ForeignDBNames[idx],
				AssociationDBName: dbName,
			},
		)
	}

	jth.Destination = JoinTableSource{ModelType: destination}
	for idx, dbName := range AssociationForeignFieldNames {
		jth.Destination.ForeignKeys = append(
			jth.Destination.ForeignKeys,
			JoinTableForeignKey{
				DBName:            AssociationForeignDBNames[idx],
				AssociationDBName: dbName,
			},
		)
	}
}

//implementation of JoinTableHandlerInterface
// Table return join table's table name
func (jth JoinTableHandler) Table(db *DBCon) string {
	return jth.TableName
}

//implementation of JoinTableHandlerInterface
// Add create relationship in join table for source and destination
func (jth JoinTableHandler) Add(handler JoinTableHandlerInterface, con *DBCon, source interface{}, destination interface{}) error {
	var (
		dialect                            = con.parent.dialect
		searchMap                          = map[string]interface{}{}
		assignColumns, binVars, conditions string
		values                             []interface{}
	)

	for _, src := range []interface{}{source, destination} {
		scp := con.NewScope(src)
		if jth.Source.ModelType == scp.GetModelStruct().ModelType {
			for _, foreignKey := range jth.Source.ForeignKeys {
				if field, ok := scp.FieldByName(foreignKey.AssociationDBName); ok {
					searchMap[foreignKey.DBName] = field.Value.Interface()
				}
			}
		} else if jth.Destination.ModelType == scp.GetModelStruct().ModelType {
			for _, foreignKey := range jth.Destination.ForeignKeys {
				if field, ok := scp.FieldByName(foreignKey.AssociationDBName); ok {
					searchMap[foreignKey.DBName] = field.Value.Interface()
				}
			}
		}
	}

	for key, value := range searchMap {
		if assignColumns != "" {
			assignColumns += ","
		}
		assignColumns += Quote(key, dialect)
		if binVars == "" {
			binVars = "?"
		} else {
			binVars += ",?"
		}
		if conditions != "" {
			conditions += " AND "
		}
		conditions += fmt.Sprintf("%v = ?", Quote(key, dialect))
		values = append(values, value)
	}

	for _, value := range values {
		values = append(values, value)
	}

	sql := fmt.Sprintf(
		"INSERT INTO %v (%v) SELECT %v %v WHERE NOT EXISTS (SELECT * FROM %v WHERE %v)",
		Quote(handler.Table(con), dialect),
		assignColumns,
		binVars,
		dialect.SelectFromDummyTable(),
		Quote(handler.Table(con), dialect),
		conditions,
	)
	//fmt.Printf("\nSQL : %s\n", sql)
	return con.Exec(sql, values...).Error
}

//implementation of JoinTableHandlerInterface
func (jth *JoinTableHandler) SetTable(name string) {
	jth.TableName = name
}

//implementation of JoinTableHandlerInterface
// Delete delete relationship in join table for sources
// TODO : @Badu - it's a lie : did nothing with sources, because sources was Relationship
func (jth JoinTableHandler) Delete(handler JoinTableHandlerInterface, con *DBCon) error {
	return con.Table(handler.Table(con)).Delete("").Error
}

//implementation of JoinTableHandlerInterface
// JoinWith query with `Join` conditions
func (jth JoinTableHandler) JoinWith(handler JoinTableHandlerInterface, con *DBCon, source interface{}) *DBCon {
	var (
		scope           = con.NewScope(source)
		tableName       = handler.Table(con)
		dialect         = scope.con.parent.dialect
		quotedTableName = Quote(tableName, dialect)
		joinConditions  string
	)

	if jth.Source.ModelType == scope.GetModelStruct().ModelType {
		destinationTableName := QuotedTableName(
			con.NewScope(
				reflect.New(jth.Destination.ModelType).Interface(),
			),
		)

		for _, foreignKey := range jth.Destination.ForeignKeys {
			if joinConditions != "" {
				joinConditions += " AND "
			}

			joinConditions += fmt.Sprintf(
				"%v.%v = %v.%v",
				quotedTableName,
				Quote(foreignKey.DBName, dialect),
				destinationTableName,
				Quote(foreignKey.AssociationDBName, dialect),
			)
		}

		var foreignDBNames StrSlice
		var foreignFieldNames StrSlice

		for _, foreignKey := range jth.Source.ForeignKeys {
			foreignDBNames = append(foreignDBNames, foreignKey.DBName)
			if field, ok := scope.FieldByName(foreignKey.AssociationDBName); ok {
				foreignFieldNames.add(field.StructName)
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

		} else {
			condString = fmt.Sprint("1 <> 1")
		}

		return con.Joins(
			fmt.Sprintf("INNER JOIN %v ON %v", quotedTableName, joinConditions)).
			Where(  condString,
				toQueryValues(foreignFieldValues)...,
			)
	}

	con.Error = errors.New("JOIN : wrong source type for join table handler")
	return con
}
