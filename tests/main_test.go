package tests

import (
	"os"
	"testing"
)

func TestAllGorm(t *testing.T) {
	//os.Setenv("GORM_DIALECT", "sqlite")
	//os.Setenv("GORM_DIALECT", "foundation")
	//os.Setenv("GORM_DIALECT", "postgres")
	os.Setenv("GORM_DIALECT", "mysql")
	os.Setenv("GORM_DBADDRESS", "127.0.0.1:3306")

	t.Run("0) Open connection", OpenTestConnection)
	if TestDBErr != nil {
		t.Fatalf("No error should happen when connecting to test database, but got err=%+v", TestDBErr)
	}
	t.Run("1) RunMigration", RunMigration)
	if TestDBErr != nil {
		t.Fatalf("No error should happen when connecting to test database, but got err=%+v", TestDBErr)
	}
	t.Run("2) TestStringPrimaryKey", StringPrimaryKey)
	t.Run("3) TestSetTable", SetTable)
	t.Run("4) TestExceptionsWithInvalidSql", ExceptionsWithInvalidSql)
	t.Run("5) TestHasTable", HasTable)
	t.Run("6) TestTableName", TableName)
	t.Run("7) TestNullValues", NullValues)
	t.Run("8) TestNullValuesWithFirstOrCreate", NullValuesWithFirstOrCreate)
	t.Run("9) TestTransaction", Transaction)
	t.Run("10) TestRow", Row)
	t.Run("11) TestRows", Rows)
	t.Run("12) TestScanRows", ScanRows)
	t.Run("13) TestScan", Scan)
	t.Run("14) TestRaw", Raw)
	t.Run("15) TestGroup", Group)
	t.Run("16) TestJoins", Joins)
	t.Run("17) TestJoinsWithSelect", JoinsWithSelect)
	t.Run("18) TestHaving", Having)
	t.Run("19) TestTimeWithZone", TimeWithZone)
	t.Run("20) TestHstore", Hstore)
	t.Run("21) TestSetAndGet", SetAndGet)
	t.Run("22) TestCompatibilityMode", CompatibilityMode)
	t.Run("23) TestOpenExistingDB", OpenExistingDB)
	t.Run("24) TestDdlErrors", DdlErrors)
	t.Run("25) TestOpenWithOneParameter", OpenWithOneParameter)
	t.Run("26) TestBelongsTo", BelongsTo)
	t.Run("27) TestBelongsToOverrideForeignKey1", BelongsToOverrideForeignKey1)
	t.Run("28) TestBelongsToOverrideForeignKey2", BelongsToOverrideForeignKey2)
	t.Run("29) TestHasOne", HasOne)
	t.Run("30) TestHasOneOverrideForeignKey1", HasOneOverrideForeignKey1)
	t.Run("31) TestHasOneOverrideForeignKey2", HasOneOverrideForeignKey2)
	t.Run("32) TestHasMany", HasMany)
	t.Run("33) TestHasManyOverrideForeignKey1", HasManyOverrideForeignKey1)
	t.Run("34) TestHasManyOverrideForeignKey2", HasManyOverrideForeignKey2)
	t.Run("35) TestManyToMany", ManyToMany)
	t.Run("36) TestRelated", Related)
	t.Run("37) TestForeignKey", ForeignKey)
	t.Run("38) TestLongForeignKey", LongForeignKey)
	t.Run("39) TestLongForeignKeyWithShortDest", LongForeignKeyWithShortDest)
	t.Run("40) TestHasManyChildrenWithOneStruct", HasManyChildrenWithOneStruct)
	t.Run("41) TestRegisterCallbackWithOrder", RegisterCallbackWithOrder)
	t.Run("42) TestRegisterCallbackWithComplexOrder", RegisterCallbackWithComplexOrder)
	t.Run("43) TestReplaceCallback", ReplaceCallback)
	t.Run("44) TestRemoveCallback", RemoveCallback)
	t.Run("45) TestRunCallbacks", RunCallbacks)
	t.Run("46) TestCallbacksWithErrors", CallbacksWithErrors)
	t.Run("47) TestCreate", Create)
	t.Run("48) TestCreateWithAutoIncrement", CreateWithAutoIncrement)
	t.Run("49) TestCreateWithNoGORMPrimayKey", CreateWithNoGORMPrimayKey)
	t.Run("50) TestCreateWithNoStdPrimaryKeyAndDefaultValues", CreateWithNoStdPrimaryKeyAndDefaultValues)
	t.Run("51) TestAnonymousScanner", AnonymousScanner)
	t.Run("52) TestAnonymousField", AnonymousField)
	t.Run("53) TestSelectWithCreate", SelectWithCreate)
	t.Run("54) TestOmitWithCreate", OmitWithCreate)
	t.Run("55) TestCustomizeColumn", DoCustomizeColumn)
	t.Run("56) TestCustomColumnAndIgnoredFieldClash", DoCustomColumnAndIgnoredFieldClash)
	t.Run("57) TestManyToManyWithCustomizedColumn", ManyToManyWithCustomizedColumn)
	t.Run("58) TestOneToOneWithCustomizedColumn", OneToOneWithCustomizedColumn)
	t.Run("59) TestOneToManyWithCustomizedColumn", OneToManyWithCustomizedColumn)
	t.Run("60) TestHasOneWithPartialCustomizedColumn", HasOneWithPartialCustomizedColumn)
	t.Run("61) TestBelongsToWithPartialCustomizedColumn", BelongsToWithPartialCustomizedColumn)
	t.Run("62) TestDelete", DoDelete)
	t.Run("63) TestInlineDelete", InlineDelete)
	t.Run("64) TestSoftDelete", SoftDelete)
	t.Run("65) TestPrefixColumnNameForEmbeddedStruct", PrefixColumnNameForEmbeddedStruct)
	t.Run("66) TestSaveAndQueryEmbeddedStruct", SaveAndQueryEmbeddedStruct)
	t.Run("67) TestCalculateField", DoCalculateField)
	t.Run("68) TestJoinTable", DoJoinTable)
	t.Run("69) TestIndexes", Indexes)
	t.Run("70) TestAutoMigration", AutoMigration)
	t.Run("71) TestMultipleIndexes", DoMultipleIndexes)
	t.Run("72) TestManyToManyWithMultiPrimaryKeys", ManyToManyWithMultiPrimaryKeys)
	t.Run("73) TestManyToManyWithCustomizedForeignKeys", ManyToManyWithCustomizedForeignKeys)
	t.Run("74) TestManyToManyWithCustomizedForeignKeys2", ManyToManyWithCustomizedForeignKeys2)
	t.Run("75) TestPointerFields", PointerFields)
	t.Run("76) TestPolymorphic", Polymorphic)
	t.Run("77) TestNamedPolymorphic", NamedPolymorphic)
	t.Run("78) TestPreload", Preload)
	t.Run("79) TestNestedPreload1", NestedPreload1)
	t.Run("80) TestNestedPreload2", NestedPreload2)
	t.Run("81) TestNestedPreload3", NestedPreload3)
	t.Run("82) TestNestedPreload4", NestedPreload4)
	t.Run("86) TestNestedPreload5", NestedPreload5)
	t.Run("87) TestNestedPreload6", NestedPreload6)
	t.Run("88) TestNestedPreload7", NestedPreload7)
	t.Run("89) TestNestedPreload8", NestedPreload8)
	t.Run("90) TestNestedPreload9", NestedPreload9)
	t.Run("91) TestNestedPreload10", NestedPreload10)
	t.Run("92) TestNestedPreload11", NestedPreload11)
	t.Run("93) TestNestedPreload12", NestedPreload12)
	t.Run("94) TestManyToManyPreloadWithMultiPrimaryKeys", ManyToManyPreloadWithMultiPrimaryKeys)
	t.Run("95) TestManyToManyPreloadForNestedPointer", ManyToManyPreloadForNestedPointer)
	t.Run("96) TestNestedManyToManyPreload", NestedManyToManyPreload)
	t.Run("97) TestNestedManyToManyPreload2", NestedManyToManyPreload2)
	t.Run("98) TestNestedManyToManyPreload3", NestedManyToManyPreload3)
	t.Run("99) TestNestedManyToManyPreload3ForStruct", NestedManyToManyPreload3ForStruct)
	t.Run("100) TestNestedManyToManyPreload4", NestedManyToManyPreload4)
	t.Run("101) TestManyToManyPreloadForPointer", ManyToManyPreloadForPointer)
	t.Run("102) TestNilPointerSlice", NilPointerSlice)
	t.Run("103) TestNilPointerSlice2", NilPointerSlice2)
	t.Run("104) TestPrefixedPreloadDuplication", PrefixedPreloadDuplication)
	t.Run("105) TestFirstAndLast", FirstAndLast)
	t.Run("106) TestFirstAndLastWithNoStdPrimaryKey", FirstAndLastWithNoStdPrimaryKey)
	t.Run("107) TestUIntPrimaryKey", UIntPrimaryKey)
	t.Run("108) TestStringPrimaryKeyForNumericValueStartingWithZero", StringPrimaryKeyForNumericValueStartingWithZero)
	t.Run("109) TestFindAsSliceOfPointers", FindAsSliceOfPointers)
	t.Run("110) TestSearchWithPlainSQL", SearchWithPlainSQL)
	t.Run("111) TestSearchWithStruct", SearchWithStruct)
	t.Run("112) TestSearchWithMap", SearchWithMap)
	t.Run("113) TestSearchWithEmptyChain", SearchWithEmptyChain)
	t.Run("114) TestSelect", Select)
	t.Run("115) TestOrderAndPluck", OrderAndPluck)
	t.Run("116) TestLimit", Limit)
	t.Run("117) TestOffset", Offset)
	t.Run("118) TestOr", Or)
	t.Run("119) TestCount", Count)
	t.Run("120) TestNot", Not)
	t.Run("121) TestFillSmallerStruct", FillSmallerStruct)
	t.Run("122) TestFindOrInitialize", FindOrInitialize)
	t.Run("123) TestFindOrCreate", FindOrCreate)
	t.Run("124) TestSelectWithEscapedFieldName", SelectWithEscapedFieldName)
	t.Run("125) TestSelectWithVariables", SelectWithVariables)
	t.Run("126) TestFirstAndLastWithRaw (fix #1214)", FirstAndLastWithRaw)
	t.Run("127) TestScannableSlices", ScannableSlices)
	t.Run("128) TestScopes", Scopes)
	t.Run("129) TestCloneSearch", CloneSearch)
	t.Run("130) TestUpdate", Update)
	t.Run("131) TestUpdateWithNoStdPrimaryKeyAndDefaultValues", UpdateWithNoStdPrimaryKeyAndDefaultValues)
	t.Run("132) TestUpdates", Updates)
	t.Run("133) TestUpdateColumn", UpdateColumn)
	t.Run("134) TestSelectWithUpdate", SelectWithUpdate)
	t.Run("135) TestSelectWithUpdateWithMap", SelectWithUpdateWithMap)
	t.Run("136) TestOmitWithUpdate", OmitWithUpdate)
	t.Run("137) TestOmitWithUpdateWithMap", OmitWithUpdateWithMap)
	t.Run("138) TestSelectWithUpdateColumn", SelectWithUpdateColumn)
	t.Run("139) TestOmitWithUpdateColumn", OmitWithUpdateColumn)
	t.Run("140) TestUpdateColumnsSkipsAssociations", UpdateColumnsSkipsAssociations)
	t.Run("141) TestUpdatesWithBlankValues", UpdatesWithBlankValues)
	t.Run("142) TestUpdatesTableWithIgnoredValues", UpdatesTableWithIgnoredValues)
	t.Run("143) TestUpdateDecodeVirtualAttributes", UpdateDecodeVirtualAttributes)
	t.Run("144) TestToDBNameGenerateFriendlyName", ToDBNameGenerateFriendlyName)
	t.Run("145) TestRegisterCallback", RegisterCallback)
	t.Run("146) TestSkipSaveAssociation", SkipSaveAssociation)
	t.Run("147) QueryOption", QueryOption)
}

func TempTestFailure(t *testing.T) {
	os.Setenv("GORM_DIALECT", "mysql")
	os.Setenv("GORM_DBADDRESS", "127.0.0.1:3306")

	t.Run("0) Open connection", OpenTestConnection)
	if TestDBErr != nil {
		t.Fatalf("No error should happen when connecting to test database, but got err=%+v", TestDBErr)
	}
	//TestDB.SetLogMode(gorm.LOG_DEBUG)
	t.Run("1) RunMigration", RunMigration)

	//t.Run("ManyToManyWithMultiPrimaryKeys", ManyToManyWithMultiPrimaryKeys)
}

func TempTestListModels(t *testing.T) {
	t.Run("0) Open connection", OpenTestConnection)
	if TestDBErr != nil {
		t.Fatalf("No error should happen when connecting to test database, but got err=%+v", TestDBErr)
	}
	//TestDB.SetLogMode(gorm.LOG_DEBUG)
	t.Run("1) RunMigration", RunMigration)

	for _, value := range TestDB.KnownModelStructs() {
		t.Logf("%v", value)
	}
}
