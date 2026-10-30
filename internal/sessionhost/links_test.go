package sessionhost

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConversationLinksCacheInvalidationAndCopies(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "conversations")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(folder, "child.json")
	stamp := time.Unix(1700000000, 0)
	write := func(body string, at time.Time) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"sessionId":"child","parentSessionId":"first"}`, stamp)
	links, records := ConversationLinks(dir)
	if links["first"] != "child" {
		t.Fatal("link missing")
	}
	links["first"] = "changed"
	records["child"].ParentSessionID = "changed"
	delete(records, "child")
	links, records = ConversationLinks(dir)
	if links["first"] != "child" || records["child"].ParentSessionID != "first" {
		t.Fatal("returned values shared cache state")
	}
	write(`{"sessionId":"child","parentSessionId":"other"}`, stamp)
	links, _ = ConversationLinks(dir)
	if links["first"] != "child" {
		t.Fatal("unchanged size/mtime did not retain cached head")
	}
	write(`{"sessionId":"child","parentSessionId":"other"}`, stamp.Add(time.Minute))
	links, _ = ConversationLinks(dir)
	if links["other"] != "child" || links["first"] != "" {
		t.Fatal("mtime change did not invalidate")
	}
	write(`{"sessionId":"child","parentSessionId":"longer-parent"}`, stamp.Add(time.Minute))
	links, _ = ConversationLinks(dir)
	if links["longer-parent"] != "child" {
		t.Fatal("size change did not invalidate")
	}
	write(`invalid`, stamp.Add(2*time.Minute))
	links, records = ConversationLinks(dir)
	if len(links) != 0 || len(records) != 0 {
		t.Fatal("invalid file retained old head")
	}
	write(`{"sessionId":"child","parentSessionId":"first"}`, stamp.Add(3*time.Minute))
	ConversationLinks(dir)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	links, records = ConversationLinks(dir)
	if len(links) != 0 || len(records) != 0 {
		t.Fatal("missing file retained old head")
	}
	write(`{"sessionId":"child","parentSessionId":"first"}`, stamp.Add(3*time.Minute))
	ConversationLinks(dir)
	other := t.TempDir()
	links, records = ConversationLinks(other)
	if len(links) != 0 || len(records) != 0 {
		t.Fatal("directory switch retained prior heads")
	}
	links, _ = ConversationLinks(dir)
	if links["first"] != "child" {
		t.Fatal("returning to a directory did not reload")
	}
}
