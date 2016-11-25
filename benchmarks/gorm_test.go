package benchmarks

import (
	"fmt"

	badu "github.com/badu/gorm"
	_ "github.com/badu/gorm/dialects/sqlite"
	"testing"
)

func initBadu() {
	var err error
	baduCon, err = badu.Open("sqlite3", "bench.db?cache=shared&mode=memory")
	checkErr(err)
	//defer DB.Close()
	baduCon.AutoMigrate(Model{})
}

func BaduInsert(b *B) {
	var m *Model
	wrapExecute(b, func() {
		initBadu()
		m = NewModel()
	})

	for i := 0; i < b.N; i++ {
		m.Id = 0
		d := baduCon.Create(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	}
}

func BaduUpdate(b *B) {
	var m *Model
	wrapExecute(b, func() {
		initBadu()
		m = NewModel()
		d := baduCon.Create(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	})

	for i := 0; i < b.N; i++ {
		d := baduCon.Save(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	}
}

func BaduRead(b *B) {
	var m *Model
	wrapExecute(b, func() {
		initBadu()
		m = NewModel()
		d := baduCon.Create(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	})
	for i := 0; i < b.N; i++ {
		d := baduCon.Find(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	}
}

func BaduReadSlice(b *B) {
	var m *Model
	wrapExecute(b, func() {
		initBadu()
		m = NewModel()
		for i := 0; i < 100; i++ {
			m.Id = 0
			d := baduCon.Create(&m)
			if d.Error != nil {
				fmt.Println(d.Error)
				b.FailNow()
			}
		}
	})

	for i := 0; i < b.N; i++ {
		var models []*Model
		d := baduCon.Where("id > ?", 0).Limit(100).Find(&models)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	}
}

func BenchmarkBaduGorm(b *testing.B) {
	st := NewSuite("badu")
	st.InitF = func() {
		st.AddBenchmark("Insert", 2000*ORM_MULTI, BaduInsert)
		st.AddBenchmark("Update", 2000*ORM_MULTI, BaduUpdate)
		st.AddBenchmark("Read", 4000*ORM_MULTI, BaduRead)
		st.AddBenchmark("MultiRead limit 100", 2000*ORM_MULTI, BaduReadSlice)
	}
	RunBenchmark("badu")
}
