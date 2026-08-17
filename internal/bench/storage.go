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
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
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
