package gorm

import (
	"fmt"
	"sync"
)

const (
	//StructField TagSettings constants
	MANY2MANY               uint8 = 1
	INDEX                   uint8 = 2
	NOT_NULL                uint8 = 3
	SIZE                    uint8 = 4
	UNIQUE_INDEX            uint8 = 5
	IS_JOINTABLE_FOREIGNKEY uint8 = 6
	DEFAULT                 uint8 = 7
	EMBEDDED_PREFIX         uint8 = 8
	FOREIGNKEY              uint8 = 9
	ASSOCIATIONFOREIGNKEY   uint8 = 10
	COLUMN                  uint8 = 11
	TYPE                    uint8 = 12
	UNIQUE                  uint8 = 13
	SAVE_ASSOCIATIONS       uint8 = 14
	POLYMORPHIC             uint8 = 15
	POLYMORPHIC_VALUE       uint8 = 16 //was both PolymorphicValue in Relationship struct, and also collected from tags
	POLYMORPHIC_TYPE        uint8 = 17 //was PolymorphicType in Relationship struct
	POLYMORPHIC_DBNAME      uint8 = 18 //was PolymorphicDBName in Relationship struct

	key_not_found_err       string = "TagSetting : COULDN'T FIND KEY FOR %q ON %q"
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
)

var (
	//this is a map for transforming strings into uint8 when reading tags of structs
	tagSettingMap = map[string]uint8{
		many_to_many:            MANY2MANY,
		index:                   INDEX,
		not_null:                NOT_NULL,
		size:                    SIZE,
		unique_index:            UNIQUE_INDEX,
		is_jointable_foreignkey: IS_JOINTABLE_FOREIGNKEY,
		default_str:             DEFAULT,
		embedded_prefix:         EMBEDDED_PREFIX,
		foreignkey:              FOREIGNKEY,
		association_foreign_key: ASSOCIATIONFOREIGNKEY,
		polymorphic:             POLYMORPHIC,
		polymorphic_value:       POLYMORPHIC_VALUE,
		column:                  COLUMN,
		type_str:                TYPE,
		unique:                  UNIQUE,
		save_associations:       SAVE_ASSOCIATIONS,
	}
	cachedReverseTagSettingsMap map[uint8]string
)

//for printing strings instead of uints
func reverseTagSettingsMap() map[uint8]string {
	if cachedReverseTagSettingsMap == nil {
		cachedReverseTagSettingsMap = make(map[uint8]string)
		for k, v := range tagSettingMap {
			cachedReverseTagSettingsMap[v] = k
		}
	}
	return cachedReverseTagSettingsMap
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

//checks if has such a key (for code readability)
func (t *TagSettings) has(named uint8) bool {
	t.l.Lock()
	defer t.l.Unlock()
	//test for a key without retrieving the value
	_, ok := t.Uint8Map[named]
	return ok
}

//gets the string value of a certain key
func (t *TagSettings) get(named uint8) interface{} {
	t.l.Lock()
	defer t.l.Unlock()
	value, ok := t.Uint8Map[named]
	if !ok {
		return nil
	}
	return value
}

func (t *TagSettings) len() int {
	return len(t.Uint8Map)
}

//Stringer implementation
func (t TagSettings) String() string {
	//never inited
	if cachedReverseTagSettingsMap == nil {
		reverseTagSettingsMap()
	}
	result := ""
	for key, value := range t.Uint8Map {
		switch key {
		case ASSOCIATIONFOREIGNKEY, FOREIGNKEY:
			slice, ok := value.(StrSlice)
			if ok {
				result += fmt.Sprintf("%s = %v ; ", cachedReverseTagSettingsMap[key], slice)
			}
		default:
			if value == "" {
				result += fmt.Sprintf("%s ; ", cachedReverseTagSettingsMap[key])
			} else {
				result += fmt.Sprintf("%s = %s ; ", cachedReverseTagSettingsMap[key], value.(string))
			}
		}

	}
	return result
}
