package resident

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// MirrorDiff answers how a snapshot differs from a fresh build of the tree
// below root, one line per difference: directories held by one and not the
// other, entries by name and type, and every held file's size and bytes.
// Empty means the snapshot mirrors the disk.
func MirrorDiff(s *Snapshot, root string) []string {
	fresh := buildAll(root)
	var diffs []string
	keys := map[string]bool{}
	for key := range s.dirs {
		keys[key] = true
	}
	for key := range fresh {
		keys[key] = true
	}
	sorted := make([]string, 0, len(keys))
	for key := range keys {
		sorted = append(sorted, key)
	}
	sort.Strings(sorted)
	for _, key := range sorted {
		held, heldOK := s.dirs[key]
		want, wantOK := fresh[key]
		switch {
		case !heldOK:
			diffs = append(diffs, fmt.Sprintf("%s: on disk, not held", filepath.ToSlash(key)))
			continue
		case !wantOK:
			diffs = append(diffs, fmt.Sprintf("%s: held, not on disk", filepath.ToSlash(key)))
			continue
		}
		if len(held.entries) != len(want.entries) {
			diffs = append(diffs, fmt.Sprintf("%s: %d entries held, %d on disk", filepath.ToSlash(key), len(held.entries), len(want.entries)))
			continue
		}
		for i := range want.entries {
			h, w := held.entries[i], want.entries[i]
			if h.Name() != w.Name() || h.Type() != w.Type() {
				diffs = append(diffs, fmt.Sprintf("%s: entry %s %v held, %s %v on disk", filepath.ToSlash(key), h.Name(), h.Type(), w.Name(), w.Type()))
				continue
			}
			wantFile, isFile := want.files[w.Name()]
			if !isFile {
				continue
			}
			heldFile, ok := held.files[h.Name()]
			if !ok {
				diffs = append(diffs, fmt.Sprintf("%s/%s: file not held", filepath.ToSlash(key), h.Name()))
				continue
			}
			hInfo, _ := h.Info()
			if !bytes.Equal(heldFile.data, wantFile.data) || heldFile.info.Size() != wantFile.info.Size() || hInfo == nil || hInfo.Size() != wantFile.info.Size() {
				diffs = append(diffs, fmt.Sprintf("%s/%s: bytes or size differ", filepath.ToSlash(key), h.Name()))
			}
		}
	}
	return diffs
}

// EarliestExpiry is the snapshot's earliest claim expiry.
func (s *Snapshot) EarliestExpiry() time.Time {
	return s.earliestExpiry
}

// SetLongForm replaces the resolver Open gives each workbench, and answers a
// function that puts the old one back.
func SetLongForm(resolve func(string) (string, error)) func() {
	old := longFormOf
	longFormOf = resolve
	return func() { longFormOf = old }
}

// QueuedRequests answers how many Current and Settle requests wait in the
// queue, not yet taken by a pass.
func QueuedRequests(w *Workbench) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.queue)
}

// Quiesce asks the applier to handle any signal it has pending and answers
// once it has, so every signal sent before the call has been handled when it
// returns. It must not be called while a hook holds the applier.
func Quiesce(w *Workbench) {
	reply := make(chan struct{})
	w.probe <- reply
	<-reply
}

// Parked reports whether the watch is down.
func Parked(w *Workbench) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state == parked
}

// Warm is the first request after Open: one Current, which builds the
// snapshot inside it. It fails the test unless that answers a snapshot, and
// then asserts that Ready is closed.
func Warm(t testing.TB, w *Workbench) *Snapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pick := w.Current(ctx, time.Now())
	if pick.Snapshot == nil {
		t.Fatalf("the first request answered no snapshot (lapsing %v)", pick.Lapsing)
	}
	select {
	case <-w.Ready():
	default:
		t.Fatal("the first request answered a snapshot and Ready is not closed")
	}
	return pick.Snapshot
}
