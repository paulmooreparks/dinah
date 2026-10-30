package drafts

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func local() Origin {
	return Origin{Kind: "local", WorkspaceID: "w1", SourceID: "s1", Path: "notes/a.md"}
}

func TestCreateGetUpdate(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	base := Baseline{Text: "# A\n", Revision: Revision("# A\n"), Title: "A"}
	d, err := s.Create(local(), base, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Text != base.Text || d.Title != "A" || d.Revision != Revision(base.Text) || d.Changed() {
		t.Fatalf("new draft %+v", d)
	}
	got, err := s.Get(d.ID)
	if err != nil || got.Text != d.Text || !reflect.DeepEqual(got.Baseline, base) {
		t.Fatalf("get %+v %v", got, err)
	}
	up, err := s.Update(d.ID, d.Revision, "# A\n\nMore.\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if up.Revision == d.Revision || !up.Changed() || up.Baseline.Text != base.Text {
		t.Fatalf("updated %+v", up)
	}
	// A change against the old revision is refused and changes nothing.
	if _, err := s.Update(d.ID, d.Revision, "# Lost\n", nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale update: %v", err)
	}
	if again, _ := s.Get(d.ID); again.Text != up.Text {
		t.Fatalf("a refused update changed the draft: %q", again.Text)
	}
}

// The sweep forgets old drafts that hold nothing of the person's, and keeps
// recent ones, changed ones, ones with suggestions, new documents and the
// ones it is told to keep.
func TestSweep(t *testing.T) {
	s, _ := Open(t.TempDir())
	old := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return old }
	base := Baseline{Text: "# A\n", Revision: Revision("# A\n"), Title: "A"}
	create := func(path string) Draft {
		d, err := s.Create(Origin{Kind: "local", Path: path}, base, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	plain, changed, suggested, kept := create("plain.md"), create("changed.md"), create("suggested.md"), create("kept.md")
	s.Update(changed.ID, changed.Revision, "# A\n\nMine.\n", nil)
	s.Propose(suggested.ID, "# A\n", "# A, better\n", Proposal{Author: "test"})
	text := "# New\n"
	fresh, _ := s.Create(Origin{Kind: OriginNew}, Baseline{}, "New", &text)
	nothing := ""
	empty, _ := s.Create(Origin{Kind: OriginNew}, Baseline{}, "Untitled", &nothing)
	s.now = func() time.Time { return old.AddDate(0, 2, 0) }
	recent := create("recent.md")
	gone, err := s.Sweep(old.AddDate(0, 1, 0), func(d Draft) bool { return d.ID == kept.ID })
	if err != nil || gone != 2 {
		t.Fatalf("swept %d %v", gone, err)
	}
	if _, err := s.Get(plain.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("an old untouched draft was kept")
	}
	if _, err := s.Get(empty.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("an old new document with nothing in it was kept")
	}
	for _, d := range []Draft{changed, suggested, kept, fresh, recent} {
		if _, err := s.Get(d.ID); err != nil {
			t.Fatalf("%s was swept: %v", d.Origin.Path, err)
		}
	}
}

func TestGivenTextAndTitle(t *testing.T) {
	s, _ := Open(t.TempDir())
	text := "# Design Note\n\n## Problem\n"
	d, err := s.Create(Origin{Kind: "new", WorkspaceID: "w1"}, Baseline{}, "Note 12", &text)
	if err != nil {
		t.Fatal(err)
	}
	if d.Text != text || d.Title != "Note 12" || !d.Changed() {
		t.Fatalf("%+v", d)
	}
	title := "Note 12, Error Recovery"
	up, err := s.Update(d.ID, d.Revision, d.Text, &title)
	if err != nil || up.Title != title {
		t.Fatalf("%+v %v", up, err)
	}
}

func TestRebaseAndDiscard(t *testing.T) {
	s, _ := Open(t.TempDir())
	d, _ := s.Create(Origin{Kind: "new", WorkspaceID: "w1"}, Baseline{}, "Doc", nil)
	d, _ = s.Update(d.ID, d.Revision, "Body\n", nil)
	published := Origin{Kind: "local-folder", WorkspaceID: "w1", SourceID: "s2", Path: "doc.md"}
	r, err := s.Rebase(d.ID, d.Revision, &published, Baseline{Text: "Body\n", Revision: "1", Storage: "<p>Body</p>", Title: "Doc"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Origin != published || r.Changed() || r.Text != "Body\n" {
		t.Fatalf("rebased %+v", r)
	}
	found, _ := s.FindByOrigin(published)
	if len(found) != 1 || found[0].ID != d.ID {
		t.Fatalf("find %+v", found)
	}
	if err := s.Discard(d.ID, "stale"); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale discard: %v", err)
	}
	if err := s.Discard(d.ID, r.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(d.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after discard: %v", err)
	}
}

// Drafts survive a new store opened on the same directory, as after a restart.
func TestRestartAndList(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	a, _ := s.Create(local(), Baseline{Text: "a"}, "", nil)
	time.Sleep(5 * time.Millisecond)
	b, _ := s.Create(Origin{Kind: "new", WorkspaceID: "w1"}, Baseline{}, "B", nil)
	// A damaged file and a stray file do not hide the others.
	os.WriteFile(filepath.Join(dir, "00000000000000000000000000000000.json"), []byte("{"), 0o600)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600)
	s2, _ := Open(dir)
	all, err := s2.List()
	if err != nil || len(all) != 2 || all[0].ID != b.ID || all[1].ID != a.ID {
		t.Fatalf("list %+v %v", all, err)
	}
}

func TestRefusals(t *testing.T) {
	s, _ := Open(t.TempDir())
	if _, err := s.Get("../../etc/passwd"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a path as an ID: %v", err)
	}
	if _, err := s.Create(Origin{}, Baseline{}, "", nil); err == nil {
		t.Error("an origin without a kind was accepted")
	}
	big := string(make([]byte, MaxText+1))
	if _, err := s.Create(local(), Baseline{}, "", &big); err == nil {
		t.Error("an oversized draft was accepted")
	}
	d, _ := s.Create(local(), Baseline{}, "", nil)
	if _, err := s.Update(d.ID, d.Revision, "a\x00b", nil); err == nil {
		t.Error("a NUL character was accepted")
	}
}

// Concurrent writers naming the same revision: exactly one wins.
func TestConcurrentUpdates(t *testing.T) {
	s, _ := Open(t.TempDir())
	d, _ := s.Create(local(), Baseline{Text: "start"}, "", nil)
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins, conflicts := 0, 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.Update(d.ID, d.Revision, string(rune('a'+i)), nil)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errors.Is(err, ErrConflict):
				conflicts++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 || conflicts != 7 {
		t.Fatalf("%d wins and %d conflicts", wins, conflicts)
	}
}

// A lock left by a writer that stopped is taken over once it is stale.
func TestStaleLock(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	d, _ := s.Create(local(), Baseline{Text: "x"}, "", nil)
	lock := filepath.Join(dir, d.ID+".lock")
	os.WriteFile(lock, nil, 0o600)
	old := time.Now().Add(-time.Minute)
	os.Chtimes(lock, old, old)
	if _, err := s.Update(d.ID, d.Revision, "y", nil); err != nil {
		t.Fatalf("a stale lock blocked the update: %v", err)
	}
	if _, err := os.Stat(lock); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the lock remained after the update")
	}
	// A fresh lock held by someone else makes a writer give up in time.
	os.WriteFile(lock, nil, 0o600)
	s.lockWait = 50 * time.Millisecond
	cur, _ := s.Get(d.ID)
	if _, err := s.Update(d.ID, cur.Revision, "z", nil); err == nil {
		t.Fatal("an update went through a held lock")
	}
}
