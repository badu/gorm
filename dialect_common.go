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
	CommonDialectName   = "common"
	CommonBoolean       = "BOOLEAN"
	CommonInteger       = "INTEGER"
	CommonAutoIncrement = "INTEGER AUTO_INCREMENT"
	CommonBigint        = "BIGINT"
	CommonFloat         = "FLOAT"
	CommonVarchar       = "VARCHAR"
	CommonTimestamp     = "TIMESTAMP"
	CommonBinary        = "BINARY"

	CommonHasindexsql  = "SELECT count(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE table_schema = ? AND table_name = ? AND index_name = ?"
	CommonDropindex    = "DROP INDEX %v"
	CommonHastableSql  = "SELECT count(*) FROM INFORMATION_SCHEMA.TABLES WHERE table_schema = ? AND table_name = ?"
	CommonHascolumnSql = "SELECT count(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE table_schema = ? AND table_name = ? AND column_name = ?"
	CommonSelectDb     = "SELECT DATABASE()"
)

func (commonDialect) GetName() string {
	return CommonDialectName
}

func (c *commonDialect) SetDB(db *sql.DB) {
	c.db = db
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
			sqlType = CommonBoolean
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uintptr:
			if field.IsAutoIncrement() {
				sqlType = fmt.Sprintf("%s %s", CommonBoolean, CommonAutoIncrement)
			} else {
				sqlType = CommonInteger
			}
		case reflect.Int64, reflect.Uint64:
			if field.IsAutoIncrement() {
				sqlType = fmt.Sprintf("%s %s", CommonBigint, CommonAutoIncrement)
			} else {
				sqlType = CommonBigint
			}
		case reflect.Float32, reflect.Float64:
			sqlType = CommonFloat
		case reflect.String:
			if size > 0 && size < 65532 {
				sqlType = fmt.Sprintf("%s(%d)", CommonVarchar, size)
			} else {
				sqlType = fmt.Sprintf("%s(%d)", CommonVarchar, 65532)
			}
		case reflect.Struct:
			if _, ok := dataValue.Interface().(time.Time); ok {
				sqlType = CommonTimestamp
			}
		default:
			if _, ok := dataValue.Interface().([]byte); ok {
				if size > 0 && size < 65532 {
					sqlType = fmt.Sprintf("%s(%d)", CommonBinary, size)
				} else {
					sqlType = fmt.Sprintf("%s(%d)", CommonBinary, 65532)
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

func (c commonDialect) HasIndex(tableName string, indexName string) bool {
	var count int
	c.db.QueryRow(CommonHasindexsql, c.CurrentDatabase(), tableName, indexName).Scan(&count)
	return count > 0
}

func (c commonDialect) RemoveIndex(tableName string, indexName string) error {
	_, err := c.db.Exec(fmt.Sprintf(CommonDropindex, indexName))
	return err
}

func (commonDialect) HasForeignKey(tableName string, foreignKeyName string) bool {
	return false
}

//TODO : cache tables and provide cached response (faster init)
func (c commonDialect) HasTable(tableName string) bool {
	var count int
	c.db.QueryRow(CommonHastableSql, c.CurrentDatabase(), tableName).Scan(&count)
	return count > 0
}

func (c commonDialect) HasColumn(tableName string, columnName string) bool {
	var count int
	c.db.QueryRow(CommonHascolumnSql, c.CurrentDatabase(), tableName, columnName).Scan(&count)
	return count > 0
}

func (c commonDialect) CurrentDatabase() (name string) {
	c.db.QueryRow(CommonSelectDb).Scan(&name)
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
