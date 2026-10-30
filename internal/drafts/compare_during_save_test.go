package drafts

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"studio/internal/resource"
	"studio/internal/resource/localfs"
)

// pausingFolder lets a test look at the draft in the moment between the file
// being written and the draft's baseline being moved to what was written.
type pausingFolder struct {
	localfs.Folder
	written chan struct{}
	release chan struct{}
}

func (p pausingFolder) WriteDocument(ctx context.Context, path string, base resource.Snapshot, edit resource.Edit) (resource.Written, error) {
	w, err := p.Folder.WriteDocument(ctx, path, base, edit)
	close(p.written)
	<-p.release
	return w, err
}

type pausingProviders struct{ folder pausingFolder }

func (p pausingProviders) Provider(context.Context, Origin) (any, error) { return p.folder, nil }

// The Editor asks whether the file changed elsewhere while its own save is in
// progress (it does so on a timer, and whenever the draft changes). A save's own
// write must never be reported as somebody else's change: that shows the person a
// notice about a change that nobody made, and it goes away at the next check.
func TestCompareDuringOwnSaveIsNotAnOutsideChange(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("one\n"), 0o644)
	store, _ := Open(t.TempDir())
	folder := pausingFolder{Folder: localfs.Folder{Root: dir}, written: make(chan struct{}), release: make(chan struct{})}
	svc := &Service{Store: store, Providers: pausingProviders{folder}}
	ctx := context.Background()
	d, err := svc.Open(ctx, Origin{Kind: "local-folder", Path: "a.md"})
	if err != nil {
		t.Fatal(err)
	}
	d, _ = store.Update(d.ID, d.Revision, "mine\n", nil)

	saved := make(chan error, 1)
	go func() { _, _, err := svc.Save(ctx, d.ID, d.Revision); saved <- err }()
	<-folder.written // the file holds the draft's text; the baseline has not caught up

	// A comparison asked for now waits for the save to finish, and so sees the
	// file and the baseline together.
	type answer struct {
		c   Comparison
		err error
	}
	compared := make(chan answer, 1)
	go func() { c, err := svc.Compare(ctx, d.ID); compared <- answer{c, err} }()
	var a answer
	select {
	case a = <-compared:
		t.Errorf("a comparison was answered in the middle of the save: moved %v (file %q)", a.c.Moved, a.c.Revision)
	case <-time.After(200 * time.Millisecond):
	}
	close(folder.release)
	if err := <-saved; err != nil {
		t.Fatal(err)
	}
	if a.c.Revision == "" && a.err == nil {
		a = <-compared
	}
	if a.err != nil || a.c.Moved {
		t.Fatalf("the draft's own write was reported as a change made elsewhere: %+v %v", a.c.Moved, a.err)
	}
	if after, err := svc.Compare(ctx, d.ID); err != nil || after.Moved {
		t.Fatalf("after the save finished the draft still looks changed elsewhere: %+v %v", after.Moved, err)
	}
}
