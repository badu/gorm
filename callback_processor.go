package gorm

import "fmt"

// After insert a new callback after callback `callbackName`, refer `Callbacks.Create`
func (cp *CallbackProcessor) After(callbackName string) *CallbackProcessor {
	cp.after = callbackName
	return cp
}

// Before insert a new callback before callback `callbackName`, refer `Callbacks.Create`
func (cp *CallbackProcessor) Before(callbackName string) *CallbackProcessor {
	cp.before = callbackName
	return cp
}

// Register a new callback, refer `Callbacks.Create`
func (cp *CallbackProcessor) Register(callbackName string, callback func(scope *Scope)) {
	cp.name = callbackName
	cp.processor = &callback
	cp.parent.processors = append(cp.parent.processors, cp)
	cp.parent.reorder()
}

// Remove a registered callback
//     db.Callback().Create().Remove("gorm:update_time_stamp_when_create")
func (cp *CallbackProcessor) Remove(callbackName string) {
	fmt.Printf("[info] removing callback `%v` from %v\n", callbackName, fileWithLineNum())
	cp.name = callbackName
	cp.remove = true
	cp.parent.processors = append(cp.parent.processors, cp)
	cp.parent.reorder()
}

// Replace a registered callback with new callback
//     db.Callback().Create().Replace("gorm:update_time_stamp_when_create", func(*Scope) {
//		   scope.SetColumn("Created", now)
//		   scope.SetColumn("Updated", now)
//     })
func (cp *CallbackProcessor) Replace(callbackName string, callback func(scope *Scope)) {
	fmt.Printf("[info] replacing callback `%v` from %v\n", callbackName, fileWithLineNum())
	cp.name = callbackName
	cp.processor = &callback
	cp.replace = true
	cp.parent.processors = append(cp.parent.processors, cp)
	cp.parent.reorder()
}

// Get registered callback
//    db.Callback().Create().Get("gorm:create")
func (cp *CallbackProcessor) Get(callbackName string) (callback func(scope *Scope)) {
	for _, p := range cp.parent.processors {
		if p.name == callbackName && p.kind == cp.kind && !cp.remove {
			return *p.processor
		}
	}
	return nil
}
