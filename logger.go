package gorm

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"time"
	"unicode"
)

type (
	logger interface {
		Print(v ...interface{})
	}

	// LogWriter log writer interface
	LogWriter interface {
		Println(v ...interface{})
	}

	// Logger default logger
	Logger struct {
		LogWriter
	}
)

var (
	//regexpSelf = regexp.MustCompile(`badu/gorm/.*.go`)
	regexpSelf = regexp.MustCompile(`/gorm/.*.go`)
	regexpTest = regexp.MustCompile(`/gorm/tests/.*.go`)
)

func isPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func fileWithLineNum() string {
	for i := 6; i < 15; i++ {
		_, file, line, ok := runtime.Caller(i)
		if ok && regexpTest.MatchString(file) {
			//matching test files - we print that
			return fmt.Sprintf("%v:%v", file, line)
		} else if ok && !regexpSelf.MatchString(file) {
			//otherwise
			return fmt.Sprintf("%v:%v", file, line)
		}
	}
	return ""
}

// Print format & print log
func (logger Logger) Print(values ...interface{}) {
	if len(values) > 1 {
		level := values[0]
		currentTime := "\n\033[33m[" + NowFunc().Format("2006-01-02 15:04:05") + "]\033[0m"
		source := fmt.Sprintf("\033[35m(%v)\033[0m", values[1])
		messages := []interface{}{source, currentTime}

		if level == "sql" {
			// duration
			messages = append(messages, fmt.Sprintf(" \033[36;1m[%.2fms]\033[0m ", float64(values[2].(time.Duration).Nanoseconds()/1e4)/100.0))
			// sql
			var sql string
			var formattedValues []string

			for _, value := range values[4].([]interface{}) {
				indirectValue := reflect.Indirect(reflect.ValueOf(value))
				if indirectValue.IsValid() {
					value = indirectValue.Interface()
					if t, ok := value.(time.Time); ok {
						formattedValues = append(formattedValues, fmt.Sprintf("'%v'", t.Format(time.RFC3339)))
					} else if b, ok := value.([]byte); ok {
						if str := string(b); isPrintable(str) {
							formattedValues = append(formattedValues, fmt.Sprintf("'%v'", str))
						} else {
							formattedValues = append(formattedValues, "'<binary>'")
						}
					} else if r, ok := value.(driver.Valuer); ok {
						if value, err := r.Value(); err == nil && value != nil {
							formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
						} else {
							formattedValues = append(formattedValues, "NULL")
						}
					} else {
						formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
					}
				} else {
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
				}
			}

			var formattedValuesLength = len(formattedValues)
			for index, value := range sqlRegexp.Split(values[3].(string), -1) {
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
