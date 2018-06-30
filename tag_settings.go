package gorm

import (
	"sync"
)

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
		case setAssociationforeignkey,
			setForeignkey,
			setForeignFieldNames,
			setForeignDbNames,
			setAssociationForeignFieldNames,
			setAssociationForeignDbNames:
			slice, ok := value.(StrSlice)
			if ok {
				collector.add("\t\t\t%s=%s (%d strings)\n", cachedReverseTagSettingsMap[key], slice, slice.len())
			}
		case setRelationKind:
			kind, ok := value.(uint8)
			if ok {
				collector.add("\t\t\t%s=%s\n", cachedReverseTagSettingsMap[key], kindNamesMap[kind])
			}
		case setJoinTableHandler:
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
