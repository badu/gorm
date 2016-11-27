package benchmarks

import (
	badu "gorm"
	jinzhu "github.com/jinzhu/gorm"
	"runtime"
	"sync"
	"time"
	"fmt"
	"sort"
	"os"
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
	jinzhuCon *jinzhu.DB
	baduCon   *badu.DBCon

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
	m.Web = "http://www.google.com"
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