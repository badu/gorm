package gorm

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
)

// RegisterDialect register new dialect
func RegisterDialect(name string, dialect Dialect) {
	dialectsMap[name] = dialect
}

func equalAsString(a interface{}, b interface{}) bool {
	return toString(a) == toString(b)
}

//TODO : @Badu - I really don't like this, being too generic, like we have no idea what we are comparing
func toString(input interface{}) string {
	switch value := input.(type) {
	case []interface{}:
		var results string
		for _, value := range value {
			if results != "" {
				results += "_"
			}
			results += toString(value)
		}
		return results
	case []byte:
		return string(value)
	default:
		if reflectValue := reflect.Indirect(reflect.ValueOf(input)); reflectValue.IsValid() {
			return fmt.Sprintf("%v", reflectValue.Interface())
		}
	}
	return ""
}

func addExtraSpaceIfExist(str string) string {
	if str != "" {
		return " " + str
	}
	return ""
}

//using inline advantage
func toQueryMarks(primaryValues [][]interface{}) string {
	result := ""
	for _, primaryValue := range primaryValues {
		marks := ""
		for range primaryValue {
			if marks != "" {
				marks += ","
			}
			marks += "?"
		}
		if result != "" {
			result += ","
		}
		if len(primaryValue) > 1 {
			result += "(" + marks + ")"
		} else {
			result += marks
		}
	}
	return result
}

//using inline advantage
func toQueryValues(values [][]interface{}) []interface{} {
	var results []interface{}
	for _, value := range values {
		for _, v := range value {
			results = append(results, v)
		}
	}
	return results
}

//using inline advantage
func generatePreloadDBWithConditions(preloadDB *DBCon, conditions []interface{}) (*DBCon, []interface{}) {
	var (
		preloadConditions []interface{}
	)

	for _, condition := range conditions {
		if scopes, ok := condition.(func(*DBCon) *DBCon); ok {
			preloadDB = scopes(preloadDB)
		} else {
			preloadConditions = append(preloadConditions, condition)
		}
	}

	return preloadDB, preloadConditions
}

//using inline advantage
func getColumnAsArray(columns StrSlice, values ...interface{}) [][]interface{} {
	var results [][]interface{}
	for _, value := range values {
		indirectValue := IndirectValue(value)
		switch indirectValue.Kind() {
		case reflect.Slice:
			for i := 0; i < indirectValue.Len(); i++ {
				var result []interface{}
				object := FieldValue(indirectValue, i)
				var hasValue = false
				for _, column := range columns {
					field := object.FieldByName(column)
					if !IsZero(field) {
						hasValue = true
					}
					result = append(result, field.Interface())
				}

				if hasValue {
					results = append(results, result)
				}
			}
		case reflect.Struct:
			var result []interface{}
			var hasValue = false
			for _, column := range columns {
				field := indirectValue.FieldByName(column)
				if !IsZero(field) {
					hasValue = true
				}
				result = append(result, field.Interface())
			}
			if hasValue {
				results = append(results, result)
			}
		}
	}
	return results
}

//using inline advantage
//returns the scope of a slice or struct column
func getColumnAsScope(column string, scope *Scope) *Scope {
	switch scope.rValue.Kind() {
	case reflect.Slice:
		if fieldStruct, ok := scope.GetModelStruct().ModelType.FieldByName(column); ok {
			fieldType := fieldStruct.Type
			if fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}

			resultsMap := map[interface{}]bool{}
			results := reflect.New(reflect.SliceOf(reflect.PtrTo(fieldType))).Elem()

			for i := 0; i < scope.rValue.Len(); i++ {
				reflectValue := FieldValue(scope.rValue, i)
				fieldRef := FieldColumn(reflectValue, column)
				if fieldRef.Kind() == reflect.Slice {
					for j := 0; j < fieldRef.Len(); j++ {
						if elem := fieldRef.Index(j); elem.CanAddr() && resultsMap[elem.Addr()] != true {
							resultsMap[elem.Addr()] = true
							results = reflect.Append(results, elem.Addr())
						}
					}
				} else if fieldRef.CanAddr() && resultsMap[fieldRef.Addr()] != true {
					resultsMap[fieldRef.Addr()] = true
					results = reflect.Append(results, fieldRef.Addr())
				}
			}
			return scope.con.emptyScope(results.Interface())
		} else {
			scope.Err(fmt.Errorf(errFieldNotFound, column, scope.GetModelStruct().ModelType))
		}
	case reflect.Struct:
		if field := scope.rValue.FieldByName(column); field.CanAddr() {
			return scope.con.emptyScope(field.Addr().Interface())
		}
	}
	return nil
}

//using inline advantage
func convertInterfaceToMap(con *DBCon, values interface{}, withIgnoredField bool) map[string]interface{} {
	var attrs = map[string]interface{}{}

	switch value := values.(type) {
	case map[string]interface{}:
		return value
	case []interface{}:
		for _, v := range value {
			for key, value := range convertInterfaceToMap(con, v, withIgnoredField) {
				attrs[key] = value
			}
		}
	case interface{}:
		reflectValue := reflect.ValueOf(values)

		switch reflectValue.Kind() {
		case reflect.Map:
			for _, key := range reflectValue.MapKeys() {
				attrs[con.parent.namesMap.toDBName(key.Interface().(string))] = reflectValue.MapIndex(key).Interface()
			}
		default:
			for _, field := range con.NewScope(values).Fields() {
				if !field.IsBlank() && (withIgnoredField || !field.IsIgnored()) {
					attrs[field.DBName] = field.Value.Interface()
				}
			}
		}
	}
	return attrs
}

//using inline advantage
func argsToInterface(args ...interface{}) interface{} {
	var result interface{}
	if len(args) == 1 {
		switch attr := args[0].(type) {
		case map[string]interface{}:
			result = attr
		case interface{}:
			result = attr
		}
	} else if len(args) > 1 {
		if str, ok := args[0].(string); ok {
			result = map[string]interface{}{str: args[1]}
		}
	}
	return result
}

//using inline advantage
func updatedAttrsWithValues(scope *Scope, value interface{}) (map[string]interface{}, bool) {
	if scope.rValue.Kind() != reflect.Struct {
		return convertInterfaceToMap(scope.con, value, false), true
	}
	var (
		results   = map[string]interface{}{}
		hasUpdate = false
	)
	for key, value := range convertInterfaceToMap(scope.con, value, true) {
		field, ok := scope.FieldByName(key)
		if !ok {
			scope.Err(fmt.Errorf(errFieldNotFound, key, scope.GetModelStruct().ModelType))
		}
		if ok && scope.Search.changeableField(field) {
			if _, ok := value.(*SqlPair); ok {
				hasUpdate = true
				results[field.DBName] = value
			} else {
				err := field.Set(value)
				if field.IsNormal() {
					hasUpdate = true
					if err == ErrUnaddressable {
						results[field.DBName] = value
					} else {
						results[field.DBName] = field.Value.Interface()
					}
				}
			}
		}
	}
	return results, hasUpdate
}

//using inline advantage
// getValueFromFields return given fields's value
func getValueFromFields(fields StrSlice, value reflect.Value) []interface{} {
	var results []interface{}
	// If value is a nil pointer, Indirect returns a zero Value!
	// Therefor we need to check for a zero value,
	// as FieldByName could panic
	if indirectValue := reflect.Indirect(value); indirectValue.IsValid() {
		for _, fieldName := range fields {
			if fieldValue := indirectValue.FieldByName(fieldName); fieldValue.IsValid() {
				result := fieldValue.Interface()
				if r, ok := result.(driver.Valuer); ok {
					result, _ = r.Value()
				}
				results = append(results, result)
			}
		}
	}
	return results
}

//using inline advantage
// IndirectValue return scope's reflect value's indirect value
func IndirectValue(value interface{}) reflect.Value {
	result := reflect.ValueOf(value)
	for result.Kind() == reflect.Ptr {
		result = result.Elem()
	}
	return result
}

//using inline advantage
func FieldValue(value reflect.Value, index int) reflect.Value {
	result := value.Index(index)
	for result.Kind() == reflect.Ptr {
		result = result.Elem()
	}
	return result
}

//using inline advantage
func FieldColumn(value reflect.Value, name string) reflect.Value {
	result := value.FieldByName(name)
	for result.Kind() == reflect.Ptr {
		result = result.Elem()
	}
	return result
}

func GetType(value interface{}) reflect.Type {
	result := reflect.ValueOf(value).Type()

	for result.Kind() == reflect.Slice || result.Kind() == reflect.Ptr {
		result = result.Elem()
	}

	return result
}

func SetZero(value reflect.Value) reflect.Value {
	return reflect.Zero(value.Type())
}

func IsZero(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return value.Len() == 0
	case reflect.Bool:
		return !value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return value.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return value.IsNil()
	}
	return reflect.DeepEqual(value.Interface(), SetZero(value).Interface())
}

func fullFileWithLineNum() string {
	result := ""
	for i := 1; i < 12; i++ {
		_, file, line, ok := runtime.Caller(i)
		if ok {
			result += fmt.Sprintf("%d %v:%v \n", i, file, line)
		} else {
			break
		}

	}
	return result
}

func fileWithLineNum() string {
	for i := 2; i < 12; i++ {
		_, file, line, ok := runtime.Caller(i)
		if ok {
			//if it's our test
			if regexpTest.MatchString(file) {
				return fmt.Sprintf("%v:%v", file, line)
			} else if !regexpSelf.MatchString(file) {
				return fmt.Sprintf("%v:%v", file, line)
			}
		}
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
	var conDialect Dialect
	if value, ok := dialectsMap[dialectName]; ok {
		conDialect = reflect.New(reflect.TypeOf(value).Elem()).Interface().(Dialect)
		conDialect.SetDB(dbSQL.(*sql.DB))
	} else {
		fmt.Printf("`%v` is not officially supported, running under compatibility mode.\n", dialectName)
		conDialect = &commonDialect{}
		conDialect.SetDB(dbSQL.(*sql.DB))
	}

	db = DBCon{
		dialect:         conDialect,
		logger:          defaultLogger,
		callbacks:       &Callbacks{},
		settings:        map[uint64]interface{}{},
		sqli:            dbSQL,
		modelsStructMap: &safeModelStructsMap{l: new(sync.RWMutex), m: make(map[reflect.Type]*ModelStruct)},
		namesMap:        &safeMap{l: new(sync.RWMutex), m: make(map[string]string)},
		quotedNames:     &safeMap{l: new(sync.RWMutex), m: make(map[string]string)},
	}
	//set no log
	db.LogMode(false)

	//TODO : @Badu - don't like that it points itself - maybe what's kept in initial should be gormSetting (use dbcon.get() to get them)
	db.parent = &db

	if err == nil {
		err = db.DB().Ping() // Send a ping to make sure the database connection is alive.
		if err != nil {
			db.DB().Close()
		}
	}

	return &db, err
}
