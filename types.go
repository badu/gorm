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

	MANY_TO_MANY uint8 = 1
	HAS_MANY     uint8 = 2
	HAS_ONE      uint8 = 3
	BELONGS_TO   uint8 = 4
)

type (
	strCase bool
	// Association Mode contains some helper methods to handle relationship things easily.
	Association struct {
		Error  error
		scope  *Scope
		column string
		field  *Field
	}

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

	// StructField model field's struct definition
	StructField struct {
		DBName          string
		Name            string
		Names           []string
		IsPrimaryKey    bool
		IsNormal        bool
		IsIgnored       bool
		IsScanner       bool
		HasDefaultValue bool
		Tag             reflect.StructTag
		TagSettings     map[string]string
		Struct          reflect.StructField
		IsForeignKey    bool
		Relationship    *Relationship
	}

	// ModelStruct model definition
	ModelStruct struct {
		PrimaryFields    []*StructField
		StructFields     []*StructField
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

	// Field model field definition
	Field struct {
		*StructField
		IsBlank bool
		Field   reflect.Value
	}

	// DB contains information for current db connection
	DB struct {
		Value             interface{}
		Error             error
		RowsAffected      int64
		callbacks         *Callback
		db                sqlCommon
		parent            *DB
		search            *search
		logMode           int
		logger            logger
		dialect           Dialect
		singularTable     bool
		source            string
		values            map[string]interface{}
		joinTableHandlers map[string]JoinTableHandler
	}

	// Scope contain current operation's information when you perform any operation on the database
	Scope struct {
		Search          *search
		Value           interface{}
		SQL             string
		SQLVars         []interface{}
		db              *DB
		instanceID      string
		primaryKeyField *Field
		skipLeft        bool
		fields          *[]*Field
		selectAttrs     *[]string
	}

	search struct {
		db               *DB
		whereConditions  []map[string]interface{}
		orConditions     []map[string]interface{}
		notConditions    []map[string]interface{}
		havingConditions []map[string]interface{}
		joinConditions   []map[string]interface{}
		initAttrs        []interface{}
		assignAttrs      []interface{}
		selects          map[string]interface{}
		omits            []string
		orders           []interface{}
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
	sqlRegexp     = regexp.MustCompile(`(\$\d+)|\?`)

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
