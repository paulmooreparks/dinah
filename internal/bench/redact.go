package bench

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/durable"
)

// RedactLeftoverSuffix ends the name of the file dinah redact composes a
// journal's new content in, beside the journal, before renaming it over the
// journal. One found standing is what a crash before the rename left, and is
// stale.
const RedactLeftoverSuffix = ".redact"

// RedactTarget is the member a redaction rewrites and the journal its lines
// stand in: a card's journal for a card comment, an item and an item comment,
// and the workbench's for a column comment.
type RedactTarget struct {
	// Kind is KindComment or KindItem.
	Kind string
	// ID is the member's identifier.
	ID string
	// Item is the item an item comment hangs on, empty otherwise.
	Item string
	// Column and ColumnTitle name the column a column comment hangs on.
	Column      string
	ColumnTitle string
	// Journal is the journal's path, and LockDir the directory whose lock
	// guards it.
	Journal string
	LockDir string
}

// RedactionAccount is what a redaction rewrites, or rewrote.
type RedactionAccount struct {
	// Own is how many of the member's own lines carry its text.
	Own int
	// Legacy is how many legacy answer lines carry a version of it.
	Legacy int
	// Journal is the journal the lines stand in.
	Journal string
	// Leftover is a stale journal.ndjson.redact the run found beside the
	// journal and removed, empty where there was none.
	Leftover string
}

// Lines is how many lines the redaction rewrites, which its redacted line
// records.
func (a RedactionAccount) Lines() int {
	return a.Own + a.Legacy
}

// redactStep, when set, is called with the path of the composed journal just
// before it is renamed over the journal, and an error it answers is taken as
// the rename failing there. Only tests set it.
var redactStep func(path string) error

// SetRedactStepForTest plants step as the failure a test drives dinah redact
// into just before its rename, and takes it out again when the test ends. It
// sets a process-global hook, so a test calling it must not run in parallel
// with one that redacts.
func SetRedactStepForTest(t testing.TB, step func(path string) error) {
	t.Helper()
	previous := redactStep
	redactStep = step
	t.Cleanup(func() { redactStep = previous })
}

// redactedDigest matches a value a redaction already replaced.
var redactedDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// redactedText is the replacement a redaction writes for one text.
func redactedText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// FindRedactTarget looks a member up by identifier in the journal its holder
// keeps, which is how a redaction reaches a member no position names any
// longer, a deleted one among them. card is the card the member belongs
// to, or nil for a column comment, whose column column identifies, live or
// archived. It answers false where no line of that journal ever created the
// member.
func (b *Bench) FindRedactTarget(card *Card, column, kind, id string) (*RedactTarget, bool, error) {
	target := &RedactTarget{Kind: kind, ID: id}
	switch {
	case card != nil:
		target.Journal, target.LockDir = card.JournalPath(), card.Dir
	case column != "" && kind == KindComment:
		target.Journal, target.LockDir = b.JournalPath(), b.Root
		target.Column, target.ColumnTitle = column, b.columnTitleAnyHalf(column)
	default:
		return nil, false, nil
	}
	events, _, err := ReadJournal(target.Journal)
	if err != nil {
		return nil, false, err
	}
	for _, ev := range events {
		switch {
		case kind == KindComment && (ev.Event == contract.EventCommented || ev.Event == contract.EventCommentBaseline) && ev.Comment == id:
			if card == nil && ev.Column != column {
				return nil, false, nil
			}
			target.Item = ev.Item
			return target, true, nil
		case kind == KindItem && (ev.Event == contract.EventItemFiled || ev.Event == contract.EventItemBaseline) && ev.Item == id:
			return target, true, nil
		}
	}
	return nil, false, nil
}

// Redact rewrites every line of one member's journal that carries the
// member's text, replacing each text with its SHA-256, and records the act in
// a redacted line written in the same replacement of the journal. With write
// false it composes the same account and writes nothing.
//
// held guards the journal's directory where write is true, and a run that
// writes nothing takes no lock and may be handed none. A stale
// journal.ndjson.redact beside the journal is removed first, by a writing run,
// and named in the account. The run is refused
// dinah.torn-sidecar-present while a torn-tail sidecar stands beside the
// journal, and on a journal whose tail is torn, since a quarantined fragment
// may hold the text and cannot be parsed to find it; and
// dinah.already-redacted where the journal already records a redaction of
// the member.
//
// The new content goes to journal.ndjson.redact and is flushed, then renamed
// over the journal, so the journal holds either the old content or the new.
// A failure before the rename removes the composed file and leaves the
// journal as it was.
func Redact(held *Lock, target RedactTarget, template Event, write bool) (RedactionAccount, error) {
	account := RedactionAccount{Journal: target.Journal}
	if write && !held.guards(filepath.Dir(target.Journal)) {
		return account, contract.Refuse(contract.JournalUnlocked, target.Journal)
	}
	leftover := target.Journal + RedactLeftoverSuffix
	if Exists(leftover) && write {
		if err := durable.Remove(leftover); err != nil {
			return account, err
		}
		account.Leftover = leftover
	}
	if sidecars := TornSidecars(filepath.Dir(target.Journal)); len(sidecars) > 0 {
		return account, contract.Refuse(contract.TornSidecarPresent, sidecars[0])
	}
	raw, err := durable.ReadFile(target.Journal)
	if err != nil && !os.IsNotExist(err) {
		return account, err
	}
	lines := bytes.Split(raw, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	} else if len(lines) > 0 && !decodesAsObject(lines[len(lines)-1]) {
		return account, contract.Refuse(contract.TornSidecarPresent, target.Journal)
	}
	events := make([]*Event, len(lines))
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			return account, contract.Refuse(contract.JournalUnreadable, target.Journal+":"+strconv.Itoa(i+1))
		}
		events[i] = &ev
	}
	plan := planRedaction(events, target)
	if plan.already {
		return account, contract.Refuse(contract.AlreadyRedacted, target.ID)
	}
	account.Own, account.Legacy = plan.own, plan.legacy
	if !write {
		return account, nil
	}
	var out bytes.Buffer
	for i, line := range lines {
		if rewritten, ok := plan.rewritten[i]; ok {
			encoded, err := EncodeEvent(rewritten)
			if err != nil {
				return account, err
			}
			out.Write(encoded)
		} else {
			out.Write(line)
		}
		out.WriteByte('\n')
	}
	record := template
	record.Event = contract.EventRedacted
	record.Kind = target.Kind
	record.Lines = account.Lines()
	switch target.Kind {
	case KindComment:
		record.Comment, record.Item = target.ID, target.Item
		record.Column, record.ColumnTitle = target.Column, target.ColumnTitle
	case KindItem:
		record.Item = target.ID
	}
	if strings.TrimSpace(record.Actor.Name) == "" {
		return account, contract.Refuse(contract.NoOwner, "")
	}
	encoded, err := EncodeEvent(record)
	if err != nil {
		return account, err
	}
	out.Write(encoded)
	out.WriteByte('\n')
	if err := durable.WriteFile(leftover, out.Bytes(), 0o644); err != nil {
		return account, err
	}
	if redactStep != nil {
		if err := redactStep(leftover); err != nil {
			durable.Remove(leftover)
			return account, err
		}
	}
	if err := durable.Replace(leftover, target.Journal); err != nil {
		durable.Remove(leftover)
		return account, err
	}
	return account, nil
}

// RemoveRedactLeftover removes a stale journal.ndjson.redact beside a journal,
// under the lock of the journal's entity, and reports whether one stood there.
func RemoveRedactLeftover(held *Lock, journal string) (bool, error) {
	leftover := journal + RedactLeftoverSuffix
	if !Exists(leftover) {
		return false, nil
	}
	if !held.guards(filepath.Dir(journal)) {
		return false, contract.Refuse(contract.JournalUnlocked, journal)
	}
	return true, durable.Remove(leftover)
}

// MemberRedaction answers who redacted a comment or an item of a store in the
// card-unit layout and when, and nil for a member nobody redacted.
func (b *Bench) MemberRedaction(entity *EntityRef) (*Redaction, error) {
	comment, item, err := b.memberOf(entity)
	if err != nil {
		return nil, err
	}
	if comment != nil {
		return comment.Redacted, nil
	}
	return item.Redacted, nil
}

// RedactLeftovers removes every stale journal.ndjson.redact standing beside a
// card's journal, live or archived, or the workbench's, each under the lock
// of the journal's entity, and answers a check.redact-leftover finding for
// each. A leftover whose lock another process holds is reported and left for
// the next run.
func (b *Bench) RedactLeftovers(actor, now string) ([]Finding, error) {
	dirs := []string{b.Root}
	for _, root := range []string{b.CardsRoot(), b.ArchivedCardsRoot()} {
		ids, err := ListIDs(root)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			dirs = append(dirs, filepath.Join(root, id))
		}
	}
	var findings []Finding
	for _, dir := range dirs {
		journal := filepath.Join(dir, JournalName)
		leftover := journal + RedactLeftoverSuffix
		if !Exists(leftover) {
			continue
		}
		findings = append(findings, Finding{Path: leftover, Key: FindingRedactLeftover, Detail: filepath.Base(leftover), Severity: SeverityCleanup})
		lock, err := Acquire(dir, actor, now)
		if err != nil {
			continue
		}
		_, err = RemoveRedactLeftover(lock, journal)
		lock.Release()
		if err != nil {
			return findings, err
		}
	}
	return findings, nil
}

// redactionPlan is what one redaction rewrites: the lines by index with their
// replacements, and the two counts.
type redactionPlan struct {
	rewritten map[int]Event
	own       int
	legacy    int
	already   bool
}

// planRedaction composes the rewrite of one member's lines. Every line
// carrying the member's text has each such value replaced by its digest and
// is marked redacted; a legacy item_updated note line of the item, or of the
// item a comment hangs on, has its from and to replaced where they are
// versions of the member's text, every non-empty one for an item and those
// equal to a version of the comment's own text for a comment. A value a
// redaction already replaced is never hashed again.
func planRedaction(events []*Event, target RedactTarget) redactionPlan {
	plan := redactionPlan{rewritten: map[int]Event{}}
	versions := map[string]bool{}
	for _, ev := range events {
		if ev == nil {
			continue
		}
		if ev.Event == contract.EventRedacted && ev.Kind == target.Kind && memberOfLine(*ev, target.Kind) == target.ID {
			plan.already = true
		}
		if target.Kind == KindComment {
			seen := *ev
			for _, text := range ownTexts(&seen, target) {
				versions[answerVersion(*text)] = true
			}
		}
	}
	for i, ev := range events {
		if ev == nil {
			continue
		}
		line := *ev
		changed := false
		for _, text := range ownTexts(&line, target) {
			changed = replaceText(text, line.Redacted) || changed
		}
		if changed {
			line.Redacted = true
			plan.rewritten[i] = line
			plan.own++
			continue
		}
		holder := target.ID
		if target.Kind == KindComment {
			holder = target.Item
		}
		if holder == "" || line.Event != contract.EventItemUpdated || line.Field != "note" || line.Note != holder {
			continue
		}
		for _, text := range []*string{&line.From, &line.To} {
			if target.Kind == KindComment && !versions[answerVersion(*text)] {
				continue
			}
			changed = replaceText(text, line.Redacted) || changed
		}
		if changed {
			line.Redacted = true
			plan.rewritten[i] = line
			plan.legacy++
		}
	}
	return plan
}

// answerVersion is a text as a legacy answer line and a comment are compared
// by: its newlines normalised and its trailing ones dropped, since a note's
// value and the body the designation conversion wrote from it differ in the
// line ending the anchor file gave the body and in nothing else.
func answerVersion(text string) string {
	return strings.TrimRight(NormalizeNewlines(text), "\n")
}

// replaceText replaces one non-empty text with its digest, unless the line is
// already marked redacted and the value is already a digest, and reports
// whether it changed anything.
func replaceText(text *string, redacted bool) bool {
	if *text == "" || redacted && redactedDigest.MatchString(*text) {
		return false
	}
	*text = redactedText(*text)
	return true
}

// ownTexts are the members of one line that carry the target's own text:
// commented, comment_updated and comment_baseline text and the reason of an
// unblocked line whose comment it is, for a comment; item_filed,
// item_updated text and item_baseline text and the title of the line that
// deleted it, for an item.
func ownTexts(ev *Event, target RedactTarget) []*string {
	switch target.Kind {
	case KindComment:
		switch {
		case (ev.Event == contract.EventCommented || ev.Event == contract.EventCommentBaseline) && ev.Comment == target.ID:
			return []*string{&ev.Text}
		case ev.Event == contract.EventCommentUpdated && ev.Note == target.ID && ev.Field == BodyField:
			return []*string{&ev.Text}
		case ev.Event == contract.EventUnblocked && ev.Comment == target.ID:
			return []*string{&ev.Reason}
		}
	case KindItem:
		switch {
		case (ev.Event == contract.EventItemFiled || ev.Event == contract.EventItemBaseline) && ev.Item == target.ID:
			return []*string{&ev.Text}
		case ev.Event == contract.EventItemUpdated && ev.Note == target.ID && ev.Field == "text":
			return []*string{&ev.Text}
		case ev.Event == contract.EventDeleted && ev.Item == target.ID && ev.Comment == "":
			return []*string{&ev.Title}
		}
	}
	return nil
}

// memberOfLine is the member a line of one kind names.
func memberOfLine(ev Event, kind string) string {
	if kind == KindComment {
		return ev.Comment
	}
	return ev.Item
}
