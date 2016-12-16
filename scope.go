package gorm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// New create a new Scope without search information
func (scope *Scope) NewScope(value interface{}) *Scope {
	return &Scope{
		con:    newCon(scope.con),
		Search: &Search{Conditions: make(SqlConditions)},
		Value:  value}
}

// Set set value by name
func (scope *Scope) Set(settingType uint64, value interface{}) *Scope {
	scope.con.localSet(settingType, value)
	return scope
}

// Get get setting by name
func (scope *Scope) Get(settingType uint64) (interface{}, bool) {
	return scope.con.get(settingType)
}

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
			if field, ok := scope.FieldByName(field_default_id_name); ok {
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
		dbName           = NamesMap.toDBName(name)
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
	switch colType := column.(type) {
	case *StructField:
		if scope.updateMaps != nil {
			scope.updateMaps[colType.DBName] = value
		}
		return colType.Set(value)
	case string:
		//looks like Scope.FieldByName
		var (
			dbName           = NamesMap.toDBName(colType)
			mostMatchedField *StructField
		)
		for _, field := range scope.Fields() {
			if field.DBName == value {
				if scope.updateMaps != nil {
					scope.updateMaps[field.DBName] = value
				}
				return field.Set(value)
			}
			if (field.DBName == dbName) || (field.StructName == colType && mostMatchedField == nil) {
				mostMatchedField = field
			}
		}

		if mostMatchedField != nil {
			if scope.updateMaps != nil {
				scope.updateMaps[mostMatchedField.DBName] = value
			}
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
	if scope.con.logMode == LOG_VERBOSE || scope.con.logMode == LOG_DEBUG {
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
	reflectValue := IndirectValue(scope.Value)
	if reflectValue.Kind() == reflect.Slice {
		for i := 0; i < reflectValue.Len(); i++ {
			scope.callMethod(methodName, reflectValue.Index(i))
		}
	} else {
		scope.callMethod(methodName, reflectValue)
	}
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
	if scope.con.logMode == LOG_VERBOSE || scope.con.logMode == LOG_DEBUG {
		defer scope.trace(NowFunc())
	}
	if scope.con.parent.callbacks.rowQueries.len() > 0 {
		scope.callCallbacks(scope.con.parent.callbacks.rowQueries)
	}
	scope.Search.prepareQuerySQL(scope)
	return scope.Search.QueryRow(scope)
}

func (scope *Scope) rows() (*sql.Rows, error) {
	//avoid call if we don't need to
	if scope.con.logMode == LOG_VERBOSE || scope.con.logMode == LOG_DEBUG {
		defer scope.trace(NowFunc())
	}
	if scope.con.parent.callbacks.rowQueries.len() > 0 {
		scope.callCallbacks(scope.con.parent.callbacks.rowQueries)
	}
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
		sqlPair := scope.Search.getFirst(cond_select_query)
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
	if saveAssociations, ok := scope.Get(gorm_setting_save_assoc); ok {
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
	tx := scope.con.set(gorm_setting_association_source, scope.Value)
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
			ForeignDBNames            = fromField.GetSliceSetting(set_foreign_db_names)
			AssociationForeignDBNames = fromField.GetSliceSetting(set_association_foreign_db_names)
		)
		//now, fail fast is over
		switch fromField.RelKind() {
		case rel_many2many:
			joinTableHandler := fromField.JoinHandler()
			scope.Err(
				joinTableHandler.JoinWith(
					joinTableHandler,
					tx,
					scope.Value,
				).
					Find(value).
					Error)

		case rel_belongs_to:
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

		case rel_has_many, rel_has_one:
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

			if fromField.HasSetting(set_polymorphic_type) {
				tx = tx.Where(
					fmt.Sprintf(
						"%v = ?",
						Quote(fromField.GetStrSetting(set_polymorphic_dbname), dialect),
					),
					fromField.GetStrSetting(set_polymorphic_value),
				)
			}

			scope.Err(tx.Find(value).Error)
		}
		return scope

	}

	scope.Err(fmt.Errorf("invalid association %v", foreignKeys))
	return scope
}

func (scope *Scope) willSaveFieldAssociations(field *StructField) bool {
	if field.IsBlank() || field.IsIgnored() || !scope.Search.changeableField(field) {
		return false
	}

	//TODO : @Badu - make field WillSaveAssociations FLAG
	if field.HasSetting(set_save_associations) {
		set := field.GetStrSetting(set_save_associations)
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

//calls methods after query
func (scope *Scope) postQuery(dest interface{}) *Scope {
	//Was "queryCallback"
	//avoid call if we don't need to
	if scope.con.logMode == LOG_VERBOSE || scope.con.logMode == LOG_DEBUG {
		defer scope.trace(NowFunc())
	}
	var (
		isSlice, isPtr  bool
		queryResultType reflect.Type
		queryResults    = IndirectValue(scope.Value)
	)

	if dest != nil {
		queryResults = reflect.Indirect(reflect.ValueOf(dest))
	}
	switch queryResults.Kind() {
	case reflect.Slice:
		isSlice = true
		queryResultType = queryResults.Type().Elem()
		queryResults.Set(reflect.MakeSlice(queryResults.Type(), 0, 0))

		if queryResultType.Kind() == reflect.Ptr {
			isPtr = true
			queryResultType = queryResultType.Elem()
		}
	case reflect.Struct:
	default:
		scope.Err(errors.New("SCOPE : unsupported destination, should be slice or struct"))
		return scope
	}

	scope.Search.prepareQuerySQL(scope)

	if !scope.HasError() {
		scope.con.RowsAffected = 0
		if str, ok := scope.Get(gorm_setting_query_opt); ok {
			scope.Search.SQL += addExtraSpaceIfExist(fmt.Sprint(str))
		}

		if rows, err := scope.Search.Query(scope); scope.Err(err) == nil {
			defer rows.Close()

			columns, _ := rows.Columns()
			for rows.Next() {
				scope.con.RowsAffected++

				elem := queryResults
				if isSlice {
					elem = reflect.New(queryResultType).Elem()
				}

				scope.scan(rows, columns, scope.NewScope(elem.Addr().Interface()).Fields())

				if isSlice {
					if isPtr {
						queryResults.Set(reflect.Append(queryResults, elem.Addr()))
					} else {
						queryResults.Set(reflect.Append(queryResults, elem))
					}
				}
			}

			if scope.con.RowsAffected == 0 && !isSlice {
				scope.Err(ErrRecordNotFound)
			}
		}
	}
	//END Was "queryCallback"

	if scope.Search.hasPreload() && !scope.HasError() {
		scope.Search.doPreload(scope)
	}

	if !scope.HasError() {
		scope.CallMethod(meth_after_find)
	}

	return scope
}

//calls methods after creation
func (scope *Scope) postCreate() *Scope {
	//begin transaction
	result, txStarted := scope.Begin()

	//call callbacks
	if !result.HasError() {
		result.CallMethod(meth_before_save)
	}
	if !result.HasError() {
		result.CallMethod(meth_before_create)
	}

	//save associations
	if result.shouldSaveAssociations() {
		result = result.saveBeforeAssociationsCallback()
	}

	//set time fields accordingly
	if !result.HasError() {
		now := NowFunc()
		result.SetColumn(Field_created_at, now)
		result.SetColumn(Field_updated_at, now)
	}

	var blankColumnsWithDefaultValue string

	//Was "createCallback" method
	if !result.HasError() {
		var (
			//columns, placeholders        StrSlice
			//because we're using it in a for, we're getting it once
			dialect                            = result.con.parent.dialect
			returningColumn                    = str_everything
			quotedTableName                    = QuotedTableName(result)
			primaryField                       = result.PK()
			extraOption, columns, placeholders string
		)

		//avoid call if we don't need to
		if result.con.logMode == LOG_VERBOSE || scope.con.logMode == LOG_DEBUG {
			defer result.trace(NowFunc())
		}

		for _, field := range result.Fields() {
			if !result.Search.changeableField(field) {
				continue
			}

			if field.IsNormal() {
				isBlankWithDefaultValue := field.IsBlank() && field.HasDefaultValue()
				isNotPKOrBlank := !field.IsPrimaryKey() || !field.IsBlank()
				if isBlankWithDefaultValue {
					if blankColumnsWithDefaultValue != "" {
						blankColumnsWithDefaultValue += ","
					}
					blankColumnsWithDefaultValue += Quote(field.DBName, dialect)
				} else if isNotPKOrBlank {
					if columns != "" {
						columns += ","
					}
					columns += Quote(field.DBName, dialect)
					if placeholders != "" {
						placeholders += ","
					}
					placeholders += result.Search.addToVars(field.Value.Interface(), dialect)
				}
			} else {
				if field.HasRelations() && field.RelationIsBelongsTo() {
					ForeignDBNames := field.GetSliceSetting(set_foreign_db_names)
					for _, foreignKey := range ForeignDBNames {
						foreignField, ok := result.FieldByName(foreignKey)
						if ok && !result.Search.changeableField(foreignField) {
							if columns != "" {
								columns += ","
							}
							columns += Quote(foreignField.DBName, dialect)
							if placeholders != "" {
								placeholders += ","
							}
							placeholders += result.Search.addToVars(foreignField.Value.Interface(), dialect)
						}
					}
				}
			}
		}

		if str, ok := result.Get(gorm_setting_insert_opt); ok {
			extraOption = fmt.Sprint(str)
		}

		if primaryField != nil {
			returningColumn = Quote(primaryField.DBName, dialect)
		}

		lastInsertIDReturningSuffix := dialect.LastInsertIDReturningSuffix(quotedTableName, returningColumn)

		if columns == "" {
			result.Raw(fmt.Sprintf(
				"INSERT INTO %v DEFAULT VALUES%v%v",
				quotedTableName,
				addExtraSpaceIfExist(extraOption),
				addExtraSpaceIfExist(lastInsertIDReturningSuffix),
			))
		} else {
			result.Raw(fmt.Sprintf(
				"INSERT INTO %v (%v) VALUES (%v)%v%v",
				QuotedTableName(result),
				columns,
				placeholders,
				addExtraSpaceIfExist(extraOption),
				addExtraSpaceIfExist(lastInsertIDReturningSuffix),
			))
		}

		// execute create sql
		if lastInsertIDReturningSuffix == "" || primaryField == nil {
			if execResult, err := result.Search.Exec(result); result.Err(err) == nil {
				// set rows affected count
				//result.con.RowsAffected, _ = execResult.RowsAffected()

				// set primary value to primary field
				if primaryField != nil && primaryField.IsBlank() {
					if primaryValue, err := execResult.LastInsertId(); result.Err(err) == nil {
						result.Err(primaryField.Set(primaryValue))
					}
				}
			}
		} else {
			if err := result.Search.QueryRow(result).
				Scan(primaryField.Value.Addr().Interface()); result.Err(err) == nil {
				primaryField.UnsetIsBlank()
				result.con.RowsAffected = 1
			}
		}
	}
	//END - Was "createCallback" method

	//Was "forceReloadAfterCreateCallback" method
	if blankColumnsWithDefaultValue != "" {
		db := newCon(result.con).Table(result.TableName()).Select(blankColumnsWithDefaultValue)
		for _, field := range result.Fields() {
			if field.IsPrimaryKey() && !field.IsBlank() {
				db = db.Where(fmt.Sprintf("%v = ?", field.DBName), field.Value.Interface())
			}
		}

		db.Scan(result.Value)
	}
	//END - Was "forceReloadAfterCreateCallback" method

	//save associations
	if result.shouldSaveAssociations() {
		result = result.saveAfterAssociationsCallback()
	}

	//call callbacks again
	if !result.HasError() {
		result.CallMethod(meth_after_create)
	}
	if !result.HasError() {
		result.CallMethod(meth_after_save)
	}

	//attempt to commit in the end
	return result.CommitOrRollback(txStarted)
}

//calls methods after update
func (scope *Scope) postUpdate() *Scope {
	if scope.attrs != nil {
		updateMaps, hasUpdate := updatedAttrsWithValues(scope, scope.attrs)
		if hasUpdate {
			scope.updateMaps = updateMaps
		} else {
			//we stop chain calls
			return scope
		}
	}

	//begin transaction
	result, txStarted := scope.Begin()

	if _, ok := result.Get(gorm_setting_update_column); !ok {
		if !result.HasError() {
			result.CallMethod(meth_before_save)
		}
		if !result.HasError() {
			result.CallMethod(meth_before_update)
		}
	}

	//save associations
	if result.shouldSaveAssociations() {
		result = result.saveBeforeAssociationsCallback()
	}

	//update the updated at column
	if _, ok := result.Get(gorm_setting_update_column); !ok {
		result.SetColumn(Field_updated_at, NowFunc())
	}

	//Was "updateCallback"
	if !result.HasError() {
		var (
			//because we're using it in a for, we're getting it once
			scopeDialect     = result.con.parent.dialect
			extraOption, sql string
		)

		if result.updateMaps != nil {
			for column, value := range result.updateMaps {
				if sql != "" {
					sql += ", "
				}
				sql += fmt.Sprintf(
					"%v = %v",
					Quote(column, scopeDialect),
					result.Search.addToVars(value, scopeDialect),
				)

			}
		} else {
			for _, field := range result.Fields() {
				if !result.Search.changeableField(field) {
					continue
				}
				if !field.IsPrimaryKey() && field.IsNormal() {
					if sql != "" {
						sql += ", "
					}
					sql += fmt.Sprintf(
						"%v = %v",
						Quote(field.DBName, scopeDialect),
						result.Search.addToVars(field.Value.Interface(), scopeDialect),
					)
				} else {
					if field.HasRelations() && field.RelationIsBelongsTo() {
						ForeignDBNames := field.GetSliceSetting(set_foreign_db_names)
						for _, foreignKey := range ForeignDBNames {
							foreignField, ok := result.FieldByName(foreignKey)
							if ok && !result.Search.changeableField(foreignField) {
								if sql != "" {
									sql += ", "
								}
								sql += fmt.Sprintf(
									"%v = %v",
									Quote(foreignField.DBName, scopeDialect),
									result.Search.addToVars(
										foreignField.Value.Interface(),
										scopeDialect,
									),
								)
							}
						}
					}
				}

			}
		}

		if str, ok := result.Get(gorm_setting_update_opt); ok {
			extraOption = fmt.Sprint(str)
		}

		if sql != "" {
			result.Raw(fmt.Sprintf(
				"UPDATE %v SET %v%v%v",
				QuotedTableName(result),
				sql,
				addExtraSpaceIfExist(result.Search.combinedConditionSql(result)),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		}
	}
	//END Was "updateCallback"

	//save associations
	if result.shouldSaveAssociations() {
		result = result.saveAfterAssociationsCallback()
	}

	if _, ok := result.Get(gorm_setting_update_column); !ok {
		if !result.HasError() {
			result.CallMethod(meth_after_update)
		}
		if !result.HasError() {
			result.CallMethod(meth_after_save)
		}
	}

	return result.CommitOrRollback(txStarted)
}

//calls methods after deletion
func (scope *Scope) postDelete() *Scope {
	//begin transaction
	result, txStarted := scope.Begin()

	//call callbacks
	if !result.HasError() {
		result.CallMethod(meth_before_delete)
	}

	//Was "deleteCallback"
	if !result.HasError() {
		var extraOption string
		if str, ok := result.Get(gorm_setting_delete_opt); ok {
			extraOption = fmt.Sprint(str)
		}

		if !result.Search.isUnscoped() && result.GetModelStruct().HasColumn(Field_deleted_at) {
			result.Raw(fmt.Sprintf(
				"UPDATE %v SET deleted_at=%v%v%v",
				QuotedTableName(result),
				result.Search.addToVars(NowFunc(), result.con.parent.dialect),
				addExtraSpaceIfExist(result.Search.combinedConditionSql(result)),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		} else {
			result.Raw(fmt.Sprintf(
				"DELETE FROM %v%v%v",
				QuotedTableName(result),
				addExtraSpaceIfExist(result.Search.combinedConditionSql(result)),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		}
	}
	//END - Was "deleteCallback"

	//call callbacks
	if !result.HasError() {
		result.CallMethod(meth_after_delete)
	}

	//attempt to commit
	return result.CommitOrRollback(txStarted)
}

////////////////////////////////////////////////////////////////////////////////
// internal callbacks functions
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

//[create step 3] [update step 3]
func (scope *Scope) saveBeforeAssociationsCallback() *Scope {
	for _, field := range scope.Fields() {
		if field.IsBlank() || field.IsIgnored() || !scope.Search.changeableField(field) {
			continue
		}

		if scope.willSaveFieldAssociations(field) && field.RelationIsBelongsTo() {
			fieldValue := field.Value.Addr().Interface()
			scope.Err(newCon(scope.con).Save(fieldValue).Error)
			var (
				ForeignFieldNames         = field.GetSliceSetting(set_foreign_field_names)
				AssociationForeignDBNames = field.GetSliceSetting(set_association_foreign_db_names)
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

//[create step 7] [update step 6]
func (scope *Scope) saveAfterAssociationsCallback() *Scope {
	for _, field := range scope.Fields() {
		if field.IsBlank() || field.IsIgnored() || !scope.Search.changeableField(field) {
			continue
		}

		//Attention : relationship.Kind <= HAS_ONE means except BELONGS_TO
		if scope.willSaveFieldAssociations(field) && field.RelKind() <= rel_has_one {
			value := field.Value
			ForeignFieldNames := field.GetSliceSetting(set_foreign_field_names)
			AssociationForeignDBNames := field.GetSliceSetting(set_association_foreign_db_names)
			switch value.Kind() {
			case reflect.Slice:
				for i := 0; i < value.Len(); i++ {
					//TODO : @Badu - cloneCon without copy, then NewScope which clone's con - but with copy
					newCon := newCon(scope.con)
					elem := value.Index(i).Addr().Interface()
					newScope := newCon.NewScope(elem)

					if !field.HasSetting(set_join_table_handler) && ForeignFieldNames.len() != 0 {
						for idx, fieldName := range ForeignFieldNames {
							associationForeignName := AssociationForeignDBNames[idx]
							if f, ok := scope.FieldByName(associationForeignName); ok {
								scope.Err(newScope.SetColumn(fieldName, f.Value.Interface()))
							}
						}
					}

					if field.HasSetting(set_polymorphic_type) {
						scope.Err(
							newScope.SetColumn(
								field.GetStrSetting(set_polymorphic_type),
								field.GetStrSetting(set_polymorphic_value)))
					}
					scope.Err(newCon.Save(elem).Error)

					if field.HasSetting(set_join_table_handler) {
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

				if field.HasSetting(set_polymorphic_type) {
					scope.Err(
						newScope.SetColumn(
							field.GetStrSetting(set_polymorphic_type),
							field.GetStrSetting(set_polymorphic_value)))
				}
				scope.Err(newCon(scope.con).Save(elem).Error)
			}
		}

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
