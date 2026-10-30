package broker

import (
	"os"
	"testing"
	"time"

	"studio/internal/layout"
)

// A summary taken once is kept with its files' signature, so the next
// program reads only the records that changed since.
func TestIndexKeepsSummariesBetweenStarts(t *testing.T) {
	state, ws := t.TempDir(), t.TempDir()
	writeElsewhere(t, state, Invocation{ID: "asks", Workspace: ws, OwnerKind: "chat", OwnerID: "c1", Action: quickRef, Status: "waiting", Deadline: time.Now().Add(time.Hour), Created: time.Now()})
	b := &Broker{State: state}
	if list, err := b.Attention(all); err != nil || len(list) != 1 || list[0].ID != "asks" {
		t.Fatalf("first scan: %+v %v", list, err)
	}
	if _, err := os.Stat(b.indexCacheFile()); err != nil {
		t.Fatalf("the scan left no cache: %v", err)
	}
	// The record is made unreadable with the size and time it had, and the
	// index is forgotten as a closed program forgets it: the next program's
	// scan takes the summary from the cache without reading the record.
	store := layout.Layout{State: state}.Runs(b.StoreRoot(ws))
	path := store.SlotPath("asks", 0)
	info, err := os.Stat(path)
	if err != nil {
		path = store.SlotPath("asks", 1)
		if info, err = os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, make([]byte, info.Size()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	indexes.Delete(stateKey(state))
	again := &Broker{State: state}
	ix, err := again.ready()
	if err != nil {
		t.Fatal(err)
	}
	if entries := ix.snapshot(); len(entries) != 1 || entries[0].ID != "asks" || entries[0].Status != "waiting" {
		t.Fatalf("the cached summary was not used: %+v", entries)
	}
	// A record whose files changed is read again; this one cannot be.
	later := info.ModTime().Add(2 * time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	indexes.Delete(stateKey(state))
	third := &Broker{State: state}
	ix, err = third.ready()
	if err != nil {
		t.Fatal(err)
	}
	if entries := ix.snapshot(); len(entries) != 0 {
		t.Fatalf("a changed record was taken from the cache: %+v", entries)
	}
}
