package benchmarks

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
