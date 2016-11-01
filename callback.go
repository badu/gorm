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

// reorder all registered processors, and reset CURD callbacks
func (c *Callback) reorder() {
	var creates, updates, deletes, queries, rowQueries []*CallbackProcessor

	for _, processor := range c.processors {
		if processor.name != "" {
			switch processor.kind {
			case CREATE_CALLBACK:
				creates = append(creates, processor)
			case UPDATE_CALLBACK:
				updates = append(updates, processor)
			case DELETE_CALLBACK:
				deletes = append(deletes, processor)
			case QUERY_CALLBACK:
				queries = append(queries, processor)
			case ROW_QUERY_CALLBACK:
				rowQueries = append(rowQueries, processor)
			}
		}
	}

	c.creates = sortProcessors(creates)
	c.updates = sortProcessors(updates)
	c.deletes = sortProcessors(deletes)
	c.queries = sortProcessors(queries)
	c.rowQueries = sortProcessors(rowQueries)
}
//Added for tests : DO NOT USE DIRECTLY
func (c *Callback) GetCreates() []*func(scope *Scope){
	return c.creates
}

func (c *Callback) GetUpdates() []*func(scope *Scope){
	return c.updates
}

func (c *Callback) GetQueries() []*func(scope *Scope){
	return c.queries
}
func (c *Callback) GetDeletes() []*func(scope *Scope){
	return c.deletes
}