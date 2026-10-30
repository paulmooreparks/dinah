package recent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"studio/internal/workspaces"
)

func ref(ws, src, path string) *workspaces.Ref {
	return &workspaces.Ref{WorkspaceID: ws, SourceID: src, Path: path}
}

func TestRecordPutsNewestFirstAndMovesARepeat(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), FileName)}
	if got, err := s.List(); err != nil || len(got) != 0 {
		t.Fatalf("an empty list: %v %v", got, err)
	}
	at := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	for i, title := range []string{"One", "Two", "Three"} {
		if err := s.Record(Entry{Ref: ref("ws_1", "src", title+".md"), Kind: "local-folder", Title: title, Opened: at.Add(time.Duration(i) * time.Minute)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Record(Entry{Ref: ref("ws_1", "src", "One.md"), Kind: "local-folder", Title: "One, Renamed", Opened: at.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	titles := []string{}
	for _, e := range got {
		titles = append(titles, e.Title)
	}
	if fmt.Sprint(titles) != "[One, Renamed Three Two]" {
		t.Fatalf("order %v", titles)
	}
}

// A renamed file's entry points at the new name and keeps its place; an
// entry already at the new name is dropped, so the file is listed once.
func TestMoveKeepsTheEntrysPlace(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), FileName)}
	if err := s.Move(*ref("ws_1", "src", "a.md"), *ref("ws_1", "src", "b.md"), "b.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Path); !os.IsNotExist(err) {
		t.Fatal("a move with no list wrote one")
	}
	for _, name := range []string{"b.md", "c.md", "a.md"} {
		if err := s.Record(Entry{Ref: ref("ws_1", "src", name), Kind: "local-folder", Title: name}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Move(*ref("ws_1", "src", "c.md"), *ref("ws_1", "src", "b.md"), "b.md"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.List()
	paths := []string{}
	for _, e := range got {
		paths = append(paths, e.Ref.Path+"="+e.Title)
	}
	if fmt.Sprint(paths) != "[a.md=a.md b.md=b.md]" {
		t.Fatalf("entries %v", paths)
	}
}

func TestRecordKeepsFifty(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), FileName)}
	for i := range Kept + 5 {
		if err := s.Record(Entry{Ref: ref("ws_1", "src", fmt.Sprintf("%d.md", i)), Kind: "local-folder", Title: fmt.Sprint(i)}); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := s.List()
	if len(got) != Kept || got[0].Title != fmt.Sprint(Kept+4) || got[Kept-1].Title != "5" {
		t.Fatalf("kept %d, first %q, last %q", len(got), got[0].Title, got[len(got)-1].Title)
	}
}

func TestComputerFilesAreOneEntryPerPath(t *testing.T) {
	dir := t.TempDir()
	s := Store{Path: filepath.Join(dir, FileName)}
	path := filepath.Join(dir, "notes.md")
	if err := s.Record(Entry{File: path, WorkspaceID: "ws_1", Kind: KindComputerFile, Title: "notes.md"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(Entry{File: path, WorkspaceID: "ws_2", Kind: KindComputerFile, Title: "notes.md"}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.List()
	if len(got) != 1 || got[0].WorkspaceID != "ws_2" {
		t.Fatalf("got %+v", got)
	}
}

func TestRecordRefusesIncompleteEntries(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), FileName)}
	for _, e := range []Entry{
		{Kind: "local-folder", Title: "Nothing"},
		{Ref: ref("ws", "src", "a.md"), File: `C:\a.md`, Title: "Both"},
		{File: "relative.md", Kind: KindComputerFile, Title: "Relative"},
		{Ref: ref("ws", "src", "a.md"), Kind: "local-folder", Title: " "},
	} {
		if err := s.Record(e); err == nil {
			t.Fatalf("accepted %+v", e)
		}
	}
	if _, err := os.Stat(s.Path); !os.IsNotExist(err) {
		t.Fatal("a refused entry wrote the file")
	}
}

func TestANewerFileIsNotReplaced(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), FileName)}
	data, _ := json.Marshal(map[string]any{"version": 2, "entries": []any{}})
	if err := os.WriteFile(s.Path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.List(); err == nil {
		t.Fatal("a newer file was read")
	}
	if err := s.Record(Entry{Ref: ref("ws", "src", "a.md"), Kind: "local-folder", Title: "A"}); err == nil {
		t.Fatal("a newer file was replaced")
	}
}

// A damaged list reads as empty and is moved aside by the next open, so
// Recent recovers without the person repairing a file.
func TestADamagedListIsMovedAside(t *testing.T) {
	dir := t.TempDir()
	s := Store{Path: filepath.Join(dir, FileName)}
	for _, damage := range []string{`not json`, `{"entries":[]}`} {
		if err := os.WriteFile(s.Path, []byte(damage), 0o600); err != nil {
			t.Fatal(err)
		}
		if got, err := s.List(); err != nil || len(got) != 0 {
			t.Fatalf("%s: list %v %v", damage, got, err)
		}
		if err := s.Record(Entry{Ref: ref("ws", "src", "a.md"), Kind: "local-folder", Title: "A"}); err != nil {
			t.Fatalf("%s: %v", damage, err)
		}
		if got, _ := s.List(); len(got) != 1 {
			t.Fatalf("%s: after recording %v", damage, got)
		}
		os.Remove(s.Path)
	}
	aside, _ := filepath.Glob(s.Path + ".damaged-*")
	if len(aside) == 0 {
		t.Fatal("the damaged list was not kept aside")
	}
}

func TestAvailableLeavesOutWhatIsGone(t *testing.T) {
	dir := t.TempDir()
	kept := filepath.Join(dir, "kept.md")
	if err := os.WriteFile(kept, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	records := []workspaces.Record{{ID: "ws_1", Sources: []workspaces.Source{{ID: "src"}}}}
	entries := []Entry{
		{Ref: ref("ws_1", "src", "a.md"), Title: "Kept"},
		{Ref: ref("ws_1", "gone", "a.md"), Title: "Source gone"},
		{Ref: ref("ws_9", "src", "a.md"), Title: "Workspace gone"},
		{Ref: ref("", workspaces.HomeSourceID, "n.md"), Title: "Home"},
		{File: kept, WorkspaceID: "ws_1", Title: "File kept"},
		{File: filepath.Join(dir, "gone.md"), Title: "File gone"},
		{File: kept, WorkspaceID: "ws_9", Title: "File's workspace gone"},
	}
	titles := []string{}
	for _, e := range Available(entries, records) {
		titles = append(titles, e.Title)
	}
	if fmt.Sprint(titles) != "[Kept Home File kept]" {
		t.Fatalf("available %v", titles)
	}
}
