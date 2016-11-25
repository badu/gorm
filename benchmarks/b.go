package benchmarks

import (
	"time"
	"fmt"
	"runtime"
)

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
