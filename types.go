package gorm

import (
	"database/sql"
	"log"
	"os"
	"reflect"
	"regexp"
	"time"
)

const (
	upper strCase = true

	TAG_SQL         string = "sql"
	TAG_GORM        string = "gorm"
	DEFAULT_ID_NAME string = "id"
)

type (
	Uint8Map map[uint8]string
	//since there is no other way of embedding a map
	TagSettings struct {
		Uint8Map
	}
	// StructField model field's struct definition
	//TODO : @Badu - a StructField should support multiple relationships
	StructField struct {
		flags  uint16
		DBName string
		Names  []string

		tagSettings TagSettings

		Struct reflect.StructField
		Value  reflect.Value
		Type   reflect.Type

		Relationship *Relationship
	}

	//easier to read and can apply methods
	StructFields []*StructField

	// Relationship described the relationship between models
	Relationship struct {
		Kind                         uint8
		PolymorphicType              string
		PolymorphicDBName            string
		PolymorphicValue             string
		ForeignFieldNames            StrSlice
		ForeignDBNames               StrSlice
		AssociationForeignFieldNames StrSlice
		AssociationForeignDBNames    StrSlice
		JoinTableHandler             JoinTableHandlerInterface
	}

	strCase bool
	//TODO : @Badu - Association has a field named Error - should be passed to DBCon
	//TODO : @Badu - Association Mode contains some helper methods to handle relationship things easily.
	Association struct {
		Error  error
		scope  *Scope
		column string
		field  *StructField
	}

	// ModelStruct model definition
	ModelStruct struct {
		//keeper of the fields, a safe map (aliases) and slice
		fieldsMap fieldsMap
		//collected from fields.fields, so we won't iterate all the time
		cachedPrimaryFields StructFields
		ModelType           reflect.Type
		defaultTableName    string
	}

	// Scope contain current operation's information when you perform any operation on the database
	Scope struct {
		con *DBCon

		Search *search
		Value  interface{}

		selectAttrs *[]string

		SQL     string
		SQLVars []interface{}

		instanceID string
		//cached version of cloned struct fields
		fields *StructFields
		//skip left remaining callbacks
		skipLeft bool
	}

	sqlConditionType uint16
	sqlCondition     struct {
		Type   sqlConditionType
		Values interface{}
	}
	sqlConditions []sqlCondition
	//TODO : @Badu - pointer to DBCon is just to expose errors
	//since they are related (Scope has a search inside)
	search struct {
		con              *DBCon
		conditions       sqlConditions
		whereConditions  []map[string]interface{}
		orConditions     []map[string]interface{}
		notConditions    []map[string]interface{}
		havingConditions []map[string]interface{}
		joinConditions   []map[string]interface{}
		selects          map[string]interface{}
		initAttrs        []interface{}
		assignAttrs      []interface{}
		orders           []interface{}
		omits            []string
		preload          []searchPreload
		offset           interface{}
		limit            interface{}
		group            string
		tableName        string
		raw              bool
		Unscoped         bool
		countingQuery    bool
	}
	searchPreload struct {
		schema     string
		conditions []interface{}
	}

	// DBCon contains information for current db connection
	DBCon struct {
		parent   *DBCon
		dialect  Dialect
		Value    interface{}
		settings map[string]interface{}

		Error error

		callback *Callback
		sqli     sqlInterf

		search       *search
		RowsAffected int64

		logMode int
		logger  logger

		singularTable bool
		source        string
	}
	//declared to allow existing code to run, dbcon.Open(...) db = &gorm.DB{*dbcon}
	DB struct {
		DBCon
	}

	// Model base model definition, including fields `ID`, `CreatedAt`, `UpdatedAt`, `DeletedAt`, which could be embedded in your models
	//    type User struct {
	//      gorm.Model
	//    }
	Model struct {
		ID        uint `gorm:"primary_key"`
		CreatedAt time.Time
		UpdatedAt time.Time
		DeletedAt *time.Time `sql:"index"`
	}

	//used for callbacks
	ScopedFuncs []*ScopedFunc
	ScopedFunc  func(*Scope)

	//easier to read and can apply methods
	CallbackProcessors []*CallbackProcessor
	// Callback is a struct that contains all CURD callbacks
	//   Field `creates` contains callbacks will be call when creating object
	//   Field `updates` contains callbacks will be call when updating object
	//   Field `deletes` contains callbacks will be call when deleting object
	//   Field `queries` contains callbacks will be call when querying object with query methods like Find, First, Related, Association...
	//   Field `rowQueries` contains callbacks will be call when querying object with Row, Rows...
	//   Field `processors` contains all callback processors, will be used to generate above callbacks in order
	Callback struct {
		creates    ScopedFuncs
		updates    ScopedFuncs
		deletes    ScopedFuncs
		queries    ScopedFuncs
		rowQueries ScopedFuncs
		processors CallbackProcessors
	}

	// CallbackProcessor contains callback informations
	CallbackProcessor struct {
		//remember : we can't remove "name" property, since callbacks gets sorted/re-ordered
		name      string      // current callback's name
		before    string      // register current callback before a callback
		after     string      // register current callback after a callback
		replace   bool        // replace callbacks with same name
		remove    bool        // delete callbacks with same name
		kind      uint8       // callback type: create, update, delete, query, row_query
		processor *ScopedFunc // callback handler
		parent    *Callback
	}

	// DefaultForeignKeyNamer contains the default foreign key name generator method
	DefaultForeignKeyNamer struct {
	}

	commonDialect struct {
		db *sql.DB
		DefaultForeignKeyNamer
	}
	mysql struct {
		commonDialect
	}
	postgres struct {
		commonDialect
	}
	sqlite3 struct {
		commonDialect
	}

	// JoinTableForeignKey join table foreign key struct
	JoinTableForeignKey struct {
		DBName            string
		AssociationDBName string
	}
	// JoinTableSource is a struct that contains model type and foreign keys
	JoinTableSource struct {
		ModelType   reflect.Type
		ForeignKeys []JoinTableForeignKey
	}
	// JoinTableHandler default join table handler
	JoinTableHandler struct {
		TableName   string          `sql:"-"`
		Source      JoinTableSource `sql:"-"`
		Destination JoinTableSource `sql:"-"`
	}

	//interface used for overriding table name
	tabler interface {
		TableName() string
	}
	//interface used for overriding table name
	dbTabler interface {
		TableName(*DBCon) string
	}
	//SQL interface
	sqlInterf interface {
		Exec(query string, args ...interface{}) (sql.Result, error)
		Prepare(query string) (*sql.Stmt, error)
		Query(query string, args ...interface{}) (*sql.Rows, error)
		QueryRow(query string, args ...interface{}) *sql.Row
	}
	//interface
	sqlDb interface {
		Begin() (*sql.Tx, error)
	}
	//interface
	sqlTx interface {
		Commit() error
		Rollback() error
	}
	// JoinTableHandlerInterface is an interface for how to handle many2many relations
	JoinTableHandlerInterface interface {
		// initialize join table handler
		Setup(relationship *Relationship, source reflect.Type, destination reflect.Type)
		// Table return join table's table name
		Table(db *DBCon) string
		// Sets table name
		SetTable(name string)
		// Add create relationship in join table for source and destination
		Add(handler JoinTableHandlerInterface, db *DBCon, source interface{}, destination interface{}) error
		// Delete delete relationship in join table for sources
		Delete(handler JoinTableHandlerInterface, db *DBCon, sources ...interface{}) error
		// JoinWith query with `Join` conditions
		JoinWith(handler JoinTableHandlerInterface, db *DBCon, source interface{}) *DBCon
		// SourceForeignKeys return source foreign keys
		SourceForeignKeys() []JoinTableForeignKey
		// DestinationForeignKeys return destination foreign keys
		DestinationForeignKeys() []JoinTableForeignKey
	}

	// Dialect interface contains behaviors that differ across SQL database
	Dialect interface {
		// GetName get dialect's name
		GetName() string
		// SetDB set db for dialect
		SetDB(db *sql.DB)
		// BindVar return the placeholder for actual values in SQL statements, in many dbs it is "?", Postgres using $1
		BindVar(i int) string
		// Quote quotes field name to avoid SQL parsing exceptions by using a reserved word as a field name
		Quote(key string) string
		// DataTypeOf return data's sql type
		DataTypeOf(field *StructField) string
		// HasIndex check has index or not
		HasIndex(tableName string, indexName string) bool
		// HasForeignKey check has foreign key or not
		HasForeignKey(tableName string, foreignKeyName string) bool
		// RemoveIndex remove index
		RemoveIndex(tableName string, indexName string) error
		// HasTable check has table or not
		HasTable(tableName string) bool
		// HasColumn check has column or not
		HasColumn(tableName string, columnName string) bool
		// LimitAndOffsetSQL return generated SQL with Limit and Offset, as mssql has special case
		LimitAndOffsetSQL(limit, offset interface{}) string
		// SelectFromDummyTable return select values, for most dbs, `SELECT values` just works, mysql needs `SELECT value FROM DUAL`
		SelectFromDummyTable() string
		// LastInsertIdReturningSuffix most dbs support LastInsertId, but postgres needs to use `RETURNING`
		LastInsertIDReturningSuffix(tableName, columnName string) string
		// BuildForeignKeyName returns a foreign key name for the given table, field and reference
		BuildForeignKeyName(tableName, field, dest string) string
		// CurrentDatabase return current database name
		CurrentDatabase() string
	}
)

var (
	dialectsMap = map[string]Dialect{}

	// DefaultTableNameHandler default table name handler
	DefaultTableNameHandler = func(con *DBCon, defaultTableName string) string {
		return defaultTableName
	}

	// NowFunc returns current time, this function is exported in order to be able
	// to give the flexibility to the developer to customize it according to their
	// needs, e.g:
	//    gorm.NowFunc = func() time.Time {
	//      return time.Now().UTC()
	//    }
	NowFunc = func() time.Time {
		return time.Now()
	}

	columnRegexp = regexp.MustCompile("^[a-zA-Z]+(\\.[a-zA-Z]+)*$") // only match string like `name`, `users.name`

	defaultLogger = Logger{log.New(os.Stdout, "\r\n", 0)}

	sqlRegexp = regexp.MustCompile(`(\$\d+)|\?`)
)
