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

// NormalizeNewlines answers the text with every run of carriage returns that
// ends at a line feed reduced to that line feed. A carriage return not followed
// by a line feed is left alone: it is not a line ending under this format, and
// stripping it would destroy a character the prose meant to carry.
//
// The unit is the whole run rather than one pair, and that is the difference
// between this function being true of its own answer and not. A single
// non-overlapping replacement of the pair consumes the carriage return adjacent
// to the line feed and leaves the one in front of it sitting against the new
// line feed, forming a fresh pair the pass never revisits: CR CR LF came back
// as CR LF, so the writer stored the very thing this normalisation exists to
// stop it storing, and the repair took one run per carriage return to clean a
// file while reporting each run as a success.
//
// Reducing the run instead is a fixed point by construction. Its output carries
// no carriage return immediately before a line feed, so running it again can
// find nothing to do, and every claim of idempotence made of the repair rests
// on that rather than on a promise.
//
// A run of two or more is prose no reader of this format could return intact
// anyway. SplitLines strips one trailing carriage return per line, so CR CR LF
// already reads back as a line carrying one carriage return, and rendering that
// line again puts a fresh pair on disk. The only way to store such a run and
// read it back unchanged is for the format to stop treating a trailing carriage
// return as part of the line ending, which is a different contract from this
// one.
func NormalizeNewlines(text string) string {
	if !strings.Contains(text, crlf) {
		return text
	}
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(text); i++ {
		if text[i] != '\r' {
			out.WriteByte(text[i])
			continue
		}
		run := i
		for run < len(text) && text[run] == '\r' {
			run++
		}
		if run < len(text) && text[run] == '\n' {
			// The whole run is the line ending, and the line feed is what it
			// reduces to.
			out.WriteByte('\n')
			i = run
			continue
		}
		// Every carriage return in the run is prose, because none of them ends
		// at a line feed.
		out.WriteString(text[i:run])
		i = run - 1
	}
	return out.String()
}

// crlf is the smallest run of carriage returns that ends at a line feed, which
// makes it the cheap test for whether a text carries anything to reduce: every
// longer run contains it, so a text not containing it has no line ending of the
// kind this format forbids a writer to produce.
//
// It is not what NormalizeNewlines reduces. That is the whole run, and reducing
// the pair instead is the defect this constant's own name is a reminder of: a
// non-overlapping replacement of it leaves a fresh one behind. Nothing here
// should replace this value; it is read, never written with.
const crlf = "\r\n"

// ReadText reads a text file, strips a byte-order mark and normalises its line
// endings. The format writes UTF-8 without a mark and with LF everywhere; a
// mark or a CRLF reaching the tree came from an editor, and the format's
// encoding rule is that a reader tolerates both rather than failing.
//
// Normalising here is what makes the three readers of a column's instructions
// agree: `dinah get` and `dinah instructions` reach the text through
// ParseAnchor, which strips, while `dinah show` returns this function's answer
// straight, and before this line one field had two spellings depending on
// which reader asked.
//
// A caller that has to see a file's stored bytes, which is the newline
// migration and the check finding behind it, reads with os.ReadFile rather
// than with this function, because this function strips the very condition
// those two exist to find.
func ReadText(path string) (string, error) {
	return readText(Disk{}, path)
}

// ReadText is the free ReadText read through this bench's source.
func (b *Bench) ReadText(path string) (string, error) {
	return readText(b.source(), path)
}

// readText is ReadText's body, reading through src.
func readText(src Source, path string) (string, error) {
	observeAnchor(path)
	text, _, err := src.Text(path)
	if err != nil {
		return "", err
	}
	return text, nil
}

// WriteText writes a text file, normalising its line endings first, which is
// what makes "writers emit LF everywhere" true of the writer rather than true
// of whoever remembered to call a helper.
//
// The write goes through a temporary beside the destination and a rename, so a
// reader sees either the old bytes or the new ones and never a half-written
// file. This is the write half of the format's concurrency answer.
func WriteText(path, text string) error {
	return writeBytes(path, []byte(NormalizeNewlines(text)))
}

// writeBytes is WriteText's temporary-and-rename mechanism with no
// normalisation, for the one caller that has to write a file's own bytes back
// unchanged. It stays unexported so that normalisation cannot be bypassed from
// outside this package.
//
// The newline migration is that caller. Its plan pass rehearses each
// destination's real write by renaming the file's own bytes back over it,
// because nothing short of performing the rename predicts whether the rename
// will be permitted.
func writeBytes(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".dinah-*")
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
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// Retrying a refused folder move or removal.
//
// Windows refuses to rename or remove a directory while another process holds
// a handle open below it, and a reader holding a file open for the length of
// one read is enough: the rename answers ERROR_ACCESS_DENIED or
// ERROR_SHARING_VIOLATION, and a removal whose files were only marked for
// deletion answers ERROR_DIR_NOT_EMPTY, since DeleteFile's documentation says
// it "marks a file for deletion on close" and the file stays until the last
// handle to it closes. A structural act writes inside the directory it is
// about to move, so any reader that watches for changes (dinah serve's
// resident workbench, an editor, the search indexer) is inside that directory
// at the moment the rename runs. The refusal is transient, because the
// reader's handle closes when its read ends, so renameFolder and removeFolder
// try again for a bounded time before they give the refusal back.
//
// The budget is one second from the first refusal, in pauses that start at
// 2ms and double up to 50ms, which is about twenty attempts. A reader's read
// takes milliseconds, so a refusal lasting the whole budget is a handle held
// open on purpose, and the act then fails as it always did: the error reaches
// the act, which reports it as the interruption it has always reported. No
// other error is retried, and outside Windows nothing is, because a POSIX
// rename or unlink is not refused by an open handle.

// folderRetryBudget is how long a transient refusal is retried, measured from
// the first refusal. A variable, so a test can wait less than the whole of it.
var folderRetryBudget = time.Second

// folderRetryFirstPause and folderRetryPauseCap bound the pause between
// attempts, which doubles from the first to the cap.
const (
	folderRetryFirstPause = 2 * time.Millisecond
	folderRetryPauseCap   = 50 * time.Millisecond
)

// renameFolder is os.Rename for a directory an act moves, retrying a
// transient refusal within folderRetryBudget.
func renameFolder(from, to string) error {
	return retryRefused(func() error { return os.Rename(from, to) }, transientRenameRefusal)
}

// removeFolder is os.RemoveAll for a directory an act removes, retrying a
// transient refusal within folderRetryBudget. RemoveAll is safe to repeat:
// whatever an earlier attempt removed is gone, and the next removes the rest.
func removeFolder(path string) error {
	return retryRefused(func() error { return os.RemoveAll(path) }, transientRemoveRefusal)
}

// retryRefused runs op, and again after each pause while it answers an error
// transient reports and the budget has not run out. It answers op's last
// error, unchanged, so a caller sees what it would have seen without the
// retry.
func retryRefused(op func() error, transient func(error) bool) error {
	err := op()
	if err == nil || !transient(err) {
		return err
	}
	deadline := time.Now().Add(folderRetryBudget)
	pause := folderRetryFirstPause
	for time.Now().Before(deadline) {
		time.Sleep(pause)
		if err = op(); err == nil || !transient(err) {
			return err
		}
		pause = min(2*pause, folderRetryPauseCap)
	}
	return err
}

// Revision is the opaque revision of an anchor file: the content hash read
// under the card lock, which is what a basis names. Callers never parse it,
// because the remote arbiter will compute its own revision another way.
func Revision(path string) (string, error) {
	return revision(Disk{}, path)
}

// Revision is the free Revision read through this bench's source.
func (b *Bench) Revision(path string) (string, error) {
	return revision(b.source(), path)
}

// revision is Revision's body, reading through src.
func revision(src Source, path string) (string, error) {
	observeAnchor(path)
	_, rev, err := src.Text(path)
	if err != nil {
		return "", err
	}
	return rev, nil
}

// readTextAndRevision reads a file once and answers its text, normalised as
// ReadText normalises it, together with the revision of its stored bytes,
// which is what Revision would answer for the same bytes. loadCard reads a
// card's anchor through it, so the text and the revision a card carries come
// from one read.
//
// The revision is hashed over the raw bytes rather than over the normalised
// text. A basis names the revision Revision answers, and hashing the
// normalised text would change it for every anchor stored with CRLF line
// endings or a byte-order mark, so a basis a caller already holds would stop
// matching the card it was taken on.
func readTextAndRevision(src Source, path string) (text, revision string, err error) {
	observeAnchor(path)
	return src.Text(path)
}

// AnchorReadObserver is a seam over the anchor reads, so a test can count the
// files a composition opens rather than infer them from what it answered. When
// it is not nil, ReadText, Revision and readTextAndRevision call it with every
// path they are about to read, and it changes nothing else.
//
// It is exported on the terms ListIDsObserver is: the compositions whose reads
// are counted live in package verb. A test setting it restores it and declares
// itself non-parallel, because it is package state.
var AnchorReadObserver func(path string)

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

// readCollection lists a directory, separating a collection this format has
// not written yet from one that exists and will not read. The existence
// question is settled by a second os.Stat made only after the read has
// failed, because os.ReadDir's own error reports a plain file sitting where a
// directory belongs as not-existing on at least one supported platform, which
// is the same answer it gives for a path nobody created. os.Stat's documented
// contract separates the two on every platform alike, so nothing here rests
// on how any particular platform spells a read failure.
//
// It is the one place in the shipped binary that decides whether a
// directory-read failure means absence, and every collection reader in this
// package goes through it rather than classifying os.ReadDir's error again.
func readCollection(src Source, dir string) ([]os.DirEntry, error) {
	entries, err := src.ReadDir(dir)
	if err != nil {
		if _, statErr := src.Stat(dir); os.IsNotExist(statErr) {
			return nil, nil
		}
		return nil, err
	}
	return entries, nil
}

// ListIDsObserver is a seam over the collection read, so a test can count the
// directory listings a composition performs rather than infer them from the
// numbers it answered. When it is not nil, ListIDs calls it with every
// collection directory it is asked for, and it changes nothing else.
//
// It is exported for the reason verb.ExecutablePath is: the composition whose
// listings are counted is Library.view, which lives in another package, and an
// unexported seam would put that count out of its reach. A test setting it
// restores it and declares itself non-parallel, because it is package state.
var ListIDsObserver func(collection string)

// ListIDs returns the identifiers of a collection directory, sorted
// ascending, ignoring anything that is not a hex directory. An absent
// collection is an empty one, which is the absent-means-empty rule, and it is
// answered with a nil slice and a nil error. A collection that is there and
// will not read is answered with the error, so a caller receiving an empty
// list and a nil error has been told the collection really was read.
func ListIDs(collection string) ([]string, error) {
	return listIDs(Disk{}, collection)
}

// ListIDs is the free ListIDs read through this bench's source.
func (b *Bench) ListIDs(collection string) ([]string, error) {
	return listIDs(b.source(), collection)
}

// listIDs is ListIDs's body, reading through src.
func listIDs(src Source, collection string) ([]string, error) {
	if ListIDsObserver != nil {
		ListIDsObserver(collection)
	}
	if lister, ok := src.(idLister); ok {
		if ids, held := lister.HeldIDs(collection); held {
			return ids, nil
		}
	}
	entries, err := readCollection(src, collection)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() || !IsID(entry.Name()) {
			continue
		}
		ids = append(ids, entry.Name())
	}
	return ids, nil
}

// idLister is what a source that holds its listings may also offer: the
// identifiers of a collection as ListIDs would answer them, computed once per
// listing it holds. held false sends the caller to the ordinary read. A
// resident snapshot offers it, because reading every card's collections on
// every request would otherwise test every name of every listing each time.
type idLister interface {
	HeldIDs(collection string) (ids []string, held bool)
}

// Exists reports whether a path is present.
func Exists(path string) bool {
	return exists(Disk{}, path)
}

// Exists is the free Exists read through this bench's source.
func (b *Bench) Exists(path string) bool {
	return exists(b.source(), path)
}

// exists is Exists's body, reading through src.
func exists(src Source, path string) bool {
	_, err := src.Stat(path)
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
// migration's first run has not created its container yet.
//
// It answers (nil, err) whenever something is there and could not be listed,
// so a caller that receives an empty list and a nil error has been told that
// the container really was read and really held nothing. The read and the
// existence discrimination are performed by readCollection, which carries the
// platform reasoning for every collection reader in this package.
func ListWorkbenchIDs(container string) ([]string, error) {
	return listWorkbenchIDs(Disk{}, container)
}

// listWorkbenchIDs is ListWorkbenchIDs's body, reading through src.
func listWorkbenchIDs(src Source, container string) ([]string, error) {
	entries, err := readCollection(src, container)
	if err != nil {
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
