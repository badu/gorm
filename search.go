package gorm

import "fmt"

const (
	where_query  sqlConditionType = 0
	where_args   sqlConditionType = 1
	not_query    sqlConditionType = 2
	not_args     sqlConditionType = 3
	or_query     sqlConditionType = 4
	or_args      sqlConditionType = 5
	init_attrs   sqlConditionType = 6
	assign_attrs sqlConditionType = 7
	select_query sqlConditionType = 8
	select_args  sqlConditionType = 9
	having_query sqlConditionType = 10
	having_args  sqlConditionType = 11
	joins_query  sqlConditionType = 12
	joins_args   sqlConditionType = 13
)

func (s *search) clone() *search {
	//TODO : @Badu - it's this a ... clone ?
	clone := *s
	return &clone
}

//typedQ - typedA are a pair , e.g. where_query, where_args
func (s *search) GetTypedConditions(typedQ sqlConditionType, typedA sqlConditionType) {
	var result sqlConditions
	for _, condition := range s.conditions {
		if condition.Type == typedQ || condition.Type == typedA {
			result = append(result, condition)
		}
	}
}

func (s *search) Where(query interface{}, values ...interface{}) *search {
	s.whereConditions = append(s.whereConditions, map[string]interface{}{"query": query, "args": values})

	s.conditions = append(s.conditions, sqlCondition{
		Type:   where_query,
		Values: query,
	})
	s.conditions = append(s.conditions, sqlCondition{
		Type:   where_args,
		Values: values,
	})
	return s
}

func (s *search) Not(query interface{}, values ...interface{}) *search {
	s.notConditions = append(s.notConditions, map[string]interface{}{"query": query, "args": values})

	s.conditions = append(s.conditions, sqlCondition{
		Type:   not_query,
		Values: query,
	})
	s.conditions = append(s.conditions, sqlCondition{
		Type:   not_args,
		Values: values,
	})
	return s
}

func (s *search) Or(query interface{}, values ...interface{}) *search {
	s.orConditions = append(s.orConditions, map[string]interface{}{"query": query, "args": values})

	s.conditions = append(s.conditions, sqlCondition{
		Type:   or_query,
		Values: query,
	})
	s.conditions = append(s.conditions, sqlCondition{
		Type:   or_args,
		Values: values,
	})
	return s
}

func (s *search) Having(query string, values ...interface{}) *search {
	s.havingConditions = append(s.havingConditions, map[string]interface{}{"query": query, "args": values})

	s.conditions = append(s.conditions, sqlCondition{
		Type:   having_query,
		Values: query,
	})
	s.conditions = append(s.conditions, sqlCondition{
		Type:   having_args,
		Values: values,
	})
	return s
}

func (s *search) Joins(query string, values ...interface{}) *search {
	s.joinConditions = append(s.joinConditions, map[string]interface{}{"query": query, "args": values})

	s.conditions = append(s.conditions, sqlCondition{
		Type:   joins_query,
		Values: query,
	})
	s.conditions = append(s.conditions, sqlCondition{
		Type:   joins_args,
		Values: values,
	})
	return s
}

func (s *search) Select(query interface{}, args ...interface{}) *search {
	s.selects = map[string]interface{}{"query": query, "args": args}

	s.conditions = append(s.conditions, sqlCondition{
		Type:   select_query,
		Values: query,
	})
	s.conditions = append(s.conditions, sqlCondition{
		Type:   select_args,
		Values: args,
	})
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
