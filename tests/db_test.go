package tests

import (
	"encoding/json"
	"fmt"
	"github.com/jinzhu/now"
	"gorm"
	_ "gorm/dialects/mysql"
	_ "gorm/dialects/sqlite"
	"os"
	"runtime"
	"testing"
	"time"
)

func toJSONString(v interface{}) []byte {
	r, _ := json.MarshalIndent(v, "", "  ")
	return r
}

func parseTime(str string) *time.Time {
	t := now.MustParse(str)
	return &t
}

func DialectHasTzSupport() bool {
	// NB: mssql and FoundationDB do not support time zones.
	if dialect := os.Getenv("GORM_DIALECT"); dialect == "mssql" || dialect == "foundation" {
		return false
	}
	return true
}

func OpenTestConnection(t *testing.T) {

	osDialect := os.Getenv("GORM_DIALECT")
	osDBAddress := os.Getenv("GORM_DBADDRESS")

	//osDialect = "mysql"
	//osDBAddress = "127.0.0.1:3306"

	switch osDialect {
	case "mysql":
		// CREATE USER 'gorm'@'localhost' IDENTIFIED BY 'gorm';
		// CREATE DATABASE gorm;
		// GRANT ALL ON * TO 'gorm'@'localhost';
		fmt.Println("testing mysql...")

		if osDBAddress != "" {
			osDBAddress = fmt.Sprintf("tcp(%v)", osDBAddress)
		}
		TestDB, TestDBErr = gorm.Open("mysql", fmt.Sprintf("root:@%v/gorm?charset=utf8&parseTime=True", osDBAddress))
	case "postgres":
		fmt.Println("testing postgres...")
		if osDBAddress != "" {
			osDBAddress = fmt.Sprintf("host=%v ", osDBAddress)
		}
		TestDB, TestDBErr = gorm.Open("postgres", fmt.Sprintf("%vuser=gorm password=gorm DB.name=gorm sslmode=disable", osDBAddress))
	case "foundation":
		fmt.Println("testing foundation...")
		TestDB, TestDBErr = gorm.Open("foundation", "dbname=gorm port=15432 sslmode=disable")
	default:

		TestDB, TestDBErr = gorm.Open("sqlite3", "test.db?cache=shared&mode=memory")
	}

	TestDB.DB().SetMaxIdleConns(10)
}

func RunMigration(t *testing.T) {
	if err := TestDB.DropTableIfExists(&User{}).Error; err != nil {
		fmt.Printf("Got error when try to delete table users, %+v\n", err)
	}

	for _, table := range []string{"animals", "user_languages"} {
		TestDB.Exec(fmt.Sprintf("drop table %v;", table))
	}

	values := []interface{}{
		&Short{},
		&ReallyLongThingThatReferencesShort{},
		&ReallyLongTableNameToTestMySQLNameLengthLimit{},
		&NotSoLongTableName{},
		&Product{},
		&Email{},
		&Address{},
		&CreditCard{},
		&Company{},
		&Role{},
		&Language{},
		&HNPost{},
		&EngadgetPost{},
		&Animal{}, &User{},
		&JoinTable{},
		&Post{},
		&Category{},
		&Comment{},
		&Cat{},
		&Dog{},
		&Hamster{},
		&Toy{},
		&ElementWithIgnoredField{},
	}
	for _, value := range values {
		TestDB.DropTable(value)
	}
	if err := TestDB.AutoMigrate(values...).Error; err != nil {
		panic(fmt.Sprintf("No error should happen when create table, but got %+v", err))
	}
}

func measureAndRun(t *testing.T, name string, f func(t *testing.T)) bool {
	runtime.ReadMemStats(&memStats)
	measurement := &Measure{
		startAllocs: memStats.Mallocs,
		startBytes:  memStats.TotalAlloc,
		name:        name,
	}

	measurement.start = time.Now()
	result := t.Run(name, f)
	measurement.duration += time.Now().Sub(measurement.start)

	runtime.ReadMemStats(&memStats)

	measurement.netAllocs += memStats.Mallocs - measurement.startAllocs
	measurement.netBytes += memStats.TotalAlloc - measurement.startBytes

	measuresData = append(measuresData, measurement)

	return result
}

func TestEverything(t *testing.T) {
	measureAndRun(t, "0) Open connection", OpenTestConnection)
	if TestDBErr != nil {
		t.Fatalf("No error should happen when connecting to test database, but got err=%+v", TestDBErr)
	}
	measureAndRun(t, "1) RunMigration", RunMigration)
	measureAndRun(t, "2) TestStringPrimaryKey", StringPrimaryKey)
	measureAndRun(t, "3) TestSetTable", SetTable)
	measureAndRun(t, "4) TestExceptionsWithInvalidSql", ExceptionsWithInvalidSql)
	measureAndRun(t, "5) TestHasTable", HasTable)
	measureAndRun(t, "6) TestTableName", TableName)
	measureAndRun(t, "7) TestNullValues", NullValues)
	measureAndRun(t, "8) TestNullValuesWithFirstOrCreate", NullValuesWithFirstOrCreate)
	measureAndRun(t, "9) TestTransaction", Transaction)
	measureAndRun(t, "10) TestRow", Row)
	measureAndRun(t, "11) TestRows", Rows)
	measureAndRun(t, "12) TestScanRows", ScanRows)
	measureAndRun(t, "13) TestScan", Scan)
	measureAndRun(t, "14) TestRaw", Raw)
	measureAndRun(t, "15) TestGroup", Group)
	measureAndRun(t, "16) TestJoins", Joins)
	measureAndRun(t, "17) TestJoinsWithSelect", JoinsWithSelect)
	measureAndRun(t, "18) TestHaving", Having)
	measureAndRun(t, "19) TestTimeWithZone", TimeWithZone)
	measureAndRun(t, "20) TestHstore", Hstore)
	measureAndRun(t, "21) TestSetAndGet", SetAndGet)
	measureAndRun(t, "22) TestCompatibilityMode", CompatibilityMode)
	measureAndRun(t, "23) TestOpenExistingDB", OpenExistingDB)
	measureAndRun(t, "24) TestDdlErrors", DdlErrors)
	measureAndRun(t, "25) TestOpenWithOneParameter", OpenWithOneParameter)
	measureAndRun(t, "26) TestBelongsTo", BelongsTo)
	measureAndRun(t, "27) TestBelongsToOverrideForeignKey1", BelongsToOverrideForeignKey1)
	measureAndRun(t, "28) TestBelongsToOverrideForeignKey2", BelongsToOverrideForeignKey2)
	measureAndRun(t, "29) TestHasOne", HasOne)
	measureAndRun(t, "30) TestHasOneOverrideForeignKey1", HasOneOverrideForeignKey1)
	measureAndRun(t, "31) TestHasOneOverrideForeignKey2", HasOneOverrideForeignKey2)
	measureAndRun(t, "32) TestHasMany", HasMany)
	measureAndRun(t, "33) TestHasManyOverrideForeignKey1", HasManyOverrideForeignKey1)
	measureAndRun(t, "34) TestHasManyOverrideForeignKey2", HasManyOverrideForeignKey2)
	measureAndRun(t, "35) TestManyToMany", ManyToMany)
	measureAndRun(t, "36) TestRelated", Related)
	measureAndRun(t, "37) TestForeignKey", ForeignKey)
	measureAndRun(t, "38) TestLongForeignKey", LongForeignKey)
	measureAndRun(t, "39) TestLongForeignKeyWithShortDest", LongForeignKeyWithShortDest)
	measureAndRun(t, "40) TestHasManyChildrenWithOneStruct", HasManyChildrenWithOneStruct)
	measureAndRun(t, "41) TestRegisterCallbackWithOrder", RegisterCallbackWithOrder)
	measureAndRun(t, "42) TestRegisterCallbackWithComplexOrder", RegisterCallbackWithComplexOrder)
	measureAndRun(t, "43) TestReplaceCallback", ReplaceCallback)
	measureAndRun(t, "44) TestRemoveCallback", RemoveCallback)
	measureAndRun(t, "45) TestRunCallbacks", RunCallbacks)
	measureAndRun(t, "46) TestCallbacksWithErrors", CallbacksWithErrors)
	measureAndRun(t, "47) TestCreate", Create)
	measureAndRun(t, "48) TestCreateWithAutoIncrement", CreateWithAutoIncrement)
	measureAndRun(t, "49) TestCreateWithNoGORMPrimayKey", CreateWithNoGORMPrimayKey)
	measureAndRun(t, "50) TestCreateWithNoStdPrimaryKeyAndDefaultValues", CreateWithNoStdPrimaryKeyAndDefaultValues)
	measureAndRun(t, "51) TestAnonymousScanner", AnonymousScanner)
	measureAndRun(t, "52) TestAnonymousField", AnonymousField)
	measureAndRun(t, "53) TestSelectWithCreate", SelectWithCreate)
	measureAndRun(t, "54) TestOmitWithCreate", OmitWithCreate)
	measureAndRun(t, "55) TestCustomizeColumn", DoCustomizeColumn)
	measureAndRun(t, "56) TestCustomColumnAndIgnoredFieldClash", DoCustomColumnAndIgnoredFieldClash)
	measureAndRun(t, "57) TestManyToManyWithCustomizedColumn", ManyToManyWithCustomizedColumn)
	measureAndRun(t, "58) TestOneToOneWithCustomizedColumn", OneToOneWithCustomizedColumn)
	measureAndRun(t, "59) TestOneToManyWithCustomizedColumn", OneToManyWithCustomizedColumn)
	measureAndRun(t, "60) TestHasOneWithPartialCustomizedColumn", HasOneWithPartialCustomizedColumn)
	measureAndRun(t, "61) TestBelongsToWithPartialCustomizedColumn", BelongsToWithPartialCustomizedColumn)
	measureAndRun(t, "62) TestDelete", DoDelete)
	measureAndRun(t, "63) TestInlineDelete", InlineDelete)
	measureAndRun(t, "64) TestSoftDelete", SoftDelete)
	measureAndRun(t, "65) TestPrefixColumnNameForEmbeddedStruct", PrefixColumnNameForEmbeddedStruct)
	measureAndRun(t, "66) TestSaveAndQueryEmbeddedStruct", SaveAndQueryEmbeddedStruct)
	measureAndRun(t, "67) TestCalculateField", DoCalculateField)
	measureAndRun(t, "68) TestJoinTable", DoJoinTable)
	measureAndRun(t, "69) TestIndexes", Indexes)
	measureAndRun(t, "70) TestAutoMigration", AutoMigration)
	measureAndRun(t, "71) TestMultipleIndexes", DoMultipleIndexes)
	measureAndRun(t, "72) TestManyToManyWithMultiPrimaryKeys", ManyToManyWithMultiPrimaryKeys)
	measureAndRun(t, "73) TestManyToManyWithCustomizedForeignKeys", ManyToManyWithCustomizedForeignKeys)
	measureAndRun(t, "74) TestManyToManyWithCustomizedForeignKeys2", ManyToManyWithCustomizedForeignKeys2)
	measureAndRun(t, "75) TestPointerFields", PointerFields)
	measureAndRun(t, "76) TestPolymorphic", Polymorphic)
	measureAndRun(t, "77) TestNamedPolymorphic", NamedPolymorphic)
	measureAndRun(t, "78) TestPreload", Preload)
	measureAndRun(t, "79) TestNestedPreload1", NestedPreload1)
	measureAndRun(t, "80) TestNestedPreload2", NestedPreload2)
	measureAndRun(t, "81) TestNestedPreload3", NestedPreload3)
	measureAndRun(t, "82) TestNestedPreload4", NestedPreload4)
	measureAndRun(t, "86) TestNestedPreload5", NestedPreload5)
	measureAndRun(t, "87) TestNestedPreload6", NestedPreload6)
	measureAndRun(t, "88) TestNestedPreload7", NestedPreload7)
	measureAndRun(t, "89) TestNestedPreload8", NestedPreload8)
	measureAndRun(t, "90) TestNestedPreload9", NestedPreload9)
	measureAndRun(t, "91) TestNestedPreload10", NestedPreload10)
	measureAndRun(t, "92) TestNestedPreload11", NestedPreload11)
	measureAndRun(t, "93) TestNestedPreload12", NestedPreload12)
	measureAndRun(t, "94) TestManyToManyPreloadWithMultiPrimaryKeys", ManyToManyPreloadWithMultiPrimaryKeys)
	measureAndRun(t, "95) TestManyToManyPreloadForNestedPointer", ManyToManyPreloadForNestedPointer)
	measureAndRun(t, "96) TestNestedManyToManyPreload", NestedManyToManyPreload)
	measureAndRun(t, "97) TestNestedManyToManyPreload2", NestedManyToManyPreload2)
	measureAndRun(t, "98) TestNestedManyToManyPreload3", NestedManyToManyPreload3)
	measureAndRun(t, "99) TestNestedManyToManyPreload3ForStruct", NestedManyToManyPreload3ForStruct)
	measureAndRun(t, "100) TestNestedManyToManyPreload4", NestedManyToManyPreload4)
	measureAndRun(t, "101) TestManyToManyPreloadForPointer", ManyToManyPreloadForPointer)
	measureAndRun(t, "102) TestNilPointerSlice", NilPointerSlice)
	measureAndRun(t, "103) TestNilPointerSlice2", NilPointerSlice2)
	measureAndRun(t, "104) TestPrefixedPreloadDuplication", PrefixedPreloadDuplication)
	measureAndRun(t, "105) TestFirstAndLast", FirstAndLast)
	measureAndRun(t, "106) TestFirstAndLastWithNoStdPrimaryKey", FirstAndLastWithNoStdPrimaryKey)
	measureAndRun(t, "107) TestUIntPrimaryKey", UIntPrimaryKey)
	measureAndRun(t, "108) TestStringPrimaryKeyForNumericValueStartingWithZero", StringPrimaryKeyForNumericValueStartingWithZero)
	measureAndRun(t, "109) TestFindAsSliceOfPointers", FindAsSliceOfPointers)
	measureAndRun(t, "110) TestSearchWithPlainSQL", SearchWithPlainSQL)
	measureAndRun(t, "111) TestSearchWithStruct", SearchWithStruct)
	measureAndRun(t, "112) TestSearchWithMap", SearchWithMap)
	measureAndRun(t, "113) TestSearchWithEmptyChain", SearchWithEmptyChain)
	measureAndRun(t, "114) TestSelect", Select)
	measureAndRun(t, "115) TestOrderAndPluck", OrderAndPluck)
	measureAndRun(t, "116) TestLimit", Limit)
	measureAndRun(t, "117) TestOffset", Offset)
	measureAndRun(t, "118) TestOr", Or)
	measureAndRun(t, "119) TestCount", Count)
	measureAndRun(t, "120) TestNot", Not)
	measureAndRun(t, "121) TestFillSmallerStruct", FillSmallerStruct)
	measureAndRun(t, "122) TestFindOrInitialize", FindOrInitialize)
	measureAndRun(t, "123) TestFindOrCreate", FindOrCreate)
	measureAndRun(t, "124) TestSelectWithEscapedFieldName", SelectWithEscapedFieldName)
	measureAndRun(t, "125) TestSelectWithVariables", SelectWithVariables)
	measureAndRun(t, "126) TestSelectWithArrayInput", SelectWithArrayInput)
	measureAndRun(t, "127) TestScannableSlices", ScannableSlices)
	measureAndRun(t, "128) TestScopes", Scopes)
	measureAndRun(t, "129) TestCloneSearch", CloneSearch)
	measureAndRun(t, "130) TestUpdate", Update)
	measureAndRun(t, "131) TestUpdateWithNoStdPrimaryKeyAndDefaultValues", UpdateWithNoStdPrimaryKeyAndDefaultValues)
	measureAndRun(t, "132) TestUpdates", Updates)
	measureAndRun(t, "133) TestUpdateColumn", UpdateColumn)
	measureAndRun(t, "134) TestSelectWithUpdate", SelectWithUpdate)
	measureAndRun(t, "135) TestSelectWithUpdateWithMap", SelectWithUpdateWithMap)
	measureAndRun(t, "136) TestOmitWithUpdate", OmitWithUpdate)
	measureAndRun(t, "137) TestOmitWithUpdateWithMap", OmitWithUpdateWithMap)
	measureAndRun(t, "138) TestSelectWithUpdateColumn", SelectWithUpdateColumn)
	measureAndRun(t, "139) TestOmitWithUpdateColumn", OmitWithUpdateColumn)
	measureAndRun(t, "140) TestUpdateColumnsSkipsAssociations", UpdateColumnsSkipsAssociations)
	measureAndRun(t, "141) TestUpdatesWithBlankValues", UpdatesWithBlankValues)
	measureAndRun(t, "142) TestUpdatesTableWithIgnoredValues", UpdatesTableWithIgnoredValues)
	measureAndRun(t, "143) TestUpdateDecodeVirtualAttributes", UpdateDecodeVirtualAttributes)
	measureAndRun(t, "144) TestToDBNameGenerateFriendlyName", ToDBNameGenerateFriendlyName)
	measureAndRun(t, "145) TestRegisterCallback", RegisterCallback)
	measureAndRun(t, "146) FEATURE : TestSkipSaveAssociation", SkipSaveAssociation)
	measureAndRun(t, "147) Test fix #1214 : FirstAndLastWithRaw", FirstAndLastWithRaw)

	t.Logf("TESTS SUMMARY:")
	totals := &Measure{
		netAllocs: 0,
		netBytes:  0,
		name:      "TOTAL:",
		duration:  0,
	}

	for _, measurement := range measuresData {
		totals.netAllocs += measurement.netAllocs
		totals.netBytes += measurement.netBytes
		totals.duration += measurement.duration
		t.Logf("%s : %s , %d allocs, %d bytes", measurement.name, measurement.duration, measurement.netAllocs, measurement.netBytes)
	}

	t.Logf("%s , %d allocs, %d bytes.", totals.duration, totals.netAllocs, totals.netBytes)
}

func TempTestAuto(t *testing.T) {
	measureAndRun(t, "0) Open connection", OpenTestConnection)
	if TestDBErr != nil {
		t.Fatalf("No error should happen when connecting to test database, but got err=%+v", TestDBErr)
	}
	measureAndRun(t, "1) RunMigration", RunMigration)

	for _, value := range gorm.ModelStructsMap.M() {
		t.Logf("%v", value)
	}
}
