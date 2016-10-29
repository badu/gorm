#TODO
- [] StructField should swallow Field model field definition
- [] Concurent slice and map where needed
- [] Stringer implementation on all structs for debugging
- [] Documentation for tests - build examples
- [] Fix test named TestNot
```
func TestNot(t *testing.T) {

}
```
# Changes log
## 29.10.2016
- [x] Moved code around
- [x] Numbered tests - so I can track what fails
- [x] Replaced some string constants like "many_to_many" and refactor accordingly
- [x] StructField is parsing by it's own gorm and sql tags with method ParseTagSettings
- [x] Replaced string constants for the tags and created a map string-to-uint8
- [x] Removed field Name from StructField since Struct property of it exposes Name
- [x] Created method GetName() for StructField to return that name
- [x] Created method GetTag() for StructField to return Struct property Tag (seems unused)