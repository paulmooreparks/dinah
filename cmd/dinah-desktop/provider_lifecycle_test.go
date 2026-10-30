package main

import (
	"studio/internal/sessionhost"
	"sync"
	"testing"
)

func TestProviderLifecycleIsStableForDirectAppFixtures(t *testing.T) {
	a := &App{}
	owners := make(chan *sessionhost.ProviderLifecycle, 8)
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() { defer workers.Done(); owners <- a.lifecycle() }()
	}
	workers.Wait()
	close(owners)
	expected := a.lifecycle()
	for owner := range owners {
		if owner != expected {
			t.Fatal("App constructed more than one lifecycle")
		}
	}
}
