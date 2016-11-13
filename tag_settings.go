package gorm

import (
	"errors"
	"fmt"
	"strings"
)

const (
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
	SAVE_ASSOCIATIONS       uint8 = 20
)

var (
	//TODO : @Badu - make this concurrent map
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
		"SAVE_ASSOCIATIONS":       SAVE_ASSOCIATIONS,
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

func (ts *TagSettings) loadFromTags(str string) error {
	tags := strings.Split(str, ";")
	for _, value := range tags {
		v := strings.Split(value, ":")
		if len(v) > 0 {
			k := strings.TrimSpace(strings.ToUpper(v[0]))
			//avoid empty keys : original gorm didn't mind creating them
			if k != "" {
				uint8Key, ok := tagSettingMap[k]
				if ok {
					if len(v) >= 2 {
						ts.set(uint8Key, strings.Join(v[1:], ":"))
					} else {
						ts.set(uint8Key, k)
					}
				} else {
					return errors.New(fmt.Sprintf("TagSetting : COULDN'T FIND KEY FOR %q ON %q", k, str))
				}
			}
		}

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
