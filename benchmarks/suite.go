package benchmarks

import "fmt"

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

