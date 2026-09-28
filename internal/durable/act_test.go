package durable

import (
	"path/filepath"
	"testing"
)

// skipWithoutThreads skips a test of which act a goroutine belongs to on a
// platform where every goroutine shares one act.
func skipWithoutThreads(t *testing.T) {
	t.Helper()
	if _, known := currentThread(); !known {
		t.Skip("this platform offers no thread identifier, so every goroutine shares one act")
	}
}

// inGoroutine runs fn on a fresh goroutine and waits for it.
func inGoroutine(fn func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	<-done
}

// TestAnActsWriteBindsOnlyThatAct asserts that once one goroutine's act has
// written, an operation of that act may no longer give up, while an operation
// on another goroutine holding no lock, and one on a goroutine holding a lock
// of its own that has not written, still may. A worker that entered the
// writing act follows it.
func TestAnActsWriteBindsOnlyThatAct(t *testing.T) {
	skipWithoutThreads(t)
	dir := t.TempDir()
	written := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	var ref *ActRef
	go func() {
		defer close(finished)
		hold, _ := Register(filepath.Join(dir, "a", "lock"), "a")
		defer hold.Unregister()
		if !classify(true).mayGive {
			t.Error("an act that had not written may not give up")
		}
		classify(true).done()
		if classify(false).mayGive {
			t.Error("a read of the act that wrote may give up")
		}
		if !hold.Wrote() {
			t.Error("the entry does not report its act's write")
		}
		ref = CurrentAct()
		close(written)
		<-release
	}()
	<-written

	if c := classify(false); !c.mayGive || c.act != nil {
		t.Errorf("a goroutine holding no lock read another act's write: %+v", c)
	}
	inGoroutine(func() {
		hold, _ := Register(filepath.Join(dir, "b", "lock"), "b")
		defer hold.Unregister()
		if !classify(false).mayGive {
			t.Error("a goroutine whose own act had not written read another act's write")
		}
	})
	inGoroutine(func() {
		defer ref.Enter()()
		if classify(false).mayGive {
			t.Error("a worker that entered the writing act may give up")
		}
	})
	inGoroutine(func() {
		if classify(false).act != nil {
			t.Error("a worker that left the act still belongs to it")
		}
	})
	close(release)
	<-finished
	if len(registry.threads) != 0 || len(registry.entries) != 0 {
		t.Errorf("the registry kept %d threads and %d entries after every act ended", len(registry.threads), len(registry.entries))
	}
}

// TestAnActOutlivesItsInnerLocks asserts that a scope BeginAct opened, or an
// outer lock, keeps the act and its write across inner locks taken and given
// back, including an inner entry removed twice, so a later inner lock of the
// same act finds the write rather than starting afresh.
func TestAnActOutlivesItsInnerLocks(t *testing.T) {
	skipWithoutThreads(t)
	dir := t.TempDir()
	inGoroutine(func() {
		scope := BeginAct()
		outer, _ := Register(filepath.Join(dir, "lock"), "outer")
		first, _ := Register(filepath.Join(dir, "one", "lock"), "one")
		classify(true).done()
		first.Unregister()
		first.Unregister()
		outer.Unregister()
		second, _ := Register(filepath.Join(dir, "two", "lock"), "two")
		if classify(true).mayGive {
			t.Error("a later lock of the act that wrote may give up")
		}
		second.Unregister()
		scope.End()
		scope.End()
		if classify(false).act != nil {
			t.Error("the act outlived its scope")
		}
		after, _ := Register(filepath.Join(dir, "three", "lock"), "three")
		if !classify(true).mayGive {
			t.Error("a new act inherited the ended act's write")
		}
		after.Unregister()
	})
}
