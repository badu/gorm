package gorm

import "reflect"

//utils for test case
func (c SqlConditions) CompareWhere(cond SqlConditions) bool {
	return reflect.DeepEqual(c[condWhereQuery], cond[condWhereQuery])
}

//utils for test case
func (c SqlConditions) CompareOrder(cond SqlConditions) bool {
	return reflect.DeepEqual(c[condOrderQuery], cond[condOrderQuery])
}

//utils for test case
func (c SqlConditions) CompareInit(cond SqlConditions) bool {
	return reflect.DeepEqual(c[condInitAttrs], cond[condInitAttrs])
}

//utils for test case
func (c SqlConditions) CompareSelect(cond SqlConditions) bool {
	return reflect.DeepEqual(c[condSelectQuery], cond[condSelectQuery])
}
