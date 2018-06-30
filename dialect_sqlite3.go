package gorm

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

const (
	SqliteDialectName = "sqlite3"
	SqliteBool        = "bool"
	SqliteInteger     = "integer"
	SqlitePk          = "integer primary key autoincrement"
	SqliteBigint      = "bigint"
	SqliteReal        = "real"
	SqliteVarchar     = "varchar(%d)"
	SqliteDatetime    = "datetime"
	SqliteBlob        = "blob"
	SqliteText        = "text"

	SqliteHasindexSql  = "SELECT count(*) FROM sqlite_master WHERE tbl_name = ? AND sql LIKE '%%INDEX %v ON%%'"
	SqliteHastableSql  = "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?"
	SqliteHascolumnSql = "SELECT count(*) FROM sqlite_master WHERE tbl_name = ? AND (sql LIKE '%%\"%v\" %%' OR sql LIKE '%%%v %%');\n"
)

func (sqlite3) GetName() string {
	return SqliteDialectName
}

// Get Data Type for Sqlite Dialect
func (sqlite3) DataTypeOf(field *StructField) string {
	var dataValue, sqlType, size, additionalType = field.ParseFieldStructForDialect()

	if sqlType == "" {

		switch dataValue.Kind() {
		case reflect.Bool:
			sqlType = SqliteBool
		case reflect.Int,
			reflect.Int8,
			reflect.Int16,
			reflect.Int32,
			reflect.Uint,
			reflect.Uint8,
			reflect.Uint16,
			reflect.Uint32,
			reflect.Uintptr:
			if field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = SqlitePk
			} else {

				sqlType = SqliteInteger
			}
		case reflect.Int64,
			reflect.Uint64:
			if field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = SqlitePk
			} else {
				sqlType = SqliteBigint
			}
		case reflect.Float32,
			reflect.Float64:
			sqlType = SqliteReal
		case reflect.String:
			if size > 0 && size < 65532 {
				sqlType = fmt.Sprintf(SqliteVarchar, size)
			} else {
				sqlType = SqliteText
			}
		case reflect.Struct:
			if _, ok := dataValue.Interface().(time.Time); ok {
				sqlType = SqliteDatetime
			}
		default:
			if _, ok := dataValue.Interface().([]byte); ok {
				sqlType = SqliteBlob
			}
		}
	}

	if sqlType == "" {
		panic(fmt.Sprintf("invalid sql type %s (%s) for sqlite3", dataValue.Type().Name(), dataValue.Kind().String()))
	}

	if strings.TrimSpace(additionalType) == "" {
		return sqlType
	}
	return fmt.Sprintf("%v %v", sqlType, additionalType)
}

func (s sqlite3) HasIndex(tableName string, indexName string) bool {
	var count int
	s.db.QueryRow(fmt.Sprintf(SqliteHasindexSql, indexName), tableName).Scan(&count)
	return count > 0
}

func (s sqlite3) HasTable(tableName string) bool {
	var count int
	s.db.QueryRow(SqliteHastableSql, tableName).Scan(&count)
	return count > 0
}

func (s sqlite3) HasColumn(tableName string, columnName string) bool {
	var count int
	s.db.QueryRow(fmt.Sprintf(SqliteHascolumnSql, columnName, columnName), tableName).Scan(&count)
	return count > 0
}

func (s sqlite3) CurrentDatabase() (name string) {
	var (
		ifaces   = make([]interface{}, 3)
		pointers = make([]*string, 3)
		i        int
	)
	for i = 0; i < 3; i++ {
		ifaces[i] = &pointers[i]
	}
	if err := s.db.QueryRow("PRAGMA database_list").Scan(ifaces...); err != nil {
		return
	}
	if pointers[1] != nil {
		name = *pointers[1]
	}
	return
}
