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
func (orm *DB) Close() error {
	return orm.parent.db.(*sql.DB).Close()
}

// DB get `*sql.DB` from current connection
func (orm *DB) DB() *sql.DB {
	return orm.db.(*sql.DB)
}

// Dialect get dialect
func (orm *DB) Dialect() Dialect {
	return orm.parent.dialect
}

// New clone a new db connection without search conditions
func (orm *DB) New() *DB {
	clone := orm.clone()
	clone.search = nil
	clone.Value = nil
	return clone
}

// NewScope create a scope for current operation
func (orm *DB) NewScope(value interface{}) *Scope {
	dbClone := orm.clone()
	dbClone.Value = value
	return &Scope{db: dbClone, Search: dbClone.search.clone(), Value: value}
}

// CommonDB return the underlying `*sql.DB` or `*sql.Tx` instance, mainly intended to allow coexistence with legacy non-GORM code.
func (orm *DB) CommonDB() sqlCommon {
	return orm.db
}

// Callback return `Callbacks` container, you could add/change/delete callbacks with it
//     db.Callback().Create().Register("update_created_at", updateCreated)
// Refer https://jinzhu.github.io/gorm/development.html#callbacks
func (orm *DB) Callback() *Callback {
	orm.parent.callbacks = orm.parent.callbacks.clone()
	return orm.parent.callbacks
}

// SetLogger replace default logger
func (orm *DB) SetLogger(log logger) {
	orm.logger = log
}

// LogMode set log mode, `true` for detailed logs, `false` for no log, default, will only print error logs
func (orm *DB) LogMode(enable bool) *DB {
	if enable {
		orm.logMode = 2
	} else {
		orm.logMode = 1
	}
	return orm
}

// SingularTable use singular table by default
func (orm *DB) SingularTable(enable bool) {
	modelStructsMap = newModelStructsMap()
	orm.parent.singularTable = enable
}

// Where return a new relation, filter records with given conditions, accepts `map`, `struct` or `string` as conditions, refer http://jinzhu.github.io/gorm/curd.html#query
func (orm *DB) Where(query interface{}, args ...interface{}) *DB {
	return orm.clone().search.Where(query, args...).db
}

// Or filter records that match before conditions or this one, similar to `Where`
func (orm *DB) Or(query interface{}, args ...interface{}) *DB {
	return orm.clone().search.Or(query, args...).db
}

// Not filter records that don't match current conditions, similar to `Where`
func (orm *DB) Not(query interface{}, args ...interface{}) *DB {
	return orm.clone().search.Not(query, args...).db
}

// Limit specify the number of records to be retrieved
func (orm *DB) Limit(limit interface{}) *DB {
	return orm.clone().search.Limit(limit).db
}

// Offset specify the number of records to skip before starting to return the records
func (orm *DB) Offset(offset interface{}) *DB {
	return orm.clone().search.Offset(offset).db
}

// Order specify order when retrieve records from database, set reorder to `true` to overwrite defined conditions
//     db.Order("name DESC")
//     db.Order("name DESC", true) // reorder
//     db.Order(gorm.Expr("name = ? DESC", "first")) // sql expression
func (orm *DB) Order(value interface{}, reorder ...bool) *DB {
	return orm.clone().search.Order(value, reorder...).db
}

// Select specify fields that you want to retrieve from database when querying, by default, will select all fields;
// When creating/updating, specify fields that you want to save to database
func (orm *DB) Select(query interface{}, args ...interface{}) *DB {
	return orm.clone().search.Select(query, args...).db
}

// Omit specify fields that you want to ignore when saving to database for creating, updating
func (orm *DB) Omit(columns ...string) *DB {
	return orm.clone().search.Omit(columns...).db
}

// Group specify the group method on the find
func (orm *DB) Group(query string) *DB {
	return orm.clone().search.Group(query).db
}

// Having specify HAVING conditions for GROUP BY
func (orm *DB) Having(query string, values ...interface{}) *DB {
	return orm.clone().search.Having(query, values...).db
}

// Joins specify Joins conditions
//     db.Joins("JOIN emails ON emails.user_id = users.id AND emails.email = ?", "jinzhu@example.org").Find(&user)
func (orm *DB) Joins(query string, args ...interface{}) *DB {
	return orm.clone().search.Joins(query, args...).db
}

// Scopes pass current database connection to arguments `func(*DB) *DB`, which could be used to add conditions dynamically
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
func (orm *DB) Scopes(funcs ...func(*DB) *DB) *DB {
	for _, f := range funcs {
		orm = f(orm)
	}
	return orm
}

// Unscoped return all record including deleted record, refer Soft Delete https://jinzhu.github.io/gorm/curd.html#soft-delete
func (orm *DB) Unscoped() *DB {
	return orm.clone().search.unscoped().db
}

// Attrs initialize struct with argument if record not found with `FirstOrInit` https://jinzhu.github.io/gorm/curd.html#firstorinit or `FirstOrCreate` https://jinzhu.github.io/gorm/curd.html#firstorcreate
func (orm *DB) Attrs(attrs ...interface{}) *DB {
	return orm.clone().search.Attrs(attrs...).db
}

// Assign assign result with argument regardless it is found or not with `FirstOrInit` https://jinzhu.github.io/gorm/curd.html#firstorinit or `FirstOrCreate` https://jinzhu.github.io/gorm/curd.html#firstorcreate
func (orm *DB) Assign(attrs ...interface{}) *DB {
	return orm.clone().search.Assign(attrs...).db
}

// First find first record that match given conditions, order by primary key
func (orm *DB) First(out interface{}, where ...interface{}) *DB {
	newScope := orm.clone().NewScope(out)
	newScope.Search.Limit(1)
	return newScope.Set("gorm:order_by_primary_key", "ASC").
		inlineCondition(where...).callCallbacks(orm.parent.callbacks.queries).db
}

// Last find last record that match given conditions, order by primary key
func (orm *DB) Last(out interface{}, where ...interface{}) *DB {
	newScope := orm.clone().NewScope(out)
	newScope.Search.Limit(1)
	return newScope.Set("gorm:order_by_primary_key", "DESC").
		inlineCondition(where...).callCallbacks(orm.parent.callbacks.queries).db
}

// Find find records that match given conditions
func (orm *DB) Find(out interface{}, where ...interface{}) *DB {
	return orm.clone().NewScope(out).inlineCondition(where...).callCallbacks(orm.parent.callbacks.queries).db
}

// Scan scan value to a struct
func (orm *DB) Scan(dest interface{}) *DB {
	return orm.clone().NewScope(orm.Value).Set("gorm:query_destination", dest).callCallbacks(orm.parent.callbacks.queries).db
}

// Row return `*sql.Row` with given conditions
func (orm *DB) Row() *sql.Row {
	return orm.NewScope(orm.Value).row()
}

// Rows return `*sql.Rows` with given conditions
func (orm *DB) Rows() (*sql.Rows, error) {
	return orm.NewScope(orm.Value).rows()
}

// ScanRows scan `*sql.Rows` to give struct
func (orm *DB) ScanRows(rows *sql.Rows, result interface{}) error {
	var (
		clone        = orm.clone()
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
func (orm *DB) Pluck(column string, value interface{}) *DB {
	return orm.NewScope(orm.Value).pluck(column, value).db
}

// Count get how many records for a model
func (orm *DB) Count(value interface{}) *DB {
	return orm.NewScope(orm.Value).count(value).db
}

// Related get related associations
func (orm *DB) Related(value interface{}, foreignKeys ...string) *DB {
	return orm.clone().NewScope(orm.Value).related(value, foreignKeys...).db
}

// FirstOrInit find first matched record or initialize a new one with given conditions (only works with struct, map conditions)
// https://jinzhu.github.io/gorm/curd.html#firstorinit
func (orm *DB) FirstOrInit(out interface{}, where ...interface{}) *DB {
	c := orm.clone()
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
func (orm *DB) FirstOrCreate(out interface{}, where ...interface{}) *DB {
	c := orm.clone()
	if result := orm.First(out, where...); result.Error != nil {
		if !result.RecordNotFound() {
			return result
		}
		return c.NewScope(out).inlineCondition(where...).initialize().callCallbacks(c.parent.callbacks.creates).db
	} else if len(c.search.assignAttrs) > 0 {
		return c.NewScope(out).InstanceSet("gorm:update_interface", c.search.assignAttrs).callCallbacks(c.parent.callbacks.updates).db
	}
	return c
}

// Update update attributes with callbacks, refer: https://jinzhu.github.io/gorm/curd.html#update
func (orm *DB) Update(attrs ...interface{}) *DB {
	return orm.Updates(toSearchableMap(attrs...), true)
}

// Updates update attributes with callbacks, refer: https://jinzhu.github.io/gorm/curd.html#update
func (orm *DB) Updates(values interface{}, ignoreProtectedAttrs ...bool) *DB {
	return orm.clone().NewScope(orm.Value).
		Set("gorm:ignore_protected_attrs", len(ignoreProtectedAttrs) > 0).
		InstanceSet("gorm:update_interface", values).
		callCallbacks(orm.parent.callbacks.updates).db
}

// UpdateColumn update attributes without callbacks, refer: https://jinzhu.github.io/gorm/curd.html#update
func (orm *DB) UpdateColumn(attrs ...interface{}) *DB {
	return orm.UpdateColumns(toSearchableMap(attrs...))
}

// UpdateColumns update attributes without callbacks, refer: https://jinzhu.github.io/gorm/curd.html#update
func (orm *DB) UpdateColumns(values interface{}) *DB {
	return orm.clone().NewScope(orm.Value).
		Set("gorm:update_column", true).
		Set("gorm:save_associations", false).
		InstanceSet("gorm:update_interface", values).
		callCallbacks(orm.parent.callbacks.updates).db
}

// Save update value in database, if the value doesn't have primary key, will insert it
func (orm *DB) Save(value interface{}) *DB {
	scope := orm.clone().NewScope(value)
	if !scope.PrimaryKeyZero() {
		newDB := scope.callCallbacks(orm.parent.callbacks.updates).db
		if newDB.Error == nil && newDB.RowsAffected == 0 {
			return orm.New().FirstOrCreate(value)
		}
		return newDB
	}
	return scope.callCallbacks(orm.parent.callbacks.creates).db
}

// Create insert the value into database
func (orm *DB) Create(value interface{}) *DB {
	scope := orm.clone().NewScope(value)
	return scope.callCallbacks(orm.parent.callbacks.creates).db
}

// Delete delete value match given conditions, if the value has primary key, then will including the primary key as condition
func (orm *DB) Delete(value interface{}, where ...interface{}) *DB {
	return orm.clone().NewScope(value).inlineCondition(where...).callCallbacks(orm.parent.callbacks.deletes).db
}

// Raw use raw sql as conditions, won't run it unless invoked by other methods
//    db.Raw("SELECT name, age FROM users WHERE name = ?", 3).Scan(&result)
func (orm *DB) Raw(sql string, values ...interface{}) *DB {
	return orm.clone().search.Raw(true).Where(sql, values...).db
}

// Exec execute raw sql
func (orm *DB) Exec(sql string, values ...interface{}) *DB {
	scope := orm.clone().NewScope(nil)
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
func (orm *DB) Model(value interface{}) *DB {
	c := orm.clone()
	c.Value = value
	return c
}

// Table specify the table you would like to run db operations
func (orm *DB) Table(name string) *DB {
	clone := orm.clone()
	clone.search.Table(name)
	clone.Value = nil
	return clone
}

// Debug start debug mode
func (orm *DB) Debug() *DB {
	return orm.clone().LogMode(true)
}

// Begin begin a transaction
func (orm *DB) Begin() *DB {
	c := orm.clone()
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
func (orm *DB) Commit() *DB {
	if db, ok := orm.db.(sqlTx); ok {
		orm.AddError(db.Commit())
	} else {
		orm.AddError(ErrInvalidTransaction)
	}
	return orm
}

// Rollback rollback a transaction
func (orm *DB) Rollback() *DB {
	if db, ok := orm.db.(sqlTx); ok {
		orm.AddError(db.Rollback())
	} else {
		orm.AddError(ErrInvalidTransaction)
	}
	return orm
}

// NewRecord check if value's primary key is blank
func (orm *DB) NewRecord(value interface{}) bool {
	return orm.clone().NewScope(value).PrimaryKeyZero()
}

// RecordNotFound check if returning ErrRecordNotFound error
func (orm *DB) RecordNotFound() bool {
	for _, err := range orm.GetErrors() {
		if err == ErrRecordNotFound {
			return true
		}
	}
	return false
}

// CreateTable create table for models
func (orm *DB) CreateTable(models ...interface{}) *DB {
	db := orm.Unscoped()
	for _, model := range models {
		db = db.NewScope(model).createTable().db
	}
	return db
}

// DropTable drop table for models
func (orm *DB) DropTable(values ...interface{}) *DB {
	db := orm.clone()
	for _, value := range values {
		if tableName, ok := value.(string); ok {
			db = db.Table(tableName)
		}

		db = db.NewScope(value).dropTable().db
	}
	return db
}

// DropTableIfExists drop table if it is exist
func (orm *DB) DropTableIfExists(values ...interface{}) *DB {
	db := orm.clone()
	for _, value := range values {
		if orm.HasTable(value) {
			db.AddError(orm.DropTable(value).Error)
		}
	}
	return db
}

// HasTable check has table or not
func (orm *DB) HasTable(value interface{}) bool {
	var (
		scope     = orm.clone().NewScope(value)
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
func (orm *DB) AutoMigrate(values ...interface{}) *DB {
	db := orm.Unscoped()
	for _, value := range values {
		db = db.NewScope(value).autoMigrate().db
	}
	return db
}

// ModifyColumn modify column to type
func (orm *DB) ModifyColumn(column string, typ string) *DB {
	scope := orm.clone().NewScope(orm.Value)
	scope.modifyColumn(column, typ)
	return scope.db
}

// DropColumn drop a column
func (orm *DB) DropColumn(column string) *DB {
	scope := orm.clone().NewScope(orm.Value)
	scope.dropColumn(column)
	return scope.db
}

// AddIndex add index for columns with given name
func (orm *DB) AddIndex(indexName string, columns ...string) *DB {
	scope := orm.Unscoped().NewScope(orm.Value)
	scope.addIndex(false, indexName, columns...)
	return scope.db
}

// AddUniqueIndex add unique index for columns with given name
func (orm *DB) AddUniqueIndex(indexName string, columns ...string) *DB {
	scope := orm.Unscoped().NewScope(orm.Value)
	scope.addIndex(true, indexName, columns...)
	return scope.db
}

// RemoveIndex remove index with name
func (orm *DB) RemoveIndex(indexName string) *DB {
	scope := orm.clone().NewScope(orm.Value)
	scope.removeIndex(indexName)
	return scope.db
}

// AddForeignKey Add foreign key to the given scope, e.g:
//     db.Model(&User{}).AddForeignKey("city_id", "cities(id)", "RESTRICT", "RESTRICT")
func (orm *DB) AddForeignKey(field string, dest string, onDelete string, onUpdate string) *DB {
	scope := orm.clone().NewScope(orm.Value)
	scope.addForeignKey(field, dest, onDelete, onUpdate)
	return scope.db
}

// Association start `Association Mode` to handler relations things easir in that mode, refer: https://jinzhu.github.io/gorm/associations.html#association-mode
func (orm *DB) Association(column string) *Association {
	var err error
	scope := orm.clone().NewScope(orm.Value)

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
func (orm *DB) Preload(column string, conditions ...interface{}) *DB {
	return orm.clone().search.Preload(column, conditions...).db
}

// Set set setting by name, which could be used in callbacks, will clone a new db, and update its setting
func (orm *DB) Set(name string, value interface{}) *DB {
	return orm.clone().InstantSet(name, value)
}

// InstantSet instant set setting, will affect current db
func (orm *DB) InstantSet(name string, value interface{}) *DB {
	orm.values[name] = value
	return orm
}

// Get get setting by name
func (orm *DB) Get(name string) (value interface{}, ok bool) {
	value, ok = orm.values[name]
	return
}

// SetJoinTableHandler set a model's join table handler for a relation
func (orm *DB) SetJoinTableHandler(source interface{}, column string, handler JoinTableHandlerInterface) {
	scope := orm.NewScope(source)
	for _, field := range scope.GetModelStruct().StructFields {
		if field.GetName() == column || field.DBName == column {
			if many2many := field.TagSettings[MANY2MANY]; many2many != "" {
				source := (&Scope{Value: source}).GetModelStruct().ModelType
				destination := (&Scope{Value: reflect.New(field.Struct.Type).Interface()}).GetModelStruct().ModelType
				handler.Setup(field.Relationship, many2many, source, destination)
				field.Relationship.JoinTableHandler = handler
				if table := handler.Table(orm); scope.Dialect().HasTable(table) {
					orm.Table(table).AutoMigrate(handler)
				}
			}
		}
	}
}

// AddError add error to the db
func (orm *DB) AddError(err error) error {
	if err != nil {
		if err != ErrRecordNotFound {
			if orm.logMode == 0 {
				go orm.print(fileWithLineNum(), err)
			} else {
				orm.log(err)
			}

			errors := Errors{errors: orm.GetErrors()}
			errors.Add(err)
			if len(errors.GetErrors()) > 1 {
				err = errors
			}
		}

		orm.Error = err
	}
	return err
}

// GetErrors get happened errors from the db
func (orm *DB) GetErrors() (errors []error) {
	if errs, ok := orm.Error.(errorsInterface); ok {
		return errs.GetErrors()
	} else if orm.Error != nil {
		return []error{orm.Error}
	}
	return
}

////////////////////////////////////////////////////////////////////////////////
// Private Methods For *gorm.DB
////////////////////////////////////////////////////////////////////////////////
func (orm *DB) clone() *DB {
	db := DB{db: orm.db, parent: orm.parent, logger: orm.logger, logMode: orm.logMode, values: map[string]interface{}{}, Value: orm.Value, Error: orm.Error}

	for key, value := range orm.values {
		db.values[key] = value
	}

	if orm.search == nil {
		db.search = &search{limit: -1, offset: -1}
	} else {
		db.search = orm.search.clone()
	}

	db.search.db = &db
	return &db
}

func (orm *DB) print(v ...interface{}) {
	orm.logger.(logger).Print(v...)
}

func (orm *DB) log(v ...interface{}) {
	if orm != nil && orm.logMode == 2 {
		orm.print(append([]interface{}{"log", fileWithLineNum()}, v...)...)
	}
}

func (orm *DB) slog(sql string, t time.Time, vars ...interface{}) {
	if orm.logMode == 2 {
		orm.print("sql", fileWithLineNum(), NowFunc().Sub(t), sql, vars)
	}
}
