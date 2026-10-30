package sessionhost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/providers"
)

func TestJournalPersistenceAndPathPolicy(t *testing.T) {
	dir := t.TempDir()
	path := func(id string) string { return JournalPath(dir, id, folderOf(id)) }
	var j Journal
	if got := j.Times("missing", path); got == nil || len(got) != 0 {
		t.Fatalf("missing journal: %#v", got)
	}
	id := "codex:thread/a"
	j.Record(id, "you", path)
	f, err := os.OpenFile(path(id), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("invalid json\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	j.Record(id, "agent", path)
	entries := j.Times(id, path)
	if len(entries) != 2 || entries[0].Kind != "you" || entries[1].Kind != "agent" || entries[0].At.IsZero() || entries[1].At.Before(entries[0].At) {
		t.Fatalf("entries: %#v", entries)
	}
	if filepath.Dir(path(id)) != filepath.Join(dir, "journal", "codex") {
		t.Fatal(path(id))
	}
	if got := JournalPath(dir, "other:a/b", ""); got != filepath.Join(dir, "journal", "other_a_b.jsonl") {
		t.Fatal(got)
	}
	j.Forget(id, path)
	if got := j.Times(id, path); got == nil || len(got) != 0 {
		t.Fatalf("forgotten journal: %#v", got)
	}
}

// folderOf is the journal folder of the provider an ID belongs to, by its
// prefix, as the window reads it from the table.
func folderOf(id string) string {
	for _, row := range providers.Registry {
		if row.SessionPrefix != "" && strings.HasPrefix(id, row.SessionPrefix) {
			return row.JournalFolder
		}
	}
	return ""
}
