package gorm

import "strings"

func init() {
	// Define callbacks for creating
	DefaultCallback.Create().Register("gorm:begin_transaction", beginTransactionCallback)
	DefaultCallback.Create().Register("gorm:before_create", beforeCreateCallback)
	DefaultCallback.Create().Register("gorm:save_before_associations", saveBeforeAssociationsCallback)
	DefaultCallback.Create().Register("gorm:update_time_stamp", updateTimeStampForCreateCallback)
	DefaultCallback.Create().Register("gorm:create", createCallback)
	DefaultCallback.Create().Register("gorm:force_reload_after_create", forceReloadAfterCreateCallback)
	DefaultCallback.Create().Register("gorm:save_after_associations", saveAfterAssociationsCallback)
	DefaultCallback.Create().Register("gorm:after_create", afterCreateCallback)
	DefaultCallback.Create().Register("gorm:commit_or_rollback_transaction", commitOrRollbackTransactionCallback)
	// Define callbacks for deleting
	DefaultCallback.Delete().Register("gorm:begin_transaction", beginTransactionCallback)
	DefaultCallback.Delete().Register("gorm:before_delete", beforeDeleteCallback)
	DefaultCallback.Delete().Register("gorm:delete", deleteCallback)
	DefaultCallback.Delete().Register("gorm:after_delete", afterDeleteCallback)
	DefaultCallback.Delete().Register("gorm:commit_or_rollback_transaction", commitOrRollbackTransactionCallback)
	// Define callbacks for querying
	DefaultCallback.Query().Register("gorm:query", queryCallback)
	DefaultCallback.Query().Register("gorm:preload", preloadCallback)
	DefaultCallback.Query().Register("gorm:after_query", afterQueryCallback)
	// Define callbacks for updating
	DefaultCallback.Update().Register("gorm:assign_updating_attributes", assignUpdatingAttributesCallback)
	DefaultCallback.Update().Register("gorm:begin_transaction", beginTransactionCallback)
	DefaultCallback.Update().Register("gorm:before_update", beforeUpdateCallback)
	DefaultCallback.Update().Register("gorm:save_before_associations", saveBeforeAssociationsCallback)
	DefaultCallback.Update().Register("gorm:update_time_stamp", updateTimeStampForUpdateCallback)
	DefaultCallback.Update().Register("gorm:update", updateCallback)
	DefaultCallback.Update().Register("gorm:save_after_associations", saveAfterAssociationsCallback)
	DefaultCallback.Update().Register("gorm:after_update", afterUpdateCallback)
	DefaultCallback.Update().Register("gorm:commit_or_rollback_transaction", commitOrRollbackTransactionCallback)

	RegisterDialect("common", &commonDialect{})
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
