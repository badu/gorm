package benchmarks

import "fmt"

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
