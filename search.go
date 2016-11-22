package gorm

import (
	"fmt"
)

const (
	Select_query  sqlConditionType = 0
	Where_query   sqlConditionType = 1
	not_query     sqlConditionType = 2
	or_query      sqlConditionType = 3
	having_query  sqlConditionType = 4
	joins_query   sqlConditionType = 5
	Init_attrs    sqlConditionType = 6
	assign_attrs  sqlConditionType = 7
	preload_query sqlConditionType = 8
	Order_query   sqlConditionType = 9

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

func (s *Search) getFirst(condType sqlConditionType) *sqlPair {
	numConditions := s.numConditions(condType)
	if numConditions != 1 {
		err := fmt.Errorf("Search getFirst : %d should have exactly one item in slice, but has %d", condType, numConditions)
		fmt.Println(err)
		if s.con != nil {
			s.con.AddError(err)
		}
		return nil
	}
	return &s.Conditions[condType][0]
}

func (s *Search) checkInit(condType sqlConditionType) {
	//create a slice of conditions for the key of map if there isn't already one
	if _, ok := s.Conditions[condType]; !ok {
		s.Conditions[condType] = make([]sqlPair, 0, 0)
	}
}

func (s *Search) Preload(schema string, values ...interface{}) *Search {
	s.checkInit(preload_query)
	//overriding sql pairs within the same schema
	for i, pair := range s.Conditions[preload_query] {
		if pair.strExpr() == schema {
			//delete from slice
			s.Conditions[preload_query] = append(s.Conditions[preload_query][:i], s.Conditions[preload_query][i+1:]...)
		}
	}
	//add preload
	newPair := sqlPair{expression: schema}
	newPair.addExpressions(values...)
	//add the condition pair to the slice
	s.Conditions[preload_query] = append(s.Conditions[preload_query], newPair)
	return s
}

func (s *Search) addSqlCondition(condType sqlConditionType, query interface{}, values ...interface{}) {
	//TODO : @Badu - VERY IMPORTANT : check in which condition we clone the search,
	//otherwise slice will grow indefinitely ( causing memory leak :) )
	s.checkInit(condType)
	//create a new condition pair
	newPair := sqlPair{expression: query}
	newPair.addExpressions(values...)
	//add the condition pair to the slice
	s.Conditions[condType] = append(s.Conditions[condType], newPair)
}

func (s *Search) numConditions(condType sqlConditionType) int {
	s.checkInit(condType)
	//should return the number of conditions of that type
	return len(s.Conditions[condType])
}

func (s *Search) Clone() *Search {
	clone := Search{}

	clone.flags = s.flags
	//clone conditions
	clone.Conditions = make(SqlConditions)
	for key, value := range s.Conditions {
		clone.Conditions[key] = value
	}
	clone.omits = s.omits
	clone.offset = s.offset
	clone.limit = s.limit
	clone.group = s.group
	clone.tableName = s.tableName
	return &clone
}

func (s *Search) Where(query interface{}, values ...interface{}) *Search {
	s.addSqlCondition(Where_query, query, values...)
	return s
}

func (s *Search) Not(query interface{}, values ...interface{}) *Search {
	s.addSqlCondition(not_query, query, values...)
	return s
}

func (s *Search) Or(query interface{}, values ...interface{}) *Search {
	s.addSqlCondition(or_query, query, values...)
	return s
}

func (s *Search) Having(query string, values ...interface{}) *Search {
	s.addSqlCondition(having_query, query, values...)
	return s
}

func (s *Search) Joins(query string, values ...interface{}) *Search {
	s.addSqlCondition(joins_query, query, values...)
	return s
}

func (s *Search) Select(query interface{}, args ...interface{}) *Search {
	s.addSqlCondition(Select_query, query, args...)
	return s
}

func (s *Search) Attrs(attrs ...interface{}) *Search {
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
		s.addSqlCondition(Init_attrs, nil, result)
	}
	return s
}

//TODO : @Badu - move this where is called
func (s *Search) GetInitAttr() ([]interface{}, bool) {
	pair := s.getFirst(Init_attrs)
	if pair == nil {
		return nil, false
	}
	return pair.args, true
}

//TODO : @Badu - move this where is called
func (s *Search) GetAssignAttr() ([]interface{}, bool) {
	pair := s.getFirst(assign_attrs)
	if pair == nil {
		return nil, false
	}
	return pair.args, true
}

func (s *Search) Assign(attrs ...interface{}) *Search {
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

func (s *Search) Order(value interface{}, reorder ...bool) *Search {
	if len(reorder) > 0 && reorder[0] {
		//reseting existing entry
		s.Conditions[Order_query] = make([]sqlPair, 0, 0)
	}
	if value != nil {
		s.addSqlCondition(Order_query, nil, value)
	}
	return s
}

func (s *Search) Omit(columns ...string) *Search {
	s.omits = columns
	return s
}

func (s *Search) Limit(limit interface{}) *Search {
	s.limit = limit
	return s
}

func (s *Search) Offset(offset interface{}) *Search {
	s.offset = offset
	return s
}

func (s *Search) Group(query string) *Search {
	s.group = s.getInterfaceAsSQL(query)
	return s
}

func (s Search) hasFlag(value uint16) bool {
	return s.flags&(1<<value) != 0
}

func (s *Search) setFlag(value uint16) {
	s.flags = s.flags | (1 << value)
}

func (s *Search) unsetFlag(value uint16) {
	s.flags = s.flags & ^(1 << value)
}

func (s *Search) isCounting() bool {
	return s.flags&(1<<IS_COUNTING) != 0
}

func (s *Search) setCounting() *Search {
	s.flags = s.flags | (1 << IS_COUNTING)
	return s
}

func (s *Search) IsRaw() bool {
	return s.flags&(1<<IS_RAW) != 0
}

func (s *Search) SetRaw() *Search {
	s.flags = s.flags | (1 << IS_RAW)
	return s
}

func (s *Search) isUnscoped() bool {
	return s.flags&(1<<IS_UNSCOPED) != 0
}

func (s *Search) setUnscoped() *Search {
	s.flags = s.flags | (1 << IS_UNSCOPED)
	return s
}

func (s *Search) Table(name string) *Search {
	s.tableName = name
	return s
}

func (s *Search) getInterfaceAsSQL(value interface{}) (str string) {
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

func (s *Search) collectAttrs() *[]string {
	attrs := []string{}
	for _, pair := range s.Conditions[Select_query] {
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
