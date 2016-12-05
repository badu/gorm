package gorm

import (
	"fmt"
	"strings"
)

//used in autoMigrate and createTable
func createJoinTable(scope *Scope, field *StructField) {
	if field.HasSetting(JOIN_TABLE_HANDLER) {
		var (
			ForeignDBNames               = field.GetSliceSetting(FOREIGN_DB_NAMES)
			ForeignFieldNames            = field.GetSliceSetting(FOREIGN_FIELD_NAMES)
			AssociationForeignFieldNames = field.GetSliceSetting(ASSOCIATION_FOREIGN_FIELD_NAMES)
			AssociationForeignDBNames    = field.GetSliceSetting(ASSOCIATION_FOREIGN_DB_NAMES)
			joinTableHandler             = field.JoinHandler()
		)
		joinTable := joinTableHandler.Table(scope.con)
		//because we're using it in a for, we're getting it once
		dialect := scope.con.parent.dialect

		if !dialect.HasTable(joinTable) {
			toScope := &Scope{Value: field.Interface()}

			var sqlTypes, primaryKeys []string
			for idx, fieldName := range ForeignFieldNames {
				if field, ok := scope.FieldByName(fieldName); ok {
					clonedField := field.clone()
					clonedField.UnsetIsPrimaryKey()
					//TODO : @Badu - document that you cannot use IS_JOINTABLE_FOREIGNKEY in conjunction with AUTO_INCREMENT
					clonedField.UnsetIsAutoIncrement()
					sqlTypes = append(sqlTypes, Quote(ForeignDBNames[idx], dialect)+" "+dialect.DataTypeOf(clonedField))
					primaryKeys = append(primaryKeys, Quote(ForeignDBNames[idx], dialect))
				}
			}

			for idx, fieldName := range AssociationForeignFieldNames {
				if field, ok := toScope.FieldByName(fieldName); ok {
					clonedField := field.clone()
					clonedField.UnsetIsPrimaryKey()
					//TODO : @Badu - document that you cannot use IS_JOINTABLE_FOREIGNKEY in conjunction with AUTO_INCREMENT
					clonedField.UnsetIsAutoIncrement()
					sqlTypes = append(sqlTypes, Quote(AssociationForeignDBNames[idx], dialect)+" "+dialect.DataTypeOf(clonedField))
					primaryKeys = append(primaryKeys, Quote(AssociationForeignDBNames[idx], dialect))
				}
			}
			//TODO : @Badu - for now, TABLE_OPT_SETTING is nowhere set - maybe it's a leftover
			// getTableOptions return the table options string or an empty string if the table options does not exist
			tableOptions, ok := scope.Get(TABLE_OPT_SETTING)
			if !ok {
				tableOptions = ""
			} else {
				tableOptions = tableOptions.(string)
			}

			scope.Err(newCon(scope.con).Exec(
				fmt.Sprintf(
					"CREATE TABLE %v (%v, PRIMARY KEY (%v)) %s",
					Quote(joinTable, dialect),
					strings.Join(sqlTypes, ","),
					strings.Join(primaryKeys, ","),
					tableOptions,
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
		if field.HasRelations() {
			createJoinTable(scope, field)
		}
	}

	if len(primaryKeys) > 0 && !primaryKeyInColumnType {
		primaryKeyStr = fmt.Sprintf(", PRIMARY KEY (%v)", strings.Join(primaryKeys, ","))
	}

	//TODO : @Badu - for now, TABLE_OPT_SETTING is nowhere set - maybe it's a leftover
	// getTableOptions return the table options string or an empty string if the table options does not exist
	tableOptions, ok := scope.Get(TABLE_OPT_SETTING)
	if !ok {
		tableOptions = ""
	} else {
		tableOptions = tableOptions.(string)
	}

	scope.Raw(
		fmt.Sprintf(
			"CREATE TABLE %v (%v %v) %s",
			QuotedTableName(scope),
			strings.Join(tags, ","),
			primaryKeyStr,
			tableOptions,
		),
	).Exec()

	autoIndex(scope)
}

//used in db.AddIndex and db.AddUniqueIndex and autoIndex
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
			QuotedTableName(scope),
			strings.Join(columns, ", "),
			scope.Search.whereSQL(scope),
		),
	).Exec()
}

//used in db.AutoMigrate
func autoMigrate(scope *Scope) {
	var (
		tableName       = scope.TableName()
		quotedTableName = QuotedTableName(scope)
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
			if field.HasRelations() {
				createJoinTable(scope, field)
			}
		}
		autoIndex(scope)
	}
}

//used in autoMigrate and createTable
func autoIndex(scope *Scope) {
	var indexes = map[string][]string{}
	var uniqueIndexes = map[string][]string{}

	for _, field := range scope.GetModelStruct().StructFields() {

		if field.HasSetting(INDEX) {
			names := strings.Split(field.GetStrSetting(INDEX), ",")

			for _, name := range names {
				if name == index || name == "" {
					name = fmt.Sprintf("idx_%v_%v", scope.TableName(), field.DBName)
				}
				indexes[name] = append(indexes[name], field.DBName)
			}
		}
		if field.HasSetting(UNIQUE_INDEX) {
			names := strings.Split(field.GetStrSetting(UNIQUE_INDEX), ",")
			for _, name := range names {
				if name == unique_index || name == "" {
					name = fmt.Sprintf("uix_%v_%v", scope.TableName(), field.DBName)
				}
				uniqueIndexes[name] = append(uniqueIndexes[name], field.DBName)
			}
		}
	}

	for name, columns := range indexes {
		addIndex(newCon(scope.con).Unscoped().NewScope(scope.Value), false, name, columns...)

	}

	for name, columns := range uniqueIndexes {
		addIndex(newCon(scope.con).Unscoped().NewScope(scope.Value), true, name, columns...)
	}
}
