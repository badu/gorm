package gorm

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Expr generate raw SQL expression, for example:
//     DB.Model(&product).Update("price", gorm.SqlPair("price * ? + ?", 2, 100))
func SqlExpr(expression interface{}, args ...interface{}) *SqlPair {
	return &SqlPair{expression: expression, args: args}
}

func (p *SqlPair) addExpressions(values ...interface{}) {
	p.args = append(p.args, values...)
}

func (p *SqlPair) strExpr() string {
	result, ok := p.expression.(string)
	if ok {
		return result
	}
	fmt.Printf("ERROR : SqlPair expression %v expected to be string. It's not!\n", p.expression)
	return ""
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
	s.checkInit(cond_preload_query)
	//overriding sql pairs within the same schema
	for i, pair := range s.Conditions[cond_preload_query] {
		if pair.strExpr() == schema {
			//delete from slice
			s.Conditions[cond_preload_query] = append(s.Conditions[cond_preload_query][:i], s.Conditions[cond_preload_query][i+1:]...)
		}
	}
	//add preload
	newPair := SqlPair{expression: schema}
	newPair.addExpressions(values...)
	//add the condition pair to the slice
	s.Conditions[cond_preload_query] = append(s.Conditions[cond_preload_query], newPair)
	s.setFlag(srch_has_preload)
	return s
}

func (s *Search) addSqlCondition(condType sqlConditionType, query interface{}, values ...interface{}) {
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
	clone.tableName = s.tableName
	clone.Value = s.Value
	return &clone
}

func (s *Search) clone(value interface{}) *Search {
	clone := Search{}
	clone.flags = s.flags
	//clone conditions
	clone.Conditions = make(SqlConditions)
	for key, value := range s.Conditions {
		clone.Conditions[key] = value
	}
	clone.tableName = s.tableName
	clone.Value = value
	return &clone
}

func (s *Search) Wheres(wheres ...interface{}) *Search {
	if len(wheres) > 0 {
		s.addSqlCondition(cond_where_query, wheres[0], wheres[1:]...)
		s.setFlag(srch_has_where)
	}
	return s
}

func (s *Search) initialize(scope *Scope) {
	for _, pair := range s.Conditions[cond_where_query] {
		updatedAttrsWithValues(scope, pair.expression)
	}
	initArgs := s.getFirst(cond_init_attrs)
	if initArgs != nil {
		updatedAttrsWithValues(scope, initArgs.args)
	}
	args := s.getFirst(cond_assign_attrs)
	if args != nil {
		updatedAttrsWithValues(scope, args.args)
	}
}

func (s *Search) Where(query interface{}, values ...interface{}) *Search {
	s.addSqlCondition(cond_where_query, query, values...)
	//fmt.Printf(fullFileWithLineNum())
	//fmt.Printf("WHERE %v %#v\n", query, values)
	s.setFlag(srch_has_where)
	return s
}

func (s *Search) Not(query interface{}, values ...interface{}) *Search {
	s.addSqlCondition(cond_not_query, query, values...)
	s.setFlag(srch_has_not)
	return s
}

func (s *Search) Or(query interface{}, values ...interface{}) *Search {
	s.addSqlCondition(cond_or_query, query, values...)
	s.setFlag(srch_has_or)
	return s
}

func (s *Search) Having(query string, values ...interface{}) *Search {
	s.addSqlCondition(cond_having_query, query, values...)
	s.setFlag(srch_has_having)
	return s
}

func (s *Search) Joins(query string, values ...interface{}) *Search {
	s.addSqlCondition(cond_joins_query, query, values...)
	s.setFlag(srch_has_joins)
	return s
}

func (s *Search) Select(query string, args ...interface{}) *Search {
	s.Conditions[cond_select_query] = make([]SqlPair, 0, 0)
	newPair := SqlPair{expression: query}
	newPair.addExpressions(args...)
	s.Conditions[cond_select_query] = append(s.Conditions[cond_select_query], newPair)
	if distinctSQLRegexp.MatchString(query) {
		s.setIsOrderIgnored()
	}
	s.setFlag(srch_has_select)
	return s
}

//TODO : @Badu - do the very same where we need only one instance (aka Singleton) - like select... (where getFirst is used)
func (s *Search) Limit(limit interface{}) *Search {
	s.Conditions[cond_limit_query] = make([]SqlPair, 0, 0)
	newPair := SqlPair{}
	newPair.addExpressions(limit)
	s.Conditions[cond_limit_query] = append(s.Conditions[cond_limit_query], newPair)

	s.setFlag(srch_has_offset_or_limit)
	return s
}

func (s *Search) Offset(offset interface{}) *Search {
	s.Conditions[cond_offset_query] = make([]SqlPair, 0, 0)
	newPair := SqlPair{}
	newPair.addExpressions(offset)
	s.Conditions[cond_offset_query] = append(s.Conditions[cond_offset_query], newPair)
	s.setFlag(srch_has_offset_or_limit)
	return s
}

func (s *Search) Group(query string) *Search {
	s.addSqlCondition(cond_group_query, query, nil)
	s.setFlag(srch_has_group)
	return s
}

func (s *Search) Attrs(attrs ...interface{}) *Search {
	result := argsToInterface(attrs...)
	if result != nil {
		s.addSqlCondition(cond_init_attrs, nil, result)
		s.setFlag(srch_has_init)
	}
	return s
}

func (s *Search) Assign(attrs ...interface{}) *Search {
	result := argsToInterface(attrs...)
	if result != nil {
		s.addSqlCondition(cond_assign_attrs, nil, result)
		s.setFlag(srch_has_assign)
	}
	return s
}

func (s *Search) Order(value interface{}, reorder ...bool) *Search {
	if len(reorder) > 0 && reorder[0] {
		//reseting existing entry
		s.Conditions[cond_order_query] = make([]SqlPair, 0, 0)
	}
	if value != nil {
		s.addSqlCondition(cond_order_query, nil, value)
		s.setFlag(srch_has_order)
	}
	return s
}

func (s *Search) Omit(columns ...string) *Search {
	s.checkInit(cond_omits_query)
	//add omit
	newPair := SqlPair{}
	//transfer slices (copy) - strings to interface
	newPair.args = make([]interface{}, len(columns))
	for i, v := range columns {
		newPair.args[i] = v
	}
	//add the condition pair to the slice
	s.Conditions[cond_omits_query] = append(s.Conditions[cond_omits_query], newPair)
	//fmt.Printf("Omit %d elements\n", s.numConditions(omits_query))
	s.setFlag(srch_has_omits)
	return s
}

func (s *Search) exec(scope *Scope, sql string, values ...interface{}) string {
	newPair := SqlPair{expression: sql}
	newPair.addExpressions(values...)
	genSql := s.buildWhereCondition(newPair, scope)
	return strings.TrimSuffix(strings.TrimPrefix(genSql, "("), ")")
}

func (s *Search) Exec(scope *Scope) (sql.Result, error) {
	result, err := scope.con.sqli.Exec(s.SQL, s.SQLVars...)
	if scope.Err(err) == nil {
		count, err := result.RowsAffected()
		if scope.Err(err) == nil {
			scope.con.RowsAffected = count
		}
	}
	return result, err
}

func (s *Search) Query(scope *Scope) (*sql.Rows, error) {
	rows, err := scope.con.sqli.Query(s.SQL, s.SQLVars...)
	return rows, err
}

func (s *Search) QueryRow(scope *Scope) *sql.Row {
	return scope.con.sqli.QueryRow(s.SQL, s.SQLVars...)
}

//should remain unused
func (s Search) hasFlag(value uint16) bool {
	return s.flags&(1<<value) != 0
}

func (s *Search) setFlag(value uint16) {
	s.flags = s.flags | (1 << value)
}

//should remain unused
func (s *Search) unsetFlag(value uint16) {
	s.flags = s.flags & ^(1 << value)
}

func (s *Search) isOrderIgnored() bool {
	return s.flags&(1<<srch_is_order_ignored) != 0
}

func (s *Search) hasSelect() bool {
	return s.flags&(1<<srch_has_select) != 0
}

func (s *Search) hasJoins() bool {
	return s.flags&(1<<srch_has_joins) != 0
}

func (s *Search) hasOrder() bool {
	return s.flags&(1<<srch_has_order) != 0
}

func (s *Search) hasAssign() bool {
	return s.flags&(1<<srch_has_assign) != 0
}

func (s *Search) hasPreload() bool {
	return s.flags&(1<<srch_has_preload) != 0
}

func (s *Search) hasHaving() bool {
	return s.flags&(1<<srch_has_having) != 0
}

func (s *Search) hasGroup() bool {
	return s.flags&(1<<srch_has_group) != 0
}

func (s *Search) hasOffsetOrLimit() bool {
	return s.flags&(1<<srch_has_offset_or_limit) != 0
}

func (s *Search) setIsOrderIgnored() *Search {
	s.flags = s.flags | (1 << srch_is_order_ignored)
	return s
}

func (s *Search) IsRaw() bool {
	return s.flags&(1<<srch_is_raw) != 0
}

func (s *Search) SetRaw() *Search {
	s.flags = s.flags | (1 << srch_is_raw)
	return s
}

func (s *Search) isUnscoped() bool {
	return s.flags&(1<<srch_is_unscoped) != 0
}

func (s *Search) setUnscoped() *Search {
	s.flags = s.flags | (1 << srch_is_unscoped)
	return s
}

func (s *Search) checkFieldIncluded(field *StructField) bool {
	fromPair := s.getFirst(cond_select_query)
	if fromPair != nil {
		strs := fromPair.strExpr()
		if field.StructName == strs || field.DBName == strs {
			return true
		}

		for _, pairArg := range fromPair.args {
			if field.StructName == pairArg || field.DBName == pairArg {
				return true
			}
		}
	}

	return false
}

func (s *Search) checkFieldOmitted(field *StructField) bool {
	pair := s.getFirst(cond_omits_query)
	if pair == nil {
		return false
	}
	for _, attr := range pair.args {
		if field.StructName == attr || field.DBName == attr {
			return true
		}
	}
	return false
}

// addToVars add value as sql's vars, used to prevent SQL injection
func (s *Search) addToVars(value interface{}, dialect Dialect) string {
	if pair, ok := value.(*SqlPair); ok {
		//TODO : @Badu - maybe it's best to split this into two function - one for sqlPair and one for value (to remove recursion)
		//fmt.Printf("CALL with pair : %v\n", fullFileWithLineNum())
		exp := pair.strExpr()
		for _, arg := range pair.args {
			exp = strings.Replace(exp, "?", s.addToVars(arg, dialect), 1)
		}
		return exp
	}
	s.SQLVars = append(s.SQLVars, value)
	return dialect.BindVar(len(s.SQLVars))
}

func (s *Search) whereSQL(scope *Scope) string {
	var (
		SQL, orSQL, andSQL, primarySQL string
		dialect                        = scope.con.parent.dialect
		quotedTableName                = scope.quotedTableName()
	)

	if !s.isUnscoped() && scope.GetModelStruct().HasColumn(field_deleted_at_name) {

		if primarySQL != "" {
			primarySQL += " AND "
		}
		primarySQL += fmt.Sprintf("%v.%s IS NULL", quotedTableName, field_deleted_at_name)
	}

	if !scope.PrimaryKeyZero() {
		for _, field := range scope.PKs() {
			if primarySQL != "" {
				primarySQL += " AND "
			}
			primarySQL += fmt.Sprintf(
				"%v.%v = %v",
				quotedTableName,
				scope.con.quote(field.DBName),
				s.addToVars(field.Value.Interface(), dialect),
			)

		}
	}

	for _, pair := range s.Conditions[cond_where_query] {
		if aStr := s.buildWhereCondition(pair, scope); aStr != "" {
			if andSQL != "" {
				andSQL += " AND "
			}
			andSQL += aStr
		}
	}

	for _, pair := range s.Conditions[cond_not_query] {
		if aStr := s.buildNotCondition(pair, scope); aStr != "" {
			if andSQL != "" {
				andSQL += " AND "
			}
			andSQL += aStr
		}
	}

	for _, pair := range s.Conditions[cond_or_query] {
		if aStr := s.buildWhereCondition(pair, scope); aStr != "" {
			if orSQL != "" {
				orSQL += " OR "
			}
			orSQL += aStr
		}
	}

	if andSQL != "" {
		if orSQL != "" {
			andSQL = andSQL + " OR " + orSQL
		}
	} else {
		andSQL = orSQL
	}

	if primarySQL != "" {
		SQL = "WHERE " + primarySQL
		if andSQL != "" {
			SQL = SQL + " AND (" + andSQL + ")"
		}
	} else if andSQL != "" {
		SQL = "WHERE " + andSQL
	}
	return SQL
}

func (s *Search) buildWhereCondition(fromPair SqlPair, scope *Scope) string {
	var (
		str             string
		quotedTableName = scope.quotedTableName()
		dialect         = scope.con.parent.dialect
		quotedPKName    = scope.con.quote(scope.PKName())
	)

	switch expType := fromPair.expression.(type) {
	case string:
		// if string is number
		if regExpNumberMatcher.MatchString(expType) {
			return fmt.Sprintf(
				"(%v.%v = %v)",
				quotedTableName,
				quotedPKName,
				s.addToVars(expType, dialect),
			)
		} else if expType != "" {
			str = fmt.Sprintf("(%v)", expType)
		}
	case int,
		int8,
		int16,
		int32,
		int64,
		uint,
		uint8,
		uint16,
		uint32,
		uint64,
		sql.NullInt64:
		return fmt.Sprintf("(%v.%v = %v)", quotedTableName, quotedPKName, s.addToVars(expType, dialect))
	case []int,
		[]int8,
		[]int16,
		[]int32,
		[]int64,
		[]uint,
		[]uint8,
		[]uint16,
		[]uint32,
		[]uint64,
		[]string,
		[]interface{}:
		str = fmt.Sprintf("(%v.%v IN (?))", quotedTableName, quotedPKName)
		//TODO : @Badu - seems really bad "work around" (boiler plate logic)
		fromPair.args = []interface{}{expType}
	case map[string]interface{}:
		var sqls []string
		for key, value := range expType {
			if value != nil {
				sqls = append(sqls,
					fmt.Sprintf(
						"(%v.%v = %v)",
						quotedTableName,
						scope.con.quote(key),
						s.addToVars(value, dialect),
					),
				)
			} else {
				sqls = append(sqls,
					fmt.Sprintf(
						"(%v.%v IS NULL)",
						quotedTableName,
						scope.con.quote(key),
					),
				)
			}
		}
		return strings.Join(sqls, " AND ")
	case interface{}:
		var sqls []string
		newScope := scope.con.emptyScope(expType)
		for _, field := range newScope.Fields() {
			if !field.IsIgnored() && !field.IsBlank() {
				sqls = append(sqls,
					fmt.Sprintf(
						"(%v.%v = %v)",
						newScope.quotedTableName(),
						scope.con.quote(field.DBName),
						s.addToVars(field.Value.Interface(), dialect),
					),
				)
			}
		}
		return strings.Join(sqls, " AND ")
	}

	for _, arg := range fromPair.args {
		switch reflect.ValueOf(arg).Kind() {
		case reflect.Slice: // For where("id in (?)", []int64{1,2})
			if bytes, ok := arg.([]byte); ok {
				str = strings.Replace(str, "?", s.addToVars(bytes, dialect), 1)
			} else if values := reflect.ValueOf(arg); values.Len() > 0 {
				var tempMarks []string
				for i := 0; i < values.Len(); i++ {
					tempMarks = append(tempMarks, s.addToVars(values.Index(i).Interface(), dialect))
				}
				str = strings.Replace(str, "?", strings.Join(tempMarks, ","), 1)
			} else {
				str = strings.Replace(str, "?", s.addToVars(SqlExpr("NULL"), dialect), 1)
			}
		default:
			if valuer, ok := interface{}(arg).(driver.Valuer); ok {
				arg, _ = valuer.Value()
			}

			str = strings.Replace(str, "?", s.addToVars(arg, dialect), 1)
		}
	}
	return str
}

func (s *Search) buildNotCondition(fromPair SqlPair, scope *Scope) string {
	var (
		str             string
		notEqualSQL     string
		primaryKey      = scope.PKName()
		quotedTableName = scope.quotedTableName()
		dialect         = scope.con.parent.dialect
		sqls            []string
		tempMarks       []string
	)
	switch exprType := fromPair.expression.(type) {
	case string:
		// is number
		if regExpNumberMatcher.MatchString(exprType) {
			id, _ := strconv.Atoi(exprType)
			return fmt.Sprintf("(%v <> %v)", scope.con.quote(primaryKey), id)
		} else if regExpLikeInMatcher.MatchString(exprType) {
			str = fmt.Sprintf(" NOT (%v) ", exprType)
			notEqualSQL = fmt.Sprintf("NOT (%v)", exprType)
		} else {
			str = fmt.Sprintf("(%v.%v NOT IN (?))", quotedTableName, scope.con.quote(exprType))
			notEqualSQL = fmt.Sprintf("(%v.%v <> ?)", quotedTableName, scope.con.quote(exprType))
		}
	case int,
		int8,
		int16,
		int32,
		int64,
		uint,
		uint8,
		uint16,
		uint32,
		uint64,
		sql.NullInt64:
		return fmt.Sprintf("(%v.%v <> %v)", quotedTableName, scope.con.quote(primaryKey), exprType)
	case []int,
		[]int8,
		[]int16,
		[]int32,
		[]int64,
		[]uint,
		[]uint8,
		[]uint16,
		[]uint32,
		[]uint64,
		[]string:
		if reflect.ValueOf(exprType).Len() > 0 {
			str = fmt.Sprintf("(%v.%v NOT IN (?))", quotedTableName, scope.con.quote(primaryKey))
			//TODO : @Badu - seems really bad "work around" (boiler plate logic)
			fromPair.args = []interface{}{exprType}
		}
		return ""
	case map[string]interface{}:
		for key, value := range exprType {
			if value != nil {
				sqls = append(sqls,
					fmt.Sprintf(
						"(%v.%v <> %v)",
						quotedTableName,
						scope.con.quote(key),
						s.addToVars(value, dialect),
					),
				)
			} else {
				sqls = append(sqls,
					fmt.Sprintf(
						"(%v.%v IS NOT NULL)",
						quotedTableName,
						scope.con.quote(key),
					),
				)
			}
		}
		return strings.Join(sqls, " AND ")
	case interface{}:
		var newScope = scope.con.emptyScope(exprType)
		for _, field := range newScope.Fields() {
			if !field.IsBlank() {
				sqls = append(sqls,
					fmt.Sprintf(
						"(%v.%v <> %v)",
						newScope.quotedTableName(),
						scope.con.quote(field.DBName),
						s.addToVars(field.Value.Interface(), dialect),
					),
				)
			}
		}
		return strings.Join(sqls, " AND ")
	}

	for _, arg := range fromPair.args {
		switch reflect.ValueOf(arg).Kind() {
		case reflect.Slice: // For where("id in (?)", []int64{1,2})
			if bytes, ok := arg.([]byte); ok {
				str = strings.Replace(str, "?", s.addToVars(bytes, dialect), 1)
			} else if values := reflect.ValueOf(arg); values.Len() > 0 {

				for i := 0; i < values.Len(); i++ {
					tempMarks = append(tempMarks, s.addToVars(values.Index(i).Interface(), dialect))
				}
				str = strings.Replace(str, "?", strings.Join(tempMarks, ","), 1)
			} else {
				str = strings.Replace(str, "?", s.addToVars(SqlExpr("NULL"), dialect), 1)
			}
		default:
			if scanner, ok := interface{}(arg).(driver.Valuer); ok {
				arg, _ = scanner.Value()
			}
			str = strings.Replace(notEqualSQL, "?", s.addToVars(arg, dialect), 1)
		}
	}
	return str
}

// CombinedConditionSql return combined condition sql
func (s *Search) combinedConditionSql(scope *Scope) string {
	//Attention : if we don't build joinSql first, joins will fail (it's mixing up the where clauses of the joins)
	//-= creating Joins =-
	SQL := ""
	for _, pair := range s.Conditions[cond_joins_query] {
		if aStr := s.buildWhereCondition(pair, scope); aStr != "" {
			if SQL != "" {
				SQL += " "
			}
			SQL += strings.TrimSuffix(strings.TrimPrefix(aStr, "("), ")")
		}
	}
	if SQL != "" {
		SQL += " "
	}
	//-= end creating Joins =-

	//-= creating Where =-
	if s.IsRaw() {
		SQL += strings.TrimSuffix(strings.TrimPrefix(s.whereSQL(scope), "WHERE ("), ")")
	} else {
		SQL += s.whereSQL(scope)
	}
	//-= end creating Where =-

	//-= creating Group =-
	if s.hasGroup() {
		SQL += " GROUP BY " + s.Conditions[cond_group_query][0].expression.(string)
	}
	//-= end creating Group =-

	//-= creating Having =-
	if s.hasHaving() {
		combinedSQL := ""
		for _, pair := range s.Conditions[cond_having_query] {
			if aStr := s.buildWhereCondition(pair, scope); aStr != "" {
				if combinedSQL != "" {
					combinedSQL += " AND "
				}
				combinedSQL += aStr
			}
		}
		if combinedSQL != "" {
			SQL += " HAVING " + combinedSQL
		}
	}
	//-= end creating Having =-

	//-= creating Order =-
	if s.hasOrder() && !s.isOrderIgnored() {
		dialect := scope.con.parent.dialect
		orderSQL := ""
		for _, orderPair := range s.Conditions[cond_order_query] {
			if str, ok := orderPair.args[0].(string); ok {
				if orderSQL != "" {
					orderSQL += ","
				}
				orderSQL += scope.quoteIfPossible(str)
			} else if pair, ok := orderPair.args[0].(*SqlPair); ok {
				exp := pair.strExpr()
				for _, arg := range pair.args {
					exp = strings.Replace(exp, "?", s.addToVars(arg, dialect), 1)
				}
				if orderSQL != "" {
					orderSQL += ","
				}
				orderSQL += exp
			}
		}
		if orderSQL != "" {
			SQL += " ORDER BY " + orderSQL
		}
	}
	//-= end creating Order =-

	if s.hasOffsetOrLimit() {
		limitValue := -1
		offsetValue := -1

		if len(s.Conditions[cond_limit_query]) > 0 {
			limitValue = s.Conditions[cond_limit_query][0].args[0].(int)
		}

		if len(s.Conditions[cond_offset_query]) > 0 {
			offsetValue = s.Conditions[cond_offset_query][0].args[0].(int)

		}
		limitAndOffsetSQL := scope.con.parent.dialect.LimitAndOffsetSQL(limitValue, offsetValue)
		return SQL + limitAndOffsetSQL
	}
	return SQL

}

func (s *Search) prepareQuerySQL(scope *Scope) {
	if s.IsRaw() {
		scope.Raw(s.combinedConditionSql(scope))
	} else {
		selectSQL := ""
		if s.hasSelect() {
			fromPair := s.getFirst(cond_select_query)
			if fromPair == nil {
				//error has occurred in getting first item in slice
				scope.Warn(fmt.Errorf("Error : error has occurred in getting first item in slice for SELECT"))
				selectSQL = ""
			} else {
				switch value := fromPair.expression.(type) {
				case string:
					selectSQL = value
				case []string:
					selectSQL = strings.Join(value, ", ")
				}
				for _, arg := range fromPair.args {
					switch reflect.ValueOf(arg).Kind() {
					case reflect.Slice:
						values := reflect.ValueOf(arg)
						marks := ""
						for i := 0; i < values.Len(); i++ {
							if marks != "" {
								marks += ","
							}
							marks += s.addToVars(
								values.Index(i).Interface(),
								scope.con.parent.dialect,
							)
						}
						selectSQL = strings.Replace(selectSQL, "?", marks, 1)
					default:
						if valuer, ok := interface{}(arg).(driver.Valuer); ok {
							arg, _ = valuer.Value()
						}
						selectSQL = strings.Replace(selectSQL, "?", s.addToVars(arg, scope.con.parent.dialect), 1)
					}
				}
			}
		} else if s.hasJoins() {
			selectSQL = fmt.Sprintf("%v.*", scope.quotedTableName())
		} else {
			selectSQL = str_everything
		}

		scope.Raw(fmt.Sprintf("SELECT %v FROM %v %v", selectSQL, scope.quotedTableName(), s.combinedConditionSql(scope)))
	}
}

func (srch *Search) doPreload(scope *Scope) {
	var (
		preloadedMap = map[string]bool{}
		fields       = scope.Fields()
	)

	for _, sqlPair := range srch.Conditions[cond_preload_query] {
		var (
			preloadFields = strings.Split(sqlPair.strExpr(), ".")
			currentScope  = scope
			currentFields = fields
		)

		for idx, preloadField := range preloadFields {
			var currentPreloadConditions []interface{}
			//there is no next level
			if currentScope == nil {
				continue
			}

			// if not preloaded
			if preloadKey := strings.Join(preloadFields[:idx+1], "."); !preloadedMap[preloadKey] {

				// assign search conditions to last preload
				if idx == len(preloadFields)-1 {
					currentPreloadConditions = sqlPair.args
				}

				for _, field := range currentFields {
					if field.StructName != preloadField || !field.HasRelations() {
						continue
					}

					switch field.RelKind() {
					case rel_has_one, rel_has_many, rel_belongs_to:
						handleRelationPreload(currentScope, field, currentPreloadConditions)
					case rel_many2many:
						handleManyToManyPreload(currentScope, field, currentPreloadConditions)
					default:
						scope.Err(fmt.Errorf(err_unsupported_relation, field.RelKind()))
					}

					preloadedMap[preloadKey] = true
					break
				}

				if !preloadedMap[preloadKey] {
					scope.Err(
						fmt.Errorf(err_cant_preload,
							preloadField,
							currentScope.GetModelStruct().ModelType))
					return
				}
			}

			// preload next level
			if idx < len(preloadFields)-1 {
				//if preloadField is struct or slice, we need to get it's scope
				currentScope = getColumnAsScope(preloadField, currentScope)

				if currentScope != nil {
					currentFields = currentScope.Fields()
				}
			}
		}
	}
}

func (s *Search) changeableField(field *StructField) bool {
	if s.hasSelect() {
		if s.checkFieldIncluded(field) {
			return true
		}
		return false
	}

	if s.checkFieldOmitted(field) {
		return false
	}

	return true
}
