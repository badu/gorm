package gorm

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	COMMON_DIALECT_NAME = "common"
	COMMON_BOOLEAN        = "BOOLEAN"
	COMMON_INTEGER        = "INTEGER"
	COMMON_AUTO_INCREMENT = "INTEGER AUTO_INCREMENT"
	COMMON_BIGINT         = "BIGINT"
	COMMON_FLOAT          = "FLOAT"
	COMMON_VARCHAR        = "VARCHAR"
	COMMON_TIMESTAMP      = "TIMESTAMP"
	COMMON_BINARY         = "BINARY"

	COMMON_HASINDEXSQL   = "SELECT count(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE table_schema = ? AND table_name = ? AND index_name = ?"
	COMMON_DROPINDEX     = "DROP INDEX %v"
	COMMON_HASTABLE_SQL  = "SELECT count(*) FROM INFORMATION_SCHEMA.TABLES WHERE table_schema = ? AND table_name = ?"
	COMMON_HASCOLUMN_SQL = "SELECT count(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE table_schema = ? AND table_name = ? AND column_name = ?"
	COMMON_SELECT_DB     = "SELECT DATABASE()"
)

func (commonDialect) GetName() string {
	return COMMON_DIALECT_NAME
}

func (dialect *commonDialect) SetDB(db *sql.DB) {
	dialect.db = db
}

func (commonDialect) BindVar(i int) string {
	return "$$"
}

func (commonDialect) GetQuoter() string {
	return "\""
}

func (commonDialect) DataTypeOf(field *StructField) string {
	var dataValue, sqlType, size, additionalType = field.ParseFieldStructForDialect()

	if sqlType == "" {
		switch dataValue.Kind() {
		case reflect.Bool:
			sqlType = COMMON_BOOLEAN
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uintptr:
			if field.IsAutoIncrement() {
				sqlType = fmt.Sprintf("%s %s", COMMON_BOOLEAN, COMMON_AUTO_INCREMENT)
			} else {
				sqlType = COMMON_INTEGER
			}
		case reflect.Int64, reflect.Uint64:
			if field.IsAutoIncrement() {
				sqlType = fmt.Sprintf("%s %s", COMMON_BIGINT, COMMON_AUTO_INCREMENT)
			} else {
				sqlType = COMMON_BIGINT
			}
		case reflect.Float32, reflect.Float64:
			sqlType = COMMON_FLOAT
		case reflect.String:
			if size > 0 && size < 65532 {
				sqlType = fmt.Sprintf("%s(%d)", COMMON_VARCHAR, size)
			} else {
				sqlType = fmt.Sprintf("%s(%d)", COMMON_VARCHAR, 65532)
			}
		case reflect.Struct:
			if _, ok := dataValue.Interface().(time.Time); ok {
				sqlType = COMMON_TIMESTAMP
			}
		default:
			if _, ok := dataValue.Interface().([]byte); ok {
				if size > 0 && size < 65532 {
					sqlType = fmt.Sprintf("%s(%d)", COMMON_BINARY, size)
				} else {
					sqlType = fmt.Sprintf("%s(%d)", COMMON_BINARY, 65532)
				}
			}
		}
	}

	if sqlType == "" {
		panic(fmt.Sprintf("invalid sql type %s (%s) for commonDialect", dataValue.Type().Name(), dataValue.Kind().String()))
	}

	if strings.TrimSpace(additionalType) == "" {
		return sqlType
	}
	return fmt.Sprintf("%v %v", sqlType, additionalType)
}

func (dialect commonDialect) HasIndex(tableName string, indexName string) bool {
	var count int
	dialect.db.QueryRow(COMMON_HASINDEXSQL, dialect.CurrentDatabase(), tableName, indexName).Scan(&count)
	return count > 0
}

func (dialect commonDialect) RemoveIndex(tableName string, indexName string) error {
	_, err := dialect.db.Exec(fmt.Sprintf(COMMON_DROPINDEX, indexName))
	return err
}

func (commonDialect) HasForeignKey(tableName string, foreignKeyName string) bool {
	return false
}

func (dialect commonDialect) HasTable(tableName string) bool {
	var count int
	dialect.db.QueryRow(COMMON_HASTABLE_SQL, dialect.CurrentDatabase(), tableName).Scan(&count)
	return count > 0
}

func (dialect commonDialect) HasColumn(tableName string, columnName string) bool {
	var count int
	dialect.db.QueryRow(COMMON_HASCOLUMN_SQL, dialect.CurrentDatabase(), tableName, columnName).Scan(&count)
	return count > 0
}

func (dialect commonDialect) CurrentDatabase() (name string) {
	dialect.db.QueryRow(COMMON_SELECT_DB).Scan(&name)
	return
}

func (commonDialect) LimitAndOffsetSQL(limit, offset interface{}) (sql string) {
	if limit != nil {
		if parsedLimit, err := strconv.ParseInt(fmt.Sprint(limit), 0, 0); err == nil && parsedLimit > 0 {
			sql += fmt.Sprintf(" LIMIT %d", parsedLimit)
		}
	}
	if offset != nil {
		if parsedOffset, err := strconv.ParseInt(fmt.Sprint(offset), 0, 0); err == nil && parsedOffset > 0 {
			sql += fmt.Sprintf(" OFFSET %d", parsedOffset)
		}
	}
	return
}

func (commonDialect) SelectFromDummyTable() string {
	return ""
}

func (commonDialect) LastInsertIDReturningSuffix(tableName, columnName string) string {
	return ""
}
