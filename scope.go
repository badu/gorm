package gorm

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// IndirectValue return scope's reflect value's indirect value
func (scope *Scope) IndirectValue() reflect.Value {
	reflectValue := reflect.ValueOf(scope.Value)
	for reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
}

// New create a new Scope without search information
func (scope *Scope) New(value interface{}) *Scope {
	return &Scope{con: scope.NewCon(), Search: &search{}, Value: value}
}

////////////////////////////////////////////////////////////////////////////////
// Scope DB
////////////////////////////////////////////////////////////////////////////////

// Con return scope's connection
func (scope *Scope) Con() *DBCon {
	return scope.con
}

// NewCon create a new Con without search information
func (scope *Scope) NewCon() *DBCon {
	if scope.con != nil {
		db := scope.con.clone(true, true)
		return db
	}
	return nil
}

// SQLDB return *sql.DB
func (scope *Scope) AsSQLDB() sqlInterf {
	return scope.con.sqli
}

// Dialect get dialect
func (scope *Scope) Dialect() Dialect {
	return scope.con.parent.dialect
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
		scope.con.AddError(err)
	}
	return err
}

// HasError check if there are any error
func (scope *Scope) HasError() bool {
	return scope.con.Error != nil
}

// Log print log message
func (scope *Scope) Log(v ...interface{}) {
	scope.con.log(v...)
}

// SkipLeft skip remaining callbacks
func (scope *Scope) SkipLeft() {
	scope.skipLeft = true
}

// Fields get value's fields from ModelStruct
func (scope *Scope) Fields() StructFields {
	if scope.fields == nil {
		//TODO : @Badu - find out why do we clone
		scope.fields = scope.GetModelStruct().cloneFieldsToScope(scope.IndirectValue())
	}
	return *scope.fields
}

//TODO : @Badu - after finding out why we clone, might be removed
// FieldByName find `gorm.StructField` with field name or db name
func (scope *Scope) FieldByName(name string) (*StructField, bool) {
	var (
		dbName           = NamesMap.ToDBName(name)
		mostMatchedField *StructField
	)

	for _, field := range scope.Fields() {
		if field.GetName() == name || field.DBName == name {
			return field, true
		}
		//TODO : @Badu - isn't that interesting ? see condition above
		if field.DBName == dbName {
			mostMatchedField = field
		}
	}
	return mostMatchedField, mostMatchedField != nil
}

//TODO : @Badu - after finding out why we clone, might be removed
// PrimaryFields return scope's primary fields
func (scope *Scope) PrimaryFields() StructFields {
	var fields StructFields
	for _, field := range scope.Fields() {
		if field.IsPrimaryKey() {
			fields.add(field)
		}
	}
	return fields
}

//TODO : @Badu - after finding out why we clone, might be removed
// PrimaryField return scope's main primary field, if defined more that one primary fields, will return the one having column name `id` or the first one
func (scope *Scope) PrimaryField() *StructField {
	primaryFieldsNo := scope.GetModelStruct().PrimaryFields.len()
	//TODO : @Badu - isn't that interesting ? We're looking to see if there are primary fields in ModelStruct
	if primaryFieldsNo > 0 {
		if primaryFieldsNo > 1 {
			//TODO : @Badu - isn't that interesting ? if ModelStruct has more than one, take "id"
			//TODO : @Badu - a boiler plate string. Get rid of it!
			if field, ok := scope.FieldByName("id"); ok {
				return field
			}
		}
		//TODO : @Badu - isn't that interesting ? otherwise, we will clone the StructFields of ModelStruct
		//and return the first one
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
	return field == nil || field.IsBlank()
}

// PrimaryKeyValue get the primary key's value
func (scope *Scope) PrimaryKeyValue() interface{} {
	if field := scope.PrimaryField(); field != nil && field.Value.IsValid() {
		return field.Value.Interface()
	}
	return 0
}

// SetColumn to set the column's value, column could be field or field's name/dbname
func (scope *Scope) SetColumn(column interface{}, value interface{}) error {
	var updateAttrs = map[string]interface{}{}
	if attrs, ok := scope.InstanceGet("gorm:update_attrs"); ok {
		updateAttrs = attrs.(map[string]interface{})
		defer scope.InstanceSet("gorm:update_attrs", updateAttrs)
	}

	if field, ok := column.(*StructField); ok {
		updateAttrs[field.DBName] = value
		return field.Set(value)
	} else if name, ok := column.(string); ok {
		var (
			dbName           = NamesMap.ToDBName(name)
			mostMatchedField *StructField
		)
		for _, field := range scope.Fields() {
			if field.DBName == value {
				updateAttrs[field.DBName] = value
				return field.Set(value)
			}
			if (field.DBName == dbName) || (field.GetName() == name && mostMatchedField == nil) {
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
		scope.selectAttrs = scope.Search.collectAttrs()
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
	switch tabler := scope.Value.(type) {
	case tabler:
		return tabler.TableName()
	case dbTabler:
		return tabler.TableName(scope.con)
	}
	return scope.GetModelStruct().TableName(scope.con.Model(scope.Value))
}

// QuotedTableName return quoted table name
func (scope *Scope) QuotedTableName() string {
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
	//Attention : if we don't build joinSql first, joins will fail (it's mixing up the where clauses of the joins)
	joinsSql := scope.joinsSQL()
	whereSql := scope.whereSQL()
	if scope.Search.raw {
		whereSql = strings.TrimSuffix(strings.TrimPrefix(whereSql, "WHERE ("), ")")
	}
	return joinsSql + whereSql + scope.groupSQL() +
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
		if result, err := scope.AsSQLDB().Exec(scope.SQL, scope.SQLVars...); scope.Err(err) == nil {
			if count, err := result.RowsAffected(); scope.Err(err) == nil {
				scope.con.RowsAffected = count
			}
		}
	}
	return scope
}

// Set set value by name
func (scope *Scope) Set(name string, value interface{}) *Scope {
	scope.con.InstantSet(name, value)
	return scope
}

// Get get setting by name
func (scope *Scope) Get(name string) (interface{}, bool) {
	return scope.con.Get(name)
}

// InstanceID get InstanceID for scope
func (scope *Scope) InstanceID() string {
	if scope.instanceID == "" {
		scope.instanceID = fmt.Sprintf("%v%v", &scope, &scope.con)
	}
	return scope.instanceID
}

// InstanceSet set instance setting for current operation,
// but not for operations in callbacks,
// like saving associations callback
func (scope *Scope) InstanceSet(name string, value interface{}) *Scope {
	return scope.Set(name+scope.InstanceID(), value)
}

// InstanceGet get instance setting from current operation
func (scope *Scope) InstanceGet(name string) (interface{}, bool) {
	return scope.Get(name + scope.InstanceID())
}

// Begin start a transaction
func (scope *Scope) Begin() *Scope {
	if db, ok := scope.AsSQLDB().(sqlDb); ok {
		//parent db implements Begin() -> call Begin()
		if tx, err := db.Begin(); err == nil {
			//TODO : @Badu - maybe the parent should do so, since it's owner of db.db
			//parent db.db implements Exec(), Prepare(), Query() and QueryRow()
			scope.con.sqli = interface{}(tx).(sqlInterf)
			scope.InstanceSet("gorm:started_transaction", true)
		}
	}
	return scope
}

// CommitOrRollback commit current transaction if no error happened, otherwise will rollback it
func (scope *Scope) CommitOrRollback() *Scope {
	if _, ok := scope.InstanceGet("gorm:started_transaction"); ok {
		if db, ok := scope.con.sqli.(sqlTx); ok {
			if scope.HasError() {
				//orm.db implements Commit() and Rollback() -> call Rollback()
				db.Rollback()
			} else {
				//orm.db implements Commit() and Rollback() -> call Commit()
				scope.Err(db.Commit())
			}
			scope.con.sqli = scope.con.parent.sqli
		}
	}
	return scope
}

////////////////////////////////////////////////////////////////////////////////
// moved here from model_struct.go
////////////////////////////////////////////////////////////////////////////////
// GetModelStruct get value's model struct, relationships based on struct and tag definition
func (scope *Scope) GetModelStruct() *ModelStruct {
	var modelStruct ModelStruct
	// Scope value can't be nil
	//TODO : @Badu - why can't be null and why we are not returning an error?
	if scope.Value == nil {
		return &modelStruct
	}

	reflectType := reflect.ValueOf(scope.Value).Type()
	for reflectType.Kind() == reflect.Slice || reflectType.Kind() == reflect.Ptr {
		//dereference
		reflectType = reflectType.Elem()
	}

	if reflectType.Kind() != reflect.Struct {
		//TODO : @Badu - why we are not returning an error?
		// Scope value need to be a struct
		return &modelStruct
	}

	// Get Cached model struct
	if value := modelStructsMap.Get(reflectType); value != nil {
		return value
	}

	modelStruct.Create(reflectType, scope)

	//set cached ModelStruc
	modelStructsMap.Set(reflectType, &modelStruct)
	// ATTN : first we add it to cache map, otherwise will infinite cycle
	// build relationships
	modelStruct.processRelations(scope)

	return &modelStruct
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
			//TODO : @Badu - see if we can use ScopedFunc
		case func(*Scope):
			method(scope)
		case func(*DBCon):
			//TODO : @Badu - see if we can use ScopedFunc and add DBConFunc - type DBConFunc func(*DBCon)
			newDB := scope.NewCon()
			method(newDB)
			scope.Err(newDB.Error)
		case func() error:
			scope.Err(method())
		case func(*Scope) error:
			scope.Err(method(scope))
		case func(*DBCon) error:
			newDB := scope.NewCon()
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

func (scope *Scope) scan(rows *sql.Rows, columns []string, fields StructFields) {
	var (
		ignored            interface{}
		values             = make([]interface{}, len(columns))
		selectFields       StructFields
		selectedColumnsMap = map[string]int{}
		resetFields        = map[int]*StructField{}
	)

	for index, column := range columns {
		values[index] = &ignored

		selectFields = fields
		if idx, ok := selectedColumnsMap[column]; ok {
			selectFields = selectFields[idx+1:]
		}

		for fieldIndex, field := range selectFields {
			if field.DBName == column {
				if field.Value.Kind() == reflect.Ptr {
					values[index] = field.Value.Addr().Interface()
				} else {
					reflectValue := reflect.New(reflect.PtrTo(field.Struct.Type))
					reflectValue.Elem().Set(field.Value.Addr())
					values[index] = reflectValue.Interface()
					resetFields[index] = field
				}

				selectedColumnsMap[column] = fieldIndex
				//TODO :@Badu - why if it's normal we break last ? shouldn't be first?
				if field.IsNormal() {
					break
				}
			}
		}
	}

	scope.Err(rows.Scan(values...))

	for index, field := range resetFields {
		if v := reflect.ValueOf(values[index]).Elem().Elem(); v.IsValid() {
			field.Value.Set(v)
		}
	}
}

func (scope *Scope) primaryCondition(value interface{}) string {
	return fmt.Sprintf("(%v.%v = %v)", scope.QuotedTableName(), scope.Quote(scope.PrimaryKey()), value)
}

func (scope *Scope) buildWhereCondition(clause map[string]interface{}) string {
	var str string
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
			if !field.IsIgnored() && !field.IsBlank() {
				sqls = append(sqls, fmt.Sprintf("(%v.%v = %v)", newScope.QuotedTableName(), scope.Quote(field.DBName), scope.AddToVars(field.Value.Interface())))
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
	return str
}

func (scope *Scope) buildNotCondition(clause map[string]interface{}) string {
	var str string
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
			if !field.IsBlank() {
				sqls = append(sqls, fmt.Sprintf("(%v.%v <> %v)", newScope.QuotedTableName(), scope.Quote(field.DBName), scope.AddToVars(field.Value.Interface())))
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
	return str
}

func (scope *Scope) buildSelectQuery(clause map[string]interface{}) string {
	var str string
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
	return str
}

func (scope *Scope) whereSQL() string {
	var (
		str                                            string
		quotedTableName                                = scope.QuotedTableName()
		primaryConditions, andConditions, orConditions []string
	)

	if !scope.Search.Unscoped && scope.GetModelStruct().HasColumn("deleted_at") {
		aStr := fmt.Sprintf("%v.deleted_at IS NULL", quotedTableName)
		primaryConditions = append(primaryConditions, aStr)
	}

	if !scope.PrimaryKeyZero() {
		for _, field := range scope.PrimaryFields() {
			aStr := fmt.Sprintf("%v.%v = %v", quotedTableName, scope.Quote(field.DBName), scope.AddToVars(field.Value.Interface()))
			primaryConditions = append(primaryConditions, aStr)
		}
	}

	for _, clause := range scope.Search.whereConditions {
		if aStr := scope.buildWhereCondition(clause); aStr != "" {
			andConditions = append(andConditions, aStr)
		}
	}

	for _, clause := range scope.Search.orConditions {
		if aStr := scope.buildWhereCondition(clause); aStr != "" {
			orConditions = append(orConditions, aStr)
		}
	}

	for _, clause := range scope.Search.notConditions {
		if aStr := scope.buildNotCondition(clause); aStr != "" {
			andConditions = append(andConditions, aStr)
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
		str = "WHERE " + strings.Join(primaryConditions, " AND ")
		if len(combinedSQL) > 0 {
			str = str + " AND (" + combinedSQL + ")"
		}
	} else if len(combinedSQL) > 0 {
		str = "WHERE " + combinedSQL
	}
	return str
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
			//TODO : @Badu - duplicated code - AddToVars
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
		if aStr := scope.buildWhereCondition(clause); aStr != "" {
			andConditions = append(andConditions, aStr)
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
		if aStr := scope.buildWhereCondition(clause); aStr != "" {
			joinConditions = append(joinConditions, strings.TrimSuffix(strings.TrimPrefix(aStr, "("), ")"))
		}
	}

	return strings.Join(joinConditions, " ") + " "
}

func (scope *Scope) prepareQuerySQL() {
	if scope.Search.raw {
		scope.Raw(scope.CombinedConditionSql())
	} else {
		scope.Raw(fmt.Sprintf("SELECT %v FROM %v %v", scope.selectSQL(), scope.QuotedTableName(), scope.CombinedConditionSql()))
	}
}

func (scope *Scope) inlineCondition(values ...interface{}) *Scope {
	if len(values) > 0 {
		scope.Search.Where(values[0], values[1:]...)
	}
	return scope
}

func (scope *Scope) callCallbacks(funcs ScopedFuncs) *Scope {
	for _, f := range funcs {
		//was (*f)(scope) - but IDE went balistic
		rf := *f
		rf(scope)
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
	return
}

func (scope *Scope) row() *sql.Row {
	defer scope.trace(NowFunc())
	scope.callCallbacks(scope.con.parent.callback.rowQueries)
	scope.prepareQuerySQL()
	return scope.AsSQLDB().QueryRow(scope.SQL, scope.SQLVars...)
}

func (scope *Scope) rows() (*sql.Rows, error) {
	defer scope.trace(NowFunc())
	scope.callCallbacks(scope.con.parent.callback.rowQueries)
	scope.prepareQuerySQL()
	return scope.AsSQLDB().Query(scope.SQL, scope.SQLVars...)
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
		scope.con.slog(scope.SQL, t, scope.SQLVars...)
	}
}

func (scope *Scope) changeableField(field *StructField) bool {
	if selectAttrs := scope.SelectAttrs(); len(selectAttrs) > 0 {
		for _, attr := range selectAttrs {
			if field.GetName() == attr || field.DBName == attr {
				return true
			}
		}
		return false
	}

	for _, attr := range scope.OmitAttrs() {
		if field.GetName() == attr || field.DBName == attr {
			return false
		}
	}

	return true
}

func (scope *Scope) shouldSaveAssociations() bool {
	if saveAssociations, ok := scope.Get("gorm:save_associations"); ok {
		if v, ok := saveAssociations.(bool); ok && !v {
			return false
		}
		if v, ok := saveAssociations.(string); ok && (v == "skip" || v == "false") {
			return false
		}
	}
	return !scope.HasError()
}

func (scope *Scope) related(value interface{}, foreignKeys ...string) *Scope {
	toScope := scope.con.NewScope(value)
	//TODO : @Badu - boilerplate string
	for _, foreignKey := range append(foreignKeys, toScope.typeName()+"Id", scope.typeName()+"Id") {
		fromField, _ := scope.FieldByName(foreignKey)
		toField, _ := toScope.FieldByName(foreignKey)

		if fromField != nil {
			if relationship := fromField.Relationship; relationship != nil {
				switch relationship.Kind {
				case MANY_TO_MANY:
					joinTableHandler := relationship.JoinTableHandler
					scope.Err(joinTableHandler.JoinWith(joinTableHandler, toScope.con, scope.Value).Find(value).Error)
				case BELONGS_TO:
					query := toScope.con
					for idx, foreignKey := range relationship.ForeignDBNames {
						if field, ok := scope.FieldByName(foreignKey); ok {
							query = query.Where(fmt.Sprintf("%v = ?", scope.Quote(relationship.AssociationForeignDBNames[idx])), field.Value.Interface())
						}
					}
					scope.Err(query.Find(value).Error)
				case HAS_MANY, HAS_ONE:
					query := toScope.con
					for idx, foreignKey := range relationship.ForeignDBNames {
						if field, ok := scope.FieldByName(relationship.AssociationForeignDBNames[idx]); ok {
							query = query.Where(fmt.Sprintf("%v = ?", scope.Quote(foreignKey)), field.Value.Interface())
						}
					}

					if relationship.PolymorphicType != "" {
						query = query.Where(fmt.Sprintf("%v = ?", scope.Quote(relationship.PolymorphicDBName)), relationship.PolymorphicValue)
					}
					scope.Err(query.Find(value).Error)
				}
			} else {
				aStr := fmt.Sprintf("%v = ?", scope.Quote(toScope.PrimaryKey()))
				scope.Err(toScope.con.Where(aStr, fromField.Value.Interface()).Find(value).Error)
			}
			return scope
		} else if toField != nil {
			aStr := fmt.Sprintf("%v = ?", scope.Quote(toField.DBName))
			scope.Err(toScope.con.Where(aStr, scope.PrimaryKeyValue()).Find(value).Error)
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
		joinTable := joinTableHandler.Table(scope.con)
		if !scope.Dialect().HasTable(joinTable) {
			toScope := &Scope{Value: reflect.New(field.Struct.Type).Interface()}

			var sqlTypes, primaryKeys []string
			for idx, fieldName := range relationship.ForeignFieldNames {
				if field, ok := scope.FieldByName(fieldName); ok {
					foreignKeyStruct := field.clone()
					foreignKeyStruct.unsetFlag(IS_PRIMARYKEY)
					//TODO : @Badu - document that you cannot use IS_JOINTABLE_FOREIGNKEY in conjunction with AUTO_INCREMENT
					foreignKeyStruct.SetSetting(IS_JOINTABLE_FOREIGNKEY, "true")
					foreignKeyStruct.UnsetSetting(AUTO_INCREMENT)
					sqlTypes = append(sqlTypes, scope.Quote(relationship.ForeignDBNames[idx])+" "+scope.Dialect().DataTypeOf(foreignKeyStruct))
					primaryKeys = append(primaryKeys, scope.Quote(relationship.ForeignDBNames[idx]))
				}
			}

			for idx, fieldName := range relationship.AssociationForeignFieldNames {
				if field, ok := toScope.FieldByName(fieldName); ok {
					foreignKeyStruct := field.clone()
					foreignKeyStruct.unsetFlag(IS_PRIMARYKEY)
					//TODO : @Badu - document that you cannot use IS_JOINTABLE_FOREIGNKEY in conjunction with AUTO_INCREMENT
					foreignKeyStruct.SetSetting(IS_JOINTABLE_FOREIGNKEY, "true")
					foreignKeyStruct.UnsetSetting(AUTO_INCREMENT)
					sqlTypes = append(sqlTypes, scope.Quote(relationship.AssociationForeignDBNames[idx])+" "+scope.Dialect().DataTypeOf(foreignKeyStruct))
					primaryKeys = append(primaryKeys, scope.Quote(relationship.AssociationForeignDBNames[idx]))
				}
			}

			scope.Err(scope.NewCon().Exec(fmt.Sprintf("CREATE TABLE %v (%v, PRIMARY KEY (%v)) %s", scope.Quote(joinTable), strings.Join(sqlTypes, ","), strings.Join(primaryKeys, ","), scope.getTableOptions())).Error)
		}
		scope.NewCon().Table(joinTable).AutoMigrate(joinTableHandler)
	}
}

func (scope *Scope) createTable() *Scope {
	var tags []string
	var primaryKeys []string
	var primaryKeyInColumnType = false
	for _, field := range scope.GetModelStruct().StructFields {
		if field.IsNormal() {
			sqlTag := scope.Dialect().DataTypeOf(field)

			// Check if the primary key constraint was specified as
			// part of the column type. If so, we can only support
			// one column as the primary key.
			//TODO : @Badu - boiler plate string
			if strings.Contains(strings.ToLower(sqlTag), "primary key") {
				primaryKeyInColumnType = true
			}

			tags = append(tags, scope.Quote(field.DBName)+" "+sqlTag)
		}

		if field.IsPrimaryKey() {
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
				if field.IsNormal() {
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

	for _, field := range scope.GetModelStruct().StructFields {
		if name := field.GetSetting(INDEX); name != "" {
			names := strings.Split(name, ",")

			for _, name := range names {
				if name == "INDEX" || name == "" {
					name = fmt.Sprintf("idx_%v_%v", scope.TableName(), field.DBName)
				}
				indexes[name] = append(indexes[name], field.DBName)
			}
		}

		if name := field.GetSetting(UNIQUE_INDEX); name != "" {
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
		scope.NewCon().Model(scope.Value).AddIndex(name, columns...)
	}

	for name, columns := range uniqueIndexes {
		scope.NewCon().Model(scope.Value).AddUniqueIndex(name, columns...)
	}

	return scope
}

func (scope *Scope) getColumnAsArray(columns StrSlice, values ...interface{}) [][]interface{} {
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
				reflectValue := indirectValue.Index(i)
				for reflectValue.Kind() == reflect.Ptr {
					reflectValue = reflectValue.Elem()
				}
				for _, column := range columns {
					result = append(result, reflectValue.FieldByName(column).Interface())
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
	return results
}

//returns the scope of a slice or struct column
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
			return scope.New(results.Interface())
		}
	case reflect.Struct:
		if field := indirectScopeValue.FieldByName(column); field.CanAddr() {
			return scope.New(field.Addr().Interface())
		}
	}
	return nil
}

func (scope *Scope) toQueryCondition(columns StrSlice) string {
	var newColumns []string
	for _, column := range columns {
		newColumns = append(newColumns, scope.Quote(column))
	}

	if len(columns) > 1 {
		return fmt.Sprintf("(%v)", strings.Join(newColumns, ","))
	}
	return strings.Join(newColumns, ",")
}

func (scope *Scope) saveFieldAsAssociation(field *StructField) (bool, *Relationship) {
	if scope.changeableField(field) && !field.IsBlank() && !field.IsIgnored() {
		if field.HasSetting(SAVE_ASSOCIATIONS) {
			set := field.GetSetting(SAVE_ASSOCIATIONS)
			if set == "false" || set == "skip" {
				return false, nil
			}
		}
		if relationship := field.Relationship; relationship != nil {
			return true, relationship
		}
	}
	return false, nil
}

////////////////////////////////////////////////////////////////////////////////
// moved here from callback_query_preload.go
////////////////////////////////////////////////////////////////////////////////
func (scope *Scope) generatePreloadDBWithConditions(conditions []interface{}) (*DBCon, []interface{}) {
	var (
		preloadDB         = scope.NewCon()
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

// handleHasOnePreload used to preload has one associations
func (scope *Scope) handleHasOnePreload(field *StructField, conditions []interface{}) {
	relation := field.Relationship

	// get relations's primary keys
	primaryKeys := scope.getColumnAsArray(relation.AssociationForeignFieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}

	// preload conditions
	preloadDB, preloadConditions := scope.generatePreloadDBWithConditions(conditions)

	// find relations
	query := fmt.Sprintf("%v IN (%v)", scope.toQueryCondition(relation.ForeignDBNames), toQueryMarks(primaryKeys))
	values := toQueryValues(primaryKeys)
	if relation.PolymorphicType != "" {
		query += fmt.Sprintf(" AND %v = ?", scope.Quote(relation.PolymorphicDBName))
		values = append(values, relation.PolymorphicValue)
	}

	results := field.makeSlice()
	scope.Err(preloadDB.Where(query, values...).Find(results, preloadConditions...).Error)

	// assign find results
	var (
		indirectScopeValue = scope.IndirectValue()
	)

	resultsValue := reflect.ValueOf(results)
	for resultsValue.Kind() == reflect.Ptr {
		resultsValue = resultsValue.Elem()
	}

	if indirectScopeValue.Kind() == reflect.Slice {
		for j := 0; j < indirectScopeValue.Len(); j++ {
			for i := 0; i < resultsValue.Len(); i++ {
				result := resultsValue.Index(i)
				foreignValues := getValueFromFields(result, relation.ForeignFieldNames)
				indirectValue := indirectScopeValue.Index(j)
				for indirectValue.Kind() == reflect.Ptr {
					indirectValue = indirectValue.Elem()
				}
				if equalAsString(getValueFromFields(indirectValue, relation.AssociationForeignFieldNames), foreignValues) {
					indirectValue.FieldByName(field.GetName()).Set(result)
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
func (scope *Scope) handleHasManyPreload(field *StructField, conditions []interface{}) {
	relation := field.Relationship

	// get relations's primary keys
	primaryKeys := scope.getColumnAsArray(relation.AssociationForeignFieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}

	// preload conditions
	preloadDB, preloadConditions := scope.generatePreloadDBWithConditions(conditions)

	// find relations
	query := fmt.Sprintf("%v IN (%v)", scope.toQueryCondition(relation.ForeignDBNames), toQueryMarks(primaryKeys))
	values := toQueryValues(primaryKeys)
	if relation.PolymorphicType != "" {
		query += fmt.Sprintf(" AND %v = ?", scope.Quote(relation.PolymorphicDBName))
		values = append(values, relation.PolymorphicValue)
	}

	results := field.makeSlice()
	scope.Err(preloadDB.Where(query, values...).Find(results, preloadConditions...).Error)

	// assign find results
	var (
		indirectScopeValue = scope.IndirectValue()
	)

	resultsValue := reflect.ValueOf(results)
	for resultsValue.Kind() == reflect.Ptr {
		resultsValue = resultsValue.Elem()
	}

	if indirectScopeValue.Kind() == reflect.Slice {
		preloadMap := make(map[string][]reflect.Value)
		for i := 0; i < resultsValue.Len(); i++ {
			result := resultsValue.Index(i)
			foreignValues := getValueFromFields(result, relation.ForeignFieldNames)
			preloadMap[toString(foreignValues)] = append(preloadMap[toString(foreignValues)], result)
		}

		for j := 0; j < indirectScopeValue.Len(); j++ {
			reflectValue := indirectScopeValue.Index(j)
			for reflectValue.Kind() == reflect.Ptr {
				reflectValue = reflectValue.Elem()
			}
			objectRealValue := getValueFromFields(reflectValue, relation.AssociationForeignFieldNames)
			f := reflectValue.FieldByName(field.GetName())
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
func (scope *Scope) handleBelongsToPreload(field *StructField, conditions []interface{}) {
	relation := field.Relationship

	// preload conditions
	preloadDB, preloadConditions := scope.generatePreloadDBWithConditions(conditions)

	// get relations's primary keys
	primaryKeys := scope.getColumnAsArray(relation.ForeignFieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}

	// find relations
	results := field.makeSlice()
	scope.Err(preloadDB.Where(fmt.Sprintf("%v IN (%v)", scope.toQueryCondition(relation.AssociationForeignDBNames), toQueryMarks(primaryKeys)), toQueryValues(primaryKeys)...).Find(results, preloadConditions...).Error)

	// assign find results
	var (
		indirectScopeValue = scope.IndirectValue()
	)

	resultsValue := reflect.ValueOf(results)
	for resultsValue.Kind() == reflect.Ptr {
		resultsValue = resultsValue.Elem()
	}

	for i := 0; i < resultsValue.Len(); i++ {
		result := resultsValue.Index(i)
		if indirectScopeValue.Kind() == reflect.Slice {
			value := getValueFromFields(result, relation.AssociationForeignFieldNames)
			for j := 0; j < indirectScopeValue.Len(); j++ {
				reflectValue := indirectScopeValue.Index(j)
				for reflectValue.Kind() == reflect.Ptr {
					reflectValue = reflectValue.Elem()
				}
				if equalAsString(getValueFromFields(reflectValue, relation.ForeignFieldNames), value) {
					reflectValue.FieldByName(field.GetName()).Set(result)
				}
			}
		} else {
			scope.Err(field.Set(result))
		}
	}
}

// handleManyToManyPreload used to preload many to many associations
func (scope *Scope) handleManyToManyPreload(field *StructField, conditions []interface{}) {
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
		var joinTableFields StructFields
		for _, sourceKey := range sourceKeys {
			joinTableFields.add(
				&StructField{
					DBName: sourceKey,
					Value:  reflect.New(foreignKeyType).Elem(),
					flags:  0 | (1 << IS_NORMAL),
				})
		}

		scope.scan(rows, columns, append(fields, joinTableFields...))

		var foreignKeys = make([]interface{}, len(sourceKeys))
		// generate hashed forkey keys in join table
		for idx, joinTableField := range joinTableFields {
			if !joinTableField.Value.IsNil() {
				foreignKeys[idx] = joinTableField.Value.Elem().Interface()
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
		foreignFieldNames  = StrSlice{}
	)

	for _, dbName := range relation.ForeignFieldNames {
		if field, ok := scope.FieldByName(dbName); ok {
			foreignFieldNames.add(field.GetName())
		}
	}

	if indirectScopeValue.Kind() == reflect.Slice {
		for j := 0; j < indirectScopeValue.Len(); j++ {
			reflectValue := indirectScopeValue.Index(j)
			for reflectValue.Kind() == reflect.Ptr {
				reflectValue = reflectValue.Elem()
			}
			key := toString(getValueFromFields(reflectValue, foreignFieldNames))
			fieldsSourceMap[key] = append(fieldsSourceMap[key], reflectValue.FieldByName(field.GetName()))
		}
	} else if indirectScopeValue.IsValid() {
		key := toString(getValueFromFields(indirectScopeValue, foreignFieldNames))
		fieldsSourceMap[key] = append(fieldsSourceMap[key], indirectScopeValue.FieldByName(field.GetName()))
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

////////////////////////////////////////////////////////////////////////////////
// moved here from callback functions
////////////////////////////////////////////////////////////////////////////////

//============================================
//Callback create functions
//============================================
// beforeCreateCallback will invoke `BeforeSave`, `BeforeCreate` method before creating
func (scope *Scope) beforeCreateCallback() {
	if !scope.HasError() {
		scope.CallMethod("BeforeSave")
	}
	if !scope.HasError() {
		scope.CallMethod("BeforeCreate")
	}
}

// updateTimeStampForCreateCallback will set `CreatedAt`, `UpdatedAt` when creating
func (scope *Scope) updateTimeStampForCreateCallback() {
	if !scope.HasError() {
		now := NowFunc()
		scope.SetColumn("CreatedAt", now)
		scope.SetColumn("UpdatedAt", now)
	}
}

// createCallback the callback used to insert data into database
func (scope *Scope) createCallback() {
	if !scope.HasError() {
		defer scope.trace(NowFunc())

		var (
			columns, placeholders        StrSlice
			blankColumnsWithDefaultValue StrSlice
		)

		for _, field := range scope.Fields() {
			if scope.changeableField(field) {
				if field.IsNormal() {
					if field.IsBlank() && field.HasDefaultValue() {
						blankColumnsWithDefaultValue.add(scope.Quote(field.DBName))
						scope.InstanceSet("gorm:blank_columns_with_default_value", blankColumnsWithDefaultValue)
					} else if !field.IsPrimaryKey() || !field.IsBlank() {
						columns.add(scope.Quote(field.DBName))
						placeholders.add(scope.AddToVars(field.Value.Interface()))
					}
				} else if field.Relationship != nil && field.Relationship.Kind == BELONGS_TO {
					for _, foreignKey := range field.Relationship.ForeignDBNames {
						if foreignField, ok := scope.FieldByName(foreignKey); ok && !scope.changeableField(foreignField) {
							columns.add(scope.Quote(foreignField.DBName))
							placeholders.add(scope.AddToVars(foreignField.Value.Interface()))
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
			if result, err := scope.AsSQLDB().Exec(scope.SQL, scope.SQLVars...); scope.Err(err) == nil {
				// set rows affected count
				scope.con.RowsAffected, _ = result.RowsAffected()

				// set primary value to primary field
				if primaryField != nil && primaryField.IsBlank() {
					if primaryValue, err := result.LastInsertId(); scope.Err(err) == nil {
						scope.Err(primaryField.Set(primaryValue))
					}
				}
			}
		} else {
			if err := scope.AsSQLDB().QueryRow(scope.SQL, scope.SQLVars...).Scan(primaryField.Value.Addr().Interface()); scope.Err(err) == nil {
				primaryField.unsetFlag(IS_BLANK)
				scope.con.RowsAffected = 1
			}
		}
	}
}

// forceReloadAfterCreateCallback will reload columns that having default value, and set it back to current object
func (scope *Scope) forceReloadAfterCreateCallback() {
	if blankColumnsWithDefaultValue, ok := scope.InstanceGet("gorm:blank_columns_with_default_value"); ok {
		sSlice, yes := blankColumnsWithDefaultValue.(StrSlice)
		if !yes {
			fmt.Errorf("ERROR in forceReloadAfterCreateCallback : blankColumnsWithDefaultValue IS NOT StrSlice!\n")
		}
		db := scope.Con().New().Table(scope.TableName()).Select(sSlice.slice())
		for _, field := range scope.Fields() {
			if field.IsPrimaryKey() && !field.IsBlank() {
				db = db.Where(fmt.Sprintf("%v = ?", field.DBName), field.Value.Interface())
			}
		}

		db.Scan(scope.Value)
	}
}

// afterCreateCallback will invoke `AfterCreate`, `AfterSave` method after creating
func (scope *Scope) afterCreateCallback() {
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
func (scope *Scope) saveBeforeAssociationsCallback() {
	if !scope.shouldSaveAssociations() {
		return
	}
	for _, field := range scope.Fields() {
		if scope.changeableField(field) && !field.IsBlank() && !field.IsIgnored() {
			if ok, relationship := scope.saveFieldAsAssociation(field); ok && relationship.Kind == BELONGS_TO {
				fieldValue := field.Value.Addr().Interface()
				scope.Err(scope.NewCon().Save(fieldValue).Error)
				if relationship.ForeignFieldNames.len() != 0 {
					// set value's foreign key
					for idx, fieldName := range relationship.ForeignFieldNames {
						associationForeignName := relationship.AssociationForeignDBNames[idx]
						if foreignField, ok := scope.New(fieldValue).FieldByName(associationForeignName); ok {
							scope.Err(scope.SetColumn(fieldName, foreignField.Value.Interface()))
						}
					}
				}
			}
		}
	}
}

func (scope *Scope) saveAfterAssociationsCallback() {
	if !scope.shouldSaveAssociations() {
		return
	}
	for _, field := range scope.Fields() {
		if scope.changeableField(field) && !field.IsBlank() && !field.IsIgnored() {
			//Attention : relationship.Kind <= HAS_ONE
			if ok, relationship := scope.saveFieldAsAssociation(field); ok && relationship.Kind <= HAS_ONE {
				value := field.Value

				switch value.Kind() {
				case reflect.Slice:
					for i := 0; i < value.Len(); i++ {
						newDB := scope.NewCon()
						elem := value.Index(i).Addr().Interface()
						newScope := newDB.NewScope(elem)

						if relationship.JoinTableHandler == nil && relationship.ForeignFieldNames.len() != 0 {
							for idx, fieldName := range relationship.ForeignFieldNames {
								associationForeignName := relationship.AssociationForeignDBNames[idx]
								if f, ok := scope.FieldByName(associationForeignName); ok {
									scope.Err(newScope.SetColumn(fieldName, f.Value.Interface()))
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
					if relationship.ForeignFieldNames.len() != 0 {
						for idx, fieldName := range relationship.ForeignFieldNames {
							associationForeignName := relationship.AssociationForeignDBNames[idx]
							if f, ok := scope.FieldByName(associationForeignName); ok {
								scope.Err(newScope.SetColumn(fieldName, f.Value.Interface()))
							}
						}
					}

					if relationship.PolymorphicType != "" {
						scope.Err(newScope.SetColumn(relationship.PolymorphicType, relationship.PolymorphicValue))
					}
					scope.Err(scope.NewCon().Save(elem).Error)
				}
			}
		}
	}
}

//============================================
// Callback update functions
//============================================
// assignUpdatingAttributesCallback assign updating attributes to model
func (scope *Scope) assignUpdatingAttributesCallback() {
	if attrs, ok := scope.InstanceGet("gorm:update_interface"); ok {
		if updateMaps, hasUpdate := scope.updatedAttrsWithValues(attrs); hasUpdate {
			scope.InstanceSet("gorm:update_attrs", updateMaps)
		} else {
			scope.SkipLeft()
		}
	}
}

// beforeUpdateCallback will invoke `BeforeSave`, `BeforeUpdate` method before updating
func (scope *Scope) beforeUpdateCallback() {
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
func (scope *Scope) updateTimeStampForUpdateCallback() {
	if _, ok := scope.Get("gorm:update_column"); !ok {
		scope.SetColumn("UpdatedAt", NowFunc())
	}
}

// updateCallback the callback used to update data to database
func (scope *Scope) updateCallback() {
	if !scope.HasError() {
		var sqls []string

		if updateAttrs, ok := scope.InstanceGet("gorm:update_attrs"); ok {
			for column, value := range updateAttrs.(map[string]interface{}) {
				sqls = append(sqls, fmt.Sprintf("%v = %v", scope.Quote(column), scope.AddToVars(value)))
			}
		} else {
			for _, field := range scope.Fields() {
				if scope.changeableField(field) {
					if !field.IsPrimaryKey() && field.IsNormal() {
						sqls = append(sqls, fmt.Sprintf("%v = %v", scope.Quote(field.DBName), scope.AddToVars(field.Value.Interface())))
					} else if relationship := field.Relationship; relationship != nil && relationship.Kind == BELONGS_TO {
						for _, foreignKey := range relationship.ForeignDBNames {
							if foreignField, ok := scope.FieldByName(foreignKey); ok && !scope.changeableField(foreignField) {
								sqls = append(sqls,
									fmt.Sprintf("%v = %v", scope.Quote(foreignField.DBName), scope.AddToVars(foreignField.Value.Interface())))
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
func (scope *Scope) afterUpdateCallback() {
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
func (scope *Scope) queryCallback() {
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
		scope.con.RowsAffected = 0
		if str, ok := scope.Get("gorm:query_option"); ok {
			scope.SQL += addExtraSpaceIfExist(fmt.Sprint(str))
		}

		if rows, err := scope.AsSQLDB().Query(scope.SQL, scope.SQLVars...); scope.Err(err) == nil {
			defer rows.Close()

			columns, _ := rows.Columns()
			for rows.Next() {
				scope.con.RowsAffected++

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

			if scope.con.RowsAffected == 0 && !isSlice {
				scope.Err(ErrRecordNotFound)
			}
		}
	}
}

// afterQueryCallback will invoke `AfterFind` method after querying
func (scope *Scope) afterQueryCallback() {
	if !scope.HasError() {
		scope.CallMethod("AfterFind")
	}
}

//============================================
// Callback query preload function
//============================================
// preloadCallback used to preload associations
func (scope *Scope) preloadCallback() {
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
			//there is no next level
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
					if field.GetName() != preloadField || field.Relationship == nil {
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
				//if preloadField is struct or slice, we need to get it's scope
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
func (scope *Scope) beforeDeleteCallback() {
	if !scope.HasError() {
		scope.CallMethod("BeforeDelete")
	}
}

// deleteCallback used to delete data from database or set deleted_at to current time (when using with soft delete)
func (scope *Scope) deleteCallback() {
	if !scope.HasError() {
		var extraOption string
		if str, ok := scope.Get("gorm:delete_option"); ok {
			extraOption = fmt.Sprint(str)
		}

		if !scope.Search.Unscoped && scope.GetModelStruct().HasColumn("DeletedAt") {
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
func (scope *Scope) afterDeleteCallback() {
	if !scope.HasError() {
		scope.CallMethod("AfterDelete")
	}
}
