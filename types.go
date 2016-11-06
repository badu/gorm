package gorm

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"reflect"
	"regexp"
	"time"
)

const (
	upper strCase = true
	//Relationship Kind constants
	MANY_TO_MANY uint8 = 1
	HAS_MANY     uint8 = 2
	HAS_ONE      uint8 = 3
	BELONGS_TO   uint8 = 4
	//Callback Kind constants
	CREATE_CALLBACK    uint8 = 1
	UPDATE_CALLBACK    uint8 = 2
	DELETE_CALLBACK    uint8 = 3
	QUERY_CALLBACK     uint8 = 4
	ROW_QUERY_CALLBACK uint8 = 5
)

type (
	Uint8Map map[uint8]string
	//since there is no other way of embedding a map
	TagSettings struct {
		Uint8Map
	}
	// StructField model field's struct definition
	//TODO : @Badu - instead of having this bunch of flags - a bitflag seems better
	//TODO : @Badu - a StructField should support multiple relationships
	//TODO : @Badu - do NOT attempt to make pointer for Struct property
	//TODO : @Badu - maybe TagSettings property should be private and have access with a method
	//since clone strategy is based exactly on that (that you get a copy of
	//Struct property instead of the pointer to the same value)
	StructField struct {
		IsPrimaryKey    bool
		IsNormal        bool
		IsIgnored       bool
		IsScanner       bool
		HasDefaultValue bool
		IsForeignKey    bool
		IsBlank         bool

		DBName string
		Names  []string

		tagSettings TagSettings

		Struct reflect.StructField
		Value  reflect.Value

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
	// Association Mode contains some helper methods to handle relationship things easily.
	Association struct {
		Error  error
		scope  *Scope
		column string
		field  *StructField
	}
	//TODO : @Badu - if StructField has IsPrimaryKey field, why having two sets of []*StructField
	//unify them into something called Fields which StructFields type would provide via method
	// ModelStruct model definition
	ModelStruct struct {
		PrimaryFields    StructFields
		StructFields     StructFields
		ModelType        reflect.Type
		defaultTableName string
	}

	// SQL expression
	expr struct {
		expr string
		args []interface{}
	}

	// DBCon contains information for current db connection
	DBCon struct {
		parent  *DBCon
		dialect Dialect
		Value   interface{}
		values  map[string]interface{}

		Error error

		callback *Callback
		db       sqlCommon

		search       *search
		RowsAffected int64

		logMode int
		logger  logger

		singularTable bool
		source        string

		joinTableHandlers map[string]JoinTableHandler
	}

	// Scope contain current operation's information when you perform any operation on the database
	Scope struct {
		db *DBCon

		Search *search
		Value  interface{}

		selectAttrs *[]string

		SQL     string
		SQLVars []interface{}

		instanceID string

		primaryKeyField *StructField
		fields          *StructFields
		//skip left remaining callbacks
		skipLeft bool
	}

	//TODO : @Badu - find out why both Scope and search structs hold pointer to DBCon,
	//since they are related (Scope has a search inside)
	search struct {
		db               *DBCon
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

	//interface used for overriding table name
	tabler interface {
		TableName() string
	}
	//interface used for overriding table name
	dbTabler interface {
		TableName(*DBCon) string
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
	sqlCommon interface {
		Exec(query string, args ...interface{}) (sql.Result, error)
		Prepare(query string) (*sql.Stmt, error)
		Query(query string, args ...interface{}) (*sql.Rows, error)
		QueryRow(query string, args ...interface{}) *sql.Row
	}
	sqlDb interface {
		Begin() (*sql.Tx, error)
	}
	sqlTx interface {
		Commit() error
		Rollback() error
	}

	// JoinTableHandlerInterface is an interface for how to handle many2many relations
	JoinTableHandlerInterface interface {
		// initialize join table handler
		Setup(relationship *Relationship, tableName string, source reflect.Type, destination reflect.Type)
		// Table return join table's table name
		Table(db *DBCon) string
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
)

var (
	dialectsMap = map[string]Dialect{}

	// DefaultTableNameHandler default table name handler
	DefaultTableNameHandler = func(db *DBCon, defaultTableName string) string {
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

	// ErrRecordNotFound record not found error, happens when haven't find any matched data when looking up with a struct
	ErrRecordNotFound = errors.New("record not found")

	// ErrInvalidSQL invalid SQL error, happens when you passed invalid SQL
	ErrInvalidSQL = errors.New("invalid SQL")

	// ErrInvalidTransaction invalid transaction when you are trying to `Commit` or `Rollback`
	ErrInvalidTransaction = errors.New("no valid transaction")

	// ErrCantStartTransaction can't start transaction when you are trying to start one with `Begin`
	ErrCantStartTransaction = errors.New("can't start transaction")

	// ErrUnaddressable unaddressable value
	ErrUnaddressable = errors.New("using unaddressable value")
)
