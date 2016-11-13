package benchmarks

import (
	"fmt"

	"github.com/jinzhu/gorm"
	_ "gorm/dialects/mysql"
	"os"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"
)

type (
	BenchmarkResult struct {
		N         int
		T         time.Duration
		MemAllocs uint64
		MemBytes  uint64
		FailedMsg string
	}
	common struct {
		mu     sync.RWMutex
		failed bool

		start    time.Time
		duration time.Duration
		signal   chan interface{}
	}
	B struct {
		common
		Brand string
		Name  string
		N     int
		F     func(b *B)

		timerOn bool

		startAllocs uint64
		startBytes  uint64

		netAllocs uint64
		netBytes  uint64

		result *BenchmarkResult
	}
	suite struct {
		Brand  string
		InitF  func()
		benchs []*B
		orders []string
	}
	BList []*B
	Model struct {
		Id      int `orm:"auto" gorm:"primary_key" db:"id"`
		Name    string
		Title   string
		Fax     string
		Web     string
		Age     int
		Right   bool
		Counter int64
	}
)

var (
	gormdb        *gorm.DB
	benchmarkLock sync.Mutex
	memStats      runtime.MemStats

	BrandNames     []string
	benchmarks     = make(map[string]*suite)
	benchmarksNums = 0

	ORM_MULTI    int
	ORM_MAX_IDLE int
	ORM_MAX_CONN int
	ORM_SOURCE   string
)

func (r BenchmarkResult) NsPerOp() int64 {
	if r.N <= 0 {
		return 0
	}
	return r.T.Nanoseconds() / int64(r.N)
}

func (r BenchmarkResult) AllocsPerOp() int64 {
	if r.N <= 0 {
		return 0
	}
	return int64(r.MemAllocs) / int64(r.N)
}

func (r BenchmarkResult) AllocedBytesPerOp() int64 {
	if r.N <= 0 {
		return 0
	}
	return int64(r.MemBytes) / int64(r.N)
}

func (r BenchmarkResult) String() string {
	if len(r.FailedMsg) > 0 {
		return "    " + r.FailedMsg
	}

	nsop := r.NsPerOp()
	total := fmt.Sprintf("   %5.2fs", float64(r.T)/float64(1e9))
	ns := fmt.Sprintf("   %10d ns/op", nsop)
	if r.N > 0 && nsop < 100 {

		if nsop < 10 {
			ns = fmt.Sprintf("%10.2f ns/op", float64(r.T.Nanoseconds())/float64(r.N))
		} else {
			ns = fmt.Sprintf("%9.1f ns/op", float64(r.T.Nanoseconds())/float64(r.N))
		}
	}
	return fmt.Sprintf("%s%s", total, ns) + fmt.Sprintf("%8d B/op  %5d allocs/op",
		r.AllocedBytesPerOp(), r.AllocsPerOp())
}

func (c *common) Fail() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failed = true
}
func (c *common) FailNow() {
	c.Fail()
	runtime.Goexit()
}

func (b *B) StartTimer() {
	if !b.timerOn {
		runtime.ReadMemStats(&memStats)
		b.startAllocs = memStats.Mallocs
		b.startBytes = memStats.TotalAlloc
		b.start = time.Now()
		b.timerOn = true
	}
}

func (b *B) StopTimer() {
	if b.timerOn {
		b.duration += time.Now().Sub(b.start)
		runtime.ReadMemStats(&memStats)
		b.netAllocs += memStats.Mallocs - b.startAllocs
		b.netBytes += memStats.TotalAlloc - b.startBytes
		b.timerOn = false
	}
}

func (b *B) ResetTimer() {
	if b.timerOn {
		runtime.ReadMemStats(&memStats)
		b.startAllocs = memStats.Mallocs
		b.startBytes = memStats.TotalAlloc
		b.start = time.Now()
	}
	b.duration = 0
	b.netAllocs = 0
	b.netBytes = 0
}

func (b *B) launch() {
	benchmarkLock.Lock()

	defer func() {
		if err := recover(); err != nil {
			b.failed = true
			b.result = &BenchmarkResult{FailedMsg: fmt.Sprint(err)}
		} else {
			b.result = &BenchmarkResult{b.N, b.duration, b.netAllocs, b.netBytes, ""}
		}

		b.signal <- b
		benchmarkLock.Unlock()
	}()

	runtime.GC()
	b.ResetTimer()
	b.StartTimer()
	b.F(b)
	b.StopTimer()
}

func (b *B) run() {
	go b.launch()
	<-b.signal
}

func (st *suite) AddBenchmark(name string, n int, run func(b *B)) {
	st.benchs = append(st.benchs, &B{
		common: common{
			signal: make(chan interface{}),
		},
		Name:  name,
		Brand: st.Brand,
		N:     n,
		F:     run,
	})
	if len(st.benchs) > benchmarksNums {
		benchmarksNums = len(st.benchs)
	}
}

func (st *suite) run() {
	for _, b := range st.benchs {
		b.run()
		fmt.Printf("%25s: %6d ", b.Name, b.N)
		fmt.Println(b.result.String())
	}
}

func NewSuite(name string) *suite {
	s := new(suite)
	s.Brand = name
	benchmarks[name] = s
	BrandNames = append(BrandNames, name)
	return s
}

func RunBenchmark(name string) {
	fmt.Printf("Running benchmark %q\n", name)
	if s, ok := benchmarks[name]; ok {
		s.InitF()
		if len(s.benchs) != benchmarksNums {
			checkErr(fmt.Errorf("%s have not enough benchmarks"))
		}
		s.run()
	} else {
		checkErr(fmt.Errorf("not found benchmark suite %s"))
	}
	MakeReport()
}

func (s BList) Len() int      { return len(s) }
func (s BList) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s BList) Less(i, j int) bool {
	if s[i].failed {
		return false
	}
	if s[j].failed {
		return true
	}
	return s[i].duration < s[j].duration
}

func MakeReport() (result string) {
	var first string

	for i := 0; i < benchmarksNums; i++ {

		var benchs BList

		for _, name := range BrandNames {

			if s, ok := benchmarks[name]; ok {

				if i >= len(s.benchs) {
					continue
				}

				b := s.benchs[i]

				if b.result == nil {
					continue
				}

				if len(first) == 0 {
					first = name
				}

				benchs = append(benchs, b)

				if name == first {
					result += fmt.Sprintf("%6d times - %s\n", b.N, b.Name)
				}
			}
		}

		sort.Sort(benchs)

		for _, b := range benchs {
			result += fmt.Sprintf("%10s: ", b.Brand) + b.result.String() + "\n"
		}

		if i < benchmarksNums-1 {
			result += "\n"
		}
	}
	return
}

func NewModel() *Model {
	m := new(Model)
	m.Name = "Orm Benchmark"
	m.Title = "Just a Benchmark for fun"
	m.Fax = "99909990"
	m.Web = "http://blog.milkpod29.me"
	m.Age = 100
	m.Right = true
	m.Counter = 1000

	return m
}

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
}

func wrapExecute(b *B, cbk func()) {
	b.StopTimer()
	defer b.StartTimer()
	cbk()
}

func initDB() {
	osDBAddress := "127.0.0.1:3306"
	if osDBAddress != "" {
		osDBAddress = fmt.Sprintf("tcp(%v)", osDBAddress)
	}
	var err error
	gormdb, err = gorm.Open("mysql", fmt.Sprintf("root:@%v/gorm?charset=utf8&parseTime=True", osDBAddress))
	checkErr(err)
	//defer DB.Close()
	gormdb.AutoMigrate(Model{})
}

func GormInsert(b *B) {
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

func GormInsertMulti(b *B) {
	panic(fmt.Errorf("Not support multi insert"))
}

func GormUpdate(b *B) {
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

func GormRead(b *B) {
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

func GormReadSlice(b *B) {
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

func BenchmarkOriginalGorm(b *testing.B) {
	st := NewSuite("gorm")
	st.InitF = func() {
		st.AddBenchmark("Insert", 2000*ORM_MULTI, GormInsert)
		st.AddBenchmark("MultiInsert 100 row", 500*ORM_MULTI, GormInsertMulti)
		st.AddBenchmark("Update", 2000*ORM_MULTI, GormUpdate)
		st.AddBenchmark("Read", 4000*ORM_MULTI, GormRead)
		st.AddBenchmark("MultiRead limit 100", 2000*ORM_MULTI, GormReadSlice)
	}
	RunBenchmark("gorm")
}
