package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"studio/internal/bus"
)

func TestBusHandleIsSharedAndCannotReopenAfterClose(t *testing.T) {
	a, _ := providerTestApp(t)
	const callers = 12
	results := make(chan *bus.Bus, callers)
	for range callers {
		go func() {
			b, err := a.busHost().Bus()
			if err != nil {
				t.Error(err)
			}
			results <- b
		}()
	}
	first := <-results
	for range callers - 1 {
		if b := <-results; b != first {
			t.Fatal("concurrent callers opened different bus handles")
		}
	}
	if first == nil {
		t.Fatal("bus did not open")
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		if b != first {
			t.Fatal("the helper did not reuse the bus handle")
		}
		_, err := b.ChangeCounter(context.Background())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	a.busHost().CloseHandle()
	a.busHost().CloseHandle()
	if _, err := first.ChangeCounter(context.Background()); err == nil {
		t.Fatal("the old handle is still usable")
	}
	if _, err := a.busHost().Bus(); err == nil {
		t.Fatal("a closed App reopened the bus")
	}
}

func TestBusOpenFailureCanBeRetried(t *testing.T) {
	a, _ := providerTestApp(t)
	path := filepath.Join(configDir(), "bus.db")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := a.busHost().Bus(); err == nil {
		t.Fatal("opening a directory as the database succeeded")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := a.busHost().Bus(); err != nil {
		t.Fatalf("retry after correcting the path: %v", err)
	}
}

func TestBusWatcherStopsWithoutClosingSharedHandle(t *testing.T) {
	a, _ := providerTestApp(t)
	b, err := a.busHost().Bus()
	if err != nil {
		t.Fatal(err)
	}
	a.busHost().StartWatcher(context.Background())
	done := make(chan struct{})
	go func() { a.busHost().StopWatcher(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("bus watcher did not stop")
	}
	if _, err := b.ChangeCounter(context.Background()); err != nil {
		t.Fatalf("watcher closed the App's handle: %v", err)
	}
}
