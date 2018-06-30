package gorm

import "fmt"

func (p *CallbacksProcessors) add(proc ...*CallbacksProcessor) {
	*p = append(*p, proc...)
}

func (p *CallbacksProcessors) len() int {
	return len(*p)
}

func (p *CallbacksProcessors) reorder(ofCallback *Callbacks) {
	var creates, updates, deletes, queries, rowQueries CallbacksProcessors
	//collect processors by their kind
	for _, processor := range *p {
		if processor.name != "" {
			switch processor.kind {
			case createCallback:
				creates.add(processor)
			case updateCallback:
				updates.add(processor)
			case deleteCallback:
				deletes.add(processor)
			case queryCallback:
				queries.add(processor)
			case rowCallback:
				rowQueries.add(processor)
			}
		}
	}
	//avoid unnecessary calls
	if creates.len() > 0 {
		ofCallback.creates = creates.sortProcessors()
	}
	if updates.len() > 0 {
		ofCallback.updates = updates.sortProcessors()
	}
	if deletes.len() > 0 {
		ofCallback.deletes = deletes.sortProcessors()
	}
	if queries.len() > 0 {
		ofCallback.queries = queries.sortProcessors()
	}
	if rowQueries.len() > 0 {
		ofCallback.rowQueries = rowQueries.sortProcessors()
	}

}

// sortProcessors sort callback processors based on its before, after, remove, replace
func (p CallbacksProcessors) sortProcessors() ScopedFuncs {
	var allNames, sortedNames StrSlice
	var sortedFuncs ScopedFuncs

	for _, cp := range p {
		// show warning message the callback name already exists
		if index := allNames.rIndex(cp.name); index > -1 && !cp.replace && !cp.remove {
			fmt.Printf("[warning] duplicated callback `%v` from %v\n", cp.name, fileWithLineNum())
		}
		allNames.add(cp.name)
	}
	for _, cp := range p {
		cp.sortCallbackProcessor(&allNames, &sortedNames, p)
	}

	for _, name := range sortedNames {
		if index := allNames.rIndex(name); !p[index].remove {
			sortedFuncs.add(p[index].processor)
		}
	}
	return sortedFuncs
}
