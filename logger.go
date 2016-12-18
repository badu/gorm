package gorm

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"time"
	"unicode"
)

// Print format & print log
func (logger Logger) Print(values ...interface{}) {
	if len(values) > 1 {
		currentTime := "\n\033[33m[" + NowFunc().Format("2006-01-02 15:04:05") + "]\033[0m"
		source := fmt.Sprintf("\033[35m %v \033[0m", values[1])
		messages := []interface{}{source, currentTime}

		if values[0] == str_tag_sql {
			//error
			if values[4] != nil {
				messages = append(messages, fmt.Sprintf("ERROR: %q\n", values[4]))
			}
			// duration
			messages = append(messages, fmt.Sprintf("\033[36;1m[%.2fms]\033[0m", float64(values[2].(time.Duration).Nanoseconds()/1e4)/100.0))
			// sql
			var sql string
			var formattedValues []string

			for _, value := range values[5].([]interface{}) {
				indirectValue := reflect.Indirect(reflect.ValueOf(value))
				if indirectValue.IsValid() {
					value = indirectValue.Interface()
					switch typ := value.(type) {
					case time.Time:
						formattedValues = append(formattedValues, fmt.Sprintf("'%v'", typ.Format(time.RFC3339)))
					case []byte:
						str := string(typ)
						//check if string is printable
						isPrintable := true
						for _, r := range str {
							if !unicode.IsPrint(r) {
								isPrintable = false
								break //break this for
							}
						}
						if isPrintable {
							formattedValues = append(formattedValues, fmt.Sprintf("'%v'", str))
						} else {
							formattedValues = append(formattedValues, "'<binary>'")
						}
					case driver.Valuer:
						if value, err := typ.Value(); err == nil && value != nil {
							formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
						} else {
							formattedValues = append(formattedValues, "NULL")
						}
					default:
						formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
					}
				} else {
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
				}
			}

			var formattedValuesLength = len(formattedValues)
			for index, value := range regExpLogger.Split(values[3].(string), -1) {
				sql += value
				if index < formattedValuesLength {
					sql += formattedValues[index]
				}
			}

			messages = append(messages, sql)
		} else {
			messages = append(messages, "\033[31;1m")
			messages = append(messages, values[2:]...)
			messages = append(messages, "\033[0m")
		}
		logger.Println(messages...)
	}
}
