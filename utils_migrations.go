package gorm

import (
	"fmt"
	"strings"
)

//used in autoMigrate and createTable
func createJoinTable(scope *Scope, field *StructField) {
	//ignore fields without join table handler
	if !field.HasSetting(setJoinTableHandler) {
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
		tableOptions, ok := scope.Get(gormSettingTableOpt)
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
					scope.Err(fmt.Errorf("ERROR : %q doesn't have a field named %q", destinationValue.ModelType.Name(), fk.AssociationDBName))
				}
			}
		} else {
			scope.Err(fmt.Errorf("ERROR : Could not find %s in ModelStructsMap", handler.Destination.ModelType.Name()))
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
					scope.Err(fmt.Errorf("ERROR : %q doesn't have a field named %q", sourceValue.ModelType.Name(), fk.AssociationDBName))
				}
			}
		} else {
			scope.Err(fmt.Errorf("ERROR : Could not find %s in ModelStructsMap", handler.Source.ModelType.Name()))
		}
		creationSQL := fmt.Sprintf(
			"CREATE TABLE %v (%v, PRIMARY KEY (%v)) %s",
			scope.con.quote(tableName),
			sqlTypes,
			primaryKeys,
			tableOptions,
		)
		scope.Err(scope.con.empty().Exec(creationSQL).Error)
	} /**
	} else {
		destinationValue := scope.con.parent.modelsStructMap.get(handler.Destination.ModelType)
		if destinationValue != nil {
			for _, fk := range handler.Destination.ForeignKeys {
				theField, ok := destinationValue.FieldByName(fk.AssociationDBName, scope.con.parent)
				if ok {
					sqlTag := dialect.DataTypeOf(theField)
					if !dialect.HasColumn(tableName, theField.DBName) {
						scope.Warn(fmt.Printf(
							"ALTER TABLE %v ADD %v %v;\n",
							scope.quotedTableName(),
							scope.con.quote(field.DBName),
							sqlTag))
					}
				} else {
					scope.Err(fmt.Errorf("ERROR : %q doesn't have a field named %q", destinationValue.ModelType.Name(), fk.AssociationDBName))
				}
			}
		} else {
			scope.Err(fmt.Errorf("ERROR : Could not find %s in ModelStructsMap", handler.Destination.ModelType.Name()))
		}
		sourceValue := scope.con.parent.modelsStructMap.get(handler.Source.ModelType)
		if sourceValue != nil {
			for _, fk := range handler.Source.ForeignKeys {
				theField, ok := sourceValue.FieldByName(fk.AssociationDBName, scope.con.parent)
				if ok {
					sqlTag := dialect.DataTypeOf(theField)
					if !dialect.HasColumn(tableName, theField.DBName) {
						scope.Warn(fmt.Printf(
							"ALTER TABLE %v ADD %v %v;\n",
							scope.quotedTableName(),
							scope.con.quote(field.DBName),
							sqlTag))
					}
				} else {
					scope.Err(fmt.Errorf("ERROR : %q doesn't have a field named %q", sourceValue.ModelType.Name(), fk.AssociationDBName))
				}
			}
		} else {
			scope.Err(fmt.Errorf("ERROR : Could not find %s in ModelStructsMap", handler.Source.ModelType.Name()))
		}

	}
	**/
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
			if strings.Contains(strings.ToLower(sqlTag), strPrimaryKey) {
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
	tableOptions, ok := scope.Get(gormSettingTableOpt)
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
	)

	if scope.con.parent.dialect.HasIndex(scope.TableName(), indexName) {
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

		if field.HasSetting(setIndex) {
			names := strings.Split(field.GetStrSetting(setIndex), ",")

			for _, name := range names {
				if name == tagIndex || name == "" {
					name = fmt.Sprintf("idx_%v_%v", scope.TableName(), field.DBName)
				}
				indexes[name] = append(indexes[name], field.DBName)
			}
		}
		if field.HasSetting(setUniqueIndex) {
			names := strings.Split(field.GetStrSetting(setUniqueIndex), ",")
			for _, name := range names {
				if name == tagUniqueIndex || name == "" {
					name = fmt.Sprintf("uix_%v_%v", scope.TableName(), field.DBName)
				}
				uniqueIndexes[name] = append(uniqueIndexes[name], field.DBName)
			}
		}
	}

	for name, columns := range indexes {
		addIndex(scope.con.empty().Unscoped().NewScope(scope.Value), false, name, columns...)

	}

	for name, columns := range uniqueIndexes {
		addIndex(scope.con.empty().Unscoped().NewScope(scope.Value), true, name, columns...)
	}
}
