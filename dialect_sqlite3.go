package gorm

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

const (
	SQLITE_DIALECT_NAME = "sqlite3"
	SQLITE_BOOL         = "bool"
	SQLITE_INTEGER      = "integer"
	SQLITE_PK           = "integer primary key autoincrement"
	SQLITE_BIGINT       = "bigint"
	SQLITE_REAL         = "real"
	SQLITE_VARCHAR      = "varchar(%d)"
	SQLITE_DATETIME     = "datetime"
	SQLITE_BLOB         = "blob"
	SQLITE_TEXT         = "text"

	SQLITE_HASINDEX_SQL  = "SELECT count(*) FROM sqlite_master WHERE tbl_name = ? AND sql LIKE '%%INDEX %v ON%%'"
	SQLITE_HASTABLE_SQL  = "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?"
	SQLITE_HASCOLUMN_SQL = "SELECT count(*) FROM sqlite_master WHERE tbl_name = ? AND (sql LIKE '%%\"%v\" %%' OR sql LIKE '%%%v %%');\n"
)

func (sqlite3) GetName() string {
	return SQLITE_DIALECT_NAME
}

// Get Data Type for Sqlite Dialect
func (sqlite3) DataTypeOf(field *StructField) string {
	var dataValue, sqlType, size, additionalType = field.ParseFieldStructForDialect()

	if sqlType == "" {

		switch dataValue.Kind() {
		case reflect.Bool:
			sqlType = SQLITE_BOOL
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
				sqlType = SQLITE_PK
			} else {

				sqlType = SQLITE_INTEGER
			}
		case reflect.Int64,
			reflect.Uint64:
			if field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = SQLITE_PK
			} else {
				sqlType = SQLITE_BIGINT
			}
		case reflect.Float32,
			reflect.Float64:
			sqlType = SQLITE_REAL
		case reflect.String:
			if size > 0 && size < 65532 {
				sqlType = fmt.Sprintf(SQLITE_VARCHAR, size)
			} else {
				sqlType = SQLITE_TEXT
			}
		case reflect.Struct:
			if _, ok := dataValue.Interface().(time.Time); ok {
				sqlType = SQLITE_DATETIME
			}
		default:
			if _, ok := dataValue.Interface().([]byte); ok {
				sqlType = SQLITE_BLOB
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
	s.db.QueryRow(fmt.Sprintf(SQLITE_HASINDEX_SQL, indexName), tableName).Scan(&count)
	return count > 0
}

func (s sqlite3) HasTable(tableName string) bool {
	var count int
	s.db.QueryRow(SQLITE_HASTABLE_SQL, tableName).Scan(&count)
	return count > 0
}

func (s sqlite3) HasColumn(tableName string, columnName string) bool {
	var count int
	s.db.QueryRow(fmt.Sprintf(SQLITE_HASCOLUMN_SQL, columnName, columnName), tableName).Scan(&count)
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
