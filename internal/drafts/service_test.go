package drafts

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"studio/internal/resource"
	"studio/internal/resource/localfs"
)

// folderProviders resolves every origin to one local folder.
type folderProviders struct{ folder localfs.Folder }

func (p folderProviders) Provider(context.Context, Origin) (any, error) { return p.folder, nil }

// readOnly offers snapshots but no writing.
type readOnly struct{}

func (readOnly) Snapshot(context.Context, string) (resource.Snapshot, error) {
	return resource.Snapshot{Text: "remote\n", Revision: "7", Title: "Page", Native: "<p>remote</p>"}, nil
}

// publishing writes and publishes, as a page on a wiki does.
type publishing struct{ readOnly }

func (publishing) WriteDocument(_ context.Context, _ string, base resource.Snapshot, edit resource.Edit) (resource.Written, error) {
	return resource.Written{Snapshot: resource.Snapshot{Text: edit.Text, Revision: "8", Title: edit.Title, Native: "<p>new</p>"}, Notes: []string{"note"}}, nil
}
func (publishing) Publishes() bool { return true }

type publishingProviders struct{}

func (publishingProviders) Provider(context.Context, Origin) (any, error) { return publishing{}, nil }

func TestPublishingSource(t *testing.T) {
	store, _ := Open(t.TempDir())
	svc := &Service{Store: store, Providers: publishingProviders{}}
	ctx := context.Background()
	d, _ := svc.Open(ctx, Origin{Kind: "wiki-page", Path: "12345"})
	d, _ = store.Update(d.ID, d.Revision, "edited\n", nil)
	v := svc.View(ctx, d)
	if !slices.Contains(v.Actions, ActionPublish) || slices.Contains(v.Actions, ActionSave) {
		t.Fatalf("a publishing source offers %v", v.Actions)
	}
	saved, notes, err := svc.Save(ctx, d.ID, d.Revision)
	if err != nil || saved.Baseline.Revision != "8" || saved.Baseline.Storage != "<p>new</p>" || saved.Changed() || len(notes) != 1 {
		t.Fatalf("published %+v %v %v", saved.Baseline, notes, err)
	}
}

type readOnlyProviders struct{}

func (readOnlyProviders) Provider(context.Context, Origin) (any, error) { return readOnly{}, nil }

func TestOpenEditSave(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\n"), 0o644)
	store, _ := Open(t.TempDir())
	svc := &Service{Store: store, Providers: folderProviders{localfs.Folder{Root: dir}}}
	ctx := context.Background()
	origin := Origin{Kind: "local-folder", WorkspaceID: "w", SourceID: "s", Path: "a.md"}

	d, err := svc.Open(ctx, origin)
	if err != nil || d.Text != "# A\n" || d.Title != "a.md" || d.Baseline.Revision != resource.ContentRevision("# A\n") {
		t.Fatalf("open %+v %v", d, err)
	}
	if v := svc.View(ctx, d); v.Changed || slices.Contains(v.Actions, ActionSave) {
		t.Fatalf("an unchanged draft offers save: %+v", v.Actions)
	}
	// Reopening continues the same draft.
	d, _ = store.Update(d.ID, d.Revision, "# A\n\nMore.\n", nil)
	again, _ := svc.Open(ctx, origin)
	if again.ID != d.ID || again.Text != d.Text {
		t.Fatalf("reopen started over: %+v", again)
	}
	if v := svc.View(ctx, d); !slices.Contains(v.Actions, ActionSave) {
		t.Fatalf("a changed draft does not offer save: %+v", v.Actions)
	}
	saved, _, err := svc.Save(ctx, d.ID, d.Revision)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "a.md"))
	if string(data) != "# A\n\nMore.\n" || saved.Changed() || saved.Baseline.Revision != resource.ContentRevision(string(data)) {
		t.Fatalf("saved %q %+v", data, saved.Baseline)
	}
}

// Opens of one resource at the same moment, as from two windows, or from a
// window and an agent, find one draft between them.
func TestOpensAtOnceFindOneDraft(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\n"), 0o644)
	store, _ := Open(t.TempDir())
	origin := Origin{Kind: "local-folder", WorkspaceID: "w", SourceID: "s", Path: "a.md"}
	ids := make(chan string, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each caller builds its own service, as the desktop does.
			svc := &Service{Store: store, Providers: folderProviders{localfs.Folder{Root: dir}}}
			d, err := svc.Open(context.Background(), origin)
			if err != nil {
				t.Error(err)
				return
			}
			ids <- d.ID
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		seen[id] = true
	}
	all, _ := store.FindByOrigin(origin)
	if len(seen) != 1 || len(all) != 1 {
		t.Fatalf("one resource has %d drafts, and the opens saw %d", len(all), len(seen))
	}
}

// A reused draft that holds nothing of the person's is brought up to the
// file as it is now; one with changes or suggestions is left alone, and so
// is one whose file cannot be read.
func TestOpenRefreshesAnUntouchedDraft(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("one\n"), 0o644)
	store, _ := Open(t.TempDir())
	svc := &Service{Store: store, Providers: folderProviders{localfs.Folder{Root: dir}}}
	ctx := context.Background()
	origin := Origin{Kind: "local-folder", Path: "a.md"}
	d, _ := svc.Open(ctx, origin)
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("two\n"), 0o644)
	again, err := svc.Open(ctx, origin)
	if err != nil || again.ID != d.ID || again.Text != "two\n" || again.Baseline.Text != "two\n" || again.Changed() {
		t.Fatalf("an untouched draft was not refreshed: %+v %v", again, err)
	}
	// A suggestion is the person's to decide, so the draft stays.
	if _, _, err := store.Propose(again.ID, "two\n", "two, changed\n", Proposal{Author: "test"}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("three\n"), 0o644)
	if kept, _ := svc.Open(ctx, origin); kept.Text != "two\n" || len(kept.Suggestions) != 1 {
		t.Fatalf("a draft with a suggestion was refreshed: %+v", kept)
	}
	kept, _ := store.Get(again.ID)
	kept, _ = store.RejectGroup(kept.ID, kept.Suggestions[0].Group)
	// A change of the person's is kept too.
	edited, _ := store.Update(kept.ID, kept.Revision, "mine\n", nil)
	if after, _ := svc.Open(ctx, origin); after.Text != "mine\n" || after.Revision != edited.Revision {
		t.Fatalf("a changed draft was refreshed: %+v", after)
	}
	// A file that cannot be read leaves an untouched draft as it was.
	edited, _ = store.Update(edited.ID, edited.Revision, "three\n", nil)
	os.Remove(filepath.Join(dir, "a.md"))
	if gone, err := svc.Open(ctx, origin); err != nil || gone.ID != d.ID {
		t.Fatalf("an unreadable file failed the open: %+v %v", gone, err)
	}
}

func TestSaveRefusesOutsideChange(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("one\n"), 0o644)
	store, _ := Open(t.TempDir())
	svc := &Service{Store: store, Providers: folderProviders{localfs.Folder{Root: dir}}}
	ctx := context.Background()
	d, _ := svc.Open(ctx, Origin{Kind: "local-folder", Path: "a.md"})
	d, _ = store.Update(d.ID, d.Revision, "mine\n", nil)
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("theirs\n"), 0o644)
	if _, _, err := svc.Save(ctx, d.ID, d.Revision); !errors.Is(err, resource.ErrConflict) {
		t.Fatalf("an outside change was overwritten: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "a.md"))
	if string(data) != "theirs\n" {
		t.Fatalf("the file was changed: %q", data)
	}
	// The draft is kept, so nothing typed is lost.
	if kept, _ := store.Get(d.ID); kept.Text != "mine\n" {
		t.Fatalf("the draft was lost: %q", kept.Text)
	}
	// A stale draft revision is refused before anything is written.
	if _, _, err := svc.Save(ctx, d.ID, "stale"); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale draft revision: %v", err)
	}
}

func TestSaveAs(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "taken.md"), []byte("someone else's\n"), 0o644)
	store, _ := Open(t.TempDir())
	svc := &Service{Store: store, Providers: folderProviders{localfs.Folder{Root: dir}}}
	ctx := context.Background()
	text := "# Plan\n"
	d, _ := store.Create(Origin{Kind: OriginNew, WorkspaceID: "w"}, Baseline{}, "Plan", &text)
	if v := svc.View(ctx, d); !slices.Contains(v.Actions, ActionSaveAs) || slices.Contains(v.Actions, ActionSave) {
		t.Fatalf("a new document offers %v", v.Actions)
	}
	if _, _, err := svc.Save(ctx, d.ID, d.Revision); err == nil {
		t.Fatal("saved a new document without a place")
	}
	if _, _, err := svc.SaveAs(ctx, d.ID, d.Revision, Origin{Kind: "local-folder", WorkspaceID: "w", SourceID: "s", Path: "taken.md"}); !errors.Is(err, resource.ErrConflict) {
		t.Fatalf("replaced an existing file: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "taken.md")); string(data) != "someone else's\n" {
		t.Fatalf("the existing file changed: %q", data)
	}
	origin := Origin{Kind: "local-folder", WorkspaceID: "w", SourceID: "s", Path: "plan.md"}
	saved, _, err := svc.SaveAs(ctx, d.ID, d.Revision, origin)
	if err != nil || saved.Origin != origin || saved.Changed() {
		t.Fatalf("save as %+v %v", saved, err)
	}
	// A file's title is its name, so the draft is plan.md now, not Plan.
	if saved.Title != "plan.md" {
		t.Fatalf("the saved draft's title is %q", saved.Title)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "plan.md")); string(data) != text {
		t.Fatalf("written %q", data)
	}
	// The draft now belongs to its file, so opening the file continues it.
	if again, _ := svc.Open(ctx, origin); again.ID != d.ID {
		t.Fatal("opening the saved file started another draft")
	}
}

// withFiles is a page with attachments, as a page on a wiki has them.
type withFiles struct{ readOnly }

func (withFiles) Snapshot(context.Context, string) (resource.Snapshot, error) {
	return resource.Snapshot{Text: "# Page\n\n![](chart.png)\n\n![](d2-diagram-1.svg)\n\n![](my%20photo.png)\n", Revision: "7", Title: "Page"}, nil
}
func (withFiles) FileNames(context.Context, string) ([]string, error) {
	return []string{"chart.png", "d2-diagram-1.svg", "my photo.png", "unused.pdf", "broken.png"}, nil
}
func (withFiles) ReadFile(_ context.Context, _ string, name string) ([]byte, error) {
	return []byte("bytes of " + name), nil
}

// pageAndFolder resolves a wiki origin to a page with attachments and
// a local one to a folder.
type pageAndFolder struct{ folder localfs.Folder }

func (p pageAndFolder) Provider(_ context.Context, o Origin) (any, error) {
	if o.Kind == "wiki-page" {
		return withFiles{}, nil
	}
	return p.folder, nil
}

// A copy saved elsewhere takes the files its text refers to from beside the
// original, and leaves the others; a file already at the new place is kept.
func TestSaveAsCopiesTheFilesTheTextUses(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "chart.png"), []byte("already here"), 0o644)
	store, _ := Open(t.TempDir())
	svc := &Service{Store: store, Providers: pageAndFolder{localfs.Folder{Root: dir}}}
	ctx := context.Background()
	d, err := svc.Open(ctx, Origin{Kind: "wiki-page", Path: "12345"})
	if err != nil {
		t.Fatal(err)
	}
	_, notes, err := svc.SaveAs(ctx, d.ID, d.Revision, Origin{Kind: "local-folder", Path: "copy.md"})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"d2-diagram-1.svg": "bytes of d2-diagram-1.svg", "my photo.png": "bytes of my photo.png", "chart.png": "already here"} {
		if data, _ := os.ReadFile(filepath.Join(dir, name)); string(data) != want {
			t.Errorf("%s holds %q", name, data)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "unused.pdf")); err == nil {
		t.Error("a file the text does not use was copied")
	}
	if len(notes) != 1 {
		t.Errorf("the file already there was not reported: %v", notes)
	}
}

// A file added to a draft is kept with it under a name the document's folder
// does not already have, saved beside the document with the text when the
// text uses it, and then no longer kept; discarding the draft removes the
// files it still keeps.
func TestDraftFilesAreSavedWithTheText(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "chart.png"), []byte("the old chart"), 0o644)
	store, _ := Open(t.TempDir())
	svc := &Service{Store: store, Providers: folderProviders{localfs.Folder{Root: dir}}}
	ctx := context.Background()
	d, _ := svc.Open(ctx, Origin{Kind: "local-folder", Path: "a.md"})
	name, err := svc.AddFile(ctx, d.ID, `C:\pictures\chart.png`, []byte("the new chart"))
	if err != nil || name != "chart (2).png" {
		t.Fatalf("kept as %q %v", name, err)
	}
	unused, _ := svc.AddFile(ctx, d.ID, "unused.txt", []byte("never used"))
	if again, _ := svc.AddFile(ctx, d.ID, "chart.png", []byte("another")); again != "chart (3).png" {
		t.Fatalf("a second file of the name was kept as %q", again)
	}
	if data, err := store.ReadFile(d.ID, name); err != nil || string(data) != "the new chart" {
		t.Fatalf("read back %q %v", data, err)
	}
	d, _ = store.Update(d.ID, d.Revision, "# A\n\n![](<chart (2).png>)\n", nil)
	if _, _, err := svc.Save(ctx, d.ID, d.Revision); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "chart (2).png")); string(data) != "the new chart" {
		t.Fatalf("saved beside the document: %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "chart.png")); string(data) != "the old chart" {
		t.Fatalf("the file already there changed: %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, unused)); err == nil {
		t.Fatal("a file the text does not use was saved")
	}
	if names, _ := store.FileNames(d.ID); slices.Contains(names, name) || !slices.Contains(names, unused) {
		t.Fatalf("kept after the save: %v", names)
	}
	d, _ = store.Get(d.ID)
	if err := store.Discard(d.ID, d.Revision); err != nil {
		t.Fatal(err)
	}
	if names, _ := store.FileNames(d.ID); len(names) != 0 {
		t.Fatalf("kept after the draft was discarded: %v", names)
	}
}

func TestCompareAndMerge(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\n\nOne.\n\nTwo.\n\nThree.\n"), 0o644)
	store, _ := Open(t.TempDir())
	svc := &Service{Store: store, Providers: folderProviders{localfs.Folder{Root: dir}}}
	ctx := context.Background()
	d, _ := svc.Open(ctx, Origin{Kind: "local-folder", Path: "a.md"})
	d, _ = store.Update(d.ID, d.Revision, "# A\n\nOne, mine.\n\nTwo.\n\nThree.\n", nil)
	if c, err := svc.Compare(ctx, d.ID); err != nil || c.Moved || len(c.Conflicts) != 0 {
		t.Fatalf("an unmoved resource %+v %v", c, err)
	}
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\n\nOne.\n\nTwo.\n\nThree, theirs.\n"), 0o644)
	if _, _, err := svc.Save(ctx, d.ID, d.Revision); !errors.Is(err, resource.ErrConflict) {
		t.Fatalf("saved over a change made elsewhere: %v", err)
	}
	c, err := svc.Compare(ctx, d.ID)
	if err != nil || !c.Moved || len(c.Conflicts) != 0 || c.Merged != "# A\n\nOne, mine.\n\nTwo.\n\nThree, theirs.\n" {
		t.Fatalf("compare %+v %v", c, err)
	}
	merged, err := svc.Merge(ctx, d.ID, d.Revision)
	if err != nil || merged.Text != c.Merged || merged.Baseline.Text != "# A\n\nOne.\n\nTwo.\n\nThree, theirs.\n" {
		t.Fatalf("merge %+v %v", merged, err)
	}
	if _, _, err := svc.Save(ctx, merged.ID, merged.Revision); err != nil {
		t.Fatalf("save after merging: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "a.md")); string(data) != c.Merged {
		t.Fatalf("file %q", data)
	}

	// Both changing the same paragraph is refused, and nothing changes.
	saved, _ := store.Get(d.ID)
	saved, _ = store.Update(saved.ID, saved.Revision, "# A\n\nOne, mine again.\n\nTwo.\n\nThree, theirs.\n", nil)
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\n\nOne, theirs again.\n\nTwo.\n\nThree, theirs.\n"), 0o644)
	if _, err := svc.Merge(ctx, saved.ID, saved.Revision); !errors.Is(err, ErrMergeConflict) {
		t.Fatalf("a conflicting merge: %v", err)
	}
	if kept, _ := store.Get(saved.ID); kept.Text != saved.Text || !reflect.DeepEqual(kept.Baseline, saved.Baseline) {
		t.Fatal("a refused merge changed the draft")
	}
}

func TestReadOnlySource(t *testing.T) {
	store, _ := Open(t.TempDir())
	svc := &Service{Store: store, Providers: readOnlyProviders{}}
	ctx := context.Background()
	d, err := svc.Open(ctx, Origin{Kind: "wiki-page", Path: "12345"})
	if err != nil || d.Baseline.Storage != "<p>remote</p>" || d.Baseline.Revision != "7" || d.Title != "Page" {
		t.Fatalf("open %+v %v", d, err)
	}
	d, _ = store.Update(d.ID, d.Revision, "edited\n", nil)
	if v := svc.View(ctx, d); slices.Contains(v.Actions, ActionSave) {
		t.Fatalf("a source without writing offers save: %+v", v.Actions)
	}
	if _, _, err := svc.Save(ctx, d.ID, d.Revision); !errors.Is(err, resource.ErrUnsupported) {
		t.Fatalf("save on a read-only source: %v", err)
	}
	if _, err := svc.Open(ctx, Origin{Kind: OriginNew}); err == nil {
		t.Fatal("opened a new document")
	}
}

// A copy of a file saved elsewhere takes the pictures its text shows from
// wherever under the document's folder they are kept, at the same paths, and
// a note says which picture was not found.
func TestSaveAsCopiesThePicturesTheTextShowsByPath(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(from, "notes", "research", "screens", "set 1"), 0o755)
	os.MkdirAll(filepath.Join(from, "notes", "images"), 0o755)
	os.WriteFile(filepath.Join(from, "notes", "research", "screens", "set 1", "01.png"), []byte("one"), 0o644)
	os.WriteFile(filepath.Join(from, "notes", "research", "screens", "02.png"), []byte("two"), 0o644)
	os.WriteFile(filepath.Join(from, "notes", "research", "03.png"), []byte("three"), 0o644)
	os.WriteFile(filepath.Join(from, "notes", "images", "04.png"), []byte("four"), 0o644)
	os.WriteFile(filepath.Join(from, "notes", "research", "unused.png"), []byte("unused"), 0o644)
	os.MkdirAll(filepath.Join(from, "notes", "files"), 0o755)
	os.WriteFile(filepath.Join(from, "notes", "files", "plan.pdf"), []byte("pdf"), 0o644)
	os.WriteFile(filepath.Join(from, "notes", "files", "other.md"), []byte("# Other"), 0o644)
	text := "# Screens\n\n{{figure:\"research/screens/set 1/01.png\" alt=\"One\"}}\n\n![Two](research/screens/02.png)\n\n" +
		"<img src=\"research/03.png\" width=\"200\">\n\n![Four](04.png)\n\n{{figure:research/missing.png alt=\"Gone\"}}\n\n" +
		"```\n![In code](research/unused.png)\n```\n\nSee [the plan](files/plan.pdf), [another document](files/other.md) and [the site](https://example.com/x.pdf).\n"
	os.WriteFile(filepath.Join(from, "notes", "doc.md"), []byte(text), 0o644)
	store, _ := Open(t.TempDir())
	svc := &Service{Store: store, Providers: folders{"from": localfs.Folder{Root: from}, "to": localfs.Folder{Root: to}}}
	ctx := context.Background()
	d, err := svc.Open(ctx, Origin{Kind: "local-folder", SourceID: "from", Path: "notes/doc.md"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.SaveAs(ctx, d.ID, d.Revision, Origin{Kind: "local-folder", SourceID: "to", Path: "copy/doc.md"})
	if err == nil {
		t.Fatal("a copy was saved into a folder that does not exist")
	}
	os.MkdirAll(filepath.Join(to, "copy"), 0o755)
	_, notes, err := svc.SaveAs(ctx, d.ID, d.Revision, Origin{Kind: "local-folder", SourceID: "to", Path: "copy/doc.md"})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"research/screens/set 1/01.png": "one", "research/screens/02.png": "two", "research/03.png": "three", "04.png": "four", "files/plan.pdf": "pdf"} {
		if data, _ := os.ReadFile(filepath.Join(to, "copy", filepath.FromSlash(name))); string(data) != want {
			t.Errorf("%s holds %q", name, data)
		}
	}
	if _, err := os.Stat(filepath.Join(to, "copy", "files", "other.md")); err == nil {
		t.Error("a document the text links to was copied as a file")
	}
	if _, err := os.Stat(filepath.Join(to, "copy", "research", "unused.png")); err == nil {
		t.Error("a picture shown only inside a code block was copied")
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "research/missing.png") {
		t.Errorf("notes %v", notes)
	}
}

// A copy saved into the folder of the original finds its pictures already
// there, which is no fault to report.
func TestSaveAsBesideTheOriginalReportsNoPictureAlreadyThere(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "pictures"), 0o755)
	os.WriteFile(filepath.Join(dir, "pictures", "a.png"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "doc.md"), []byte("![A](pictures/a.png)\n"), 0o644)
	store, _ := Open(t.TempDir())
	svc := &Service{Store: store, Providers: folders{"": localfs.Folder{Root: dir}}}
	ctx := context.Background()
	d, err := svc.Open(ctx, Origin{Kind: "local-folder", Path: "doc.md"})
	if err != nil {
		t.Fatal(err)
	}
	if _, notes, err := svc.SaveAs(ctx, d.ID, d.Revision, Origin{Kind: "local-folder", Path: "doc 2.md"}); err != nil || len(notes) != 0 {
		t.Fatalf("notes %v, %v", notes, err)
	}
}

// folders gives the local folder of each source ID.
type folders map[string]localfs.Folder

func (f folders) Provider(_ context.Context, o Origin) (any, error) { return f[o.SourceID], nil }
