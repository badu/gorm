package utils

import "sync"

// ConcurrentSlice type that can be safely shared between goroutines
type ConcurrentSlice struct {
	sync.RWMutex
	items []interface{}
}

// ConcurrentSliceItem contains the index/value pair of an item in a
// concurrent slice
type ConcurrentSliceItem struct {
	Index int
	Value interface{}
}

// NewConcurrentSlice creates a new concurrent slice
func NewConcurrentSlice() *ConcurrentSlice {
	cs := &ConcurrentSlice{
		items: make([]interface{}, 0),
	}

	return cs
}

// Append adds an item to the concurrent slice
func (cs *ConcurrentSlice) Add(item interface{}) {
	cs.Lock()
	defer cs.Unlock()

	cs.items = append(cs.items, item)
}

//returns the item at index
func (cs *ConcurrentSlice) At(index int) <-chan ConcurrentSliceItem {
	cs.Lock()
	defer cs.Unlock()

	c := make(chan ConcurrentSliceItem)

	f := func() {
		cs.Lock()
		defer cs.Lock()
		for idx, value := range cs.items {
			if idx == index {
				c <- ConcurrentSliceItem{idx, value}
				break
			}
		}
		close(c)
	}
	go f()

	return c

}

//Deletes the item with index from slice
func (cs *ConcurrentSlice) Delete(index int) {
	cs.Lock()
	defer cs.Unlock()
	cs.items = append(cs.items[:index], cs.items[index+1:]...)
}

// Iter iterates over the items in the concurrent slice
// Each item is sent over a channel, so that
// we can iterate over the slice using the builtin range keyword
func (cs *ConcurrentSlice) Iter() <-chan ConcurrentSliceItem {
	c := make(chan ConcurrentSliceItem)

	f := func() {
		cs.Lock()
		defer cs.Lock()
		for index, value := range cs.items {
			c <- ConcurrentSliceItem{index, value}
		}
		close(c)
	}
	go f()

	return c
}
