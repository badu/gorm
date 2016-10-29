package gorm

import (
	"fmt"
	"regexp"
)

func (DefaultForeignKeyNamer) BuildForeignKeyName(tableName, field, dest string) string {
	keyName := fmt.Sprintf("%s_%s_%s_foreign", tableName, field, dest)
	keyName = regexp.MustCompile("(_*[^a-zA-Z]+_*|_+)").ReplaceAllString(keyName, "_")
	return keyName
}
