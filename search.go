package gorm

import "fmt"

const (
	select_query sqlConditionType = 0
	where_query  sqlConditionType = 1
	not_query    sqlConditionType = 2
	or_query     sqlConditionType = 3
	having_query sqlConditionType = 4
	joins_query  sqlConditionType = 5
	init_attrs   sqlConditionType = 6
	assign_attrs sqlConditionType = 7
)

func (s *sqlPair) add(values ... interface{}){
	s.args = append(s.args, values...)
}

func (s *search) clone() *search {
	//TODO : @Badu - it's this a ... clone ?
	clone := *s
	return &clone
}

func (s *search) Where(query interface{}, values ...interface{}) *search {
	s.whereConditions = append(s.whereConditions, map[string]interface{}{"query": query, "args": values})
	entry, ok := s.conditions[where_query]
	if !ok {
		entry = make(sqlCondition, 0)
	}
	newPair := sqlPair{expression: query}
	newPair.add(values)
	entry = append(entry, newPair)
	return s
}

func (s *search) Not(query interface{}, values ...interface{}) *search {
	s.notConditions = append(s.notConditions, map[string]interface{}{"query": query, "args": values})

	entry, ok := s.conditions[not_query]
	if !ok {
		entry = make(sqlCondition, 0)
	}
	newPair := sqlPair{expression: query}
	newPair.add(values)
	entry = append(entry, newPair)
	return s
}

func (s *search) Or(query interface{}, values ...interface{}) *search {
	s.orConditions = append(s.orConditions, map[string]interface{}{"query": query, "args": values})

	entry, ok := s.conditions[or_query]
	if !ok {
		entry = make(sqlCondition, 0)
	}
	newPair := sqlPair{expression: query}
	newPair.add(values)
	entry = append(entry, newPair)
	return s
}

func (s *search) Having(query string, values ...interface{}) *search {
	s.havingConditions = append(s.havingConditions, map[string]interface{}{"query": query, "args": values})

	entry, ok := s.conditions[having_query]
	if !ok {
		entry = make(sqlCondition, 0)
	}
	newPair := sqlPair{expression: query}
	newPair.add(values)
	entry = append(entry, newPair)
	return s
}

func (s *search) Joins(query string, values ...interface{}) *search {
	s.joinConditions = append(s.joinConditions, map[string]interface{}{"query": query, "args": values})

	entry, ok := s.conditions[joins_query]
	if !ok {
		entry = make(sqlCondition, 0)
	}
	newPair := sqlPair{expression: query}
	newPair.add(values)
	entry = append(entry, newPair)
	return s
}

func (s *search) Select(query interface{}, args ...interface{}) *search {
	s.selects = map[string]interface{}{"query": query, "args": args}

	entry, ok := s.conditions[select_query]
	if !ok {
		entry = make(sqlCondition, 0)
	}
	newPair := sqlPair{expression: query}
	newPair.add(args)
	entry = append(entry, newPair)
	return s
}

func (s *search) Attrs(attrs ...interface{}) *search {
	s.initAttrs = append(s.initAttrs, toSearchableMap(attrs...))
	return s
}

func (s *search) Assign(attrs ...interface{}) *search {
	s.assignAttrs = append(s.assignAttrs, toSearchableMap(attrs...))
	return s
}

func (s *search) Order(value interface{}, reorder ...bool) *search {
	if len(reorder) > 0 && reorder[0] {
		s.orders = []interface{}{}
	}

	if value != nil {
		s.orders = append(s.orders, value)
	}
	return s
}

func (s *search) Omit(columns ...string) *search {
	s.omits = columns
	return s
}

func (s *search) Limit(limit interface{}) *search {
	s.limit = limit
	return s
}

func (s *search) Offset(offset interface{}) *search {
	s.offset = offset
	return s
}

func (s *search) Group(query string) *search {
	s.group = s.getInterfaceAsSQL(query)
	return s
}

func (s *search) Preload(schema string, values ...interface{}) *search {
	var preloads []searchPreload
	for _, preload := range s.preload {
		if preload.schema != schema {
			preloads = append(preloads, preload)
		}
	}
	preloads = append(preloads, searchPreload{schema, values})
	s.preload = preloads
	return s
}

func (s *search) Raw(b bool) *search {
	s.raw = b
	return s
}

func (s *search) unscoped() *search {
	s.Unscoped = true
	return s
}

func (s *search) Table(name string) *search {
	s.tableName = name
	return s
}

func (s *search) getInterfaceAsSQL(value interface{}) (str string) {
	switch value.(type) {
	case string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		str = fmt.Sprintf("%v", value)
	default:
		s.con.AddError(ErrInvalidSQL)
	}

	if str == "-1" {
		return ""
	}
	return
}

func (s *search) collectAttrs() *[]string {
	attrs := []string{}
	for _, value := range s.selects {
		switch strs := value.(type) {
		case string:
			attrs = append(attrs, strs)
		case []string:
			attrs = append(attrs, strs...)
		case []interface{}:
			for _, str := range strs {
				attrs = append(attrs, fmt.Sprintf("%v", str))
			}
		}
	}
	return &attrs
}
