package gorm

import (
	"fmt"
	"strings"
)

//used in autoMigrate and createTable
func createJoinTable(scope *Scope, field *StructField) {
	if relationship := field.Relationship; relationship != nil && relationship.JoinTableHandler != nil {
		joinTableHandler := relationship.JoinTableHandler
		joinTable := joinTableHandler.Table(scope.con)
		//because we're using it in a for, we're getting it once
		dialect := scope.con.parent.dialect

		if !dialect.HasTable(joinTable) {
			toScope := &Scope{Value: field.Interface()}

			var sqlTypes, primaryKeys []string
			for idx, fieldName := range relationship.ForeignFieldNames {
				if field, ok := scope.FieldByName(fieldName); ok {
					clonedField := field.clone()
					clonedField.unsetFlag(IS_PRIMARYKEY)
					//TODO : @Badu - document that you cannot use IS_JOINTABLE_FOREIGNKEY in conjunction with AUTO_INCREMENT
					clonedField.SetJoinTableFK("true")
					clonedField.UnsetIsAutoIncrement()
					sqlTypes = append(sqlTypes, Quote(relationship.ForeignDBNames[idx], dialect)+" "+dialect.DataTypeOf(clonedField))
					primaryKeys = append(primaryKeys, Quote(relationship.ForeignDBNames[idx], dialect))
				}
			}

			for idx, fieldName := range relationship.AssociationForeignFieldNames {
				if field, ok := toScope.FieldByName(fieldName); ok {
					clonedField := field.clone()
					clonedField.unsetFlag(IS_PRIMARYKEY)
					//TODO : @Badu - document that you cannot use IS_JOINTABLE_FOREIGNKEY in conjunction with AUTO_INCREMENT
					clonedField.SetJoinTableFK("true")
					clonedField.UnsetIsAutoIncrement()
					sqlTypes = append(sqlTypes, Quote(relationship.AssociationForeignDBNames[idx], dialect)+" "+dialect.DataTypeOf(clonedField))
					primaryKeys = append(primaryKeys, Quote(relationship.AssociationForeignDBNames[idx], dialect))
				}
			}

			scope.Err(newCon(scope.con).Exec(
				fmt.Sprintf(
					"CREATE TABLE %v (%v, PRIMARY KEY (%v)) %s",
					Quote(joinTable, dialect),
					strings.Join(sqlTypes, ","),
					strings.Join(primaryKeys, ","),
					scope.getTableOptions(),
				),
			).Error)
		}
		newCon(scope.con).Table(joinTable).AutoMigrate(joinTableHandler)
	}
}

//used in db.CreateTable and autoMigrate
func createTable(scope *Scope) {
	var (
		tags                   []string
		primaryKeys            []string
		primaryKeyInColumnType = false
		primaryKeyStr          string
		//because we're using it in a for, we're getting it once
		dialect = scope.con.parent.dialect
	)

	for _, field := range scope.GetModelStruct().StructFields() {
		if field.IsNormal() {
			sqlTag := dialect.DataTypeOf(field)
			// Check if the primary key constraint was specified as
			// part of the column type. If so, we can only support
			// one column as the primary key.
			//TODO : @Badu - boiler plate string
			if strings.Contains(strings.ToLower(sqlTag), "primary key") {
				primaryKeyInColumnType = true
			}
			tags = append(tags, Quote(field.DBName, dialect)+" "+sqlTag)
		}

		if field.IsPrimaryKey() {
			primaryKeys = append(primaryKeys, Quote(field.DBName, dialect))
		}

		createJoinTable(scope, field)
	}

	if len(primaryKeys) > 0 && !primaryKeyInColumnType {
		primaryKeyStr = fmt.Sprintf(", PRIMARY KEY (%v)", strings.Join(primaryKeys, ","))
	}

	scope.Raw(
		fmt.Sprintf(
			"CREATE TABLE %v (%v %v) %s",
			scope.QuotedTableName(),
			strings.Join(tags, ","),
			primaryKeyStr,
			scope.getTableOptions(),
		),
	).Exec()

	autoIndex(scope)
}

//used in db.AddIndex and db.AddUniqueIndex
func addIndex(scope *Scope, unique bool, indexName string, column ...string) {
	var (
		columns []string
		dialect = scope.con.parent.dialect
	)

	if dialect.HasIndex(scope.TableName(), indexName) {
		return
	}

	for _, name := range column {
		columns = append(columns, QuoteIfPossible(name, dialect))
	}

	sqlCreate := "CREATE INDEX"
	if unique {
		sqlCreate = "CREATE UNIQUE INDEX"
	}

	scope.Raw(
		fmt.Sprintf(
			"%s %v ON %v(%v) %v",
			sqlCreate,
			indexName,
			scope.QuotedTableName(),
			strings.Join(columns, ", "),
			scope.Search.whereSQL(scope),
		),
	).Exec()
}

//used in db.AutoMigrate
func autoMigrate(scope *Scope) {
	var (
		tableName       = scope.TableName()
		quotedTableName = scope.QuotedTableName()
		//because we're using it in a for, we're getting it once
		dialect = scope.con.parent.dialect
	)

	if !dialect.HasTable(tableName) {
		createTable(scope)
	} else {

		for _, field := range scope.GetModelStruct().StructFields() {
			if !dialect.HasColumn(tableName, field.DBName) {
				if field.IsNormal() {
					sqlTag := dialect.DataTypeOf(field)
					scope.Raw(
						fmt.Sprintf(
							"ALTER TABLE %v ADD %v %v;",
							quotedTableName,
							Quote(field.DBName, dialect),
							sqlTag),
					).Exec()
				}
			}
			createJoinTable(scope, field)
		}
		autoIndex(scope)
	}
}

//used in autoMigrate and createTable
func autoIndex(scope *Scope) {
	var indexes = map[string][]string{}
	var uniqueIndexes = map[string][]string{}

	for _, field := range scope.GetModelStruct().StructFields() {
		if name := field.GetSetting(INDEX); name != "" {
			names := strings.Split(name, ",")

			for _, name := range names {
				if name == "INDEX" || name == "" {
					name = fmt.Sprintf("idx_%v_%v", scope.TableName(), field.DBName)
				}
				indexes[name] = append(indexes[name], field.DBName)
			}
		}

		if name := field.GetSetting(UNIQUE_INDEX); name != "" {
			names := strings.Split(name, ",")

			for _, name := range names {
				if name == "UNIQUE_INDEX" || name == "" {
					name = fmt.Sprintf("uix_%v_%v", scope.TableName(), field.DBName)
				}
				uniqueIndexes[name] = append(uniqueIndexes[name], field.DBName)
			}
		}
	}

	for name, columns := range indexes {
		newCon(scope.con).Model(scope.Value).AddIndex(name, columns...)
	}

	for name, columns := range uniqueIndexes {
		newCon(scope.con).Model(scope.Value).AddUniqueIndex(name, columns...)
	}
}
