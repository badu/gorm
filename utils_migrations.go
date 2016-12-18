package gorm

import (
	"fmt"
	"strings"
)

//used in autoMigrate and createTable
func createJoinTable(scope *Scope, field *StructField) {
	//ignore fields without join table handler
	if !field.HasSetting(set_join_table_handler) {
		return
	}
	var (
		joinHandler           = field.JoinHandler()
		dialect               = scope.con.parent.dialect
		handler               = joinHandler.GetHandlerStruct()
		tableName             = handler.Table(scope.con)
		sqlTypes, primaryKeys string
	)

	if !dialect.HasTable(tableName) {
		// getTableOptions return the table options string or an empty string if the table options does not exist
		// It's settable by end user
		tableOptions, ok := scope.Get(gorm_setting_table_opt)
		if !ok {
			tableOptions = ""
		} else {
			tableOptions = tableOptions.(string)
		}
		destinationValue := scope.con.parent.modelsStructMap.get(handler.Destination.ModelType)
		if destinationValue != nil {
			for _, fk := range handler.Destination.ForeignKeys {
				if sqlTypes != "" {
					sqlTypes += ","
				}
				if primaryKeys != "" {
					primaryKeys += ","
				}
				theField, ok := destinationValue.FieldByName(fk.AssociationDBName, scope.con.parent)
				if ok {
					//clone the field so we can unset primary key and autoincrement values
					clone := theField.clone()
					clone.UnsetIsPrimaryKey()
					clone.UnsetIsAutoIncrement()
					sqlTypes += scope.con.quote(fk.DBName) + " " + dialect.DataTypeOf(clone)
					primaryKeys += scope.con.quote(fk.DBName)
				} else {
					errMsg := fmt.Errorf("ERROR : %q doesn't have a field named %q", destinationValue.ModelType.Name(), fk.AssociationDBName)
					fmt.Println(errMsg)
					scope.Err(errMsg)
				}
			}
		} else {
			errMsg := fmt.Errorf("ERROR : Could not find %s in ModelStructsMap", handler.Destination.ModelType.Name())
			fmt.Println(errMsg)
			scope.Err(errMsg)
		}
		sourceValue := scope.con.parent.modelsStructMap.get(handler.Source.ModelType)
		if sourceValue != nil {
			for _, fk := range handler.Source.ForeignKeys {
				if sqlTypes != "" {
					sqlTypes += ","
				}
				if primaryKeys != "" {
					primaryKeys += ","
				}
				theField, ok := sourceValue.FieldByName(fk.AssociationDBName, scope.con.parent)
				if ok {
					//clone the field so we can unset primary key and autoincrement values
					clone := theField.clone()
					clone.UnsetIsPrimaryKey()
					clone.UnsetIsAutoIncrement()
					sqlTypes += scope.con.quote(fk.DBName) + " " + dialect.DataTypeOf(clone)
					primaryKeys += scope.con.quote(fk.DBName)
				} else {
					errMsg := fmt.Errorf("ERROR : %q doesn't have a field named %q", sourceValue.ModelType.Name(), fk.AssociationDBName)
					fmt.Println(errMsg)
					scope.Err(errMsg)
				}
			}
		} else {
			errMsg := fmt.Errorf("ERROR : Could not find %s in ModelStructsMap", handler.Source.ModelType.Name())
			fmt.Println(errMsg)
			scope.Err(errMsg)
		}
		creationSQL := fmt.Sprintf(
			"CREATE TABLE %v (%v, PRIMARY KEY (%v)) %s",
			scope.con.quote(tableName),
			sqlTypes,
			primaryKeys,
			tableOptions,
		)
		scope.Err(newCon(scope.con).Exec(creationSQL).Error)
	} else {
		//TODO : make update - see below
		/**
		var (
			quotedTableName = QuotedTableName(scope)
		)

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
		*/
	}

}

//used in db.CreateTable and autoMigrate
func createTable(scope *Scope) {
	var (
		tags                   string
		primaryKeys            string
		primaryKeyInColumnType = false
		primaryKeyStr          string
		//because we're using it in a for, we're getting it once
		dialect   = scope.con.parent.dialect
		tableName = scope.quotedTableName()
	)

	for _, field := range scope.GetModelStruct().StructFields() {
		if field.IsNormal() {
			sqlTag := dialect.DataTypeOf(field)
			// Check if the primary key constraint was specified as
			// part of the column type. If so, we can only support
			// one column as the primary key.
			if strings.Contains(strings.ToLower(sqlTag), str_primary_key) {
				primaryKeyInColumnType = true
			}
			if tags != "" {
				tags += ","
			}
			tags += scope.con.quote(field.DBName) + " " + sqlTag
		}

		if field.IsPrimaryKey() {
			if primaryKeys != "" {
				primaryKeys += ","
			}
			primaryKeys += scope.con.quote(field.DBName)
		}
		if field.HasRelations() {
			createJoinTable(scope, field)
		}
	}

	if primaryKeys != "" && !primaryKeyInColumnType {
		primaryKeyStr = ", PRIMARY KEY (" + primaryKeys + ")"
	}

	// getTableOptions return the table options string or an empty string if the table options does not exist
	// It's settable by end user
	tableOptions, ok := scope.Get(gorm_setting_table_opt)
	if !ok {
		tableOptions = ""
	} else {
		tableOptions = tableOptions.(string)
	}

	scope.Raw(
		fmt.Sprintf(
			"CREATE TABLE %v (%v %v) %s",
			tableName,
			tags,
			primaryKeyStr,
			tableOptions,
		),
	).Exec()

	autoIndex(scope)
}

//used in db.AddIndex and db.AddUniqueIndex and autoIndex
func addIndex(scope *Scope, unique bool, indexName string, column ...string) {
	var (
		columns string
		dialect = scope.con.parent.dialect
	)

	if dialect.HasIndex(scope.TableName(), indexName) {
		return
	}

	for _, name := range column {
		if columns != "" {
			columns += ","
		}
		columns += scope.quoteIfPossible(name)
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
			scope.quotedTableName(),
			columns,
			scope.Search.whereSQL(scope),
		),
	).Exec()
}

//used in db.AutoMigrate
func autoMigrate(scope *Scope) {
	var (
		tableName = scope.TableName()
		dialect   = scope.con.parent.dialect
	)

	if !dialect.HasTable(tableName) {
		createTable(scope)
	} else {
		var (
			quotedTableName = scope.quotedTableName()
		)

		for _, field := range scope.GetModelStruct().StructFields() {
			if !dialect.HasColumn(tableName, field.DBName) {
				if field.IsNormal() {
					sqlTag := dialect.DataTypeOf(field)
					scope.Raw(
						fmt.Sprintf(
							"ALTER TABLE %v ADD %v %v;",
							quotedTableName,
							scope.con.quote(field.DBName),
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

		if field.HasSetting(set_index) {
			names := strings.Split(field.GetStrSetting(set_index), ",")

			for _, name := range names {
				if name == tag_index || name == "" {
					name = fmt.Sprintf("idx_%v_%v", scope.TableName(), field.DBName)
				}
				indexes[name] = append(indexes[name], field.DBName)
			}
		}
		if field.HasSetting(set_unique_index) {
			names := strings.Split(field.GetStrSetting(set_unique_index), ",")
			for _, name := range names {
				if name == tag_unique_index || name == "" {
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
