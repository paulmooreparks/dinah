package httphead

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"dinah/internal/durable"
)

// lockedBuffer is a buffer two goroutines may write, since a notice arrives
// on whichever goroutine's act is waiting.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write appends to the buffer under its lock.
func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String answers what was written.
func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestAWaitReachesStandardErrorAndTheCommandLog asserts that the handler
// installs its wait sink, and that a notice through it is printed to the
// process's standard error prefixed dinah: and drawn in the command log the
// pages show, with the path written relative to the workbench.
func TestAWaitReachesStandardErrorAndTheCommandLog(t *testing.T) {
	stderr := &lockedBuffer{}
	var sink func(durable.Wait)
	f := newFixture(t, func(cfg *Config) {
		cfg.Notify = func(line string) { stderr.Write([]byte(line + "\n")) }
		cfg.InstallWaiting = func(installed func(durable.Wait)) { sink = installed }
	})
	if sink == nil {
		t.Fatal("the handler installed no wait sink")
	}
	sink(durable.Wait{
		Op:      "append",
		Path:    filepath.Join(f.root, "cards", "0123456789ab", "journal.ndjson"),
		Last:    errors.New("the file is in use"),
		Elapsed: 5 * time.Second,
		Notice:  1,
	})
	printed := stderr.String()
	if !strings.HasPrefix(printed, "dinah: waiting for cards/0123456789ab/journal.ndjson") {
		t.Errorf("standard error carries %q", printed)
	}
	page := f.page("/commands").body
	if !strings.Contains(page, "waiting for cards/0123456789ab/journal.ndjson") {
		t.Errorf("the command log page does not draw the notice:\n%s", page)
	}
}
