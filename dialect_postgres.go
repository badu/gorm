package gorm

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

const (
	PgDialectName = "postgres"
	PgBooleanType = "boolean"
	PgIntType     = "integer"
	PgSerial      = "serial"
	PgBigSerial   = "bigserial"
	PgBigint      = "bigint"
	PgNumeric     = "numeric"
	PgText        = "text"
	PgTimestamp   = "timestamp with time zone"
	PgVarchar     = "varchar(%d)"
	PgHstore      = "hstore"
	PgBytea       = "bytea"
	PgUuid        = "uuid"

	PgHasindexSql  = "SELECT count(*) FROM pg_indexes WHERE tablename = $1 AND indexname = $2"
	PgHasfkSql     = "SELECT count(con.conname) FROM pg_constraint con WHERE $1::regclass::oid = con.conrelid AND con.conname = $2 AND con.contype='f'"
	PgHastableSql  = "SELECT count(*) FROM INFORMATION_SCHEMA.tables WHERE table_name = $1 AND table_type = 'BASE TABLE'"
	PgHascolumnSql = "SELECT count(*) FROM INFORMATION_SCHEMA.columns WHERE table_name = $1 AND column_name = $2"
	PgCurrdbSql    = "SELECT CURRENT_DATABASE()"
)

func (postgres) GetName() string {
	return PgDialectName
}

func (postgres) BindVar(i int) string {
	return fmt.Sprintf("$%v", i)
}

func (postgres) DataTypeOf(field *StructField) string {
	var dataValue, sqlType, size, additionalType = field.ParseFieldStructForDialect()

	if sqlType == "" {
		switch dataValue.Kind() {
		case reflect.Bool:
			sqlType = PgBooleanType
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uintptr:
			if field.IsAutoIncrement() || field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = PgSerial
			} else {
				sqlType = PgIntType
			}
		case reflect.Int64, reflect.Uint64:
			if field.IsAutoIncrement() || field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = PgBigSerial
			} else {
				sqlType = PgBigint
			}
		case reflect.Float32, reflect.Float64:
			sqlType = PgNumeric
		case reflect.String:
			if !field.HasSetting(setSize) {
				// if SIZE haven't been set, use `text` as the default type, as there are no performance different
				size = 0
			}

			if size > 0 && size < 65532 {
				sqlType = fmt.Sprintf(PgVarchar, size)
			} else {
				sqlType = PgText
			}
		case reflect.Struct:
			if _, ok := dataValue.Interface().(time.Time); ok {
				sqlType = PgTimestamp
			}
		case reflect.Map:
			if dataValue.Type().Name() == "Hstore" {
				sqlType = PgHstore
			}
		default:
			if (dataValue.Kind() == reflect.Array ||
				dataValue.Kind() == reflect.Slice) &&
				dataValue.Type().Elem() == reflect.TypeOf(uint8(0)) {
				sqlType = PgBytea
			} else if dataValue.Kind() == reflect.Array &&
				dataValue.Type().Len() == 16 {
				typename := dataValue.Type().Name()
				lower := strings.ToLower(typename)
				if "uuid" == lower || "guid" == lower {
					sqlType = PgUuid
				}
			}
		}
	}

	if sqlType == "" {
		panic(fmt.Sprintf("invalid sql type %s (%s) for postgres", dataValue.Type().Name(), dataValue.Kind().String()))
	}

	if strings.TrimSpace(additionalType) == "" {
		return sqlType
	}
	return fmt.Sprintf("%v %v", sqlType, additionalType)
}

func (p postgres) HasIndex(tableName string, indexName string) bool {
	var count int
	p.db.QueryRow(PgHasindexSql, tableName, indexName).Scan(&count)
	return count > 0
}

func (p postgres) HasForeignKey(tableName string, foreignKeyName string) bool {
	var count int
	p.db.QueryRow(PgHasfkSql, tableName, foreignKeyName).Scan(&count)
	return count > 0
}

func (p postgres) HasTable(tableName string) bool {
	var count int
	p.db.QueryRow(PgHastableSql, tableName).Scan(&count)
	return count > 0
}

func (p postgres) HasColumn(tableName string, columnName string) bool {
	var count int
	p.db.QueryRow(PgHascolumnSql, tableName, columnName).Scan(&count)
	return count > 0
}

func (p postgres) CurrentDatabase() (name string) {
	p.db.QueryRow(PgCurrdbSql).Scan(&name)
	return
}

func (p postgres) LastInsertIDReturningSuffix(tableName, key string) string {
	return fmt.Sprintf("RETURNING %v.%v", tableName, key)
}

func (postgres) SupportLastInsertID() bool {
	return false
}
