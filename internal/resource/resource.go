// Package resource defines what a workspace source can do with its resources,
// as interfaces that each kind of source implements. A local folder is a
// provider, and so would be a wiki's page tree or a remote service. An
// applet, a flow, the Editor and the command line use a provider through
// these interfaces and never through its kind, so a new kind of source plugs
// in by implementing them, and a new consumer works with every source that
// offers the capability it needs.
//
// A provider implements only the capabilities its source supports. Callers
// discover a capability with a type assertion and report ErrUnsupported when it
// is absent, rather than branching on the source's kind.
package resource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// ContentRevision is the revision of a text resource: the hex SHA-256 of its
// UTF-8 bytes. Every provider that reports a revision for content it holds as
// text uses it, so a revision read from one place can be compared with one
// computed anywhere else.
func ContentRevision(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// Text is a resource's text content and the revision it was read at.
type Text struct {
	Text     string
	Revision string
	// Format is "markdown" or "text".
	Format string
}

// TextReader reads a resource as text. Path is relative to the provider's
// source, with forward slashes.
type TextReader interface {
	ReadText(ctx context.Context, path string) (Text, error)
}

// Entry is one entry of a folder in a source.
type Entry struct {
	Name   string
	Folder bool
	// Size is a file's size in bytes.
	Size int64
}

// Lister lists the entries of a folder in a source; the path "." is the
// source's root. A source with folders implements it, which together with
// TextReader lets FS present the source as an fs.FS.
type Lister interface {
	List(ctx context.Context, path string) ([]Entry, error)
}

// TextWriter replaces a resource's text, provided the resource is still at
// expected, the revision the caller last read. An empty expected means the
// resource must not exist yet, so a write can never replace something the
// caller has not seen. It returns the new revision.
type TextWriter interface {
	WriteText(ctx context.Context, path, expected, text string) (string, error)
}

// Snapshot is a resource as a document editor needs it: its text, revision
// and title, and its native form when the source stores something other than
// the text (the markup a wiki keeps a page in), which a later publication of
// changes back to the source works from.
type Snapshot struct {
	Text     string
	Revision string
	Format   string
	Title    string
	Native   string
	// Labels are a page's labels, which a document type may be recognised
	// by; a source without labels leaves them empty.
	Labels []string
}

// Labeler adds labels to a resource, as a page on a wiki may have them. Adding a
// label the resource already has changes nothing.
type Labeler interface {
	AddLabels(ctx context.Context, path string, labels []string) error
}

// Snapshotter takes a Snapshot of a resource. A source whose resources can be
// opened for editing implements it.
type Snapshotter interface {
	Snapshot(ctx context.Context, path string) (Snapshot, error)
}

// Edit is what a document editor writes back: the edited text and title.
type Edit struct {
	Text  string
	Title string
	// Files are files the text refers to by name that are to be kept beside
	// the document, such as images added in the Editor, or a page's images
	// and rendered diagrams, which a new copy of the document needs beside
	// it. A writer or creator writes them with the text.
	Files []File
}

// File is a file kept beside a document, by its name there.
type File struct {
	Name string
	Data []byte
}

// MaxFile bounds one file kept beside a document that is read or written.
const MaxFile = 50 << 20

// FileSource lists and reads the files kept beside a document, as a
// page on a wiki keeps its attachments.
type FileSource interface {
	FileNames(ctx context.Context, path string) ([]string, error)
	ReadFile(ctx context.Context, path, name string) ([]byte, error)
}

// Written is the outcome of writing a document back: the resource as it now
// is, which becomes the draft's new baseline, and notes for the person about
// what the write could not keep (a comment that lost its anchor, a title a
// source cannot change).
type Written struct {
	Snapshot Snapshot
	Notes    []string
	// Path is where a created document is, when the source decides it rather
	// than taking the path it was given, as when a remote service gives a new
	// page its own ID; it is empty otherwise.
	Path string
}

// DocumentWriter writes an edited document back to its source, given the
// snapshot the edit started from. Having the starting point lets a source that
// stores another form than the text (a wiki's own page markup) change
// only what was edited. The write is refused with ErrConflict when the
// resource has changed since that snapshot.
type DocumentWriter interface {
	WriteDocument(ctx context.Context, path string, base Snapshot, edit Edit) (Written, error)
}

// DocumentCreator creates a new document at path from an edit, refusing with
// ErrConflict when something already exists there. A source where new
// documents can be made implements it, so an editor offers only those sources
// as places to save a new document. For a source that chooses its documents'
// paths itself, path is where the document goes (a wiki page's parent), and
// Written.Path is the document's own path.
type DocumentCreator interface {
	CreateDocument(ctx context.Context, path string, edit Edit) (Written, error)
}

// Renamer renames a document within its folder, refusing with ErrConflict
// when something else already has the new name. A source whose documents are
// files implements it.
type Renamer interface {
	Rename(ctx context.Context, from, to string) error
}

// Publisher marks a source whose writes make a document visible to others,
// such as a wiki, as opposed to saving a private file. Editors call the
// write Publish for such a source and may ask for confirmation before it.
type Publisher interface {
	Publishes() bool
}

// ErrConflict means the resource is no longer at the expected revision, or
// already exists when it was expected not to.
var ErrConflict = errors.New("the resource has changed since it was read")

// ErrUnsupported means the source does not offer the requested capability.
var ErrUnsupported = errors.New("this source does not support the operation")

// ErrNotFound means the resource does not exist.
var ErrNotFound = errors.New("the resource does not exist")
