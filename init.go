package gorm

import "strings"

func init() {
	RegisterDialect("common", &commonDialect{})
	//TODO : @Badu - maybe we should include only the dialect used by the user's application
	//avoiding fat executables
	RegisterDialect("mysql", &mysql{})
	RegisterDialect("sqlite", &sqlite3{})
	RegisterDialect("sqlite3", &sqlite3{})
	RegisterDialect("postgres", &postgres{})

	var commonInitialismsForReplacer []string
	for _, initialism := range commonInitialisms {
		commonInitialismsForReplacer = append(commonInitialismsForReplacer, initialism, strings.Title(strings.ToLower(initialism)))
	}
	commonInitialismsReplacer = strings.NewReplacer(commonInitialismsForReplacer...)
}
