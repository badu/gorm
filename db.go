package gorm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

func (con *DBCon) set(settintType uint64, value interface{}) *DBCon {
	return con.clone().localSet(settintType, value)
}

// InstantSet instant set setting, will affect current db
func (con *DBCon) localSet(settintType uint64, value interface{}) *DBCon {
	con.settings[settintType] = value
	return con
}

func (con *DBCon) get(settingType uint64) (interface{}, bool) {
	value, ok := con.settings[settingType]
	return value, ok
}

// Set set setting by name, which could be used in callbacks, will clone a new db, and update its setting
func (con *DBCon) Set(name string, value interface{}) *DBCon {
	clone := con.clone()
	settingType, ok := gormSettingsMap[name]
	if ok {
		clone.localSet(settingType, value)
	} else {
		clone.AddError(fmt.Errorf("Setting %q not declared. Can't set!", name))
	}
	return clone
}

// Get get setting by name
func (con *DBCon) Get(name string) (interface{}, bool) {
	settingType, has := gormSettingsMap[name]
	if has {
		value, ok := con.get(settingType)
		return value, ok
	}
	return nil, false
}

////////////////////////////////////////////////////////////////////////////////
// "unscoped" methods
////////////////////////////////////////////////////////////////////////////////
// Where return a new relation, filter records with given conditions, accepts `map`, `struct` or `string` as conditions
// Note : no scope
func (con *DBCon) Where(query interface{}, args ...interface{}) *DBCon {
	clone := con.clone()
	clone.search.Where(query, args...)
	return clone
}

// Or filter records that match before conditions or this one, similar to `Where`
// Note : no scope
func (con *DBCon) Or(query interface{}, args ...interface{}) *DBCon {
	clone := con.clone()
	clone.search.Or(query, args...)
	return clone
}

// Not filter records that don't match current conditions, similar to `Where`
// Note : no scope
func (con *DBCon) Not(query interface{}, args ...interface{}) *DBCon {
	clone := con.clone()
	clone.search.Not(query, args...)
	return clone
}

// Limit specify the number of records to be retrieved
//Note : no scope
func (con *DBCon) Limit(limit interface{}) *DBCon {
	clone := con.clone()
	clone.search.Limit(limit)
	return clone
}

// Offset specify the number of records to skip before starting to return the records
// Note : no scope
func (con *DBCon) Offset(offset interface{}) *DBCon {
	clone := con.clone()
	clone.search.Offset(offset)
	return clone
}

// Order specify order when retrieve records from database, set reorder to `true` to overwrite defined conditions
//     db.Order("name DESC")
//     db.Order("name DESC", true) // reorder
//     db.Order(gorm.Expr("name = ? DESC", "first")) // sql expression
// Note : no scope
func (con *DBCon) Order(value interface{}, reorder ...bool) *DBCon {
	clone := con.clone()
	clone.search.Order(value, reorder...)
	return clone
}

// Select specify fields that you want to retrieve from database when querying, by default, will select all fields;
// When creating/updating, specify fields that you want to save to database
// Note : no scope
func (con *DBCon) Select(query string, args ...interface{}) *DBCon {
	clone := con.clone()
	clone.search.Select(query, args...)
	return clone
}

// Omit specify fields that you want to ignore when saving to database for creating, updating
// Note : no scope
func (con *DBCon) Omit(columns ...string) *DBCon {
	clone := con.clone()
	clone.search.Omit(columns...)
	return clone
}

// Group specify the group method on the find
// Note : no scope
func (con *DBCon) Group(query string) *DBCon {
	clone := con.clone()
	clone.search.Group(query)
	return clone
}

// Having specify HAVING conditions for GROUP BY
// Note : no scope
func (con *DBCon) Having(query string, values ...interface{}) *DBCon {
	clone := con.clone()
	clone.search.Having(query, values...)
	return clone
}

// Joins specify Joins conditions
//     db.Joins("JOIN emails ON emails.user_id = users.id AND emails.email = ?", "user@example.org").Find(&user)
//Note:no scope
func (con *DBCon) Joins(query string, args ...interface{}) *DBCon {
	clone := con.clone()
	clone.search.Joins(query, args...)
	return clone
}

// Unscoped return all record including deleted record, refer Soft Delete
// Note : no scope (as the name says)
func (con *DBCon) Unscoped() *DBCon {
	clone := con.clone()
	clone.search.setUnscoped()
	return clone
}

// Attrs initialize struct with argument if record not found with `FirstOrInit` or `FirstOrCreate`
// Note : no scope
func (con *DBCon) Attrs(attrs ...interface{}) *DBCon {
	clone := con.clone()
	clone.search.Attrs(attrs...)
	return clone
}

// Assign assign result with argument regardless it is found or not with `FirstOrInit` or `FirstOrCreate`
// Note : no scope
func (con *DBCon) Assign(attrs ...interface{}) *DBCon {
	clone := con.clone()
	clone.search.Assign(attrs...)
	return clone
}

// Raw use raw sql as conditions, won't run it unless invoked by other methods
//    db.Raw("SELECT name, age FROM users WHERE name = ?", 3).Scan(&result)
// Note : no scope
func (con *DBCon) Raw(sql string, values ...interface{}) *DBCon {
	clone := con.clone()
	clone.search.SetRaw().Where(sql, values...)
	return clone
}

// Preload preload associations with given conditions
//    db.Preload("Orders", "state NOT IN (?)", "cancelled").Find(&users)
// Note : no scope
func (con *DBCon) Preload(column string, conditions ...interface{}) *DBCon {
	clone := con.clone()
	clone.search.Preload(column, conditions...)
	return clone
}

////////////////////////////////////////////////////////////////////////////////
// scoped and other methods
////////////////////////////////////////////////////////////////////////////////
// Close close current db connection
func (con *DBCon) Close() error {
	return con.parent.sqli.(*sql.DB).Close()
}

// gets interface casted to `*sql.DB` from current connection
func (con *DBCon) DB() *sql.DB {
	return con.sqli.(*sql.DB)
}

// Dialect get dialect
func (con *DBCon) Dialect() Dialect {
	return con.parent.dialect
}

// NewScope create a scope for current operation
func (con *DBCon) NewScope(value interface{}) *Scope {
	conClone := con.clone()
	conClone.search.Value = value
	//TODO : @Badu - this is the point where connection passes over the search to scope
	return &Scope{con: conClone, Search: conClone.search.Clone(), Value: value}
}

// CommonDB return the underlying `*sql.DB` or `*sql.Tx` instance, mainly intended to allow coexistence with legacy non-GORM code.
func (con *DBCon) AsSQLDB() sqlInterf {
	return con.sqli
}

// Callback return `Callbacks` container, you could add/change/delete callbacks with it
//     db.Callback().Create().Register("update_created_at", updateCreated)
func (con *DBCon) Callback() *Callbacks {
	con.parent.callbacks = con.parent.callbacks.clone()
	return con.parent.callbacks
}

// SetLogger replace default logger
func (con *DBCon) SetLogger(log logger) {
	con.logger = log
}

// LogMode set log mode, `true` for detailed logs, `false` for no log, default, will only print error logs
func (con *DBCon) LogMode(enable bool) *DBCon {
	if enable {
		con.logMode = LOG_VERBOSE
	} else {
		con.logMode = LOG_OFF
	}
	return con
}

func (con *DBCon) SetLogMode(mode int) {
	con.logMode = mode
}

// SingularTable use singular table by default
func (con *DBCon) SingularTable(enable bool) {
	ModelStructsMap = &safeModelStructsMap{l: new(sync.RWMutex), m: make(map[reflect.Type]*ModelStruct)}
	con.parent.singularTable = enable
}

// Scopes pass current database connection to arguments `func(*DBCon) *DBCon`, which could be used to add conditions dynamically
//     func AmountGreaterThan1000(db *gorm.DB) *gorm.DB {
//         return db.Where("amount > ?", 1000)
//     }
//
//     func OrderStatus(status []string) func (db *gorm.DB) *gorm.DB {
//         return func (db *gorm.DB) *gorm.DB {
//             return db.Scopes(AmountGreaterThan1000).Where("status in (?)", status)
//         }
//     }
//
//     db.Scopes(AmountGreaterThan1000, OrderStatus([]string{"paid", "shipped"})).Find(&orders)
func (con *DBCon) Scopes(funcs ...DBConFunc) *DBCon {
	for _, f := range funcs {
		//TODO : @Badu - assignment to method receiver propagates only to callees but not to callers
		con = f(con)
	}
	return con
}

// First find first record that match given conditions, order by primary key
func (con *DBCon) First(out interface{}, where ...interface{}) *DBCon {
	newScope := con.NewScope(out)
	newScope.Search.Limit(1)
	newScope.Set(ORDER_BY_PK_SETTING, ASCENDENT)
	if len(where) > 0 {
		newScope.Search.Wheres(where...)
	}
	newScope = newScope.postQuery()
	if con.parent.callbacks.queries.len() > 0 {
		newScope.callCallbacks(con.parent.callbacks.queries)
	}
	return newScope.con
}

// Last find last record that match given conditions, order by primary key
func (con *DBCon) Last(out interface{}, where ...interface{}) *DBCon {
	newScope := con.NewScope(out)
	newScope.Search.Limit(1)
	newScope.Set(ORDER_BY_PK_SETTING, DESCENDENT)
	if len(where) > 0 {
		newScope.Search.Wheres(where...)
	}
	newScope = newScope.postQuery()
	if con.parent.callbacks.queries.len() > 0 {
		newScope.callCallbacks(con.parent.callbacks.queries)
	}
	return newScope.con
}

// Find find records that match given conditions
func (con *DBCon) Find(out interface{}, where ...interface{}) *DBCon {
	newScope := con.NewScope(out)
	if len(where) > 0 {
		newScope.Search.Wheres(where...)
	}
	newScope = newScope.postQuery()
	if con.parent.callbacks.queries.len() > 0 {
		newScope.callCallbacks(con.parent.callbacks.queries)
	}
	return newScope.con
}

// Scan scan value to a struct
func (con *DBCon) Scan(dest interface{}) *DBCon {
	newScope := con.NewScope(con.search.Value)
	newScope = newScope.Set(QUERY_DEST_SETTING, dest).postQuery()
	if con.parent.callbacks.queries.len() > 0 {
		newScope.callCallbacks(con.parent.callbacks.queries)
	}
	return newScope.con
}

// Row return `*sql.Row` with given conditions
func (con *DBCon) Row() *sql.Row {
	return con.NewScope(con.search.Value).row()
}

// Rows return `*sql.Rows` with given conditions
func (con *DBCon) Rows() (*sql.Rows, error) {
	return con.NewScope(con.search.Value).rows()
}

// ScanRows scan `*sql.Rows` to give struct
func (con *DBCon) ScanRows(rows *sql.Rows, result interface{}) error {
	var (
		clone        = con.clone()
		scope        = clone.NewScope(result)
		columns, err = rows.Columns()
	)

	if clone.AddError(err) == nil {
		scope.scan(rows, columns, scope.Fields())
	}

	return clone.Error
}

// Pluck used to query single column from a model as a map
//     var ages []int64
//     db.Find(&users).Pluck("age", &ages)
func (con *DBCon) Pluck(column string, value interface{}) *DBCon {
	return con.NewScope(con.search.Value).pluck(column, value).con
}

// Count get how many records for a model
func (con *DBCon) Count(value interface{}) *DBCon {
	return con.NewScope(con.search.Value).count(value).con
}

// Related get related associations
func (con *DBCon) Related(value interface{}, foreignKeys ...string) *DBCon {
	return con.NewScope(con.search.Value).related(value, foreignKeys...).con
}

// FirstOrInit find first matched record or initialize a new one with given conditions (only works with struct, map conditions)
func (con *DBCon) FirstOrInit(out interface{}, where ...interface{}) *DBCon {
	conClone := con.clone()
	if result := conClone.First(out, where...); result.Error != nil {
		if !result.RecordNotFound() {
			return result
		}
		newScope := conClone.NewScope(out)
		newScope.Search.Wheres(where...).initialize(newScope)
	} else {
		scope := conClone.NewScope(out)
		args := scope.Search.getFirst(assign_attrs)
		if args != nil {
			updatedAttrsWithValues(scope, args.args)
		}
	}
	return conClone
}

// FirstOrCreate find first matched record or create a new one with given conditions (only works with struct, map conditions)
func (con *DBCon) FirstOrCreate(out interface{}, where ...interface{}) *DBCon {
	conClone := con.clone()
	if result := con.First(out, where...); result.Error != nil {
		if !result.RecordNotFound() {
			return result
		}
		newScope := conClone.NewScope(out)
		newScope.Search.Wheres(where...).initialize(newScope)
		newScope = newScope.postCreate()
		if conClone.parent.callbacks.creates.len() > 0 {
			newScope.callCallbacks(conClone.parent.callbacks.creates)
		}
		return newScope.con
	} else if conClone.search.hasAssign() {
		scope := conClone.NewScope(out)
		args := scope.Search.getFirst(assign_attrs)
		if args != nil {
			scope.attrs = args.args
			//scope.InstanceSet(UPDATE_INTERF_SETTING, args.args)
		}
		scope = scope.postUpdate()
		if conClone.parent.callbacks.updates.len() > 0 {
			scope.callCallbacks(conClone.parent.callbacks.updates)
		}
		return scope.con
	}
	return conClone
}

// Updates update attributes with callbacks
func (con *DBCon) Updates(values interface{}, ignoreProtectedAttrs ...bool) *DBCon {
	newScope := con.NewScope(con.search.Value)
	newScope.attrs = values
	newScope = newScope.Set(IGNORE_PROTEC_SETTING, len(ignoreProtectedAttrs) > 0).postUpdate()
	if con.parent.callbacks.updates.len() > 0 {
		newScope.callCallbacks(con.parent.callbacks.updates)
	}
	return newScope.con
}

// Update update attributes with callbacks
func (con *DBCon) Update(attrs ...interface{}) *DBCon {
	result := argsToInterface(attrs...)
	return con.Updates(result, true)
}

// UpdateColumns update attributes without callbacks
func (con *DBCon) UpdateColumns(values interface{}) *DBCon {
	newScope := con.NewScope(con.search.Value)
	newScope.attrs = values
	newScope = newScope.Set(UPDATE_COLUMN_SETTING, true).Set(SAVE_ASSOC_SETTING, false).postUpdate()
	if con.parent.callbacks.updates.len() > 0 {
		newScope.callCallbacks(con.parent.callbacks.updates)
	}
	return newScope.con
}

// UpdateColumn update attributes without callbacks
func (con *DBCon) UpdateColumn(attrs ...interface{}) *DBCon {
	result := argsToInterface(attrs...)
	return con.UpdateColumns(result)
}

// Save update value in database, if the value doesn't have primary key, will insert it
func (con *DBCon) Save(value interface{}) *DBCon {
	scope := con.NewScope(value)
	if !scope.PrimaryKeyZero() {
		scope = scope.postUpdate()
		if con.parent.callbacks.updates.len() > 0 {
			scope.callCallbacks(con.parent.callbacks.updates)
		}
		if scope.con.Error == nil && scope.con.RowsAffected == 0 {
			return newCon(con).FirstOrCreate(value)
		}
		return scope.con
	}
	scope = scope.postCreate()
	if con.parent.callbacks.creates.len() > 0 {
		scope.callCallbacks(con.parent.callbacks.creates)
	}
	return scope.con
}

// Create insert the value into database
func (con *DBCon) Create(value interface{}) *DBCon {
	scope := con.NewScope(value).postCreate()
	if con.parent.callbacks.creates.len() > 0 {
		scope.callCallbacks(con.parent.callbacks.creates)
	}
	return scope.con
}

// Delete delete value match given conditions, if the value has primary key, then will including the primary key as condition
func (con *DBCon) Delete(value interface{}, where ...interface{}) *DBCon {
	scope := con.NewScope(value)
	scope.Search.Wheres(where...)
	scope = scope.postDelete()
	if con.parent.callbacks.deletes.len() > 0 {
		scope.callCallbacks(con.parent.callbacks.deletes)
	}
	return scope.con
}

// Exec execute raw sql
func (con *DBCon) Exec(sql string, values ...interface{}) *DBCon {
	scope := con.NewScope(nil)
	scope.Raw(scope.Search.exec(scope, sql, values...))
	return scope.Exec().con
}

// Model specify the model you would like to run db operations
//    // update all users's name to `hello`
//    db.Model(&User{}).Update("name", "hello")
//    // if user's primary key is non-blank, will use it as condition, then will only update the user's name to `hello`
//    db.Model(&user).Update("name", "hello")
func (con *DBCon) Model(value interface{}) *DBCon {
	c := con.clone()
	c.search.Value = value
	return c
}

// Table specify the table you would like to run db operations
func (con *DBCon) Table(name string) *DBCon {
	clone := con.clone()
	clone.search.Table(name)
	//reseting the value
	clone.search.Value = nil
	return clone
}

// Debug start debug mode
func (con *DBCon) Debug() *DBCon {
	return con.clone().LogMode(true)
}

// Begin begin a transaction
func (con *DBCon) Begin() *DBCon {
	c := con.clone()
	if db, ok := c.sqli.(sqlDb); ok {
		//clone.db implements Begin() -> call Begin()
		tx, err := db.Begin()
		c.sqli = interface{}(tx).(sqlInterf)
		c.AddError(err)
	} else {
		c.AddError(ErrCantStartTransaction)
	}
	return c
}

// Commit commit a transaction
func (con *DBCon) Commit() *DBCon {
	if db, ok := con.sqli.(sqlTx); ok {
		//orm.db implements Commit() and Rollback() -> call Commit()
		con.AddError(db.Commit())
	} else {
		con.AddError(ErrInvalidTransaction)
	}
	return con
}

// Rollback rollback a transaction
func (con *DBCon) Rollback() *DBCon {
	if db, ok := con.sqli.(sqlTx); ok {
		//orm.db implements Commit() and Rollback() -> call Rollback()
		con.AddError(db.Rollback())
	} else {
		con.AddError(ErrInvalidTransaction)
	}
	return con
}

// NewRecord check if value's primary key is blank
func (con *DBCon) NewRecord(value interface{}) bool {
	return con.NewScope(value).PrimaryKeyZero()
}

// RecordNotFound check if returning ErrRecordNotFound error
func (con *DBCon) RecordNotFound() bool {
	for _, err := range con.GetErrors() {
		if err == ErrRecordNotFound {
			return true
		}
	}
	return false
}

// CreateTable create table for models
func (con *DBCon) CreateTable(models ...interface{}) *DBCon {
	conn := con.Unscoped()
	for _, model := range models {
		scope := conn.NewScope(model)
		createTable(scope)
		conn = scope.con
	}
	return conn
}

// DropTable drop table for models
func (con *DBCon) DropTable(values ...interface{}) *DBCon {
	conn := con.clone()
	for _, value := range values {
		if tableName, ok := value.(string); ok {
			conn = conn.Table(tableName)
		}
		scope := conn.NewScope(value)
		scope.Raw(
			fmt.Sprintf(
				"DROP TABLE %v",
				QuotedTableName(scope),
			),
		).Exec()
		conn = scope.con
	}
	return conn
}

// DropTableIfExists drop table if it is exist
func (con *DBCon) DropTableIfExists(values ...interface{}) *DBCon {
	db := con.clone()
	for _, value := range values {
		if con.HasTable(value) {
			db.AddError(con.DropTable(value).Error)
		}
	}
	return db
}

// HasTable check has table or not
func (con *DBCon) HasTable(value interface{}) bool {
	var (
		scope     = con.NewScope(value)
		tableName string
	)

	if name, ok := value.(string); ok {
		tableName = name
	} else {
		tableName = scope.TableName()
	}

	has := con.parent.dialect.HasTable(tableName)
	con.AddError(scope.con.Error)
	return has
}

// AutoMigrate run auto migration for given models, will only add missing fields, won't delete/change current data
func (con *DBCon) AutoMigrate(values ...interface{}) *DBCon {
	conn := con.Unscoped()
	//TODO : @Badu - find a way to order by relationships, so we can drop foreign keys before migrate
	for _, value := range values {
		scope := conn.NewScope(value)
		autoMigrate(scope)
		conn = scope.con
	}
	return conn
}

// ModifyColumn modify column to type
func (con *DBCon) ModifyColumn(column string, typ string) *DBCon {
	scope := con.NewScope(con.search.Value)
	scope.Raw(
		fmt.Sprintf(
			"ALTER TABLE %v MODIFY %v %v",
			QuotedTableName(scope),
			Quote(column, con.parent.dialect),
			typ,
		),
	).Exec()
	return scope.con
}

// DropColumn drop a column
func (con *DBCon) DropColumn(column string) *DBCon {
	scope := con.NewScope(con.search.Value)
	scope.Raw(
		fmt.Sprintf(
			"ALTER TABLE %v DROP COLUMN %v",
			QuotedTableName(scope),
			Quote(column, scope.con.parent.dialect),
		),
	).Exec()
	return scope.con
}

// AddIndex add index for columns with given name
func (con *DBCon) AddIndex(indexName string, columns ...string) *DBCon {
	scope := con.Unscoped().NewScope(con.search.Value)
	addIndex(scope, false, indexName, columns...)
	return scope.con
}

// AddUniqueIndex add unique index for columns with given name
func (con *DBCon) AddUniqueIndex(indexName string, columns ...string) *DBCon {
	scope := con.Unscoped().NewScope(con.search.Value)
	addIndex(scope, true, indexName, columns...)
	return scope.con
}

// RemoveIndex remove index with name
func (con *DBCon) RemoveIndex(indexName string) *DBCon {
	scope := con.NewScope(con.search.Value)
	con.parent.dialect.RemoveIndex(scope.TableName(), indexName)
	return scope.con
}

// AddForeignKey Add foreign key to the given scope, e.g:
//     db.Model(&User{}).AddForeignKey("city_id", "cities(id)", "RESTRICT", "RESTRICT")
//TODO : @Badu - make it work with interfaces instead of strings (field, dest)
func (con *DBCon) AddForeignKey(field string, dest string, onDelete string, onUpdate string) *DBCon {
	var (
		scope     = con.NewScope(con.search.Value)
		dialect   = con.parent.dialect
		tableName = scope.TableName()
		keyName   = dialect.BuildForeignKeyName(tableName, field, dest)
		query     = `ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s ON DELETE %s ON UPDATE %s;`
	)

	if dialect.HasForeignKey(tableName, keyName) {
		return scope.con
	}

	scope.Raw(
		fmt.Sprintf(
			query,
			QuotedTableName(scope),
			QuoteIfPossible(keyName, dialect),
			QuoteIfPossible(field, dialect),
			dest,
			onDelete,
			onUpdate,
		),
	).Exec()

	return scope.con
}

// Association start `Association Mode` to handler relations things easier in that mode
func (con *DBCon) Association(column string) *Association {
	var err error
	//ASSOCIATION_SOURCE_SETTING for plugins to extend gorm (original commit of 05.12.2016)
	scope := con.set(ASSOCIATION_SOURCE_SETTING, con.search.Value).NewScope(con.search.Value)
	primaryField := scope.PK()
	if primaryField == nil {
		err = errors.New("SCOPE : primary field is NIL")
	}
	if primaryField.IsBlank() {
		err = errors.New("primary key can't be nil")
	} else {
		if field, ok := scope.FieldByName(column); ok {
			ForeignFieldNames := field.GetSliceSetting(FOREIGN_FIELD_NAMES)
			if !field.HasRelations() || ForeignFieldNames.len() == 0 {
				err = fmt.Errorf("invalid association %v for %v", column, IndirectValue(scope.Value).Type())
			} else {
				return &Association{scope: scope, column: column, field: field}
			}
		} else {
			err = fmt.Errorf("%v doesn't have column %v", IndirectValue(scope.Value).Type(), column)
		}
	}

	return &Association{Error: err}
}

// SetJoinTableHandler set a model's join table handler for a relation
func (con *DBCon) SetJoinTableHandler(source interface{}, column string, handler JoinTableHandlerInterface) {
	scope := con.NewScope(source)
	for _, field := range scope.GetModelStruct().StructFields() {
		if field.StructName == column || field.DBName == column {
			if field.HasSetting(MANY2MANY_NAME) {
				src := (&Scope{Value: source}).GetModelStruct().ModelType
				destination := (&Scope{Value: field.Interface()}).GetModelStruct().ModelType
				handler.SetTable(field.GetStrSetting(MANY2MANY_NAME))
				handler.Setup(field, src, destination)
				//field.Relationship.JoinTableHandler = handler
				field.SetTagSetting(JOIN_TABLE_HANDLER, handler)
				if table := handler.Table(con); con.parent.dialect.HasTable(table) {
					con.Table(table).AutoMigrate(handler)
				}
			}
		}
	}
}

////////////////////////////////////////////////////////////////////////////////
//Errors
////////////////////////////////////////////////////////////////////////////////
// AddError add error to the db
func (con *DBCon) AddError(err error) error {
	if err != nil {
		if err != ErrRecordNotFound {
			switch con.logMode {
			case LOG_VERBOSE:
				con.log(err)
			case LOG_OFF:
				go con.toLog(fileWithLineNum(), err)
			case LOG_DEBUG:
				fmt.Printf("ERROR : %v\n%s\n", err, fullFileWithLineNum())
			}
			gormErrors := GormErrors(con.GetErrors())
			gormErrors = gormErrors.Add(err)
			if len(gormErrors.GetErrors()) > 1 {
				err = gormErrors
			}
		}

		con.Error = err
	}
	return err
}

// GetErrors get happened errors from the db
func (con *DBCon) GetErrors() []error {
	if errs, ok := con.Error.(errorsInterface); ok {
		return errs.GetErrors()
	} else if con.Error != nil {
		return []error{con.Error}
	}
	return nil
}

func (con *DBCon) Log(v ...interface{}) {
	con.toLog(append([]interface{}{"info", fileWithLineNum()}, v...)...)
}

////////////////////////////////////////////////////////////////////////////////
// Private Methods For *gorm.DBCon
////////////////////////////////////////////////////////////////////////////////
//returns a cloned connection - with extradata included
func (con *DBCon) clone() *DBCon {
	clone := DBCon{
		sqli:     con.sqli,
		parent:   con.parent,
		logger:   con.logger,
		logMode:  con.logMode,
		settings: map[uint64]interface{}{},
		Error:    con.Error,
	}
	for key, value := range con.settings {
		clone.settings[key] = value
	}
	if con.search == nil {
		clone.search = &Search{Conditions: make(SqlConditions)}
	} else {
		clone.search = con.search.Clone()
	}
	return &clone
}

func (con *DBCon) toLog(v ...interface{}) {
	con.logger.(logger).Print(v...)
}

func (con *DBCon) warnLog(v ...interface{}) {
	if con != nil {
		con.toLog(append([]interface{}{"warn", fileWithLineNum()}, v...)...)
	} else {
		fmt.Printf("Connection is NIL!")
	}
}

func (con *DBCon) log(v ...interface{}) {
	if con != nil {
		con.toLog(append([]interface{}{"log", fileWithLineNum()}, v...)...)
	}
}

func (con *DBCon) slog(sql string, t time.Time, vars ...interface{}) {
	if con.logMode == LOG_VERBOSE {
		con.toLog("sql", fileWithLineNum(), NowFunc().Sub(t), sql, vars)
	}
}
