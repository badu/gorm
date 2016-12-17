package gorm

import "reflect"

//utils for test case
func (c SqlConditions) CompareWhere(cond SqlConditions) bool {
	return reflect.DeepEqual(c[cond_where_query], cond[cond_where_query])
}

//utils for test case
func (c SqlConditions) CompareOrder(cond SqlConditions) bool {
	return reflect.DeepEqual(c[cond_order_query], cond[cond_order_query])
}

//utils for test case
func (c SqlConditions) CompareInit(cond SqlConditions) bool {
	return reflect.DeepEqual(c[cond_init_attrs], cond[cond_init_attrs])
}

//utils for test case
func (c SqlConditions) CompareSelect(cond SqlConditions) bool {
	return reflect.DeepEqual(c[cond_select_query], cond[cond_select_query])
}
