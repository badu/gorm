package gorm

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

//============================================
//Callback common functions
//============================================

// getRIndex get right index from string slice
func getRIndex(strs []string, str string) int {
	for i := len(strs) - 1; i >= 0; i-- {
		if strs[i] == str {
			return i
		}
	}
	return -1
}

// sortProcessors sort callback processors based on its before, after, remove, replace
func sortProcessors(cps []*CallbackProcessor) []*func(scope *Scope) {
	var (
		allNames, sortedNames []string
		sortCallbackProcessor func(c *CallbackProcessor)
	)

	for _, cp := range cps {
		// show warning message the callback name already exists
		if index := getRIndex(allNames, cp.name); index > -1 && !cp.replace && !cp.remove {
			fmt.Printf("[warning] duplicated callback `%v` from %v\n", cp.name, fileWithLineNum())
		}
		allNames = append(allNames, cp.name)
	}

	sortCallbackProcessor = func(c *CallbackProcessor) {
		if getRIndex(sortedNames, c.name) == -1 { // if not sorted
			if c.before != "" { // if defined before callback
				if index := getRIndex(sortedNames, c.before); index != -1 {
					// if before callback already sorted, append current callback just after it
					sortedNames = append(sortedNames[:index], append([]string{c.name}, sortedNames[index:]...)...)
				} else if index := getRIndex(allNames, c.before); index != -1 {
					// if before callback exists but haven't sorted, append current callback to last
					sortedNames = append(sortedNames, c.name)
					sortCallbackProcessor(cps[index])
				}
			}

			if c.after != "" { // if defined after callback
				if index := getRIndex(sortedNames, c.after); index != -1 {
					// if after callback already sorted, append current callback just before it
					sortedNames = append(sortedNames[:index+1], append([]string{c.name}, sortedNames[index+1:]...)...)
				} else if index := getRIndex(allNames, c.after); index != -1 {
					// if after callback exists but haven't sorted
					cp := cps[index]
					// set after callback's before callback to current callback
					if cp.before == "" {
						cp.before = c.name
					}
					sortCallbackProcessor(cp)
				}
			}

			// if current callback haven't been sorted, append it to last
			if getRIndex(sortedNames, c.name) == -1 {
				sortedNames = append(sortedNames, c.name)
			}
		}
	}

	for _, cp := range cps {
		sortCallbackProcessor(cp)
	}

	var sortedFuncs []*func(scope *Scope)
	for _, name := range sortedNames {
		if index := getRIndex(allNames, name); !cps[index].remove {
			sortedFuncs = append(sortedFuncs, cps[index].processor)
		}
	}

	return sortedFuncs
}

//============================================
//Callback create functions
//============================================

// beforeCreateCallback will invoke `BeforeSave`, `BeforeCreate` method before creating
func beforeCreateCallback(scope *Scope) {
	if !scope.HasError() {
		scope.CallMethod("BeforeSave")
	}
	if !scope.HasError() {
		scope.CallMethod("BeforeCreate")
	}
}

// updateTimeStampForCreateCallback will set `CreatedAt`, `UpdatedAt` when creating
func updateTimeStampForCreateCallback(scope *Scope) {
	if !scope.HasError() {
		now := NowFunc()
		scope.SetColumn("CreatedAt", now)
		scope.SetColumn("UpdatedAt", now)
	}
}

// createCallback the callback used to insert data into database
func createCallback(scope *Scope) {
	if !scope.HasError() {
		defer scope.trace(NowFunc())

		var (
			columns, placeholders        []string
			blankColumnsWithDefaultValue []string
		)

		for _, field := range scope.Fields() {
			if scope.changeableField(field) {
				if field.IsNormal {
					if field.IsBlank && field.HasDefaultValue {
						blankColumnsWithDefaultValue = append(blankColumnsWithDefaultValue, scope.Quote(field.DBName))
						scope.InstanceSet("gorm:blank_columns_with_default_value", blankColumnsWithDefaultValue)
					} else if !field.IsPrimaryKey || !field.IsBlank {
						columns = append(columns, scope.Quote(field.DBName))
						placeholders = append(placeholders, scope.AddToVars(field.Field.Interface()))
					}
				} else if field.Relationship != nil && field.Relationship.Kind == BELONGS_TO {
					for _, foreignKey := range field.Relationship.ForeignDBNames {
						if foreignField, ok := scope.FieldByName(foreignKey); ok && !scope.changeableField(foreignField) {
							columns = append(columns, scope.Quote(foreignField.DBName))
							placeholders = append(placeholders, scope.AddToVars(foreignField.Field.Interface()))
						}
					}
				}
			}
		}

		var (
			returningColumn = "*"
			quotedTableName = scope.QuotedTableName()
			primaryField    = scope.PrimaryField()
			extraOption     string
		)

		if str, ok := scope.Get("gorm:insert_option"); ok {
			extraOption = fmt.Sprint(str)
		}

		if primaryField != nil {
			returningColumn = scope.Quote(primaryField.DBName)
		}

		lastInsertIDReturningSuffix := scope.Dialect().LastInsertIDReturningSuffix(quotedTableName, returningColumn)

		if len(columns) == 0 {
			scope.Raw(fmt.Sprintf(
				"INSERT INTO %v DEFAULT VALUES%v%v",
				quotedTableName,
				addExtraSpaceIfExist(extraOption),
				addExtraSpaceIfExist(lastInsertIDReturningSuffix),
			))
		} else {
			scope.Raw(fmt.Sprintf(
				"INSERT INTO %v (%v) VALUES (%v)%v%v",
				scope.QuotedTableName(),
				strings.Join(columns, ","),
				strings.Join(placeholders, ","),
				addExtraSpaceIfExist(extraOption),
				addExtraSpaceIfExist(lastInsertIDReturningSuffix),
			))
		}

		// execute create sql
		if lastInsertIDReturningSuffix == "" || primaryField == nil {
			if result, err := scope.SQLDB().Exec(scope.SQL, scope.SQLVars...); scope.Err(err) == nil {
				// set rows affected count
				scope.db.RowsAffected, _ = result.RowsAffected()

				// set primary value to primary field
				if primaryField != nil && primaryField.IsBlank {
					if primaryValue, err := result.LastInsertId(); scope.Err(err) == nil {
						scope.Err(primaryField.Set(primaryValue))
					}
				}
			}
		} else {
			if err := scope.SQLDB().QueryRow(scope.SQL, scope.SQLVars...).Scan(primaryField.Field.Addr().Interface()); scope.Err(err) == nil {
				primaryField.IsBlank = false
				scope.db.RowsAffected = 1
			}
		}
	}
}

// forceReloadAfterCreateCallback will reload columns that having default value, and set it back to current object
func forceReloadAfterCreateCallback(scope *Scope) {
	if blankColumnsWithDefaultValue, ok := scope.InstanceGet("gorm:blank_columns_with_default_value"); ok {
		db := scope.DB().New().Table(scope.TableName()).Select(blankColumnsWithDefaultValue.([]string))
		for _, field := range scope.Fields() {
			if field.IsPrimaryKey && !field.IsBlank {
				db = db.Where(fmt.Sprintf("%v = ?", field.DBName), field.Field.Interface())
			}
		}
		db.Scan(scope.Value)
	}
}

// afterCreateCallback will invoke `AfterCreate`, `AfterSave` method after creating
func afterCreateCallback(scope *Scope) {
	if !scope.HasError() {
		scope.CallMethod("AfterCreate")
	}
	if !scope.HasError() {
		scope.CallMethod("AfterSave")
	}
}

//============================================
// Callback save functions
//============================================

func beginTransactionCallback(scope *Scope) {
	scope.Begin()
}

func commitOrRollbackTransactionCallback(scope *Scope) {
	scope.CommitOrRollback()
}

func saveBeforeAssociationsCallback(scope *Scope) {
	if !scope.shouldSaveAssociations() {
		return
	}
	for _, field := range scope.Fields() {
		if scope.changeableField(field) && !field.IsBlank && !field.IsIgnored {
			if relationship := field.Relationship; relationship != nil && relationship.Kind == BELONGS_TO {
				fieldValue := field.Field.Addr().Interface()
				scope.Err(scope.NewDB().Save(fieldValue).Error)
				if len(relationship.ForeignFieldNames) != 0 {
					// set value's foreign key
					for idx, fieldName := range relationship.ForeignFieldNames {
						associationForeignName := relationship.AssociationForeignDBNames[idx]
						if foreignField, ok := scope.New(fieldValue).FieldByName(associationForeignName); ok {
							scope.Err(scope.SetColumn(fieldName, foreignField.Field.Interface()))
						}
					}
				}
			}
		}
	}
}

func saveAfterAssociationsCallback(scope *Scope) {
	if !scope.shouldSaveAssociations() {
		return
	}
	for _, field := range scope.Fields() {
		if scope.changeableField(field) && !field.IsBlank && !field.IsIgnored {
			if relationship := field.Relationship; relationship != nil && relationship.Kind <= HAS_ONE {
				value := field.Field

				switch value.Kind() {
				case reflect.Slice:
					for i := 0; i < value.Len(); i++ {
						newDB := scope.NewDB()
						elem := value.Index(i).Addr().Interface()
						newScope := newDB.NewScope(elem)

						if relationship.JoinTableHandler == nil && len(relationship.ForeignFieldNames) != 0 {
							for idx, fieldName := range relationship.ForeignFieldNames {
								associationForeignName := relationship.AssociationForeignDBNames[idx]
								if f, ok := scope.FieldByName(associationForeignName); ok {
									scope.Err(newScope.SetColumn(fieldName, f.Field.Interface()))
								}
							}
						}

						if relationship.PolymorphicType != "" {
							scope.Err(newScope.SetColumn(relationship.PolymorphicType, relationship.PolymorphicValue))
						}

						scope.Err(newDB.Save(elem).Error)

						if joinTableHandler := relationship.JoinTableHandler; joinTableHandler != nil {
							scope.Err(joinTableHandler.Add(joinTableHandler, newDB, scope.Value, newScope.Value))
						}
					}
				default:
					elem := value.Addr().Interface()
					newScope := scope.New(elem)
					if len(relationship.ForeignFieldNames) != 0 {
						for idx, fieldName := range relationship.ForeignFieldNames {
							associationForeignName := relationship.AssociationForeignDBNames[idx]
							if f, ok := scope.FieldByName(associationForeignName); ok {
								scope.Err(newScope.SetColumn(fieldName, f.Field.Interface()))
							}
						}
					}

					if relationship.PolymorphicType != "" {
						scope.Err(newScope.SetColumn(relationship.PolymorphicType, relationship.PolymorphicValue))
					}
					scope.Err(scope.NewDB().Save(elem).Error)
				}
			}
		}
	}
}

//============================================
// Callback update functions
//============================================

// assignUpdatingAttributesCallback assign updating attributes to model
func assignUpdatingAttributesCallback(scope *Scope) {
	if attrs, ok := scope.InstanceGet("gorm:update_interface"); ok {
		if updateMaps, hasUpdate := scope.updatedAttrsWithValues(attrs); hasUpdate {
			scope.InstanceSet("gorm:update_attrs", updateMaps)
		} else {
			scope.SkipLeft()
		}
	}
}

// beforeUpdateCallback will invoke `BeforeSave`, `BeforeUpdate` method before updating
func beforeUpdateCallback(scope *Scope) {
	if _, ok := scope.Get("gorm:update_column"); !ok {
		if !scope.HasError() {
			scope.CallMethod("BeforeSave")
		}
		if !scope.HasError() {
			scope.CallMethod("BeforeUpdate")
		}
	}
}

// updateTimeStampForUpdateCallback will set `UpdatedAt` when updating
func updateTimeStampForUpdateCallback(scope *Scope) {
	if _, ok := scope.Get("gorm:update_column"); !ok {
		scope.SetColumn("UpdatedAt", NowFunc())
	}
}

// updateCallback the callback used to update data to database
func updateCallback(scope *Scope) {
	if !scope.HasError() {
		var sqls []string

		if updateAttrs, ok := scope.InstanceGet("gorm:update_attrs"); ok {
			for column, value := range updateAttrs.(map[string]interface{}) {
				sqls = append(sqls, fmt.Sprintf("%v = %v", scope.Quote(column), scope.AddToVars(value)))
			}
		} else {
			for _, field := range scope.Fields() {
				if scope.changeableField(field) {
					if !field.IsPrimaryKey && field.IsNormal {
						sqls = append(sqls, fmt.Sprintf("%v = %v", scope.Quote(field.DBName), scope.AddToVars(field.Field.Interface())))
					} else if relationship := field.Relationship; relationship != nil && relationship.Kind == BELONGS_TO {
						for _, foreignKey := range relationship.ForeignDBNames {
							if foreignField, ok := scope.FieldByName(foreignKey); ok && !scope.changeableField(foreignField) {
								sqls = append(sqls,
									fmt.Sprintf("%v = %v", scope.Quote(foreignField.DBName), scope.AddToVars(foreignField.Field.Interface())))
							}
						}
					}
				}
			}
		}

		var extraOption string
		if str, ok := scope.Get("gorm:update_option"); ok {
			extraOption = fmt.Sprint(str)
		}

		if len(sqls) > 0 {
			scope.Raw(fmt.Sprintf(
				"UPDATE %v SET %v%v%v",
				scope.QuotedTableName(),
				strings.Join(sqls, ", "),
				addExtraSpaceIfExist(scope.CombinedConditionSql()),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		}
	}
}

// afterUpdateCallback will invoke `AfterUpdate`, `AfterSave` method after updating
func afterUpdateCallback(scope *Scope) {
	if _, ok := scope.Get("gorm:update_column"); !ok {
		if !scope.HasError() {
			scope.CallMethod("AfterUpdate")
		}
		if !scope.HasError() {
			scope.CallMethod("AfterSave")
		}
	}
}

//============================================
// Callback query functions
//============================================

// queryCallback used to query data from database
func queryCallback(scope *Scope) {
	defer scope.trace(NowFunc())

	var (
		isSlice, isPtr bool
		resultType     reflect.Type
		results        = scope.IndirectValue()
	)

	if orderBy, ok := scope.Get("gorm:order_by_primary_key"); ok {
		if primaryField := scope.PrimaryField(); primaryField != nil {
			scope.Search.Order(fmt.Sprintf("%v.%v %v", scope.QuotedTableName(), scope.Quote(primaryField.DBName), orderBy))
		}
	}

	if value, ok := scope.Get("gorm:query_destination"); ok {
		results = reflect.Indirect(reflect.ValueOf(value))
	}

	if kind := results.Kind(); kind == reflect.Slice {
		isSlice = true
		resultType = results.Type().Elem()
		results.Set(reflect.MakeSlice(results.Type(), 0, 0))

		if resultType.Kind() == reflect.Ptr {
			isPtr = true
			resultType = resultType.Elem()
		}
	} else if kind != reflect.Struct {
		scope.Err(errors.New("unsupported destination, should be slice or struct"))
		return
	}

	scope.prepareQuerySQL()

	if !scope.HasError() {
		scope.db.RowsAffected = 0
		if str, ok := scope.Get("gorm:query_option"); ok {
			scope.SQL += addExtraSpaceIfExist(fmt.Sprint(str))
		}

		if rows, err := scope.SQLDB().Query(scope.SQL, scope.SQLVars...); scope.Err(err) == nil {
			defer rows.Close()

			columns, _ := rows.Columns()
			for rows.Next() {
				scope.db.RowsAffected++

				elem := results
				if isSlice {
					elem = reflect.New(resultType).Elem()
				}

				scope.scan(rows, columns, scope.New(elem.Addr().Interface()).Fields())

				if isSlice {
					if isPtr {
						results.Set(reflect.Append(results, elem.Addr()))
					} else {
						results.Set(reflect.Append(results, elem))
					}
				}
			}

			if scope.db.RowsAffected == 0 && !isSlice {
				scope.Err(ErrRecordNotFound)
			}
		}
	}
}

// afterQueryCallback will invoke `AfterFind` method after querying
func afterQueryCallback(scope *Scope) {
	if !scope.HasError() {
		scope.CallMethod("AfterFind")
	}
}

//============================================
// Callback query preload function
//============================================

// preloadCallback used to preload associations
func preloadCallback(scope *Scope) {
	if scope.Search.preload == nil || scope.HasError() {
		return
	}

	var (
		preloadedMap = map[string]bool{}
		fields       = scope.Fields()
	)

	for _, preload := range scope.Search.preload {
		var (
			preloadFields = strings.Split(preload.schema, ".")
			currentScope  = scope
			currentFields = fields
		)

		for idx, preloadField := range preloadFields {
			var currentPreloadConditions []interface{}

			if currentScope == nil {
				continue
			}

			// if not preloaded
			if preloadKey := strings.Join(preloadFields[:idx+1], "."); !preloadedMap[preloadKey] {

				// assign search conditions to last preload
				if idx == len(preloadFields)-1 {
					currentPreloadConditions = preload.conditions
				}

				for _, field := range currentFields {
					if field.Name != preloadField || field.Relationship == nil {
						continue
					}

					switch field.Relationship.Kind {
					case HAS_ONE:
						currentScope.handleHasOnePreload(field, currentPreloadConditions)
					case HAS_MANY:
						currentScope.handleHasManyPreload(field, currentPreloadConditions)
					case BELONGS_TO:
						currentScope.handleBelongsToPreload(field, currentPreloadConditions)
					case MANY_TO_MANY:
						currentScope.handleManyToManyPreload(field, currentPreloadConditions)
					default:
						scope.Err(errors.New("unsupported relation"))
					}

					preloadedMap[preloadKey] = true
					break
				}

				if !preloadedMap[preloadKey] {
					scope.Err(fmt.Errorf("can't preload field %s for %s", preloadField, currentScope.GetModelStruct().ModelType))
					return
				}
			}

			// preload next level
			if idx < len(preloadFields)-1 {
				currentScope = currentScope.getColumnAsScope(preloadField)
				if currentScope != nil {
					currentFields = currentScope.Fields()
				}
			}
		}
	}
}

//============================================
// Callback delete functions
//============================================

// beforeDeleteCallback will invoke `BeforeDelete` method before deleting
func beforeDeleteCallback(scope *Scope) {
	if !scope.HasError() {
		scope.CallMethod("BeforeDelete")
	}
}

// deleteCallback used to delete data from database or set deleted_at to current time (when using with soft delete)
func deleteCallback(scope *Scope) {
	if !scope.HasError() {
		var extraOption string
		if str, ok := scope.Get("gorm:delete_option"); ok {
			extraOption = fmt.Sprint(str)
		}

		if !scope.Search.Unscoped && scope.HasColumn("DeletedAt") {
			scope.Raw(fmt.Sprintf(
				"UPDATE %v SET deleted_at=%v%v%v",
				scope.QuotedTableName(),
				scope.AddToVars(NowFunc()),
				addExtraSpaceIfExist(scope.CombinedConditionSql()),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		} else {
			scope.Raw(fmt.Sprintf(
				"DELETE FROM %v%v%v",
				scope.QuotedTableName(),
				addExtraSpaceIfExist(scope.CombinedConditionSql()),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		}
	}
}

// afterDeleteCallback will invoke `AfterDelete` method after deleting
func afterDeleteCallback(scope *Scope) {
	if !scope.HasError() {
		scope.CallMethod("AfterDelete")
	}
}

//============================================
// Dialect functions
//============================================

func newDialect(name string, db *sql.DB) Dialect {
	if value, ok := dialectsMap[name]; ok {
		dialect := reflect.New(reflect.TypeOf(value).Elem()).Interface().(Dialect)
		dialect.SetDB(db)
		return dialect
	}

	fmt.Printf("`%v` is not officially supported, running under compatibility mode.\n", name)
	commontDialect := &commonDialect{}
	commontDialect.SetDB(db)
	return commontDialect
}

// RegisterDialect register new dialect
func RegisterDialect(name string, dialect Dialect) {
	dialectsMap[name] = dialect
}

// ParseFieldStructForDialect parse field struct for dialect
func ParseFieldStructForDialect(field *StructField) (fieldValue reflect.Value, sqlType string, size int, additionalType string) {
	// Get redirected field type
	var reflectType = field.Struct.Type
	for reflectType.Kind() == reflect.Ptr {
		reflectType = reflectType.Elem()
	}

	// Get redirected field value
	fieldValue = reflect.Indirect(reflect.New(reflectType))

	// Get scanner's real value
	var getScannerValue func(reflect.Value)
	getScannerValue = func(value reflect.Value) {
		fieldValue = value
		if _, isScanner := reflect.New(fieldValue.Type()).Interface().(sql.Scanner); isScanner && fieldValue.Kind() == reflect.Struct {
			getScannerValue(fieldValue.Field(0))
		}
	}
	getScannerValue(fieldValue)

	// Default Size
	if num, ok := field.TagSettings["SIZE"]; ok {
		size, _ = strconv.Atoi(num)
	} else {
		size = 255
	}

	// Default type from tag setting
	additionalType = field.TagSettings["NOT NULL"] + " " + field.TagSettings["UNIQUE"]
	if value, ok := field.TagSettings["DEFAULT"]; ok {
		additionalType = additionalType + " DEFAULT " + value
	}

	return fieldValue, field.TagSettings["TYPE"], size, strings.TrimSpace(additionalType)
}

func isByteArrayOrSlice(value reflect.Value) bool {
	return (value.Kind() == reflect.Array || value.Kind() == reflect.Slice) && value.Type().Elem() == reflect.TypeOf(uint8(0))
}

func isUUID(value reflect.Value) bool {
	if value.Kind() != reflect.Array || value.Type().Len() != 16 {
		return false
	}
	typename := value.Type().Name()
	lower := strings.ToLower(typename)
	return "uuid" == lower || "guid" == lower
}

//============================================
// Other utils functions
//============================================

func newSafeMap() *safeMap {
	return &safeMap{l: new(sync.RWMutex), m: make(map[string]string)}
}

func isPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

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
				attrs[ToDBName(key.Interface().(string))] = reflectValue.MapIndex(key).Interface()
			}
		default:
			for _, field := range (&Scope{Value: values}).Fields() {
				if !field.IsBlank && (withIgnoredField || !field.IsIgnored) {
					attrs[field.DBName] = field.Field.Interface()
				}
			}
		}
	}
	return attrs
}

// ToDBName convert string to db name
func ToDBName(name string) string {
	if v := smap.Get(name); v != "" {
		return v
	}

	if name == "" {
		return ""
	}

	var (
		value                        = commonInitialismsReplacer.Replace(name)
		buf                          = bytes.NewBufferString("")
		lastCase, currCase, nextCase strCase
	)

	for i, v := range value[:len(value)-1] {
		nextCase = strCase(value[i+1] >= 'A' && value[i+1] <= 'Z')
		if i > 0 {
			if currCase == upper {
				if lastCase == upper && nextCase == upper {
					buf.WriteRune(v)
				} else {
					if value[i-1] != '_' && value[i+1] != '_' {
						buf.WriteRune('_')
					}
					buf.WriteRune(v)
				}
			} else {
				buf.WriteRune(v)
			}
		} else {
			currCase = upper
			buf.WriteRune(v)
		}
		lastCase = currCase
		currCase = nextCase
	}

	buf.WriteByte(value[len(value)-1])

	s := strings.ToLower(buf.String())
	smap.Set(name, s)
	return s
}

// Expr generate raw SQL expression, for example:
//     DB.Model(&product).Update("price", gorm.Expr("price * ? + ?", 2, 100))
func Expr(expression string, args ...interface{}) *expr {
	return &expr{expr: expression, args: args}
}

func indirect(reflectValue reflect.Value) reflect.Value {
	for reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
}

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

func toQueryCondition(scope *Scope, columns []string) string {
	var newColumns []string
	for _, column := range columns {
		newColumns = append(newColumns, scope.Quote(column))
	}

	if len(columns) > 1 {
		return fmt.Sprintf("(%v)", strings.Join(newColumns, ","))
	}
	return strings.Join(newColumns, ",")
}

func toQueryValues(values [][]interface{}) (results []interface{}) {
	for _, value := range values {
		for _, v := range value {
			results = append(results, v)
		}
	}
	return
}

func fileWithLineNum() string {
	for i := 2; i < 15; i++ {
		_, file, line, ok := runtime.Caller(i)
		if ok && (!regexp.MustCompile(`jinzhu/gorm/.*.go`).MatchString(file) || regexp.MustCompile(`jinzhu/gorm/.*test.go`).MatchString(file)) {
			return fmt.Sprintf("%v:%v", file, line)
		}
	}
	return ""
}

func isBlank(value reflect.Value) bool {
	return reflect.DeepEqual(value.Interface(), reflect.Zero(value.Type()).Interface())
}

func toSearchableMap(attrs ...interface{}) (result interface{}) {
	if len(attrs) > 1 {
		if str, ok := attrs[0].(string); ok {
			result = map[string]interface{}{str: attrs[1]}
		}
	} else if len(attrs) == 1 {
		if attr, ok := attrs[0].(map[string]interface{}); ok {
			result = attr
		}

		if attr, ok := attrs[0].(interface{}); ok {
			result = attr
		}
	}
	return
}

func equalAsString(a interface{}, b interface{}) bool {
	return toString(a) == toString(b)
}

func toString(str interface{}) string {
	if values, ok := str.([]interface{}); ok {
		var results []string
		for _, value := range values {
			results = append(results, toString(value))
		}
		return strings.Join(results, "_")
	} else if bytes, ok := str.([]byte); ok {
		return string(bytes)
	} else if reflectValue := reflect.Indirect(reflect.ValueOf(str)); reflectValue.IsValid() {
		return fmt.Sprintf("%v", reflectValue.Interface())
	}
	return ""
}

func makeSlice(elemType reflect.Type) interface{} {
	if elemType.Kind() == reflect.Slice {
		elemType = elemType.Elem()
	}
	sliceType := reflect.SliceOf(elemType)
	slice := reflect.New(sliceType)
	slice.Elem().Set(reflect.MakeSlice(sliceType, 0, 0))
	return slice.Interface()
}

func strInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// getValueFromFields return given fields's value
func getValueFromFields(value reflect.Value, fieldNames []string) (results []interface{}) {
	// If value is a nil pointer, Indirect returns a zero Value!
	// Therefor we need to check for a zero value,
	// as FieldByName could panic
	if indirectValue := reflect.Indirect(value); indirectValue.IsValid() {
		for _, fieldName := range fieldNames {
			if fieldValue := indirectValue.FieldByName(fieldName); fieldValue.IsValid() {
				result := fieldValue.Interface()
				if r, ok := result.(driver.Valuer); ok {
					result, _ = r.Value()
				}
				results = append(results, result)
			}
		}
	}
	return
}

func addExtraSpaceIfExist(str string) string {
	if str != "" {
		return " " + str
	}
	return ""
}

func newModelStructsMap() *safeModelStructsMap {
	return &safeModelStructsMap{l: new(sync.RWMutex), m: make(map[reflect.Type]*ModelStruct)}
}

func getForeignField(column string, fields []*StructField) *StructField {
	for _, field := range fields {
		if field.Name == column || field.DBName == column || field.DBName == ToDBName(column) {
			return field
		}
	}
	return nil
}

func parseTagSetting(tags reflect.StructTag) map[string]string {
	setting := map[string]string{}
	for _, str := range []string{tags.Get("sql"), tags.Get("gorm")} {
		tags := strings.Split(str, ";")
		for _, value := range tags {
			v := strings.Split(value, ":")
			k := strings.TrimSpace(strings.ToUpper(v[0]))
			if len(v) >= 2 {
				setting[k] = strings.Join(v[1:], ":")
			} else {
				setting[k] = k
			}
		}
	}
	return setting
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
func Open(dialect string, args ...interface{}) (*DB, error) {
	var db DB
	var err error

	if len(args) == 0 {
		err = errors.New("invalid database source")
		return nil, err
	}
	var source string
	var dbSQL sqlCommon

	switch value := args[0].(type) {
	case string:
		var driver = dialect
		if len(args) == 1 {
			source = value
		} else if len(args) >= 2 {
			driver = value
			source = args[1].(string)
		}
		dbSQL, err = sql.Open(driver, source)
	case sqlCommon:
		source = reflect.Indirect(reflect.ValueOf(value)).FieldByName("dsn").String()
		dbSQL = value
	}

	db = DB{
		dialect:   newDialect(dialect, dbSQL.(*sql.DB)),
		logger:    defaultLogger,
		callbacks: DefaultCallback,
		source:    source,
		values:    map[string]interface{}{},
		db:        dbSQL,
	}
	db.parent = &db

	if err == nil {
		err = db.DB().Ping() // Send a ping to make sure the database connection is alive.
		if err != nil {
			db.DB().Close()
		}
	}

	return &db, err
}
