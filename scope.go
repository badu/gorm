package gorm

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Set set value by name
func (s *Scope) Set(settingType uint64, value interface{}) *Scope {
	s.con.localSet(settingType, value)
	return s
}

// Get get setting by name
func (s *Scope) Get(settingType uint64) (interface{}, bool) {
	return s.con.get(settingType)
}

// Err add error to Scope
func (s *Scope) Err(err error) error {
	if err != nil {
		//these are passed over to the con, so it can print what SQL was executing
		s.con.search.SQL = s.Search.SQL
		s.con.search.SQLVars = s.Search.SQLVars
		s.con.AddError(err)
	}
	return err
}

// HasError check if there are any error
func (s *Scope) HasError() bool {
	return s.con.Error != nil
}

func (s *Scope) Warn(v ...interface{}) {
	s.con.warnLog(v...)
}

// Fields get value's fields from ModelStruct
func (s *Scope) Fields() StructFields {
	if s.fields == nil {
		var (
			result        StructFields
			modelStruct   = s.GetModelStruct()
			scopeIsStruct = s.rValue.Kind() == reflect.Struct
		)
		for _, field := range modelStruct.fieldsMap.fields {
			if scopeIsStruct {
				fieldValue := s.rValue
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
		s.fields = &result
	}
	return *s.fields
}

// GetModelStruct get value's model struct, relationships based on struct and tag definition
func (s *Scope) GetModelStruct() *ModelStruct {
	var modelStruct ModelStruct
	// Scope value can't be nil
	if s.Value == nil {
		return &modelStruct
	}
	if s.rType.Kind() != reflect.Struct {
		// Scope value need to be a struct
		return &modelStruct
	}

	// Get Cached model struct
	if value := s.con.parent.modelsStructMap.get(s.rType); value != nil {
		return value
	}

	modelStruct.Create(s)

	//set cached ModelStruc
	s.con.parent.modelsStructMap.set(s.rType, &modelStruct)
	// ATTN : first we add it to cache map, otherwise will infinite cycle
	// build relationships
	modelStruct.processRelations(s)

	return &modelStruct
}

//deprecated
func (s *Scope) PrimaryFields() StructFields {
	return s.PKs()
}

// was PrimaryFields() : PKs() return scope's primary fields
func (s *Scope) PKs() StructFields {
	var fields StructFields
	for _, field := range s.Fields() {
		if field.IsPrimaryKey() {
			fields.add(field)
		}
	}
	return fields
}

// deprecated
func (s *Scope) PrimaryField() *StructField {
	return s.PK()
}

// was PrimaryField() - PK() return scope's main primary field, if defined more that one primary fields, will return the one having column name `id` or the first one
func (s *Scope) PK() *StructField {
	primaryFieldsLen := s.GetModelStruct().noOfPKs()
	if primaryFieldsLen > 0 {
		if primaryFieldsLen > 1 {
			if field, ok := s.FieldByName(fieldDefaultIdName); ok {
				return field
			}
		}
		//and return the first one
		return s.PKs()[0]
	}
	return nil
}

//deprecated
func (s *Scope) PrimaryKey() string {
	return s.PKName()
}

// was PrimaryKey() - PKName() get main primary field's db name
func (s *Scope) PKName() string {
	if field := s.PK(); field != nil {
		return field.DBName
	}
	return ""
}

// PrimaryKeyZero check main primary field's value is blank or not
func (s *Scope) PrimaryKeyZero() bool {
	field := s.PK()
	return field == nil || field.IsBlank()
}

// PrimaryKeyValue get the primary key's value
func (s *Scope) PrimaryKeyValue() interface{} {
	if field := s.PK(); field != nil && field.Value.IsValid() {
		return field.Value.Interface()
	}
	return 0
}

// FieldByName find `gorm.StructField` with field name or db name
func (s *Scope) FieldByName(name string) (*StructField, bool) {
	var (
		dbName           = s.con.parent.namesMap.toDBName(name)
		mostMatchedField *StructField
	)

	for _, field := range s.Fields() {
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
func (s *Scope) SetColumn(column interface{}, value interface{}) error {
	switch colType := column.(type) {
	case *StructField:
		if s.updateMaps != nil {
			s.updateMaps[colType.DBName] = value
		}
		return colType.Set(value)
	case string:
		//looks like Scope.FieldByName
		var (
			dbName           = s.con.parent.namesMap.toDBName(colType)
			mostMatchedField *StructField
		)
		for _, field := range s.Fields() {
			if field.DBName == value {
				if s.updateMaps != nil {
					s.updateMaps[field.DBName] = value
				}
				return field.Set(value)
			}
			if (field.DBName == dbName) || (field.StructName == colType && mostMatchedField == nil) {
				mostMatchedField = field
			}
		}

		if mostMatchedField != nil {
			if s.updateMaps != nil {
				s.updateMaps[mostMatchedField.DBName] = value
			}
			return mostMatchedField.Set(value)
		}
	}
	return fmt.Errorf("SCOPE : could not convert column %q to field %v", column, value)
}

// TableName return table name
func (s *Scope) TableName() string {
	//fix : if implements tabler or dbtabler should override the search table name
	switch tblr := s.Value.(type) {
	case tabler:
		return tblr.TableName()
	case dbTabler:
		return tblr.TableName(s.con)
	}

	if s.Search != nil && s.Search.tableName != "" {
		return s.Search.tableName
	}

	return s.GetModelStruct().TableName(s.con.Model(s.Value))
}

// Raw set raw sql
func (s *Scope) Raw(sql string) *Scope {
	s.Search.SQL = strings.Replace(sql, "$$", "?", -1)
	s.Search.SetRaw()
	return s
}

// Exec perform generated SQL
func (s *Scope) Exec() *Scope {
	//fail fast
	if s.HasError() {
		return s
	}
	//avoid call if we don't need to
	if s.con.logMode == LogVerbose || s.con.logMode == LogDebug {
		defer s.trace(NowFunc())
	}
	s.Search.Exec(s)
	return s
}

// CallMethod call scope value's method, if it is a slice, will call its element's method one by one
func (s *Scope) CallMethod(methodName string) {
	if s.Value == nil {
		return
	}
	if s.rValue.Kind() == reflect.Slice {
		for i := 0; i < s.rValue.Len(); i++ {
			s.callMethod(methodName, s.rValue.Index(i))
		}
	} else {
		s.callMethod(methodName, s.rValue)
	}
}

////////////////////////////////////////////////////////////////////////////////
// Private Methods For *gorm.Scope
////////////////////////////////////////////////////////////////////////////////

func (s *Scope) getColumnAsArray(columns StrSlice) [][]interface{} {
	var results [][]interface{}
	switch s.rValue.Kind() {
	case reflect.Slice:
		for i := 0; i < s.rValue.Len(); i++ {
			var result []interface{}
			object := FieldValue(s.rValue, i)
			var hasValue = false
			for _, column := range columns {
				field := object.FieldByName(column)
				if !IsZero(field) {
					hasValue = true
				}
				result = append(result, field.Interface())
			}

			if hasValue {
				results = append(results, result)
			}
		}
	case reflect.Struct:
		var result []interface{}
		var hasValue = false
		for _, column := range columns {
			field := s.rValue.FieldByName(column)
			if !IsZero(field) {
				hasValue = true
			}
			result = append(result, field.Interface())
		}
		if hasValue {
			results = append(results, result)
		}
	}

	return results
}

func (s *Scope) callMethod(methodName string, reflectValue reflect.Value) {
	// Only get address from non-pointer
	if reflectValue.CanAddr() && reflectValue.Kind() != reflect.Ptr {
		reflectValue = reflectValue.Addr()
	}

	if methodValue := reflectValue.MethodByName(methodName); methodValue.IsValid() {
		switch method := methodValue.Interface().(type) {
		case func():
			method()
		case func(*Scope):
			method(s)
		case func(*DBCon):
			newCon := s.con.empty()
			method(newCon)
			s.Err(newCon.Error)
		case func() error:
			s.Err(method())
		case func(*Scope) error:
			s.Err(method(s))
		case func(*DBCon) error:
			newCon := s.con.empty()
			s.Err(method(newCon))
			s.Err(newCon.Error)
		default:
			s.Err(fmt.Errorf("unsupported function %v", methodName))
		}
	}
}

func (s *Scope) scan(rows *sql.Rows, columns []string, fields StructFields) {
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

	s.Err(rows.Scan(values...))

	for index, field := range resetFields {
		if v := reflect.ValueOf(values[index]).Elem().Elem(); v.IsValid() {
			field.Value.Set(v)
		}
	}
}

func (s *Scope) row() *sql.Row {
	//avoid call if we don't need to
	if s.con.logMode == LogVerbose || s.con.logMode == LogDebug {
		defer s.trace(NowFunc())
	}
	if s.con.parent.callbacks.rowQueries.len() > 0 {
		s.callCallbacks(s.con.parent.callbacks.rowQueries)
	}
	s.Search.prepareQuerySQL(s)
	return s.Search.QueryRow(s)
}

func (s *Scope) rows() (*sql.Rows, error) {
	//avoid call if we don't need to
	if s.con.logMode == LogVerbose || s.con.logMode == LogDebug {
		defer s.trace(NowFunc())
	}
	if s.con.parent.callbacks.rowQueries.len() > 0 {
		s.callCallbacks(s.con.parent.callbacks.rowQueries)
	}
	s.Search.prepareQuerySQL(s)
	return s.Search.Query(s)
}

func (s *Scope) pluck(column string, value interface{}) *Scope {
	dest := reflect.Indirect(reflect.ValueOf(value))
	s.Search.Select(column)
	if dest.Kind() != reflect.Slice {
		s.Err(fmt.Errorf("results should be a slice, not %s", dest.Kind()))
		return s
	}

	rows, err := s.rows()
	if s.Err(err) == nil {
		defer rows.Close()
		for rows.Next() {
			elem := reflect.New(dest.Type().Elem()).Interface()
			s.Err(rows.Scan(elem))
			dest.Set(reflect.Append(dest, reflect.ValueOf(elem).Elem()))
		}
	}
	return s
}

func (s *Scope) count(value interface{}) *Scope {
	if !s.Search.hasSelect() {
		s.Search.Select("count(*)")
	} else {
		sqlPair := s.Search.getFirst(condSelectQuery)
		if sqlPair == nil {
			s.Warn("ERROR : search select_query should have exaclty one count")
			//error has occured in getting first item in slice
			return s
		}
		if !regExpCounter.MatchString(fmt.Sprint(sqlPair.expression)) {
			s.Search.Select("count(*)")
		}
	}
	s.Search.setIsOrderIgnored()
	s.Err(s.row().Scan(value))
	return s
}

// trace print sql log
func (s *Scope) trace(t time.Time) {
	if s.Search.SQL != "" {
		s.con.slog(s.Search.SQL, t, s.Search.SQLVars...)
	}
}

func (s *Scope) shouldSaveAssociations() bool {
	if saveAssociations, ok := s.Get(gormSettingSaveAssoc); ok {
		if v, ok := saveAssociations.(bool); ok && !v {
			return false
		}
		if v, ok := saveAssociations.(string); ok && (v == "skip" || v == "false") {
			return false
		}
	}
	return !s.HasError()
}

func (s *Scope) quoteIfPossible(str string) string {
	// only match string like `name`, `users.name`
	if regExpNameMatcher.MatchString(str) {
		return s.con.quote(str)
	}
	return str
}

func (s *Scope) toQueryCondition(columns StrSlice) string {
	newColumns := ""
	for _, column := range columns {
		if newColumns != "" {
			newColumns += ","
		}
		newColumns += s.con.quote(column)
	}

	if len(columns) > 1 {
		return fmt.Sprintf("(%v)", newColumns)
	}
	return newColumns
}

//TODO : since table name can be overriden we should use model's not search
func (s *Scope) quotedTableName() string {
	result := ""
	//fail fast
	if s.Search == nil || s.Search.tableName == "" {
		result = s.con.quote(s.TableName())
	}

	if strings.Index(s.Search.tableName, " ") != -1 {
		result = s.Search.tableName
	}
	if result == "" {
		result = s.con.quote(s.Search.tableName)
	}
	return result
}

func (s *Scope) related(value interface{}, foreignKeys ...string) *Scope {
	toScope := s.con.NewScope(value)
	tx := s.con.set(gormSettingAssociationSource, s.Value)

	dest := toScope.rValue.Type()
	for dest.Kind() == reflect.Slice || dest.Kind() == reflect.Ptr {
		dest = dest.Elem()
	}
	src := s.rValue.Type()
	for src.Kind() == reflect.Slice || src.Kind() == reflect.Ptr {
		src = src.Elem()
	}
	allKeys := append(foreignKeys, dest.Name()+fieldIdName, src.Name()+fieldIdName)

	for _, foreignKey := range allKeys {
		fromField, _ := s.FieldByName(foreignKey)
		//fail fast - from field is nil
		if fromField == nil {
			toField, _ := toScope.FieldByName(foreignKey)
			if toField == nil {
				//fail fast - continue : both fields are nil
				continue
			}
			aStr := fmt.Sprintf("%v = ?", s.con.quote(toField.DBName))
			s.Err(tx.Where(aStr, s.PrimaryKeyValue()).Find(value).Error)
			return s
		}

		//fail fast - relationship is nil
		if !fromField.HasRelations() {
			aStr := fmt.Sprintf("%v = ?", s.con.quote(toScope.PKName()))
			s.Err(tx.Where(aStr, fromField.Value.Interface()).Find(value).Error)
			return s
		}
		var (
			ForeignDBNames            = fromField.GetForeignDBNames()
			AssociationForeignDBNames = fromField.GetAssociationDBNames()
		)
		//now, fail fast is over
		switch fromField.RelKind() {
		case relMany2many:
			joinTableHandler := fromField.JoinHandler()
			s.Err(
				joinTableHandler.JoinWith(
					joinTableHandler,
					tx,
					s.Value,
				).
					Find(value).
					Error)

		case relBelongsTo:
			for idx, foreignKey := range ForeignDBNames {
				if field, ok := s.FieldByName(foreignKey); ok {
					tx = tx.Where(
						fmt.Sprintf(
							"%v = ?",
							s.con.quote(AssociationForeignDBNames[idx]),
						),
						field.Value.Interface(),
					)
				}
			}
			s.Err(tx.Find(value).Error)

		case relHasMany, relHasOne:
			for idx, foreignKey := range ForeignDBNames {
				if field, ok := s.FieldByName(AssociationForeignDBNames[idx]); ok {
					tx = tx.Where(
						fmt.Sprintf(
							"%v = ?",
							s.con.quote(foreignKey),
						),
						field.Value.Interface(),
					)
				}
			}

			if fromField.HasSetting(setPolymorphicType) {
				tx = tx.Where(
					fmt.Sprintf(
						"%v = ?",
						s.con.quote(fromField.GetStrSetting(setPolymorphicDbname)),
					),
					fromField.GetStrSetting(setPolymorphicValue),
				)
			}

			s.Err(tx.Find(value).Error)
		}
		return s

	}

	s.Err(fmt.Errorf("invalid association %v", foreignKeys))
	return s
}

func (s *Scope) willSaveFieldAssociations(field *StructField) bool {
	if field.IsBlank() || field.IsIgnored() || !s.Search.changeableField(field) {
		return false
	}

	//TODO : @Badu - make field WillSaveAssociations FLAG
	if field.HasSetting(setSaveAssociations) {
		set := field.GetStrSetting(setSaveAssociations)
		if set == "false" || set == "skip" {
			return false
		}
	}
	if field.HasRelations() {
		return true
	}

	return false
}

func (s *Scope) callCallbacks(funcs ScopedFuncs) *Scope {
	for _, f := range funcs {
		//was (*f)(s) - but IDE went balistic
		rf := *f
		rf(s)
	}
	return s
}

//calls methods after query
func (s *Scope) postQuery(dest interface{}) *Scope {
	//Was "queryCallback"
	//avoid call if we don't need to
	if s.con.logMode == LogVerbose || s.con.logMode == LogDebug {
		defer s.trace(NowFunc())
	}
	var (
		isSlice, isPtr  bool
		queryResultType reflect.Type
		queryResults    = s.rValue
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
		s.Err(fmt.Errorf("SCOPE : unsupported destination, should be slice or struct : %v", s))
		return s
	}

	s.Search.prepareQuerySQL(s)

	if !s.HasError() {
		s.con.RowsAffected = 0
		if str, ok := s.Get(gormSettingQueryOpt); ok {
			s.Search.SQL += addExtraSpaceIfExist(fmt.Sprint(str))
		}

		if rows, err := s.Search.Query(s); s.Err(err) == nil {
			defer rows.Close()

			columns, _ := rows.Columns()
			for rows.Next() {
				s.con.RowsAffected++

				elem := queryResults
				if isSlice {
					elem = reflect.New(queryResultType).Elem()
				}

				s.scan(rows, columns, s.con.emptyScope(elem.Addr().Interface()).Fields())

				if isSlice {
					if isPtr {
						queryResults.Set(reflect.Append(queryResults, elem.Addr()))
					} else {
						queryResults.Set(reflect.Append(queryResults, elem))
					}
				}
			}

			if s.con.RowsAffected == 0 && !isSlice {
				s.Err(ErrRecordNotFound)
			}
		}
	}
	//END Was "queryCallback"

	if s.Search.hasPreload() && !s.HasError() {
		s.Search.doPreload(s)
	}

	if !s.HasError() {
		s.CallMethod(methAfterFind)
	}

	return s
}

//calls methods after creation
func (s *Scope) postCreate() *Scope {
	//begin transaction
	result, txStarted := s.begin()

	//call callbacks
	if !result.HasError() {
		result.CallMethod(methBeforeSave)
	}
	if !result.HasError() {
		result.CallMethod(methBeforeCreate)
	}

	//save associations
	if result.shouldSaveAssociations() {
		result = result.saveBeforeAssociationsCallback()
	}

	//set time fields accordingly
	if !result.HasError() {
		now := NowFunc()
		result.SetColumn(FieldCreatedAt, now)
		result.SetColumn(FieldUpdatedAt, now)
	}

	var blankColumnsWithDefaultValue string

	//Was "createCallback" method
	if !result.HasError() {
		var (
			//because we're using it in a for, we're getting it once
			dialect                            = result.con.parent.dialect
			returningColumn                    = strEverything
			quotedTableName                    = result.quotedTableName()
			primaryField                       = result.PK()
			extraOption, columns, placeholders string
		)

		//avoid call if we don't need to
		if result.con.logMode == LogVerbose || s.con.logMode == LogDebug {
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
					blankColumnsWithDefaultValue += s.con.quote(field.DBName)
				} else if isNotPKOrBlank {
					if columns != "" {
						columns += ","
					}
					columns += s.con.quote(field.DBName)
					if placeholders != "" {
						placeholders += ","
					}
					placeholders += result.Search.addToVars(field.Value.Interface(), dialect)
				}
			} else {
				if field.HasRelations() && field.RelationIsBelongsTo() {
					ForeignDBNames := field.GetForeignDBNames()
					for _, foreignKey := range ForeignDBNames {
						foreignField, ok := result.FieldByName(foreignKey)
						if ok && !result.Search.changeableField(foreignField) {
							if columns != "" {
								columns += ","
							}
							columns += s.con.quote(foreignField.DBName)
							if placeholders != "" {
								placeholders += ","
							}
							placeholders += result.Search.addToVars(foreignField.Value.Interface(), dialect)
						}
					}
				}
			}
		}

		if str, ok := result.Get(gormSettingInsertOpt); ok {
			extraOption = fmt.Sprint(str)
		}

		if primaryField != nil {
			returningColumn = s.con.quote(primaryField.DBName)
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
				result.quotedTableName(),
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
		db := s.con.empty().Table(result.TableName()).Select(blankColumnsWithDefaultValue)
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
		result.CallMethod(methAfterCreate)
	}
	if !result.HasError() {
		result.CallMethod(methAfterSave)
	}

	//attempt to commit in the end
	return result.commitOrRollback(txStarted)
}

//calls methods after update
func (s *Scope) postUpdate(attrs interface{}) *Scope {
	if attrs != nil {
		updateMaps, hasUpdate := updatedAttrsWithValues(s, attrs)
		if hasUpdate {
			s.updateMaps = updateMaps
		} else {
			//we stop chain calls
			return s
		}
	}

	//begin transaction
	result, txStarted := s.begin()

	if _, ok := result.Get(gormSettingUpdateColumn); !ok {
		if !result.HasError() {
			result.CallMethod(methBeforeSave)
		}
		if !result.HasError() {
			result.CallMethod(methBeforeUpdate)
		}
	}

	//save associations
	if result.shouldSaveAssociations() {
		result = result.saveBeforeAssociationsCallback()
	}

	//update the updated at column
	if _, ok := result.Get(gormSettingUpdateColumn); !ok {
		result.SetColumn(FieldUpdatedAt, NowFunc())
	}

	//Was "updateCallback"
	if !result.HasError() {
		var (
			//because we're using it in a for, we're getting it once
			dialect          = result.con.parent.dialect
			extraOption, sql string
		)

		if result.updateMaps != nil {
			for column, value := range result.updateMaps {
				if sql != "" {
					sql += ", "
				}
				sql += fmt.Sprintf(
					"%v = %v",
					s.con.quote(column),
					result.Search.addToVars(value, dialect),
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
						s.con.quote(field.DBName),
						result.Search.addToVars(field.Value.Interface(), dialect),
					)
				} else {
					if field.HasRelations() && field.RelationIsBelongsTo() {
						ForeignDBNames := field.GetForeignDBNames()
						for _, foreignKey := range ForeignDBNames {
							foreignField, ok := result.FieldByName(foreignKey)
							if ok && !result.Search.changeableField(foreignField) {
								if sql != "" {
									sql += ", "
								}
								sql += fmt.Sprintf(
									"%v = %v",
									s.con.quote(foreignField.DBName),
									result.Search.addToVars(
										foreignField.Value.Interface(),
										dialect,
									),
								)
							}
						}
					}
				}

			}
		}

		if str, ok := result.Get(gormSettingUpdateOpt); ok {
			extraOption = fmt.Sprint(str)
		}

		if sql != "" && s.TableName() != "" {
			result.Raw(fmt.Sprintf(
				"UPDATE %v SET %v%v%v",
				result.quotedTableName(),
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

	if _, ok := result.Get(gormSettingUpdateColumn); !ok {
		if !result.HasError() {
			result.CallMethod(methAfterUpdate)
		}
		if !result.HasError() {
			result.CallMethod(methAfterSave)
		}
	}

	return result.commitOrRollback(txStarted)
}

//calls methods after deletion
func (s *Scope) postDelete() *Scope {
	//begin transaction
	result, txStarted := s.begin()

	//call callbacks
	if !result.HasError() {
		result.CallMethod(methBeforeDelete)
	}

	//Was "deleteCallback"
	if !result.HasError() {
		var extraOption string
		if str, ok := result.Get(gormSettingDeleteOpt); ok {
			extraOption = fmt.Sprint(str)
		}

		if !result.Search.isUnscoped() && result.GetModelStruct().HasColumn(FieldDeletedAt) {
			result.Raw(fmt.Sprintf(
				"UPDATE %v SET deleted_at=%v%v%v",
				result.quotedTableName(),
				result.Search.addToVars(NowFunc(), result.con.parent.dialect),
				addExtraSpaceIfExist(result.Search.combinedConditionSql(result)),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		} else {
			result.Raw(fmt.Sprintf(
				"DELETE FROM %v%v%v",
				result.quotedTableName(),
				addExtraSpaceIfExist(result.Search.combinedConditionSql(result)),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		}
	}
	//END - Was "deleteCallback"

	//call callbacks
	if !result.HasError() {
		result.CallMethod(methAfterDelete)
	}

	//attempt to commit
	return result.commitOrRollback(txStarted)
}

////////////////////////////////////////////////////////////////////////////////
// internal callbacks functions
////////////////////////////////////////////////////////////////////////////////
//[create step 1] [delete step 1] [update step 1]
// Begin start a transaction
func (s *Scope) begin() (*Scope, bool) {
	if db, ok := s.con.sqli.(sqlDb); ok {
		//parent db implements Begin() -> call Begin()
		if tx, err := db.Begin(); err == nil {
			//TODO : @Badu - maybe the parent should do so, since it's owner of db.db
			//parent db.db implements Exec(), Prepare(), Query() and QueryRow()
			//TODO : @Badu - it's paired with commit or rollback - see below
			s.con.sqli = interface{}(tx).(sqlInterf)
			return s, true
		}
	}
	return s, false
}

//[create step 3] [update step 3]
func (s *Scope) saveBeforeAssociationsCallback() *Scope {
	for _, field := range s.Fields() {
		if field.IsBlank() || field.IsIgnored() || !s.Search.changeableField(field) {
			continue
		}

		if s.willSaveFieldAssociations(field) && field.RelationIsBelongsTo() {
			fieldValue := field.Value.Addr().Interface()
			s.Err(s.con.empty().Save(fieldValue).Error)
			var (
				ForeignFieldNames         = field.GetForeignFieldNames()
				AssociationForeignDBNames = field.GetAssociationDBNames()
			)
			if ForeignFieldNames.len() != 0 {
				// set value's foreign key
				for idx, fieldName := range ForeignFieldNames {
					associationForeignName := AssociationForeignDBNames[idx]
					if foreignField, ok := s.con.emptyScope(fieldValue).FieldByName(associationForeignName); ok {
						s.Err(s.SetColumn(fieldName, foreignField.Value.Interface()))
					}
				}
			}
		}
	}
	return s
}

//[create step 7] [update step 6]
func (s *Scope) saveAfterAssociationsCallback() *Scope {
	for _, field := range s.Fields() {
		if field.IsBlank() || field.IsIgnored() || !s.Search.changeableField(field) {
			continue
		}

		//Attention : relationship.Kind <= HAS_ONE means except BELONGS_TO
		if s.willSaveFieldAssociations(field) && field.RelKind() <= relHasOne {
			value := field.Value
			ForeignFieldNames := field.GetForeignFieldNames()
			AssociationForeignDBNames := field.GetAssociationDBNames()
			switch value.Kind() {
			case reflect.Slice:
				for i := 0; i < value.Len(); i++ {
					//TODO : @Badu - cloneCon without copy, then NewScope which clone's con - but with copy
					newCon := s.con.empty()
					elem := value.Index(i).Addr().Interface()
					newScope := newCon.NewScope(elem)

					if !field.HasSetting(setJoinTableHandler) && ForeignFieldNames.len() != 0 {
						for idx, fieldName := range ForeignFieldNames {
							associationForeignName := AssociationForeignDBNames[idx]
							if f, ok := s.FieldByName(associationForeignName); ok {
								s.Err(newScope.SetColumn(fieldName, f.Value.Interface()))
							}
						}
					}

					if field.HasSetting(setPolymorphicType) {
						s.Err(
							newScope.SetColumn(
								field.GetStrSetting(setPolymorphicType),
								field.GetStrSetting(setPolymorphicValue)))
					}
					s.Err(newCon.Save(elem).Error)

					if field.HasSetting(setJoinTableHandler) {
						joinTableHandler := field.JoinHandler()
						s.Err(joinTableHandler.Add(joinTableHandler, newCon, s.Value, newScope.Value))
					}
				}
			default:
				elem := value.Addr().Interface()
				newScope := s.con.emptyScope(elem)

				if ForeignFieldNames.len() != 0 {
					for idx, fieldName := range ForeignFieldNames {
						associationForeignName := AssociationForeignDBNames[idx]
						if f, ok := s.FieldByName(associationForeignName); ok {
							s.Err(newScope.SetColumn(fieldName, f.Value.Interface()))
						}
					}
				}

				if field.HasSetting(setPolymorphicType) {
					s.Err(
						newScope.SetColumn(
							field.GetStrSetting(setPolymorphicType),
							field.GetStrSetting(setPolymorphicValue)))
				}
				s.Err(s.con.empty().Save(elem).Error)
			}
		}

	}
	return s
}

//[create step 9] [delete step 5] [update step 8]
// CommitOrRollback commit current transaction if no error happened, otherwise will rollback it
func (s *Scope) commitOrRollback(txStarted bool) *Scope {
	if txStarted {
		if db, ok := s.con.sqli.(sqlTx); ok {
			if s.HasError() {
				//orm.db implements Commit() and Rollback() -> call Rollback()
				db.Rollback()
			} else {
				//orm.db implements Commit() and Rollback() -> call Commit()
				s.Err(db.Commit())
			}
			//TODO : @Badu - it's paired with begin - see above
			s.con.sqli = s.con.parent.sqli
		}
	}
	return s
}
