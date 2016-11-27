package gorm

import (
	"fmt"
)

func (DefaultForeignKeyNamer) BuildForeignKeyName(tableName, field, dest string) string {
	keyName := fmt.Sprintf("%s_%s_%s_foreign", tableName, field, dest)
	keyName = regExpFKName.ReplaceAllString(keyName, "_")
	return keyName
}
