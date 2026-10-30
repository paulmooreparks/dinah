package rotlog

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

func TestBuffersUntilFlush(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.log")
	w := Open(p, Options{Delay: time.Hour})
	w.WriteString("one\n")
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("a write reached the disk before a flush")
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "one\n" {
		t.Fatalf("got %q", got)
	}
	w.Close()
}

func TestFlushesAfterTheDelay(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.log")
	w := Open(p, Options{Delay: 20 * time.Millisecond})
	defer w.Close()
	w.WriteString("later\n")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(p); err == nil && string(b) == "later\n" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the buffer was not written after the delay")
}

func TestFlushesPastTheThreshold(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.log")
	w := Open(p, Options{FlushSize: 8, Delay: time.Hour})
	defer w.Close()
	w.WriteString("0123456789\n")
	if got := read(t, p); got != "0123456789\n" {
		t.Fatalf("got %q", got)
	}
}

func TestRotatesAndKeeps(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "trace.log")
	w := Open(p, Options{MaxSize: 10, Keep: 3, FlushSize: 1, Delay: time.Hour})
	for _, s := range []string{"aaaaaaaa\n", "bbbbbbbb\n", "cccccccc\n", "dddddddd\n"} {
		w.WriteString(s)
	}
	w.Close()
	if got := read(t, p); got != "dddddddd\n" {
		t.Fatalf("current: %q", got)
	}
	if got := read(t, filepath.Join(dir, "trace.1.log")); got != "cccccccc\n" {
		t.Fatalf("first older: %q", got)
	}
	if got := read(t, filepath.Join(dir, "trace.2.log")); got != "bbbbbbbb\n" {
		t.Fatalf("second older: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "trace.3.log")); !os.IsNotExist(err) {
		t.Fatal("more files were kept than asked")
	}
}

func TestContinuesAnExistingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.log")
	os.WriteFile(p, []byte("before\n"), 0o600)
	w := Open(p, Options{})
	w.WriteString("after\n")
	w.Close()
	if got := read(t, p); got != "before\nafter\n" {
		t.Fatalf("got %q", got)
	}
}

func TestDoesNotRecreateARemovedFolderTree(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "gone", "logs", "a.log")
	w := Open(p, Options{Delay: time.Hour})
	w.WriteString("x\n")
	if err := w.Flush(); err == nil {
		t.Fatal("a flush created the folders above the log's own")
	}
	if _, err := os.Stat(filepath.Join(dir, "gone")); !os.IsNotExist(err) {
		t.Fatal("the removed tree came back")
	}
	w.Close()
}

func TestCreatesItsOwnFolder(t *testing.T) {
	p := filepath.Join(t.TempDir(), "chats", "a.log")
	w := Open(p, Options{})
	w.WriteString("x\n")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "x\n" {
		t.Fatalf("got %q", got)
	}
}

func TestTheFileIsNotHeldOpen(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.log")
	w := Open(p, Options{FlushSize: 1})
	w.WriteString("x\n")
	// On Windows an open file cannot be removed.
	if err := os.Remove(p); err != nil {
		t.Fatalf("the file is held: %v", err)
	}
	w.WriteString("y\n")
	w.Close()
	if got := read(t, p); got != "y\n" {
		t.Fatalf("got %q", got)
	}
}

func TestKeepsAndThenDropsWhileTheFileCannotBeWritten(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocked")
	os.WriteFile(blocker, []byte("x"), 0o600)
	// The log's folder is a file, so the log cannot be created.
	p := filepath.Join(blocker, "a.log")
	w := Open(p, Options{MaxBuffer: 16, FlushSize: 1 << 20, Delay: time.Hour})
	w.WriteString("0123456789\n")
	w.WriteString("this one does not fit\n")
	if err := w.Flush(); err == nil {
		t.Fatal("a flush to an impossible path succeeded")
	}
	// The way is cleared: the buffered line is written with a note of what
	// was dropped.
	os.Remove(blocker)
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	w.Close()
	got := read(t, p)
	if !strings.Contains(got, "22 bytes were dropped") || !strings.HasSuffix(got, "0123456789\n") {
		t.Fatalf("got %q", got)
	}
}

func TestConcurrentWritersKeepWholeLines(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.log")
	w := Open(p, Options{MaxSize: 1 << 20, FlushSize: 256})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 200 {
				w.WriteString("line of a writer\n")
			}
		})
	}
	wg.Wait()
	w.Close()
	lines := strings.Split(strings.TrimSuffix(read(t, p), "\n"), "\n")
	if len(lines) != 1600 {
		t.Fatalf("got %d lines", len(lines))
	}
	for _, l := range lines {
		if l != "line of a writer" {
			t.Fatalf("a torn line: %q", l)
		}
	}
}

func TestWritesAfterCloseAreIgnored(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.log")
	w := Open(p, Options{})
	w.WriteString("kept\n")
	w.Close()
	w.WriteString("ignored\n")
	w.Flush()
	if got := read(t, p); got != "kept\n" {
		t.Fatalf("got %q", got)
	}
}
