package postgres

import (
	"database/sql"
	"database/sql/driver"

	_ "github.com/lib/pq"
	"github.com/lib/pq/hstore"
)

type Hstore map[string]*string

// Value get value of Hstore
func (h Hstore) Value() (driver.Value, error) {
	hstoreMap := hstore.Hstore{Map: map[string]sql.NullString{}}
	if len(h) == 0 {
		return nil, nil
	}

	for key, value := range h {
		var s sql.NullString
		if value != nil {
			s.String = *value
			s.Valid = true
		}
		hstoreMap.Map[key] = s
	}
	return hstoreMap.Value()
}

// Scan scan value into Hstore
func (h *Hstore) Scan(value interface{}) error {
	hstoreMap := hstore.Hstore{}

	if err := hstoreMap.Scan(value); err != nil {
		return err
	}

	if len(hstoreMap.Map) == 0 {
		return nil
	}

	*h = Hstore{}
	for k := range hstoreMap.Map {
		elem := hstoreMap.Map[k]
		if elem.Valid {
			s := hstoreMap.Map[k].String
			(*h)[k] = &s
		} else {
			(*h)[k] = nil
		}
	}

	return nil
}
