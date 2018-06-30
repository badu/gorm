package gorm

import (
	"fmt"
	"reflect"
)

//implementation of JoinTableHandlerInterface
// SourceForeignKeys return source foreign keys
func (h *JoinTableHandler) SourceForeignKeys() []JoinTableForeignKey {
	return h.Source.ForeignKeys
}

//implementation of JoinTableHandlerInterface
// DestinationForeignKeys return destination foreign keys
func (h *JoinTableHandler) DestinationForeignKeys() []JoinTableForeignKey {
	return h.Destination.ForeignKeys
}

//implementation of JoinTableHandlerInterface
// Setup initialize a default join table handler
func (h *JoinTableHandler) Setup(
	field *StructField,
	source reflect.Type,
	destination reflect.Type) {

	var (
		ForeignDBNames               = field.GetForeignDBNames()
		ForeignFieldNames            = field.GetForeignFieldNames()
		AssociationForeignFieldNames = field.GetAssociationForeignFieldNames()
		AssociationForeignDBNames    = field.GetAssociationDBNames()
	)

	h.Source = JoinTableInfo{ModelType: source}
	for idx, dbName := range ForeignFieldNames {
		h.Source.ForeignKeys = append(h.Source.ForeignKeys,
			JoinTableForeignKey{
				DBName:            ForeignDBNames[idx],
				AssociationDBName: dbName,
			},
		)
	}

	h.Destination = JoinTableInfo{ModelType: destination}
	for idx, dbName := range AssociationForeignFieldNames {
		h.Destination.ForeignKeys = append(
			h.Destination.ForeignKeys,
			JoinTableForeignKey{
				DBName:            AssociationForeignDBNames[idx],
				AssociationDBName: dbName,
			},
		)
	}
}

//implementation of JoinTableHandlerInterface
// Table return join table's table name
func (h JoinTableHandler) Table(db *DBCon) string {
	return h.TableName
}

//implementation of JoinTableHandlerInterface
// Add create relationship in join table for source and destination
func (h JoinTableHandler) Add(handler JoinTableHandlerInterface, con *DBCon, source interface{}, destination interface{}) error {
	var (
		dialect                            = con.parent.dialect
		searchMap                          = map[string]interface{}{}
		assignColumns, binVars, conditions string
		values                             []interface{}
	)

	for _, src := range []interface{}{source, destination} {
		scp := con.NewScope(src)
		if h.Source.ModelType == scp.GetModelStruct().ModelType {
			for _, foreignKey := range h.Source.ForeignKeys {
				if field, ok := scp.FieldByName(foreignKey.AssociationDBName); ok {
					searchMap[foreignKey.DBName] = field.Value.Interface()
				}
			}
		} else if h.Destination.ModelType == scp.GetModelStruct().ModelType {
			for _, foreignKey := range h.Destination.ForeignKeys {
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
		assignColumns += con.quote(key)
		if binVars == "" {
			binVars = "?"
		} else {
			binVars += ",?"
		}
		if conditions != "" {
			conditions += " AND "
		}
		conditions += fmt.Sprintf("%v = ?", con.quote(key))
		values = append(values, value)
	}

	for _, value := range values {
		values = append(values, value)
	}

	sql := fmt.Sprintf(
		"INSERT INTO %v (%v) SELECT %v %v WHERE NOT EXISTS (SELECT * FROM %v WHERE %v)",
		con.quote(handler.Table(con)),
		assignColumns,
		binVars,
		dialect.SelectFromDummyTable(),
		con.quote(handler.Table(con)),
		conditions,
	)
	return con.Exec(sql, values...).Error
}

//implementation of JoinTableHandlerInterface
func (h *JoinTableHandler) SetTable(name string) {
	h.TableName = name
}

//implementation of JoinTableHandlerInterface
// Delete delete relationship in join table for sources
func (h JoinTableHandler) Delete(handler JoinTableHandlerInterface, con *DBCon) error {
	return con.Table(handler.Table(con)).Delete("").Error
}

//implementation of JoinTableHandlerInterface
// JoinWith query with `Join` conditions
func (h JoinTableHandler) JoinWith(handler JoinTableHandlerInterface, con *DBCon, source interface{}) *DBCon {
	var (
		scope           = con.NewScope(source)
		tableName       = handler.Table(con)
		quotedTableName = con.quote(tableName)
		joinConditions  string
	)

	if h.Source.ModelType == scope.GetModelStruct().ModelType {
		destinationTableName := con.NewScope(reflect.New(h.Destination.ModelType).Interface()).quotedTableName()

		for _, foreignKey := range h.Destination.ForeignKeys {
			if joinConditions != "" {
				joinConditions += " AND "
			}

			joinConditions += fmt.Sprintf(
				"%v.%v = %v.%v",
				quotedTableName,
				con.quote(foreignKey.DBName),
				destinationTableName,
				con.quote(foreignKey.AssociationDBName),
			)
		}

		var foreignDBNames StrSlice
		var foreignFieldNames StrSlice

		for _, foreignKey := range h.Source.ForeignKeys {
			foreignDBNames = append(foreignDBNames, foreignKey.DBName)
			if field, ok := scope.FieldByName(foreignKey.AssociationDBName); ok {
				foreignFieldNames.add(field.StructName)
			}
		}

		foreignFieldValues := scope.getColumnAsArray(foreignFieldNames)

		var condString string
		if len(foreignFieldValues) > 0 {
			var quotedForeignDBNames []string
			for _, dbName := range foreignDBNames {
				quotedForeignDBNames = append(quotedForeignDBNames, tableName+"."+dbName)
			}

			condString = fmt.Sprintf(
				"%v IN (%v)",
				scope.toQueryCondition(quotedForeignDBNames),
				toQueryMarks(foreignFieldValues),
			)

		} else {
			condString = fmt.Sprint("1 <> 1")
		}

		return con.Joins(
			fmt.Sprintf("INNER JOIN %v ON %v", quotedTableName, joinConditions)).
			Where(condString,
				toQueryValues(foreignFieldValues)...,
			)
	}

	con.Error = fmt.Errorf("JOIN : wrong source type for join table handler for %v", h.Source.ModelType)
	return con
}

//for debugging
func (h *JoinTableHandler) GetHandlerStruct() *JoinTableHandler {
	return h
}

//implementation of Stringer
func (h JoinTableHandler) String() string {
	var collector Collector

	collector.add("\tTable name : %s\n", h.TableName)
	collector.add("\t\tDestination model type : %v\n", h.Destination.ModelType)
	for _, fk := range h.Destination.ForeignKeys {
		collector.add("\t\t\tDestination FK : %s -> %s\n", fk.DBName, fk.AssociationDBName)
	}
	collector.add("\t\tSource model type : %v\n", h.Source.ModelType)
	for _, fk := range h.Source.ForeignKeys {
		collector.add("\t\t\tSource FK : %s -> %s\n", fk.DBName, fk.AssociationDBName)
	}

	return collector.String()
}
