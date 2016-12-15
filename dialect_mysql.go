package gorm

import (
	"crypto/sha1"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MYSQL_BOOLEAN_TYPE   = "boolean"
	MYSQL_INT_TYPE       = "int"
	MYSQL_AUTO_INCREMENT = "AUTO_INCREMENT"
	MYSQL_UNSIGNED       = "unsigned"
	MYSQL_BIGINT         = "bigint"
	MYSQL_DOUBLE         = "double"
	MYSQL_LONGTEXT       = "longtext"
	MYSQL_VARCHAR        = "varchar"
	MYSQL_TIMESTAMP      = "timestamp"
	MYSQL_LONGBLOG       = "longblob"
	MYSQL_VARBINARY      = "varbinary"

	MYSQL_HAS_FOREIGN_KEY = "SELECT count(*) FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=? AND TABLE_NAME=? AND CONSTRAINT_NAME=? AND CONSTRAINT_TYPE='FOREIGN KEY'"
	MYSQL_DROP_INDEX      = "DROP INDEX %v ON %v"
	MYSQL_SELECT_DB       = "SELECT DATABASE()"
)

func (mysql) GetName() string {
	return "mysql"
}

func (mysql) GetQuoter() string {
	return "`"
}

// Get Data Type for MySQL Dialect
func (mysql) DataTypeOf(field *StructField) string {
	var dataValue, sqlType, size, additionalType = field.ParseFieldStructForDialect()

	// MySQL allows only one auto increment column per table, and it must
	// be a KEY column.
	//TODO : @Badu : document that if it has auto_increment but it's not an index, we ignore auto_increment
	if field.IsAutoIncrement() {
		if !field.HasSetting(set_index) && !field.IsPrimaryKey() {
			field.UnsetIsAutoIncrement()
		}
	}

	if sqlType == "" {
		switch dataValue.Kind() {
		case reflect.Bool:
			sqlType = MYSQL_BOOLEAN_TYPE
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
			if field.IsAutoIncrement() || field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = fmt.Sprintf("%s %s", MYSQL_INT_TYPE, MYSQL_AUTO_INCREMENT)
			} else {
				sqlType = MYSQL_INT_TYPE
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uintptr:
			if field.IsAutoIncrement() || field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = fmt.Sprintf("%s %s %s", MYSQL_INT_TYPE, MYSQL_UNSIGNED, MYSQL_AUTO_INCREMENT)
			} else {
				sqlType = fmt.Sprintf("%s %s", MYSQL_INT_TYPE, MYSQL_UNSIGNED)
			}
		case reflect.Int64:
			if field.IsAutoIncrement() || field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = fmt.Sprintf("%s %s", MYSQL_BIGINT, MYSQL_AUTO_INCREMENT)
			} else {
				sqlType = MYSQL_BIGINT
			}
		case reflect.Uint64:
			if field.IsAutoIncrement() || field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = fmt.Sprintf("%s %s %s", MYSQL_BIGINT, MYSQL_UNSIGNED, MYSQL_AUTO_INCREMENT)
			} else {
				sqlType = fmt.Sprintf("%s %s", MYSQL_BIGINT, MYSQL_UNSIGNED)
			}
		case reflect.Float32, reflect.Float64:
			sqlType = MYSQL_DOUBLE
		case reflect.String:
			if size > 0 && size < 65532 {
				sqlType = fmt.Sprintf("%s(%d)", MYSQL_VARCHAR, size)
			} else {
				sqlType = MYSQL_LONGTEXT
			}
		case reflect.Struct:
			if _, ok := dataValue.Interface().(time.Time); ok {
				if field.HasSetting(set_not_null) {
					sqlType = MYSQL_TIMESTAMP
				} else {
					sqlType = MYSQL_TIMESTAMP + " NULL"
				}
			}
		default:
			if _, ok := dataValue.Interface().([]byte); ok {
				if size > 0 && size < 65532 {
					sqlType = fmt.Sprintf("%s(%d)", MYSQL_VARBINARY, size)
				} else {
					sqlType = MYSQL_LONGBLOG
				}
			}
		}
	}

	if sqlType == "" {
		panic(fmt.Sprintf("invalid sql type %s (%s) for mysql", dataValue.Type().Name(), dataValue.Kind().String()))
	}

	if strings.TrimSpace(additionalType) == "" {
		return sqlType
	}
	return fmt.Sprintf("%v %v", sqlType, additionalType)
}

func (dialect *mysql) RemoveIndex(tableName string, indexName string) error {
	//TODO : @Badu - test if Quote(tableName, dialect) works correctly here
	_, err := dialect.db.Exec(fmt.Sprintf(MYSQL_DROP_INDEX, indexName, Quote(tableName, dialect)))
	return err
}

func (dialect mysql) HasForeignKey(tableName string, foreignKeyName string) bool {
	var count int

	dialect.db.QueryRow(MYSQL_HAS_FOREIGN_KEY, dialect.CurrentDatabase(), tableName, foreignKeyName).Scan(&count)
	return count > 0
}

func (dialect mysql) CurrentDatabase() (name string) {
	dialect.db.QueryRow(MYSQL_SELECT_DB).Scan(&name)
	return
}

func (mysql) SelectFromDummyTable() string {
	return "FROM DUAL"
}

func (dialect mysql) BuildForeignKeyName(tableName, field, dest string) string {
	keyName := dialect.commonDialect.BuildForeignKeyName(tableName, field, dest)
	if utf8.RuneCountInString(keyName) <= 64 {
		return keyName
	}
	h := sha1.New()
	h.Write([]byte(keyName))
	bs := h.Sum(nil)

	// sha1 is 40 digits, keep first 24 characters of destination
	destRunes := []rune(regExpFKName.ReplaceAllString(dest, "_"))
	if len(destRunes) > 24 {
		destRunes = destRunes[:24]
	}

	return fmt.Sprintf("%s%x", string(destRunes), bs)
}
