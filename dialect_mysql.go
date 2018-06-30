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
	MysqlDialectName   = "mysql"
	MysqlBooleanType   = "boolean"
	MysqlIntType       = "int"
	MysqlAutoIncrement = "AUTO_INCREMENT"
	MysqlUnsigned      = "unsigned"
	MysqlBigint        = "bigint"
	MysqlDouble        = "double"
	MysqlLongtext      = "longtext"
	MysqlVarchar       = "varchar"
	MysqlTimestamp     = "timestamp"
	MysqlLongblog      = "longblob"
	MysqlVarbinary     = "varbinary"

	MysqlHasForeignKey = "SELECT count(*) FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=? AND TABLE_NAME=? AND CONSTRAINT_NAME=? AND CONSTRAINT_TYPE='FOREIGN KEY'"
	MysqlDropIndex     = "DROP INDEX %v ON %v"
	MysqlSelectDb      = "SELECT DATABASE()"
)

func (mysql) GetName() string {
	return MysqlDialectName
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
		if !field.HasSetting(setIndex) && !field.IsPrimaryKey() {
			field.UnsetIsAutoIncrement()
		}
	}

	if sqlType == "" {
		switch dataValue.Kind() {
		case reflect.Bool:
			sqlType = MysqlBooleanType
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
			if field.IsAutoIncrement() || field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = fmt.Sprintf("%s %s", MysqlIntType, MysqlAutoIncrement)
			} else {
				sqlType = MysqlIntType
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uintptr:
			if field.IsAutoIncrement() || field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = fmt.Sprintf("%s %s %s", MysqlIntType, MysqlUnsigned, MysqlAutoIncrement)
			} else {
				sqlType = fmt.Sprintf("%s %s", MysqlIntType, MysqlUnsigned)
			}
		case reflect.Int64:
			if field.IsAutoIncrement() || field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = fmt.Sprintf("%s %s", MysqlBigint, MysqlAutoIncrement)
			} else {
				sqlType = MysqlBigint
			}
		case reflect.Uint64:
			if field.IsAutoIncrement() || field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = fmt.Sprintf("%s %s %s", MysqlBigint, MysqlUnsigned, MysqlAutoIncrement)
			} else {
				sqlType = fmt.Sprintf("%s %s", MysqlBigint, MysqlUnsigned)
			}
		case reflect.Float32, reflect.Float64:
			sqlType = MysqlDouble
		case reflect.String:
			if size > 0 && size < 65532 {
				sqlType = fmt.Sprintf("%s(%d)", MysqlVarchar, size)
			} else {
				sqlType = MysqlLongtext
			}
		case reflect.Struct:
			if _, ok := dataValue.Interface().(time.Time); ok {
				if field.HasSetting(setNotNull) {
					sqlType = MysqlTimestamp
				} else {
					sqlType = MysqlTimestamp + " NULL"
				}
			}
		default:
			if _, ok := dataValue.Interface().([]byte); ok {
				if size > 0 && size < 65532 {
					sqlType = fmt.Sprintf("%s(%d)", MysqlVarbinary, size)
				} else {
					sqlType = MysqlLongblog
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

func (m *mysql) RemoveIndex(tableName string, indexName string) error {
	q := m.GetQuoter()
	_, err := m.db.Exec(fmt.Sprintf(MysqlDropIndex, indexName, q+regExpPeriod.ReplaceAllString(tableName, q+"."+q)+q))
	return err
}

func (m mysql) HasForeignKey(tableName string, foreignKeyName string) bool {
	var count int

	m.db.QueryRow(MysqlHasForeignKey, m.CurrentDatabase(), tableName, foreignKeyName).Scan(&count)
	return count > 0
}

func (m mysql) CurrentDatabase() (name string) {
	m.db.QueryRow(MysqlSelectDb).Scan(&name)
	return
}

func (mysql) SelectFromDummyTable() string {
	return "FROM DUAL"
}

func (m mysql) BuildForeignKeyName(tableName, field, dest string) string {
	keyName := m.commonDialect.BuildForeignKeyName(tableName, field, dest)
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
