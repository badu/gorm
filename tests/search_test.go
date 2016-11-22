package tests

import (
	"reflect"
	"testing"
	"gorm"
)

func TestCloneSearch(t *testing.T) {
	t.Log("129) TestCloneSearch")
	s := &gorm.Search{Conditions: make(gorm.SqlConditions)}
	s.Where("name = ?", "jinzhu").Order("name").Attrs("name", "jinzhu").Select("name, age")

	s1 := s.Clone()
	s1.Where("age = ?", 20).Order("age").Attrs("email", "a@e.org").Select("email")

	if reflect.DeepEqual(s.Conditions[gorm.Where_query], s1.Conditions[gorm.Where_query]) {
		t.Errorf("Where should be copied (NOT deep equal)")
	}

	if reflect.DeepEqual(s.Conditions[gorm.Order_query], s1.Conditions[gorm.Order_query]) {
		t.Errorf("Order should be copied")
	}

	if reflect.DeepEqual(s.Conditions[gorm.Init_attrs], s1.Conditions[gorm.Init_attrs]) {
		t.Errorf("InitAttrs should be copied")
	}

	if reflect.DeepEqual(s.Conditions[gorm.Select_query], s1.Conditions[gorm.Select_query]) {
		t.Errorf("selectStr should be copied")
	}
}
