package gorm

func (sf *StructFields) add(field *StructField) {
	*sf = append(*sf, field)
}

func (sf *StructFields) len() int {
	return len(*sf)
}
