package benchmarks

import "runtime"

func (c *common) Fail() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failed = true
}
func (c *common) FailNow() {
	c.Fail()
	runtime.Goexit()
}
