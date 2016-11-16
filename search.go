package gorm

import (
	"fmt"
)

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

func (p *sqlPair) addExpressions(values ...interface{}) {
	p.args = append(p.args, values...)
}

func (s *search) addSqlCondition(condType sqlConditionType, query interface{}, values ...interface{}) {
	//TODO : @Badu - just until we get stable with this
	if s.conditions == nil {
		s.conditions = make(sqlConditions)
	}
	//create a new condition pair
	newPair := sqlPair{expression: query}
	newPair.addExpressions(values...)
	//create a slice of conditions for the key of map if there isn't already one
	if _, ok := s.conditions[condType]; !ok {
		s.conditions[condType] = make([]sqlPair, 0, 0)
	}
	//add the condition pair to the slice
	s.conditions[condType] = append(s.conditions[condType], newPair)
}

func (s *search) numConditions(condType sqlConditionType) int {
	//TODO : @Badu - just until we get stable with this
	if s.conditions == nil {
		s.conditions = make(sqlConditions)
	}
	if _, ok := s.conditions[condType]; !ok {
		s.conditions[condType] = make([]sqlPair, 0, 0)
	}
	//should return the number of conditions of that type
	return len(s.conditions[condType])
}

func (s *search) clone() *search {
	//TODO : @Badu - it's this a ... clone ?
	clone := *s
	//clone conditions
	clone.conditions = make(sqlConditions)
	for key, value := range s.conditions {
		clone.conditions[key] = value
	}
	return &clone
}

func (s *search) Where(query interface{}, values ...interface{}) *search {
	s.addSqlCondition(where_query, query, values...)
	return s
}

func (s *search) Not(query interface{}, values ...interface{}) *search {
	s.notConditions = append(s.notConditions, map[string]interface{}{"query": query, "args": values})
	s.addSqlCondition(not_query, query, values...)
	return s
}

func (s *search) Or(query interface{}, values ...interface{}) *search {
	s.addSqlCondition(or_query, query, values...)
	return s
}

func (s *search) Having(query string, values ...interface{}) *search {
	s.addSqlCondition(having_query, query, values...)
	return s
}

func (s *search) Joins(query string, values ...interface{}) *search {
	s.addSqlCondition(joins_query, query, values...)
	return s
}

func (s *search) Select(query interface{}, args ...interface{}) *search {
	s.selects = map[string]interface{}{"query": query, "args": args}
	s.addSqlCondition(select_query, query, args...)
	return s
}

func (s *search) Attrs(attrs ...interface{}) *search {
	s.initAttrs = append(s.initAttrs, toSearchableMap(attrs...))
	s.addSqlCondition(init_attrs, nil, attrs...)
	return s
}

func (s *search) Assign(attrs ...interface{}) *search {
	s.assignAttrs = append(s.assignAttrs, toSearchableMap(attrs...))
	s.addSqlCondition(assign_attrs, nil, attrs...)
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
		//TODO: @Badu - separate string situation and print integers as integers
		str = fmt.Sprintf("%v", value)
	default:
		s.con.AddError(ErrInvalidSQL)
	}
	//TODO : @Badu - this is from limit and offset. Kind of boilerplate, huh?
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
