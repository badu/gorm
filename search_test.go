package gorm

import (
	"reflect"
	"testing"
)

func TestCloneSearch(t *testing.T) {
	t.Log("129) TestCloneSearch")
	s := &search{conditions: make(sqlConditions)}
	s.Where("name = ?", "jinzhu").Order("name").Attrs("name", "jinzhu").Select("name, age")

	s1 := s.clone()
	s1.Where("age = ?", 20).Order("age").Attrs("email", "a@e.org").Select("email")

	if reflect.DeepEqual(s.conditions[where_query], s1.conditions[where_query]) {
		t.Errorf("Where should be copied (NOT deep equal)")
	}

	if reflect.DeepEqual(s.conditions[order_query], s1.conditions[order_query]) {
		t.Errorf("Order should be copied")
	}

	if reflect.DeepEqual(s.conditions[init_attrs], s1.conditions[init_attrs]) {
		t.Errorf("InitAttrs should be copied")
	}

	if reflect.DeepEqual(s.conditions[select_query], s1.conditions[select_query]) {
		t.Errorf("selectStr should be copied")
	}
}
