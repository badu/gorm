package gorm

const (
	//Callback Kind constants
	CREATE_CALLBACK    uint8 = 1
	UPDATE_CALLBACK    uint8 = 2
	DELETE_CALLBACK    uint8 = 3
	QUERY_CALLBACK     uint8 = 4
	ROW_QUERY_CALLBACK uint8 = 5
)

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

func takeAddr(callback ScopedFunc) *ScopedFunc {
	return &callback
}

func (c *Callback) registerGORMDefaultCallbacks() {
	c.processors.add(
		// Define callbacks for creating
		&CallbackProcessor{
			kind:      CREATE_CALLBACK,
			parent:    c,
			name:      "gorm:begin_transaction",
			processor: takeAddr(beginTransactionCallback),
		},
		&CallbackProcessor{
			kind:      CREATE_CALLBACK,
			parent:    c,
			name:      "gorm:before_create",
			processor: takeAddr(beforeCreateCallback),
		},
		&CallbackProcessor{
			kind:      CREATE_CALLBACK,
			parent:    c,
			name:      "gorm:save_before_associations",
			processor: takeAddr(saveBeforeAssociationsCallback),
		},
		&CallbackProcessor{
			kind:      CREATE_CALLBACK,
			parent:    c,
			name:      "gorm:update_time_stamp",
			processor: takeAddr(updateTimeStampForCreateCallback),
		},
		&CallbackProcessor{
			kind:      CREATE_CALLBACK,
			parent:    c,
			name:      "gorm:create",
			processor: takeAddr(createCallback),
		},
		&CallbackProcessor{
			kind:      CREATE_CALLBACK,
			parent:    c,
			name:      "gorm:force_reload_after_create",
			processor: takeAddr(forceReloadAfterCreateCallback),
		},
		&CallbackProcessor{
			kind:      CREATE_CALLBACK,
			parent:    c,
			name:      "gorm:save_after_associations",
			processor: takeAddr(saveAfterAssociationsCallback),
		},
		&CallbackProcessor{
			kind:      CREATE_CALLBACK,
			parent:    c,
			name:      "gorm:after_create",
			processor: takeAddr(afterCreateCallback),
		},
		&CallbackProcessor{
			kind:      CREATE_CALLBACK,
			parent:    c,
			name:      "gorm:commit_or_rollback_transaction",
			processor: takeAddr(commitOrRollbackTransactionCallback),
		},
		// Define callbacks for deleting
		&CallbackProcessor{
			kind:      DELETE_CALLBACK,
			parent:    c,
			name:      "gorm:begin_transaction",
			processor: takeAddr(beginTransactionCallback),
		},
		&CallbackProcessor{
			kind:      DELETE_CALLBACK,
			parent:    c,
			name:      "gorm:before_delete",
			processor: takeAddr(beforeDeleteCallback),
		},
		&CallbackProcessor{
			kind:      DELETE_CALLBACK,
			parent:    c,
			name:      "gorm:delete",
			processor: takeAddr(deleteCallback),
		},
		&CallbackProcessor{
			kind:      DELETE_CALLBACK,
			parent:    c,
			name:      "gorm:after_delete",
			processor: takeAddr(afterDeleteCallback),
		},
		&CallbackProcessor{
			kind:      DELETE_CALLBACK,
			parent:    c,
			name:      "gorm:commit_or_rollback_transaction",
			processor: takeAddr(commitOrRollbackTransactionCallback),
		},
		// Define callbacks for querying
		&CallbackProcessor{
			kind:      QUERY_CALLBACK,
			parent:    c,
			name:      "gorm:query",
			processor: takeAddr(queryCallback),
		},
		&CallbackProcessor{
			kind:      QUERY_CALLBACK,
			parent:    c,
			name:      "gorm:preload",
			processor: takeAddr(preloadCallback),
		},
		&CallbackProcessor{
			kind:      QUERY_CALLBACK,
			parent:    c,
			name:      "gorm:after_query",
			processor: takeAddr(afterQueryCallback),
		},
		// Define callbacks for updating

		&CallbackProcessor{
			kind:      UPDATE_CALLBACK,
			parent:    c,
			name:      "gorm:assign_updating_attributes",
			processor: takeAddr(assignUpdatingAttributesCallback),
		},
		&CallbackProcessor{
			kind:      UPDATE_CALLBACK,
			parent:    c,
			name:      "gorm:begin_transaction",
			processor: takeAddr(beginTransactionCallback),
		},
		&CallbackProcessor{
			kind:      UPDATE_CALLBACK,
			parent:    c,
			name:      "gorm:before_update",
			processor: takeAddr(beforeUpdateCallback),
		},
		&CallbackProcessor{
			kind:      UPDATE_CALLBACK,
			parent:    c,
			name:      "gorm:save_before_associations",
			processor: takeAddr(saveBeforeAssociationsCallback),
		},
		&CallbackProcessor{
			kind:      UPDATE_CALLBACK,
			parent:    c,
			name:      "gorm:update_time_stamp",
			processor: takeAddr(updateTimeStampForUpdateCallback),
		},
		&CallbackProcessor{
			kind:      UPDATE_CALLBACK,
			parent:    c,
			name:      "gorm:update",
			processor: takeAddr(updateCallback),
		},
		&CallbackProcessor{
			kind:      UPDATE_CALLBACK,
			parent:    c,
			name:      "gorm:save_after_associations",
			processor: takeAddr(saveAfterAssociationsCallback),
		},
		&CallbackProcessor{
			kind:      UPDATE_CALLBACK,
			parent:    c,
			name:      "gorm:after_update",
			processor: takeAddr(afterUpdateCallback),
		},
		&CallbackProcessor{
			kind:      UPDATE_CALLBACK,
			parent:    c,
			name:      "gorm:commit_or_rollback_transaction",
			processor: takeAddr(commitOrRollbackTransactionCallback),
		},
	)
	//finally, we call reorder
	c.processors.reorder(c)
}
