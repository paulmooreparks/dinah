package bench

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TimeFormat is the timestamp form every journal line and every claim field
// carries. It is RFC 3339, and the format's encoding rules put it in UTC so
// that a bench read on another machine sorts the way it was written.
const TimeFormat = time.RFC3339

// Stamp renders a time the way the format stores one.
func Stamp(t time.Time) string {
	return t.UTC().Format(TimeFormat)
}

// ParseStamp reads a stored timestamp. A value that does not parse comes back
// as the zero time rather than as an error, because a hand-edited line is a
// thing check reports rather than a thing a read refuses.
func ParseStamp(s string) time.Time {
	t, err := time.Parse(TimeFormat, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// byteOrderMark is the mark an editor writes at the head of a UTF-8 file. The
// format writes text without one, so it is stripped on read.
const byteOrderMark = "\ufeff"

// ReadText reads a text file and strips a byte-order mark. The format writes
// UTF-8 without one; a mark reaching the tree came from an editor, and
// stripping it on read is the same tolerance the CRLF rule extends.
func ReadText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(string(data), byteOrderMark), nil
}

// WriteText writes a file by writing a temporary beside it and renaming, so a
// reader sees either the old bytes or the new ones and never a half-written
// file. This is the write half of the format's concurrency answer.
func WriteText(path, text string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".dinah-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// Revision is the opaque revision of an anchor file: the content hash read
// under the card lock, which is what a basis names. Callers never parse it,
// because the remote arbiter will compute its own revision another way.
func Revision(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return TextRevision(string(data)), nil
}

// TextRevision is the same opaque revision Revision computes, over a string a
// caller already holds rather than over a file it has to read. The instruction
// chain identifies a layer by the revision of the text it serves, and Revision
// calls this so one function computes both and the two forms can never drift.
func TextRevision(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// NewID mints a 12-character lowercase hex identifier.
func NewID() (string, error) {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

// IsID reports whether a string is a well-formed 12-hex identifier. The test
// is ASCII by construction, which is what keeps a bench parsing identically
// under a Turkish locale and a neutral one.
func IsID(s string) bool {
	if len(s) != 12 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		digit := c >= '0' && c <= '9'
		letter := c >= 'a' && c <= 'f'
		if !digit && !letter {
			return false
		}
	}
	return true
}

// ClaimID creates the directory of a fresh entity, which is the atomic
// test-and-claim of its identifier: mkdir either wins the name or reports
// that somebody else holds it. It retries on a collision and returns the id
// it claimed.
func ClaimID(collection string, taken func(string) bool) (string, error) {
	if err := os.MkdirAll(collection, 0o755); err != nil {
		return "", err
	}
	for attempt := 0; attempt < 16; attempt++ {
		id, err := NewID()
		if err != nil {
			return "", err
		}
		if taken != nil && taken(id) {
			continue
		}
		err = os.Mkdir(filepath.Join(collection, id), 0o755)
		if err == nil {
			return id, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("could not claim an identifier in %s after 16 attempts", collection)
}

// ListIDs returns the identifiers of a collection directory, sorted
// ascending, ignoring anything that is not a hex directory. An absent
// collection is an empty one, which is the absent-means-empty rule.
func ListIDs(collection string) []string {
	entries, err := os.ReadDir(collection)
	if err != nil {
		return nil
	}
	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() || !IsID(entry.Name()) {
			continue
		}
		ids = append(ids, entry.Name())
	}
	return ids
}

// Exists reports whether a path is present.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// WorkbenchIDLength is the number of characters a workbench identifier takes:
// sixteen bytes of UUID rendered as two lowercase hex characters each.
const WorkbenchIDLength = 32

// NewWorkbenchID mints a workbench identifier, which is a UUID version 7 per
// RFC 9562 section 5.7 rendered as WorkbenchIDLength lowercase hex characters
// with the canonical hyphens stripped.
//
// The layout the RFC fixes is a 48-bit big-endian count of milliseconds since
// the Unix epoch, a 4-bit version field holding 0111, a 12-bit random field,
// a 2-bit variant field holding 10, and a 62-bit random field. Both random
// fields come from crypto/rand, so two workbenches created on two machines in
// the same millisecond still differ, and the leading timestamp orders a
// listing by the moment each workbench was created.
//
// The hyphens are dropped because every directory name this format mints is
// lowercase hex and nothing else, which is what lets one glance at a path say
// whether a name was minted by the tool or typed by a person.
func NewWorkbenchID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	milliseconds := time.Now().UTC().UnixMilli()
	for i := 0; i < 6; i++ {
		raw[i] = byte(milliseconds >> (40 - 8*i))
	}
	raw[6] = (raw[6] & 0x0f) | 0x70
	raw[8] = (raw[8] & 0x3f) | 0x80
	return hex.EncodeToString(raw[:]), nil
}

// IsWorkbenchID reports whether a string is a workbench identifier this build
// minted: WorkbenchIDLength lowercase hex characters decoding to a UUID whose
// version field is 7 and whose variant field is 10.
//
// It is deliberately not IsID widened. IsID governs every entity directory in
// the format and has to go on refusing anything wider, so that a workbench
// identifier can never be read as a card, a comment or an attachment, and a
// 12-hex entity identifier can never be read as a workbench. The two
// predicates are disjoint by their lengths alone, which is what makes a
// legacy workbench directory and a migrated one impossible to confuse.
func IsWorkbenchID(s string) bool {
	if len(s) != WorkbenchIDLength {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		digit := c >= '0' && c <= '9'
		letter := c >= 'a' && c <= 'f'
		if !digit && !letter {
			return false
		}
	}
	if s[12] != '7' {
		return false
	}
	switch s[16] {
	case '8', '9', 'a', 'b':
		return true
	}
	return false
}

// ClaimWorkbenchID creates the directory of a fresh workbench inside a
// container, which is the atomic test-and-claim of its identifier exactly as
// ClaimID performs it for an entity: mkdir either wins the name or reports
// that somebody else holds it. It retries on a collision and returns the id
// it claimed.
func ClaimWorkbenchID(container string) (string, error) {
	if err := os.MkdirAll(container, 0o755); err != nil {
		return "", err
	}
	for attempt := 0; attempt < 16; attempt++ {
		id, err := NewWorkbenchID()
		if err != nil {
			return "", err
		}
		err = os.Mkdir(filepath.Join(container, id), 0o755)
		if err == nil {
			return id, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("could not claim a workbench identifier in %s after 16 attempts", container)
}

// ListWorkbenchIDs returns the identifiers of a .dinah container, sorted
// ascending, admitting both widths a workbench directory can carry: the wide
// identifier this build mints, and the 12-hex one a workbench written before
// the container rule still carries until the migration reminents it.
//
// It is separate from ListIDs rather than a widening of it because ListIDs
// governs every entity collection, and those stay 12-hex only. A container is
// the one directory in the format holding names of two widths, so it is the
// one directory that gets its own listing, and a caller asking what a
// container holds asks this rather than reaching for the entity listing.
//
// It answers (nil, nil) for a container that names nothing on disk, because a
// discovery walk visits mostly directories that hold no .dinah at all and a
// migration's first run has not created its container yet. It answers
// (nil, err) whenever something is there and could not be listed, so a caller
// that receives an empty list and a nil error has been told that the container
// really was read and really held nothing.
//
// The existence question is settled by a second os.Stat on the container
// rather than by classifying os.ReadDir's own error, and that is the whole
// point of the two calls. os.ReadDir reports a path where a plain file sits in
// place of the directory through an error that os.IsNotExist and
// errors.Is(err, fs.ErrNotExist) both answer true for on at least one
// supported platform, which is the same answer they give for a container
// nobody ever created. Nothing here depends on that, and the code is written
// so that nothing can come to depend on it: os.ReadDir's error is never
// classified at all, and os.Stat, whose documented contract distinguishes a
// path that does not exist from a path that exists and is not a directory,
// decides the case on every platform alike. listIdentifiers
// (internal/bench/vocabulary.go) classifies os.ReadDir's own error for a
// different collection and carries the same latent gap, so do not copy that
// idiom into this function.
func ListWorkbenchIDs(container string) ([]string, error) {
	entries, err := os.ReadDir(container)
	if err != nil {
		if _, statErr := os.Stat(container); os.IsNotExist(statErr) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() {
			continue
		}
		if !IsID(name) && !IsWorkbenchID(name) {
			continue
		}
		ids = append(ids, name)
	}
	return ids, nil
}
