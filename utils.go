package gorm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

//============================================
// Dialect functions
//============================================

// RegisterDialect register new dialect
func RegisterDialect(name string, dialect Dialect) {
	dialectsMap[name] = dialect
}

//============================================
// Other utils functions
//============================================

func convertInterfaceToMap(values interface{}, withIgnoredField bool) map[string]interface{} {
	var attrs = map[string]interface{}{}

	switch value := values.(type) {
	case map[string]interface{}:
		return value
	case []interface{}:
		for _, v := range value {
			for key, value := range convertInterfaceToMap(v, withIgnoredField) {
				attrs[key] = value
			}
		}
	case interface{}:
		reflectValue := reflect.ValueOf(values)

		switch reflectValue.Kind() {
		case reflect.Map:
			for _, key := range reflectValue.MapKeys() {
				attrs[NamesMap.ToDBName(key.Interface().(string))] = reflectValue.MapIndex(key).Interface()
			}
		default:
			for _, field := range (&Scope{Value: values}).Fields() {
				if !field.IsBlank() && (withIgnoredField || !field.IsIgnored()) {
					attrs[field.DBName] = field.Value.Interface()
				}
			}
		}
	}
	return attrs
}

func toSearchableMap(attrs ...interface{}) interface{} {
	var result interface{}
	//TODO : @Badu - what happens to zero ? return nil, right? Return warning
	if len(attrs) == 1 {
		if attr, ok := attrs[0].(map[string]interface{}); ok {
			result = attr
		}

		if attr, ok := attrs[0].(interface{}); ok {
			result = attr
		}
	} else if len(attrs) > 1 {
		if str, ok := attrs[0].(string); ok {
			result = map[string]interface{}{str: attrs[1]}
		}
	}
	return result
}

func equalAsString(a interface{}, b interface{}) bool {
	return toString(a) == toString(b)
}

//TODO : @Badu - I really don't like this, being too generic, like we have no idea what we are comparing
func toString(str interface{}) string {
	if values, ok := str.([]interface{}); ok {
		var results []string
		for _, value := range values {
			results = append(results, toString(value))
		}
		return strings.Join(results, "_")
	} else if strBytes, ok := str.([]byte); ok {
		return string(strBytes)
	} else if reflectValue := reflect.Indirect(reflect.ValueOf(str)); reflectValue.IsValid() {
		return fmt.Sprintf("%v", reflectValue.Interface())
	}
	return ""
}

func addExtraSpaceIfExist(str string) string {
	if str != "" {
		return " " + str
	}
	return ""
}

// Open initialize a new db connection, need to import driver first, e.g:
//
//     import _ "github.com/go-sql-driver/mysql"
//     func main() {
//       db, err := gorm.Open("mysql", "user:password@/dbname?charset=utf8&parseTime=True&loc=Local")
//     }
// GORM has wrapped some drivers, for easier to remember driver's import path, so you could import the mysql driver with
//    import _ "github.com/badu/gorm/dialects/mysql"
//    // import _ "github.com/badu/gorm/dialects/postgres"
//    // import _ "github.com/badu/gorm/dialects/sqlite"
//    // import _ "github.com/badu/gorm/dialects/mssql"
func Open(dialectName string, args ...interface{}) (*DBCon, error) {
	var db DBCon
	var err error

	if len(args) == 0 {
		err = errors.New("OPEN ERROR : invalid database source")
		return nil, err
	}
	var source string
	var dbSQL sqlInterf

	switch value := args[0].(type) {
	case string:
		var driverName = dialectName
		if len(args) == 1 {
			source = value
		} else if len(args) >= 2 {
			driverName = value
			source = args[1].(string)
		}
		dbSQL, err = sql.Open(driverName, source)
	case sqlInterf:
		source = reflect.Indirect(reflect.ValueOf(value)).FieldByName("dsn").String()
		dbSQL = value
	}
	//TODO : dialects map should disappear - instead of dialectName we should receive directly the Dialect
	var commontDialect Dialect
	if value, ok := dialectsMap[dialectName]; ok {
		commontDialect = reflect.New(reflect.TypeOf(value).Elem()).Interface().(Dialect)
		commontDialect.SetDB(dbSQL.(*sql.DB))
	} else {
		fmt.Printf("`%v` is not officially supported, running under compatibility mode.\n", dialectName)
		commontDialect = &commonDialect{}
		commontDialect.SetDB(dbSQL.(*sql.DB))
	}

	db = DBCon{
		dialect:  commontDialect,
		logger:   defaultLogger,
		callback: &Callback{},
		source:   source,
		settings: map[string]interface{}{},
		sqli:     dbSQL,
	}
	//register all default callbacks
	db.callback.registerGORMDefaultCallbacks()
	//TODO : @Badu - don't like that it points itself
	db.parent = &db

	if err == nil {
		err = db.DB().Ping() // Send a ping to make sure the database connection is alive.
		if err != nil {
			db.DB().Close()
		}
	}

	return &db, err
}
