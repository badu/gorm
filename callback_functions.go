package gorm

//============================================
//Callback create functions
//============================================
// beforeCreateCallback will invoke `BeforeSave`, `BeforeCreate` method before creating
func beforeCreateCallback(scope *Scope) {
	scope.beforeCreateCallback()
}

// updateTimeStampForCreateCallback will set `CreatedAt`, `UpdatedAt` when creating
func updateTimeStampForCreateCallback(scope *Scope) {
	scope.updateTimeStampForCreateCallback()
}

// createCallback the callback used to insert data into database
func createCallback(scope *Scope) {
	scope.createCallback()
}

// forceReloadAfterCreateCallback will reload columns that having default value, and set it back to current object
func forceReloadAfterCreateCallback(scope *Scope) {
	scope.forceReloadAfterCreateCallback()
}

// afterCreateCallback will invoke `AfterCreate`, `AfterSave` method after creating
func afterCreateCallback(scope *Scope) {
	scope.afterCreateCallback()
}

//============================================
// Callback save functions
//============================================
func beginTransactionCallback(scope *Scope) {
	scope.Begin()
}

func commitOrRollbackTransactionCallback(scope *Scope) {
	scope.CommitOrRollback()
}

func saveBeforeAssociationsCallback(scope *Scope) {
	scope.saveBeforeAssociationsCallback()
}

func saveAfterAssociationsCallback(scope *Scope) {
	scope.saveAfterAssociationsCallback()
}

//============================================
// Callback update functions
//============================================
// assignUpdatingAttributesCallback assign updating attributes to model
func assignUpdatingAttributesCallback(scope *Scope) {
	scope.assignUpdatingAttributesCallback()
}

// beforeUpdateCallback will invoke `BeforeSave`, `BeforeUpdate` method before updating
func beforeUpdateCallback(scope *Scope) {
	scope.beforeUpdateCallback()
}

// updateTimeStampForUpdateCallback will set `UpdatedAt` when updating
func updateTimeStampForUpdateCallback(scope *Scope) {
	scope.updateTimeStampForUpdateCallback()
}

// updateCallback the callback used to update data to database
func updateCallback(scope *Scope) {
	scope.updateCallback()
}

// afterUpdateCallback will invoke `AfterUpdate`, `AfterSave` method after updating
func afterUpdateCallback(scope *Scope) {
	scope.afterUpdateCallback()
}

//============================================
// Callback query functions
//============================================
// queryCallback used to query data from database
func queryCallback(scope *Scope) {
	scope.queryCallback()
}

// afterQueryCallback will invoke `AfterFind` method after querying
func afterQueryCallback(scope *Scope) {
	scope.afterQueryCallback()
}

//============================================
// Callback query preload function
//============================================
// preloadCallback used to preload associations
func preloadCallback(scope *Scope) {
	scope.preloadCallback()
}

//============================================
// Callback delete functions
//============================================
// beforeDeleteCallback will invoke `BeforeDelete` method before deleting
func beforeDeleteCallback(scope *Scope) {
	scope.beforeDeleteCallback()
}

// deleteCallback used to delete data from database or set deleted_at to current time (when using with soft delete)
func deleteCallback(scope *Scope) {
	scope.deleteCallback()
}

// afterDeleteCallback will invoke `AfterDelete` method after deleting
func afterDeleteCallback(scope *Scope) {
	scope.afterDeleteCallback()
}
