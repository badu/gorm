package gorm

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

const (
	PG_DIALECT_NAME = "postgres"
	PG_BOOLEAN_TYPE = "boolean"
	PG_INT_TYPE     = "integer"
	PG_SERIAL       = "serial"
	PG_BIG_SERIAL   = "bigserial"
	PG_BIGINT       = "bigint"
	PG_NUMERIC      = "numeric"
	PG_TEXT         = "text"
	PG_TIMESTAMP    = "timestamp with time zone"
	PG_VARCHAR      = "varchar(%d)"
	PG_HSTORE       = "hstore"
	PG_BYTEA        = "bytea"
	PG_UUID         = "uuid"

	PG_HASINDEX_SQL  = "SELECT count(*) FROM pg_indexes WHERE tablename = $1 AND indexname = $2"
	PG_HASFK_SQL     = "SELECT count(con.conname) FROM pg_constraint con WHERE $1::regclass::oid = con.conrelid AND con.conname = $2 AND con.contype='f'"
	PG_HASTABLE_SQL  = "SELECT count(*) FROM INFORMATION_SCHEMA.tables WHERE table_name = $1 AND table_type = 'BASE TABLE'"
	PG_HASCOLUMN_SQL = "SELECT count(*) FROM INFORMATION_SCHEMA.columns WHERE table_name = $1 AND column_name = $2"
	PG_CURRDB_SQL    = "SELECT CURRENT_DATABASE()"
)

func (postgres) GetName() string {
	return PG_DIALECT_NAME
}

func (postgres) BindVar(i int) string {
	return fmt.Sprintf("$%v", i)
}

func (postgres) DataTypeOf(field *StructField) string {
	var dataValue, sqlType, size, additionalType = field.ParseFieldStructForDialect()

	if sqlType == "" {
		switch dataValue.Kind() {
		case reflect.Bool:
			sqlType = PG_BOOLEAN_TYPE
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uintptr:
			if field.IsAutoIncrement() || field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = PG_SERIAL
			} else {
				sqlType = PG_INT_TYPE
			}
		case reflect.Int64, reflect.Uint64:
			if field.IsAutoIncrement() || field.IsPrimaryKey() {
				field.SetIsAutoIncrement()
				sqlType = PG_BIG_SERIAL
			} else {
				sqlType = PG_BIGINT
			}
		case reflect.Float32, reflect.Float64:
			sqlType = PG_NUMERIC
		case reflect.String:
			if !field.HasSetting(set_size) {
				// if SIZE haven't been set, use `text` as the default type, as there are no performance different
				size = 0
			}

			if size > 0 && size < 65532 {
				sqlType = fmt.Sprintf(PG_VARCHAR, size)
			} else {
				sqlType = PG_TEXT
			}
		case reflect.Struct:
			if _, ok := dataValue.Interface().(time.Time); ok {
				sqlType = PG_TIMESTAMP
			}
		case reflect.Map:
			if dataValue.Type().Name() == "Hstore" {
				sqlType = PG_HSTORE
			}
		default:
			if (dataValue.Kind() == reflect.Array ||
				dataValue.Kind() == reflect.Slice) &&
				dataValue.Type().Elem() == reflect.TypeOf(uint8(0)) {
				sqlType = PG_BYTEA
			} else if dataValue.Kind() == reflect.Array &&
				dataValue.Type().Len() == 16 {
				typename := dataValue.Type().Name()
				lower := strings.ToLower(typename)
				if "uuid" == lower || "guid" == lower {
					sqlType = PG_UUID
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

func (dialect postgres) HasIndex(tableName string, indexName string) bool {
	var count int
	dialect.db.QueryRow(PG_HASINDEX_SQL, tableName, indexName).Scan(&count)
	return count > 0
}

func (dialect postgres) HasForeignKey(tableName string, foreignKeyName string) bool {
	var count int
	dialect.db.QueryRow(PG_HASFK_SQL, tableName, foreignKeyName).Scan(&count)
	return count > 0
}

func (dialect postgres) HasTable(tableName string) bool {
	var count int
	dialect.db.QueryRow(PG_HASTABLE_SQL, tableName).Scan(&count)
	return count > 0
}

func (dialect postgres) HasColumn(tableName string, columnName string) bool {
	var count int
	dialect.db.QueryRow(PG_HASCOLUMN_SQL, tableName, columnName).Scan(&count)
	return count > 0
}

func (dialect postgres) CurrentDatabase() (name string) {
	dialect.db.QueryRow(PG_CURRDB_SQL).Scan(&name)
	return
}

func (dialect postgres) LastInsertIDReturningSuffix(tableName, key string) string {
	return fmt.Sprintf("RETURNING %v.%v", tableName, key)
}

func (postgres) SupportLastInsertID() bool {
	return false
}
