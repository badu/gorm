package gorm

import (
	"bytes"
	"fmt"
)

type (

	Collector struct {
		values []interface{}
		result bytes.Buffer
	}
)

func (c *Collector) add(str string, params ...interface{}){
	c.result.WriteString(str)
	c.values = append(c.values, params...)
}

func (c Collector) String() string{
	return fmt.Sprintf(c.result.String(), c.values...)
}