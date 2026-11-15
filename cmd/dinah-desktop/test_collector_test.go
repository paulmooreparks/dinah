package main

import "sync"

// testCollector owns callback observations. Readers receive a copy so an
// asynchronous append cannot change the slice being inspected by an assertion.
type testCollector[T any] struct {
	mu    sync.Mutex
	items []T
}

func (c *testCollector[T]) Add(items ...T) {
	c.mu.Lock()
	c.items = append(c.items, items...)
	c.mu.Unlock()
}

func (c *testCollector[T]) Snapshot() []T {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]T(nil), c.items...)
}

func (c *testCollector[T]) Reset() {
	c.mu.Lock()
	c.items = nil
	c.mu.Unlock()
}
