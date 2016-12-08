package gorm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unsafe"
)

// New create a new Scope without search information
func (scope *Scope) NewScope(value interface{}) *Scope {
	return &Scope{
		con:    newCon(scope.con),
		Search: &Search{Conditions: make(SqlConditions)},
		Value:  value}
}

////////////////////////////////////////////////////////////////////////////////
// Setter-Getters
////////////////////////////////////////////////////////////////////////////////

// Set set value by name
func (scope *Scope) Set(settingType uint64, value interface{}) *Scope {
	scope.con.instanceSet(settingType, value)
	return scope
}

// Get get setting by name
func (scope *Scope) Get(settingType uint64) (interface{}, bool) {
	return scope.con.get(settingType)
}

// InstanceSet set instance setting for current operation,
// but not for operations in callbacks,
// like saving associations callback
func (scope *Scope) InstanceSet(settingType uint64, value interface{}) *Scope {
	if scope.instanceID <= 0 {
		//gets the pointer of self and convert it to uint64 - it's unique enough, since no two scopes can share same address
		scope.instanceID = *(*uint64)(unsafe.Pointer(&scope))
	}
	return scope.Set(scope.instanceID+settingType, value)
}

// InstanceGet get instance setting from current operation
func (scope *Scope) InstanceGet(settingType uint64) (interface{}, bool) {
	if scope.instanceID <= 0 {
		//gets the pointer of self and convert it to uint64 - it's unique enough, since no two scopes can share same address
		scope.instanceID = *(*uint64)(unsafe.Pointer(&scope))
	}
	return scope.Get(scope.instanceID + settingType)
}

////////////////////////////////////////////////////////////////////////////////
// Scope DB
////////////////////////////////////////////////////////////////////////////////
// Err add error to Scope
func (scope *Scope) Err(err error) error {
	if err != nil {
		scope.con.AddError(err)
	}
	return err
}

// HasError check if there are any error
func (scope *Scope) HasError() bool {
	return scope.con.Error != nil
}

func (scope *Scope) Warn(v ...interface{}) {
	//scope.con.warnLog(v...)
}

// Fields get value's fields from ModelStruct
func (scope *Scope) Fields() StructFields {
	if scope.fields == nil {
		var (
			result        StructFields
			modelStruct   = scope.GetModelStruct()
			scopeValue    = IndirectValue(scope.Value)
			scopeIsStruct = scopeValue.Kind() == reflect.Struct
		)
		for _, field := range modelStruct.fieldsMap.fields {
			if scopeIsStruct {
				fieldValue := scopeValue
				for _, name := range field.Names {
					fieldValue = reflect.Indirect(fieldValue).FieldByName(name)
				}
				clonedField := field.cloneWithValue(fieldValue)
				result.add(clonedField)
			} else {
				clonedField := field.clone()
				clonedField.SetIsBlank()
				result.add(clonedField)
			}
		}
		scope.fields = &result
	}
	return *scope.fields
}

//deprecated
func (scope *Scope) PrimaryFields() StructFields {
	return scope.PKs()
}

// was PrimaryFields() : PKs() return scope's primary fields
func (scope *Scope) PKs() StructFields {
	var fields StructFields
	for _, field := range scope.Fields() {
		if field.IsPrimaryKey() {
			fields.add(field)
		}
	}
	return fields
}

// deprecated
func (scope *Scope) PrimaryField() *StructField {
	return scope.PK()
}

// was PrimaryField() - PK() return scope's main primary field, if defined more that one primary fields, will return the one having column name `id` or the first one
func (scope *Scope) PK() *StructField {
	primaryFieldsLen := scope.GetModelStruct().noOfPKs()
	if primaryFieldsLen > 0 {
		if primaryFieldsLen > 1 {
			if field, ok := scope.FieldByName(DEFAULT_ID_NAME); ok {
				return field
			}
		}
		//and return the first one
		return scope.PKs()[0]
	}

	//TODO : @Badu - investigate where this is called and gets the nil
	return nil
}

//deprecated
func (scope *Scope) PrimaryKey() string {
	return scope.PKName()
}

// was PrimaryKey() - PKName() get main primary field's db name
func (scope *Scope) PKName() string {
	if field := scope.PK(); field != nil {
		return field.DBName
	}
	//TODO : @Badu - investigate where this is called and gets the empty string
	return ""
}

// PrimaryKeyZero check main primary field's value is blank or not
func (scope *Scope) PrimaryKeyZero() bool {
	field := scope.PK()
	return field == nil || field.IsBlank()
}

// PrimaryKeyValue get the primary key's value
func (scope *Scope) PrimaryKeyValue() interface{} {
	if field := scope.PK(); field != nil && field.Value.IsValid() {
		return field.Value.Interface()
	}
	return 0
}

// FieldByName find `gorm.StructField` with field name or db name
func (scope *Scope) FieldByName(name string) (*StructField, bool) {
	var (
		dbName           = NamesMap.ToDBName(name)
		mostMatchedField *StructField
	)

	for _, field := range scope.Fields() {
		if field.StructName == name || field.DBName == name {
			return field, true
		}
		if field.DBName == dbName {
			mostMatchedField = field
		}
	}
	return mostMatchedField, mostMatchedField != nil
}

// SetColumn to set the column's value, column could be field or field's name/dbname
func (scope *Scope) SetColumn(column interface{}, value interface{}) error {
	var (
		updateAttrs = map[string]interface{}{}
	)
	if attrs, ok := scope.InstanceGet(UPDATE_ATTRS_SETTING); ok {
		updateAttrs = attrs.(map[string]interface{})
		defer scope.InstanceSet(UPDATE_ATTRS_SETTING, updateAttrs)
	}
	switch colType := column.(type) {
	case *StructField:
		updateAttrs[colType.DBName] = value
		return colType.Set(value)
	case string:
		//looks like Scope.FieldByName
		var (
			dbName           = NamesMap.ToDBName(colType)
			mostMatchedField *StructField
		)
		for _, field := range scope.Fields() {
			if field.DBName == value {
				updateAttrs[field.DBName] = value
				return field.Set(value)
			}
			if (field.DBName == dbName) || (field.StructName == colType && mostMatchedField == nil) {
				mostMatchedField = field
			}
		}

		if mostMatchedField != nil {
			updateAttrs[mostMatchedField.DBName] = value
			return mostMatchedField.Set(value)
		}
	}
	//TODO : @Badu - make this error more explicit : what's column name
	return errors.New("SCOPE : could not convert column to field")
}

// TableName return table name
func (scope *Scope) TableName() string {
	if scope.Search != nil && scope.Search.tableName != "" {
		return scope.Search.tableName
	}
	switch tabler := scope.Value.(type) {
	case tabler:
		return tabler.TableName()
	case dbTabler:
		return tabler.TableName(scope.con)
	}
	return scope.GetModelStruct().TableName(scope.con.Model(scope.Value))
}

// Raw set raw sql
func (scope *Scope) Raw(sql string) *Scope {
	scope.Search.SQL = strings.Replace(sql, "$$", "?", -1)
	scope.Search.SetRaw()
	return scope
}

// Exec perform generated SQL
func (scope *Scope) Exec() *Scope {
	//fail fast
	if scope.HasError() {
		return scope
	}
	//avoid call if we don't need to
	if scope.con.logMode == LOG_VERBOSE {
		defer scope.trace(NowFunc())
	}
	scope.Search.Exec(scope)
	return scope
}

// GetModelStruct get value's model struct, relationships based on struct and tag definition
func (scope *Scope) GetModelStruct() *ModelStruct {
	var modelStruct ModelStruct
	// Scope value can't be nil
	//TODO : @Badu - why can't be nil and why we are not returning an warning/error?
	if scope.Value == nil {
		return &modelStruct
	}

	reflectType := GetTType(scope.Value)

	if reflectType.Kind() != reflect.Struct {
		//TODO : @Badu - why we are not returning an error?
		// Scope value need to be a struct
		return &modelStruct
	}

	// Get Cached model struct
	if value := ModelStructsMap.Get(reflectType); value != nil {
		return value
	}

	modelStruct.Create(reflectType, scope)

	//set cached ModelStruc
	ModelStructsMap.Set(reflectType, &modelStruct)
	// ATTN : first we add it to cache map, otherwise will infinite cycle
	// build relationships
	modelStruct.processRelations(scope)

	return &modelStruct
}

// CallMethod call scope value's method, if it is a slice, will call its element's method one by one
func (scope *Scope) CallMethod(methodName string) {
	if scope.Value == nil {
		return
	}

	if indirectScopeValue := IndirectValue(scope.Value); indirectScopeValue.Kind() == reflect.Slice {
		for i := 0; i < indirectScopeValue.Len(); i++ {
			scope.callMethod(methodName, indirectScopeValue.Index(i))
		}
	} else {
		scope.callMethod(methodName, indirectScopeValue)
	}
}

//calls methods after creation
func (scope *Scope) PostCreate() *Scope {
	result, txStarted := scope.Begin()
	var blankColumnsWithDefaultValue string
	result, blankColumnsWithDefaultValue = result.beforeCreateCallback().
		saveBeforeAssociationsCallback().
		updateTimeStampForCreateCallback().
		createCallback()
	result.forceReloadAfterCreateCallback(blankColumnsWithDefaultValue).
		saveAfterAssociationsCallback().
		afterCreateCallback().
		CommitOrRollback(txStarted)
	return result
}

//calls methods after deletion
func (scope *Scope) PostDelete() *Scope {
	result, txStarted := scope.Begin()
	return result.beforeDeleteCallback().
		deleteCallback().
		afterDeleteCallback().
		CommitOrRollback(txStarted)
}

//calls methods after query
func (scope *Scope) PostQuery() *Scope {
	return scope.
		queryCallback().
		preloadCallback().
		afterQueryCallback()
}

//calls methods after update
func (scope *Scope) PostUpdate() *Scope {
	if attrs, ok := scope.InstanceGet(UPDATE_INTERF_SETTING); ok {
		if updateMaps, hasUpdate := updatedAttrsWithValues(scope, attrs); hasUpdate {
			scope.InstanceSet(UPDATE_ATTRS_SETTING, updateMaps)
		} else {
			//we stop chain calls
			return scope
		}
	}
	result, txStarted := scope.Begin()
	return result.
		beforeUpdateCallback().
		saveBeforeAssociationsCallback().
		updateTimeStampForUpdateCallback().
		updateCallback().
		saveAfterAssociationsCallback().
		afterUpdateCallback().
		CommitOrRollback(txStarted)
}

////////////////////////////////////////////////////////////////////////////////
// Private Methods For *gorm.Scope
////////////////////////////////////////////////////////////////////////////////

func (scope *Scope) callMethod(methodName string, reflectValue reflect.Value) {
	// Only get address from non-pointer
	if reflectValue.CanAddr() && reflectValue.Kind() != reflect.Ptr {
		reflectValue = reflectValue.Addr()
	}

	if methodValue := reflectValue.MethodByName(methodName); methodValue.IsValid() {
		switch method := methodValue.Interface().(type) {
		case func():
			method()
		case func(*Scope):
			method(scope)
		case func(*DBCon):
			newCon := newCon(scope.con)
			method(newCon)
			scope.Err(newCon.Error)
		case func() error:
			scope.Err(method())
		case func(*Scope) error:
			scope.Err(method(scope))
		case func(*DBCon) error:
			newCon := newCon(scope.con)
			scope.Err(method(newCon))
			scope.Err(newCon.Error)
		default:
			scope.Err(fmt.Errorf("unsupported function %v", methodName))
		}
	}
}

func (scope *Scope) scan(rows *sql.Rows, columns []string, fields StructFields) {
	var (
		ignored            interface{}
		values             = make([]interface{}, len(columns))
		selectFields       StructFields
		selectedColumnsMap = map[string]int{}
		resetFields        = map[int]*StructField{}
	)

	for index, column := range columns {
		values[index] = &ignored

		selectFields = fields
		if idx, ok := selectedColumnsMap[column]; ok {
			selectFields = selectFields[idx+1:]
		}

		for fieldIndex, field := range selectFields {
			if field.DBName == column {
				switch field.Value.Kind() {
				case reflect.Ptr:
					values[index] = field.Value.Addr().Interface()
				default:
					reflectValue := field.ptrToLoad()
					reflectValue.Elem().Set(field.Value.Addr())
					values[index] = reflectValue.Interface()
					resetFields[index] = field
				}
				selectedColumnsMap[column] = fieldIndex
				//we have a normal field, we break second for
				if field.IsNormal() {
					break
				}
			}
		}
	}

	scope.Err(rows.Scan(values...))

	for index, field := range resetFields {
		if v := reflect.ValueOf(values[index]).Elem().Elem(); v.IsValid() {
			field.Value.Set(v)
		}
	}
}

func (scope *Scope) row() *sql.Row {
	//avoid call if we don't need to
	if scope.con.logMode == LOG_VERBOSE {
		defer scope.trace(NowFunc())
	}
	scope.callCallbacks(scope.con.parent.callbacks.rowQueries)
	scope.Search.prepareQuerySQL(scope)
	return scope.Search.QueryRow(scope)
}

func (scope *Scope) rows() (*sql.Rows, error) {
	//avoid call if we don't need to
	if scope.con.logMode == LOG_VERBOSE {
		defer scope.trace(NowFunc())
	}
	scope.callCallbacks(scope.con.parent.callbacks.rowQueries)
	scope.Search.prepareQuerySQL(scope)
	return scope.Search.Query(scope)
}

func (scope *Scope) pluck(column string, value interface{}) *Scope {
	dest := reflect.Indirect(reflect.ValueOf(value))
	scope.Search.Select(column)
	if dest.Kind() != reflect.Slice {
		scope.Err(fmt.Errorf("results should be a slice, not %s", dest.Kind()))
		return scope
	}

	rows, err := scope.rows()
	if scope.Err(err) == nil {
		defer rows.Close()
		for rows.Next() {
			elem := reflect.New(dest.Type().Elem()).Interface()
			scope.Err(rows.Scan(elem))
			dest.Set(reflect.Append(dest, reflect.ValueOf(elem).Elem()))
		}
	}
	return scope
}

func (scope *Scope) count(value interface{}) *Scope {
	if !scope.Search.hasSelect() {
		scope.Search.Select("count(*)")
	} else {
		sqlPair := scope.Search.getFirst(Select_query)
		if sqlPair == nil {
			scope.Warn("ERROR : search select_query should have exaclty one count")
			//error has occured in getting first item in slice
			return scope
		}
		if !regExpCounter.MatchString(fmt.Sprint(sqlPair.expression)) {
			scope.Search.Select("count(*)")
		}
	}
	scope.Search.setIsOrderIgnored()
	scope.Err(scope.row().Scan(value))
	return scope
}

// trace print sql log
func (scope *Scope) trace(t time.Time) {
	if scope.Search.SQL != "" {
		scope.con.slog(scope.Search.SQL, t, scope.Search.SQLVars...)
	}
}

func (scope *Scope) shouldSaveAssociations() bool {
	if saveAssociations, ok := scope.Get(SAVE_ASSOC_SETTING); ok {
		if v, ok := saveAssociations.(bool); ok && !v {
			return false
		}
		if v, ok := saveAssociations.(string); ok && (v == "skip" || v == "false") {
			return false
		}
	}
	return !scope.HasError()
}

func (scope *Scope) related(value interface{}, foreignKeys ...string) *Scope {
	toScope := scope.con.NewScope(value)
	tx := scope.con.set(ASSOCIATION_SOURCE_SETTING, scope.Value)
	//TODO : @Badu - boilerplate string
	allKeys := append(foreignKeys, GetType(toScope.Value).Name()+"Id", GetType(scope.Value).Name()+"Id")

	//because we're using it in a for, we're getting it once
	dialect := scope.con.parent.dialect

	for _, foreignKey := range allKeys {
		fromField, _ := scope.FieldByName(foreignKey)
		//fail fast - from field is nil
		if fromField == nil {
			toField, _ := toScope.FieldByName(foreignKey)
			if toField == nil {
				//fail fast - continue : both fields are nil
				continue
			}
			aStr := fmt.Sprintf("%v = ?", Quote(toField.DBName, dialect))
			scope.Err(tx.Where(aStr, scope.PrimaryKeyValue()).Find(value).Error)
			return scope
		}

		//fail fast - relationship is nil
		if !fromField.HasRelations() {
			aStr := fmt.Sprintf("%v = ?", Quote(toScope.PKName(), dialect))
			scope.Err(tx.Where(aStr, fromField.Value.Interface()).Find(value).Error)
			return scope
		}
		var (
			ForeignDBNames            = fromField.GetSliceSetting(FOREIGN_DB_NAMES)
			AssociationForeignDBNames = fromField.GetSliceSetting(ASSOCIATION_FOREIGN_DB_NAMES)
		)
		//now, fail fast is over
		switch fromField.RelKind() {
		case MANY_TO_MANY:
			joinTableHandler := fromField.JoinHandler()
			scope.Err(
				joinTableHandler.JoinWith(
					joinTableHandler,
					tx,
					scope.Value,
				).
					Find(value).
					Error)

		case BELONGS_TO:
			for idx, foreignKey := range ForeignDBNames {
				if field, ok := scope.FieldByName(foreignKey); ok {
					tx = tx.Where(
						fmt.Sprintf(
							"%v = ?",
							Quote(AssociationForeignDBNames[idx], dialect),
						),
						field.Value.Interface(),
					)
				}
			}
			scope.Err(tx.Find(value).Error)

		case HAS_MANY, HAS_ONE:
			for idx, foreignKey := range ForeignDBNames {
				if field, ok := scope.FieldByName(AssociationForeignDBNames[idx]); ok {
					tx = tx.Where(
						fmt.Sprintf(
							"%v = ?",
							Quote(foreignKey, dialect),
						),
						field.Value.Interface(),
					)
				}
			}

			if fromField.HasSetting(POLYMORPHIC_TYPE) {
				tx = tx.Where(
					fmt.Sprintf(
						"%v = ?",
						Quote(fromField.GetStrSetting(POLYMORPHIC_DBNAME), dialect),
					),
					fromField.GetStrSetting(POLYMORPHIC_VALUE),
				)
			}

			scope.Err(tx.Find(value).Error)
		}
		return scope

	}

	scope.Err(fmt.Errorf("invalid association %v", foreignKeys))
	return scope
}

func (scope *Scope) saveFieldAsAssociation(field *StructField) bool {
	if field.IsBlank() || field.IsIgnored() || !scope.Search.changeableField(field) {
		return false
	}

	//TODO : @Badu - make field WillSaveAssociations FLAG
	if field.HasSetting(SAVE_ASSOCIATIONS) {
		set := field.GetStrSetting(SAVE_ASSOCIATIONS)
		if set == "false" || set == "skip" {
			return false
		}
	}
	if field.HasRelations() {
		return true
	}

	return false
}

func (scope *Scope) callCallbacks(funcs ScopedFuncs) *Scope {
	for _, f := range funcs {
		//was (*f)(scope) - but IDE went balistic
		rf := *f
		rf(scope)
	}
	return scope
}

////////////////////////////////////////////////////////////////////////////////
// after create functions
////////////////////////////////////////////////////////////////////////////////
//[create step 1] [delete step 1] [update step 1]
// Begin start a transaction
func (scope *Scope) Begin() (*Scope, bool) {
	if db, ok := scope.con.sqli.(sqlDb); ok {
		//parent db implements Begin() -> call Begin()
		if tx, err := db.Begin(); err == nil {
			//TODO : @Badu - maybe the parent should do so, since it's owner of db.db
			//parent db.db implements Exec(), Prepare(), Query() and QueryRow()
			//TODO : @Badu - it's paired with commit or rollback - see below
			scope.con.sqli = interface{}(tx).(sqlInterf)
			return scope, true
		}
	}
	return scope, false
}

//[create step 2]
// beforeCreateCallback will invoke `BeforeSave`, `BeforeCreate` method before creating
func (scope *Scope) beforeCreateCallback() *Scope {
	if !scope.HasError() {
		scope.CallMethod("BeforeSave")
	}
	if !scope.HasError() {
		scope.CallMethod("BeforeCreate")
	}
	return scope
}

//[create step 3] [update step 3]
func (scope *Scope) saveBeforeAssociationsCallback() *Scope {
	if !scope.shouldSaveAssociations() {
		return scope
	}
	for _, field := range scope.Fields() {

		if field.IsBlank() || field.IsIgnored() || !scope.Search.changeableField(field) {
			continue
		}
		if scope.saveFieldAsAssociation(field) && field.RelKind() == BELONGS_TO {
			fieldValue := field.Value.Addr().Interface()
			scope.Err(newCon(scope.con).Save(fieldValue).Error)
			var (
				ForeignFieldNames         = field.GetSliceSetting(FOREIGN_FIELD_NAMES)
				AssociationForeignDBNames = field.GetSliceSetting(ASSOCIATION_FOREIGN_DB_NAMES)
			)
			if ForeignFieldNames.len() != 0 {
				// set value's foreign key
				for idx, fieldName := range ForeignFieldNames {
					associationForeignName := AssociationForeignDBNames[idx]
					if foreignField, ok := scope.NewScope(fieldValue).FieldByName(associationForeignName); ok {
						scope.Err(scope.SetColumn(fieldName, foreignField.Value.Interface()))
					}
				}
			}
		}
	}
	return scope
}

//[create step 4]
// updateTimeStampForCreateCallback will set `CreatedAt`, `UpdatedAt` when creating
func (scope *Scope) updateTimeStampForCreateCallback() *Scope {
	if !scope.HasError() {
		now := NowFunc()
		scope.SetColumn("CreatedAt", now)
		scope.SetColumn("UpdatedAt", now)
	}
	return scope
}

//[create step 5]
// createCallback the callback used to insert data into database
func (scope *Scope) createCallback() (*Scope, string) {
	var (
		columns, placeholders        StrSlice
		blankColumnsWithDefaultValue StrSlice
		//because we're using it in a for, we're getting it once
		dialect         = scope.con.parent.dialect
		returningColumn = "*"
		quotedTableName = QuotedTableName(scope)
		primaryField    = scope.PK()
		extraOption     string
	)

	if !scope.HasError() {
		//avoid call if we don't need to
		if scope.con.logMode == LOG_VERBOSE {
			defer scope.trace(NowFunc())
		}

		for _, field := range scope.Fields() {
			if !scope.Search.changeableField(field) {
				continue
			}

			if field.IsNormal() {
				isBlankWithDefaultValue := field.IsBlank() && field.HasDefaultValue()
				isNotPKOrBlank := !field.IsPrimaryKey() || !field.IsBlank()
				if isBlankWithDefaultValue {
					blankColumnsWithDefaultValue.add(Quote(field.DBName, dialect))
				} else if isNotPKOrBlank {
					columns.add(Quote(field.DBName, dialect))
					placeholders.add(scope.Search.addToVars(field.Value.Interface(), dialect))
				}
			} else {
				if field.HasRelations() && field.RelKind() == BELONGS_TO {
					ForeignDBNames := field.GetSliceSetting(FOREIGN_DB_NAMES)
					for _, foreignKey := range ForeignDBNames {
						foreignField, ok := scope.FieldByName(foreignKey)
						if ok && !scope.Search.changeableField(foreignField) {
							columns.add(Quote(foreignField.DBName, dialect))
							placeholders.add(scope.Search.addToVars(foreignField.Value.Interface(), dialect))
						}
					}
				}
			}
		}

		if str, ok := scope.Get(INSERT_OPT_SETTING); ok {
			extraOption = fmt.Sprint(str)
		}

		if primaryField != nil {
			returningColumn = Quote(primaryField.DBName, dialect)
		}

		lastInsertIDReturningSuffix := dialect.LastInsertIDReturningSuffix(quotedTableName, returningColumn)

		if len(columns) == 0 {
			scope.Raw(fmt.Sprintf(
				"INSERT INTO %v DEFAULT VALUES%v%v",
				quotedTableName,
				addExtraSpaceIfExist(extraOption),
				addExtraSpaceIfExist(lastInsertIDReturningSuffix),
			))
		} else {
			scope.Raw(fmt.Sprintf(
				"INSERT INTO %v (%v) VALUES (%v)%v%v",
				QuotedTableName(scope),
				strings.Join(columns, ","),
				strings.Join(placeholders, ","),
				addExtraSpaceIfExist(extraOption),
				addExtraSpaceIfExist(lastInsertIDReturningSuffix),
			))
		}

		// execute create sql
		if lastInsertIDReturningSuffix == "" || primaryField == nil {
			if result, err := scope.Search.Exec(scope); scope.Err(err) == nil {
				// set rows affected count
				//scope.con.RowsAffected, _ = result.RowsAffected()

				// set primary value to primary field
				if primaryField != nil && primaryField.IsBlank() {
					if primaryValue, err := result.LastInsertId(); scope.Err(err) == nil {
						scope.Err(primaryField.Set(primaryValue))
					}
				}
			}
		} else {
			if err := scope.Search.QueryRow(scope).
				Scan(primaryField.Value.Addr().Interface()); scope.Err(err) == nil {
				primaryField.UnsetIsBlank()
				scope.con.RowsAffected = 1
			}
		}
	}
	return scope, blankColumnsWithDefaultValue.asString()
}

//[create step 6]
// forceReloadAfterCreateCallback will reload columns that having default value, and set it back to current object
func (scope *Scope) forceReloadAfterCreateCallback(blankColumnsWithDefaultValue string) *Scope {
	if blankColumnsWithDefaultValue != "" {
		db := newCon(scope.con).Table(scope.TableName()).Select(blankColumnsWithDefaultValue)
		for _, field := range scope.Fields() {
			if field.IsPrimaryKey() && !field.IsBlank() {
				db = db.Where(fmt.Sprintf("%v = ?", field.DBName), field.Value.Interface())
			}
		}

		db.Scan(scope.Value)
	}
	return scope
}

//[create step 7] [update step 6]
func (scope *Scope) saveAfterAssociationsCallback() *Scope {
	if !scope.shouldSaveAssociations() {
		return scope
	}
	for _, field := range scope.Fields() {
		if field.IsBlank() || field.IsIgnored() || !scope.Search.changeableField(field) {
			continue
		}

		//Attention : relationship.Kind <= HAS_ONE means except BELONGS_TO
		if scope.saveFieldAsAssociation(field) && field.RelKind() <= HAS_ONE {
			value := field.Value
			ForeignFieldNames := field.GetSliceSetting(FOREIGN_FIELD_NAMES)
			AssociationForeignDBNames := field.GetSliceSetting(ASSOCIATION_FOREIGN_DB_NAMES)
			switch value.Kind() {
			case reflect.Slice:
				for i := 0; i < value.Len(); i++ {
					//TODO : @Badu - cloneCon without copy, then NewScope which clone's con - but with copy
					newCon := newCon(scope.con)
					elem := value.Index(i).Addr().Interface()
					newScope := newCon.NewScope(elem)

					if !field.HasSetting(JOIN_TABLE_HANDLER) && ForeignFieldNames.len() != 0 {
						for idx, fieldName := range ForeignFieldNames {
							associationForeignName := AssociationForeignDBNames[idx]
							if f, ok := scope.FieldByName(associationForeignName); ok {
								scope.Err(newScope.SetColumn(fieldName, f.Value.Interface()))
							}
						}
					}

					if field.HasSetting(POLYMORPHIC_TYPE) {
						scope.Err(
							newScope.SetColumn(
								field.GetStrSetting(POLYMORPHIC_TYPE),
								field.GetStrSetting(POLYMORPHIC_VALUE)))
					}
					scope.Err(newCon.Save(elem).Error)

					if field.HasSetting(JOIN_TABLE_HANDLER) {
						joinTableHandler := field.JoinHandler()
						scope.Err(joinTableHandler.Add(joinTableHandler, newCon, scope.Value, newScope.Value))
					}
				}
			default:
				elem := value.Addr().Interface()
				newScope := scope.NewScope(elem)

				if ForeignFieldNames.len() != 0 {
					for idx, fieldName := range ForeignFieldNames {
						associationForeignName := AssociationForeignDBNames[idx]
						if f, ok := scope.FieldByName(associationForeignName); ok {
							scope.Err(newScope.SetColumn(fieldName, f.Value.Interface()))
						}
					}
				}

				if field.HasSetting(POLYMORPHIC_TYPE) {
					scope.Err(
						newScope.SetColumn(
							field.GetStrSetting(POLYMORPHIC_TYPE),
							field.GetStrSetting(POLYMORPHIC_VALUE)))
				}
				scope.Err(newCon(scope.con).Save(elem).Error)
			}
		}

	}
	return scope
}

//[create step 8]
// afterCreateCallback will invoke `AfterCreate`, `AfterSave` method after creating
func (scope *Scope) afterCreateCallback() *Scope {
	if !scope.HasError() {
		scope.CallMethod("AfterCreate")
	}
	if !scope.HasError() {
		scope.CallMethod("AfterSave")
	}
	return scope
}

//[create step 9] [delete step 5] [update step 8]
// CommitOrRollback commit current transaction if no error happened, otherwise will rollback it
func (scope *Scope) CommitOrRollback(txStarted bool) *Scope {
	if txStarted {
		if db, ok := scope.con.sqli.(sqlTx); ok {
			if scope.HasError() {
				//orm.db implements Commit() and Rollback() -> call Rollback()
				db.Rollback()
			} else {
				//orm.db implements Commit() and Rollback() -> call Commit()
				scope.Err(db.Commit())
			}
			//TODO : @Badu - it's paired with begin - see above
			scope.con.sqli = scope.con.parent.sqli
		}
	}
	return scope
}

//[delete step 2]
// beforeDeleteCallback will invoke `BeforeDelete` method before deleting
func (scope *Scope) beforeDeleteCallback() *Scope {
	if !scope.HasError() {
		scope.CallMethod("BeforeDelete")
	}
	return scope
}

//[delete step 3]
// deleteCallback used to delete data from database or set deleted_at to current time (when using with soft delete)
func (scope *Scope) deleteCallback() *Scope {
	if !scope.HasError() {
		var extraOption string
		if str, ok := scope.Get(DELETE_OPT_SETTING); ok {
			extraOption = fmt.Sprint(str)
		}

		if !scope.Search.isUnscoped() && scope.GetModelStruct().HasColumn("DeletedAt") {
			scope.Raw(fmt.Sprintf(
				"UPDATE %v SET deleted_at=%v%v%v",
				QuotedTableName(scope),
				scope.Search.addToVars(NowFunc(), scope.con.parent.dialect),
				addExtraSpaceIfExist(scope.Search.combinedConditionSql(scope)),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		} else {
			scope.Raw(fmt.Sprintf(
				"DELETE FROM %v%v%v",
				QuotedTableName(scope),
				addExtraSpaceIfExist(scope.Search.combinedConditionSql(scope)),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		}
	}
	return scope
}

//[delete step 4]
// afterDeleteCallback will invoke `AfterDelete` method after deleting
func (scope *Scope) afterDeleteCallback() *Scope {
	if !scope.HasError() {
		scope.CallMethod("AfterDelete")
	}
	return scope
}

// [query step 1]
// queryCallback used to query data from database
func (scope *Scope) queryCallback() *Scope {
	//avoid call if we don't need to
	if scope.con.logMode == LOG_VERBOSE {
		defer scope.trace(NowFunc())
	}
	var (
		isSlice, isPtr bool
		resultType     reflect.Type
		results        = IndirectValue(scope.Value)
		dialect        = scope.con.parent.dialect
	)

	if orderBy, ok := scope.Get(ORDER_BY_PK_SETTING); ok {
		if primaryField := scope.PK(); primaryField != nil {
			scope.Search.Order(
				fmt.Sprintf(
					"%v.%v %v",
					QuotedTableName(scope),
					Quote(primaryField.DBName, dialect), orderBy),
			)
		}
	}

	if value, ok := scope.Get(QUERY_DEST_SETTING); ok {
		results = reflect.Indirect(reflect.ValueOf(value))
	}
	switch results.Kind() {
	case reflect.Slice:
		isSlice = true
		resultType = results.Type().Elem()
		results.Set(reflect.MakeSlice(results.Type(), 0, 0))

		if resultType.Kind() == reflect.Ptr {
			isPtr = true
			resultType = resultType.Elem()
		}
	case reflect.Struct:
	default:
		scope.Err(errors.New("SCOPE : unsupported destination, should be slice or struct"))
		return scope
	}

	scope.Search.prepareQuerySQL(scope)

	if !scope.HasError() {
		scope.con.RowsAffected = 0
		if str, ok := scope.Get(QUERY_OPT_SETTING); ok {
			scope.Search.SQL += addExtraSpaceIfExist(fmt.Sprint(str))
		}

		if rows, err := scope.Search.Query(scope); scope.Err(err) == nil {
			defer rows.Close()

			columns, _ := rows.Columns()
			for rows.Next() {
				scope.con.RowsAffected++

				elem := results
				if isSlice {
					elem = reflect.New(resultType).Elem()
				}

				scope.scan(rows, columns, scope.NewScope(elem.Addr().Interface()).Fields())

				if isSlice {
					if isPtr {
						results.Set(reflect.Append(results, elem.Addr()))
					} else {
						results.Set(reflect.Append(results, elem))
					}
				}
			}

			if scope.con.RowsAffected == 0 && !isSlice {
				scope.Err(ErrRecordNotFound)
			}
		}
	}
	return scope
}

// [query step 2]
// preloadCallback used to preload associations
func (scope *Scope) preloadCallback() *Scope {
	if !scope.Search.hasPreload() || scope.HasError() {
		return scope
	}
	scope.Search.doPreload(scope)
	return scope
}

// [query step 3]
// afterQueryCallback will invoke `AfterFind` method after querying
func (scope *Scope) afterQueryCallback() *Scope {
	if !scope.HasError() {
		scope.CallMethod("AfterFind")
	}
	return scope
}

// [update step 2]
// beforeUpdateCallback will invoke `BeforeSave`, `BeforeUpdate` method before updating
func (scope *Scope) beforeUpdateCallback() *Scope {
	if _, ok := scope.Get(UPDATE_COLUMN_SETTING); !ok {
		if !scope.HasError() {
			scope.CallMethod("BeforeSave")
		}
		if !scope.HasError() {
			scope.CallMethod("BeforeUpdate")
		}
	}
	return scope
}

// [update step 4]
// updateTimeStampForUpdateCallback will set `UpdatedAt` when updating
func (scope *Scope) updateTimeStampForUpdateCallback() *Scope {
	if _, ok := scope.Get(UPDATE_COLUMN_SETTING); !ok {
		scope.SetColumn("UpdatedAt", NowFunc())
	}
	return scope
}

// [update step 5]
// updateCallback the callback used to update data to database
func (scope *Scope) updateCallback() *Scope {
	var (
		sqls []string
		//because we're using it in a for, we're getting it once
		scopeDialect = scope.con.parent.dialect
		extraOption  string
	)
	if !scope.HasError() {

		if updateAttrs, ok := scope.InstanceGet(UPDATE_ATTRS_SETTING); ok {
			for column, value := range updateAttrs.(map[string]interface{}) {
				sqls = append(sqls,
					fmt.Sprintf(
						"%v = %v",
						Quote(column, scopeDialect),
						scope.Search.addToVars(value, scopeDialect),
					),
				)
			}
		} else {
			for _, field := range scope.Fields() {
				if !scope.Search.changeableField(field) {
					continue
				}
				if !field.IsPrimaryKey() && field.IsNormal() {
					sqls = append(sqls,
						fmt.Sprintf(
							"%v = %v",
							Quote(field.DBName, scopeDialect),
							scope.Search.addToVars(field.Value.Interface(), scopeDialect),
						),
					)
				} else {
					if field.HasRelations() && field.RelKind() == BELONGS_TO {
						var (
							ForeignDBNames = field.GetSliceSetting(FOREIGN_DB_NAMES)
						)
						for _, foreignKey := range ForeignDBNames {
							foreignField, ok := scope.FieldByName(foreignKey)
							if ok && !scope.Search.changeableField(foreignField) {
								sqls = append(sqls,
									fmt.Sprintf(
										"%v = %v",
										Quote(foreignField.DBName, scopeDialect),
										scope.Search.addToVars(
											foreignField.Value.Interface(),
											scopeDialect,
										),
									),
								)
							}
						}
					}
				}

			}
		}

		if str, ok := scope.Get(UPDATE_OPT_SETTING); ok {
			extraOption = fmt.Sprint(str)
		}

		if len(sqls) > 0 {
			scope.Raw(fmt.Sprintf(
				"UPDATE %v SET %v%v%v",
				QuotedTableName(scope),
				strings.Join(sqls, ", "),
				addExtraSpaceIfExist(scope.Search.combinedConditionSql(scope)),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		}
	}
	return scope
}

// [update step 7]
// afterUpdateCallback will invoke `AfterUpdate`, `AfterSave` method after updating
func (scope *Scope) afterUpdateCallback() *Scope {
	if _, ok := scope.Get(UPDATE_COLUMN_SETTING); !ok {
		if !scope.HasError() {
			scope.CallMethod("AfterUpdate")
		}
		if !scope.HasError() {
			scope.CallMethod("AfterSave")
		}
	}
	return scope
}
