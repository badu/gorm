package gorm

import (
	"errors"
	"fmt"
	"strings"
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
	POLYMORPHIC             uint8 = 11
	POLYMORPHIC_VALUE       uint8 = 12
	COLUMN                  uint8 = 13
	TYPE                    uint8 = 14
	UNIQUE                  uint8 = 15
	SAVE_ASSOCIATIONS       uint8 = 16

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
	//TODO : @Badu - make this concurrent map
	//this is a map for transforming strings into uint8 when reading tags of structs
	//@See : &StructField{}.ParseTagSettings()
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

func (ts *TagSettings) loadFromTags(field *StructField, str string) error {
	tags := strings.Split(str, ";")

	for _, value := range tags {
		v := strings.Split(value, ":")
		if len(v) > 0 {
			k := strings.TrimSpace(strings.ToUpper(v[0]))
			//avoid empty keys : original gorm didn't mind creating them
			if k != "" {
				//setting some flags directly
				switch k {
				case ignored:
					field.setFlag(IS_IGNORED)
				case primary_key:
					field.setFlag(IS_PRIMARYKEY)
				case auto_increment:
					field.setFlag(IS_AUTOINCREMENT)
				case embedded:
					field.setFlag(IS_EMBED_OR_ANON)
				default:
					if k == default_str {
						field.setFlag(HAS_DEFAULT_VALUE)
					}
					uint8Key, ok := tagSettingMap[k]
					if ok {
						if len(v) >= 2 {
							ts.set(uint8Key, strings.Join(v[1:], ":"))
						} else {
							ts.set(uint8Key, k)
						}
					} else {
						return errors.New(fmt.Sprintf(key_not_found_err, k, str))
					}
				}

			}
		}

	}
	if field.IsAutoIncrement() && !field.IsPrimaryKey() {
		field.setFlag(HAS_DEFAULT_VALUE)
	}
	return nil
}

//returns a clone of tag settings (used in cloning StructField)
func (t *TagSettings) clone() TagSettings {
	clone := TagSettings{Uint8Map: make(map[uint8]string)}
	for key, value := range t.Uint8Map {
		clone.Uint8Map[key] = value
	}
	return clone
}

//deletes a key from the settings map
func (t *TagSettings) unset(named uint8) {
	delete(t.Uint8Map, named)
}

//adds a key to the settings map
func (t *TagSettings) set(named uint8, value string) {
	t.Uint8Map[named] = value
}

//checks if has such a key (for code readability)
func (t *TagSettings) has(named uint8) bool {
	//test for a key without retrieving the value
	_, ok := t.Uint8Map[named]
	return ok
}

//gets the string value of a certain key
func (t *TagSettings) get(named uint8) string {
	value, ok := t.Uint8Map[named]
	if !ok {
		return ""
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
		if value == "" {
			result += fmt.Sprintf("%q ; ", cachedReverseTagSettingsMap[key])
		} else {
			result += fmt.Sprintf("%q = %q ; ", cachedReverseTagSettingsMap[key], value)
		}
	}
	return result
}
