package gorm

import (
	"fmt"
	"reflect"
	"strings"
)

func (structField *StructField) clone() *StructField {
	clone := &StructField{
		IsPrimaryKey:    structField.IsPrimaryKey,
		IsNormal:        structField.IsNormal,
		IsIgnored:       structField.IsIgnored,
		IsScanner:       structField.IsScanner,
		HasDefaultValue: structField.HasDefaultValue,
		IsForeignKey:    structField.IsForeignKey,

		DBName: structField.DBName,
		Names:           structField.Names,
		TagSettings:     map[uint8]string{},
		Struct:          structField.Struct,
		Relationship:    structField.Relationship,
	}

	for key, value := range structField.TagSettings {
		clone.TagSettings[key] = value
	}

	return clone
}

//Function collects information from tags named `sql:""` and `gorm:""`
func (structField *StructField) ParseTagSettings() {
	setting := map[uint8]string{}
	for _, str := range []string{structField.Struct.Tag.Get("sql"), structField.Struct.Tag.Get("gorm")} {
		tags := strings.Split(str, ";")
		for _, value := range tags {
			v := strings.Split(value, ":")
			k := strings.TrimSpace(strings.ToUpper(v[0]))
			uint8Key, ok := tagSettingMap[k]
			if ok {
				if len(v) >= 2 {
					setting[uint8Key] = strings.Join(v[1:], ":")
				} else {
					setting[uint8Key] = k
				}
			} else {
				fmt.Errorf("ERROR : COULDN'T FIND KEY FOR %q", k)
			}
		}
	}
	structField.TagSettings = setting
}

func (structField *StructField) GetName() string {
	return structField.Struct.Name
}
//TODO : @Badu - might need removal since seems unused
//seems unused
func (structField *StructField) GetTag() reflect.StructTag {
	return structField.Struct.Tag
}
