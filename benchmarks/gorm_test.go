package benchmarks

import (
	"fmt"

	"github.com/badu/gorm"
	_ "gorm/dialects/mysql"
	"testing"

)

var (
	gormdb2        *gorm.DBCon
)

func initDB2() {
	osDBAddress := "127.0.0.1:3306"
	if osDBAddress != "" {
		osDBAddress = fmt.Sprintf("tcp(%v)", osDBAddress)
	}
	var err error
	gormdb2, err = gorm.Open("mysql", fmt.Sprintf("root:@%v/gorm?charset=utf8&parseTime=True", osDBAddress))
	checkErr(err)
	//defer DB.Close()
	gormdb2.AutoMigrate(Model{})
}

func GormInsert2(b *B) {
	fmt.Printf("GormInsert\n")
	var m *Model
	wrapExecute(b, func() {
		initDB()
		m = NewModel()
	})

	for i := 0; i < b.N; i++ {
		m.Id = 0
		d := gormdb.Create(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	}
}

func GormInsertMulti2(b *B) {
	panic(fmt.Errorf("Not support multi insert"))
}

func GormUpdate2(b *B) {
	fmt.Printf("GormUpdate\n")
	var m *Model
	wrapExecute(b, func() {
		initDB()
		m = NewModel()
		d := gormdb.Create(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	})

	for i := 0; i < b.N; i++ {
		d := gormdb.Save(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	}
}

func GormRead2(b *B) {
	fmt.Printf("GormRead\n")
	var m *Model
	wrapExecute(b, func() {
		initDB()
		m = NewModel()
		d := gormdb.Create(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	})
	for i := 0; i < b.N; i++ {
		d := gormdb.Find(&m)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	}
}

func GormReadSlice2(b *B) {
	fmt.Printf("GormReadSlice\n")
	var m *Model
	wrapExecute(b, func() {
		initDB()
		m = NewModel()
		for i := 0; i < 100; i++ {
			m.Id = 0
			d := gormdb.Create(&m)
			if d.Error != nil {
				fmt.Println(d.Error)
				b.FailNow()
			}
		}
	})

	for i := 0; i < b.N; i++ {
		var models []*Model
		d := gormdb.Where("id > ?", 0).Limit(100).Find(&models)
		if d.Error != nil {
			fmt.Println(d.Error)
			b.FailNow()
		}
	}
}
//2918166900 ns/op 2685153500 ns/op
func BenchmarkTGorm(b *testing.B) {
	st := NewSuite("gorm")
	st.InitF = func() {
		st.AddBenchmark("Insert", 2000*ORM_MULTI, GormInsert2)
		st.AddBenchmark("MultiInsert 100 row", 500*ORM_MULTI, GormInsertMulti2)
		st.AddBenchmark("Update", 2000*ORM_MULTI, GormUpdate2)
		st.AddBenchmark("Read", 4000*ORM_MULTI, GormRead2)
		st.AddBenchmark("MultiRead limit 100", 2000*ORM_MULTI, GormReadSlice2)
	}
	RunBenchmark("gorm")
}
