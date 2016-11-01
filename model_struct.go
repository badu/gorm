package gorm

// TableName get model's table name
func (s *ModelStruct) TableName(db *DBCon) string {
	return DefaultTableNameHandler(db, s.defaultTableName)
}

func (s *ModelStruct) getForeignField(column string) *StructField {
	//TODO : @Badu - find a easier way to deliver this, instead of iterating over slice
	//Attention : after you get rid of ToDBName and other unnecessary fields
	for _, field := range s.StructFields {
		if field.GetName() == column || field.DBName == column || field.DBName == ToDBName(column) {
			return field
		}
	}
	return nil
}
