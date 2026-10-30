package keylock

import (
	"sync"
	"testing"
	"time"
)

func TestOneKeyIsExclusive(t *testing.T) {
	var s Set
	var wg sync.WaitGroup
	inside, most := 0, 0
	var mu sync.Mutex
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := s.Lock("a")
			mu.Lock()
			inside++
			most = max(most, inside)
			mu.Unlock()
			time.Sleep(time.Millisecond)
			mu.Lock()
			inside--
			mu.Unlock()
			unlock()
		}()
	}
	wg.Wait()
	if most != 1 {
		t.Fatalf("%d callers held one key at once", most)
	}
	if len(s.keys) != 0 {
		t.Fatalf("%d keys were kept after use", len(s.keys))
	}
}

func TestKeysAreIndependent(t *testing.T) {
	var s Set
	unlock := s.Lock("a")
	defer unlock()
	done := make(chan struct{})
	go func() { s.Lock("b")(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("locking one key waited for another")
	}
}
