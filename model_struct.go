package gorm

// TableName get model's table name
func (s *ModelStruct) TableName(db *DB) string {
	return DefaultTableNameHandler(db, s.defaultTableName)
}
