package gorm

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	lower strCase = false
	upper strCase = true
	//Relationship Kind constants
	MANY_TO_MANY uint8 = 1
	HAS_MANY     uint8 = 2
	HAS_ONE      uint8 = 3
	BELONGS_TO   uint8 = 4
	//StructField TagSettings constants
	MANY2MANY               uint8 = 1
	AUTO_INCREMENT          uint8 = 2
	INDEX                   uint8 = 3
	NOT_NULL                uint8 = 4
	SIZE                    uint8 = 5
	UNIQUE_INDEX            uint8 = 6
	IS_JOINTABLE_FOREIGNKEY uint8 = 7
	PRIMARY_KEY             uint8 = 8
	DEFAULT                 uint8 = 9
	IGNORED                 uint8 = 10
	EMBEDDED                uint8 = 11
	EMBEDDED_PREFIX         uint8 = 12
	FOREIGNKEY              uint8 = 13
	ASSOCIATIONFOREIGNKEY   uint8 = 14
	POLYMORPHIC             uint8 = 15
	POLYMORPHIC_VALUE       uint8 = 16
	COLUMN                  uint8 = 17
	TYPE                    uint8 = 18
	UNIQUE                  uint8 = 19
)

type (
	/**
	reflect.StructField{
		// Name is the field name.
		Name string
		// PkgPath is the package path that qualifies a lower case (unexported)
		// field name. It is empty for upper case (exported) field names.
		// See https://golang.org/ref/spec#Uniqueness_of_identifiers
		PkgPath string

		Type      Type      // field type
		Tag       StructTag // field tag string
		Offset    uintptr   // offset within struct, in bytes
		Index     []int     // index sequence for Type.FieldByIndex
		Anonymous bool      // is an embedded field
	}
	*/
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

		TagSettings map[uint8]string

		Struct reflect.StructField
		Value  reflect.Value

		Relationship *Relationship
	}

	//For code readability
	StructFields []*StructField

	// Relationship described the relationship between models
	Relationship struct {
		Kind                         uint8
		PolymorphicType              string
		PolymorphicDBName            string
		PolymorphicValue             string
		ForeignFieldNames            []string
		ForeignDBNames               []string
		AssociationForeignFieldNames []string
		AssociationForeignDBNames    []string
		JoinTableHandler             JoinTableHandlerInterface
	}

	strCase bool
	// Association Mode contains some helper methods to handle relationship things easily.
	Association struct {
		Error  error
		scope  *Scope
		column string
		field  *StructField
	}

	// ModelStruct model definition
	ModelStruct struct {
		PrimaryFields    StructFields
		StructFields     StructFields
		ModelType        reflect.Type
		defaultTableName string
	}

	safeMap struct {
		m map[string]string
		l *sync.RWMutex
	}

	safeModelStructsMap struct {
		m map[reflect.Type]*ModelStruct
		l *sync.RWMutex
	}

	// SQL expression
	expr struct {
		expr string
		args []interface{}
	}

	// DB contains information for current db connection
	//TODO : @Badu - if it holds current db connection why not name it accordingly???
	DB struct {
		parent  *DB
		dialect Dialect
		Value   interface{}
		values  map[string]interface{}

		Error error

		callbacks *Callback
		db        sqlCommon

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
		db *DB

		Search *search
		Value  interface{}

		selectAttrs *[]string

		SQL     string
		SQLVars []interface{}

		instanceID string

		primaryKeyField *StructField
		fields          *StructFields

		skipLeft bool
	}

	search struct {
		db               *DB
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

	tabler interface {
		TableName() string
	}
	dbTabler interface {
		TableName(*DB) string
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

	logger interface {
		Print(v ...interface{})
	}

	// LogWriter log writer interface
	LogWriter interface {
		Println(v ...interface{})
	}

	// Logger default logger
	Logger struct {
		LogWriter
	}

	// Callback is a struct that contains all CURD callbacks
	//   Field `creates` contains callbacks will be call when creating object
	//   Field `updates` contains callbacks will be call when updating object
	//   Field `deletes` contains callbacks will be call when deleting object
	//   Field `queries` contains callbacks will be call when querying object with query methods like Find, First, Related, Association...
	//   Field `rowQueries` contains callbacks will be call when querying object with Row, Rows...
	//   Field `processors` contains all callback processors, will be used to generate above callbacks in order
	Callback struct {
		creates    []*func(scope *Scope)
		updates    []*func(scope *Scope)
		deletes    []*func(scope *Scope)
		queries    []*func(scope *Scope)
		rowQueries []*func(scope *Scope)
		processors []*CallbackProcessor
	}

	// CallbackProcessor contains callback informations
	CallbackProcessor struct {
		name      string              // current callback's name
		before    string              // register current callback before a callback
		after     string              // register current callback after a callback
		replace   bool                // replace callbacks with same name
		remove    bool                // delete callbacks with same name
		kind      string              // callback type: create, update, delete, query, row_query
		processor *func(scope *Scope) // callback handler
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

	errorsInterface interface {
		GetErrors() []error
	}
	// Errors contains all happened errors
	Errors struct {
		errors []error
	}

	// JoinTableHandlerInterface is an interface for how to handle many2many relations
	JoinTableHandlerInterface interface {
		// initialize join table handler
		Setup(relationship *Relationship, tableName string, source reflect.Type, destination reflect.Type)
		// Table return join table's table name
		Table(db *DB) string
		// Add create relationship in join table for source and destination
		Add(handler JoinTableHandlerInterface, db *DB, source interface{}, destination interface{}) error
		// Delete delete relationship in join table for sources
		Delete(handler JoinTableHandlerInterface, db *DB, sources ...interface{}) error
		// JoinWith query with `Join` conditions
		JoinWith(handler JoinTableHandlerInterface, db *DB, source interface{}) *DB
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
	//this is a map for transforming strings into uint8 when reading tags of structs
	//@See : &StructField{}.ParseTagSettings()
	tagSettingMap = map[string]uint8{
		"MANY2MANY":               MANY2MANY,
		"AUTO_INCREMENT":          AUTO_INCREMENT,
		"INDEX":                   INDEX,
		"NOT NULL":                NOT_NULL,
		"SIZE":                    SIZE,
		"UNIQUE_INDEX":            UNIQUE_INDEX,
		"IS_JOINTABLE_FOREIGNKEY": IS_JOINTABLE_FOREIGNKEY,
		"PRIMARY_KEY":             PRIMARY_KEY,
		"DEFAULT":                 DEFAULT,
		"-":                       IGNORED,
		"EMBEDDED":                EMBEDDED,
		"EMBEDDED_PREFIX":         EMBEDDED_PREFIX,
		"FOREIGNKEY":              FOREIGNKEY,
		"ASSOCIATIONFOREIGNKEY":   ASSOCIATIONFOREIGNKEY,
		"POLYMORPHIC":             POLYMORPHIC,
		"POLYMORPHIC_VALUE":       POLYMORPHIC_VALUE,
		"COLUMN":                  COLUMN,
		"TYPE":                    TYPE,
		"UNIQUE":                  UNIQUE,
	}

	dialectsMap = map[string]Dialect{}

	// DefaultCallback default callbacks defined by gorm
	DefaultCallback = &Callback{}

	smap = newSafeMap()
	// DefaultTableNameHandler default table name handler
	DefaultTableNameHandler = func(db *DB, defaultTableName string) string {
		return defaultTableName
	}
	modelStructsMap = newModelStructsMap()

	// NowFunc returns current time, this function is exported in order to be able
	// to give the flexibility to the developer to customize it according to their
	// needs, e.g:
	//    gorm.NowFunc = func() time.Time {
	//      return time.Now().UTC()
	//    }
	NowFunc = func() time.Time {
		return time.Now()
	}

	// Copied from golint
	commonInitialisms         = []string{"API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID", "HTML", "HTTP", "HTTPS", "ID", "IP", "JSON", "LHS", "QPS", "RAM", "RHS", "RPC", "SLA", "SMTP", "SSH", "TLS", "TTL", "UI", "UID", "UUID", "URI", "URL", "UTF8", "VM", "XML", "XSRF", "XSS"}
	commonInitialismsReplacer *strings.Replacer

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
