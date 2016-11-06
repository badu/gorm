package gorm

func init() {
	RegisterDialect("common", &commonDialect{})
	//TODO : @Badu - maybe we should include only the dialect used by the user's application
	//avoiding fat executables
	RegisterDialect("mysql", &mysql{})
	RegisterDialect("sqlite", &sqlite3{})
	RegisterDialect("sqlite3", &sqlite3{})
	RegisterDialect("postgres", &postgres{})
}
