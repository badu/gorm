package tests

import (
	"testing"
)

func ScannableSlices(t *testing.T) {
	if err := TestDB.AutoMigrate(&RecordWithSlice{}).Error; err != nil {
		t.Errorf("Should create table with slice values correctly: %s", err)
	}

	r1 := RecordWithSlice{
		Strings: ExampleStringSlice{"a", "b", "c"},
		Structs: ExampleStructSlice{
			{"name1", "value1"},
			{"name2", "value2"},
		},
	}

	if err := TestDB.Save(&r1).Error; err != nil {
		t.Errorf("Should save record with slice values")
	}

	var r2 RecordWithSlice

	if err := TestDB.Find(&r2).Error; err != nil {
		t.Errorf("Should fetch record with slice values")
	}

	if len(r2.Strings) != 3 || r2.Strings[0] != "a" || r2.Strings[1] != "b" || r2.Strings[2] != "c" {
		t.Errorf("Should have serialised and deserialised a string array")
	}

	if len(r2.Structs) != 2 || r2.Structs[0].Name != "name1" || r2.Structs[0].Value != "value1" || r2.Structs[1].Name != "name2" || r2.Structs[1].Value != "value2" {
		t.Errorf("Should have serialised and deserialised a struct array")
	}
}
