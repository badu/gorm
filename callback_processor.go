package gorm

// After insert a new callback after callback `callbackName`, refer `Callbacks.Create`
func (cp *CallbacksProcessor) After(callbackName string) *CallbacksProcessor {
	cp.after = callbackName
	return cp
}

// Before insert a new callback before callback `callbackName`, refer `Callbacks.Create`
func (cp *CallbacksProcessor) Before(callbackName string) *CallbacksProcessor {
	cp.before = callbackName
	return cp
}

// Register a new callback, refer `Callbacks.Create`
func (cp *CallbacksProcessor) Register(callbackName string, callback ScopedFunc) {
	cp.name = callbackName
	cp.processor = &callback
	cp.parent.processors.add(cp)
	cp.parent.reorder()
}

// Remove a registered callback
//     db.Callback().Create().Remove("gorm:update_time_stamp_when_create")
func (cp *CallbacksProcessor) Remove(callbackName string) {
	//fmt.Printf("[info] removing callback `%v` from %v\n", callbackName, fileWithLineNum())
	cp.name = callbackName
	cp.remove = true
	cp.parent.processors.add(cp)
	cp.parent.reorder()
}

// Replace a registered callback with new callback
//     db.Callback().Create().Replace("gorm:update_time_stamp_when_create", func(*Scope) {
//		   scope.SetColumn("Created", now)
//		   scope.SetColumn("Updated", now)
//     })
func (cp *CallbacksProcessor) Replace(callbackName string, callback ScopedFunc) {
	//fmt.Printf("[info] replacing callback `%v` from %v\n", callbackName, fileWithLineNum())
	cp.name = callbackName
	cp.processor = &callback
	cp.replace = true
	cp.parent.processors.add(cp)
	cp.parent.reorder()
}

// Get registered callback
//    db.Callback().Create().Get("gorm:create")
func (cp *CallbacksProcessor) Get(callbackName string) ScopedFunc {
	for _, p := range cp.parent.processors {
		if p.name == callbackName && p.kind == cp.kind && !cp.remove {
			return *p.processor
		}
	}
	return nil
}

//sorts callback processors
func (cp *CallbacksProcessor) sortCallbackProcessor(allNames, sortedNames *StrSlice, parent CallbacksProcessors) {
	if sortedNames.rIndex(cp.name) == -1 { // if not sorted
		if cp.before != "" {
			// if defined before callback
			if idx := sortedNames.rIndex(cp.before); idx != -1 {
				// if before callback already sorted, append current callback just after it
				sortedNames.insertAt(idx, cp.name)
			} else if index := allNames.rIndex(cp.before); index != -1 {
				// if before callback exists but haven't sorted, append current callback to last
				sortedNames.add(cp.name)
				//sort next
				nextCp := parent[index]
				nextCp.sortCallbackProcessor(allNames, sortedNames, parent)
			}
		}

		if cp.after != "" {
			// if defined after callback
			if index := sortedNames.rIndex(cp.after); index != -1 {
				// if after callback already sorted, append current callback just before it
				sortedNames.insertAt(index+1, cp.name)
			} else if idx := allNames.rIndex(cp.after); idx != -1 {
				//sort next
				// if after callback exists but haven't sorted
				nextCp := parent[idx]
				// set after callback's before callback to current callback
				if nextCp.before == "" {
					nextCp.before = cp.name
				}
				nextCp.sortCallbackProcessor(allNames, sortedNames, parent)
			}
		}

		// if current callback haven't been sorted, append it to last
		if sortedNames.rIndex(cp.name) == -1 {
			sortedNames.add(cp.name)
		}
	}
}
