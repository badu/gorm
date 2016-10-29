package gorm

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/inflection"
	"go/ast"
	"reflect"
)

// IndirectValue return scope's reflect value's indirect value
func (scope *Scope) IndirectValue() reflect.Value {
	return indirect(reflect.ValueOf(scope.Value))
}

// New create a new Scope without search information
func (scope *Scope) New(value interface{}) *Scope {
	return &Scope{db: scope.NewDB(), Search: &search{}, Value: value}
}

////////////////////////////////////////////////////////////////////////////////
// Scope DB
////////////////////////////////////////////////////////////////////////////////

// DB return scope's DB connection
func (scope *Scope) DB() *DB {
	return scope.db
}

// NewDB create a new DB without search information
func (scope *Scope) NewDB() *DB {
	if scope.db != nil {
		//TODO : @Badu - so, we clone it then we set stuff to nil ?
		db := scope.db.clone()
		db.search = nil
		db.Value = nil
		return db
	}
	return nil
}

// SQLDB return *sql.DB
func (scope *Scope) SQLDB() sqlCommon {
	return scope.db.db
}

// Dialect get dialect
func (scope *Scope) Dialect() Dialect {
	return scope.db.parent.dialect
}

// Quote used to quote string to escape them for database
func (scope *Scope) Quote(str string) string {
	if strings.Index(str, ".") != -1 {
		newStrs := []string{}
		for _, str := range strings.Split(str, ".") {
			newStrs = append(newStrs, scope.Dialect().Quote(str))
		}
		return strings.Join(newStrs, ".")
	}

	return scope.Dialect().Quote(str)
}

// Err add error to Scope
func (scope *Scope) Err(err error) error {
	if err != nil {
		scope.db.AddError(err)
	}
	return err
}

// HasError check if there are any error
func (scope *Scope) HasError() bool {
	return scope.db.Error != nil
}

// Log print log message
func (scope *Scope) Log(v ...interface{}) {
	scope.db.log(v...)
}

// SkipLeft skip remaining callbacks
func (scope *Scope) SkipLeft() {
	scope.skipLeft = true
}

// Fields get value's fields
func (scope *Scope) Fields() []*Field {
	if scope.fields == nil {
		var (
			fields             []*Field
			indirectScopeValue = scope.IndirectValue()
			isStruct           = indirectScopeValue.Kind() == reflect.Struct
		)

		for _, structField := range scope.GetModelStruct().StructFields {
			if isStruct {
				fieldValue := indirectScopeValue
				for _, name := range structField.Names {
					fieldValue = reflect.Indirect(fieldValue).FieldByName(name)
				}
				fields = append(fields, &Field{StructField: structField, Field: fieldValue, IsBlank: isBlank(fieldValue)})
			} else {
				fields = append(fields, &Field{StructField: structField, IsBlank: true})
			}
		}
		scope.fields = &fields
	}

	return *scope.fields
}

// FieldByName find `gorm.Field` with field name or db name
func (scope *Scope) FieldByName(name string) (field *Field, ok bool) {
	var (
		dbName           = ToDBName(name)
		mostMatchedField *Field
	)

	for _, field := range scope.Fields() {
		if field.Name == name || field.DBName == name {
			return field, true
		}
		if field.DBName == dbName {
			mostMatchedField = field
		}
	}
	return mostMatchedField, mostMatchedField != nil
}

// PrimaryFields return scope's primary fields
func (scope *Scope) PrimaryFields() (fields []*Field) {
	for _, field := range scope.Fields() {
		if field.IsPrimaryKey {
			fields = append(fields, field)
		}
	}
	return fields
}

// PrimaryField return scope's main primary field, if defined more that one primary fields, will return the one having column name `id` or the first one
func (scope *Scope) PrimaryField() *Field {
	if primaryFields := scope.GetModelStruct().PrimaryFields; len(primaryFields) > 0 {
		if len(primaryFields) > 1 {
			if field, ok := scope.FieldByName("id"); ok {
				return field
			}
		}
		return scope.PrimaryFields()[0]
	}
	return nil
}

// PrimaryKey get main primary field's db name
func (scope *Scope) PrimaryKey() string {
	if field := scope.PrimaryField(); field != nil {
		return field.DBName
	}
	return ""
}

// PrimaryKeyZero check main primary field's value is blank or not
func (scope *Scope) PrimaryKeyZero() bool {
	field := scope.PrimaryField()
	return field == nil || field.IsBlank
}

// PrimaryKeyValue get the primary key's value
func (scope *Scope) PrimaryKeyValue() interface{} {
	if field := scope.PrimaryField(); field != nil && field.Field.IsValid() {
		return field.Field.Interface()
	}
	return 0
}

// HasColumn to check if has column
func (scope *Scope) HasColumn(column string) bool {
	for _, field := range scope.GetStructFields() {
		if field.IsNormal && (field.Name == column || field.DBName == column) {
			return true
		}
	}
	return false
}

// SetColumn to set the column's value, column could be field or field's name/dbname
func (scope *Scope) SetColumn(column interface{}, value interface{}) error {
	var updateAttrs = map[string]interface{}{}
	if attrs, ok := scope.InstanceGet("gorm:update_attrs"); ok {
		updateAttrs = attrs.(map[string]interface{})
		defer scope.InstanceSet("gorm:update_attrs", updateAttrs)
	}

	if field, ok := column.(*Field); ok {
		updateAttrs[field.DBName] = value
		return field.Set(value)
	} else if name, ok := column.(string); ok {
		var (
			dbName           = ToDBName(name)
			mostMatchedField *Field
		)
		for _, field := range scope.Fields() {
			if field.DBName == value {
				updateAttrs[field.DBName] = value
				return field.Set(value)
			}
			if (field.DBName == dbName) || (field.Name == name && mostMatchedField == nil) {
				mostMatchedField = field
			}
		}

		if mostMatchedField != nil {
			updateAttrs[mostMatchedField.DBName] = value
			return mostMatchedField.Set(value)
		}
	}
	return errors.New("could not convert column to field")
}

// CallMethod call scope value's method, if it is a slice, will call its element's method one by one
func (scope *Scope) CallMethod(methodName string) {
	if scope.Value == nil {
		return
	}

	if indirectScopeValue := scope.IndirectValue(); indirectScopeValue.Kind() == reflect.Slice {
		for i := 0; i < indirectScopeValue.Len(); i++ {
			scope.callMethod(methodName, indirectScopeValue.Index(i))
		}
	} else {
		scope.callMethod(methodName, indirectScopeValue)
	}
}

// AddToVars add value as sql's vars, used to prevent SQL injection
func (scope *Scope) AddToVars(value interface{}) string {
	if expr, ok := value.(*expr); ok {
		exp := expr.expr
		for _, arg := range expr.args {
			exp = strings.Replace(exp, "?", scope.AddToVars(arg), 1)
		}
		return exp
	}

	scope.SQLVars = append(scope.SQLVars, value)
	return scope.Dialect().BindVar(len(scope.SQLVars))
}

// SelectAttrs return selected attributes
func (scope *Scope) SelectAttrs() []string {
	if scope.selectAttrs == nil {
		attrs := []string{}
		for _, value := range scope.Search.selects {
			if str, ok := value.(string); ok {
				attrs = append(attrs, str)
			} else if strs, ok := value.([]string); ok {
				attrs = append(attrs, strs...)
			} else if strs, ok := value.([]interface{}); ok {
				for _, str := range strs {
					attrs = append(attrs, fmt.Sprintf("%v", str))
				}
			}
		}
		scope.selectAttrs = &attrs
	}
	return *scope.selectAttrs
}

// OmitAttrs return omitted attributes
func (scope *Scope) OmitAttrs() []string {
	return scope.Search.omits
}

// TableName return table name
func (scope *Scope) TableName() string {
	if scope.Search != nil && len(scope.Search.tableName) > 0 {
		return scope.Search.tableName
	}

	if tabler, ok := scope.Value.(tabler); ok {
		return tabler.TableName()
	}

	if tabler, ok := scope.Value.(dbTabler); ok {
		return tabler.TableName(scope.db)
	}

	return scope.GetModelStruct().TableName(scope.db.Model(scope.Value))
}

// QuotedTableName return quoted table name
func (scope *Scope) QuotedTableName() (name string) {
	if scope.Search != nil && len(scope.Search.tableName) > 0 {
		if strings.Index(scope.Search.tableName, " ") != -1 {
			return scope.Search.tableName
		}
		return scope.Quote(scope.Search.tableName)
	}

	return scope.Quote(scope.TableName())
}

// CombinedConditionSql return combined condition sql
func (scope *Scope) CombinedConditionSql() string {
	return scope.joinsSQL() + scope.whereSQL() + scope.groupSQL() +
		scope.havingSQL() + scope.orderSQL() + scope.limitAndOffsetSQL()
}

// Raw set raw sql
func (scope *Scope) Raw(sql string) *Scope {
	scope.SQL = strings.Replace(sql, "$$", "?", -1)
	return scope
}

// Exec perform generated SQL
func (scope *Scope) Exec() *Scope {
	defer scope.trace(NowFunc())

	if !scope.HasError() {
		if result, err := scope.SQLDB().Exec(scope.SQL, scope.SQLVars...); scope.Err(err) == nil {
			if count, err := result.RowsAffected(); scope.Err(err) == nil {
				scope.db.RowsAffected = count
			}
		}
	}
	return scope
}

// Set set value by name
func (scope *Scope) Set(name string, value interface{}) *Scope {
	scope.db.InstantSet(name, value)
	return scope
}

// Get get setting by name
func (scope *Scope) Get(name string) (interface{}, bool) {
	return scope.db.Get(name)
}

// InstanceID get InstanceID for scope
func (scope *Scope) InstanceID() string {
	if scope.instanceID == "" {
		scope.instanceID = fmt.Sprintf("%v%v", &scope, &scope.db)
	}
	return scope.instanceID
}

// InstanceSet set instance setting for current operation, but not for operations in callbacks, like saving associations callback
func (scope *Scope) InstanceSet(name string, value interface{}) *Scope {
	return scope.Set(name+scope.InstanceID(), value)
}

// InstanceGet get instance setting from current operation
func (scope *Scope) InstanceGet(name string) (interface{}, bool) {
	return scope.Get(name + scope.InstanceID())
}

// Begin start a transaction
func (scope *Scope) Begin() *Scope {
	if db, ok := scope.SQLDB().(sqlDb); ok {
		if tx, err := db.Begin(); err == nil {
			scope.db.db = interface{}(tx).(sqlCommon)
			scope.InstanceSet("gorm:started_transaction", true)
		}
	}
	return scope
}

// CommitOrRollback commit current transaction if no error happened, otherwise will rollback it
func (scope *Scope) CommitOrRollback() *Scope {
	if _, ok := scope.InstanceGet("gorm:started_transaction"); ok {
		if db, ok := scope.db.db.(sqlTx); ok {
			if scope.HasError() {
				db.Rollback()
			} else {
				scope.Err(db.Commit())
			}
			scope.db.db = scope.db.parent.db
		}
	}
	return scope
}

////////////////////////////////////////////////////////////////////////////////
// Private Methods For *gorm.Scope
////////////////////////////////////////////////////////////////////////////////

func (scope *Scope) callMethod(methodName string, reflectValue reflect.Value) {
	// Only get address from non-pointer
	if reflectValue.CanAddr() && reflectValue.Kind() != reflect.Ptr {
		reflectValue = reflectValue.Addr()
	}

	if methodValue := reflectValue.MethodByName(methodName); methodValue.IsValid() {
		switch method := methodValue.Interface().(type) {
		case func():
			method()
		case func(*Scope):
			method(scope)
		case func(*DB):
			newDB := scope.NewDB()
			method(newDB)
			scope.Err(newDB.Error)
		case func() error:
			scope.Err(method())
		case func(*Scope) error:
			scope.Err(method(scope))
		case func(*DB) error:
			newDB := scope.NewDB()
			scope.Err(method(newDB))
			scope.Err(newDB.Error)
		default:
			scope.Err(fmt.Errorf("unsupported function %v", methodName))
		}
	}
}

func (scope *Scope) quoteIfPossible(str string) string {
	if columnRegexp.MatchString(str) {
		return scope.Quote(str)
	}
	return str
}

func (scope *Scope) scan(rows *sql.Rows, columns []string, fields []*Field) {
	var (
		ignored            interface{}
		values             = make([]interface{}, len(columns))
		selectFields       []*Field
		selectedColumnsMap = map[string]int{}
		resetFields        = map[int]*Field{}
	)

	for index, column := range columns {
		values[index] = &ignored

		selectFields = fields
		if idx, ok := selectedColumnsMap[column]; ok {
			selectFields = selectFields[idx+1:]
		}

		for fieldIndex, field := range selectFields {
			if field.DBName == column {
				if field.Field.Kind() == reflect.Ptr {
					values[index] = field.Field.Addr().Interface()
				} else {
					reflectValue := reflect.New(reflect.PtrTo(field.Struct.Type))
					reflectValue.Elem().Set(field.Field.Addr())
					values[index] = reflectValue.Interface()
					resetFields[index] = field
				}

				selectedColumnsMap[column] = fieldIndex

				if field.IsNormal {
					break
				}
			}
		}
	}

	scope.Err(rows.Scan(values...))

	for index, field := range resetFields {
		if v := reflect.ValueOf(values[index]).Elem().Elem(); v.IsValid() {
			field.Field.Set(v)
		}
	}
}

func (scope *Scope) primaryCondition(value interface{}) string {
	return fmt.Sprintf("(%v.%v = %v)", scope.QuotedTableName(), scope.Quote(scope.PrimaryKey()), value)
}

func (scope *Scope) buildWhereCondition(clause map[string]interface{}) (str string) {
	switch value := clause["query"].(type) {
	case string:
		// if string is number
		if regexp.MustCompile("^\\s*\\d+\\s*$").MatchString(value) {
			return scope.primaryCondition(scope.AddToVars(value))
		} else if value != "" {
			str = fmt.Sprintf("(%v)", value)
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, sql.NullInt64:
		return scope.primaryCondition(scope.AddToVars(value))
	case []int, []int8, []int16, []int32, []int64, []uint, []uint8, []uint16, []uint32, []uint64, []string, []interface{}:
		str = fmt.Sprintf("(%v.%v IN (?))", scope.QuotedTableName(), scope.Quote(scope.PrimaryKey()))
		clause["args"] = []interface{}{value}
	case map[string]interface{}:
		var sqls []string
		for key, value := range value {
			if value != nil {
				sqls = append(sqls, fmt.Sprintf("(%v.%v = %v)", scope.QuotedTableName(), scope.Quote(key), scope.AddToVars(value)))
			} else {
				sqls = append(sqls, fmt.Sprintf("(%v.%v IS NULL)", scope.QuotedTableName(), scope.Quote(key)))
			}
		}
		return strings.Join(sqls, " AND ")
	case interface{}:
		var sqls []string
		newScope := scope.New(value)
		for _, field := range newScope.Fields() {
			if !field.IsIgnored && !field.IsBlank {
				sqls = append(sqls, fmt.Sprintf("(%v.%v = %v)", newScope.QuotedTableName(), scope.Quote(field.DBName), scope.AddToVars(field.Field.Interface())))
			}
		}
		return strings.Join(sqls, " AND ")
	}

	args := clause["args"].([]interface{})
	for _, arg := range args {
		switch reflect.ValueOf(arg).Kind() {
		case reflect.Slice: // For where("id in (?)", []int64{1,2})
			if bytes, ok := arg.([]byte); ok {
				str = strings.Replace(str, "?", scope.AddToVars(bytes), 1)
			} else if values := reflect.ValueOf(arg); values.Len() > 0 {
				var tempMarks []string
				for i := 0; i < values.Len(); i++ {
					tempMarks = append(tempMarks, scope.AddToVars(values.Index(i).Interface()))
				}
				str = strings.Replace(str, "?", strings.Join(tempMarks, ","), 1)
			} else {
				str = strings.Replace(str, "?", scope.AddToVars(Expr("NULL")), 1)
			}
		default:
			if valuer, ok := interface{}(arg).(driver.Valuer); ok {
				arg, _ = valuer.Value()
			}

			str = strings.Replace(str, "?", scope.AddToVars(arg), 1)
		}
	}
	return
}

func (scope *Scope) buildNotCondition(clause map[string]interface{}) (str string) {
	var notEqualSQL string
	var primaryKey = scope.PrimaryKey()

	switch value := clause["query"].(type) {
	case string:
		// is number
		if regexp.MustCompile("^\\s*\\d+\\s*$").MatchString(value) {
			id, _ := strconv.Atoi(value)
			return fmt.Sprintf("(%v <> %v)", scope.Quote(primaryKey), id)
		} else if regexp.MustCompile("(?i) (=|<>|>|<|LIKE|IS|IN) ").MatchString(value) {
			str = fmt.Sprintf(" NOT (%v) ", value)
			notEqualSQL = fmt.Sprintf("NOT (%v)", value)
		} else {
			str = fmt.Sprintf("(%v.%v NOT IN (?))", scope.QuotedTableName(), scope.Quote(value))
			notEqualSQL = fmt.Sprintf("(%v.%v <> ?)", scope.QuotedTableName(), scope.Quote(value))
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, sql.NullInt64:
		return fmt.Sprintf("(%v.%v <> %v)", scope.QuotedTableName(), scope.Quote(primaryKey), value)
	case []int, []int8, []int16, []int32, []int64, []uint, []uint8, []uint16, []uint32, []uint64, []string:
		if reflect.ValueOf(value).Len() > 0 {
			str = fmt.Sprintf("(%v.%v NOT IN (?))", scope.QuotedTableName(), scope.Quote(primaryKey))
			clause["args"] = []interface{}{value}
		}
		return ""
	case map[string]interface{}:
		var sqls []string
		for key, value := range value {
			if value != nil {
				sqls = append(sqls, fmt.Sprintf("(%v.%v <> %v)", scope.QuotedTableName(), scope.Quote(key), scope.AddToVars(value)))
			} else {
				sqls = append(sqls, fmt.Sprintf("(%v.%v IS NOT NULL)", scope.QuotedTableName(), scope.Quote(key)))
			}
		}
		return strings.Join(sqls, " AND ")
	case interface{}:
		var sqls []string
		var newScope = scope.New(value)
		for _, field := range newScope.Fields() {
			if !field.IsBlank {
				sqls = append(sqls, fmt.Sprintf("(%v.%v <> %v)", newScope.QuotedTableName(), scope.Quote(field.DBName), scope.AddToVars(field.Field.Interface())))
			}
		}
		return strings.Join(sqls, " AND ")
	}

	args := clause["args"].([]interface{})
	for _, arg := range args {
		switch reflect.ValueOf(arg).Kind() {
		case reflect.Slice: // For where("id in (?)", []int64{1,2})
			if bytes, ok := arg.([]byte); ok {
				str = strings.Replace(str, "?", scope.AddToVars(bytes), 1)
			} else if values := reflect.ValueOf(arg); values.Len() > 0 {
				var tempMarks []string
				for i := 0; i < values.Len(); i++ {
					tempMarks = append(tempMarks, scope.AddToVars(values.Index(i).Interface()))
				}
				str = strings.Replace(str, "?", strings.Join(tempMarks, ","), 1)
			} else {
				str = strings.Replace(str, "?", scope.AddToVars(Expr("NULL")), 1)
			}
		default:
			if scanner, ok := interface{}(arg).(driver.Valuer); ok {
				arg, _ = scanner.Value()
			}
			str = strings.Replace(notEqualSQL, "?", scope.AddToVars(arg), 1)
		}
	}
	return
}

func (scope *Scope) buildSelectQuery(clause map[string]interface{}) (str string) {
	switch value := clause["query"].(type) {
	case string:
		str = value
	case []string:
		str = strings.Join(value, ", ")
	}

	args := clause["args"].([]interface{})
	for _, arg := range args {
		switch reflect.ValueOf(arg).Kind() {
		case reflect.Slice:
			values := reflect.ValueOf(arg)
			var tempMarks []string
			for i := 0; i < values.Len(); i++ {
				tempMarks = append(tempMarks, scope.AddToVars(values.Index(i).Interface()))
			}
			str = strings.Replace(str, "?", strings.Join(tempMarks, ","), 1)
		default:
			if valuer, ok := interface{}(arg).(driver.Valuer); ok {
				arg, _ = valuer.Value()
			}
			str = strings.Replace(str, "?", scope.AddToVars(arg), 1)
		}
	}
	return
}

func (scope *Scope) whereSQL() (sql string) {
	var (
		quotedTableName                                = scope.QuotedTableName()
		primaryConditions, andConditions, orConditions []string
	)

	if !scope.Search.Unscoped && scope.HasColumn("deleted_at") {
		sql := fmt.Sprintf("%v.deleted_at IS NULL", quotedTableName)
		primaryConditions = append(primaryConditions, sql)
	}

	if !scope.PrimaryKeyZero() {
		for _, field := range scope.PrimaryFields() {
			sql := fmt.Sprintf("%v.%v = %v", quotedTableName, scope.Quote(field.DBName), scope.AddToVars(field.Field.Interface()))
			primaryConditions = append(primaryConditions, sql)
		}
	}

	for _, clause := range scope.Search.whereConditions {
		if sql := scope.buildWhereCondition(clause); sql != "" {
			andConditions = append(andConditions, sql)
		}
	}

	for _, clause := range scope.Search.orConditions {
		if sql := scope.buildWhereCondition(clause); sql != "" {
			orConditions = append(orConditions, sql)
		}
	}

	for _, clause := range scope.Search.notConditions {
		if sql := scope.buildNotCondition(clause); sql != "" {
			andConditions = append(andConditions, sql)
		}
	}

	orSQL := strings.Join(orConditions, " OR ")
	combinedSQL := strings.Join(andConditions, " AND ")
	if len(combinedSQL) > 0 {
		if len(orSQL) > 0 {
			combinedSQL = combinedSQL + " OR " + orSQL
		}
	} else {
		combinedSQL = orSQL
	}

	if len(primaryConditions) > 0 {
		sql = "WHERE " + strings.Join(primaryConditions, " AND ")
		if len(combinedSQL) > 0 {
			sql = sql + " AND (" + combinedSQL + ")"
		}
	} else if len(combinedSQL) > 0 {
		sql = "WHERE " + combinedSQL
	}
	return
}

func (scope *Scope) selectSQL() string {
	if len(scope.Search.selects) == 0 {
		if len(scope.Search.joinConditions) > 0 {
			return fmt.Sprintf("%v.*", scope.QuotedTableName())
		}
		return "*"
	}
	return scope.buildSelectQuery(scope.Search.selects)
}

func (scope *Scope) orderSQL() string {
	if len(scope.Search.orders) == 0 || scope.Search.countingQuery {
		return ""
	}

	var orders []string
	for _, order := range scope.Search.orders {
		if str, ok := order.(string); ok {
			orders = append(orders, scope.quoteIfPossible(str))
		} else if expr, ok := order.(*expr); ok {
			exp := expr.expr
			for _, arg := range expr.args {
				exp = strings.Replace(exp, "?", scope.AddToVars(arg), 1)
			}
			orders = append(orders, exp)
		}
	}
	return " ORDER BY " + strings.Join(orders, ",")
}

func (scope *Scope) limitAndOffsetSQL() string {
	return scope.Dialect().LimitAndOffsetSQL(scope.Search.limit, scope.Search.offset)
}

func (scope *Scope) groupSQL() string {
	if len(scope.Search.group) == 0 {
		return ""
	}
	return " GROUP BY " + scope.Search.group
}

func (scope *Scope) havingSQL() string {
	if len(scope.Search.havingConditions) == 0 {
		return ""
	}

	var andConditions []string
	for _, clause := range scope.Search.havingConditions {
		if sql := scope.buildWhereCondition(clause); sql != "" {
			andConditions = append(andConditions, sql)
		}
	}

	combinedSQL := strings.Join(andConditions, " AND ")
	if len(combinedSQL) == 0 {
		return ""
	}

	return " HAVING " + combinedSQL
}

func (scope *Scope) joinsSQL() string {
	var joinConditions []string
	for _, clause := range scope.Search.joinConditions {
		if sql := scope.buildWhereCondition(clause); sql != "" {
			joinConditions = append(joinConditions, strings.TrimSuffix(strings.TrimPrefix(sql, "("), ")"))
		}
	}

	return strings.Join(joinConditions, " ") + " "
}

func (scope *Scope) prepareQuerySQL() {
	if scope.Search.raw {
		scope.Raw(strings.TrimSuffix(strings.TrimPrefix(scope.CombinedConditionSql(), " WHERE ("), ")"))
	} else {
		scope.Raw(fmt.Sprintf("SELECT %v FROM %v %v", scope.selectSQL(), scope.QuotedTableName(), scope.CombinedConditionSql()))
	}
	return
}

func (scope *Scope) inlineCondition(values ...interface{}) *Scope {
	if len(values) > 0 {
		scope.Search.Where(values[0], values[1:]...)
	}
	return scope
}

func (scope *Scope) callCallbacks(funcs []*func(s *Scope)) *Scope {
	for _, f := range funcs {
		(*f)(scope)
		if scope.skipLeft {
			break
		}
	}
	return scope
}

func (scope *Scope) updatedAttrsWithValues(value interface{}) (results map[string]interface{}, hasUpdate bool) {
	if scope.IndirectValue().Kind() != reflect.Struct {
		return convertInterfaceToMap(value, false), true
	}

	results = map[string]interface{}{}

	for key, value := range convertInterfaceToMap(value, true) {
		if field, ok := scope.FieldByName(key); ok && scope.changeableField(field) {
			if _, ok := value.(*expr); ok {
				hasUpdate = true
				results[field.DBName] = value
			} else {
				err := field.Set(value)
				if field.IsNormal {
					hasUpdate = true
					if err == ErrUnaddressable {
						results[field.DBName] = value
					} else {
						results[field.DBName] = field.Field.Interface()
					}
				}
			}
		}
	}
	return
}

func (scope *Scope) row() *sql.Row {
	defer scope.trace(NowFunc())
	scope.callCallbacks(scope.db.parent.callbacks.rowQueries)
	scope.prepareQuerySQL()
	return scope.SQLDB().QueryRow(scope.SQL, scope.SQLVars...)
}

func (scope *Scope) rows() (*sql.Rows, error) {
	defer scope.trace(NowFunc())
	scope.callCallbacks(scope.db.parent.callbacks.rowQueries)
	scope.prepareQuerySQL()
	return scope.SQLDB().Query(scope.SQL, scope.SQLVars...)
}

func (scope *Scope) initialize() *Scope {
	for _, clause := range scope.Search.whereConditions {
		scope.updatedAttrsWithValues(clause["query"])
	}
	scope.updatedAttrsWithValues(scope.Search.initAttrs)
	scope.updatedAttrsWithValues(scope.Search.assignAttrs)
	return scope
}

func (scope *Scope) pluck(column string, value interface{}) *Scope {
	dest := reflect.Indirect(reflect.ValueOf(value))
	scope.Search.Select(column)
	if dest.Kind() != reflect.Slice {
		scope.Err(fmt.Errorf("results should be a slice, not %s", dest.Kind()))
		return scope
	}

	rows, err := scope.rows()
	if scope.Err(err) == nil {
		defer rows.Close()
		for rows.Next() {
			elem := reflect.New(dest.Type().Elem()).Interface()
			scope.Err(rows.Scan(elem))
			dest.Set(reflect.Append(dest, reflect.ValueOf(elem).Elem()))
		}
	}
	return scope
}

func (scope *Scope) count(value interface{}) *Scope {
	if query, ok := scope.Search.selects["query"]; !ok || !regexp.MustCompile("(?i)^count(.+)$").MatchString(fmt.Sprint(query)) {
		scope.Search.Select("count(*)")
	}
	scope.Search.countingQuery = true
	scope.Err(scope.row().Scan(value))
	return scope
}

func (scope *Scope) typeName() string {
	typ := scope.IndirectValue().Type()

	for typ.Kind() == reflect.Slice || typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	return typ.Name()
}

// trace print sql log
func (scope *Scope) trace(t time.Time) {
	if len(scope.SQL) > 0 {
		scope.db.slog(scope.SQL, t, scope.SQLVars...)
	}
}

func (scope *Scope) changeableField(field *Field) bool {
	if selectAttrs := scope.SelectAttrs(); len(selectAttrs) > 0 {
		for _, attr := range selectAttrs {
			if field.Name == attr || field.DBName == attr {
				return true
			}
		}
		return false
	}

	for _, attr := range scope.OmitAttrs() {
		if field.Name == attr || field.DBName == attr {
			return false
		}
	}

	return true
}

func (scope *Scope) shouldSaveAssociations() bool {
	if saveAssociations, ok := scope.Get("gorm:save_associations"); ok && !saveAssociations.(bool) {
		return false
	}
	return true && !scope.HasError()
}

func (scope *Scope) related(value interface{}, foreignKeys ...string) *Scope {
	toScope := scope.db.NewScope(value)

	for _, foreignKey := range append(foreignKeys, toScope.typeName()+"Id", scope.typeName()+"Id") {
		fromField, _ := scope.FieldByName(foreignKey)
		toField, _ := toScope.FieldByName(foreignKey)

		if fromField != nil {
			if relationship := fromField.Relationship; relationship != nil {
				if relationship.Kind == "many_to_many" {
					joinTableHandler := relationship.JoinTableHandler
					scope.Err(joinTableHandler.JoinWith(joinTableHandler, toScope.db, scope.Value).Find(value).Error)
				} else if relationship.Kind == "belongs_to" {
					query := toScope.db
					for idx, foreignKey := range relationship.ForeignDBNames {
						if field, ok := scope.FieldByName(foreignKey); ok {
							query = query.Where(fmt.Sprintf("%v = ?", scope.Quote(relationship.AssociationForeignDBNames[idx])), field.Field.Interface())
						}
					}
					scope.Err(query.Find(value).Error)
				} else if relationship.Kind == "has_many" || relationship.Kind == "has_one" {
					query := toScope.db
					for idx, foreignKey := range relationship.ForeignDBNames {
						if field, ok := scope.FieldByName(relationship.AssociationForeignDBNames[idx]); ok {
							query = query.Where(fmt.Sprintf("%v = ?", scope.Quote(foreignKey)), field.Field.Interface())
						}
					}

					if relationship.PolymorphicType != "" {
						query = query.Where(fmt.Sprintf("%v = ?", scope.Quote(relationship.PolymorphicDBName)), relationship.PolymorphicValue)
					}
					scope.Err(query.Find(value).Error)
				}
			} else {
				sql := fmt.Sprintf("%v = ?", scope.Quote(toScope.PrimaryKey()))
				scope.Err(toScope.db.Where(sql, fromField.Field.Interface()).Find(value).Error)
			}
			return scope
		} else if toField != nil {
			sql := fmt.Sprintf("%v = ?", scope.Quote(toField.DBName))
			scope.Err(toScope.db.Where(sql, scope.PrimaryKeyValue()).Find(value).Error)
			return scope
		}
	}

	scope.Err(fmt.Errorf("invalid association %v", foreignKeys))
	return scope
}

// getTableOptions return the table options string or an empty string if the table options does not exist
func (scope *Scope) getTableOptions() string {
	tableOptions, ok := scope.Get("gorm:table_options")
	if !ok {
		return ""
	}
	return tableOptions.(string)
}

func (scope *Scope) createJoinTable(field *StructField) {
	if relationship := field.Relationship; relationship != nil && relationship.JoinTableHandler != nil {
		joinTableHandler := relationship.JoinTableHandler
		joinTable := joinTableHandler.Table(scope.db)
		if !scope.Dialect().HasTable(joinTable) {
			toScope := &Scope{Value: reflect.New(field.Struct.Type).Interface()}

			var sqlTypes, primaryKeys []string
			for idx, fieldName := range relationship.ForeignFieldNames {
				if field, ok := scope.FieldByName(fieldName); ok {
					foreignKeyStruct := field.clone()
					foreignKeyStruct.IsPrimaryKey = false
					foreignKeyStruct.TagSettings["IS_JOINTABLE_FOREIGNKEY"] = "true"
					delete(foreignKeyStruct.TagSettings, "AUTO_INCREMENT")
					sqlTypes = append(sqlTypes, scope.Quote(relationship.ForeignDBNames[idx])+" "+scope.Dialect().DataTypeOf(foreignKeyStruct))
					primaryKeys = append(primaryKeys, scope.Quote(relationship.ForeignDBNames[idx]))
				}
			}

			for idx, fieldName := range relationship.AssociationForeignFieldNames {
				if field, ok := toScope.FieldByName(fieldName); ok {
					foreignKeyStruct := field.clone()
					foreignKeyStruct.IsPrimaryKey = false
					foreignKeyStruct.TagSettings["IS_JOINTABLE_FOREIGNKEY"] = "true"
					delete(foreignKeyStruct.TagSettings, "AUTO_INCREMENT")
					sqlTypes = append(sqlTypes, scope.Quote(relationship.AssociationForeignDBNames[idx])+" "+scope.Dialect().DataTypeOf(foreignKeyStruct))
					primaryKeys = append(primaryKeys, scope.Quote(relationship.AssociationForeignDBNames[idx]))
				}
			}

			scope.Err(scope.NewDB().Exec(fmt.Sprintf("CREATE TABLE %v (%v, PRIMARY KEY (%v)) %s", scope.Quote(joinTable), strings.Join(sqlTypes, ","), strings.Join(primaryKeys, ","), scope.getTableOptions())).Error)
		}
		scope.NewDB().Table(joinTable).AutoMigrate(joinTableHandler)
	}
}

func (scope *Scope) createTable() *Scope {
	var tags []string
	var primaryKeys []string
	var primaryKeyInColumnType = false
	for _, field := range scope.GetModelStruct().StructFields {
		if field.IsNormal {
			sqlTag := scope.Dialect().DataTypeOf(field)

			// Check if the primary key constraint was specified as
			// part of the column type. If so, we can only support
			// one column as the primary key.
			if strings.Contains(strings.ToLower(sqlTag), "primary key") {
				primaryKeyInColumnType = true
			}

			tags = append(tags, scope.Quote(field.DBName)+" "+sqlTag)
		}

		if field.IsPrimaryKey {
			primaryKeys = append(primaryKeys, scope.Quote(field.DBName))
		}
		scope.createJoinTable(field)
	}

	var primaryKeyStr string
	if len(primaryKeys) > 0 && !primaryKeyInColumnType {
		primaryKeyStr = fmt.Sprintf(", PRIMARY KEY (%v)", strings.Join(primaryKeys, ","))
	}

	scope.Raw(fmt.Sprintf("CREATE TABLE %v (%v %v) %s", scope.QuotedTableName(), strings.Join(tags, ","), primaryKeyStr, scope.getTableOptions())).Exec()

	scope.autoIndex()
	return scope
}

func (scope *Scope) dropTable() *Scope {
	scope.Raw(fmt.Sprintf("DROP TABLE %v", scope.QuotedTableName())).Exec()
	return scope
}

func (scope *Scope) modifyColumn(column string, typ string) {
	scope.Raw(fmt.Sprintf("ALTER TABLE %v MODIFY %v %v", scope.QuotedTableName(), scope.Quote(column), typ)).Exec()
}

func (scope *Scope) dropColumn(column string) {
	scope.Raw(fmt.Sprintf("ALTER TABLE %v DROP COLUMN %v", scope.QuotedTableName(), scope.Quote(column))).Exec()
}

func (scope *Scope) addIndex(unique bool, indexName string, column ...string) {
	if scope.Dialect().HasIndex(scope.TableName(), indexName) {
		return
	}

	var columns []string
	for _, name := range column {
		columns = append(columns, scope.quoteIfPossible(name))
	}

	sqlCreate := "CREATE INDEX"
	if unique {
		sqlCreate = "CREATE UNIQUE INDEX"
	}

	scope.Raw(fmt.Sprintf("%s %v ON %v(%v) %v", sqlCreate, indexName, scope.QuotedTableName(), strings.Join(columns, ", "), scope.whereSQL())).Exec()
}

func (scope *Scope) addForeignKey(field string, dest string, onDelete string, onUpdate string) {
	keyName := scope.Dialect().BuildForeignKeyName(scope.TableName(), field, dest)

	if scope.Dialect().HasForeignKey(scope.TableName(), keyName) {
		return
	}
	var query = `ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s ON DELETE %s ON UPDATE %s;`
	scope.Raw(fmt.Sprintf(query, scope.QuotedTableName(), scope.quoteIfPossible(keyName), scope.quoteIfPossible(field), dest, onDelete, onUpdate)).Exec()
}

func (scope *Scope) removeIndex(indexName string) {
	scope.Dialect().RemoveIndex(scope.TableName(), indexName)
}

func (scope *Scope) autoMigrate() *Scope {
	tableName := scope.TableName()
	quotedTableName := scope.QuotedTableName()

	if !scope.Dialect().HasTable(tableName) {
		scope.createTable()
	} else {
		for _, field := range scope.GetModelStruct().StructFields {
			if !scope.Dialect().HasColumn(tableName, field.DBName) {
				if field.IsNormal {
					sqlTag := scope.Dialect().DataTypeOf(field)
					scope.Raw(fmt.Sprintf("ALTER TABLE %v ADD %v %v;", quotedTableName, scope.Quote(field.DBName), sqlTag)).Exec()
				}
			}
			scope.createJoinTable(field)
		}
		scope.autoIndex()
	}
	return scope
}

func (scope *Scope) autoIndex() *Scope {
	var indexes = map[string][]string{}
	var uniqueIndexes = map[string][]string{}

	for _, field := range scope.GetStructFields() {
		if name, ok := field.TagSettings["INDEX"]; ok {
			names := strings.Split(name, ",")

			for _, name := range names {
				if name == "INDEX" || name == "" {
					name = fmt.Sprintf("idx_%v_%v", scope.TableName(), field.DBName)
				}
				indexes[name] = append(indexes[name], field.DBName)
			}
		}

		if name, ok := field.TagSettings["UNIQUE_INDEX"]; ok {
			names := strings.Split(name, ",")

			for _, name := range names {
				if name == "UNIQUE_INDEX" || name == "" {
					name = fmt.Sprintf("uix_%v_%v", scope.TableName(), field.DBName)
				}
				uniqueIndexes[name] = append(uniqueIndexes[name], field.DBName)
			}
		}
	}

	for name, columns := range indexes {
		scope.NewDB().Model(scope.Value).AddIndex(name, columns...)
	}

	for name, columns := range uniqueIndexes {
		scope.NewDB().Model(scope.Value).AddUniqueIndex(name, columns...)
	}

	return scope
}

func (scope *Scope) getColumnAsArray(columns []string, values ...interface{}) (results [][]interface{}) {
	for _, value := range values {
		indirectValue := reflect.ValueOf(value)
		for indirectValue.Kind() == reflect.Ptr {
			indirectValue = indirectValue.Elem()
		}

		switch indirectValue.Kind() {
		case reflect.Slice:
			for i := 0; i < indirectValue.Len(); i++ {
				var result []interface{}
				var object = indirect(indirectValue.Index(i))
				for _, column := range columns {
					result = append(result, object.FieldByName(column).Interface())
				}
				results = append(results, result)
			}
		case reflect.Struct:
			var result []interface{}
			for _, column := range columns {
				result = append(result, indirectValue.FieldByName(column).Interface())
			}
			results = append(results, result)
		}
	}
	return
}

func (scope *Scope) getColumnAsScope(column string) *Scope {
	indirectScopeValue := scope.IndirectValue()

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
				result := indirect(indirect(indirectScopeValue.Index(i)).FieldByName(column))

				if result.Kind() == reflect.Slice {
					for j := 0; j < result.Len(); j++ {
						if elem := result.Index(j); elem.CanAddr() && resultsMap[elem.Addr()] != true {
							resultsMap[elem.Addr()] = true
							results = reflect.Append(results, elem.Addr())
						}
					}
				} else if result.CanAddr() && resultsMap[result.Addr()] != true {
					resultsMap[result.Addr()] = true
					results = reflect.Append(results, result.Addr())
				}
			}
			return scope.New(results.Interface())
		}
	case reflect.Struct:
		if field := indirectScopeValue.FieldByName(column); field.CanAddr() {
			return scope.New(field.Addr().Interface())
		}
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// moved here from model_struct.go
////////////////////////////////////////////////////////////////////////////////
// GetModelStruct get value's model struct, relationships based on struct and tag definition
func (scope *Scope) GetModelStruct() *ModelStruct {
	var modelStruct ModelStruct
	// Scope value can't be nil
	if scope.Value == nil {
		return &modelStruct
	}

	reflectType := reflect.ValueOf(scope.Value).Type()
	for reflectType.Kind() == reflect.Slice || reflectType.Kind() == reflect.Ptr {
		reflectType = reflectType.Elem()
	}

	// Scope value need to be a struct
	if reflectType.Kind() != reflect.Struct {
		return &modelStruct
	}

	// Get Cached model struct
	if value := modelStructsMap.Get(reflectType); value != nil {
		return value
	}

	modelStruct.ModelType = reflectType

	// Set default table name
	if tabler, ok := reflect.New(reflectType).Interface().(tabler); ok {
		modelStruct.defaultTableName = tabler.TableName()
	} else {
		tableName := ToDBName(reflectType.Name())
		if scope.db == nil || !scope.db.parent.singularTable {
			tableName = inflection.Plural(tableName)
		}
		modelStruct.defaultTableName = tableName
	}

	// Get all fields
	for i := 0; i < reflectType.NumField(); i++ {
		if fieldStruct := reflectType.Field(i); ast.IsExported(fieldStruct.Name) {
			field := &StructField{
				Struct:      fieldStruct,
				Name:        fieldStruct.Name,
				Names:       []string{fieldStruct.Name},
				Tag:         fieldStruct.Tag,
				TagSettings: parseTagSetting(fieldStruct.Tag),
			}

			// is ignored field
			if _, ok := field.TagSettings["-"]; ok {
				field.IsIgnored = true
			} else {
				if _, ok := field.TagSettings["PRIMARY_KEY"]; ok {
					field.IsPrimaryKey = true
					modelStruct.PrimaryFields = append(modelStruct.PrimaryFields, field)
				}

				if _, ok := field.TagSettings["DEFAULT"]; ok {
					field.HasDefaultValue = true
				}

				if _, ok := field.TagSettings["AUTO_INCREMENT"]; ok && !field.IsPrimaryKey {
					field.HasDefaultValue = true
				}

				indirectType := fieldStruct.Type
				for indirectType.Kind() == reflect.Ptr {
					indirectType = indirectType.Elem()
				}

				fieldValue := reflect.New(indirectType).Interface()
				if _, isScanner := fieldValue.(sql.Scanner); isScanner {
					// is scanner
					field.IsScanner, field.IsNormal = true, true
					if indirectType.Kind() == reflect.Struct {
						for i := 0; i < indirectType.NumField(); i++ {
							for key, value := range parseTagSetting(indirectType.Field(i).Tag) {
								field.TagSettings[key] = value
							}
						}
					}
				} else if _, isTime := fieldValue.(*time.Time); isTime {
					// is time
					field.IsNormal = true
				} else if _, ok := field.TagSettings["EMBEDDED"]; ok || fieldStruct.Anonymous {
					// is embedded struct
					for _, subField := range scope.New(fieldValue).GetStructFields() {
						subField = subField.clone()
						subField.Names = append([]string{fieldStruct.Name}, subField.Names...)
						if prefix, ok := field.TagSettings["EMBEDDED_PREFIX"]; ok {
							subField.DBName = prefix + subField.DBName
						}
						if subField.IsPrimaryKey {
							modelStruct.PrimaryFields = append(modelStruct.PrimaryFields, subField)
						}
						modelStruct.StructFields = append(modelStruct.StructFields, subField)
					}
					continue
				} else {
					// build relationships
					switch indirectType.Kind() {
					case reflect.Slice:
						defer func(field *StructField) {
							var (
								relationship           = &Relationship{}
								toScope                = scope.New(reflect.New(field.Struct.Type).Interface())
								foreignKeys            []string
								associationForeignKeys []string
								elemType               = field.Struct.Type
							)

							if foreignKey := field.TagSettings["FOREIGNKEY"]; foreignKey != "" {
								foreignKeys = strings.Split(field.TagSettings["FOREIGNKEY"], ",")
							}

							if foreignKey := field.TagSettings["ASSOCIATIONFOREIGNKEY"]; foreignKey != "" {
								associationForeignKeys = strings.Split(field.TagSettings["ASSOCIATIONFOREIGNKEY"], ",")
							}

							for elemType.Kind() == reflect.Slice || elemType.Kind() == reflect.Ptr {
								elemType = elemType.Elem()
							}

							if elemType.Kind() == reflect.Struct {
								if many2many := field.TagSettings["MANY2MANY"]; many2many != "" {
									relationship.Kind = "many_to_many"

									// if no foreign keys defined with tag
									if len(foreignKeys) == 0 {
										for _, field := range modelStruct.PrimaryFields {
											foreignKeys = append(foreignKeys, field.DBName)
										}
									}

									for _, foreignKey := range foreignKeys {
										if foreignField := getForeignField(foreignKey, modelStruct.StructFields); foreignField != nil {
											// source foreign keys (db names)
											relationship.ForeignFieldNames = append(relationship.ForeignFieldNames, foreignField.DBName)
											// join table foreign keys for source
											joinTableDBName := ToDBName(reflectType.Name()) + "_" + foreignField.DBName
											relationship.ForeignDBNames = append(relationship.ForeignDBNames, joinTableDBName)
										}
									}

									// if no association foreign keys defined with tag
									if len(associationForeignKeys) == 0 {
										for _, field := range toScope.PrimaryFields() {
											associationForeignKeys = append(associationForeignKeys, field.DBName)
										}
									}

									for _, name := range associationForeignKeys {
										if field, ok := toScope.FieldByName(name); ok {
											// association foreign keys (db names)
											relationship.AssociationForeignFieldNames = append(relationship.AssociationForeignFieldNames, field.DBName)
											// join table foreign keys for association
											joinTableDBName := ToDBName(elemType.Name()) + "_" + field.DBName
											relationship.AssociationForeignDBNames = append(relationship.AssociationForeignDBNames, joinTableDBName)
										}
									}

									joinTableHandler := JoinTableHandler{}
									joinTableHandler.Setup(relationship, many2many, reflectType, elemType)
									relationship.JoinTableHandler = &joinTableHandler
									field.Relationship = relationship
								} else {
									// User has many comments, associationType is User, comment use UserID as foreign key
									var associationType = reflectType.Name()
									var toFields = toScope.GetStructFields()
									relationship.Kind = "has_many"

									if polymorphic := field.TagSettings["POLYMORPHIC"]; polymorphic != "" {
										// Dog has many toys, tag polymorphic is Owner, then associationType is Owner
										// Toy use OwnerID, OwnerType ('dogs') as foreign key
										if polymorphicType := getForeignField(polymorphic+"Type", toFields); polymorphicType != nil {
											associationType = polymorphic
											relationship.PolymorphicType = polymorphicType.Name
											relationship.PolymorphicDBName = polymorphicType.DBName
											// if Dog has multiple set of toys set name of the set (instead of default 'dogs')
											if value, ok := field.TagSettings["POLYMORPHIC_VALUE"]; ok {
												relationship.PolymorphicValue = value
											} else {
												relationship.PolymorphicValue = scope.TableName()
											}
											polymorphicType.IsForeignKey = true
										}
									}

									// if no foreign keys defined with tag
									if len(foreignKeys) == 0 {
										// if no association foreign keys defined with tag
										if len(associationForeignKeys) == 0 {
											for _, field := range modelStruct.PrimaryFields {
												foreignKeys = append(foreignKeys, associationType+field.Name)
												associationForeignKeys = append(associationForeignKeys, field.Name)
											}
										} else {
											// generate foreign keys from defined association foreign keys
											for _, scopeFieldName := range associationForeignKeys {
												if foreignField := getForeignField(scopeFieldName, modelStruct.StructFields); foreignField != nil {
													foreignKeys = append(foreignKeys, associationType+foreignField.Name)
													associationForeignKeys = append(associationForeignKeys, foreignField.Name)
												}
											}
										}
									} else {
										// generate association foreign keys from foreign keys
										if len(associationForeignKeys) == 0 {
											for _, foreignKey := range foreignKeys {
												if strings.HasPrefix(foreignKey, associationType) {
													associationForeignKey := strings.TrimPrefix(foreignKey, associationType)
													if foreignField := getForeignField(associationForeignKey, modelStruct.StructFields); foreignField != nil {
														associationForeignKeys = append(associationForeignKeys, associationForeignKey)
													}
												}
											}
											if len(associationForeignKeys) == 0 && len(foreignKeys) == 1 {
												associationForeignKeys = []string{scope.PrimaryKey()}
											}
										} else if len(foreignKeys) != len(associationForeignKeys) {
											scope.Err(errors.New("invalid foreign keys, should have same length"))
											return
										}
									}

									for idx, foreignKey := range foreignKeys {
										if foreignField := getForeignField(foreignKey, toFields); foreignField != nil {
											if associationField := getForeignField(associationForeignKeys[idx], modelStruct.StructFields); associationField != nil {
												// source foreign keys
												foreignField.IsForeignKey = true
												relationship.AssociationForeignFieldNames = append(relationship.AssociationForeignFieldNames, associationField.Name)
												relationship.AssociationForeignDBNames = append(relationship.AssociationForeignDBNames, associationField.DBName)

												// association foreign keys
												relationship.ForeignFieldNames = append(relationship.ForeignFieldNames, foreignField.Name)
												relationship.ForeignDBNames = append(relationship.ForeignDBNames, foreignField.DBName)
											}
										}
									}

									if len(relationship.ForeignFieldNames) != 0 {
										field.Relationship = relationship
									}
								}
							} else {
								field.IsNormal = true
							}
						}(field)
					case reflect.Struct:
						defer func(field *StructField) {
							var (
								// user has one profile, associationType is User, profile use UserID as foreign key
								// user belongs to profile, associationType is Profile, user use ProfileID as foreign key
								associationType           = reflectType.Name()
								relationship              = &Relationship{}
								toScope                   = scope.New(reflect.New(field.Struct.Type).Interface())
								toFields                  = toScope.GetStructFields()
								tagForeignKeys            []string
								tagAssociationForeignKeys []string
							)

							if foreignKey := field.TagSettings["FOREIGNKEY"]; foreignKey != "" {
								tagForeignKeys = strings.Split(field.TagSettings["FOREIGNKEY"], ",")
							}

							if foreignKey := field.TagSettings["ASSOCIATIONFOREIGNKEY"]; foreignKey != "" {
								tagAssociationForeignKeys = strings.Split(field.TagSettings["ASSOCIATIONFOREIGNKEY"], ",")
							}

							if polymorphic := field.TagSettings["POLYMORPHIC"]; polymorphic != "" {
								// Cat has one toy, tag polymorphic is Owner, then associationType is Owner
								// Toy use OwnerID, OwnerType ('cats') as foreign key
								if polymorphicType := getForeignField(polymorphic+"Type", toFields); polymorphicType != nil {
									associationType = polymorphic
									relationship.PolymorphicType = polymorphicType.Name
									relationship.PolymorphicDBName = polymorphicType.DBName
									// if Cat has several different types of toys set name for each (instead of default 'cats')
									if value, ok := field.TagSettings["POLYMORPHIC_VALUE"]; ok {
										relationship.PolymorphicValue = value
									} else {
										relationship.PolymorphicValue = scope.TableName()
									}
									polymorphicType.IsForeignKey = true
								}
							}

							// Has One
							{
								var foreignKeys = tagForeignKeys
								var associationForeignKeys = tagAssociationForeignKeys
								// if no foreign keys defined with tag
								if len(foreignKeys) == 0 {
									// if no association foreign keys defined with tag
									if len(associationForeignKeys) == 0 {
										for _, primaryField := range modelStruct.PrimaryFields {
											foreignKeys = append(foreignKeys, associationType+primaryField.Name)
											associationForeignKeys = append(associationForeignKeys, primaryField.Name)
										}
									} else {
										// generate foreign keys form association foreign keys
										for _, associationForeignKey := range tagAssociationForeignKeys {
											if foreignField := getForeignField(associationForeignKey, modelStruct.StructFields); foreignField != nil {
												foreignKeys = append(foreignKeys, associationType+foreignField.Name)
												associationForeignKeys = append(associationForeignKeys, foreignField.Name)
											}
										}
									}
								} else {
									// generate association foreign keys from foreign keys
									if len(associationForeignKeys) == 0 {
										for _, foreignKey := range foreignKeys {
											if strings.HasPrefix(foreignKey, associationType) {
												associationForeignKey := strings.TrimPrefix(foreignKey, associationType)
												if foreignField := getForeignField(associationForeignKey, modelStruct.StructFields); foreignField != nil {
													associationForeignKeys = append(associationForeignKeys, associationForeignKey)
												}
											}
										}
										if len(associationForeignKeys) == 0 && len(foreignKeys) == 1 {
											associationForeignKeys = []string{scope.PrimaryKey()}
										}
									} else if len(foreignKeys) != len(associationForeignKeys) {
										scope.Err(errors.New("invalid foreign keys, should have same length"))
										return
									}
								}

								for idx, foreignKey := range foreignKeys {
									if foreignField := getForeignField(foreignKey, toFields); foreignField != nil {
										if scopeField := getForeignField(associationForeignKeys[idx], modelStruct.StructFields); scopeField != nil {
											foreignField.IsForeignKey = true
											// source foreign keys
											relationship.AssociationForeignFieldNames = append(relationship.AssociationForeignFieldNames, scopeField.Name)
											relationship.AssociationForeignDBNames = append(relationship.AssociationForeignDBNames, scopeField.DBName)

											// association foreign keys
											relationship.ForeignFieldNames = append(relationship.ForeignFieldNames, foreignField.Name)
											relationship.ForeignDBNames = append(relationship.ForeignDBNames, foreignField.DBName)
										}
									}
								}
							}

							if len(relationship.ForeignFieldNames) != 0 {
								relationship.Kind = "has_one"
								field.Relationship = relationship
							} else {
								var foreignKeys = tagForeignKeys
								var associationForeignKeys = tagAssociationForeignKeys

								if len(foreignKeys) == 0 {
									// generate foreign keys & association foreign keys
									if len(associationForeignKeys) == 0 {
										for _, primaryField := range toScope.PrimaryFields() {
											foreignKeys = append(foreignKeys, field.Name+primaryField.Name)
											associationForeignKeys = append(associationForeignKeys, primaryField.Name)
										}
									} else {
										// generate foreign keys with association foreign keys
										for _, associationForeignKey := range associationForeignKeys {
											if foreignField := getForeignField(associationForeignKey, toFields); foreignField != nil {
												foreignKeys = append(foreignKeys, field.Name+foreignField.Name)
												associationForeignKeys = append(associationForeignKeys, foreignField.Name)
											}
										}
									}
								} else {
									// generate foreign keys & association foreign keys
									if len(associationForeignKeys) == 0 {
										for _, foreignKey := range foreignKeys {
											if strings.HasPrefix(foreignKey, field.Name) {
												associationForeignKey := strings.TrimPrefix(foreignKey, field.Name)
												if foreignField := getForeignField(associationForeignKey, toFields); foreignField != nil {
													associationForeignKeys = append(associationForeignKeys, associationForeignKey)
												}
											}
										}
										if len(associationForeignKeys) == 0 && len(foreignKeys) == 1 {
											associationForeignKeys = []string{toScope.PrimaryKey()}
										}
									} else if len(foreignKeys) != len(associationForeignKeys) {
										scope.Err(errors.New("invalid foreign keys, should have same length"))
										return
									}
								}

								for idx, foreignKey := range foreignKeys {
									if foreignField := getForeignField(foreignKey, modelStruct.StructFields); foreignField != nil {
										if associationField := getForeignField(associationForeignKeys[idx], toFields); associationField != nil {
											foreignField.IsForeignKey = true

											// association foreign keys
											relationship.AssociationForeignFieldNames = append(relationship.AssociationForeignFieldNames, associationField.Name)
											relationship.AssociationForeignDBNames = append(relationship.AssociationForeignDBNames, associationField.DBName)

											// source foreign keys
											relationship.ForeignFieldNames = append(relationship.ForeignFieldNames, foreignField.Name)
											relationship.ForeignDBNames = append(relationship.ForeignDBNames, foreignField.DBName)
										}
									}
								}

								if len(relationship.ForeignFieldNames) != 0 {
									relationship.Kind = "belongs_to"
									field.Relationship = relationship
								}
							}
						}(field)
					default:
						field.IsNormal = true
					}
				}
			}

			// Even it is ignored, also possible to decode db value into the field
			if value, ok := field.TagSettings["COLUMN"]; ok {
				field.DBName = value
			} else {
				field.DBName = ToDBName(fieldStruct.Name)
			}

			modelStruct.StructFields = append(modelStruct.StructFields, field)
		}
	}

	if len(modelStruct.PrimaryFields) == 0 {
		if field := getForeignField("id", modelStruct.StructFields); field != nil {
			field.IsPrimaryKey = true
			modelStruct.PrimaryFields = append(modelStruct.PrimaryFields, field)
		}
	}

	modelStructsMap.Set(reflectType, &modelStruct)

	return &modelStruct
}

// GetStructFields get model's field structs
func (scope *Scope) GetStructFields() (fields []*StructField) {
	return scope.GetModelStruct().StructFields
}

////////////////////////////////////////////////////////////////////////////////
// moved here from callback_query_preload.go
////////////////////////////////////////////////////////////////////////////////
func (scope *Scope) generatePreloadDBWithConditions(conditions []interface{}) (*DB, []interface{}) {
	var (
		preloadDB         = scope.NewDB()
		preloadConditions []interface{}
	)

	for _, condition := range conditions {
		if scopes, ok := condition.(func(*DB) *DB); ok {
			preloadDB = scopes(preloadDB)
		} else {
			preloadConditions = append(preloadConditions, condition)
		}
	}

	return preloadDB, preloadConditions
}

// handleHasOnePreload used to preload has one associations
func (scope *Scope) handleHasOnePreload(field *Field, conditions []interface{}) {
	relation := field.Relationship

	// get relations's primary keys
	primaryKeys := scope.getColumnAsArray(relation.AssociationForeignFieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}

	// preload conditions
	preloadDB, preloadConditions := scope.generatePreloadDBWithConditions(conditions)

	// find relations
	query := fmt.Sprintf("%v IN (%v)", toQueryCondition(scope, relation.ForeignDBNames), toQueryMarks(primaryKeys))
	values := toQueryValues(primaryKeys)
	if relation.PolymorphicType != "" {
		query += fmt.Sprintf(" AND %v = ?", scope.Quote(relation.PolymorphicDBName))
		values = append(values, relation.PolymorphicValue)
	}

	results := makeSlice(field.Struct.Type)
	scope.Err(preloadDB.Where(query, values...).Find(results, preloadConditions...).Error)

	// assign find results
	var (
		resultsValue       = indirect(reflect.ValueOf(results))
		indirectScopeValue = scope.IndirectValue()
	)

	if indirectScopeValue.Kind() == reflect.Slice {
		for j := 0; j < indirectScopeValue.Len(); j++ {
			for i := 0; i < resultsValue.Len(); i++ {
				result := resultsValue.Index(i)
				foreignValues := getValueFromFields(result, relation.ForeignFieldNames)
				if indirectValue := indirect(indirectScopeValue.Index(j)); equalAsString(getValueFromFields(indirectValue, relation.AssociationForeignFieldNames), foreignValues) {
					indirectValue.FieldByName(field.Name).Set(result)
					break
				}
			}
		}
	} else {
		for i := 0; i < resultsValue.Len(); i++ {
			result := resultsValue.Index(i)
			scope.Err(field.Set(result))
		}
	}
}

// handleHasManyPreload used to preload has many associations
func (scope *Scope) handleHasManyPreload(field *Field, conditions []interface{}) {
	relation := field.Relationship

	// get relations's primary keys
	primaryKeys := scope.getColumnAsArray(relation.AssociationForeignFieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}

	// preload conditions
	preloadDB, preloadConditions := scope.generatePreloadDBWithConditions(conditions)

	// find relations
	query := fmt.Sprintf("%v IN (%v)", toQueryCondition(scope, relation.ForeignDBNames), toQueryMarks(primaryKeys))
	values := toQueryValues(primaryKeys)
	if relation.PolymorphicType != "" {
		query += fmt.Sprintf(" AND %v = ?", scope.Quote(relation.PolymorphicDBName))
		values = append(values, relation.PolymorphicValue)
	}

	results := makeSlice(field.Struct.Type)
	scope.Err(preloadDB.Where(query, values...).Find(results, preloadConditions...).Error)

	// assign find results
	var (
		resultsValue       = indirect(reflect.ValueOf(results))
		indirectScopeValue = scope.IndirectValue()
	)

	if indirectScopeValue.Kind() == reflect.Slice {
		preloadMap := make(map[string][]reflect.Value)
		for i := 0; i < resultsValue.Len(); i++ {
			result := resultsValue.Index(i)
			foreignValues := getValueFromFields(result, relation.ForeignFieldNames)
			preloadMap[toString(foreignValues)] = append(preloadMap[toString(foreignValues)], result)
		}

		for j := 0; j < indirectScopeValue.Len(); j++ {
			object := indirect(indirectScopeValue.Index(j))
			objectRealValue := getValueFromFields(object, relation.AssociationForeignFieldNames)
			f := object.FieldByName(field.Name)
			if results, ok := preloadMap[toString(objectRealValue)]; ok {
				f.Set(reflect.Append(f, results...))
			} else {
				f.Set(reflect.MakeSlice(f.Type(), 0, 0))
			}
		}
	} else {
		scope.Err(field.Set(resultsValue))
	}
}

// handleBelongsToPreload used to preload belongs to associations
func (scope *Scope) handleBelongsToPreload(field *Field, conditions []interface{}) {
	relation := field.Relationship

	// preload conditions
	preloadDB, preloadConditions := scope.generatePreloadDBWithConditions(conditions)

	// get relations's primary keys
	primaryKeys := scope.getColumnAsArray(relation.ForeignFieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}

	// find relations
	results := makeSlice(field.Struct.Type)
	scope.Err(preloadDB.Where(fmt.Sprintf("%v IN (%v)", toQueryCondition(scope, relation.AssociationForeignDBNames), toQueryMarks(primaryKeys)), toQueryValues(primaryKeys)...).Find(results, preloadConditions...).Error)

	// assign find results
	var (
		resultsValue       = indirect(reflect.ValueOf(results))
		indirectScopeValue = scope.IndirectValue()
	)

	for i := 0; i < resultsValue.Len(); i++ {
		result := resultsValue.Index(i)
		if indirectScopeValue.Kind() == reflect.Slice {
			value := getValueFromFields(result, relation.AssociationForeignFieldNames)
			for j := 0; j < indirectScopeValue.Len(); j++ {
				object := indirect(indirectScopeValue.Index(j))
				if equalAsString(getValueFromFields(object, relation.ForeignFieldNames), value) {
					object.FieldByName(field.Name).Set(result)
				}
			}
		} else {
			scope.Err(field.Set(result))
		}
	}
}

// handleManyToManyPreload used to preload many to many associations
func (scope *Scope) handleManyToManyPreload(field *Field, conditions []interface{}) {
	var (
		relation         = field.Relationship
		joinTableHandler = relation.JoinTableHandler
		fieldType        = field.Struct.Type.Elem()
		foreignKeyValue  interface{}
		foreignKeyType   = reflect.ValueOf(&foreignKeyValue).Type()
		linkHash         = map[string][]reflect.Value{}
		isPtr            bool
	)

	if fieldType.Kind() == reflect.Ptr {
		isPtr = true
		fieldType = fieldType.Elem()
	}

	var sourceKeys = []string{}
	for _, key := range joinTableHandler.SourceForeignKeys() {
		sourceKeys = append(sourceKeys, key.DBName)
	}

	// preload conditions
	preloadDB, preloadConditions := scope.generatePreloadDBWithConditions(conditions)

	// generate query with join table
	newScope := scope.New(reflect.New(fieldType).Interface())
	preloadDB = preloadDB.Table(newScope.TableName()).Model(newScope.Value).Select("*")
	preloadDB = joinTableHandler.JoinWith(joinTableHandler, preloadDB, scope.Value)

	// preload inline conditions
	if len(preloadConditions) > 0 {
		preloadDB = preloadDB.Where(preloadConditions[0], preloadConditions[1:]...)
	}

	rows, err := preloadDB.Rows()

	if scope.Err(err) != nil {
		return
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	for rows.Next() {
		var (
			elem   = reflect.New(fieldType).Elem()
			fields = scope.New(elem.Addr().Interface()).Fields()
		)

		// register foreign keys in join tables
		var joinTableFields []*Field
		for _, sourceKey := range sourceKeys {
			joinTableFields = append(joinTableFields, &Field{StructField: &StructField{DBName: sourceKey, IsNormal: true}, Field: reflect.New(foreignKeyType).Elem()})
		}

		scope.scan(rows, columns, append(fields, joinTableFields...))

		var foreignKeys = make([]interface{}, len(sourceKeys))
		// generate hashed forkey keys in join table
		for idx, joinTableField := range joinTableFields {
			if !joinTableField.Field.IsNil() {
				foreignKeys[idx] = joinTableField.Field.Elem().Interface()
			}
		}
		hashedSourceKeys := toString(foreignKeys)

		if isPtr {
			linkHash[hashedSourceKeys] = append(linkHash[hashedSourceKeys], elem.Addr())
		} else {
			linkHash[hashedSourceKeys] = append(linkHash[hashedSourceKeys], elem)
		}
	}

	// assign find results
	var (
		indirectScopeValue = scope.IndirectValue()
		fieldsSourceMap    = map[string][]reflect.Value{}
		foreignFieldNames  = []string{}
	)

	for _, dbName := range relation.ForeignFieldNames {
		if field, ok := scope.FieldByName(dbName); ok {
			foreignFieldNames = append(foreignFieldNames, field.Name)
		}
	}

	if indirectScopeValue.Kind() == reflect.Slice {
		for j := 0; j < indirectScopeValue.Len(); j++ {
			object := indirect(indirectScopeValue.Index(j))
			key := toString(getValueFromFields(object, foreignFieldNames))
			fieldsSourceMap[key] = append(fieldsSourceMap[key], object.FieldByName(field.Name))
		}
	} else if indirectScopeValue.IsValid() {
		key := toString(getValueFromFields(indirectScopeValue, foreignFieldNames))
		fieldsSourceMap[key] = append(fieldsSourceMap[key], indirectScopeValue.FieldByName(field.Name))
	}
	for source, link := range linkHash {
		for i, field := range fieldsSourceMap[source] {
			//If not 0 this means Value is a pointer and we already added preloaded models to it
			if fieldsSourceMap[source][i].Len() != 0 {
				continue
			}
			field.Set(reflect.Append(fieldsSourceMap[source][i], link...))
		}

	}
}
