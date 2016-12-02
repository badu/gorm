package tests

import (
	"fmt"
	"testing"
	"gorm"
)

func TestCalculateField(t *testing.T) {
	//t.Log("67) TestCalculateField")
	var field CalculateField
	var scope = TestDB.NewScope(&field)
	if field, ok := scope.FieldByName("Children"); !ok || field.Relationship == nil {
		t.Errorf("Should calculate fields correctly for the first time")
	}

	if field, ok := scope.FieldByName("Category"); !ok || field.Relationship == nil {
		t.Errorf("Should calculate fields correctly for the first time")
	}

	if field, ok := scope.FieldByName("EmbeddedName"); !ok {
		t.Errorf("should find embedded field")
	} else if !field.HasSetting(gorm.NOT_NULL) {
		t.Errorf(fmt.Sprintf("Should find embedded field's tag settings\n%s", field))
	}
}
