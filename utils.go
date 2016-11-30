package gorm

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// RegisterDialect register new dialect
func RegisterDialect(name string, dialect Dialect) {
	dialectsMap[name] = dialect
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

//quotes a field - called almost everywhere - using inline advantage
func Quote(field string, dialect Dialect) string {
	q := dialect.GetQuoter()
	return q + regExpPeriod.ReplaceAllString(field, q+"."+q) + q
}

//using inline advantage
func QuoteIfPossible(str string, dialect Dialect) string {
	// only match string like `name`, `users.name`
	if regExpNameMatcher.MatchString(str) {
		return Quote(str, dialect)
	}
	return str
}

//using inline advantage
// QuotedTableName return quoted table name
func QuotedTableName(scope *Scope) string {
	if scope.Search != nil && scope.Search.tableName != "" {
		if strings.Index(scope.Search.tableName, " ") != -1 {
			return scope.Search.tableName
		}
		return Quote(scope.Search.tableName, scope.con.parent.dialect)
	}

	return Quote(scope.TableName(), scope.con.parent.dialect)
}

//using inline advantage
func toQueryCondition(columns StrSlice, dialect Dialect) string {
	var newColumns []string
	for _, column := range columns {
		newColumns = append(newColumns, Quote(column, dialect))
	}

	if len(columns) > 1 {
		return fmt.Sprintf("(%v)", strings.Join(newColumns, ","))
	}
	return strings.Join(newColumns, ",")
}

//using inline advantage
func toQueryMarks(primaryValues [][]interface{}) string {
	var results []string

	for _, primaryValue := range primaryValues {
		var marks []string
		for range primaryValue {
			marks = append(marks, "?")
		}

		if len(marks) > 1 {
			results = append(results, fmt.Sprintf("(%v)", strings.Join(marks, ",")))
		} else {
			results = append(results, strings.Join(marks, ""))
		}
	}
	return strings.Join(results, ",")
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
		indirectValue := reflect.ValueOf(value)
		for indirectValue.Kind() == reflect.Ptr {
			indirectValue = indirectValue.Elem()
		}
		switch indirectValue.Kind() {
		case reflect.Slice:
			for i := 0; i < indirectValue.Len(); i++ {
				var result []interface{}
				var object = indirectValue.Index(i)
				for object.Kind() == reflect.Ptr {
					object = object.Elem()
				}
				var hasValue = false
				for _, column := range columns {
					field := object.FieldByName(column)
					if hasValue || !reflect.DeepEqual(field.Interface(), reflect.Zero(field.Type()).Interface()) {
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
				if hasValue || !reflect.DeepEqual(field.Interface(), reflect.Zero(field.Type()).Interface()) {
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
	indirectScopeValue := IndirectValue(scope)

	switch indirectScopeValue.Kind() {
	case reflect.Slice:
		if fieldStruct, ok := scope.GetModelStruct().ModelType.FieldByName(column); ok {
			fieldType := fieldStruct.Type
			if fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}

			resultsMap := map[interface{}]bool{}
			results := reflect.New(reflect.SliceOf(reflect.PtrTo(fieldType))).Elem()

			for i := 0; i < indirectScopeValue.Len(); i++ {
				reflectValue := indirectScopeValue.Index(i)
				for reflectValue.Kind() == reflect.Ptr {
					reflectValue = reflectValue.Elem()
				}

				fieldRef := reflectValue.FieldByName(column)
				for fieldRef.Kind() == reflect.Ptr {
					fieldRef = fieldRef.Elem()
				}

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
			return scope.NewScope(results.Interface())
		}
	case reflect.Struct:
		if field := indirectScopeValue.FieldByName(column); field.CanAddr() {
			return scope.NewScope(field.Addr().Interface())
		}
	}
	return nil
}

//using inline advantage
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

//using inline advantage
func newCon(con *DBCon) *DBCon {
	clone := DBCon{
		sqli:     con.sqli,
		parent:   con.parent,
		logger:   con.logger,
		logMode:  con.logMode,
		settings: map[uint64]interface{}{},
		Error:    con.Error,
	}
	return &clone
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
	if IndirectValue(scope).Kind() != reflect.Struct {
		return convertInterfaceToMap(value, false), true
	}

	results := map[string]interface{}{}
	hasUpdate := false

	for key, value := range convertInterfaceToMap(value, true) {
		if field, ok := scope.FieldByName(key); ok && scope.changeableField(field) {
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
func getValueFromFields(s StrSlice, value reflect.Value) []interface{} {
	var results []interface{}
	// If value is a nil pointer, Indirect returns a zero Value!
	// Therefor we need to check for a zero value,
	// as FieldByName could panic
	if indirectValue := reflect.Indirect(value); indirectValue.IsValid() {
		for _, fieldName := range s {
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
func getSearchMap(jth JoinTableHandler, con *DBCon, sources ...interface{}) map[string]interface{} {
	values := map[string]interface{}{}

	for _, source := range sources {
		scope := con.NewScope(source)
		modelType := scope.GetModelStruct().ModelType

		if jth.Source.ModelType == modelType {
			for _, foreignKey := range jth.Source.ForeignKeys {
				if field, ok := scope.FieldByName(foreignKey.AssociationDBName); ok {
					values[foreignKey.DBName] = field.Value.Interface()
				}
			}
		} else if jth.Destination.ModelType == modelType {
			for _, foreignKey := range jth.Destination.ForeignKeys {
				if field, ok := scope.FieldByName(foreignKey.AssociationDBName); ok {
					values[foreignKey.DBName] = field.Value.Interface()
				}
			}
		}
	}
	return values
}

//using inline advantage
// IndirectValue return scope's reflect value's indirect value
func IndirectValue(scope *Scope) reflect.Value {
	reflectValue := reflect.ValueOf(scope.Value)
	for reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
}

//using inline advantage
func FieldValue(value reflect.Value, index int) reflect.Value {
	reflectValue := value.Index(index)
	for reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
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
		dialect:  conDialect,
		logger:   defaultLogger,
		callback: &Callback{},
		settings: map[uint64]interface{}{},
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
