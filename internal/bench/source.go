package bench

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
)

// Source is where a Bench reads the files below its root. Disk reads the
// filesystem; a resident snapshot answers from memory. Every method takes an
// absolute path and answers what the named os or bench function answers for
// it, errors included, so a caller cannot tell the two apart by result.
type Source interface {
	// ReadFile is os.ReadFile. The bytes are the caller's own.
	ReadFile(path string) ([]byte, error)
	// ReadHead is at most n bytes from the start of a file, read as
	// os.Open followed by io.ReadFull, with io.EOF and io.ErrUnexpectedEOF
	// answered as a short read and not as an error.
	ReadHead(path string, n int) ([]byte, error)
	// ReadDir is os.ReadDir: every entry, sorted by name.
	ReadDir(dir string) ([]fs.DirEntry, error)
	// Stat is os.Stat.
	Stat(path string) (fs.FileInfo, error)
	// Text is readTextAndRevision: the text as ReadText normalises it and
	// the revision of the stored bytes as Revision computes it.
	Text(path string) (text, revision string, err error)
	// Derive answers derive applied to the file's text and revision. derive
	// must be a pure function of its arguments and of path. A source may
	// memoise the answer per path and kind and hand the same value to every
	// caller, so the caller treats it as immutable and clones before it
	// hands a value out.
	Derive(path string, kind DeriveKind, derive func(path, text, revision string) (any, error)) (any, error)
}

// DeriveKind names what Derive builds, so a memo keyed on it never answers
// one kind with another.
type DeriveKind string

const (
	DeriveAnchor      DeriveKind = "anchor"       // *parsedAnchor: ParseAnchor's frontmatter and body
	DeriveCard        DeriveKind = "card"         // *Card, current vocabulary
	DeriveCardRetired DeriveKind = "card-retired" // *Card, retired vocabulary
	DeriveItem        DeriveKind = "item"         // *Item
	DeriveComment     DeriveKind = "comment"      // *Comment
	DeriveAttachment  DeriveKind = "attachment"   // *Attachment
	DeriveJournal     DeriveKind = "journal"      // *parsedJournal: ReadJournal's events and torn flag
)

// Disk is the Source every Bench reads through unless it was opened over
// another. Its methods call os and the existing readers, and Derive calls
// derive every time.
type Disk struct{}

// AttachmentHeadBytes is how much of an attachment's payload a read may take
// from its start. Search reads no more than this, and a resident snapshot
// holds no more than this of any payload.
const AttachmentHeadBytes = 65536

// ReadFile is os.ReadFile.
func (Disk) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// ReadHead is os.Open followed by io.ReadFull into an n-byte buffer, with a
// file shorter than n answered as a short read.
func (Disk) ReadHead(path string, n int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ReadHeadFrom(file, n)
}

// ReadHeadFrom is ReadHead's reading half over an open reader: at most n
// bytes, io.EOF and io.ErrUnexpectedEOF answered as a short read. A resident
// snapshot reads a payload's head through it, so the two agree on what a
// head is.
func ReadHeadFrom(r io.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	read, err := io.ReadFull(r, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return buf[:read], nil
}

// ReadDir is os.ReadDir.
func (Disk) ReadDir(dir string) ([]fs.DirEntry, error) {
	return os.ReadDir(dir)
}

// Stat is os.Stat.
func (Disk) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

// Text reads the file once and answers its normalised text and the revision
// of its stored bytes.
func (Disk) Text(path string) (text, revision string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	text, revision = TextAndRevisionOf(data)
	return text, revision, nil
}

// Derive reads the file and calls derive, every time, so a bench on Disk
// parses what it parsed before the seam and holds nothing between calls.
func (d Disk) Derive(path string, _ DeriveKind, derive func(path, text, revision string) (any, error)) (any, error) {
	text, revision, err := d.Text(path)
	if err != nil {
		return nil, err
	}
	return derive(path, text, revision)
}

// TextAndRevisionOf is what Text answers for a file's stored bytes: the text
// with its byte-order mark stripped and its line endings normalised, and the
// revision of the bytes as stored.
func TextAndRevisionOf(data []byte) (text, revision string) {
	stored := string(data)
	return NormalizeNewlines(strings.TrimPrefix(stored, byteOrderMark)), TextRevision(stored)
}

// OpenWith is Open reading through src: the same checks in the same order,
// the same refusals, with every file read through src.
func OpenWith(root string, src Source) (*Bench, error) {
	return openWithVocabulary(src, root, currentVocabulary, admitProfileAfterVocabulary, true, true)
}

// source is the Source this bench reads through: the one it was opened
// over, or Disk for a bench built any other way.
func (b *Bench) source() Source {
	if b == nil || b.src == nil {
		return Disk{}
	}
	return b.src
}

// Reopen opens this bench's root afresh through the source it was opened
// over, which is how a verb re-reads the workbench after a write changed its
// definition.
func (b *Bench) Reopen() (*Bench, error) {
	return OpenWith(b.Root, b.source())
}

// OpenAt opens another workbench through this bench's source. A resident
// snapshot answers a path outside its own root from the disk, so this reads
// another workbench exactly as Open does.
func (b *Bench) OpenAt(root string) (*Bench, error) {
	return OpenWith(root, b.source())
}

// parsedAnchor is what DeriveAnchor memoises: ParseAnchor's two answers.
type parsedAnchor struct {
	fm   *Frontmatter
	body string
}

// deriveAnchor is DeriveAnchor's derive function.
func deriveAnchor(_ string, text, _ string) (any, error) {
	fm, body := ParseAnchor(text)
	fm.markShared()
	return &parsedAnchor{fm: fm, body: body}, nil
}

// anchorOf reads an anchor through src and answers its header and body, the
// header a clone the caller may change.
func anchorOf(src Source, path string) (*Frontmatter, string, error) {
	observeAnchor(path)
	value, err := src.Derive(path, DeriveAnchor, deriveAnchor)
	if err != nil {
		return nil, "", err
	}
	parsed := value.(*parsedAnchor)
	return parsed.fm.Clone(), parsed.body, nil
}

// observeAnchor calls AnchorReadObserver when a test has set it.
func observeAnchor(path string) {
	if AnchorReadObserver != nil {
		AnchorReadObserver(path)
	}
}

// ReadHead is at most n bytes from the start of a file, read through this
// bench's source on the terms Source.ReadHead states.
func (b *Bench) ReadHead(path string, n int) ([]byte, error) {
	return b.source().ReadHead(path, n)
}
