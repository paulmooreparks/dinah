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
