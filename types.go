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
	// Callback Kind constants
	createCallback uint8 = 1
	updateCallback uint8 = 2
	deleteCallback uint8 = 3
	queryCallback  uint8 = 4
	rowCallback    uint8 = 5

	//StructField TagSettings constants
	setMany2manyName                uint8 = 1
	setIndex                        uint8 = 2
	setNotNull                      uint8 = 3
	setSize                         uint8 = 4
	setUniqueIndex                  uint8 = 5
	setIsJointableForeignkey        uint8 = 6
	setDefault                      uint8 = 7
	setEmbeddedPrefix               uint8 = 8
	setForeignkey                   uint8 = 9
	setAssociationforeignkey        uint8 = 10
	setColumn                       uint8 = 11
	setType                         uint8 = 12
	setUnique                       uint8 = 13
	setSaveAssociations             uint8 = 14
	setPolymorphic                  uint8 = 15
	setPolymorphicValue             uint8 = 16 // was both PolymorphicValue in Relationship struct, and also collected from tags
	setPolymorphicType              uint8 = 17 // was PolymorphicType in Relationship struct
	setPolymorphicDbname            uint8 = 18 // was PolymorphicDBName in Relationship struct
	setRelationKind                 uint8 = 19 // was Kind in Relationship struct
	setJoinTableHandler             uint8 = 20 // was JoinTableHandler in Relationship struct
	setForeignFieldNames            uint8 = 21 // was ForeignFieldNames in Relationship struct
	setForeignDbNames               uint8 = 22 // was ForeignDBNames in Relationship struct
	setAssociationForeignFieldNames uint8 = 23 // was AssociationForeignFieldNames in Relationship struct
	setAssociationForeignDbNames    uint8 = 24 // was AssociationForeignDBNames in Relationship struct

	// Tags that can be defined `sql` or `gorm`
	tagAutoIncrement         = "AUTO_INCREMENT"
	tagPrimaryKey            = "PRIMARY_KEY"
	tagTransient             = "-"
	tagDefaultStr            = "DEFAULT"
	tagEmbedded              = "EMBEDDED"
	tagManyToMany            = "MANY2MANY"
	tagIndex                 = "INDEX"
	tagNotNull               = "NOT NULL"
	tagSize                  = "SIZE"
	tagUniqueIndex           = "UNIQUE_INDEX"
	tagIsJointableForeignkey = "IS_JOINTABLE_FOREIGNKEY"
	tagEmbeddedPrefix        = "EMBEDDED_PREFIX"
	tagForeignkey            = "FOREIGNKEY"
	tagAssociationForeignKey = "ASSOCIATIONFOREIGNKEY"
	tagPolymorphic           = "POLYMORPHIC"
	tagPolymorphicValue      = "POLYMORPHIC_VALUE"
	tagColumn                = "COLUMN"
	tagType                  = "TYPE"
	tagUnique                = "UNIQUE"
	tagSaveAssociations      = "SAVE_ASSOCIATIONS"

	//not really tags, but used in cachedReverseTagSettingsMap for Stringer
	tagRelationKind           = "Relation kind"
	tagJoinTableHandler       = "Join Table Handler"
	tagForeignFieldNames      = "Foreign field names"
	tagForeignDbNames         = "Foreign db names"
	tagAssocForeignFieldNames = "Assoc foreign field names"
	tagAssocForeignDbNames    = "Assoc foreign db names"

	//StructField bit flags - flags are uint16, which means we can use 16 flags
	ffIsPrimarykey    uint8 = 0
	ffIsNormal        uint8 = 1
	ffIsIgnored       uint8 = 2
	ffIsScanner       uint8 = 3
	ffIsTime          uint8 = 4
	ffHasDefaultValue uint8 = 5
	ffIsForeignkey    uint8 = 6
	ffIsBlank         uint8 = 7
	ffIsSlice         uint8 = 8
	ffIsStruct        uint8 = 9
	ffHasRelations    uint8 = 10
	ffIsEmbedOrAnon   uint8 = 11
	ffIsAutoincrement uint8 = 12
	ffIsPointer       uint8 = 13
	ffRelationCheck   uint8 = 14

	//Relationship Kind constants
	relMany2many uint8 = 1
	relHasMany   uint8 = 2
	relHasOne    uint8 = 3
	relBelongsTo uint8 = 4
	//Attention : relationship.Kind <= rel_has_one in callback_functions.go saveAfterAssociationsCallback()
	//which means except rel_belongs_to

	//Search struct map keys
	condSelectQuery  sqlConditionType = 0
	condWhereQuery   sqlConditionType = 1
	condNotQuery     sqlConditionType = 2
	condOrQuery      sqlConditionType = 3
	condHavingQuery  sqlConditionType = 4
	condJoinsQuery   sqlConditionType = 5
	condInitAttrs    sqlConditionType = 6
	condAssignAttrs  sqlConditionType = 7
	condPreloadQuery sqlConditionType = 8
	condOrderQuery   sqlConditionType = 9
	condOmitsQuery   sqlConditionType = 10
	condGroupQuery   sqlConditionType = 11
	condLimitQuery   sqlConditionType = 12
	condOffsetQuery  sqlConditionType = 13

	//Search struct flag constants
	srchIsUnscoped       uint16 = 0
	srchIsRaw            uint16 = 1
	srchIsOrderIgnored   uint16 = 2
	srchHasSelect        uint16 = 3
	srchHasWhere         uint16 = 4
	srchHasNot           uint16 = 5
	srchHasOr            uint16 = 6
	srchHasHaving        uint16 = 7
	srchHasJoins         uint16 = 8
	srchHasInit          uint16 = 9
	srchHasAssign        uint16 = 10
	srchHasPreload       uint16 = 11
	srchHasOrder         uint16 = 12
	srchHasOmits         uint16 = 13
	srchHasGroup         uint16 = 14
	srchHasOffsetOrLimit uint16 = 15

	//Method names
	methAfterCreate  = "AfterCreate"
	methAfterSave    = "AfterSave"
	methAfterDelete  = "AfterDelete"
	methAfterFind    = "AfterFind"
	methAfterUpdate  = "AfterUpdate"
	methBeforeCreate = "BeforeCreate"
	methBeforeSave   = "BeforeSave"
	methBeforeDelete = "BeforeDelete"
	methBeforeUpdate = "BeforeUpdate"

	//Errors
	errKeyNotFound         = "error TagSetting : COULDN'T FIND KEY FOR %q ON %q"
	errMissingFieldNames   = "error TagSetting : missing (or two many) field names in foreign or association key (%s %s)"
	errCannotConvert       = "could not convert argument of field %s from %s to %s"
	errStructFieldNotValid = "error StructField : field value not valid"
	errFkLengthNotEqual    = "rel [%q]: invalid foreign keys, should have same length"
	errProcessingTags      = "error ModelStruct %q processing tags error : %v"
	errAddingField         = "error ModelStruct %q add field error : %v"
	errNoBelongOrHasone    = "%q (%q [%q]) is HAS ONE / BELONG TO missing"
	errFieldNotFound       = "field %q not found on %q"
	errUnsupportedRelation = "unsupported relation : %d"
	errCantPreload         = "can't preload field %s for %s"
	//Warnings
	warnPolyFieldNotFound   = "\nrel : polymorphic field %q not found on model struct %q"
	warnFkFieldNotFound     = "\nrel [%q]: foreign key field %q not found on model struct %q pointed by %q [%q]"
	warnAfkFieldNotFound    = "\nrel [%q]: association foreign key field %q not found on model struct %q"
	warnHasNoForeignKey     = "\nrel [%q]: field has no foreign key setting"
	warnHasNoAssociationKey = "\nrel [%q]: field has no association key setting"

	//typical fields constants
	fieldDefaultIdName = "id"
	fieldDeletedAtName = "deleted_at"
	fieldPolyType      = "Type"
	FieldCreatedAt     = "CreatedAt"
	FieldUpdatedAt     = "UpdatedAt"
	FieldDeletedAt     = "DeletedAt"
	fieldIdName        = "Id"

	//Extracted strings
	strTagSql     = "sql"
	strTagGorm    = "gorm"
	strAscendent  = "ASC"
	strDescendent = "DESC"
	strHasmany    = "HasMany"
	strHasone     = "HasOne"
	strBelongsto  = "BelongTo"
	strCollectfks = "CollectFKs"
	strEverything = "*"
	strPrimaryKey = "primary key"

	//Gorm settings for map (Set / Get)
	gormSettingUpdateColumn      uint64 = 1
	gormSettingInsertOpt         uint64 = 2
	gormSettingDeleteOpt         uint64 = 3
	gormSettingTableOpt          uint64 = 4
	gormSettingQueryOpt          uint64 = 5 // usually, this contains "FOR UPDATE". See QueryOption test
	gormSettingSaveAssoc         uint64 = 6
	gormSettingUpdateOpt         uint64 = 7
	gormSettingAssociationSource uint64 = 8 //TODO : @Badu - maybe it's better to keep this info in Association struct

	//
	upper strCase = true

	LogOff     int = 1
	LogVerbose int = 2
	LogDebug   int = 3
)

type (
	Uint8Map map[uint8]interface{}
	//since there is no other way of embedding a map
	TagSettings struct {
		Uint8Map
		l *sync.RWMutex
	}
	// StructField model field's struct definition
	StructField struct {
		flags       uint16
		DBName      string
		StructName  string
		Names       []string
		tagSettings TagSettings
		Value       reflect.Value
		Type        reflect.Type
	}

	//easier to read and can apply methods
	StructFields []*StructField

	strCase bool
	//TODO : @Badu - Association has a field named Error - should be passed to DBCon
	//TODO : @Badu - Association Mode contains some helper methods to handle relationship things easily.
	Association struct {
		Error  error
		scope  *Scope
		column string
		field  *StructField
	}

	fieldsMap struct {
		aliases map[string]*StructField
		mu      *sync.RWMutex
		fields  StructFields
	}

	// ModelStruct model definition
	ModelStruct struct {
		fieldsMap           fieldsMap    //keeper of the fields, a safe map (aliases) and slice
		cachedPrimaryFields StructFields //collected from fields.fields, so we won't iterate all the time
		ModelType           reflect.Type
		defaultTableName    string
	}

	// Scope contain current operation's information when you perform any operation on the database
	Scope struct {
		con    *DBCon
		Search *Search
		fields *StructFields //cached version of cloned struct fields
		Value  interface{}
		rValue reflect.Value
		rType  reflect.Type
		//added to get rid of UPDATE_ATTRS_SETTING - since it's accessible only in that instance
		updateMaps map[string]interface{}
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
		search        *Search //TODO : @Badu - should always have a Scope, not a Search - better hierarchy
		logMode       int
		logger        logger
		callbacks     *Callbacks
		sqli          sqlInterf
		singularTable bool
		Error         error

		RowsAffected int64 //TODO : @Badu - this should sit inside Scope, because it's contextual
		//TODO : @Badu - add flags - which includes singularTable, future blockGlobalUpdate and logMode (encoded on 3 bytes)

		modelsStructMap *safeModelStructsMap
		namesMap        *safeMap
		quotedNames     *safeMap
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
	CallbacksProcessors []*CallbacksProcessor
	// Callback is a struct that contains all CURD callbacks
	//   Field `creates` contains callbacks will be call when creating object
	//   Field `updates` contains callbacks will be call when updating object
	//   Field `deletes` contains callbacks will be call when deleting object
	//   Field `queries` contains callbacks will be call when querying object with query methods like Find, First, Related, Association...
	//   Field `rowQueries` contains callbacks will be call when querying object with Row, Rows...
	//   Field `processors` contains all callback processors, will be used to generate above callbacks in order
	Callbacks struct {
		creates    ScopedFuncs
		updates    ScopedFuncs
		deletes    ScopedFuncs
		queries    ScopedFuncs
		rowQueries ScopedFuncs
		processors CallbacksProcessors
	}

	// CallbackProcessor contains callback informations
	CallbacksProcessor struct {
		//remember : we can't remove "name" property, since callbacks gets sorted/re-ordered
		name      string      // current callback's name
		before    string      // register current callback before a callback
		after     string      // register current callback after a callback
		replace   bool        // replace callbacks with same name
		remove    bool        // delete callbacks with same name
		kind      uint8       // callback type: create, update, delete, query, row_query
		processor *ScopedFunc // callback handler
		parent    *Callbacks
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
	//TODO : @Badu -this holds some sort of processed clone of FOREIGN_DB_NAMES, FOREIGN_FIELD_NAMES, ASSOCIATION_FOREIGN_FIELD_NAMES, ASSOCIATION_FOREIGN_DB_NAMES
	JoinTableForeignKey struct {
		DBName            string
		AssociationDBName string
	}
	// JoinTableSource is a struct that contains model type and foreign keys
	JoinTableInfo struct {
		ModelType   reflect.Type
		ForeignKeys []JoinTableForeignKey
	}
	// JoinTableHandler default join table handler
	JoinTableHandler struct {
		TableName   string        `sql:"-"`
		Source      JoinTableInfo `sql:"-"`
		Destination JoinTableInfo `sql:"-"`
	}

	safeModelStructsMap struct {
		m map[reflect.Type]*ModelStruct
		l *sync.RWMutex
	}

	safeMap struct {
		m map[string]string
		l *sync.RWMutex
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

	errorsInterface interface {
		GetErrors() []error
	}
	// Errors contains all happened errors
	GormErrors []error

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
		Setup(field *StructField, source reflect.Type, destination reflect.Type)
		// Table return join table's table name
		Table(db *DBCon) string
		// Sets table name
		SetTable(name string)
		// Add create relationship in join table for source and destination
		Add(handler JoinTableHandlerInterface, db *DBCon, source interface{}, destination interface{}) error
		// Delete delete relationship in join table for sources
		Delete(handler JoinTableHandlerInterface, db *DBCon) error
		// JoinWith query with `Join` conditions
		JoinWith(handler JoinTableHandlerInterface, db *DBCon, source interface{}) *DBCon
		// SourceForeignKeys return source foreign keys
		SourceForeignKeys() []JoinTableForeignKey
		// DestinationForeignKeys return destination foreign keys
		DestinationForeignKeys() []JoinTableForeignKey
		//for debugging purposes
		GetHandlerStruct() *JoinTableHandler
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
		//TODO : @Badu - should return a rune
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

	// Copied from golint
	commonInitialisms         = []string{"API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID", "HTML", "HTTP", "HTTPS", "ID", "IP", "JSON", "LHS", "QPS", "RAM", "RHS", "RPC", "SLA", "SMTP", "SSH", "TLS", "TTL", "UI", "UID", "UUID", "URI", "URL", "UTF8", "VM", "XML", "XSRF", "XSS"}
	commonInitialismsReplacer *strings.Replacer

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
	gormSettingsMap = map[string]uint64{
		"gorm:update_column":      gormSettingUpdateColumn,
		"gorm:insert_option":      gormSettingInsertOpt,
		"gorm:update_option":      gormSettingUpdateOpt,
		"gorm:delete_option":      gormSettingDeleteOpt,
		"gorm:table_options":      gormSettingTableOpt,
		"gorm:query_option":       gormSettingQueryOpt,
		"gorm:save_associations":  gormSettingSaveAssoc,
		"gorm:association:source": gormSettingAssociationSource,
	}

	//this is a map for transforming strings into uint8 when reading tags of structs
	tagSettingMap = map[string]uint8{
		tagManyToMany:             setMany2manyName,
		tagIndex:                  setIndex,
		tagNotNull:                setNotNull,
		tagSize:                   setSize,
		tagUniqueIndex:            setUniqueIndex,
		tagIsJointableForeignkey:  setIsJointableForeignkey,
		tagDefaultStr:             setDefault,
		tagEmbeddedPrefix:         setEmbeddedPrefix,
		tagForeignkey:             setForeignkey,
		tagAssociationForeignKey:  setAssociationforeignkey,
		tagPolymorphic:            setPolymorphic,
		tagPolymorphicValue:       setPolymorphicValue,
		tagColumn:                 setColumn,
		tagType:                   setType,
		tagUnique:                 setUnique,
		tagSaveAssociations:       setSaveAssociations,
		tagRelationKind:           setRelationKind,
		tagJoinTableHandler:       setJoinTableHandler,
		tagForeignFieldNames:      setForeignFieldNames,
		tagForeignDbNames:         setForeignDbNames,
		tagAssocForeignFieldNames: setAssociationForeignFieldNames,
		tagAssocForeignDbNames:    setAssociationForeignDbNames,
	}

	kindNamesMap = map[uint8]string{
		relMany2many: "Many to many",
		relHasMany:   "Has many",
		relHasOne:    "Has one",
		relBelongsTo: "Belongs to",
	}

	regexpSelf   = regexp.MustCompile(`gorm/.*.go`)
	regexpTest   = regexp.MustCompile(`gorm/tests/.*.go`)
	regExpLogger = regexp.MustCompile(`(\$\d+)|\?`)

	cachedReverseTagSettingsMap map[uint8]string
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
	//checks for DISTINCT presence in SQL expression
	distinctSQLRegexp = regexp.MustCompile(`(?i)distinct[^a-z]+[a-z]+`)

	//positiveIntegerMatcher = regexp.MustCompile("/^\\d+$/")
	//negativeIntegerMatcher= regexp.MustCompile("/^-\\d+$/")
	//integerMatcher = regexp.MustCompile("/^-?\\d+$/")
	//positiveNumberMatcher = regexp.MustCompile("/^\\d*\\.?\\d+$/")
	//negativeNumberMatcher = regexp.MustCompile("/^-\\d*\\.?\\d+$/")
	//numberMatcher = regexp.MustCompile("/^-?\\d*\\.?\\d+$/")
	//time24HourMatcher = regexp.MustCompile("/([01]?[0-9]|2[0-3]):[0-5][0-9]/")
	//dateTimeISO8601Matcher = regexp.MustCompile("/^(?![+-]?\\d{4,5}-?(?:\\d{2}|W\\d{2})T)(?:|(\\d{4}|[+-]\\d{5})-?(?:|(0\\d|1[0-2])(?:|-?([0-2]\\d|3[0-1]))|([0-2]\\d{2}|3[0-5]\\d|36[0-6])|W([0-4]\\d|5[0-3])(?:|-?([1-7])))(?:(?!\\d)|T(?=\\d)))(?:|([01]\\d|2[0-4])(?:|:?([0-5]\\d)(?:|:?([0-5]\\d)(?:|\\.(\\d{3})))(?:|[zZ]|([+-](?:[01]\\d|2[0-4]))(?:|:?([0-5]\\d)))))$/")

	// ErrRecordNotFound record not found error, happens when haven't find any matched data when looking up with a struct
	ErrRecordNotFound = errors.New("record not found")

	// ErrInvalidSQL invalid SQL error, happens when you passed invalid SQL
	_ = errors.New("invalid SQL")

	// ErrInvalidTransaction invalid transaction when you are trying to `Commit` or `Rollback`
	ErrInvalidTransaction = errors.New("no valid transaction")

	// ErrCantStartTransaction can't start transaction when you are trying to start one with `Begin`
	ErrCantStartTransaction = errors.New("can't start transaction")

	// ErrUnaddressable unaddressable value
	ErrUnaddressable = errors.New("using unaddressable value")
)
