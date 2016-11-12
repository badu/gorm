# Todo
- [ ] Debug SQL string even when it fails
- [ ] All create, migrate and alter functions should be moved from the scope inside a separate object 
(since we're automigrating just at startup) 
- [ ] Documentation for tests and build examples
- [ ] Stringer implementation on all structs for debugging
- [ ] Extract strings from code (make constants)
- [ ] Collect errors and their messages in one place
- [ ] replace slices of strings with Collector
- [ ] make StructField be able to provide a value
- [ ] make Relationship have some methods so we can move code from ModelStruct
- [ ] Relationships should be kept by ModelStruct

# Comments and thoughts
- Generated SQL let's the SQL engine cast : SELECT * FROM aTable WHERE id = '1' (id being int). I think it's a bad practice and it should be fixed
- I'm almost convinced that default Callbacks should be Scope methods :
 handleHasOnePreload, handleHasManyPreload, handleBelongsToPreload, handleManyToManyPreload are all in Scope, but called
 from preloadCallback in callback_functions.go. Anyway, callbacks needs reconsideration - it has good parts (customization) and bad parts 
 (the separation is artificial, since if you remove default callbacks, gorm won't work as intended)
- There are so many checks all over the place, which show insecurity. For example, we know what's to know about a StructField - maybe it's best 
to have the dereferenced pointer to the struct/slice kept inside. Same goes for Scope...
- Set and Get, SetInstance and all that, should be nicer

# Last merge
- #1242 - "(Make gorm.Errors available for use outside gorm #1242)" 
 
# Breaking changes
- DB struct - renamed to DBCon, since that is what it represents.
    However, you can do the following, to use the old gorm.DB:
    `dbcon, err := gorm.Open("mysql", dbstr+"?parseTime=true")`
    `db = &gorm.DB{*dbcon}`
- Removed MSSQL support - out of my concerns with this project

# Changes log

## 12.11.2016
- [x] switched bitflag from uint64 to uint16 (we really don't need more than 16 at the time)
- [x] make ModelStruct map it's fields : fieldsMap struct
- [x] make ModelStruct map it's fields : logic modification in fieldByName() - if mapped not found, looking into NamesMap
- [x] make ModelStruct map it's fields : ModelStruct has addField(field) method
- [x] make ModelStruct map it's fields : ModelStruct has addPK(field) method (primary keys)
- [x] make ModelStruct map it's fields : ModelStruct has HasColumn(name) method 
- [x] make ModelStruct map it's fields : removed Scope method HasColumn(name)
- [x] refactored Scope Fields() method - calls a cloneStructFields method of ModelStruct
- [x] simplified further the GetModelStruct() of Scope to cleanup the fields mess

## 11.11.2016
- [x] instead of having this bunch of flags in StructField - bitflag
- [x] removed joinTableHandlers property from DBCon (was probably leftover of work in progress)
- [x] simplified Setup(relationship *Relationship, source reflect.Type, destination reflect.Type) of JoinTableHandlerInterface
- [x] added SetTable(name string) to JoinTableHandlerInterface
- [x] renamed property "db" of DBCon to "sqli"
- [x] renamed interface sqlCommon to sqlInterf
- [x] renamed property "db" of Scope to "con"
- [x] renamed property "db" of search struct to "con"
- [x] search struct has collectAttrs() method which loads the cached selectAttrs of the Scope

## 10.11.2016
- [x] StructField has field UnderlyingType (should keep reflect.Type so we won't use reflection everywhere)
- [x] finally got rid of defer inside loop of Scope's GetModelStruct method (ModelStruct's processRelations method has a loop in which calls relationship processors)
- [x] introduced a HasRelations and IsTime in StructField

## 09.11.2016
- [x] Collector - a helper to avoid multiple calls on fmt.Sprintf : stores values and string
- [x] replaced some statements with switch
- [x] GetModelStruct refactoring
- [x] GromErrors change and fix (from original gorm commits)

## 08.11.2016
- [x] adopted skip association tag from https://github.com/slockij/gorm (`gorm:"save_associations:false"`)
- [x] adopted db.Raw().First() makes wrong sql fix #1214 #1243
- [x] registerGORMDefaultCallbacks() calls reorder at the end of registration
- [x] Scope toQueryCondition() from utils.go
- [x] Moved callbacks into Scope (needs closure functions)
- [x] Removed some postgres specific functions from utils.go

## 07.11.2016
- [x] have NOT integrate original-gorm pull request #1252 (prevent delete/update if conditions are not met, thus preventing delete-all, update-all) tests fail
- [x] have looked upon all original-gorm pull requests - and decided to skip them 
- [x] have NOT integrate original-gorm pull request #1251 - can be done with Scanner and Valuer
- [x] have NOT integrate original-gorm pull request #1242 - can be done simplier
- [x] ParseFieldStructForDialect() moved to struct_field.go from utils.go
- [x] makeSlice() moved to struct_field.go from utils.go
- [x] indirect() from utils.go, swallowed where needed (shows logic better when dereferencing pointer)
- [x] file for expr struct (will add more functionality)
- [x] cloneWithValue() method in db.go 

## 06.11.2016
- [x] got rid of parseTagSetting method from utils.go
- [x] moved ToDBName into safe_map.go - renamed smap to NamesMap
- [x] StrSlice is used in Relationship
- [x] more cleanups
- [x] more renaming

## 05.11.2016
- [x] DefaultCallback removed from types - it's made under open and registers all callbacks there
- [x] callback.go has now a method named registerDefaults
- [x] scope's GetModelStruct refactored and fixed a few lint problems

## 02.11.2016
- [x] avoid unnecessary calls in CallbackProcessors reorder method (lengths zero)
- [x] Refactored sortProcessors not to be recursive, but have a method called sortCallbackProcessor inside CallbackProcessor
- [x] Concurent slice and map in utils (so far, unused)
- [x] type CallbackProcessors []*CallbackProcessor for readability
- [x] callback_processors.go file which holds methods for type CallbackProcessors (add, len)
- [x] moved sortProcessors from utils.go to callback_processors.go as method
- [x] created type ScopedFunc  func(*Scope)
- [x] created type ScopedFuncs []*ScopedFunc
- [x] replaced types ScopedFunc and ScopedFuncs to be more readable  

## 01.11.2016
- [x] TestCloneSearch could not be moved
- [x] Exposed some methods on Callback for tests to run (GetCreates, GetUpdates, GetQueries, GetDeletes)
- [x] Moved tests to tests folder (helps IDE)
- [x] Extracted strings from dialect_common.go
- [x] Extracted strings from dialect_mysql.go
- [x] Modified some variable names to comply to linter ("collides with imported package name")
- [x] Remove explicit variable name on returns
- [x] Removed method newDialect from utils.go (moved it into Open() method)
- [x] Removed MSSQL support - out of my concerns with this project
- [x] Fix (chore) in StructField Set method : if implements Scanner don't attempt to convert, just pass it over
- [x] Test named TestNot
- [x] CallbackProcessor kind field changed from string to uint8

## 30.10.2016 (Others)
- [x] DB struct - renamed to DBCon, since that is what it represents

## 30.10.2016 (Operation Field -> StructField)

- [x] StructFields has it's own file to get rid of append() everywhere in the code
- [x] TagSettings map[uint8]string in StructField will become a struct by itself, to support has(), get(), set(), clone(), loadFromTags()
- [x] TagSettings should be private in StructField
- [x] replace everywhere []*StructField with type StructFields
- [x] create StructFields type []*StructField for code readability
- [x] NewStructField method to create StructField from reflect.StructField
- [x] Field struct, "Field" property renamed to "Value", since it is a reflect.Value
- [x] StructField should swallow Field model field definition
- [x] created cloneWithValue(value reflect.Value) on StructField -> calls setIsBlank()
- [x] moved isBlank(fieldValue) from utils to StructField named setIsBlank()
- [x] remove getForeignField from utils.go -> ModelStruct has a method called getForeignField(fieldName string)

## 29.10.2016

- [x] Moved code around
- [x] Numbered tests - so I can track what fails
- [x] Replaced some string constants like "many_to_many" and refactor accordingly
- [x] StructField is parsing by it's own gorm and sql tags with method ParseTagSettings
- [x] Replaced string constants for the tags and created a map string-to-uint8
- [x] Removed field Name from StructField since Struct property of it exposes Name
- [x] Created method GetName() for StructField to return that name
- [x] Created method GetTag() for StructField to return Struct property Tag (seems unused)