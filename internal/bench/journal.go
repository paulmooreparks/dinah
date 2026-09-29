package bench

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"dinah/internal/contract"
	"dinah/internal/durable"
)

// Actor is who acted, together with whatever the caller declared about what
// performed the act. The name is written on every line, because Dinah refuses
// to write an event with no actor rather than inventing one; the other four
// members are written only where the caller declared them.
//
// The three provenance members spell their keys as OpenTelemetry spells the
// attributes they carry. A name this format cites from an external vocabulary
// is not a name this format defines, so the full stops inside them belong to
// the vocabulary they came from.
//
// An absent member is a fact nobody declared. Nothing is ever stamped with a
// literal unknown, because a workbench is free to run a provider named
// unknown and a magic value would collide with it.
type Actor struct {
	// Name is who acted, which is the string the actor member carried before
	// storage format 5.
	Name string `json:"name"`
	// Harness is the harness the process ran under, one lowercase segment of
	// the declared-field key grammar.
	Harness string `json:"harness,omitempty"`
	// Provider is the provider the model was served by.
	Provider string `json:"gen_ai.provider.name,omitempty"`
	// Model is the model the harness loaded.
	Model string `json:"gen_ai.request.model,omitempty"`
	// Server is the address the model was reached at, declared only where the
	// provider's own name does not imply it.
	Server string `json:"server.address,omitempty"`
}

// NamedActor composes an actor block from a name alone, which is what a write
// with no declaring caller records. It is one of the two functions that
// compose an actor, the other being Request.Acting in internal/verb, so no
// event is ever built from a bare struct literal at a construction site.
func NamedActor(name string) Actor {
	return Actor{Name: name}
}

// actorObject is Actor's own shape without the unmarshaller, which is what
// lets UnmarshalJSON decode the object form without recursing into itself.
type actorObject Actor

// UnmarshalJSON reads either shape a journal can carry: a JSON string, which
// is the actor of every line written before storage format 5 and reads as the
// name alone, or the object this format writes.
//
// It is the one tolerant branch this format adds, and retiring it is deleting
// this method. It is ungated, where every other tolerant branch in this
// package compares a workbench's declared format inside a read path, because
// the question it answers is about a line rather than about a workbench: a
// store part-way through the migration carries both shapes in one file, and
// no Bench is in hand here to consult a format against.
func (a *Actor) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var name string
		if err := json.Unmarshal(data, &name); err != nil {
			return err
		}
		*a = Actor{Name: name}
		return nil
	}
	var object actorObject
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	*a = Actor(object)
	return nil
}

// Event is one line of a journal: the universal skeleton (a timestamp, an
// event name and an actor) plus the fields a particular event carries.
//
// A cross-entity reference carries both the identifier and the display name
// as of the event, which is what lets a journal read as a story without
// resolving anything and survive the rename or removal of everything it
// mentions.
type Event struct {
	// TS is when the event happened, RFC 3339 in UTC.
	TS string `json:"ts"`
	// Event is the event name, from the closed set in the contract package.
	Event string `json:"event"`
	// Actor is who acted and what performed the act, self-declared
	// attribution rather than authority.
	Actor Actor `json:"actor"`
	// Title is the card's own title, carried by the created event.
	Title string `json:"title,omitempty"`
	// From and To are the state identifiers a move left and entered, the
	// values a workbench_updated event rewrote a field from and to, and the
	// previous filename an attachment_renamed event rewrote the anchor from.
	// To is not carried by attachment_renamed, since the new name sits in
	// Filename alongside the three sibling attachment events, so a reader
	// resolving "what is the attachment called as of this line" reads
	// Filename on every event in the family and From only on a rename.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// FromTitle and ToTitle are those states' titles as of the move.
	FromTitle string `json:"from_title,omitempty"`
	ToTitle   string `json:"to_title,omitempty"`
	// Override marks the one act a move admitted under CORE-MOVE-9 records.
	Override bool `json:"override,omitempty"`
	// Reject marks a moved event whose destination is the departure state's
	// own reject_to target. Set only by move; pull never carries it, because
	// a pull's destination is always a state a card is carried forward into
	// by carriesInto's walk, and a state that walk ever selects always takes
	// work up, which the terminal region and every buffer, intake and done
	// state reject_to may legally name do not. That argument leaves one
	// destination over: an ordinary work state ahead of the declaring one,
	// which carriesInto can select and a pull into it records no mark for.
	// Nothing refuses such a declaration, and nothing needs to, because it is
	// a defect in the board rather than a rejection anybody meant, and
	// checkRejectTargets reports it under check.reject-target-forward.
	Reject bool `json:"reject,omitempty"`
	// Reshape marks a moved event a reshape wrote, which is a card carried
	// out of a column the workbench no longer declares rather than a decision
	// somebody took about the work. It sits beside Override and Reject on the
	// same terms, and a reader that does not know the marker reads an
	// ordinary move, which is what the line already is: state and holder are
	// unchanged, and from, to and both titles carry what they always carry.
	Reshape bool `json:"reshape,omitempty"`
	// Grant marks an item_withdrawn event the criterion-retirement grant
	// admitted, which is exactly when the actor was not the workbench
	// operator. It sits beside Override, Reject and Reshape on the same
	// terms, and a reader that does not know the marker reads an ordinary
	// withdrawal, which is what the line already is.
	Grant bool `json:"grant,omitempty"`
	// Cards are the identifiers of the cards a designations_migrated event
	// passed the claim of. The line is written only by the designation
	// conversion, whose report names the same cards, so a reader afterwards
	// can name each claim the operator judged dead.
	Cards []string `json:"cards,omitempty"`
	// Reason is a block's prose reason, the reason an unblock gave for
	// lifting one, or the reason a raise gave for requiring more of a card
	// than it required a moment ago. Block, unblock and raise populate it:
	// block always, unblock only when the lift said why, and an ordinary
	// per-column tier write carries none, so a reader meeting a
	// tier_overridden line with no reason is meeting one of those rather
	// than a raise that omitted it.
	Reason string `json:"reason,omitempty"`
	// Kind is a block's optional class of obstacle.
	Kind string `json:"kind,omitempty"`
	// Expires is the expiry a claim carried.
	Expires string `json:"expires,omitempty"`
	// Attachment and Filename identify an attachment lifecycle event.
	Attachment string `json:"attachment,omitempty"`
	Filename   string `json:"filename,omitempty"`
	// Comment is the identifier of a comment the event concerns.
	Comment string `json:"comment,omitempty"`
	// Field is the workbench field a workbench_updated event rewrote, with
	// From and To carrying its value on either side of the write.
	Field string `json:"field,omitempty"`
	// Workstream is the identifier of the workstream a membership event
	// concerns, carried by workstream_joined and workstream_left on the
	// card's own journal.
	Workstream string `json:"workstream,omitempty"`
	// Column is the identifier of the column a tier override or a column
	// comment concerns, carried by tier_overridden, tier_override_dropped and
	// commented. It is the resolved identifier rather than the reference
	// somebody typed, because the reference can be a slug a later rename
	// changes and the line is history.
	Column string `json:"column,omitempty"`
	// ColumnTitle is that column's title as of the event, captured at write
	// time the way FromTitle and ToTitle capture a move's states, so a reader
	// is not left holding a bare identifier for a column that may since have
	// been renamed or retired. A raise and a column comment each populate it;
	// the ordinary per-column tier write does not, and the renderer falls
	// back to Column for every line that carries none.
	ColumnTitle string `json:"column_title,omitempty"`
	// Expr is what a person typed for a tier override, absolute or relative,
	// carried by tier_overridden alongside the resolved value in To. Keeping
	// it means a later reader is not left inferring the intent from the
	// result.
	Expr string `json:"expr,omitempty"`
	// Against is the column's own tier default at the moment a relative
	// expression was resolved, carried by tier_overridden. It is absent on an
	// absolute write, which needed no baseline to be relative to.
	Against string `json:"against,omitempty"`
	// Item is the identifier of the checklist item a lifecycle event
	// concerns, carried by the six item events on the card's own journal and
	// by a commented line for a comment left on an item. The
	// line points at the item the way a comment event points at the comment,
	// so the item's own text and note are read from its anchor rather than
	// copied into history.
	Item string `json:"item,omitempty"`
	// Standing is the key of the standing entry a declaration filed an item
	// from, carried by an item_filed line an arrival at a declaring column
	// wrote, beside Column and ColumnTitle naming the declaring column. The
	// actor stays the arriving act's, because a column is not an actor and
	// the format refuses a line with none; the three members say what
	// decided the filing. A hand filing carries none of the three, and a
	// reader that does not know them reads an ordinary filing.
	Standing string `json:"standing,omitempty"`
	// Scheme and Target are the citation an item_cited event recorded, as the
	// caller typed them. Nothing here resolves either: a citation is taken at
	// its word at write time, and dinah check is what tells a reader it was
	// wrong.
	Scheme string `json:"scheme,omitempty"`
	Target string `json:"target,omitempty"`
	// Note is the human's free prose, unparseable by design.
	Note string `json:"note,omitempty"`
	// Text is the prose a line records on a store in the card-unit layout:
	// a comment's body on commented, comment_updated and comment_baseline,
	// an item's text on item_filed, item_updated and item_baseline, a card's
	// body on created and card_updated, and card.md's whole text on
	// card_baseline. It is absent where that prose is empty.
	Text string `json:"text,omitempty"`
	// Ordinal is the creation ordinal of the member a line creates or
	// baselines.
	Ordinal int `json:"ordinal,omitempty"`
	// Owner is an item's owner, on item_filed and item_baseline.
	Owner string `json:"owner,omitempty"`
	// Evidence is an item's evidence scheme, on item_filed and
	// item_baseline.
	Evidence string `json:"evidence,omitempty"`
	// State is an item's state, on item_baseline only. Every other line
	// states an item's state by what it is.
	State string `json:"state,omitempty"`
	// Resolution is the identifier of the comment a settling line designated
	// as the item's answer, and the item's answer on item_baseline.
	Resolution string `json:"resolution,omitempty"`
	// Citations are an item's citations in stored order, on item_baseline
	// only.
	Citations []CitationRecord `json:"citations,omitempty"`
	// Observed is the observation an item_cited line recorded, written
	// "<before>:<after>" as dinah cite --observed takes it.
	Observed string `json:"observed,omitempty"`
	// Author is a comment's author, on comment_baseline only. A comment
	// created on a store in the card-unit layout is authored by the actor of
	// its commented line.
	Author string `json:"author,omitempty"`
	// AuthorUnrecoverable records a comment the store cannot attribute, on
	// comment_baseline only.
	AuthorUnrecoverable bool `json:"author_unrecoverable,omitempty"`
	// Written is a member's creation timestamp, on the two member baselines
	// only, since every other creation line's own timestamp is the member's.
	Written string `json:"written,omitempty"`
	// Digest is a comment's recorded digest, on comment_baseline only.
	Digest string `json:"digest,omitempty"`
	// Archived records a member baselined in the archived half, on the two
	// member baselines only.
	Archived bool `json:"archived,omitempty"`
	// Witnessed marks a card_updated line the witness wrote to make the
	// journal agree with a hand edit of card.md.
	Witnessed bool `json:"witnessed,omitempty"`
	// Trimmed is the byte count a journal_tail_trimmed line moved to its
	// sidecar.
	Trimmed int `json:"trimmed,omitempty"`
	// Accepted are the manifest keys an operator accepted with
	// --accept-difference, on storage_migrated only.
	Accepted []string `json:"accepted,omitempty"`
	// WrittenDuringRun are the manifest keys the storage migration found
	// written while it ran, on storage_migrated only.
	WrittenDuringRun []string `json:"written_during_run,omitempty"`
	// Redacted marks a line dinah redact rewrote, whose text now reads
	// sha256:<digest of the text it carried>.
	Redacted bool `json:"redacted,omitempty"`
	// Lines is how many lines a redacted line records rewriting.
	Lines int `json:"lines,omitempty"`
	// Fields are, on a created line, the card fields the filing set beside
	// its title and its column, keyed by their frontmatter key: the levels,
	// the route and the scheduling dates it named. A filing that named none
	// carries none. The card-field replay reads them, so the line states
	// everything the filing wrote into card.md.
	Fields map[string]string `json:"fields,omitempty"`
	// ColumnRef is, on a tier_overridden or tier_override_dropped line, the
	// reference the card's tier_at entry is written under, which is the
	// spelling the card-field replay puts back; column carries the
	// identifier it resolves to.
	ColumnRef string `json:"column_ref,omitempty"`
}

// CitationRecord is one citation as a journal line carries it.
type CitationRecord struct {
	// Scheme and Target are the citation as the caller typed them.
	Scheme string `json:"scheme"`
	Target string `json:"target"`
	// Observed is "<before>:<after>", absent where the entry recorded none.
	Observed string `json:"observed,omitempty"`
}

// AppendEvent adds one line to the journal of the entity whose lock the caller
// holds, creating the journal when absent. held must be the lock Acquire
// returned for that entity's own directory: the card directory for a card
// journal, the workstream directory for a workstream journal, the workbench
// root for the workbench journal. The write is an append rather than a
// rewrite, so a crash can tear at most the final line and never the records
// already in the file.
//
// It refuses an event carrying no actor name, and an append whose lock does
// not guard the journal's own directory, before touching the file, because it
// is the one function every journal write in this codebase funnels through,
// with no journal path written to by any other means. Every mutating verb's
// own precondition list already checks req.Actor == "" ahead of its own
// write, and those checks stay, because they are what gives dinah help <verb>
// its documented check order; this guard is the backstop that makes the
// guarantee hold for every path, including one added later that forgets its
// own check. It carries no *Request to enrich the refusal with a declared
// harness the way Library.refuse does, because it has none to read: every
// caller composes the enrichment on its own side, where a request is still in
// scope, before the refusal this function raises ever reaches a reader.
//
// The lock is what makes the tail repair safe. A journal that does not end in
// a newline is repaired before the line is written, as repairTail describes,
// and a repair made by a process that did not hold the entity's lock could cut
// a line another process was writing under it.
func AppendEvent(held *Lock, path string, ev Event) error {
	if strings.TrimSpace(ev.Actor.Name) == "" {
		return contract.Refuse(contract.NoOwner, "")
	}
	if !held.guards(filepath.Dir(path)) {
		return contract.Refuse(contract.JournalUnlocked, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	line, err := EncodeEvent(ev)
	if err != nil {
		return err
	}
	prefix, err := repairTail(path, ev.TS, ev.Actor.Name)
	if err != nil {
		return err
	}
	return durable.AppendLine(path, append(prefix, line...))
}

// AppendEvent is the free AppendEvent reached through this bench, which is how
// the library appends. The tail repair every append makes reads the end of
// the file the line lands in, a read this package's seam allows inside it
// (internal/bench/sourceguard_test.go names it) and which a caller outside it
// therefore reaches only through a method.
func (b *Bench) AppendEvent(held *Lock, path string, ev Event) error {
	return AppendEvent(held, path, ev)
}

// AppendEvents appends several lines to one journal in one write and one
// flush, on the terms AppendEvent appends one: the caller holds the lock of
// the journal's own entity, every line names an actor, and a torn tail is
// repaired first. A failed write is cut back, so either every line lands or
// none does. It serves a writer with many lines for one entity at once, which
// is the storage migration's baselines, where a flush per line is most of the
// run's cost.
func AppendEvents(held *Lock, path string, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	for _, ev := range events {
		if strings.TrimSpace(ev.Actor.Name) == "" {
			return contract.Refuse(contract.NoOwner, "")
		}
	}
	if !held.guards(filepath.Dir(path)) {
		return contract.Refuse(contract.JournalUnlocked, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lines := make([][]byte, 0, len(events))
	for _, ev := range events {
		line, err := EncodeEvent(ev)
		if err != nil {
			return err
		}
		lines = append(lines, line)
	}
	prefix, err := repairTail(path, events[0].TS, events[0].Actor.Name)
	if err != nil {
		return err
	}
	return durable.AppendLine(path, append(prefix, bytes.Join(lines, []byte("\n"))...))
}

// EncodeEvent is the one encoding of a journal line: every string member
// normalised as NormalizeNewlines normalises it, and the JSON written with
// HTML escaping switched off, so a line carries <, > and & as themselves
// rather than as < escapes and the journal stays readable with grep. The
// two spellings decode identically, so nothing reading a journal changes.
// The answer carries no trailing newline.
func EncodeEvent(ev Event) ([]byte, error) {
	ev = normalizeEventText(ev)
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(ev); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}

// TornSidecarPrefix begins the name of the file a torn journal tail is moved
// to, beside the journal it was cut from. It is the one file besides an
// entity's own that may stand in a card, workstream or workbench directory,
// and no command deletes one: dinah check reports it until a person has read
// it and removed it.
const TornSidecarPrefix = "journal.torn."

// repairTail makes a journal end in a newline before a line is appended to
// it. It is reached only through AppendEvent, after the check that the caller
// holds the lock of the journal's entity. It answers the bytes the append has
// to write ahead of its own line: a newline when the final line is whole and
// only lacks one, the journal_tail_trimmed line when a torn tail was
// quarantined, and nothing when the journal already ends in a newline.
//
// A final line with no newline after it is one of two things. When its bytes
// decode as one JSON object it is complete, and a hand edit, an editor or a
// merge removed the newline; nothing is removed, and the newline is written
// ahead of the new line. Otherwise it is the torn tail of an act that crashed
// before returning success, so quarantining it removes nothing any caller
// was told had happened: the bytes are written to a sidecar and flushed, the
// journal is cut back to just after its last newline and flushed, and a
// journal_tail_trimmed line naming the sidecar and the byte count is appended
// before the new line.
func repairTail(path, ts, actor string) ([]byte, error) {
	cut, tail, err := finalLine(path)
	if err != nil {
		return nil, err
	}
	if tail == nil {
		return nil, nil
	}
	if decodesAsObject(tail) {
		return []byte("\n"), nil
	}
	sidecar, err := quarantineTail(filepath.Dir(path), tail, ts)
	if err != nil {
		return nil, err
	}
	if err := durable.Truncate(path, cut); err != nil {
		return nil, err
	}
	trimmed, err := EncodeEvent(Event{
		TS:      ts,
		Event:   contract.EventJournalTailTrimmed,
		Actor:   NamedActor(actor),
		Trimmed: len(tail),
		Note:    filepath.Base(sidecar),
	})
	if err != nil {
		return nil, err
	}
	return append(trimmed, '\n'), nil
}

// RepairJournalTail is the tail repair on its own, for dinah check --witness:
// it makes a journal end in a newline on the writer's two cases and appends
// nothing else. held must guard the journal's own directory, as it must for
// AppendEvent, and the answer reports whether anything was written.
func RepairJournalTail(held *Lock, path, actor, ts string) (bool, error) {
	if !held.guards(filepath.Dir(path)) {
		return false, contract.Refuse(contract.JournalUnlocked, path)
	}
	prefix, err := repairTail(path, ts, actor)
	if err != nil || len(prefix) == 0 {
		return false, err
	}
	return true, durable.AppendLine(path, bytes.TrimSuffix(prefix, []byte("\n")))
}

// finalLine reads the end of a journal and answers the offset just after its
// last newline together with the bytes that follow it, when the journal does
// not end in a newline. A journal that is absent, empty, or ends in a newline
// answers a nil tail. Only the end of the file is read, a chunk at a time
// back to the last newline, so an append to a long journal does not read the
// whole of it.
func finalLine(path string) (int64, []byte, error) {
	f, err := durable.Open(path)
	if os.IsNotExist(err) {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, nil, err
	}
	size := info.Size()
	if size == 0 {
		return 0, nil, nil
	}
	last := make([]byte, 1)
	if _, err := f.ReadAt(last, size-1); err != nil {
		return 0, nil, err
	}
	if last[0] == '\n' {
		return 0, nil, nil
	}
	const chunk = 4096
	var tail []byte
	for end := size; end > 0; {
		start := max(end-chunk, 0)
		buffer := make([]byte, end-start)
		if _, err := f.ReadAt(buffer, start); err != nil {
			return 0, nil, err
		}
		if at := bytes.LastIndexByte(buffer, '\n'); at >= 0 {
			return start + int64(at) + 1, append(buffer[at+1:], tail...), nil
		}
		tail = append(buffer, tail...)
		end = start
	}
	return 0, tail, nil
}

// decodesAsObject reports whether a final line's bytes decode as one whole
// JSON object, which is what separates a complete line that only lacks its
// newline from the torn fragment of an append that never finished.
func decodesAsObject(line []byte) bool {
	var object map[string]json.RawMessage
	return json.Unmarshal(bytes.TrimSpace(line), &object) == nil
}

// quarantineTail writes a torn tail's bytes to a fresh sidecar beside the
// journal, named for the moment of the repair and suffixed -2, -3 and upward
// when that name is taken, and answers the sidecar's path. The stamp is the
// appending line's own timestamp, compacted to YYYYMMDDTHHMMSSZ, so the
// sidecar and the journal_tail_trimmed line that names it agree on when the
// repair happened.
func quarantineTail(dir string, tail []byte, ts string) (string, error) {
	stamp := strings.NewReplacer("-", "", ":", "").Replace(ts)
	if stamp == "" {
		stamp = strings.NewReplacer("-", "", ":", "").Replace(Stamp(time.Now()))
	}
	base := filepath.Join(dir, TornSidecarPrefix+stamp)
	for n := 1; ; n++ {
		candidate := base
		if n > 1 {
			candidate = base + "-" + strconv.Itoa(n)
		}
		if _, err := os.Lstat(candidate); err == nil {
			continue
		}
		if err := durable.WriteFile(candidate, tail, 0o644); err != nil {
			return "", err
		}
		return candidate, nil
	}
}

// TornSidecars lists the torn-tail sidecars standing in an entity directory,
// sorted by name. A directory that will not read answers none, since the
// question is asked of a directory the caller has already read something
// from.
func TornSidecars(dir string) []string {
	return tornSidecars(Disk{}, dir)
}

// tornSidecars is TornSidecars's body, reading through src.
func tornSidecars(src Source, dir string) []string {
	entries, err := src.ReadDir(dir)
	if err != nil {
		return nil
	}
	var found []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), TornSidecarPrefix) {
			found = append(found, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(found)
	return found
}

// ReadJournal reads a journal in file order, which is event order. An absent
// journal is an empty history.
//
// A final line that decodes is kept whether or not a newline follows it, and
// one that does not decode is skipped, because a reader holding no lock may
// be looking at an append in flight. The second return value reports that
// skip, which is what check reports as a torn journal. A reader never
// truncates and never writes a sidecar; the writer holding the entity's lock
// repairs the tail before it appends.
//
// A line that does not decode and is not the final line is damage rather
// than a crash artifact, and the read is refused dinah.journal-unreadable,
// naming the file and the one-based line number, so a card's members are
// never answered from a journal a reader cannot fully read.
func ReadJournal(path string) ([]Event, bool, error) {
	return readJournal(Disk{}, path)
}

// ReadJournal is the free ReadJournal read through this bench's source.
func (b *Bench) ReadJournal(path string) ([]Event, bool, error) {
	return readJournal(b.source(), path)
}

// readJournal is ReadJournal's body, reading through src.
func readJournal(src Source, path string) ([]Event, bool, error) {
	observeAnchor(path)
	value, err := src.Derive(path, DeriveJournal, deriveJournal)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	parsed := value.(*parsedJournal)
	if parsed.err != nil {
		return nil, parsed.torn, parsed.err
	}
	return cloneEvents(parsed.events), parsed.torn, nil
}

// readJournalShared is readJournal for a reader inside this package that only
// reads the events: it answers the parse a memoising source shares rather
// than a copy, so the caller must not change what it is handed. Arrival is
// such a reader, and a sort asks it once per card.
func readJournalShared(src Source, path string) (*parsedJournal, error) {
	observeAnchor(path)
	value, err := src.Derive(path, DeriveJournal, deriveJournal)
	if os.IsNotExist(err) {
		return &parsedJournal{}, nil
	}
	if err != nil {
		return nil, err
	}
	parsed := value.(*parsedJournal)
	if parsed.err != nil {
		return nil, parsed.err
	}
	return parsed, nil
}

// JournalLines visits each event of a journal in file order with its index,
// its stored stamp, the stamp parsed as ParseStamp parses it, and its event
// name, as ReadJournal would answer them, and answers ReadJournal's error. It
// copies no event: a reader that needs only the stamps, which a cursor over
// every journal on the workbench does, pays for no copy of every event's
// other members, and a source that memoises the parse parses each stamp once.
// An absent journal visits nothing.
func (b *Bench) JournalLines(path string, visit func(index int, ts string, at time.Time, event string)) error {
	parsed, err := readJournalShared(b.source(), path)
	if err != nil {
		return err
	}
	for index, event := range parsed.events {
		visit(index, event.TS, parsed.stamps[index], event.Event)
	}
	return nil
}

// parsedJournal is what DeriveJournal memoises: the events, whether a torn
// final line was skipped, and the error a line that is not the last one
// raised, which is part of the answer rather than a failure to read.
type parsedJournal struct {
	events []Event
	// stamps are the events' stamps parsed as ParseStamp parses them, one
	// per event.
	stamps []time.Time
	torn   bool
	err    error
}

// deriveJournal is DeriveJournal's derive function: readJournal's body after
// its read.
func deriveJournal(path string, text, _ string) (any, error) {
	events, torn, err := parseJournal(path, text)
	stamps := make([]time.Time, len(events))
	for i, event := range events {
		stamps[i] = ParseStamp(event.TS)
	}
	return &parsedJournal{events: events, stamps: stamps, torn: torn, err: err}, nil
}

// parseJournal reads the text of the journal at path into its events.
func parseJournal(path, text string) ([]Event, bool, error) {
	var events []Event
	torn := false
	lines := SplitLines(text)
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			if i == len(lines)-1 {
				torn = true
				continue
			}
			return nil, torn, contract.Refuse(contract.JournalUnreadable, path+":"+strconv.Itoa(i+1))
		}
		events = append(events, ev)
	}
	return events, torn, nil
}

// normalizeEventText answers the event with every string member normalised.
// The walk is total rather than a list of the prose-bearing members, because
// normalisation is a no-op on a timestamp, an event name and an identifier, so
// totality costs nothing and removes a list that would go stale the next time
// a member is added. It recurses into a nested struct so that a member which
// later becomes one is covered without this function being revisited.
//
// The recursion is load-bearing rather than tidy. Actor is a struct of five
// strings, so a walk over top-level string members alone would reach the
// actor's name not at all, and the actor's name is on every line this journal
// carries.
//
// A carriage return needs removing here rather than being left to the encoder,
// because json.Marshal stores one as an escape: the NDJSON line structure
// stays intact and the carriage return is stored anyway.
func normalizeEventText(ev Event) Event {
	value := reflect.ValueOf(&ev).Elem()
	normalizeStringsIn(value)
	return ev
}

// normalizeStringsIn normalises every settable string this value reaches, at
// any depth, walking a struct's own fields and following a pointer and a slice
// to whatever they hold. An unexported field is not settable, so it is passed
// over rather than panicking the walk.
func normalizeStringsIn(value reflect.Value) {
	switch value.Kind() {
	case reflect.String:
		if value.CanSet() {
			value.SetString(NormalizeNewlines(value.String()))
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			normalizeStringsIn(value.Field(i))
		}
	case reflect.Ptr, reflect.Interface:
		if !value.IsNil() {
			normalizeStringsIn(value.Elem())
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			normalizeStringsIn(value.Index(i))
		}
	}
}
