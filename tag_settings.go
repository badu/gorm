package gorm

import (
	"sync"
)

const (
	//error if tag not defined
	key_not_found_err       string = "TagSetting : COULDN'T FIND KEY FOR %q ON %q"
	missing_field_names_err string = "TagSetting : missing (or two many) field names in foreign or association key"

	//StructField TagSettings constants
	MANY2MANY_NAME                  uint8 = 1
	INDEX                           uint8 = 2
	NOT_NULL                        uint8 = 3
	SIZE                            uint8 = 4
	UNIQUE_INDEX                    uint8 = 5
	IS_JOINTABLE_FOREIGNKEY         uint8 = 6
	DEFAULT                         uint8 = 7
	EMBEDDED_PREFIX                 uint8 = 8
	FOREIGNKEY                      uint8 = 9
	ASSOCIATIONFOREIGNKEY           uint8 = 10
	COLUMN                          uint8 = 11
	TYPE                            uint8 = 12
	UNIQUE                          uint8 = 13
	SAVE_ASSOCIATIONS               uint8 = 14
	POLYMORPHIC                     uint8 = 15
	POLYMORPHIC_VALUE               uint8 = 16 // was both PolymorphicValue in Relationship struct, and also collected from tags
	POLYMORPHIC_TYPE                uint8 = 17 // was PolymorphicType in Relationship struct
	POLYMORPHIC_DBNAME              uint8 = 18 // was PolymorphicDBName in Relationship struct
	RELATION_KIND                   uint8 = 19 // was Kind in Relationship struct
	JOIN_TABLE_HANDLER              uint8 = 20 // was JoinTableHandler in Relationship struct
	FOREIGN_FIELD_NAMES             uint8 = 21 // was ForeignFieldNames in Relationship struct
	FOREIGN_DB_NAMES                uint8 = 22 // was ForeignDBNames in Relationship struct
	ASSOCIATION_FOREIGN_FIELD_NAMES uint8 = 23 // was AssociationForeignFieldNames in Relationship struct
	ASSOCIATION_FOREIGN_DB_NAMES    uint8 = 24 // was AssociationForeignDBNames in Relationship struct

	//Tags that can be defined `sql` or `gorm`
	auto_increment          string = "AUTO_INCREMENT"
	primary_key             string = "PRIMARY_KEY"
	ignored                 string = "-"
	default_str             string = "DEFAULT"
	embedded                string = "EMBEDDED"
	many_to_many            string = "MANY2MANY"
	index                   string = "INDEX"
	not_null                string = "NOT NULL"
	size                    string = "SIZE"
	unique_index            string = "UNIQUE_INDEX"
	is_jointable_foreignkey string = "IS_JOINTABLE_FOREIGNKEY"
	embedded_prefix         string = "EMBEDDED_PREFIX"
	foreignkey              string = "FOREIGNKEY"
	association_foreign_key string = "ASSOCIATIONFOREIGNKEY"
	polymorphic             string = "POLYMORPHIC"
	polymorphic_value       string = "POLYMORPHIC_VALUE"
	column                  string = "COLUMN"
	type_str                string = "TYPE"
	unique                  string = "UNIQUE"
	save_associations       string = "SAVE_ASSOCIATIONS"

	//not really tags, but used in cachedReverseTagSettingsMap for Stringer
	relation_kind             string = "Relation kind"
	join_table_handler        string = "Join Table Handler"
	foreign_field_names       string = "Foreign field names"
	foreign_db_names          string = "Foreign db names"
	assoc_foreign_field_names string = "Assoc foreign field names"
	assoc_foreign_db_names    string = "Assoc foreign db names"

	//Relationship Kind constants
	MANY_TO_MANY uint8 = 1
	HAS_MANY     uint8 = 2
	HAS_ONE      uint8 = 3
	BELONGS_TO   uint8 = 4 //Attention : relationship.Kind <= HAS_ONE in callback_functions.go saveAfterAssociationsCallback()
	//which means except BELONGS_TO
)

var (
	//this is a map for transforming strings into uint8 when reading tags of structs
	tagSettingMap = map[string]uint8{
		many_to_many:              MANY2MANY_NAME,
		index:                     INDEX,
		not_null:                  NOT_NULL,
		size:                      SIZE,
		unique_index:              UNIQUE_INDEX,
		is_jointable_foreignkey:   IS_JOINTABLE_FOREIGNKEY,
		default_str:               DEFAULT,
		embedded_prefix:           EMBEDDED_PREFIX,
		foreignkey:                FOREIGNKEY,
		association_foreign_key:   ASSOCIATIONFOREIGNKEY,
		polymorphic:               POLYMORPHIC,
		polymorphic_value:         POLYMORPHIC_VALUE,
		column:                    COLUMN,
		type_str:                  TYPE,
		unique:                    UNIQUE,
		save_associations:         SAVE_ASSOCIATIONS,
		relation_kind:             RELATION_KIND,
		join_table_handler:        JOIN_TABLE_HANDLER,
		foreign_field_names:       FOREIGN_FIELD_NAMES,
		foreign_db_names:          FOREIGN_DB_NAMES,
		assoc_foreign_field_names: ASSOCIATION_FOREIGN_FIELD_NAMES,
		assoc_foreign_db_names:    ASSOCIATION_FOREIGN_DB_NAMES,
	}

	kindNamesMap = map[uint8]string{
		MANY_TO_MANY: "Many to many",
		HAS_MANY:     "Has many",
		HAS_ONE:      "Has one",
		BELONGS_TO:   "Belongs to",
	}

	cachedReverseTagSettingsMap map[uint8]string
)

func init() {
	cachedReverseTagSettingsMap = make(map[uint8]string)
	for k, v := range tagSettingMap {
		cachedReverseTagSettingsMap[v] = k
	}
}

//returns a clone of tag settings (used in cloning StructField)
func (t *TagSettings) clone() TagSettings {
	clone := TagSettings{Uint8Map: make(map[uint8]interface{}), l: new(sync.RWMutex)}
	for key, value := range t.Uint8Map {
		clone.Uint8Map[key] = value
	}
	return clone
}

//adds a key to the settings map
func (t *TagSettings) set(named uint8, value interface{}) {
	t.l.Lock()
	defer t.l.Unlock()
	t.Uint8Map[named] = value
}

func (t *TagSettings) unset(named uint8) {
	t.l.Lock()
	defer t.l.Unlock()
	delete(t.Uint8Map, named)
}

//checks if has such a key (for code readability)
func (t *TagSettings) has(named uint8) bool {
	t.l.Lock()
	defer t.l.Unlock()
	//test for a key without retrieving the value
	_, ok := t.Uint8Map[named]
	return ok
}

func (t *TagSettings) get(named uint8) (interface{}, bool) {
	t.l.Lock()
	defer t.l.Unlock()
	value, ok := t.Uint8Map[named]
	return value, ok
}

func (t *TagSettings) len() int {
	return len(t.Uint8Map)
}

//Stringer implementation
func (t TagSettings) String() string {
	var collector Collector

	for key, value := range t.Uint8Map {
		switch key {
		case ASSOCIATIONFOREIGNKEY,
			FOREIGNKEY,
			FOREIGN_FIELD_NAMES,
			FOREIGN_DB_NAMES,
			ASSOCIATION_FOREIGN_FIELD_NAMES,
			ASSOCIATION_FOREIGN_DB_NAMES:
			slice, ok := value.(StrSlice)
			if ok {
				collector.add("\t\t\t%s=%s (%d elements)\n", cachedReverseTagSettingsMap[key], slice, slice.len())
			}
		case RELATION_KIND:
			kind, ok := value.(uint8)
			if ok {
				collector.add("\t\t\t%s=%s\n", cachedReverseTagSettingsMap[key], kindNamesMap[kind])
			}
		case JOIN_TABLE_HANDLER:
			collector.add("\t\t\tHAS %s\n", cachedReverseTagSettingsMap[key])
		default:
			if value == "" {
				collector.add("\t\t\t%s\n", cachedReverseTagSettingsMap[key])
			} else {
				collector.add("\t\t\t%s=%s\n", cachedReverseTagSettingsMap[key], value.(string))
			}
		}

	}
	return collector.String()
}
