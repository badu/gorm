package gorm

func (sf *StructFields) add(field *StructField) {
	//TODO : @Badu - assignment to method receiver propagates only to callees but not to callers
	*sf = append(*sf, field)
}

func (sf *StructFields) len() int {
	return len(*sf)
}
