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
func (orm *DBCon) Close() error {
	return orm.parent.db.(*sql.DB).Close()
}

// DB get `*sql.DB` from current connection
func (orm *DBCon) DB() *sql.DB {
	return orm.db.(*sql.DB)
}

// Dialect get dialect
func (orm *DBCon) Dialect() Dialect {
	return orm.parent.dialect
}

// New clone a new db connection without search conditions
func (orm *DBCon) New() *DBCon {
	clone := orm.clone(true)
	return clone
}

// NewScope create a scope for current operation
func (orm *DBCon) NewScope(value interface{}) *Scope {
	dbClone := orm.clone(false)
	dbClone.Value = value
	return &Scope{db: dbClone, Search: dbClone.search.clone(), Value: value}
}

// CommonDB return the underlying `*sql.DB` or `*sql.Tx` instance, mainly intended to allow coexistence with legacy non-GORM code.
func (orm *DBCon) CommonDB() sqlCommon {
	return orm.db
}

// Callback return `Callbacks` container, you could add/change/delete callbacks with it
//     db.Callback().Create().Register("update_created_at", updateCreated)
// Refer https://jinzhu.github.io/gorm/development.html#callbacks
func (orm *DBCon) Callback() *Callback {
	orm.parent.callback = orm.parent.callback.clone()
	return orm.parent.callback
}

// SetLogger replace default logger
func (orm *DBCon) SetLogger(log logger) {
	orm.logger = log
}

// LogMode set log mode, `true` for detailed logs, `false` for no log, default, will only print error logs
func (orm *DBCon) LogMode(enable bool) *DBCon {
	if enable {
		orm.logMode = 2
	} else {
		orm.logMode = 1
	}
	return orm
}

// SingularTable use singular table by default
func (orm *DBCon) SingularTable(enable bool) {
	modelStructsMap = newModelStructsMap()
	orm.parent.singularTable = enable
}

// Where return a new relation, filter records with given conditions, accepts `map`, `struct` or `string` as conditions, refer http://jinzhu.github.io/gorm/curd.html#query
func (orm *DBCon) Where(query interface{}, args ...interface{}) *DBCon {
	return orm.clone(false).search.Where(query, args...).db
}

// Or filter records that match before conditions or this one, similar to `Where`
func (orm *DBCon) Or(query interface{}, args ...interface{}) *DBCon {
	return orm.clone(false).search.Or(query, args...).db
}

// Not filter records that don't match current conditions, similar to `Where`
func (orm *DBCon) Not(query interface{}, args ...interface{}) *DBCon {
	return orm.clone(false).search.Not(query, args...).db
}

// Limit specify the number of records to be retrieved
func (orm *DBCon) Limit(limit interface{}) *DBCon {
	return orm.clone(false).search.Limit(limit).db
}

// Offset specify the number of records to skip before starting to return the records
func (orm *DBCon) Offset(offset interface{}) *DBCon {
	return orm.clone(false).search.Offset(offset).db
}

// Order specify order when retrieve records from database, set reorder to `true` to overwrite defined conditions
//     db.Order("name DESC")
//     db.Order("name DESC", true) // reorder
//     db.Order(gorm.Expr("name = ? DESC", "first")) // sql expression
func (orm *DBCon) Order(value interface{}, reorder ...bool) *DBCon {
	return orm.clone(false).search.Order(value, reorder...).db
}

// Select specify fields that you want to retrieve from database when querying, by default, will select all fields;
// When creating/updating, specify fields that you want to save to database
func (orm *DBCon) Select(query interface{}, args ...interface{}) *DBCon {
	return orm.clone(false).search.Select(query, args...).db
}

// Omit specify fields that you want to ignore when saving to database for creating, updating
func (orm *DBCon) Omit(columns ...string) *DBCon {
	return orm.clone(false).search.Omit(columns...).db
}

// Group specify the group method on the find
func (orm *DBCon) Group(query string) *DBCon {
	return orm.clone(false).search.Group(query).db
}

// Having specify HAVING conditions for GROUP BY
func (orm *DBCon) Having(query string, values ...interface{}) *DBCon {
	return orm.clone(false).search.Having(query, values...).db
}

// Joins specify Joins conditions
//     db.Joins("JOIN emails ON emails.user_id = users.id AND emails.email = ?", "jinzhu@example.org").Find(&user)
func (orm *DBCon) Joins(query string, args ...interface{}) *DBCon {
	return orm.clone(false).search.Joins(query, args...).db
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
// Refer https://jinzhu.github.io/gorm/curd.html#scopes
//TODO : Badu - replace this - it's soooo ugly
func (orm *DBCon) Scopes(funcs ...func(*DBCon) *DBCon) *DBCon {
	for _, f := range funcs {
		//TODO : @Badu - assignment to method receiver propagates only to callees but not to callers
		orm = f(orm)
	}
	return orm
}

// Unscoped return all record including deleted record, refer Soft Delete https://jinzhu.github.io/gorm/curd.html#soft-delete
func (orm *DBCon) Unscoped() *DBCon {
	return orm.clone(false).search.unscoped().db
}

// Attrs initialize struct with argument if record not found with `FirstOrInit` https://jinzhu.github.io/gorm/curd.html#firstorinit or `FirstOrCreate` https://jinzhu.github.io/gorm/curd.html#firstorcreate
func (orm *DBCon) Attrs(attrs ...interface{}) *DBCon {
	return orm.clone(false).search.Attrs(attrs...).db
}

// Assign assign result with argument regardless it is found or not with `FirstOrInit` https://jinzhu.github.io/gorm/curd.html#firstorinit or `FirstOrCreate` https://jinzhu.github.io/gorm/curd.html#firstorcreate
func (orm *DBCon) Assign(attrs ...interface{}) *DBCon {
	return orm.clone(false).search.Assign(attrs...).db
}

// First find first record that match given conditions, order by primary key
func (orm *DBCon) First(out interface{}, where ...interface{}) *DBCon {
	newScope := orm.clone(false).NewScope(out)
	newScope.Search.Limit(1)
	return newScope.Set("gorm:order_by_primary_key", "ASC").
		inlineCondition(where...).callCallbacks(orm.parent.callback.queries).db
}

// Last find last record that match given conditions, order by primary key
func (orm *DBCon) Last(out interface{}, where ...interface{}) *DBCon {
	newScope := orm.clone(false).NewScope(out)
	newScope.Search.Limit(1)
	return newScope.Set("gorm:order_by_primary_key", "DESC").
		inlineCondition(where...).callCallbacks(orm.parent.callback.queries).db
}

// Find find records that match given conditions
func (orm *DBCon) Find(out interface{}, where ...interface{}) *DBCon {
	return orm.clone(false).NewScope(out).inlineCondition(where...).callCallbacks(orm.parent.callback.queries).db
}

// Scan scan value to a struct
func (orm *DBCon) Scan(dest interface{}) *DBCon {
	return orm.clone(false).NewScope(orm.Value).Set("gorm:query_destination", dest).callCallbacks(orm.parent.callback.queries).db
}

// Row return `*sql.Row` with given conditions
func (orm *DBCon) Row() *sql.Row {
	return orm.NewScope(orm.Value).row()
}

// Rows return `*sql.Rows` with given conditions
func (orm *DBCon) Rows() (*sql.Rows, error) {
	return orm.NewScope(orm.Value).rows()
}

// ScanRows scan `*sql.Rows` to give struct
func (orm *DBCon) ScanRows(rows *sql.Rows, result interface{}) error {
	var (
		clone        = orm.clone(false)
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
func (orm *DBCon) Pluck(column string, value interface{}) *DBCon {
	return orm.NewScope(orm.Value).pluck(column, value).db
}

// Count get how many records for a model
func (orm *DBCon) Count(value interface{}) *DBCon {
	return orm.NewScope(orm.Value).count(value).db
}

// Related get related associations
func (orm *DBCon) Related(value interface{}, foreignKeys ...string) *DBCon {
	return orm.clone(false).NewScope(orm.Value).related(value, foreignKeys...).db
}

// FirstOrInit find first matched record or initialize a new one with given conditions (only works with struct, map conditions)
// https://jinzhu.github.io/gorm/curd.html#firstorinit
func (orm *DBCon) FirstOrInit(out interface{}, where ...interface{}) *DBCon {
	c := orm.clone(false)
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
// https://jinzhu.github.io/gorm/curd.html#firstorcreate
func (orm *DBCon) FirstOrCreate(out interface{}, where ...interface{}) *DBCon {
	c := orm.clone(false)
	if result := orm.First(out, where...); result.Error != nil {
		if !result.RecordNotFound() {
			return result
		}
		return c.NewScope(out).inlineCondition(where...).initialize().callCallbacks(c.parent.callback.creates).db
	} else if len(c.search.assignAttrs) > 0 {
		return c.NewScope(out).InstanceSet("gorm:update_interface", c.search.assignAttrs).callCallbacks(c.parent.callback.updates).db
	}
	return c
}

// Update update attributes with callbacks, refer: https://jinzhu.github.io/gorm/curd.html#update
func (orm *DBCon) Update(attrs ...interface{}) *DBCon {
	return orm.Updates(toSearchableMap(attrs...), true)
}

// Updates update attributes with callbacks, refer: https://jinzhu.github.io/gorm/curd.html#update
func (orm *DBCon) Updates(values interface{}, ignoreProtectedAttrs ...bool) *DBCon {
	return orm.clone(false).NewScope(orm.Value).
		Set("gorm:ignore_protected_attrs", len(ignoreProtectedAttrs) > 0).
		InstanceSet("gorm:update_interface", values).
		callCallbacks(orm.parent.callback.updates).db
}

// UpdateColumn update attributes without callbacks, refer: https://jinzhu.github.io/gorm/curd.html#update
func (orm *DBCon) UpdateColumn(attrs ...interface{}) *DBCon {
	return orm.UpdateColumns(toSearchableMap(attrs...))
}

// UpdateColumns update attributes without callbacks, refer: https://jinzhu.github.io/gorm/curd.html#update
func (orm *DBCon) UpdateColumns(values interface{}) *DBCon {
	return orm.clone(false).NewScope(orm.Value).
		Set("gorm:update_column", true).
		Set("gorm:save_associations", false).
		InstanceSet("gorm:update_interface", values).
		callCallbacks(orm.parent.callback.updates).db
}

// Save update value in database, if the value doesn't have primary key, will insert it
func (orm *DBCon) Save(value interface{}) *DBCon {
	scope := orm.clone(false).NewScope(value)
	if !scope.PrimaryKeyZero() {
		newDB := scope.callCallbacks(orm.parent.callback.updates).db
		if newDB.Error == nil && newDB.RowsAffected == 0 {
			return orm.New().FirstOrCreate(value)
		}
		return newDB
	}
	return scope.callCallbacks(orm.parent.callback.creates).db
}

// Create insert the value into database
func (orm *DBCon) Create(value interface{}) *DBCon {
	scope := orm.clone(false).NewScope(value)
	return scope.callCallbacks(orm.parent.callback.creates).db
}

// Delete delete value match given conditions, if the value has primary key, then will including the primary key as condition
func (orm *DBCon) Delete(value interface{}, where ...interface{}) *DBCon {
	return orm.clone(false).NewScope(value).inlineCondition(where...).callCallbacks(orm.parent.callback.deletes).db
}

// Raw use raw sql as conditions, won't run it unless invoked by other methods
//    db.Raw("SELECT name, age FROM users WHERE name = ?", 3).Scan(&result)
func (orm *DBCon) Raw(sql string, values ...interface{}) *DBCon {
	return orm.clone(false).search.Raw(true).Where(sql, values...).db
}

// Exec execute raw sql
func (orm *DBCon) Exec(sql string, values ...interface{}) *DBCon {
	scope := orm.clone(false).NewScope(nil)
	generatedSQL := scope.buildWhereCondition(map[string]interface{}{"query": sql, "args": values})
	generatedSQL = strings.TrimSuffix(strings.TrimPrefix(generatedSQL, "("), ")")
	scope.Raw(generatedSQL)
	return scope.Exec().db
}

// Model specify the model you would like to run db operations
//    // update all users's name to `hello`
//    db.Model(&User{}).Update("name", "hello")
//    // if user's primary key is non-blank, will use it as condition, then will only update the user's name to `hello`
//    db.Model(&user).Update("name", "hello")
func (orm *DBCon) Model(value interface{}) *DBCon {
	c := orm.clone(false)
	c.Value = value
	return c
}

// Table specify the table you would like to run db operations
func (orm *DBCon) Table(name string) *DBCon {
	clone := orm.clone(false)
	clone.search.Table(name)
	clone.Value = nil
	return clone
}

// Debug start debug mode
func (orm *DBCon) Debug() *DBCon {
	return orm.clone(false).LogMode(true)
}

// Begin begin a transaction
func (orm *DBCon) Begin() *DBCon {
	c := orm.clone(false)
	if db, ok := c.db.(sqlDb); ok {
		tx, err := db.Begin()
		c.db = interface{}(tx).(sqlCommon)
		c.AddError(err)
	} else {
		c.AddError(ErrCantStartTransaction)
	}
	return c
}

// Commit commit a transaction
func (orm *DBCon) Commit() *DBCon {
	if db, ok := orm.db.(sqlTx); ok {
		orm.AddError(db.Commit())
	} else {
		orm.AddError(ErrInvalidTransaction)
	}
	return orm
}

// Rollback rollback a transaction
func (orm *DBCon) Rollback() *DBCon {
	if db, ok := orm.db.(sqlTx); ok {
		orm.AddError(db.Rollback())
	} else {
		orm.AddError(ErrInvalidTransaction)
	}
	return orm
}

// NewRecord check if value's primary key is blank
func (orm *DBCon) NewRecord(value interface{}) bool {
	return orm.clone(false).NewScope(value).PrimaryKeyZero()
}

// RecordNotFound check if returning ErrRecordNotFound error
func (orm *DBCon) RecordNotFound() bool {
	for _, err := range orm.GetErrors() {
		if err == ErrRecordNotFound {
			return true
		}
	}
	return false
}

// CreateTable create table for models
func (orm *DBCon) CreateTable(models ...interface{}) *DBCon {
	db := orm.Unscoped()
	for _, model := range models {
		db = db.NewScope(model).createTable().db
	}
	return db
}

// DropTable drop table for models
func (orm *DBCon) DropTable(values ...interface{}) *DBCon {
	db := orm.clone(false)
	for _, value := range values {
		if tableName, ok := value.(string); ok {
			db = db.Table(tableName)
		}

		db = db.NewScope(value).dropTable().db
	}
	return db
}

// DropTableIfExists drop table if it is exist
func (orm *DBCon) DropTableIfExists(values ...interface{}) *DBCon {
	db := orm.clone(false)
	for _, value := range values {
		if orm.HasTable(value) {
			db.AddError(orm.DropTable(value).Error)
		}
	}
	return db
}

// HasTable check has table or not
func (orm *DBCon) HasTable(value interface{}) bool {
	var (
		scope     = orm.clone(false).NewScope(value)
		tableName string
	)

	if name, ok := value.(string); ok {
		tableName = name
	} else {
		tableName = scope.TableName()
	}

	has := scope.Dialect().HasTable(tableName)
	orm.AddError(scope.db.Error)
	return has
}

// AutoMigrate run auto migration for given models, will only add missing fields, won't delete/change current data
func (orm *DBCon) AutoMigrate(values ...interface{}) *DBCon {
	db := orm.Unscoped()
	for _, value := range values {
		db = db.NewScope(value).autoMigrate().db
	}
	return db
}

// ModifyColumn modify column to type
func (orm *DBCon) ModifyColumn(column string, typ string) *DBCon {
	scope := orm.clone(false).NewScope(orm.Value)
	scope.modifyColumn(column, typ)
	return scope.db
}

// DropColumn drop a column
func (orm *DBCon) DropColumn(column string) *DBCon {
	scope := orm.clone(false).NewScope(orm.Value)
	scope.dropColumn(column)
	return scope.db
}

// AddIndex add index for columns with given name
func (orm *DBCon) AddIndex(indexName string, columns ...string) *DBCon {
	scope := orm.Unscoped().NewScope(orm.Value)
	scope.addIndex(false, indexName, columns...)
	return scope.db
}

// AddUniqueIndex add unique index for columns with given name
func (orm *DBCon) AddUniqueIndex(indexName string, columns ...string) *DBCon {
	scope := orm.Unscoped().NewScope(orm.Value)
	scope.addIndex(true, indexName, columns...)
	return scope.db
}

// RemoveIndex remove index with name
func (orm *DBCon) RemoveIndex(indexName string) *DBCon {
	scope := orm.clone(false).NewScope(orm.Value)
	scope.removeIndex(indexName)
	return scope.db
}

// AddForeignKey Add foreign key to the given scope, e.g:
//     db.Model(&User{}).AddForeignKey("city_id", "cities(id)", "RESTRICT", "RESTRICT")
func (orm *DBCon) AddForeignKey(field string, dest string, onDelete string, onUpdate string) *DBCon {
	scope := orm.clone(false).NewScope(orm.Value)
	scope.addForeignKey(field, dest, onDelete, onUpdate)
	return scope.db
}

// Association start `Association Mode` to handler relations things easir in that mode, refer: https://jinzhu.github.io/gorm/associations.html#association-mode
func (orm *DBCon) Association(column string) *Association {
	var err error
	scope := orm.clone(false).NewScope(orm.Value)

	if primaryField := scope.PrimaryField(); primaryField.IsBlank {
		err = errors.New("primary key can't be nil")
	} else {
		if field, ok := scope.FieldByName(column); ok {
			if field.Relationship == nil || len(field.Relationship.ForeignFieldNames) == 0 {
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
func (orm *DBCon) Preload(column string, conditions ...interface{}) *DBCon {
	return orm.clone(false).search.Preload(column, conditions...).db
}

// Set set setting by name, which could be used in callbacks, will clone a new db, and update its setting
func (orm *DBCon) Set(name string, value interface{}) *DBCon {
	return orm.clone(false).InstantSet(name, value)
}

// InstantSet instant set setting, will affect current db
func (orm *DBCon) InstantSet(name string, value interface{}) *DBCon {
	orm.values[name] = value
	return orm
}

// Get get setting by name
func (orm *DBCon) Get(name string) (value interface{}, ok bool) {
	value, ok = orm.values[name]
	return
}

// SetJoinTableHandler set a model's join table handler for a relation
func (orm *DBCon) SetJoinTableHandler(source interface{}, column string, handler JoinTableHandlerInterface) {
	scope := orm.NewScope(source)
	for _, field := range scope.GetModelStruct().StructFields {
		if field.GetName() == column || field.DBName == column {
			if many2many := field.GetSetting(MANY2MANY); many2many != "" {
				src := (&Scope{Value: source}).GetModelStruct().ModelType
				destination := (&Scope{Value: reflect.New(field.Struct.Type).Interface()}).GetModelStruct().ModelType
				handler.Setup(field.Relationship, many2many, src, destination)
				field.Relationship.JoinTableHandler = handler
				if table := handler.Table(orm); scope.Dialect().HasTable(table) {
					orm.Table(table).AutoMigrate(handler)
				}
			}
		}
	}
}

// AddError add error to the db
func (orm *DBCon) AddError(err error) error {
	if err != nil {
		if err != ErrRecordNotFound {
			if orm.logMode == 0 {
				go orm.toLog(fileWithLineNum(), err)
			} else {
				orm.log(err)
			}
			gormErrors := GormErrors{errors: orm.GetErrors()}
			gormErrors.Add(err)
			if len(gormErrors.GetErrors()) > 1 {
				err = gormErrors
			}
		}

		orm.Error = err
	}
	return err
}

// GetErrors get happened errors from the db
func (orm *DBCon) GetErrors() []error {
	if errs, ok := orm.Error.(errorsInterface); ok {
		return errs.GetErrors()
	} else if orm.Error != nil {
		return []error{orm.Error}
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// Private Methods For *gorm.DBCon
////////////////////////////////////////////////////////////////////////////////
//clone - blank param is for copying values and search as well
func (orm *DBCon) clone(blank bool) *DBCon {
	db := DBCon{
		db:      orm.db,
		parent:  orm.parent,
		logger:  orm.logger,
		logMode: orm.logMode,
		values:  map[string]interface{}{},
		Value:   orm.Value,
		Error:   orm.Error,
	}
	if !blank {
		for key, value := range orm.values {
			db.values[key] = value
		}

		if orm.search == nil {
			db.search = &search{limit: -1, offset: -1}
		} else {
			db.search = orm.search.clone()
		}

		db.search.db = &db
	}
	return &db
}

func (orm *DBCon) toLog(v ...interface{}) {
	orm.logger.(logger).Print(v...)
}

func (orm *DBCon) log(v ...interface{}) {
	if orm != nil && orm.logMode == 2 {
		orm.toLog(append([]interface{}{"log", fileWithLineNum()}, v...)...)
	}
}

func (orm *DBCon) slog(sql string, t time.Time, vars ...interface{}) {
	if orm.logMode == 2 {
		orm.toLog("sql", fileWithLineNum(), NowFunc().Sub(t), sql, vars)
	}
}
