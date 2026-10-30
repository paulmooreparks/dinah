// Package keylock gives a mutex per key, so that work on one record waits
// only for other work on the same record. A key's mutex is kept only while
// something holds it or waits for it.
package keylock

import "sync"

// Set is a set of mutexes by key. The zero value is ready to use.
type Set struct {
	mu   sync.Mutex
	keys map[string]*entry
}

type entry struct {
	mu   sync.Mutex
	refs int
}

// Lock locks key's mutex, waiting while another caller holds it, and returns
// the function that unlocks it.
func (s *Set) Lock(key string) (unlock func()) {
	s.mu.Lock()
	if s.keys == nil {
		s.keys = map[string]*entry{}
	}
	e := s.keys[key]
	if e == nil {
		e = &entry{}
		s.keys[key] = e
	}
	e.refs++
	s.mu.Unlock()
	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		s.mu.Lock()
		e.refs--
		if e.refs == 0 {
			delete(s.keys, key)
		}
		s.mu.Unlock()
	}
}
