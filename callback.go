package gorm

func (c *Callback) clone() *Callback {
	return &Callback{
		creates:    c.creates,
		updates:    c.updates,
		deletes:    c.deletes,
		queries:    c.queries,
		rowQueries: c.rowQueries,
		processors: c.processors,
	}
}

func (c *Callback) registerGORMDefaultCallbacks() {
	// Define callbacks for creating
	c.Create().Register("gorm:begin_transaction", beginTransactionCallback)
	c.Create().Register("gorm:before_create", beforeCreateCallback)
	c.Create().Register("gorm:save_before_associations", saveBeforeAssociationsCallback)
	c.Create().Register("gorm:update_time_stamp", updateTimeStampForCreateCallback)
	c.Create().Register("gorm:create", createCallback)
	c.Create().Register("gorm:force_reload_after_create", forceReloadAfterCreateCallback)
	c.Create().Register("gorm:save_after_associations", saveAfterAssociationsCallback)
	c.Create().Register("gorm:after_create", afterCreateCallback)
	c.Create().Register("gorm:commit_or_rollback_transaction", commitOrRollbackTransactionCallback)
	// Define callbacks for deleting
	c.Delete().Register("gorm:begin_transaction", beginTransactionCallback)
	c.Delete().Register("gorm:before_delete", beforeDeleteCallback)
	c.Delete().Register("gorm:delete", deleteCallback)
	c.Delete().Register("gorm:after_delete", afterDeleteCallback)
	c.Delete().Register("gorm:commit_or_rollback_transaction", commitOrRollbackTransactionCallback)
	// Define callbacks for querying
	c.Query().Register("gorm:query", queryCallback)
	c.Query().Register("gorm:preload", preloadCallback)
	c.Query().Register("gorm:after_query", afterQueryCallback)
	// Define callbacks for updating
	c.Update().Register("gorm:assign_updating_attributes", assignUpdatingAttributesCallback)
	c.Update().Register("gorm:begin_transaction", beginTransactionCallback)
	c.Update().Register("gorm:before_update", beforeUpdateCallback)
	c.Update().Register("gorm:save_before_associations", saveBeforeAssociationsCallback)
	c.Update().Register("gorm:update_time_stamp", updateTimeStampForUpdateCallback)
	c.Update().Register("gorm:update", updateCallback)
	c.Update().Register("gorm:save_after_associations", saveAfterAssociationsCallback)
	c.Update().Register("gorm:after_update", afterUpdateCallback)
	c.Update().Register("gorm:commit_or_rollback_transaction", commitOrRollbackTransactionCallback)
}

// reorder all registered processors, and reset CURD callbacks
func (c *Callback) reorder() {
	c.processors.reorder(c)
}

// Create could be used to register callbacks for creating object
//     db.Callback().Create().After("gorm:create").Register("plugin:run_after_create", func(*Scope) {
//       // business logic
//       ...
//
//       // set error if some thing wrong happened, will rollback the creating
//       scope.Err(errors.New("error"))
//     })
func (c *Callback) Create() *CallbackProcessor {
	return &CallbackProcessor{kind: CREATE_CALLBACK, parent: c}
}

// Update could be used to register callbacks for updating object, refer `Create` for usage
func (c *Callback) Update() *CallbackProcessor {
	return &CallbackProcessor{kind: UPDATE_CALLBACK, parent: c}
}

// Delete could be used to register callbacks for deleting object, refer `Create` for usage
func (c *Callback) Delete() *CallbackProcessor {
	return &CallbackProcessor{kind: DELETE_CALLBACK, parent: c}
}

// Query could be used to register callbacks for querying objects with query methods like `Find`, `First`, `Related`, `Association`...
// Refer `Create` for usage
func (c *Callback) Query() *CallbackProcessor {
	return &CallbackProcessor{kind: QUERY_CALLBACK, parent: c}
}

// RowQuery could be used to register callbacks for querying objects with `Row`, `Rows`, refer `Create` for usage
func (c *Callback) RowQuery() *CallbackProcessor {
	return &CallbackProcessor{kind: ROW_QUERY_CALLBACK, parent: c}
}

//Added for tests : DO NOT USE DIRECTLY
func (c *Callback) GetCreates() ScopedFuncs {
	return c.creates
}

func (c *Callback) GetUpdates() ScopedFuncs {
	return c.updates
}

func (c *Callback) GetQueries() ScopedFuncs {
	return c.queries
}

func (c *Callback) GetDeletes() ScopedFuncs {
	return c.deletes
}
