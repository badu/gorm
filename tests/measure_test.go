package tests

import (
	_ "gorm/dialects/mysql"
	_ "gorm/dialects/sqlite"
	"runtime"
	"sort"
	"testing"
	"time"
)

func (m *Measure) Go() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.startAllocs = memStats.Mallocs
	m.startBytes = memStats.TotalAlloc
	m.start = time.Now()
}

func (m *Measure) Stop() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.duration = uint64(time.Now().Sub(m.start).Nanoseconds())
	m.netAllocs += memStats.Mallocs - m.startAllocs
	m.netBytes += memStats.TotalAlloc - m.startBytes
}

func measureAndRun(t *testing.T, name string, f func(t *testing.T)) bool {
	measurement := &Measure{name: name}

	measurement.Go()
	result := t.Run(name, f)
	measurement.Stop()

	measuresData = append(measuresData, measurement)

	return result
}

func TempTestEverything(t *testing.T) {
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
	measureAndRun(t, "126) Test fix #1214 : FirstAndLastWithRaw", FirstAndLastWithRaw)
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
	measureAndRun(t, "147) QueryOption", QueryOption)

	totals := &Measure{
		netAllocs: 0,
		netBytes:  0,
		name:      "TOTAL:",
		duration:  0,
	}

	sort.Sort(measuresData)

	for _, measurement := range measuresData {
		totals.netAllocs += measurement.netAllocs
		totals.netBytes += measurement.netBytes
		totals.duration += measurement.duration
		t.Logf("%s\n\t\t\t\t%d allocs, %d bytes, took %s", measurement.name, measurement.netAllocs, measurement.netBytes, DurationToString(measurement.duration))
	}

	t.Logf("TESTS SUMMARY:")
	t.Logf("\t\t\t%d allocs, %d bytes, took %s", totals.netAllocs, totals.netBytes, DurationToString(totals.duration))
}

type (
	MeasureData []*Measure
	Measure     struct {
		name        string
		duration    uint64
		startAllocs uint64 // The initial states of memStats.Mallocs and memStats.TotalAlloc.
		startBytes  uint64
		netAllocs   uint64 // The net total of this test after being run.
		netBytes    uint64
		start       time.Time
	}
)

var (
	measuresData MeasureData = make(MeasureData, 0, 0)
)

//implementation of Sort
func (ts MeasureData) Len() int {
	return len(ts)
}

//implementation of Sort
func (ts MeasureData) Swap(i, j int) {
	ts[i], ts[j] = ts[j], ts[i]
}

//implementation of Sort
func (ts MeasureData) Less(i, j int) bool {
	return ts[i].netAllocs < ts[j].netAllocs
}

//Copied from time.Duration
func DurationToString(u uint64) string {
	// Largest time is 2540400h10m10.000000000s
	var buf [32]byte
	w := len(buf)

	if u < uint64(time.Second) {
		// Special case: if duration is smaller than a second,
		// use smaller units, like 1.2ms
		var prec int
		w--
		buf[w] = 's'
		w--
		switch {
		case u == 0:
			return "0s"
		case u < uint64(time.Microsecond):
			// print nanoseconds
			prec = 0
			buf[w] = 'n'
		case u < uint64(time.Millisecond):
			// print microseconds
			prec = 3
			// U+00B5 'µ' micro sign == 0xC2 0xB5
			w-- // Need room for two bytes.
			copy(buf[w:], "µ")
		default:
			// print milliseconds
			prec = 6
			buf[w] = 'm'
		}
		w, u = fmtFrac(buf[:w], u, prec)
		w = fmtInt(buf[:w], u)
	} else {
		w--
		buf[w] = 's'

		w, u = fmtFrac(buf[:w], u, 9)

		// u is now integer seconds
		w = fmtInt(buf[:w], u%60)
		u /= 60

		// u is now integer minutes
		if u > 0 {
			w--
			buf[w] = 'm'
			w = fmtInt(buf[:w], u%60)
			u /= 60

			// u is now integer hours
			// Stop at hours because days can be different lengths.
			if u > 0 {
				w--
				buf[w] = 'h'
				w = fmtInt(buf[:w], u)
			}
		}
	}

	return string(buf[w:])
}

//Copied from time.Duration
// fmtFrac formats the fraction of v/10**prec (e.g., ".12345") into the
// tail of buf, omitting trailing zeros.  it omits the decimal
// point too when the fraction is 0.  It returns the index where the
// output bytes begin and the value v/10**prec.
func fmtFrac(buf []byte, v uint64, prec int) (nw int, nv uint64) {
	// Omit trailing zeros up to and including decimal point.
	w := len(buf)
	print := false
	for i := 0; i < prec; i++ {
		digit := v % 10
		print = print || digit != 0
		if print {
			w--
			buf[w] = byte(digit) + '0'
		}
		v /= 10
	}
	if print {
		w--
		buf[w] = '.'
	}
	return w, v
}

//Copied from time.Duration
// fmtInt formats v into the tail of buf.
// It returns the index where the output begins.
func fmtInt(buf []byte, v uint64) int {
	w := len(buf)
	if v == 0 {
		w--
		buf[w] = '0'
	} else {
		for v > 0 {
			w--
			buf[w] = byte(v%10) + '0'
			v /= 10
		}
	}
	return w
}
