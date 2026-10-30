package workspaces

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"studio/internal/layout"
	"studio/internal/resource"
	"studio/internal/resource/localfs"
)

func TestResolver(t *testing.T) {
	l := layout.Layout{State: t.TempDir()}
	folder := t.TempDir()
	os.MkdirAll(filepath.Join(folder, ".devin", "skills"), 0o755)
	record := Record{ID: "ws_1", Name: "One", ExecutionPath: folder, PrimarySourceID: "src", Sources: []Source{{ID: "src", Kind: "local-folder", Name: "Files", Path: folder}}}
	home := Source{ID: HomeSourceID, Kind: "local-folder", Name: "Home", Path: t.TempDir()}
	r := Resolver{Layout: l, Records: func() []Record { return []Record{record} }, Personal: func(string) (Source, error) { return home, nil }}
	if s, err := r.Source(Ref{WorkspaceID: "ws_1", SourceID: "src"}); err != nil || s.Path != folder {
		t.Fatalf("source %+v %v", s, err)
	}
	// Another attached folder is content: its packages are not offered.
	other := t.TempDir()
	os.MkdirAll(filepath.Join(other, ".devin", "skills"), 0o755)
	record.Sources = append(record.Sources, Source{ID: "other", Kind: "local-folder", Name: "Other", Path: other})
	collections := Collections(l, record)
	if len(collections) != 1 {
		t.Fatalf("collections %+v", collections)
	}
	if s, err := r.Source(Ref{WorkspaceID: "ws_1", SourceID: collections[0].ID}); err != nil || s.Path != filepath.Join(folder, ".devin", "skills") {
		t.Fatalf("a package collection was not a source: %+v %v", s, err)
	}
	if s, err := r.Source(Ref{SourceID: HomeSourceID}); err != nil || s.Path != home.Path {
		t.Fatalf("home %+v %v", s, err)
	}
	if _, err := r.Source(Ref{WorkspaceID: "ws_9", SourceID: "src"}); err == nil {
		t.Fatal("an unregistered workspace was found")
	}
	p, _, err := r.Provider(context.Background(), Ref{WorkspaceID: "ws_1", SourceID: "src"})
	if _, ok := p.(localfs.Folder); err != nil || !ok {
		t.Fatalf("provider %T %v", p, err)
	}
	record.Sources = append(record.Sources, Source{ID: "unknown", Kind: "unknown-kind"})
	if _, _, err := r.Provider(context.Background(), Ref{WorkspaceID: "ws_1", SourceID: "unknown"}); !errors.Is(err, resource.ErrUnsupported) {
		t.Fatalf("a kind with no provider %v", err)
	}
}
