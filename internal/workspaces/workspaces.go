// Package workspaces is Dinah's workspace registry: a workspace is a
// record with a stable identity, the folder its agents work in, and the
// local folders attached to it as its sources, each of which a resource
// reference points into. The desktop app keeps the registry in its settings
// and changes it.
//
// The package also resolves a reference to its source's resource provider,
// so that documents are opened and saved the same way everywhere.
package workspaces

import (
	"encoding/json"
	"fmt"
	"strings"

	"studio/internal/layout"
)

// Source is a source attached to a workspace.
type Source struct {
	ID         string `json:"id"`
	MountAlias string `json:"mountAlias"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Path       string `json:"path,omitempty"`
	// Extra holds the fields of a source of another kind than a local
	// folder that this build does not know, such as a web page's address,
	// so that they are saved again unchanged (extra.go).
	Extra map[string]json.RawMessage `json:"-"`
}

// MarshalJSON writes the source with the fields Extra holds.
func (s Source) MarshalJSON() ([]byte, error) {
	type plain Source
	return marshalWithExtra(plain(s), s.Extra)
}

// UnmarshalJSON reads the source, keeping the fields it does not know in
// Extra.
func (s *Source) UnmarshalJSON(data []byte) error {
	type plain Source
	var p plain
	extra, err := unmarshalWithExtra(data, &p)
	if err != nil {
		return err
	}
	*s = Source(p)
	s.Extra = extra
	return nil
}

// Record is a registered workspace.
type Record struct {
	ID              string   `json:"id"`
	MountAlias      string   `json:"mountAlias"`
	Name            string   `json:"name"`
	PrimarySourceID string   `json:"primarySourceId"`
	ExecutionPath   string   `json:"executionPath,omitempty"`
	Sources         []Source `json:"sources"`
	// Pins are the items the person pinned to the top of the workspace's
	// pane, in their order.
	Pins []Pin `json:"pins,omitempty"`
	// PlannedFolders are, by source ID, the folders inside a local folder
	// that was not on the computer when the workspace was imported, as its
	// export recorded them: relative to the folder and separated by "/".
	// The Sources tree shows them while the folder is missing, and Create
	// the Folders… makes them (docs/design/workspace-export.md).
	PlannedFolders map[string][]string `json:"plannedFolders,omitempty"`
}

// Pin is an item pinned to a workspace: a chat by its session, or a resource
// of one of the workspace's sources by its reference. Title is what the
// pinned row shows until the item tells a newer one; ItemKind and Directory
// are the resource's, so the row has its glyph without reading the source.
// A pin of another kind, which a workspace export can bring from a build
// that has it, keeps the fields this build does not know in Extra.
type Pin struct {
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Session   string `json:"session,omitempty"`
	Ref       *Ref   `json:"ref,omitempty"`
	ItemKind  string `json:"itemKind,omitempty"`
	Directory bool   `json:"directory,omitempty"`
	// Extra holds the fields this build does not know (extra.go).
	Extra map[string]json.RawMessage `json:"-"`
}

// MarshalJSON writes the pin with the fields Extra holds.
func (p Pin) MarshalJSON() ([]byte, error) {
	type plain Pin
	return marshalWithExtra(plain(p), p.Extra)
}

// UnmarshalJSON reads the pin, keeping the fields it does not know in Extra.
func (p *Pin) UnmarshalJSON(data []byte) error {
	type plain Pin
	var q plain
	extra, err := unmarshalWithExtra(data, &q)
	if err != nil {
		return err
	}
	*p = Pin(q)
	p.Extra = extra
	return nil
}

// PinKey is a pin's identity within its workspace: the same item pinned
// twice is one pin. A pin of a kind this build does not know is told apart
// by the fields it does not know.
func PinKey(p Pin) string {
	switch p.Kind {
	case "chat":
		return "chat:" + p.Session
	case "resource":
		if p.Ref != nil {
			return "resource:" + p.Ref.SourceID + "/" + p.Ref.Path
		}
		return ""
	case "":
		return ""
	}
	return p.Kind + ":" + extraKey(p.Extra)
}

// Ref is a resource of a workspace: a path within one of its sources.
type Ref struct {
	WorkspaceID string `json:"workspaceId"`
	SourceID    string `json:"sourceId"`
	Path        string `json:"path"`
}

// Copy copies records, so that the copy's sources can be changed.
func Copy(records []Record) []Record {
	out := make([]Record, len(records))
	for i, record := range records {
		out[i] = record
		out[i].Sources = append([]Source{}, record.Sources...)
		for j := range out[i].Sources {
			out[i].Sources[j].Extra = copyExtra(out[i].Sources[j].Extra)
		}
		if record.Pins != nil {
			out[i].Pins = append([]Pin{}, record.Pins...)
			for j := range out[i].Pins {
				out[i].Pins[j].Extra = copyExtra(out[i].Pins[j].Extra)
			}
		}
		if record.PlannedFolders != nil {
			out[i].PlannedFolders = make(map[string][]string, len(record.PlannedFolders))
			for id, folders := range record.PlannedFolders {
				out[i].PlannedFolders[id] = append([]string{}, folders...)
			}
		}
	}
	return out
}

// ExecutionPath is the folder a workspace's agents and runs work in.
func ExecutionPath(record Record) string {
	if record.ExecutionPath != "" {
		return record.ExecutionPath
	}
	source, _ := PrimarySource(record)
	return source.Path
}

// PrimarySource is a workspace's primary local folder.
func PrimarySource(record Record) (Source, bool) {
	for _, source := range record.Sources {
		if source.Kind == "local-folder" && (record.PrimarySourceID == "" || source.ID == record.PrimarySourceID) {
			return source, true
		}
	}
	return Source{}, false
}

// IndexAt is the index of the workspace that works in a folder, or -1.
func IndexAt(records []Record, path string) int {
	for i, record := range records {
		if layout.SamePath(ExecutionPath(record), path) {
			return i
		}
	}
	return -1
}

// Index is the index of the workspace with an identity, or -1.
func Index(records []Record, id string) int {
	for i, record := range records {
		if record.ID == id {
			return i
		}
	}
	return -1
}

// ValidAttachmentName reports whether a name can be an attachment's in a
// resource path.
func ValidAttachmentName(name string) bool {
	return name != "" && name != "." && name != ".." && len(name) <= 255 && !strings.ContainsAny(name, "/\\:\x00\r\n") && !strings.HasPrefix(name, ".")
}

// MountAlias is a name for a mount point among those used, made from a
// label, with characters that would split a path escaped and a number added
// to tell it from one already used.
func MountAlias(name, fallback string, used map[string]bool) string {
	base := strings.NewReplacer("%", "%25", "/", "%2F", "\\", "%5C").Replace(name)
	if base == "" {
		base = fallback
	}
	if base == "." || base == ".." {
		base = strings.ReplaceAll(base, ".", "%2E")
	}
	alias := base
	for n := 2; used[alias]; n++ {
		alias = fmt.Sprintf("%s (%d)", base, n)
	}
	used[alias] = true
	return alias
}
