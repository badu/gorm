package gorm

import (
	"testing"
)

func TestCalculateField(t *testing.T) {
	t.Log("67) TestCalculateField")
	var field CalculateField
	var scope = TestDB.NewScope(&field)
	if field, ok := scope.FieldByName("Children"); !ok || field.Relationship == nil {
		t.Errorf("Should calculate fields correctly for the first time")
	}

	if field, ok := scope.FieldByName("Category"); !ok || field.Relationship == nil {
		t.Errorf("Should calculate fields correctly for the first time")
	}

	if field, ok := scope.FieldByName("embedded_name"); !ok {
		t.Errorf("should find embedded field")
	} else if _, ok := field.TagSettings[NOT_NULL]; !ok {
		t.Errorf("should find embedded field's tag settings")
	}
}
