package main

import (
	"context"
	"strings"
	"testing"

	"studio/internal/resource"
)

// orderedSource is a source whose folder "latest" has an order of its own
// and whose other folders have none.
type orderedSource struct{}

func (orderedSource) List(context.Context, string) ([]resource.Entry, error) { return nil, nil }
func (orderedSource) ListsInOrder(path string) bool                          { return path == "latest" }

// plainSource is a source that says nothing about order.
type plainSource struct{}

func (plainSource) List(context.Context, string) ([]resource.Entry, error) { return nil, nil }

func titles(entries []resource.Entry) string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name
	}
	return strings.Join(out, " ")
}

func sample() []resource.Entry {
	return []resource.Entry{{Name: "R-12"}, {Name: "R-3"}, {Name: "notes", Folder: true}, {Name: "R-101"}}
}

// A folder that has an order of its source's own is shown in that order
// (docs/design/source-kinds.md, "A Folder's Order").
func TestAFolderWithAnOrderOfItsOwnKeepsIt(t *testing.T) {
	entries := sample()
	orderEntries(orderedSource{}, "latest", entries)
	if got := titles(entries); got != "R-12 R-3 notes R-101" {
		t.Fatalf("the order is %q", got)
	}
}

// Another folder of the same source, and every folder of a source that says
// nothing, has its folders first and each by name.
func TestAFolderWithoutAnOrderIsShownByName(t *testing.T) {
	for _, c := range []struct {
		provider any
		folder   string
	}{{orderedSource{}, "."}, {orderedSource{}, "other"}, {plainSource{}, "latest"}} {
		entries := sample()
		orderEntries(c.provider, c.folder, entries)
		if got := titles(entries); got != "notes R-3 R-12 R-101" {
			t.Fatalf("the order of %q is %q", c.folder, got)
		}
	}
}
