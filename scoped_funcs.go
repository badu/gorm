package gorm

//shorter than append, better reading
func (s *ScopedFuncs) add(fx *ScopedFunc) {
	*s = append(*s, fx)
}

func (s *ScopedFuncs) len() int{
	return len(*s)
}