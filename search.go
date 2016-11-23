package gorm

import (
	"strings"
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
	omits_query   sqlConditionType = 10

	IS_UNSCOPED uint16 = 0
	IS_RAW      uint16 = 1
	IS_COUNTING uint16 = 2

	HAS_SELECT  uint16 = 3
	HAS_WHERE   uint16 = 4
	HAS_NOT     uint16 = 5
	HAS_OR      uint16 = 6
	HAS_HAVING  uint16 = 7
	HAS_JOINS   uint16 = 8
	HAS_INIT    uint16 = 9
	HAS_ASSIGN  uint16 = 10
	HAS_PRELOAD uint16 = 11
	HAS_ORDER   uint16 = 12
	HAS_OMITS   uint16 = 13
)

// Expr generate raw SQL expression, for example:
//     DB.Model(&product).Update("price", gorm.SqlPair("price * ? + ?", 2, 100))
func SqlExpr(expression interface{}, args ...interface{}) *SqlPair {
	return &SqlPair{expression: expression, args: args}
}

//TODO : @Badu - move some pieces of code from Scope AddToVars, orderSQL, updatedAttrsWithValues
//TODO : @Badu - make expr string bytes buffer, allow args to be added, allow bytes buffer to be written into
//TODO : @Badu - before doing above, benchmark bytesbuffer versus string concat
/**
var buf bytes.Buffer
var prParams []interface{}
if p.Id > 0 {
	buf.WriteString("%q:%d,")
	prParams = append(prParams, "id")
	prParams = append(prParams, p.Id)
}
buf.WriteString("%q:%q,%q:%q,%q:%t,%q:{%v}")
prParams = append(prParams, "name")
prParams = append(prParams, p.DisplayName)
prParams = append(prParams, "states")
prParams = append(prParams, p.USStates)
prParams = append(prParams, "customerPays")
prParams = append(prParams, p.AppliesToCustomer)
prParams = append(prParams, "price")
prParams = append(prParams, p.Price)
return fmt.Sprintf(buf.String(), prParams...)
*/
//TODO : @Badu - use it to build strings with multiple fmt.Sprintf calls - making one call

func (p *SqlPair) addExpressions(values ...interface{}) {
	p.args = append(p.args, values...)
}

func (p *SqlPair) strExpr() string {
	result := p.expression.(string)
	return result
}

func (s *Search) getFirst(condType sqlConditionType) *SqlPair {
	s.checkInit(condType)
	//should return the number of conditions of that type
	numConditions := len(s.Conditions[condType])
	if numConditions != 1 {
		//err := fmt.Errorf("Search getFirst : %d should have exactly one item in slice, but has %d", condType, numConditions)
		//fmt.Println(err)
		//if s.con != nil {
		//	s.con.AddError(err)
		//}
		return nil
	}
	return &s.Conditions[condType][0]
}

func (s *Search) checkInit(condType sqlConditionType) {
	//create a slice of conditions for the key of map if there isn't already one
	if _, ok := s.Conditions[condType]; !ok {
		s.Conditions[condType] = make([]SqlPair, 0, 0)
	}
}

func (s *Search) Preload(schema string, values ...interface{}) *Search {
	//Note to self : order matters here : if you attempt to replace the existing item,
	//logic will break - as in many many places
	s.checkInit(preload_query)
	//overriding sql pairs within the same schema
	for i, pair := range s.Conditions[preload_query] {
		if pair.strExpr() == schema {
			//delete from slice
			s.Conditions[preload_query] = append(s.Conditions[preload_query][:i], s.Conditions[preload_query][i+1:]...)
		}
	}
	//add preload
	newPair := SqlPair{expression: schema}
	newPair.addExpressions(values...)
	//add the condition pair to the slice
	s.Conditions[preload_query] = append(s.Conditions[preload_query], newPair)
	s.setFlag(HAS_PRELOAD)
	return s
}

func (s *Search) addSqlCondition(condType sqlConditionType, query interface{}, values ...interface{}) {
	//TODO : @Badu - VERY IMPORTANT : check in which condition we clone the search,
	//otherwise slice will grow indefinitely ( causing memory leak :) )
	s.checkInit(condType)
	//create a new condition pair
	newPair := SqlPair{expression: query}
	newPair.addExpressions(values...)
	//add the condition pair to the slice
	s.Conditions[condType] = append(s.Conditions[condType], newPair)
}

func (s *Search) Clone() *Search {
	clone := Search{}

	clone.flags = s.flags
	//clone conditions
	clone.Conditions = make(SqlConditions)
	for key, value := range s.Conditions {
		clone.Conditions[key] = value
	}
	clone.offset = s.offset
	clone.limit = s.limit
	clone.group = s.group
	clone.tableName = s.tableName
	return &clone
}

func (s *Search) Where(query interface{}, values ...interface{}) *Search {
	s.addSqlCondition(Where_query, query, values...)
	s.setFlag(HAS_WHERE)
	return s
}

func (s *Search) Not(query interface{}, values ...interface{}) *Search {
	s.addSqlCondition(not_query, query, values...)
	s.setFlag(HAS_NOT)
	return s
}

func (s *Search) Or(query interface{}, values ...interface{}) *Search {
	s.addSqlCondition(or_query, query, values...)
	s.setFlag(HAS_OR)
	return s
}

func (s *Search) Having(query string, values ...interface{}) *Search {
	s.addSqlCondition(having_query, query, values...)
	s.setFlag(HAS_HAVING)
	return s
}

func (s *Search) Joins(query string, values ...interface{}) *Search {
	s.addSqlCondition(joins_query, query, values...)
	s.setFlag(HAS_JOINS)
	return s
}

func (s *Search) Select(query interface{}, args ...interface{}) *Search {
	s.addSqlCondition(Select_query, query, args...)
	s.setFlag(HAS_SELECT)
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
		s.setFlag(HAS_INIT)
	}
	return s
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
		s.setFlag(HAS_ASSIGN)
	}
	return s
}

func (s *Search) Order(value interface{}, reorder ...bool) *Search {
	if len(reorder) > 0 && reorder[0] {
		//reseting existing entry
		s.Conditions[Order_query] = make([]SqlPair, 0, 0)
	}
	if value != nil {
		s.addSqlCondition(Order_query, nil, value)
		s.setFlag(HAS_ORDER)
	}
	return s
}

func (s *Search) Omit(columns ...string) *Search {
	s.checkInit(omits_query)
	//add omit
	newPair := SqlPair{}
	//transfer slices (copy) - strings to interface
	newPair.args = make([]interface{}, len(columns))
	for i, v := range columns {
		newPair.args[i] = v
	}
	//add the condition pair to the slice
	s.Conditions[omits_query] = append(s.Conditions[omits_query], newPair)
	//fmt.Printf("Omit %d elements\n", s.numConditions(omits_query))
	s.setFlag(HAS_OMITS)
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
	s.group = query
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

func (s *Search) hasSelect() bool {
	return s.flags&(1<<HAS_SELECT) != 0
}

func (s *Search) hasJoins() bool {
	return s.flags&(1<<HAS_JOINS) != 0
}

func (s *Search) hasOrder() bool {
	return s.flags&(1<<HAS_ORDER) != 0
}

func (s *Search) hasAssign() bool {
	return s.flags&(1<<HAS_ASSIGN) != 0
}

func (s *Search) hasPreload() bool {
	return s.flags&(1<<HAS_PRELOAD) != 0
}

func (s *Search) hasHaving() bool {
	return s.flags&(1<<HAS_HAVING) != 0
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

func (s *Search) GetInitAttr() ([]interface{}, bool) {
	pair := s.getFirst(Init_attrs)
	if pair == nil {
		return nil, false
	}
	return pair.args, true
}

func (s *Search) GetAssignAttr() ([]interface{}, bool) {
	pair := s.getFirst(assign_attrs)
	if pair == nil {
		return nil, false
	}
	return pair.args, true
}

func (s *Search) checkFieldIncluded(field *StructField) bool {
	for _, pair := range s.Conditions[Select_query] {
		switch strs := pair.expression.(type) {
		case string:
			if field.GetName() == strs || field.DBName == strs {
				//fmt.Printf("[str] Field %q included\n", strs)
				return true
			}

		case []string:
			for _, o := range strs {
				if field.GetName() == o || field.DBName == o {
					//fmt.Printf("[slice] Field %q included\n", o)
					return true
				}
			}
		}

		for _, pairArg := range pair.args {
			if field.GetName() == pairArg || field.DBName == pairArg {
				//fmt.Printf("[arg] Field %q included\n", pairArg)
				return true
			}
		}
	}
	return false
}

func (s *Search) checkFieldOmitted(field *StructField) bool {
	pair := s.getFirst(omits_query)
	if pair == nil {
		return false
	}
	for _, attr := range pair.args {
		if field.GetName() == attr || field.DBName == attr {
			//fmt.Printf("Field %q omitted\n", attr.(string))
			return true
		}
	}
	return false
}

// AddToVars add value as sql's vars, used to prevent SQL injection
func (s *Search) AddToVars(value interface{}, dialect Dialect) string {
	if pair, ok := value.(*SqlPair); ok {
		exp := pair.strExpr()
		for _, arg := range pair.args {
			exp = strings.Replace(exp, "?", s.AddToVars(arg, dialect), 1)
		}
		return exp
	}
	s.SQLVars = append(s.SQLVars, value)
	return dialect.BindVar(len(s.SQLVars))
}
