# Todo
- [ ] Create method named ComponentScan which is called either in Automigrate and stores the reflected entities in a cache
- [ ] Concurent slice and map where needed
- [ ] Documentation for tests and build examples
- [ ] Stringer implementation on all structs for debugging
- [ ] Reorganize local vars from various places
- [ ] Fix test named TestNot
```
func TestNot(t *testing.T) {

}
```
# Decisions

- [ ] Instead of cloning Scope, maybe it's a better idea to have a scope pool to reuse objects

# Changes log

## 30.10.2016 (Operation Field -> StructField)
- [ ] Stringer implementation StructField
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