package drafts

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// pages is a source like a wiki: a page can be given by its own name or by
// the names of the pages down to it, and is the same page by its last name.
type pages struct{ readOnly }

func (pages) Identity(path string) string { return path[strings.LastIndex(path, "/")+1:] }

type pagesProviders struct{}

func (pagesProviders) Provider(context.Context, Origin) (any, error) { return pages{}, nil }

// One resource asked for by two of its paths has one draft, so that what an
// agent proposes by one path is in the document a person has open by the
// other (Paul, 13 November 2026).
func TestTwoPathsToOneResourceHaveOneDraft(t *testing.T) {
	store, _ := Open(t.TempDir())
	svc := &Service{Store: store, Providers: pagesProviders{}}
	ctx := context.Background()
	long := Origin{Kind: "wiki-page", WorkspaceID: "w", SourceID: "s", Path: "10/11/12"}
	short := Origin{Kind: "wiki-page", WorkspaceID: "w", SourceID: "s", Path: "12"}

	first, err := svc.Open(ctx, long)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Open(ctx, short)
	if err != nil || second.ID != first.ID {
		t.Fatalf("the short path opened %s, the long one %s (%v)", second.ID, first.ID, err)
	}
	if second.Origin != long {
		t.Fatalf("the draft's path changed: %+v", second.Origin)
	}
	// Another page, another source and another workspace are other resources.
	for _, other := range []Origin{
		{Kind: "wiki-page", WorkspaceID: "w", SourceID: "s", Path: "10/11/13"},
		{Kind: "wiki-page", WorkspaceID: "w", SourceID: "t", Path: "12"},
		{Kind: "wiki-page", WorkspaceID: "x", SourceID: "s", Path: "12"},
	} {
		d, err := svc.Open(ctx, other)
		if err != nil || d.ID == first.ID {
			t.Fatalf("%+v opened the first page's draft (%v)", other, err)
		}
	}
	if found, _ := svc.Find(ctx, short); len(found) != 1 || found[0].ID != first.ID {
		t.Fatalf("found by the short path: %+v", found)
	}
}

// Two drafts of one resource that were made before its paths were known to
// be one are both kept, and the resource opens as the one changed last.
func TestTwoOldDraftsOfOneResourceAreBothKept(t *testing.T) {
	store, _ := Open(t.TempDir())
	ctx := context.Background()
	long := Origin{Kind: "wiki-page", WorkspaceID: "w", SourceID: "s", Path: "10/11/12"}
	short := Origin{Kind: "wiki-page", WorkspaceID: "w", SourceID: "s", Path: "12"}
	// A source that says nothing has a draft for each path.
	plain := &Service{Store: store, Providers: readOnlyProviders{}}
	byShort, _ := plain.Open(ctx, short)
	byLong, _ := plain.Open(ctx, long)
	if byShort.ID == byLong.ID {
		t.Fatal("a source with one resource for each path gave two paths one draft")
	}
	byLong, err := store.Update(byLong.ID, byLong.Revision, "the work in the window\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{Store: store, Providers: pagesProviders{}}
	for _, origin := range []Origin{short, long} {
		d, err := svc.Open(ctx, origin)
		if err != nil || d.ID != byLong.ID {
			t.Fatalf("%s opened %s, and the draft changed last is %s (%v)", origin.Path, d.ID, byLong.ID, err)
		}
	}
	if kept, err := store.Get(byShort.ID); err != nil || kept.Origin != short {
		t.Fatalf("the other draft was not kept: %+v %v", kept, err)
	}
	if found, _ := svc.Find(ctx, short); len(found) != 2 {
		t.Fatalf("the resource's drafts: %d", len(found))
	}
}

// Opens of one resource at the same moment by two of its paths find one
// draft between them.
func TestOpensAtOnceByTwoPathsFindOneDraft(t *testing.T) {
	store, _ := Open(t.TempDir())
	ids := make(chan string, 8)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path := "12"
			if i%2 == 0 {
				path = "10/11/12"
			}
			svc := &Service{Store: store, Providers: pagesProviders{}}
			d, err := svc.Open(context.Background(), Origin{Kind: "wiki-page", WorkspaceID: "w", SourceID: "s", Path: path})
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
	if all, _ := store.List(); len(seen) != 1 || len(all) != 1 {
		t.Fatalf("one resource has %d drafts, and the opens saw %d", len(all), len(seen))
	}
}
