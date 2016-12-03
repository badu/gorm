package gorm

import (
	"database/sql"
	"log"
	"os"
	"reflect"
	"regexp"
	"sync"
	"time"
)

const (
	upper strCase = true

	TAG_SQL         string = "sql"
	TAG_GORM        string = "gorm"
	DEFAULT_ID_NAME string = "id"

	ASCENDENT  string = "ASC"
	DESCENDENT string = "DESC"

	UPDATE_COLUMN_SETTING      uint64 = 1
	INSERT_OPT_SETTING         uint64 = 2
	DELETE_OPT_SETTING         uint64 = 3
	ORDER_BY_PK_SETTING        uint64 = 4
	TABLE_OPT_SETTING          uint64 = 5
	QUERY_DEST_SETTING         uint64 = 6
	QUERY_OPT_SETTING          uint64 = 7
	SAVE_ASSOC_SETTING         uint64 = 8
	UPDATE_OPT_SETTING         uint64 = 9
	UPDATE_INTERF_SETTING      uint64 = 10
	IGNORE_PROTEC_SETTING      uint64 = 11
	UPDATE_ATTRS_SETTING       uint64 = 12
	STARTED_TX_SETTING         uint64 = 13
	BLANK_COLS_DEFAULT_SETTING uint64 = 14
)

type (
	Uint8Map map[uint8]string
	//since there is no other way of embedding a map
	TagSettings struct {
		Uint8Map
		l *sync.RWMutex
	}
	// StructField model field's struct definition
	//TODO : @Badu - a StructField should support multiple relationships
	StructField struct {
		flags        uint16
		DBName       string
		StructName   string
		Names        []string
		tagSettings  TagSettings
		Value        reflect.Value
		Type         reflect.Type
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
		source                       *StructField
		destination                  *StructField
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
		fieldsMap           fieldsMap    //keeper of the fields, a safe map (aliases) and slice
		cachedPrimaryFields StructFields //collected from fields.fields, so we won't iterate all the time
		ModelType           reflect.Type
		defaultTableName    string
		relations           []*Relationship
	}

	// Scope contain current operation's information when you perform any operation on the database
	Scope struct {
		con        *DBCon
		Search     *Search
		instanceID uint64
		fields     *StructFields //cached version of cloned struct fields
		Value      interface{}
		//TODO : @Badu - add Type of Value here to avoid "so much reflection" effect
	}

	sqlConditionType uint16
	SqlPair          struct {
		expression interface{}
		args       []interface{}
	}
	sqlCondition  []SqlPair
	SqlConditions map[sqlConditionType]sqlCondition

	Search struct {
		flags      uint16
		Conditions SqlConditions
		tableName  string
		SQL        string
		SQLVars    []interface{}
		Value      interface{} //TODO : @Badu - moved here from DBCon - in the end should use Scope's Value
	}

	DBConFunc func(*DBCon) *DBCon

	// DBCon contains information for current db connection
	DBCon struct {
		parent        *DBCon
		dialect       Dialect
		settings      map[uint64]interface{}
		search        *Search
		logMode       int
		logger        logger
		callback      *Callback
		sqli          sqlInterf
		singularTable bool
		Error         error

		RowsAffected int64
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
		// GetQuoter returns the rune for quoting field name to avoid SQL parsing exceptions by using a reserved word as a field name
		GetQuoter() string
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

	defaultLogger = Logger{log.New(os.Stdout, "\r\n", 0)}

	//reverse map to allow external settings
	settingsMap = map[string]uint64{
		"gorm:update_column":                    UPDATE_COLUMN_SETTING,
		"gorm:insert_option":                    INSERT_OPT_SETTING,
		"gorm:update_option":                    UPDATE_OPT_SETTING,
		"gorm:delete_option":                    DELETE_OPT_SETTING,
		"gorm:table_options":                    TABLE_OPT_SETTING,
		"gorm:query_option":                     QUERY_OPT_SETTING,
		"gorm:order_by_primary_key":             ORDER_BY_PK_SETTING,
		"gorm:query_destination":                QUERY_DEST_SETTING,
		"gorm:save_associations":                SAVE_ASSOC_SETTING,
		"gorm:update_interface":                 UPDATE_INTERF_SETTING,
		"gorm:ignore_protected_attrs":           IGNORE_PROTEC_SETTING,
		"gorm:update_attrs":                     UPDATE_ATTRS_SETTING,
		"gorm:started_transaction":              STARTED_TX_SETTING,
		"gorm:blank_columns_with_default_value": BLANK_COLS_DEFAULT_SETTING,
	}
	// Attention : using "unprepared" regexp.MustCompile is really slow : ten times slower
	// only matches string like `name`, `users.name`
	regExpNameMatcher = regexp.MustCompile("^[a-zA-Z]+(\\.[a-zA-Z]+)*$")
	// only matches numbers
	regExpNumberMatcher = regexp.MustCompile("^\\s*\\d+\\s*$")
	//matches like, is, in, compare ...
	regExpLikeInMatcher = regexp.MustCompile("(?i) (=|<>|>|<|LIKE|IS|IN) ")
	//matches word "count"
	regExpCounter = regexp.MustCompile("(?i)^count(.+)$")
	//foreign key matcher
	regExpFKName = regexp.MustCompile("(_*[^a-zA-Z]+_*|_+)")
	//used in Quote to replace all periods with quote-period-quote
	regExpPeriod = regexp.MustCompile("\\.")

	//positiveIntegerMatcher = regexp.MustCompile("/^\\d+$/")
	//negativeIntegerMatcher= regexp.MustCompile("/^-\\d+$/")
	//integerMatcher = regexp.MustCompile("/^-?\\d+$/")
	//positiveNumberMatcher = regexp.MustCompile("/^\\d*\\.?\\d+$/")
	//negativeNumberMatcher = regexp.MustCompile("/^-\\d*\\.?\\d+$/")
	//numberMatcher = regexp.MustCompile("/^-?\\d*\\.?\\d+$/")
	//time24HourMatcher = regexp.MustCompile("/([01]?[0-9]|2[0-3]):[0-5][0-9]/")
	//dateTimeISO8601Matcher = regexp.MustCompile("/^(?![+-]?\\d{4,5}-?(?:\\d{2}|W\\d{2})T)(?:|(\\d{4}|[+-]\\d{5})-?(?:|(0\\d|1[0-2])(?:|-?([0-2]\\d|3[0-1]))|([0-2]\\d{2}|3[0-5]\\d|36[0-6])|W([0-4]\\d|5[0-3])(?:|-?([1-7])))(?:(?!\\d)|T(?=\\d)))(?:|([01]\\d|2[0-4])(?:|:?([0-5]\\d)(?:|:?([0-5]\\d)(?:|\\.(\\d{3})))(?:|[zZ]|([+-](?:[01]\\d|2[0-4]))(?:|:?([0-5]\\d)))))$/")
)
