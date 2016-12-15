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
	//StructField TagSettings constants
	set_many2many_name                  uint8 = 1
	set_index                           uint8 = 2
	set_not_null                        uint8 = 3
	set_size                            uint8 = 4
	set_unique_index                    uint8 = 5
	set_is_jointable_foreignkey         uint8 = 6
	set_default                         uint8 = 7
	set_embedded_prefix                 uint8 = 8
	set_foreignkey                      uint8 = 9
	set_associationforeignkey           uint8 = 10
	set_column                          uint8 = 11
	set_type                            uint8 = 12
	set_unique                          uint8 = 13
	set_save_associations               uint8 = 14
	set_polymorphic                     uint8 = 15
	set_polymorphic_value               uint8 = 16 // was both PolymorphicValue in Relationship struct, and also collected from tags
	set_polymorphic_type                uint8 = 17 // was PolymorphicType in Relationship struct
	set_polymorphic_dbname              uint8 = 18 // was PolymorphicDBName in Relationship struct
	set_relation_kind                   uint8 = 19 // was Kind in Relationship struct
	set_join_table_handler              uint8 = 20 // was JoinTableHandler in Relationship struct
	set_foreign_field_names             uint8 = 21 // was ForeignFieldNames in Relationship struct
	set_foreign_db_names                uint8 = 22 // was ForeignDBNames in Relationship struct
	set_association_foreign_field_names uint8 = 23 // was AssociationForeignFieldNames in Relationship struct
	set_association_foreign_db_names    uint8 = 24 // was AssociationForeignDBNames in Relationship struct

	//Tags that can be defined `sql` or `gorm`
	tag_auto_increment          string = "AUTO_INCREMENT"
	tag_primary_key             string = "PRIMARY_KEY"
	tag_ignored                 string = "-"
	tag_default_str             string = "DEFAULT"
	tag_embedded                string = "EMBEDDED"
	tag_many_to_many            string = "MANY2MANY"
	tag_index                   string = "INDEX"
	tag_not_null                string = "NOT NULL"
	tag_size                    string = "SIZE"
	tag_unique_index            string = "UNIQUE_INDEX"
	tag_is_jointable_foreignkey string = "IS_JOINTABLE_FOREIGNKEY"
	tag_embedded_prefix         string = "EMBEDDED_PREFIX"
	tag_foreignkey              string = "FOREIGNKEY"
	tag_association_foreign_key string = "ASSOCIATIONFOREIGNKEY"
	tag_polymorphic             string = "POLYMORPHIC"
	tag_polymorphic_value       string = "POLYMORPHIC_VALUE"
	tag_column                  string = "COLUMN"
	tag_type                    string = "TYPE"
	tag_unique                  string = "UNIQUE"
	tag_save_associations       string = "SAVE_ASSOCIATIONS"
	//not really tags, but used in cachedReverseTagSettingsMap for Stringer
	tag_relation_kind             string = "Relation kind"
	tag_join_table_handler        string = "Join Table Handler"
	tag_foreign_field_names       string = "Foreign field names"
	tag_foreign_db_names          string = "Foreign db names"
	tag_assoc_foreign_field_names string = "Assoc foreign field names"
	tag_assoc_foreign_db_names    string = "Assoc foreign db names"

	//StructField bit flags - flags are uint16, which means we can use 16 flags
	ff_is_primarykey     uint8 = 0
	ff_is_normal         uint8 = 1
	ff_is_ignored        uint8 = 2
	ff_is_scanner        uint8 = 3
	ff_is_time           uint8 = 4
	ff_has_default_value uint8 = 5
	ff_is_foreignkey     uint8 = 6
	ff_is_blank          uint8 = 7
	ff_is_slice          uint8 = 8
	ff_is_struct         uint8 = 9
	ff_has_relations     uint8 = 10
	ff_is_embed_or_anon  uint8 = 11
	ff_is_autoincrement  uint8 = 12
	ff_is_pointer        uint8 = 13
	ff_relation_check    uint8 = 14

	//Relationship Kind constants
	rel_many2many  uint8 = 1
	rel_has_many   uint8 = 2
	rel_has_one    uint8 = 3
	rel_belongs_to uint8 = 4
	//Attention : relationship.Kind <= rel_has_one in callback_functions.go saveAfterAssociationsCallback()
	//which means except rel_belongs_to

	//Method names
	meth_after_create  string = "AfterCreate"
	meth_after_save    string = "AfterSave"
	meth_after_delete  string = "AfterDelete"
	meth_after_find    string = "AfterFind"
	meth_after_update  string = "AfterUpdate"
	meth_before_create string = "BeforeCreate"
	meth_before_save   string = "BeforeSave"
	meth_before_delete string = "BeforeDelete"
	meth_before_update string = "BeforeUpdate"

	//TODO : Extract errors here
	//ERRORS
	err_key_not_found          string = "TagSetting : COULDN'T FIND KEY FOR %q ON %q"
	err_missing_field_names    string = "TagSetting : missing (or two many) field names in foreign or association key"
	err_cannot_convert         string = "could not convert argument of field %s from %s to %s"
	err_struct_field_not_valid string = "StructField : field value not valid"

	upper strCase = true

	TAG_SQL         string = "sql"
	TAG_GORM        string = "gorm"
	DEFAULT_ID_NAME string = "id"

	ASCENDENT  string = "ASC"
	DESCENDENT string = "DESC"

	UPDATE_COLUMN_SETTING      uint64 = 1
	INSERT_OPT_SETTING         uint64 = 2
	DELETE_OPT_SETTING         uint64 = 3
	TABLE_OPT_SETTING          uint64 = 4
	QUERY_OPT_SETTING          uint64 = 5 // usually, this contains "FOR UPDATE". See QueryOption test
	SAVE_ASSOC_SETTING         uint64 = 6
	UPDATE_OPT_SETTING         uint64 = 7
	ASSOCIATION_SOURCE_SETTING uint64 = 8 //TODO : @Badu - maybe it's better to keep this info in Association struct

	LOG_OFF     int = 1
	LOG_VERBOSE int = 2
	LOG_DEBUG   int = 3

	CREATED_AT_FIELD_NAME string = "CreatedAt"
	UPDATED_AT_FIELD_NAME string = "UpdatedAt"
	DELETED_AT_FIELD_NAME string = "DeletedAt"
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
		//TODO : @Badu - add Type of Value here to avoid "so much reflection" effect

		//added to get rid of UPDATE_ATTRS_SETTING - since it's accessible only in that instance
		updateMaps map[string]interface{}
		//added to get rid of UPDATE_INTERF_SETTING - since it's accessible only in that instance
		attrs interface{}
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
		"gorm:update_column":      UPDATE_COLUMN_SETTING,
		"gorm:insert_option":      INSERT_OPT_SETTING,
		"gorm:update_option":      UPDATE_OPT_SETTING,
		"gorm:delete_option":      DELETE_OPT_SETTING,
		"gorm:table_options":      TABLE_OPT_SETTING,
		"gorm:query_option":       QUERY_OPT_SETTING,
		"gorm:save_associations":  SAVE_ASSOC_SETTING,
		"gorm:association:source": ASSOCIATION_SOURCE_SETTING,
	}

	//this is a map for transforming strings into uint8 when reading tags of structs
	tagSettingMap = map[string]uint8{
		tag_many_to_many:              set_many2many_name,
		tag_index:                     set_index,
		tag_not_null:                  set_not_null,
		tag_size:                      set_size,
		tag_unique_index:              set_unique_index,
		tag_is_jointable_foreignkey:   set_is_jointable_foreignkey,
		tag_default_str:               set_default,
		tag_embedded_prefix:           set_embedded_prefix,
		tag_foreignkey:                set_foreignkey,
		tag_association_foreign_key:   set_associationforeignkey,
		tag_polymorphic:               set_polymorphic,
		tag_polymorphic_value:         set_polymorphic_value,
		tag_column:                    set_column,
		tag_type:                      set_type,
		tag_unique:                    set_unique,
		tag_save_associations:         set_save_associations,
		tag_relation_kind:             set_relation_kind,
		tag_join_table_handler:        set_join_table_handler,
		tag_foreign_field_names:       set_foreign_field_names,
		tag_foreign_db_names:          set_foreign_db_names,
		tag_assoc_foreign_field_names: set_association_foreign_field_names,
		tag_assoc_foreign_db_names:    set_association_foreign_db_names,
	}

	kindNamesMap = map[uint8]string{
		rel_many2many:  "Many to many",
		rel_has_many:   "Has many",
		rel_has_one:    "Has one",
		rel_belongs_to: "Belongs to",
	}

	cachedReverseTagSettingsMap map[uint8]string
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
)
