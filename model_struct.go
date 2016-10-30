package gorm

// TableName get model's table name
func (s *ModelStruct) TableName(db *DBCon) string {
	return DefaultTableNameHandler(db, s.defaultTableName)
}

func (s *ModelStruct) getForeignField(column string) *StructField {
	for _, field := range s.StructFields {
		if field.GetName() == column || field.DBName == column || field.DBName == ToDBName(column) {
			return field
		}
	}
	return nil
}
