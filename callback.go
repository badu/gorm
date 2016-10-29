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
	return &CallbackProcessor{kind: "create", parent: c}
}

// Update could be used to register callbacks for updating object, refer `Create` for usage
func (c *Callback) Update() *CallbackProcessor {
	return &CallbackProcessor{kind: "update", parent: c}
}

// Delete could be used to register callbacks for deleting object, refer `Create` for usage
func (c *Callback) Delete() *CallbackProcessor {
	return &CallbackProcessor{kind: "delete", parent: c}
}

// Query could be used to register callbacks for querying objects with query methods like `Find`, `First`, `Related`, `Association`...
// Refer `Create` for usage
func (c *Callback) Query() *CallbackProcessor {
	return &CallbackProcessor{kind: "query", parent: c}
}

// RowQuery could be used to register callbacks for querying objects with `Row`, `Rows`, refer `Create` for usage
func (c *Callback) RowQuery() *CallbackProcessor {
	return &CallbackProcessor{kind: "row_query", parent: c}
}

// reorder all registered processors, and reset CURD callbacks
func (c *Callback) reorder() {
	var creates, updates, deletes, queries, rowQueries []*CallbackProcessor

	for _, processor := range c.processors {
		if processor.name != "" {
			switch processor.kind {
			case "create":
				creates = append(creates, processor)
			case "update":
				updates = append(updates, processor)
			case "delete":
				deletes = append(deletes, processor)
			case "query":
				queries = append(queries, processor)
			case "row_query":
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
