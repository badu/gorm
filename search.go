package gorm

import (
	"fmt"
)

const (
	select_query  sqlConditionType = 0
	where_query   sqlConditionType = 1
	not_query     sqlConditionType = 2
	or_query      sqlConditionType = 3
	having_query  sqlConditionType = 4
	joins_query   sqlConditionType = 5
	init_attrs    sqlConditionType = 6
	assign_attrs  sqlConditionType = 7
	preload_query sqlConditionType = 8
	order_query   sqlConditionType = 9

	IS_UNSCOPED uint16 = 0
	IS_RAW      uint16 = 1
	IS_COUNTING uint16 = 2
)

func (p *sqlPair) addExpressions(values ...interface{}) {
	p.args = append(p.args, values...)
}

func (p *sqlPair) strExpr() string {
	result := p.expression.(string)
	return result
}

func (s *search) checkInit(condType sqlConditionType) {
	if s.conditions == nil {
		s.conditions = make(sqlConditions)
	}
	//create a slice of conditions for the key of map if there isn't already one
	if _, ok := s.conditions[condType]; !ok {
		s.conditions[condType] = make([]sqlPair, 0, 0)
	}
}

func (s *search) Preload(schema string, values ...interface{}) *search {
	s.checkInit(preload_query)
	//overriding sql pairs within the same schema
	for i, pair := range s.conditions[preload_query] {
		if pair.strExpr() == schema {
			//delete from slice
			s.conditions[preload_query] = append(s.conditions[preload_query][:i], s.conditions[preload_query][i+1:]...)
		}
	}
	//add preload
	newPair := sqlPair{expression: schema}
	newPair.addExpressions(values...)
	//add the condition pair to the slice
	s.conditions[preload_query] = append(s.conditions[preload_query], newPair)
	return s
}

func (s *search) addSqlCondition(condType sqlConditionType, query interface{}, values ...interface{}) {
	//TODO : @Badu - VERY IMPORTANT : check in which condition we clone the search,
	//otherwise slice will grow indefinitely ( causing memory leak :) )
	s.checkInit(condType)
	//create a new condition pair
	newPair := sqlPair{expression: query}
	newPair.addExpressions(values...)
	//add the condition pair to the slice
	s.conditions[condType] = append(s.conditions[condType], newPair)
}

func (s *search) numConditions(condType sqlConditionType) int {
	s.checkInit(condType)
	//should return the number of conditions of that type
	return len(s.conditions[condType])
}

func (s *search) clone() *search {
	//TODO : @Badu - make it a true clone, after you finish cleanup of fields!!!
	clone := *s
	//clone conditions
	clone.conditions = make(sqlConditions)
	clone.flags = s.flags
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
	s.addSqlCondition(select_query, query, args...)
	return s
}

func (s *search) Attrs(attrs ...interface{}) *search {
	var result interface{}
	if len(attrs) == 1 {
		if attr, ok := attrs[0].(map[string]interface{}); ok {
			result = attr
		}

		if attr, ok := attrs[0].(interface{}); ok {
			result = attr
		}
	} else if len(attrs) > 1 {
		if str, ok := attrs[0].(string); ok {
			result = map[string]interface{}{str: attrs[1]}
		}
	}
	if result != nil {
		s.addSqlCondition(init_attrs, nil, result)
	}
	return s
}

//TODO : @Badu - make getter of first item in slice method - since most conditions have exactly one
//@see select_query logic
func (s *search) GetInitAttr() ([]interface{}, bool) {
	if s.numConditions(init_attrs) == 1 {
		assign := s.conditions[init_attrs][0]
		return assign.args, true
	}
	return nil, false
}

func (s *search) GetAssignAttr() ([]interface{}, bool) {
	if s.numConditions(assign_attrs) == 1 {
		assign := s.conditions[assign_attrs][0]
		return assign.args, true
	}
	return nil, false
}

func (s *search) Assign(attrs ...interface{}) *search {
	var result interface{}
	if len(attrs) == 1 {
		if attr, ok := attrs[0].(map[string]interface{}); ok {
			result = attr
		}

		if attr, ok := attrs[0].(interface{}); ok {
			result = attr
		}
	} else if len(attrs) > 1 {
		if str, ok := attrs[0].(string); ok {
			result = map[string]interface{}{str: attrs[1]}
		}
	}
	if result != nil {
		s.addSqlCondition(assign_attrs, nil, result)
	}
	return s
}

func (s *search) Order(value interface{}, reorder ...bool) *search {
	if len(reorder) > 0 && reorder[0] {
		s.conditions[order_query] = make([]sqlPair, 0, 0)
	}
	if value != nil {
		s.addSqlCondition(order_query, nil, value)
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

func (s search) hasFlag(value uint16) bool {
	return s.flags&(1<<value) != 0
}

func (s *search) setFlag(value uint16) {
	s.flags = s.flags | (1 << value)
}

func (s *search) unsetFlag(value uint16) {
	s.flags = s.flags & ^(1 << value)
}

func (s *search) isCounting() bool {
	return s.flags&(1<<IS_COUNTING) != 0
}

func (s *search) setCounting() *search {
	s.flags = s.flags | (1 << IS_COUNTING)
	return s
}

func (s *search) IsRaw() bool {
	return s.flags&(1<<IS_RAW) != 0
}

func (s *search) SetRaw() *search {
	s.flags = s.flags | (1 << IS_RAW)
	return s
}

func (s *search) isUnscoped() bool {
	return s.flags&(1<<IS_UNSCOPED) != 0
}

func (s *search) setUnscoped() *search {
	s.flags = s.flags | (1 << IS_UNSCOPED)
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
	for _, pair := range s.conditions[select_query] {
		switch strs := pair.expression.(type) {
		case string:
			attrs = append(attrs, strs)
		case []string:
			attrs = append(attrs, strs...)
		}
		for _, pairArg := range pair.args {
			attrs = append(attrs, fmt.Sprintf("%v", pairArg))
		}
	}

	return &attrs
}
