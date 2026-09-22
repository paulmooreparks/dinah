package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// LedgerName is the file in the user base where setup records what it wrote.
const LedgerName = "setup-ledger.json"

// ledgerFormat is the ledger format revision this build reads and writes.
const ledgerFormat = 1

// The kinds of location a ledger entry records.
const (
	entryJSONMember = "json-member"
	entrySection    = "section"
	entryFile       = "file"
)

// ledgerEntry is one location setup wrote, which is what lets a later run tell
// its own work from somebody else's.
type ledgerEntry struct {
	// Recipe, Scope and Base name the run the entry belongs to.
	Recipe string `json:"recipe"`
	Scope  string `json:"scope"`
	Base   string `json:"base"`
	// File is the file the location lives in, absolute, cleaned and written
	// with forward slashes.
	File string `json:"file"`
	// Kind is json-member, section or file.
	Kind string `json:"kind"`
	// Key is the member's JSON pointer, the section's <recipe>/<id>, or the
	// empty string for a whole file.
	Key string `json:"key"`
	// Digest is sha256: and the hex digest of what setup wrote there.
	Digest string `json:"digest"`
	// Workbench is the workbench the run was set up for, at project scope,
	// and empty at user scope.
	Workbench string `json:"workbench"`
	// CreatedFile is true when a run of this recipe created the file.
	CreatedFile bool `json:"created_file"`
	// CreatedParents are the pointers of parent objects setup created for a
	// member, outermost first.
	CreatedParents []string `json:"created_parents,omitempty"`
}

// ledger is the whole ledger file, with the bytes it was read from so that a
// run which changes nothing leaves the file untouched.
type ledger struct {
	// path is the ledger file.
	path string
	// read is the bytes the file held when it was read, nil when it was
	// absent.
	read []byte
	// Entries are the recorded locations, in file order.
	Entries []ledgerEntry
}

// ledgerDocument is the ledger's on-disk shape.
type ledgerDocument struct {
	Format  int           `json:"format"`
	Entries []ledgerEntry `json:"entries"`
}

// readLedger reads the ledger in a user base. An absent ledger is an empty
// one; a ledger that does not parse is refused, because guessing about
// ownership is the one thing setup must not do.
func readLedger(userBase string) (*ledger, error) {
	path := filepath.Join(userBase, LedgerName)
	l := &ledger{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return l, fmt.Errorf("%s: %v", path, err)
	}
	var document ledgerDocument
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&document); err != nil {
		return l, fmt.Errorf("%s: does not parse as a setup ledger (%v)", path, err)
	}
	if document.Format != ledgerFormat {
		return l, fmt.Errorf("%s: declares format %d, and this build reads format %d", path, document.Format, ledgerFormat)
	}
	l.read = data
	l.Entries = document.Entries
	return l, nil
}

// find returns the entry for one location of one run, or nil.
func (l *ledger) find(recipe, scope, base, file, key string) *ledgerEntry {
	for i := range l.Entries {
		e := &l.Entries[i]
		if e.Recipe == recipe && e.Scope == scope && e.Base == base && e.File == file && e.Key == key {
			return e
		}
	}
	return nil
}

// ofRun returns every entry belonging to one run, in ledger order.
func (l *ledger) ofRun(recipe, scope, base string) []ledgerEntry {
	var entries []ledgerEntry
	for _, e := range l.Entries {
		if e.Recipe == recipe && e.Scope == scope && e.Base == base {
			entries = append(entries, e)
		}
	}
	return entries
}

// replaceRun returns the ledger's entries with those of one run replaced by a
// new set. The new entries stand where the run's first old entry stood, so a
// rerun that changes nothing writes the same bytes, and they go at the end
// when the run had none. keep, when non-nil, names old entries of the run to
// keep beside the new ones.
func (l *ledger) replaceRun(recipe, scope, base string, fresh []ledgerEntry, keep func(ledgerEntry) bool) []ledgerEntry {
	var out []ledgerEntry
	placed := false
	for _, e := range l.Entries {
		if e.Recipe != recipe || e.Scope != scope || e.Base != base {
			out = append(out, e)
			continue
		}
		if !placed {
			out = append(out, fresh...)
			placed = true
		}
		if keep != nil && keep(e) {
			out = append(out, e)
		}
	}
	if !placed {
		out = append(out, fresh...)
	}
	return out
}

// write stores a set of entries, leaving the file untouched when its bytes
// would not change and creating no file for an empty ledger nobody wrote.
func (l *ledger) write(entries []ledgerEntry) error {
	if entries == nil {
		entries = []ledgerEntry{}
	}
	if l.read == nil && len(entries) == 0 {
		return nil
	}
	document := ledgerDocument{Format: ledgerFormat, Entries: entries}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if bytes.Equal(data, l.read) {
		return nil
	}
	if err := writeFileAtomic(l.path, data); err != nil {
		return err
	}
	l.read = data
	l.Entries = entries
	return nil
}

// writeFileAtomic writes a file through a temporary beside it and a rename, so
// a reader sees the old bytes or the new ones and never half of either. A new
// file gets permission 0o644 and an existing file keeps its own, and missing
// directories are created with 0o755.
//
// bench.WriteText does the same for Dinah's own files and normalises line
// endings on the way, which setup must not do to a person's file, and the
// byte-exact writer beside it is unexported so that nothing outside bench can
// skip that normalisation. So setup carries this one.
func writeFileAtomic(path string, data []byte) error {
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".dinah-setup-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
