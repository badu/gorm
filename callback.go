package gorm

func (c *Callbacks) clone() *Callbacks {
	return &Callbacks{
		creates:    c.creates,
		updates:    c.updates,
		deletes:    c.deletes,
		queries:    c.queries,
		rowQueries: c.rowQueries,
		processors: c.processors,
	}
}

// reorder all registered processors, and reset CURD callbacks
func (c *Callbacks) reorder() {
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
func (c *Callbacks) Create() *CallbacksProcessor {
	return &CallbacksProcessor{kind: cb_create, parent: c}
}

// Update could be used to register callbacks for updating object, refer `Create` for usage
func (c *Callbacks) Update() *CallbacksProcessor {
	return &CallbacksProcessor{kind: cb_update, parent: c}
}

// Delete could be used to register callbacks for deleting object, refer `Create` for usage
func (c *Callbacks) Delete() *CallbacksProcessor {
	return &CallbacksProcessor{kind: cb_delete, parent: c}
}

// Query could be used to register callbacks for querying objects with query methods like `Find`, `First`, `Related`, `Association`...
// Refer `Create` for usage
func (c *Callbacks) Query() *CallbacksProcessor {
	return &CallbacksProcessor{kind: cb_query, parent: c}
}

// RowQuery could be used to register callbacks for querying objects with `Row`, `Rows`, refer `Create` for usage
func (c *Callbacks) RowQuery() *CallbacksProcessor {
	return &CallbacksProcessor{kind: cb_row, parent: c}
}

//Added for tests : DO NOT USE DIRECTLY
func (c *Callbacks) GetCreates() ScopedFuncs {
	return c.creates
}

func (c *Callbacks) GetUpdates() ScopedFuncs {
	return c.updates
}

func (c *Callbacks) GetQueries() ScopedFuncs {
	return c.queries
}

func (c *Callbacks) GetDeletes() ScopedFuncs {
	return c.deletes
}
