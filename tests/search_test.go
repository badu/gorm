package tests

import (
	. "github.com/badu/reGorm"
	"testing"
)

func CloneSearch(t *testing.T) {
	s := &Search{Conditions: make(SqlConditions)}
	s.Where("name = ?", "jinzhu").Order("name").Attrs("name", "jinzhu").Select("name, age")

	s1 := s.Clone()
	s1.Where("age = ?", 20).Order("age").Attrs("email", "a@e.org").Select("email")

	if s.Conditions.CompareWhere(s1.Conditions) {
		t.Errorf("Where should be copied (NOT deep equal)")
	}

	if s.Conditions.CompareOrder(s1.Conditions) {
		t.Errorf("Order should be copied")
	}

	if s.Conditions.CompareInit(s1.Conditions) {
		t.Errorf("InitAttrs should be copied")
	}

	if s.Conditions.CompareSelect(s1.Conditions) {
		t.Errorf("selectStr should be copied")
	}
}
