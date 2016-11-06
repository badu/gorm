# Todo
- [ ] Extract strings from dialect_postgres.go
- [ ] Extracted strings from dialect_sqlite3.go
- [ ] Documentation for tests and build examples
- [ ] Stringer implementation on all structs for debugging
- [ ] Reorganize local vars from various places
- [ ] Reorganize deferred functions from various places
- [ ] Extract strings from code (make constants)
- [ ] Collect errors and their messages in one place
- [ ] replace slices of strings with 
        `var buf bytes.Buffer
        buf.WriteString("string")
        buf.Write([]byte("bytes"))
        return buf.String()`
. Also, organize fmt.Sprintf to be called once

# Comments and thoughts
- Generated SQL let's the SQL engine cast : SELECT * FROM aTable WHERE id = '1' (id being int). I think it's a bad practice and it should be fixed

# Breaking changes
- DB struct - renamed to DBCon, since that is what it represents
- Removed MSSQL support - out of my concerns with this project

# Changes log

## 06.11.2016
- [x] got rid of parseTagSetting method from utils.go

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