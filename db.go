package gorm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

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

// New clone a new db connection without search conditions
func (con *DBCon) New() *DBCon {
	return con.clone(true, true)
}

// NewScope create a scope for current operation
func (con *DBCon) NewScope(value interface{}) *Scope {
	dbClone := con.clone(false, false)
	dbClone.Value = value
	return &Scope{con: dbClone, Search: dbClone.search.clone(), Value: value}
}

// CommonDB return the underlying `*sql.DB` or `*sql.Tx` instance, mainly intended to allow coexistence with legacy non-GORM code.
func (con *DBCon) CommonDB() sqlInterf {
	return con.sqli
}

// Callback return `Callbacks` container, you could add/change/delete callbacks with it
//     db.Callback().Create().Register("update_created_at", updateCreated)
func (con *DBCon) Callback() *Callback {
	con.parent.callback = con.parent.callback.clone()
	return con.parent.callback
}

// SetLogger replace default logger
func (con *DBCon) SetLogger(log logger) {
	con.logger = log
}

// LogMode set log mode, `true` for detailed logs, `false` for no log, default, will only print error logs
func (con *DBCon) LogMode(enable bool) *DBCon {
	if enable {
		con.logMode = 2
	} else {
		con.logMode = 1
	}
	return con
}

// SingularTable use singular table by default
func (con *DBCon) SingularTable(enable bool) {
	modelStructsMap = newModelStructsMap()
	con.parent.singularTable = enable
}

// Where return a new relation, filter records with given conditions, accepts `map`, `struct` or `string` as conditions
func (con *DBCon) Where(query interface{}, args ...interface{}) *DBCon {
	return con.clone(false, false).search.Where(query, args...).con
}

// Or filter records that match before conditions or this one, similar to `Where`
func (con *DBCon) Or(query interface{}, args ...interface{}) *DBCon {
	return con.clone(false, false).search.Or(query, args...).con
}

// Not filter records that don't match current conditions, similar to `Where`
func (con *DBCon) Not(query interface{}, args ...interface{}) *DBCon {
	return con.clone(false, false).search.Not(query, args...).con
}

// Limit specify the number of records to be retrieved
func (con *DBCon) Limit(limit interface{}) *DBCon {
	return con.clone(false, false).search.Limit(limit).con
}

// Offset specify the number of records to skip before starting to return the records
func (con *DBCon) Offset(offset interface{}) *DBCon {
	return con.clone(false, false).search.Offset(offset).con
}

// Order specify order when retrieve records from database, set reorder to `true` to overwrite defined conditions
//     db.Order("name DESC")
//     db.Order("name DESC", true) // reorder
//     db.Order(gorm.Expr("name = ? DESC", "first")) // sql expression
func (con *DBCon) Order(value interface{}, reorder ...bool) *DBCon {
	return con.clone(false, false).search.Order(value, reorder...).con
}

// Select specify fields that you want to retrieve from database when querying, by default, will select all fields;
// When creating/updating, specify fields that you want to save to database
func (con *DBCon) Select(query interface{}, args ...interface{}) *DBCon {
	return con.clone(false, false).search.Select(query, args...).con
}

// Omit specify fields that you want to ignore when saving to database for creating, updating
func (con *DBCon) Omit(columns ...string) *DBCon {
	return con.clone(false, false).search.Omit(columns...).con
}

// Group specify the group method on the find
func (con *DBCon) Group(query string) *DBCon {
	return con.clone(false, false).search.Group(query).con
}

// Having specify HAVING conditions for GROUP BY
func (con *DBCon) Having(query string, values ...interface{}) *DBCon {
	return con.clone(false, false).search.Having(query, values...).con
}

// Joins specify Joins conditions
//     db.Joins("JOIN emails ON emails.user_id = users.id AND emails.email = ?", "user@example.org").Find(&user)
func (con *DBCon) Joins(query string, args ...interface{}) *DBCon {
	return con.clone(false, false).search.Joins(query, args...).con
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
//TODO : Badu - replace this - it's soooo ugly
func (con *DBCon) Scopes(funcs ...func(*DBCon) *DBCon) *DBCon {
	for _, f := range funcs {
		//TODO : @Badu - assignment to method receiver propagates only to callees but not to callers
		con = f(con)
	}
	return con
}

// Unscoped return all record including deleted record, refer Soft Delete
func (con *DBCon) Unscoped() *DBCon {
	return con.clone(false, false).search.unscoped().con
}

// Attrs initialize struct with argument if record not found with `FirstOrInit` or `FirstOrCreate`
func (con *DBCon) Attrs(attrs ...interface{}) *DBCon {
	return con.clone(false, false).search.Attrs(attrs...).con
}

// Assign assign result with argument regardless it is found or not with `FirstOrInit` or `FirstOrCreate`
func (con *DBCon) Assign(attrs ...interface{}) *DBCon {
	return con.clone(false, false).search.Assign(attrs...).con
}

// First find first record that match given conditions, order by primary key
func (con *DBCon) First(out interface{}, where ...interface{}) *DBCon {
	newScope := con.clone(false, false).NewScope(out)
	newScope.Search.Limit(1)
	return newScope.Set("gorm:order_by_primary_key", "ASC").
		inlineCondition(where...).callCallbacks(con.parent.callback.queries).con
}

// Last find last record that match given conditions, order by primary key
func (con *DBCon) Last(out interface{}, where ...interface{}) *DBCon {
	newScope := con.clone(false, false).NewScope(out)
	newScope.Search.Limit(1)
	return newScope.Set("gorm:order_by_primary_key", "DESC").
		inlineCondition(where...).callCallbacks(con.parent.callback.queries).con
}

// Find find records that match given conditions
func (con *DBCon) Find(out interface{}, where ...interface{}) *DBCon {
	return con.clone(false, false).NewScope(out).inlineCondition(where...).callCallbacks(con.parent.callback.queries).con
}

// Scan scan value to a struct
func (con *DBCon) Scan(dest interface{}) *DBCon {
	return con.clone(false, false).NewScope(con.Value).Set("gorm:query_destination", dest).callCallbacks(con.parent.callback.queries).con
}

// Row return `*sql.Row` with given conditions
func (con *DBCon) Row() *sql.Row {
	return con.NewScope(con.Value).row()
}

// Rows return `*sql.Rows` with given conditions
func (con *DBCon) Rows() (*sql.Rows, error) {
	return con.NewScope(con.Value).rows()
}

// ScanRows scan `*sql.Rows` to give struct
func (con *DBCon) ScanRows(rows *sql.Rows, result interface{}) error {
	var (
		clone        = con.clone(false, false)
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
	return con.NewScope(con.Value).pluck(column, value).con
}

// Count get how many records for a model
func (con *DBCon) Count(value interface{}) *DBCon {
	return con.NewScope(con.Value).count(value).con
}

// Related get related associations
func (con *DBCon) Related(value interface{}, foreignKeys ...string) *DBCon {
	return con.clone(false, false).NewScope(con.Value).related(value, foreignKeys...).con
}

// FirstOrInit find first matched record or initialize a new one with given conditions (only works with struct, map conditions)
func (con *DBCon) FirstOrInit(out interface{}, where ...interface{}) *DBCon {
	c := con.clone(false, false)
	if result := c.First(out, where...); result.Error != nil {
		if !result.RecordNotFound() {
			return result
		}
		c.NewScope(out).inlineCondition(where...).initialize()
	} else {
		c.NewScope(out).updatedAttrsWithValues(c.search.assignAttrs)
	}
	return c
}

// FirstOrCreate find first matched record or create a new one with given conditions (only works with struct, map conditions)
func (con *DBCon) FirstOrCreate(out interface{}, where ...interface{}) *DBCon {
	c := con.clone(false, false)
	if result := con.First(out, where...); result.Error != nil {
		if !result.RecordNotFound() {
			return result
		}
		return c.NewScope(out).inlineCondition(where...).initialize().callCallbacks(c.parent.callback.creates).con
	} else if len(c.search.assignAttrs) > 0 {
		return c.NewScope(out).InstanceSet("gorm:update_interface", c.search.assignAttrs).callCallbacks(c.parent.callback.updates).con
	}
	return c
}

// Update update attributes with callbacks
func (con *DBCon) Update(attrs ...interface{}) *DBCon {
	return con.Updates(toSearchableMap(attrs...), true)
}

// Updates update attributes with callbacks
func (con *DBCon) Updates(values interface{}, ignoreProtectedAttrs ...bool) *DBCon {
	return con.clone(false, false).NewScope(con.Value).
		Set("gorm:ignore_protected_attrs", len(ignoreProtectedAttrs) > 0).
		InstanceSet("gorm:update_interface", values).
		callCallbacks(con.parent.callback.updates).con
}

// UpdateColumn update attributes without callbacks
func (con *DBCon) UpdateColumn(attrs ...interface{}) *DBCon {
	return con.UpdateColumns(toSearchableMap(attrs...))
}

// UpdateColumns update attributes without callbacks
func (con *DBCon) UpdateColumns(values interface{}) *DBCon {
	return con.clone(false, false).NewScope(con.Value).
		Set("gorm:update_column", true).
		Set("gorm:save_associations", false).
		InstanceSet("gorm:update_interface", values).
		callCallbacks(con.parent.callback.updates).con
}

// Save update value in database, if the value doesn't have primary key, will insert it
func (con *DBCon) Save(value interface{}) *DBCon {
	scope := con.clone(false, false).NewScope(value)
	if !scope.PrimaryKeyZero() {
		newDB := scope.callCallbacks(con.parent.callback.updates).con
		if newDB.Error == nil && newDB.RowsAffected == 0 {
			return con.New().FirstOrCreate(value)
		}
		return newDB
	}
	return scope.callCallbacks(con.parent.callback.creates).con
}

// Create insert the value into database
func (con *DBCon) Create(value interface{}) *DBCon {
	scope := con.clone(false, false).NewScope(value)
	return scope.callCallbacks(con.parent.callback.creates).con
}

// Delete delete value match given conditions, if the value has primary key, then will including the primary key as condition
func (con *DBCon) Delete(value interface{}, where ...interface{}) *DBCon {
	return con.clone(false, false).NewScope(value).inlineCondition(where...).callCallbacks(con.parent.callback.deletes).con
}

// Raw use raw sql as conditions, won't run it unless invoked by other methods
//    db.Raw("SELECT name, age FROM users WHERE name = ?", 3).Scan(&result)
func (con *DBCon) Raw(sql string, values ...interface{}) *DBCon {
	return con.clone(false, false).search.Raw(true).Where(sql, values...).con
}

// Exec execute raw sql
func (con *DBCon) Exec(sql string, values ...interface{}) *DBCon {
	scope := con.clone(false, false).NewScope(nil)
	generatedSQL := scope.buildWhereCondition(map[string]interface{}{"query": sql, "args": values})
	generatedSQL = strings.TrimSuffix(strings.TrimPrefix(generatedSQL, "("), ")")
	scope.Raw(generatedSQL)
	return scope.Exec().con
}

// Model specify the model you would like to run db operations
//    // update all users's name to `hello`
//    db.Model(&User{}).Update("name", "hello")
//    // if user's primary key is non-blank, will use it as condition, then will only update the user's name to `hello`
//    db.Model(&user).Update("name", "hello")
func (con *DBCon) Model(value interface{}) *DBCon {
	c := con.clone(false, false)
	c.Value = value
	return c
}

// Table specify the table you would like to run db operations
func (con *DBCon) Table(name string) *DBCon {
	clone := con.clone(false, false)
	clone.search.Table(name)
	clone.Value = nil
	return clone
}

// Debug start debug mode
func (con *DBCon) Debug() *DBCon {
	return con.clone(false, false).LogMode(true)
}

// Begin begin a transaction
func (con *DBCon) Begin() *DBCon {
	c := con.clone(false, false)
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
	return con.clone(false, false).NewScope(value).PrimaryKeyZero()
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
	db := con.Unscoped()
	for _, model := range models {
		db = db.NewScope(model).createTable().con
	}
	return db
}

// DropTable drop table for models
func (con *DBCon) DropTable(values ...interface{}) *DBCon {
	db := con.clone(false, false)
	for _, value := range values {
		if tableName, ok := value.(string); ok {
			db = db.Table(tableName)
		}

		db = db.NewScope(value).dropTable().con
	}
	return db
}

// DropTableIfExists drop table if it is exist
func (con *DBCon) DropTableIfExists(values ...interface{}) *DBCon {
	db := con.clone(false, false)
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
		scope     = con.clone(false, false).NewScope(value)
		tableName string
	)

	if name, ok := value.(string); ok {
		tableName = name
	} else {
		tableName = scope.TableName()
	}

	has := scope.Dialect().HasTable(tableName)
	con.AddError(scope.con.Error)
	return has
}

// AutoMigrate run auto migration for given models, will only add missing fields, won't delete/change current data
func (con *DBCon) AutoMigrate(values ...interface{}) *DBCon {
	db := con.Unscoped()
	//TODO : @Badu - find a way to order by relationships, so we can drop foreign keys before migrate
	for _, value := range values {
		db = db.NewScope(value).autoMigrate().con
	}
	return db
}

// ModifyColumn modify column to type
func (con *DBCon) ModifyColumn(column string, typ string) *DBCon {
	scope := con.clone(false, false).NewScope(con.Value)
	scope.modifyColumn(column, typ)
	return scope.con
}

// DropColumn drop a column
func (con *DBCon) DropColumn(column string) *DBCon {
	scope := con.clone(false, false).NewScope(con.Value)
	scope.dropColumn(column)
	return scope.con
}

// AddIndex add index for columns with given name
func (con *DBCon) AddIndex(indexName string, columns ...string) *DBCon {
	scope := con.Unscoped().NewScope(con.Value)
	scope.addIndex(false, indexName, columns...)
	return scope.con
}

// AddUniqueIndex add unique index for columns with given name
func (con *DBCon) AddUniqueIndex(indexName string, columns ...string) *DBCon {
	scope := con.Unscoped().NewScope(con.Value)
	scope.addIndex(true, indexName, columns...)
	return scope.con
}

// RemoveIndex remove index with name
func (con *DBCon) RemoveIndex(indexName string) *DBCon {
	scope := con.clone(false, false).NewScope(con.Value)
	scope.removeIndex(indexName)
	return scope.con
}

// AddForeignKey Add foreign key to the given scope, e.g:
//     db.Model(&User{}).AddForeignKey("city_id", "cities(id)", "RESTRICT", "RESTRICT")
//TODO : @Badu - make it work with interfaces instead of strings (field, dest)
func (con *DBCon) AddForeignKey(field string, dest string, onDelete string, onUpdate string) *DBCon {
	scope := con.clone(false, false).NewScope(con.Value)
	scope.addForeignKey(field, dest, onDelete, onUpdate)
	return scope.con
}

// Association start `Association Mode` to handler relations things easier in that mode
func (con *DBCon) Association(column string) *Association {
	var err error
	scope := con.clone(false, false).NewScope(con.Value)

	if primaryField := scope.PrimaryField(); primaryField.IsBlank {
		err = errors.New("primary key can't be nil")
	} else {
		if field, ok := scope.FieldByName(column); ok {
			if field.Relationship == nil || field.Relationship.ForeignFieldNames.len() == 0 {
				err = fmt.Errorf("invalid association %v for %v", column, scope.IndirectValue().Type())
			} else {
				return &Association{scope: scope, column: column, field: field}
			}
		} else {
			err = fmt.Errorf("%v doesn't have column %v", scope.IndirectValue().Type(), column)
		}
	}

	return &Association{Error: err}
}

// Preload preload associations with given conditions
//    db.Preload("Orders", "state NOT IN (?)", "cancelled").Find(&users)
func (con *DBCon) Preload(column string, conditions ...interface{}) *DBCon {
	return con.clone(false, false).search.Preload(column, conditions...).con
}

// Set set setting by name, which could be used in callbacks, will clone a new db, and update its setting
func (con *DBCon) Set(name string, value interface{}) *DBCon {
	return con.clone(false, false).InstantSet(name, value)
}

// InstantSet instant set setting, will affect current db
func (con *DBCon) InstantSet(name string, value interface{}) *DBCon {
	con.values[name] = value
	return con
}

// Get get setting by name
func (con *DBCon) Get(name string) (value interface{}, ok bool) {
	value, ok = con.values[name]
	return
}

// SetJoinTableHandler set a model's join table handler for a relation
func (con *DBCon) SetJoinTableHandler(source interface{}, column string, handler JoinTableHandlerInterface) {
	scope := con.NewScope(source)
	for _, field := range scope.GetModelStruct().StructFields {
		if field.GetName() == column || field.DBName == column {
			if many2many := field.GetSetting(MANY2MANY); many2many != "" {
				src := (&Scope{Value: source}).GetModelStruct().ModelType
				destination := (&Scope{Value: reflect.New(field.Struct.Type).Interface()}).GetModelStruct().ModelType
				handler.SetTable(many2many)
				handler.Setup(field.Relationship, src, destination)
				field.Relationship.JoinTableHandler = handler
				if table := handler.Table(con); scope.Dialect().HasTable(table) {
					con.Table(table).AutoMigrate(handler)
				}
			}
		}
	}
}

// AddError add error to the db
func (con *DBCon) AddError(err error) error {
	if err != nil {
		if err != ErrRecordNotFound {
			if con.logMode == 0 {
				go con.toLog(fileWithLineNum(), err)
			} else {
				con.log(err)
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

////////////////////////////////////////////////////////////////////////////////
// Private Methods For *gorm.DBCon
////////////////////////////////////////////////////////////////////////////////
//clone - blank param is for copying values and search as well
func (con *DBCon) clone(withoutValues bool, withoutSearch bool) *DBCon {
	clone := DBCon{
		sqli:      con.sqli,
		parent:  con.parent,
		logger:  con.logger,
		logMode: con.logMode,
		values:  map[string]interface{}{},
		Value:   con.Value,
		Error:   con.Error,
	}
	if !withoutValues {
		for key, value := range con.values {
			clone.values[key] = value
		}
	}
	if !withoutSearch {
		if con.search == nil {
			clone.search = &search{limit: -1, offset: -1}
		} else {
			clone.search = con.search.clone()
		}

		clone.search.con = &clone
	}
	return &clone
}

func (con *DBCon) toLog(v ...interface{}) {
	con.logger.(logger).Print(v...)
}

func (con *DBCon) log(v ...interface{}) {
	if con != nil && con.logMode == 2 {
		con.toLog(append([]interface{}{"log", fileWithLineNum()}, v...)...)
	}
}

func (con *DBCon) slog(sql string, t time.Time, vars ...interface{}) {
	if con.logMode == 2 {
		con.toLog("sql", fileWithLineNum(), NowFunc().Sub(t), sql, vars)
	}
}
