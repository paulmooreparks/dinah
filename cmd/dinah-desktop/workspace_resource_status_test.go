package main

import (
	"encoding/json"
	"strings"
	"testing"

	"studio/internal/resource"
)

/* An entry's status and its changed mark reach the page on the entry's item
   (docs/design/source-kinds.md, "An Entry's Status"). An entry without them
   has an item with neither field, so that the page draws it as before. */

func TestProviderItemHasTheEntrysStatus(t *testing.T) {
	parent := ResourceRef{WorkspaceID: "w", SourceID: "s"}
	item := providerItem(parent, WorkspaceSource{}, resource.Entry{Name: "r-1", Title: "First record", Kind: "record", Status: resource.Status{Label: "  In\n Review ", State: resource.StatusAccent}, Changed: true}, false)
	if item.Status == nil || item.Status.Label != "In Review" || item.Status.State != "accent" || !item.Changed {
		t.Fatalf("the item's status: %+v changed %v", item.Status, item.Changed)
	}
	data, err := json.Marshal(item)
	if err != nil || !strings.Contains(string(data), `"status":{"label":"In Review","state":"accent"}`) || !strings.Contains(string(data), `"changed":true`) {
		t.Fatalf("the item as the page reads it: %s %v", data, err)
	}
}

func TestProviderItemShowsAnUnknownStateAsThePlainBadge(t *testing.T) {
	item := providerItem(ResourceRef{}, WorkspaceSource{}, resource.Entry{Name: "r-2", Status: resource.Status{Label: "Parked", State: "purple"}}, false)
	if item.Status == nil || item.Status.Label != "Parked" || item.Status.State != resource.StatusNeutral {
		t.Fatalf("the item's status: %+v", item.Status)
	}
	for _, state := range []string{resource.StatusNeutral, resource.StatusAccent, resource.StatusPositive, resource.StatusWarn, resource.StatusDanger} {
		if got := (resource.Status{Label: "x", State: state}).ShownState(); got != state {
			t.Fatalf("the state %q is shown as %q", state, got)
		}
	}
}

func TestProviderItemWithoutAStatusHasNone(t *testing.T) {
	for _, e := range []resource.Entry{{Name: "plain.md", Size: 3}, {Name: "words", Status: resource.Status{Label: "  ", State: resource.StatusWarn}}} {
		item := providerItem(ResourceRef{}, WorkspaceSource{}, e, false)
		data, err := json.Marshal(item)
		if err != nil || item.Status != nil || item.Changed || strings.Contains(string(data), `"status"`) || strings.Contains(string(data), `"changed"`) {
			t.Fatalf("an entry without a status: %s %v", data, err)
		}
	}
}

func TestMountItemKeepsTheStatus(t *testing.T) {
	item := providerItem(ResourceRef{WorkspaceID: "w", SourceID: "s"}, WorkspaceSource{}, resource.Entry{Name: "r-1", Status: resource.Status{Label: "Done", State: resource.StatusPositive}, Changed: true}, false)
	mounted := mountResourceItem(mountTarget{record: WorkspaceRecord{MountAlias: "w"}, source: WorkspaceSource{MountAlias: "s"}}, item)
	if mounted.Status == nil || mounted.Status.Label != "Done" || mounted.Status.State != "positive" || !mounted.Changed || mounted.Path != "/workspaces/w/s/r-1" {
		t.Fatalf("the mount item: %+v", mounted)
	}
}

/* The page reads a folder's counts from the folder's item
   (docs/design/source-kinds.md, "A Folder's Counts"). A count that is known
   is a number, zero included, and a count that is not known is absent, so
   the page can tell a folder with nothing in it from one that has not been
   read. */

func TestProviderItemHasAFoldersKnownCounts(t *testing.T) {
	entry := resource.Entry{Name: "open", Title: "Open Records", Kind: "sample-query", Folder: true, Count: resource.Counted(12), ChangedCount: resource.Counted(3)}
	item := providerItem(ResourceRef{WorkspaceID: "w", SourceID: "s"}, WorkspaceSource{}, entry, false)
	if item.Count == nil || *item.Count != 12 || item.ChangedCount == nil || *item.ChangedCount != 3 {
		t.Fatalf("the item's counts: %v %v", item.Count, item.ChangedCount)
	}
	data, err := json.Marshal(item)
	if err != nil || !strings.Contains(string(data), `"count":12`) || !strings.Contains(string(data), `"changedCount":3`) {
		t.Fatalf("the item as the page reads it: %s %v", data, err)
	}
	// The item's numbers are its own, so a later change to the entry changes no item.
	*entry.Count = 99
	if *item.Count != 12 {
		t.Fatalf("the item's count followed the entry: %d", *item.Count)
	}
}

func TestProviderItemTellsAZeroCountFromOneNotKnown(t *testing.T) {
	zero := providerItem(ResourceRef{}, WorkspaceSource{}, resource.Entry{Name: "empty", Folder: true, Count: resource.Counted(0), ChangedCount: resource.Counted(0)}, false)
	data, err := json.Marshal(zero)
	if err != nil || zero.Count == nil || *zero.Count != 0 || zero.ChangedCount == nil || *zero.ChangedCount != 0 || !strings.Contains(string(data), `"count":0`) || !strings.Contains(string(data), `"changedCount":0`) {
		t.Fatalf("a folder with counts of zero: %s %v", data, err)
	}
	unread := providerItem(ResourceRef{}, WorkspaceSource{}, resource.Entry{Name: "unread", Folder: true}, false)
	data, err = json.Marshal(unread)
	if err != nil || unread.Count != nil || unread.ChangedCount != nil || strings.Contains(string(data), `ount"`) {
		t.Fatalf("a folder whose counts are not known: %s %v", data, err)
	}
	// One count may be known while the other is not.
	half := providerItem(ResourceRef{}, WorkspaceSource{}, resource.Entry{Name: "half", Folder: true, Count: resource.Counted(7)}, false)
	data, err = json.Marshal(half)
	if err != nil || half.Count == nil || *half.Count != 7 || half.ChangedCount != nil || !strings.Contains(string(data), `"count":7`) || strings.Contains(string(data), `"changedCount"`) {
		t.Fatalf("a folder with one count known: %s %v", data, err)
	}
}

func TestProviderItemIgnoresCountsThatDoNotApply(t *testing.T) {
	// An entry that is not a folder has no counts, and a count below zero is no count.
	for _, e := range []resource.Entry{{Name: "r-1", Count: resource.Counted(4), ChangedCount: resource.Counted(1)}, {Name: "odd", Folder: true, Count: resource.Counted(-1), ChangedCount: resource.Counted(-5)}} {
		item := providerItem(ResourceRef{}, WorkspaceSource{}, e, false)
		data, err := json.Marshal(item)
		if err != nil || item.Count != nil || item.ChangedCount != nil || strings.Contains(string(data), `ount"`) {
			t.Fatalf("counts that do not apply: %s %v", data, err)
		}
	}
}

func TestMountItemKeepsTheCounts(t *testing.T) {
	target := mountTarget{record: WorkspaceRecord{MountAlias: "w"}, source: WorkspaceSource{MountAlias: "s"}}
	item := providerItem(ResourceRef{WorkspaceID: "w", SourceID: "s"}, WorkspaceSource{}, resource.Entry{Name: "open", Folder: true, Count: resource.Counted(0), ChangedCount: resource.Counted(2)}, false)
	mounted := mountResourceItem(target, item)
	data, err := json.Marshal(mounted)
	if err != nil || mounted.Count == nil || *mounted.Count != 0 || mounted.ChangedCount == nil || *mounted.ChangedCount != 2 || !strings.Contains(string(data), `"count":0`) || !strings.Contains(string(data), `"changedCount":2`) {
		t.Fatalf("the mount item: %s %v", data, err)
	}
	unread := mountResourceItem(target, providerItem(ResourceRef{WorkspaceID: "w", SourceID: "s"}, WorkspaceSource{}, resource.Entry{Name: "unread", Folder: true}, false))
	data, err = json.Marshal(unread)
	if err != nil || unread.Count != nil || unread.ChangedCount != nil || strings.Contains(string(data), `ount"`) {
		t.Fatalf("a mount item whose counts are not known: %s %v", data, err)
	}
}

func TestALocalFoldersItemsHaveNoCounts(t *testing.T) {
	item := workspaceSourceResource(WorkspaceRecord{ID: "w"}, WorkspaceSource{ID: "s", Kind: "local-folder", Name: "Files"})
	data, err := json.Marshal(item)
	if err != nil || item.Count != nil || item.ChangedCount != nil || strings.Contains(string(data), `ount"`) {
		t.Fatalf("a local folder's item: %s %v", data, err)
	}
}
