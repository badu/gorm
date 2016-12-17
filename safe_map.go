package gorm

import (
	"bytes"
	"strings"
)

func (s *safeMap) set(key string, value string) {
	s.l.Lock()
	defer s.l.Unlock()
	s.m[key] = value
}

func (s *safeMap) get(key string) string {
	s.l.RLock()
	defer s.l.RUnlock()
	//If the requested key doesn't exist, we get the value type's zero value ("")
	return s.m[key]
}

// ToDBName convert string to db name
func (smap *safeMap) toDBName(name string) string {
	//attempt to retrieve it from map
	if v := smap.get(name); v != "" {
		return v
	}

	if name == "" {
		return ""
	}
	//building it
	var (
		value                        = commonInitialismsReplacer.Replace(name)
		buf                          = bytes.NewBufferString("")
		lastCase, currCase, nextCase strCase
	)

	for i, v := range value[:len(value)-1] {
		nextCase = strCase(value[i+1] >= 'A' && value[i+1] <= 'Z')
		if i > 0 {
			if currCase == upper {
				if lastCase == upper && nextCase == upper {
					buf.WriteRune(v)
				} else {
					if value[i-1] != '_' && value[i+1] != '_' {
						buf.WriteRune('_')
					}
					buf.WriteRune(v)
				}
			} else {
				buf.WriteRune(v)
			}
		} else {
			currCase = upper
			buf.WriteRune(v)
		}
		lastCase = currCase
		currCase = nextCase
	}

	buf.WriteByte(value[len(value)-1])

	s := strings.ToLower(buf.String())
	//store it to the map
	smap.set(name, s)
	return s
}
