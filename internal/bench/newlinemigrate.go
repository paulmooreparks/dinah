package bench

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"dinah/internal/contract"
)

// The conditions the plan pass refuses a destination for. Each is a token
// rather than a sentence, on the terms BranchConflictUnreadable is one.
const (
	NewlineConflictUnreadable  = "unreadable"
	NewlineConflictUnwritable  = "unwritable"
	NewlineConflictUnsupported = "unsupported"
	NewlineConflictLocked      = "locked"
)

// NewlineRewrite is one file the run carried, or would carry.
type NewlineRewrite struct {
	Path    string `json:"path"`    // absolute path to the file
	Returns int    `json:"returns"` // line-ending carriage returns the transform removed
	Loose   int    `json:"loose"`   // carriage returns left alone, which are not line endings
	Written bool   `json:"written"` // false on a preview, and on a file the run did not reach
}

// NewlineConflict is one destination the plan pass refused, with the condition
// it met. Every conflict is reported rather than the first, on the terms
// BranchConflict states.
type NewlineConflict struct {
	Path      string `json:"path"`
	Condition string `json:"condition"`
	Detail    string `json:"detail,omitempty"` // the frontmatter key, the record number, or the lock's holder
}

// NewlineMigration is what one run answers.
type NewlineMigration struct {
	Examined  int               `json:"examined"` // text files read
	Rewrites  []NewlineRewrite  `json:"rewrites"`
	Conflicts []NewlineConflict `json:"conflicts"`
	Applied   bool              `json:"applied"` // false on a preview
}

// Clean reports whether the run left anything a person has to act on that
// nothing else reports, which is what decides whether a check that ran this
// repair still exits clean.
//
// A conflict is exactly that, so a conflict is unclean. A rewrite is not, and
// this counted rewrites once and was wrong for it: a confirmed run that
// repaired every destination printed "No structural defects found." and then
// exited non-zero, because the files it had just cleaned were still in the
// count. A preview that found work exits non-zero anyway, since the check
// finding names those same files, so counting rewrites here buys nothing and
// costs the confirmed run its exit code.
//
// That is also what BranchMigration.Clean counts, and this function says so
// deliberately rather than by coincidence: the two migrations answer the same
// situation the same way, which is what the doc comment on MigrateNewlines
// claims when it says the shape is copied.
func (m *NewlineMigration) Clean() bool {
	return m == nil || len(m.Conflicts) == 0
}

// newlineTransform is one file's repair, and it is also that file's detector.
// The output differing from the bytes read is the whole of what makes a file a
// destination, so detection and repair cannot disagree about which files are
// dirty and idempotence is a property of the definition rather than a claim
// about it.
type newlineTransform struct {
	Out     []byte
	Returns int // line-ending carriage returns removed
	Loose   int // carriage returns the transform left alone
	// Condition and Detail are set where the transform itself refuses the
	// file, which is a frontmatter key it cannot re-render and a journal
	// record whose transformed form does not carry what the original did.
	Condition string
	Detail    string
}

// MigrateNewlines repairs every workbench text file storing a carriage return
// that stands for a line ending, which the format's Encoding section forbids a
// writer to produce. It is the repair half of the write-path normalisation
// this build now performs, and it exists because an external editor writing an
// anchor, which is the one site no write-path change can reach, keeps putting
// the condition back.
//
// The signature copies MigrateBranches, including apply, so the flag previews
// without --yes and applies with it.
//
// A preview WRITES. For each destination it rehearses the real write by
// writing the file's own unmodified bytes back over it, because nothing short
// of performing that write predicts whether it will be permitted. The content
// is unchanged and the modification time becomes now, and the report says both
// in its own heading rather than claiming that nothing was written.
//
// A probe of the containing directory is not that prediction, and the reason is
// the one the format document already states under the ordinal migration: every
// write in this format is a temporary renamed over its target, so the right
// that governs is the right to replace a name, POSIX grants that right through
// the containing directory, and Windows asks the file's own attribute instead.
// A directory probe therefore answers for one platform and not the other.
// Measured both ways: a read-only file in a writable directory is replaced
// without complaint on Linux and refused on Windows, and a writable file in a
// read-only directory is refused on Linux and replaced on Windows. Performing
// the write is the only question that is the same question everywhere, which is
// also why nothing here reads a permission bit ahead of it.
//
// A run carrying an unreadable, unwritable or unsupported conflict rewrites
// nothing at all, so a preview predicts the outcome instead of a run
// discovering a refusal half way through. The opposite choice, repairing the
// clean ones and reporting the rest, is the one somebody will otherwise make.
// A locked file is the single exception: it is reported and skipped, the other
// destinations are repaired, and the command says which files were busy. A
// lock is held for milliseconds, a run takes one per file across thousands of
// files and several sessions work a busy workbench at once, so a run that
// refused over a transient lock would refuse most of the time. The repair is
// per file and idempotent, which is what makes skipping safe.
//
// The run as a whole is not atomic across files and this does not pretend
// otherwise. WriteText renames a whole temporary into place, so a file that
// fails to be written keeps its old bytes and no file is ever half written. On
// the first write error the report built so far travels beside the error, with
// Written true on every file rewritten and false on every file not reached.
func (b *Bench) MigrateNewlines(actor, now string, apply bool) (*NewlineMigration, error) {
	report := &NewlineMigration{Applied: apply}
	lock, err := Acquire(b.Root, actor, now)
	if err != nil {
		return report, err
	}
	defer lock.Release()
	paths, err := b.newlineFiles()
	if err != nil {
		return report, err
	}
	report.Examined = len(paths)
	var destinations []string
	for _, path := range paths {
		held, taken, err := b.holdForFile(path, actor, now)
		if err != nil {
			// A file the run would not have touched is not worth reporting
			// busy, and reporting one told a reader to run the command again
			// over a file a second run finds nothing to do with. Whether it
			// was a destination is decided from a read taken without the lock,
			// which is safe because nothing is written on this path and
			// because every write in this format lands by rename, so an
			// unlocked read sees the old bytes or the new ones and never a
			// half-written file. A file that turns dirty a moment later is
			// picked up by the next run, which is what a busy file gets
			// anyway.
			if candidate, readErr := os.ReadFile(path); readErr == nil {
				if result := transformNewlines(path, candidate); result.Condition == "" && bytes.Equal(result.Out, candidate) {
					continue
				}
			}
			report.Conflicts = append(report.Conflicts, b.lockConflict(path, err))
			continue
		}
		original, err := os.ReadFile(path)
		if err != nil {
			release(held, taken)
			report.Conflicts = append(report.Conflicts, NewlineConflict{Path: path, Condition: NewlineConflictUnreadable})
			continue
		}
		result := transformNewlines(path, original)
		if result.Condition != "" {
			release(held, taken)
			report.Conflicts = append(report.Conflicts, NewlineConflict{Path: path, Condition: result.Condition, Detail: result.Detail})
			continue
		}
		if bytes.Equal(result.Out, original) {
			release(held, taken)
			continue
		}
		// The rehearsal's read and its rename sit on the same side of one
		// acquisition, so a concurrent write cannot land between them and be
		// silently reverted by the rename.
		newlinePause(newlinePhaseRehearse, path)
		rehearsal := writeBytes(path, original)
		release(held, taken)
		if rehearsal != nil {
			report.Conflicts = append(report.Conflicts, NewlineConflict{Path: path, Condition: NewlineConflictUnwritable})
			continue
		}
		report.Rewrites = append(report.Rewrites, NewlineRewrite{Path: path, Returns: result.Returns, Loose: result.Loose})
		destinations = append(destinations, path)
	}
	if !apply || poisoned(report.Conflicts) {
		return report, nil
	}
	for index, path := range destinations {
		newlinePause(newlinePhaseWrite, path)
		written, err := b.rewriteNewlines(path, actor, now)
		if err != nil {
			return report, err
		}
		report.Rewrites[index].Written = written
	}
	return report, nil
}

// The two moments a run can be interrupted at from a test. The first is
// between a rehearsal's read and its rename, and the second is before one
// destination's real write.
const (
	newlinePhaseRehearse = "rehearse"
	newlinePhaseWrite    = "write"
)

// newlineHook runs at the two moments above and is nil in a shipped binary.
//
// It exists because two properties of this run cannot be asserted from outside
// it. The lock protocol is one: a test whose hook attempts to take the file's
// own lock during a rehearsal has to be refused, because a hook that succeeds
// proves the rehearsal is not holding it, and that refusal is what a second
// process would meet, so the window a concurrent write could land in does not
// exist. The partial-failure account is the other: a destination has to become
// unwritable after its rehearsal and before its write, which is the state a run
// meets when somebody changes a permission underneath it.
var newlineHook func(phase, path string)

// newlinePause runs the hook where one is set.
func newlinePause(phase, path string) {
	if newlineHook != nil {
		newlineHook(phase, path)
	}
}

// poisoned reports whether the conflicts the plan pass met stop the whole run.
// Every condition but a held lock does: each says the plan is wrong rather than
// that the store is busy, and none of them goes away by running the command
// again.
func poisoned(conflicts []NewlineConflict) bool {
	for _, conflict := range conflicts {
		if conflict.Condition != NewlineConflictLocked {
			return true
		}
	}
	return false
}

// rewriteNewlines performs one file's half of the write pass and reports
// whether it wrote.
//
// The bytes are read again under the file's own lock rather than carried from
// the plan pass, on the discipline writeBranchMigrant's doc comment states for
// its own re-read: the plan pass stood outside that lock for the interval
// between the two passes and the file could have changed. A file whose
// transform is a no-op on the second read is skipped and reported as not
// written.
func (b *Bench) rewriteNewlines(path, actor, now string) (bool, error) {
	held, taken, err := b.holdForFile(path, actor, now)
	if err != nil {
		// A lock another process holds is the skip this repair is built to
		// tolerate. Any other failure to take one is a failure to prepare the
		// write, and reporting it as a file somebody else is busy with would
		// tell a reader to run the command again over a condition running it
		// again cannot clear.
		if lockHeld(err) {
			return false, nil
		}
		return false, err
	}
	defer release(held, taken)
	original, err := os.ReadFile(path)
	if err != nil {
		return false, nil
	}
	result := transformNewlines(path, original)
	if result.Condition != "" || bytes.Equal(result.Out, original) {
		return false, nil
	}
	if err := writeBytes(path, result.Out); err != nil {
		return false, err
	}
	return true, nil
}

// lockConflict reads a failed acquisition and answers the condition it really
// is.
//
// Acquire refuses with contract.Locked when the lock file already stands, and
// answers the underlying error for everything else, and the two mean opposite
// things to a reader. A held lock is transient and the run says to try again; a
// lock that cannot be created at all is the directory refusing a write, which
// is the same condition the rehearsal exists to find and which running the
// command again will meet identically.
//
// The platforms differ here in a way that made this matter. Replacing a file
// through a temporary and a rename is governed by the mode of the DIRECTORY on
// POSIX and by the mode of the FILE on Windows, so a POSIX store whose
// directory refuses a write refuses the lock first and never reaches the
// rehearsal at all.
func (b *Bench) lockConflict(path string, err error) NewlineConflict {
	if !lockHeld(err) {
		return NewlineConflict{Path: path, Condition: NewlineConflictUnwritable}
	}
	return NewlineConflict{Path: path, Condition: NewlineConflictLocked, Detail: LockHolder(filepath.Join(b.lockDirForFile(path), LockName))}
}

// lockHeld reports whether a failed acquisition failed because another process
// holds the lock, rather than because the lock could not be created.
func lockHeld(err error) bool {
	var refusal *contract.Refusal
	return errors.As(err, &refusal) && refusal.Name == contract.Locked
}

// holdForFile takes the lock guarding one workbench file, and reports whether
// it took one at all. A file whose lock directory is the workbench root is
// already covered by the run's own outer acquisition and takes no second lock,
// because Acquire refuses rather than recursing.
func (b *Bench) holdForFile(path, actor, now string) (*Lock, bool, error) {
	dir := b.lockDirForFile(path)
	if dir == b.Root {
		return nil, false, nil
	}
	held, err := Acquire(dir, actor, now)
	if err != nil {
		return nil, false, err
	}
	return held, true, nil
}

// release gives back a lock the run took, and does nothing for a file whose
// lock the run never took because the outer acquisition already covered it.
func release(held *Lock, taken bool) {
	if taken {
		held.Release()
	}
}

// lockDirForFile names the lock that guards a workbench file, which the
// format's concurrency section fixes as the nearest enclosing journal-bearing
// entity: the card's own directory for anything inside a card, that
// workstream's directory for anything inside a workstream, and the workbench
// root for everything else, a column's anchor and the workbench's own included.
// It is the file-path spelling of what lockDirFor answers for an entity.
//
// An archived card's files take the archived card's own directory, which is
// where its lock lives while the directory sits in the archive half.
func (b *Bench) lockDirForFile(path string) string {
	for _, root := range []string{b.CardsRoot(), filepath.Join(b.Root, ArchiveDir, CardsDir), filepath.Join(b.Root, WorkstreamsDir), filepath.Join(b.Root, ArchiveDir, WorkstreamsDir)} {
		relative, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(relative, "..") {
			continue
		}
		segments := strings.Split(filepath.ToSlash(relative), "/")
		if len(segments) < 2 {
			continue
		}
		return filepath.Join(root, segments[0])
	}
	return b.Root
}

// newlineFiles lists the files one run examines: every .md and .ndjson under
// the workbench root, in the live half and in the archive, except those under a
// directory named payload.
//
// The selection is by extension rather than by a list of the anchor names the
// format fixes, because such a list goes stale: the vocabulary already declares
// a retired state.md that a pre-vocabulary workbench still carries, and this
// repair is meant to run against other people's workbenches rather than against
// the one it was written on. The skip by directory name is what keeps a payload
// called card.md a payload, and the extension filter is also what keeps a lock
// file out, since a lock is named lock.
func (b *Bench) newlineFiles() ([]string, error) {
	var paths []string
	err := filepath.WalkDir(b.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == PayloadDir {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(entry.Name()) {
		case ".md", ".ndjson":
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// transformNewlines computes one file's repair, choosing the branch by what the
// file is rather than by what it is named.
//
// A .ndjson takes the journal branch. A .md takes the anchor branch exactly
// when its parse round-trips, and the whole-file branch otherwise; a .md
// carrying one of the anchor names the format fixes and failing that gate is a
// damaged store rather than somebody's note, so it is refused. Everything else
// takes the whole-file branch.
//
// The gate is a round trip rather than a test of how the file starts, and the
// difference is not cosmetic. A Markdown note whose first line is a horizontal
// rule opens with a fence, and ParseAnchor then drops every header line that is
// not a key, so such a note comes back with the prose between its first two
// rules gone. Because the transform is also the detector, such a file would
// select itself for repair and the confirmed run would write the mangled form.
// The gate errs toward the whole-file branch, which can only reduce a CRLF pair
// to LF and can never move a line, so a file it misroutes is under-repaired
// rather than damaged.
func transformNewlines(path string, data []byte) newlineTransform {
	if filepath.Ext(path) == ".ndjson" {
		return transformJournal(data)
	}
	if filepath.Ext(path) != ".md" {
		return transformWholeFile(data)
	}
	text := string(data)
	fm, body := ParseAnchor(text)
	if fm.Render(body) != NormalizeNewlines(text) {
		if isFixedAnchorName(filepath.Base(path)) {
			return newlineTransform{Out: data, Condition: NewlineConflictUnsupported, Detail: "header does not round-trip"}
		}
		return transformWholeFile(data)
	}
	return transformAnchor(fm, body, data)
}

// isFixedAnchorName reports whether a file name is one the format fixes for an
// anchor, which is the question that decides whether a .md failing the
// round-trip gate is a damaged store or somebody's note.
//
// The mounted kinds are asked of the containment grammar rather than listed
// here, so an anchor a later kind brings with it is covered without this
// function being revisited. Three names the grammar does not mount are added:
// the workbench's own anchor and a workstream's, which no collection contains,
// and the retired state.md a pre-vocabulary workbench still carries.
func isFixedAnchorName(name string) bool {
	if _, mounted := KindOfAnchor(name); mounted {
		return true
	}
	switch name {
	case WorkbenchAnchor, WorkstreamAnchor, PreVocabularyAnchor:
		return true
	}
	return false
}

// transformWholeFile is NormalizeNewlines over the file and nothing else. It
// cannot move a line or lose one, which is why the anchor gate errs toward it.
func transformWholeFile(data []byte) newlineTransform {
	out := []byte(NormalizeNewlines(string(data)))
	return newlineTransform{
		Out:     out,
		Returns: bytes.Count(data, []byte("\r")) - bytes.Count(out, []byte("\r")),
		Loose:   bytes.Count(out, []byte("\r")),
	}
}

// transformAnchor repairs one anchor, through the readers and writers this
// codebase already has rather than through any search for a byte.
//
// ParseAnchor strips a trailing carriage return from every line it reads,
// header lines and body alike, which disposes of a CRLF pair without it being
// looked for, and its unquote turns a frontmatter value's escaped line feed
// back into a real one, which is what exposes the split form a frontmatter
// scalar stores: a raw carriage return followed by the two characters backslash
// and n, which carries no CRLF pair at all. A key whose parsed value carries a
// pair after that is re-set through quote, which normalises. Every other key's
// stored lines are left exactly as they are, so a key the tool has never heard
// of is neither re-quoted nor re-ordered, and neither is a key whose only
// carriage returns are loose ones.
//
// A key whose stored lines carry a carriage return and whose shape is neither a
// scalar nor a block of dashed entries refuses the file, with its own name in
// the detail. That errs toward refusing, a lone carriage return in such a block
// included, because a shape this repair cannot re-render is a shape it cannot
// repair either, and it would rather say so than guess. No such value is known
// to exist: every nested block this tool writes renders its scalars through
// quote.
func transformAnchor(fm *Frontmatter, body string, data []byte) newlineTransform {
	for _, key := range fm.Keys() {
		raw := fm.Raw(key)
		if !strings.Contains(strings.Join(raw, "\n"), "\r") {
			continue
		}
		scalar := fm.Value(key)
		if len(raw) == 1 && !isFlowSequence(scalar) {
			verbatim := key + ": " + quoteVerbatim(scalar)
			if verbatim == key+": "+quote(scalar) {
				// Every carriage return this value carries is a loose one,
				// which the repair leaves exactly where it is.
				continue
			}
			if raw[0] != verbatim {
				return newlineTransform{Out: data, Condition: NewlineConflictUnsupported, Detail: key}
			}
			fm.Set(key, scalar)
			continue
		}
		if items, dashed := dashedSequence(fm, key, raw); dashed {
			if !carriesPair(items) {
				continue
			}
			fm.SetSeq(key, items)
			continue
		}
		return newlineTransform{Out: data, Condition: NewlineConflictUnsupported, Detail: key}
	}
	out := []byte(fm.Render(body))
	return newlineTransform{
		Out:     out,
		Returns: bytes.Count(data, []byte("\r")) - bytes.Count(out, []byte("\r")),
		Loose:   bytes.Count(out, []byte("\r")),
	}
}

// isFlowSequence reports whether a scalar line is a sequence written inline,
// which the anchor branch refuses rather than guessing at: re-setting it as a
// scalar would quote the whole bracketed form as one value, and re-setting it
// as a sequence would change the shape the file chose. No such value carrying a
// carriage return is known to exist.
func isFlowSequence(value string) bool {
	return strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]")
}

// carriesPair reports whether any entry of a sequence carries a pair this
// repair reduces, so a sequence whose only carriage returns are loose ones is
// left alone rather than re-rendered.
func carriesPair(items []string) bool {
	for _, item := range items {
		if strings.Contains(item, crlf) {
			return true
		}
	}
	return false
}

// dashedSequence answers a key's entries when its stored lines are a block of
// dashed entries a plain re-render reproduces, which is the one non-scalar
// shape this repair can re-set.
func dashedSequence(fm *Frontmatter, key string, raw []string) ([]string, bool) {
	if len(raw) < 2 || raw[0] != key+":" {
		return nil, false
	}
	items := fm.Seq(key)
	if len(items) != len(raw)-1 {
		return nil, false
	}
	for i, item := range items {
		if raw[i+1] != "  - "+quoteVerbatim(item) {
			return nil, false
		}
	}
	return items, true
}

// transformJournal repairs one journal, record by record.
//
// Two things put a carriage return in a journal and they need different
// answers. A record separator written as CRLF by an editor or by a misconfigured
// version-control filter sits outside every string literal, so a transform that
// looked only inside them left such a file dirty and reported it clean; it is
// answered by stripping the separator's own carriage return. A carriage return
// inside a record is stored as an escape and carries no 0x0D at all; it is
// answered by decoding.
//
// The repair spells no escape of its own. JSON admits the two characters
// backslash and r and a six-character unicode escape naming the same code
// point, and a line feed likewise, so four byte sequences spell one pair and
// any code enumerating them is correct only by accident. Decoding makes the
// question not arise.
//
// A trailing record that does not decode is a torn tail, which this format
// expects and ReadJournal already tolerates by design, so its bytes are copied
// verbatim and the file is not refused over it. A record that does not decode
// and is not the last is a damaged store, which is a different piece of work
// from deciding what a line ending should be, so the file is refused.
func transformJournal(data []byte) newlineTransform {
	records := strings.SplitAfter(string(data), "\n")
	if len(records) > 0 && records[len(records)-1] == "" {
		records = records[:len(records)-1]
	}
	final := lastRecordIndex(records)
	var out strings.Builder
	result := newlineTransform{}
	for index, record := range records {
		content, terminator := splitRecordTerminator(record)
		if strings.TrimSpace(content) == "" {
			out.WriteString(NormalizeNewlines(content))
			out.WriteString(NormalizeNewlines(terminator))
			result.Loose += strings.Count(NormalizeNewlines(content+terminator), "\r")
			continue
		}
		if index == final && !decodesAsJSON(content) {
			// The torn tail a crash left. Its bytes are copied exactly as
			// ReadJournal reads past them.
			out.WriteString(content)
			out.WriteString(terminator)
			result.Loose += strings.Count(content+terminator, "\r")
			continue
		}
		if !decodesAsJSON(content) {
			return newlineTransform{Out: data, Condition: NewlineConflictUnsupported, Detail: "record " + strconv.Itoa(index+1)}
		}
		repaired, returns, loose, refusal := transformRecord(content)
		if refusal != "" {
			return newlineTransform{Out: data, Condition: NewlineConflictUnsupported, Detail: "record " + strconv.Itoa(index+1) + ": " + refusal}
		}
		if verified := verifyRecord(content, repaired); !verified {
			return newlineTransform{Out: data, Condition: NewlineConflictUnsupported, Detail: "record " + strconv.Itoa(index+1)}
		}
		normalizedTerminator := NormalizeNewlines(terminator)
		result.Returns += returns + strings.Count(terminator, "\r") - strings.Count(normalizedTerminator, "\r")
		result.Loose += loose + strings.Count(normalizedTerminator, "\r")
		out.WriteString(repaired)
		out.WriteString(normalizedTerminator)
	}
	result.Out = []byte(out.String())
	return result
}

// splitRecordTerminator parts one record's own bytes from the line terminator
// that follows it, which is what lets the separator be repaired without the
// record's bytes being touched.
func splitRecordTerminator(record string) (string, string) {
	if !strings.HasSuffix(record, "\n") {
		return record, ""
	}
	content := strings.TrimSuffix(record, "\n")
	if strings.HasSuffix(content, "\r") {
		return strings.TrimSuffix(content, "\r"), "\r\n"
	}
	return content, "\n"
}

// lastRecordIndex answers the index of the final record a journal carries,
// which is the one place a torn record is tolerated.
//
// A journal is appended to a line at a time, so a crash can tear the final line
// and no other, which is what ReadJournal already tolerates by design and what
// check.torn-journal carries its own repair for. A record that does not decode
// anywhere else is a damaged store rather than a torn tail, and deciding what a
// damaged record should become is a different piece of work from deciding what
// a line ending should be.
func lastRecordIndex(records []string) int {
	for index := len(records) - 1; index >= 0; index-- {
		content, _ := splitRecordTerminator(records[index])
		if strings.TrimSpace(content) != "" {
			return index
		}
	}
	return -1
}

// decodesAsJSON reports whether one record is a JSON document this build can
// read at all, which is the question that separates a torn record from a whole
// one.
func decodesAsJSON(content string) bool {
	var probe any
	return json.Unmarshal([]byte(content), &probe) == nil
}

// transformRecord rewrites one record's string literals, and answers the
// refusal the literal walk raises rather than a boolean, so the report can say
// which literal stopped it.
//
// A literal is located by RFC 8259's string grammar, which is that a literal
// runs from an unescaped quotation mark to the next one and that a backslash
// inside it escapes the character after it. Its value is then read by
// json.Unmarshal, and only a literal whose value the normalisation changes is
// re-encoded; every other literal's original bytes are copied, so no spelling
// anywhere in the record is disturbed by a repair elsewhere in it.
func transformRecord(record string) (string, int, int, string) {
	var out strings.Builder
	returns, loose := 0, 0
	for i := 0; i < len(record); {
		if record[i] != '"' {
			if record[i] == '\r' {
				loose++
			}
			out.WriteByte(record[i])
			i++
			continue
		}
		end := literalEnd(record, i)
		if end < 0 {
			out.WriteString(record[i:])
			return out.String(), returns, loose, ""
		}
		literal := record[i:end]
		i = end
		var value string
		if err := json.Unmarshal([]byte(literal), &value); err != nil {
			out.WriteString(literal)
			continue
		}
		normalized := NormalizeNewlines(value)
		if normalized == value {
			out.WriteString(literal)
			loose += strings.Count(value, "\r")
			continue
		}
		// A decode that replaced something cannot be re-encoded without
		// losing what it replaced, and the two cannot be told apart by
		// reading the decoded value back through the same decoder, which is
		// how a lone surrogate came back as the replacement character with
		// the check reporting success. json.Unmarshal documents that invalid
		// UTF-8 and invalid UTF-16 surrogate pairs are replaced by U+FFFD
		// rather than refused, so a decoded value carrying no U+FFFD carries
		// no replacement and is safe to re-encode. A literal carrying one is
		// refused rather than guessed at: it may be prose that genuinely
		// carries the character, and this repair has no way to tell.
		if strings.ContainsRune(value, utf8.RuneError) {
			return "", 0, 0, "a string carrying the replacement character cannot be re-encoded"
		}
		encoded, err := json.Marshal(normalized)
		if err != nil {
			return "", 0, 0, "a string will not re-encode"
		}
		returns += strings.Count(value, "\r") - strings.Count(normalized, "\r")
		loose += strings.Count(normalized, "\r")
		out.Write(encoded)
	}
	return out.String(), returns, loose, ""
}

// literalEnd answers the index just past the closing quotation mark of the
// literal opening at start, and a negative number for a literal the record
// never closes.
func literalEnd(record string, start int) int {
	for i := start + 1; i < len(record); i++ {
		switch record[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return -1
}

// verifyRecord checks the repair rather than trusting it: the transformed
// record has to decode to the original record with every string it carries
// normalised, at any depth. Re-encoding a literal that changed can re-spell
// escapes inside that one literal, since json.Marshal writes the encoder's own
// spelling rather than the document's, and those are changes to bytes and not
// to the record. This is the check that says so for every record rather than an
// argument that says so in prose.
//
// The comparison is reflect.DeepEqual rather than one written here. What
// json.Unmarshal produces into an any is a closed set, a map, a slice, a
// string, a float64, a bool and nil, and DeepEqual answers every one of them,
// so a hand-written comparison would duplicate the standard library for no
// reading a reader gains. This package already imports reflect for the event
// walk.
func verifyRecord(original, repaired string) bool {
	var before, after any
	if err := json.Unmarshal([]byte(original), &before); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(repaired), &after); err != nil {
		return false
	}
	return reflect.DeepEqual(normalizeDecoded(before), after)
}

// normalizeDecoded answers a decoded document with every string it carries, at
// any depth, normalised.
func normalizeDecoded(value any) any {
	switch typed := value.(type) {
	case string:
		return NormalizeNewlines(typed)
	case []any:
		out := make([]any, len(typed))
		for i, element := range typed {
			out[i] = normalizeDecoded(element)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for name, element := range typed {
			out[name] = normalizeDecoded(element)
		}
		return out
	}
	return value
}

// newlineCounts renders the detail the two counting findings carry: what the
// file's own transform would remove, and separately what it would leave alone.
// The second number is legal and the first is not, which is why they are
// reported apart rather than summed.
func newlineCounts(result newlineTransform) string {
	return strconv.Itoa(result.Returns) + " line-ending, " + strconv.Itoa(result.Loose) + " loose"
}

// checkStoredNewlines reports every workbench text file storing a carriage
// return that stands for a line ending.
//
// It shares the migration's file selection, its raw read and its transforms,
// which is what makes the finding and the repair incapable of disagreeing about
// which files are dirty, and it reports whether or not --migrate-newlines was
// asked for. It takes no locks and writes nothing, because it only reads.
//
// The read is os.ReadFile and never bench.ReadText. ReadText now normalises,
// so routing this sweep through it would leave a sweep that cannot fail, which
// is worse than no sweep.
//
// A file whose only carriage returns are loose ones is reported with a
// line-ending count of zero and is not a destination. Reporting it costs a line
// and tells a reader that the bytes were looked at, and refusing to rewrite it
// is what keeps the decision to leave a lone carriage return alone true.
func (b *Bench) checkStoredNewlines() ([]Finding, error) {
	paths, err := b.newlineFiles()
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		result := transformNewlines(path, data)
		// Three shapes reach this sweep and they are three different things to
		// tell a person, so each gets a finding whose sentence is true of it.
		// One key covered all three once, and its sentence said the file stores
		// a carriage return standing for a line ending, which is false of a
		// file the repair will not decide (one was reported carrying zero
		// carriage returns of any kind) and false of a file whose only
		// carriage returns are the loose ones this card decided to keep.
		switch {
		case result.Condition == NewlineConflictUnsupported:
			findings = append(findings, Finding{Path: path, Key: FindingNewlineRepairUnsupported, Detail: result.Detail})
		case result.Returns > 0:
			findings = append(findings, Finding{Path: path, Key: FindingStoredCarriageReturn, Detail: newlineCounts(result)})
		case result.Loose > 0:
			findings = append(findings, Finding{Path: path, Key: FindingLooseCarriageReturn, Detail: newlineCounts(result)})
		}
	}
	return findings, nil
}
