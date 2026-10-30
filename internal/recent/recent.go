// Package recent keeps the documents a person has opened recently, newest
// first, in recent-documents.json in Dinah's state folder. The desktop
// app and the command line both record into it, so a document opened by a
// skill in a terminal is under Documents › Recent in the app too.
//
// An entry is where the document is (a workspace resource, or the path of a
// file on the computer that belongs to no workspace), never a draft's ID,
// which a discard destroys, and never a workspace's name, which is looked up
// when the list is shown so that a renamed workspace shows its new name.
package recent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"studio/internal/layout"
	"studio/internal/profile"
	"studio/internal/statefile"
	"studio/internal/workspaces"
)

// Kept is how many entries the file keeps; the menu shows fewer.
const Kept = 50

// FileName is the file's name in the state folder.
const FileName = "recent-documents.json"

// Entry is one recently opened document.
type Entry struct {
	// Ref is a workspace resource's reference. It is nil for a file on the
	// computer that belongs to no workspace.
	Ref *workspaces.Ref `json:"ref,omitempty"`
	// File is the absolute path of a file on the computer that belongs to no
	// workspace; it reopens as a new document with the file's text.
	File string `json:"file,omitempty"`
	// WorkspaceID is the workspace a computer file was opened in, which its
	// new document belongs to.
	WorkspaceID string `json:"workspaceId,omitempty"`
	// Kind is the kind of source the document is in, as its draft's origin
	// gives it ("local-folder" and so on), or "computer-file".
	Kind   string    `json:"kind"`
	Title  string    `json:"title"`
	Opened time.Time `json:"opened"`
}

// KindComputerFile is the kind of a file outside every workspace.
const KindComputerFile = "computer-file"

type file struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

// Store is the recent list of one state folder.
type Store struct{ Path string }

// For is the recent list of a state folder.
func For(l layout.Layout) Store { return Store{Path: filepath.Join(l.State, FileName)} }

// same reports whether two entries are one document.
func same(a, b Entry) bool {
	if a.Ref != nil || b.Ref != nil {
		return a.Ref != nil && b.Ref != nil && *a.Ref == *b.Ref
	}
	return a.File != "" && layout.SamePath(a.File, b.File)
}

func check(e Entry) error {
	switch {
	case e.Ref != nil && e.File != "":
		return errors.New("a recent document is a workspace resource or a file, not both")
	case e.Ref == nil && e.File == "":
		return errors.New("a recent document needs a resource or a file")
	case e.File != "" && !filepath.IsAbs(e.File):
		return errors.New("a recent file needs an absolute path")
	case strings.TrimSpace(e.Title) == "":
		return errors.New("a recent document needs a title")
	}
	return nil
}

// errDamaged is a file that is not a recent list at all, which Record moves
// aside, since the list is a convenience a person cannot repair from the
// application. A list written by a newer Dinah is refused instead,
// so that it is never replaced unread.
var errDamaged = errors.New("the recent documents list is damaged")

func decode(data []byte) ([]Entry, error) {
	if data == nil {
		return []Entry{}, nil
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil || f.Version < 1 {
		return nil, errDamaged
	}
	if f.Version > 1 {
		return nil, fmt.Errorf("%s was written by a newer %s", FileName, profile.Current().Name)
	}
	if f.Entries == nil {
		f.Entries = []Entry{}
	}
	return f.Entries, nil
}

// List returns the entries, newest first. A damaged list reads as empty.
func (s Store) List() ([]Entry, error) {
	data, err := statefile.ReadShared(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	entries, err := decode(data)
	if errors.Is(err, errDamaged) {
		return []Entry{}, nil
	}
	return entries, err
}

// Record puts a document at the top of the list, moving it there when it is
// already in it, and keeps the newest Kept entries.
func (s Store) Record(e Entry) error {
	if err := check(e); err != nil {
		return err
	}
	if e.Opened.IsZero() {
		e.Opened = time.Now()
	}
	e.Opened = e.Opened.UTC()
	return statefile.Update(s.Path, func(previous []byte) ([]byte, error) {
		entries, err := decode(previous)
		if errors.Is(err, errDamaged) {
			// Kept beside the new list for whoever wants to see it.
			if err := os.Rename(s.Path, s.Path+".damaged-"+time.Now().UTC().Format("20060102T150405Z")); err != nil {
				return nil, err
			}
			entries, err = []Entry{}, nil
		}
		if err != nil {
			return nil, err
		}
		next := []Entry{e}
		for _, old := range entries {
			if !same(old, e) && len(next) < Kept {
				next = append(next, old)
			}
		}
		return json.MarshalIndent(file{Version: 1, Entries: next}, "", "  ")
	})
}

// Move points the entry of the resource from at to instead, with a new
// title, as when a file is renamed; the entry keeps its place in the list,
// and an entry already at to is dropped, so the document is listed once. A
// list without the entry is left as it is.
func (s Store) Move(from, to workspaces.Ref, title string) error {
	return statefile.Update(s.Path, func(previous []byte) ([]byte, error) {
		entries, err := decode(previous)
		if errors.Is(err, errDamaged) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		target := Entry{Ref: &to}
		next := []Entry{}
		found := false
		for _, e := range entries {
			switch {
			case e.Ref != nil && *e.Ref == from:
				moved := to
				e.Ref, e.Title, found = &moved, title, true
			case same(e, target):
				continue
			}
			next = append(next, e)
		}
		if !found {
			return nil, nil
		}
		return json.MarshalIndent(file{Version: 1, Entries: next}, "", "  ")
	})
}

// Available leaves out the entries that can no longer be opened: a resource
// whose workspace or source is gone, and a computer file that is gone. A
// reference to a source that belongs to no workspace, such as the person's
// home folder, is kept.
func Available(entries []Entry, records []workspaces.Record) []Entry {
	out := []Entry{}
	for _, e := range entries {
		switch {
		case e.Ref != nil && e.Ref.WorkspaceID == "" && e.Ref.SourceID != "":
			out = append(out, e)
		case e.Ref != nil:
			if i := workspaces.Index(records, e.Ref.WorkspaceID); i >= 0 && hasSource(records[i], e.Ref.SourceID) {
				out = append(out, e)
			}
		case e.File != "":
			// The file reopens as a new document of its workspace, so a
			// workspace that is gone takes the entry with it.
			if e.WorkspaceID != "" && workspaces.Index(records, e.WorkspaceID) < 0 {
				continue
			}
			if info, err := os.Stat(e.File); err == nil && info.Mode().IsRegular() {
				out = append(out, e)
			}
		}
	}
	return out
}

func hasSource(record workspaces.Record, id string) bool {
	for _, source := range record.Sources {
		if source.ID == id {
			return true
		}
	}
	return false
}
