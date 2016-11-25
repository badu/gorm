package benchmarks

import (
	"fmt"

	jinzhu "github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"testing"
)

func initJinzhu() {
	var err error
	jinzhuCon, err = jinzhu.Open("sqlite3", "bench.db?cache=shared&mode=memory")
	checkErr(err)
	//defer DB.Close()
	jinzhuCon.AutoMigrate(Model{})
}

func JinzhuInsert(b *B) {
	var m *Model
	wrapExecute(b, func() {
		initJinzhu()
		m = NewModel()
	})

	for i := 0; i < b.N; i++ {
		m.Id = 0
		d := jinzhuCon.Create(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	}
}

func JinzhuUpdate(b *B) {
	var m *Model
	wrapExecute(b, func() {
		initJinzhu()
		m = NewModel()
		d := jinzhuCon.Create(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	})

	for i := 0; i < b.N; i++ {
		d := jinzhuCon.Save(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	}
}

func JinzhuRead(b *B) {
	var m *Model
	wrapExecute(b, func() {
		initJinzhu()
		m = NewModel()
		d := jinzhuCon.Create(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	})
	for i := 0; i < b.N; i++ {
		d := jinzhuCon.Find(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	}
}

func JinzhuReadSlice(b *B) {
	var m *Model
	wrapExecute(b, func() {
		initJinzhu()
		m = NewModel()
		for i := 0; i < 100; i++ {
			m.Id = 0
			d := jinzhuCon.Create(&m)
			if d.Error != nil {
				fmt.Println(d.Error)
				b.FailNow()
			}
		}
	})

	for i := 0; i < b.N; i++ {
		var models []*Model
		d := jinzhuCon.Where("id > ?", 0).Limit(100).Find(&models)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	}
}

func BenchmarkJinzhuGorm(b *testing.B) {
	st := NewSuite("jinzhu")
	st.InitF = func() {
		st.AddBenchmark("Insert", 2000*ORM_MULTI, JinzhuInsert)
		st.AddBenchmark("Update", 2000*ORM_MULTI, JinzhuUpdate)
		st.AddBenchmark("Read", 4000*ORM_MULTI, JinzhuRead)
		st.AddBenchmark("MultiRead limit 100", 2000*ORM_MULTI, JinzhuReadSlice)
	}
	RunBenchmark("jinzhu")
}
