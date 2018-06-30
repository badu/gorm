package gorm

func (s *StructFields) add(field *StructField) {
	*s = append(*s, field)
}

func (s *StructFields) len() int {
	return len(*s)
}
