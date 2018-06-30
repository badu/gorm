package gorm

// After insert a new callback after callback `callbackName`, refer `Callbacks.Create`
func (c *CallbacksProcessor) After(callbackName string) *CallbacksProcessor {
	c.after = callbackName
	return c
}

// Before insert a new callback before callback `callbackName`, refer `Callbacks.Create`
func (c *CallbacksProcessor) Before(callbackName string) *CallbacksProcessor {
	c.before = callbackName
	return c
}

// Register a new callback, refer `Callbacks.Create`
func (c *CallbacksProcessor) Register(callbackName string, callback ScopedFunc) {
	c.name = callbackName
	c.processor = &callback
	c.parent.processors.add(c)
	c.parent.reorder()
}

// Remove a registered callback
//     db.Callback().Create().Remove("gorm:update_time_stamp_when_create")
func (c *CallbacksProcessor) Remove(callbackName string) {
	//fmt.Printf("[info] removing callback `%v` from %v\n", callbackName, fileWithLineNum())
	c.name = callbackName
	c.remove = true
	c.parent.processors.add(c)
	c.parent.reorder()
}

// Replace a registered callback with new callback
//     db.Callback().Create().Replace("gorm:update_time_stamp_when_create", func(*Scope) {
//		   scope.SetColumn("Created", now)
//		   scope.SetColumn("Updated", now)
//     })
func (c *CallbacksProcessor) Replace(callbackName string, callback ScopedFunc) {
	//fmt.Printf("[info] replacing callback `%v` from %v\n", callbackName, fileWithLineNum())
	c.name = callbackName
	c.processor = &callback
	c.replace = true
	c.parent.processors.add(c)
	c.parent.reorder()
}

// Get registered callback
//    db.Callback().Create().Get("gorm:create")
func (c *CallbacksProcessor) Get(callbackName string) ScopedFunc {
	for _, p := range c.parent.processors {
		if p.name == callbackName && p.kind == c.kind && !c.remove {
			return *p.processor
		}
	}
	return nil
}

//sorts callback processors
func (c *CallbacksProcessor) sortCallbackProcessor(allNames, sortedNames *StrSlice, parent CallbacksProcessors) {
	if sortedNames.rIndex(c.name) == -1 { // if not sorted
		if c.before != "" {
			// if defined before callback
			if idx := sortedNames.rIndex(c.before); idx != -1 {
				// if before callback already sorted, append current callback just after it
				sortedNames.insertAt(idx, c.name)
			} else if index := allNames.rIndex(c.before); index != -1 {
				// if before callback exists but haven't sorted, append current callback to last
				sortedNames.add(c.name)
				//sort next
				nextCp := parent[index]
				nextCp.sortCallbackProcessor(allNames, sortedNames, parent)
			}
		}

		if c.after != "" {
			// if defined after callback
			if index := sortedNames.rIndex(c.after); index != -1 {
				// if after callback already sorted, append current callback just before it
				sortedNames.insertAt(index+1, c.name)
			} else if idx := allNames.rIndex(c.after); idx != -1 {
				//sort next
				// if after callback exists but haven't sorted
				nextCp := parent[idx]
				// set after callback's before callback to current callback
				if nextCp.before == "" {
					nextCp.before = c.name
				}
				nextCp.sortCallbackProcessor(allNames, sortedNames, parent)
			}
		}

		// if current callback haven't been sorted, append it to last
		if sortedNames.rIndex(c.name) == -1 {
			sortedNames.add(c.name)
		}
	}
}
