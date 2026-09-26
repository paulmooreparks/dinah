package verb

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// ChangeSet is the answer to one checkpoint.
type ChangeSet struct {
	// Cursor is the token to hand back at the next checkpoint. Present on
	// every answer, including one reporting no change.
	Cursor string `json:"cursor"`
	// Changed reports whether either digest term moved. It is a fact about
	// the whole workbench even when a filter narrowed the arrays below, so
	// a filtered call can answer true with every array empty.
	Changed bool `json:"changed"`
	// Events are the journal lines after the cursor's position, in the
	// total order, filtered to what the caller asked about.
	Events []ChangeEvent `json:"events,omitempty"`
	// Cards are the live cards this call has a reason to report, as they now
	// stand, so a caller learns the new column without a second call.
	Cards []*CardView `json:"cards,omitempty"`
	// Gone reports what left, one entry per archived or deleted event after
	// the cursor's position. Read Kind before assuming an entry is a card.
	Gone []GoneEntity `json:"gone,omitempty"`
	// Columns are the live columns this call has a reason to report, as they
	// now stand. A column carries no journal, so a call has evidence that
	// the column half moved and no evidence of which column moved it, and
	// the member therefore carries the whole flow whenever the column term
	// moved and nothing when it did not. The flow is a small fixed set, so
	// reporting all of it costs one anchor read per column rather than the
	// read of every card anchor that a truthful occupancy would cost.
	Columns []ColumnChange `json:"columns,omitempty"`
	// Unreadable names the entities whose journals this call could not
	// parse, whose events are therefore absent from this answer. It reports
	// what this walk hit rather than the health of the bench, since a call
	// that parses nothing finds nothing. dinah check is the standing report.
	Unreadable  []string `json:"unreadable,omitempty"`
	Affordances []string `json:"affordances"`
}

// ChangeEvent is one journal line with the entity it came from attached,
// since a caller reading a merged stream cannot tell otherwise.
type ChangeEvent struct {
	// Scope is card, workstream or workbench, naming which journal the
	// line was read from.
	Scope string `json:"scope"`
	// ID is the entity's identifier, empty for the workbench.
	ID string `json:"id,omitempty"`
	// Ref is the human reference of the entity whose journal this line came
	// from, when one can be composed from an anchor that still exists. It is
	// a convenience and never load-bearing, because ID is always present.
	Ref string `json:"ref,omitempty"`
	bench.Event
}

// GoneEntity reports one departure. The two fates are not the same claim and
// Kind is where the difference shows: an archived entry is known to be a card
// because the event was found in that card's own journal, while a removed
// entry is an identifier out of a deleted event, which names no entity kind.
// A caller matches a removed entry against the identifiers it is holding and
// ignores the ones it does not know.
type GoneEntity struct {
	// ID is the 12-hex identifier, from the journal's own entity for an
	// archived card and from the deleted event's note for a removed one.
	ID string `json:"id"`
	// Kind is card when this answer can prove it, and empty when the journal
	// does not say. Empty is not "unknown card"; it is "unknown entity".
	Kind string `json:"kind,omitempty"`
	// Ref is the card's human reference, filled for an archived card out of
	// the anchor that still exists and empty for a removed entity, which has
	// none.
	Ref string `json:"ref,omitempty"`
	// Title is the title as of the event: the archived card's anchor title,
	// or the title the deleted event carried.
	Title string `json:"title,omitempty"`
	// Fate is archived or removed.
	Fate string `json:"fate"`
}

// ColumnChange is one column a checkpoint reports as moved. It is the
// column's identity and the one fact the move is usually about, and it
// deliberately carries no occupancy, because counting cards is the read the
// column digest term exists to avoid.
type ColumnChange struct {
	// ID is the column's identifier.
	ID string `json:"id"`
	// Slug is the column's short handle, absent where the column carries none.
	Slug string `json:"slug,omitempty"`
	// Title is the column's title as it now stands.
	Title string `json:"title"`
	// HoldsOnEntry reports whether an item naming this column can hold a
	// card entering it. It is Column.HoldsOnEntry, published rather than
	// derived, because Column.Hold's own comment directs a reader to those
	// two rather than to a comparison against the raw string.
	HoldsOnEntry bool `json:"holds_on_entry"`
	// HoldsOnExit reports whether an item naming this column can hold a card
	// leaving it.
	HoldsOnExit bool `json:"holds_on_exit"`
}

// The scopes a change event names, which say which journal the line was read
// from rather than what the line is about.
const (
	ScopeCard       = "card"
	ScopeWorkstream = "workstream"
	ScopeWorkbench  = "workbench"
)

// The two fates a gone entry carries.
const (
	// FateArchived is a card whose own journal recorded its departure, so
	// the answer can prove the entity was a card.
	FateArchived = "archived"
	// FateRemoved is an identifier a deleted event named, of a kind the
	// journal format does not record.
	FateRemoved = "removed"
)

// archiveEvents are the only lines the archive half of the walk yields. An
// archived entity's own acts are not a caller's to act on, so the half reports
// the departure and the return and nothing else. `dinah restore` is the one
// verb that takes an archived card as its subject, and the line it appends to
// that card's own journal is why restored sits in this set beside archived.
//
// Both reads of that half use this set, the reporting one and the one a mint
// makes to find the end of the total order, so a minted position can never sit
// on a line no later call would deliver.
var archiveEvents = map[string]bool{
	contract.EventArchived: true,
	contract.EventRestored: true,
}

// cursorVersion is the shape number the token carries, so a token minted by a
// later shape is refused by an earlier binary rather than misread by it.
const cursorVersion = 3

// cursor is what a caller hands back, rendered as base64url of this object.
//
// The three digest terms answer "did anything change" against file bytes. What
// the caller has already been told is a boundary second plus a frontier
// within it, and the shape is chosen because the merged order is not monotone
// with arrival.
//
// The merged order sorts a line by its parsed stamp, then its stored stamp
// text, then the entity key, then the index within that entity. That is a
// total order, and it is the right order to present lines in, but
// bench.TimeFormat is second resolution, so it is not the order the lines
// arrived in: two acts inside one second are separated by an entity key that
// says nothing about which came first. A cursor recorded as a single point in
// that order therefore classifies a later act on a lower-sorting entity as
// already delivered, and drops it for good, since a call that delivers
// nothing leaves the position where it was while the digest terms move on.
//
// So the cursor covers a line rather than out-ranking it. TS is the stored
// stamp of the newest line the caller has been given. Every line whose second
// is strictly older than TS is covered. Within TS itself, a line is covered
// only when Frontier records that entity at an index at least as high, which
// is exact: a journal is append-only, so a line written into that second
// after the cursor was taken carries an index above the one recorded, and an
// entity absent from Frontier has been told nothing at that second at all.
//
// Frontier grows with the entities acted on inside one second and not with
// the board, which is the property D-6 and D-9 chose the token's shape for. A
// workbench that fills a hundred cards inside one second mints a token
// carrying a hundred entries; a bench where acts are seconds apart carries
// one or two, whatever its size.
//
// An index is the index into the slice bench.ReadJournal returns rather than
// a physical line number. That reader skips blank lines and a torn final
// line, so the two differ, and the parsed index is the one that stays stable
// when a torn tail is later appended past or trimmed by check.
type cursor struct {
	Version int `json:"v"`
	// Workbench is the slug of the workbench that issued the token. The
	// member is spelled in full rather than shortened, which is the product's
	// own word everywhere a reader meets one.
	Workbench string `json:"workbench"`
	Live      string `json:"live"`
	Archive   string `json:"archive"`
	// Columns is the term over every live column's anchor. It is separate
	// from Live because a column has no journal, so a column edit that
	// moved the live term would deliver no line to explain it and would
	// resync every card on the board.
	Columns string `json:"columns"`
	// TS is the boundary second: the stored stamp of the newest line this
	// cursor covers. Empty on a cursor that covers nothing.
	TS string `json:"ts,omitempty"`
	// Frontier is the highest journal index covered within each entity that
	// has a covered line at TS. An entity that is absent is covered at TS by
	// nothing.
	Frontier map[string]int `json:"frontier,omitempty"`
	// parsedTS is TS parsed, and parsedFor the TS it was parsed from, which
	// cover keeps so a fold does not parse the boundary once per line. They
	// are not part of the token.
	parsedTS  time.Time
	parsedFor string
}

// encode renders a cursor as the opaque token a caller carries.
func (c cursor) encode() (string, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// decodeCursor reads a token back, refusing rather than resyncing. A silent
// resync would hide the caller's bug and lose the events it was owed, and it
// would do so quietly enough that nobody found out.
func decodeCursor(token string) (cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return cursor{}, contract.Refuse(contract.Malformed, token)
	}
	var read cursor
	if err := json.Unmarshal(raw, &read); err != nil {
		return cursor{}, contract.Refuse(contract.Malformed, token)
	}
	if read.Version != cursorVersion || read.Live == "" || read.Archive == "" || read.Columns == "" {
		return cursor{}, contract.Refuse(contract.Malformed, token)
	}
	return read, nil
}

// position is one journal line with everything the total order needs: the
// entity it was read from and its index within that entity's history.
type position struct {
	key   string
	index int
	event bench.Event
}

// before reports whether one position sorts ahead of another under the total
// order: timestamp, then entity key, then index within the journal.
//
// Equal parsed timestamps break to the stored text before the key, so two
// stamps that parse alike and read differently still order deterministically
// rather than by whichever the walk reached first. A stamp that will not parse
// comes back as the zero time, which sorts it to the front, and the stored
// text keeps it stable there.
func (p position) before(other position) bool {
	if p.event.TS != other.event.TS {
		return stampLess(p.event.TS, other.event.TS)
	}
	if p.key != other.key {
		return p.key < other.key
	}
	return p.index < other.index
}

// stampLess orders two stored stamps: by the instant each parses to, and by
// the stored text when they parse alike. A stamp that will not parse comes
// back as the zero time, which sorts it to the front, and the stored text
// keeps it stable there.
func stampLess(a, b string) bool {
	return stampLessParsed(a, bench.ParseStamp(a), b, bench.ParseStamp(b))
}

// stampLessParsed is stampLess over stamps a caller has already parsed.
func stampLessParsed(a string, parsedA time.Time, b string, parsedB time.Time) bool {
	if !parsedA.Equal(parsedB) {
		return parsedA.Before(parsedB)
	}
	return a < b
}

// delivered reports whether the cursor has already been told about a line,
// which is what decides whether the line is delivered again.
//
// The question is coverage rather than rank. A line older than the boundary
// second is covered. A line newer is not. A line inside the boundary second
// is covered only when the frontier records its own entity at an index at
// least as high, which is what keeps an act written into that second after
// the cursor was taken from being classified as ancient history because of
// how its entity key happens to sort.
func (c cursor) delivered(p position) bool {
	// A cursor that has covered nothing yet carries neither a boundary nor a
	// frontier. The two are read together rather than the boundary alone,
	// because a journal line with no stamp at all lands on the empty boundary
	// and is covered by its frontier entry rather than by the second.
	if c.TS == "" && c.Frontier == nil {
		return false
	}
	if c.TS != p.event.TS {
		return stampLess(p.event.TS, c.TS)
	}
	covered, ok := c.Frontier[p.key]
	return ok && p.index <= covered
}

// coverThrough folds a set of delivered lines into the cursor's coverage and
// returns the result, leaving the receiver's own frontier untouched so a
// caller may keep comparing against the token it was handed.
func (c cursor) coverThrough(delivered []position) cursor {
	frontier := make(map[string]int, len(c.Frontier)+len(delivered))
	for key, index := range c.Frontier {
		frontier[key] = index
	}
	c.Frontier = frontier
	for _, at := range delivered {
		c.cover(at.key, at.index, at.event.TS)
	}
	if len(c.Frontier) == 0 {
		c.Frontier = nil
	}
	return c
}

// cover folds one delivered line into the cursor's coverage, which is
// coverThrough's step for one position. A caller that only needs the fold
// calls it line by line rather than building the positions first. The
// boundary's parse is kept beside it, so a fold over many lines parses each
// line's stamp once and the boundary once per change of boundary.
func (c *cursor) cover(key string, index int, ts string) {
	if c.TS != c.parsedFor {
		c.parsedTS, c.parsedFor = bench.ParseStamp(c.TS), c.TS
	}
	switch {
	case c.TS != "" && c.TS != ts && stampLessParsed(ts, bench.ParseStamp(ts), c.TS, c.parsedTS):
		// A line older than the boundary is already covered by it.
	case c.TS == ts:
		if covered, ok := c.Frontier[key]; !ok || index > covered {
			c.Frontier[key] = index
		}
	default:
		c.TS = ts
		c.Frontier = map[string]int{key: index}
	}
}

// Changes answers one checkpoint: what has happened on this workbench since
// the caller's cursor, and a fresh cursor to ask with next time.
//
// It reads and never writes. No lock is taken, no journal is appended, no
// anchor is saved and no basis is consumed, which is why it does not lapse an
// expired claim the way the other reads of the bench do: reporting a card as
// stored is the honest answer from a call that is not allowed to change it.
//
// A call carrying req.Wait holds the call open rather than answering the
// first checkpoint immediately; waitForChange is the whole of what that adds,
// and it is reached only after the flag-grammar refusals in
// cmd/dinah/commands.go, so a call that reaches here with Wait set is already
// known to carry a non-empty Since and no Root.
func (l *Library) Changes(req *Request) (*ChangeSet, error) {
	if req.Wait {
		return l.waitForChange(req)
	}
	return l.checkpoint(req)
}

// waitPollInterval is the fixed sleep between iterations of a waiting changes
// call. D-2 (dinah-546): measured cost is about 40ms per full checkpoint walk
// against this project's own 337-entity workbench, so this interval leaves
// comfortable headroom while still giving sub-second wake latency; a flag
// would add a surface for a value nothing here shows a present need to tune.
const waitPollInterval = 500 * time.Millisecond

// waitForChange runs the ordinary checkpoint on a fixed interval until either
// it reports a change (or an error) or req.Timeout's deadline passes, which
// never happens when req.Timeout is zero. It follows internal/lsp/poll.go's
// own adaptive-sleep rule rather than importing it, since that package
// already imports this one: the sleep between iterations is the larger of
// waitPollInterval and the duration the checkpoint just took, so a slow
// workbench degrades the wait's own latency instead of running two walks at
// once or spinning a core.
//
// The first iteration runs before any sleep, so a caller already behind gets
// exactly the answer an unconditional changes --since <cursor> would give,
// with no waiting at all. The deadline, when one is set, additionally caps
// the last sleep so a call does not overshoot it by up to one whole interval.
//
// No new synchronization primitive is introduced beyond time.Sleep and a
// deadline comparison: the CLI process runs one command per invocation on one
// goroutine, so nothing here races anything else in the same process. A
// concurrent second process, or another shell running as the same or a
// different actor, changing the workbench while this one waits is exactly
// what wakes it, because the digest terms carry no notion of who wrote the
// bytes, only that they moved.
func (l *Library) waitForChange(req *Request) (*ChangeSet, error) {
	var deadline time.Time
	if req.Timeout > 0 {
		deadline = time.Now().Add(req.Timeout)
	}
	for {
		started := time.Now()
		// Each poll is its own answer and reads its own day, so a wait
		// running across midnight draws the change it reports on the day it
		// reports it rather than on the day the wait began.
		poll := *req
		poll.day = nil
		set, err := l.checkpoint(&poll)
		walked := time.Since(started)
		if err != nil {
			return nil, err
		}
		if set.Changed {
			return set, nil
		}
		if !deadline.IsZero() && !time.Now().Before(deadline) {
			return set, nil
		}
		sleep := waitPollInterval
		if walked > sleep {
			sleep = walked
		}
		if !deadline.IsZero() {
			if remaining := time.Until(deadline); remaining < sleep {
				sleep = remaining
			}
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}
	}
}

// checkpoint is the ordinary, immediate answer Changes has always given: one
// walk of the bench compared against the caller's cursor. waitForChange calls
// it once per iteration; every caller that does not set req.Wait reaches it
// directly through Changes and sees no behavior below this point that did not
// exist before this card.
func (l *Library) checkpoint(req *Request) (*ChangeSet, error) {
	live, archive, columns, err := l.Bench.WatchedEntities()
	if err != nil {
		return nil, err
	}
	terms := cursor{
		Version:   cursorVersion,
		Workbench: l.Bench.Slug,
		Live:      bench.Digest(live),
		Archive:   bench.Digest(archive),
		Columns:   bench.Digest(columns),
	}
	// The cursor is read before the two filters, which is the order the
	// command's own check list declares: a call carrying a bad token is not a
	// call about a card or a column yet. A call carrying no token is still a
	// call about a card, so the filters are checked either way.
	minting := strings.TrimSpace(req.Since) == ""
	var held cursor
	if !minting {
		read, err := decodeCursor(req.Since)
		if err != nil {
			return nil, err
		}
		if read.Workbench != l.Bench.Slug {
			return nil, contract.Refuse(contract.Malformed, req.Since)
		}
		held = read
	}
	wantedCard, wantedColumn, err := l.changeFilters(req)
	if err != nil {
		return nil, err
	}
	if minting {
		return l.mintedChangeSet(terms, live, archive)
	}
	if held.Live == terms.Live && held.Archive == terms.Archive && held.Columns == terms.Columns {
		// The token comes back byte for byte rather than re-encoded, so a
		// caller comparing two answers compares tokens without decoding one.
		return &ChangeSet{Cursor: req.Since, Changed: false, Affordances: l.changeAffordances()}, nil
	}
	return l.changedSince(held, terms, live, archive, wantedCard, wantedColumn, l.dayOf(req))
}

// mintedChangeSet is the answer to a first call: a cursor and nothing else.
// A fresh session is asking what happens from now, and what ever happened is
// in each card's own journal.
//
// This is the one call that parses the whole bench and reports nothing. The
// position the token carries has to name the end of the total order as it
// stands, and only a parse can find it, because the design reads no clock and
// "everything up to now" is therefore a place in the journals rather than a
// moment. A cursor left without a position would replay the whole board on
// the next call that found the digest moved, which is exactly what a first
// call is specified not to do.
//
// The lines are folded into the cursor as they are read, in the order
// readHalf would deliver them, rather than gathered first: a fresh cursor
// delivers every line, so the fold over readHalf's answer and the fold line by
// line are the same fold, and the second builds no list of every line on the
// workbench to throw it away.
func (l *Library) mintedChangeSet(terms cursor, live, archive []bench.Watched) (*ChangeSet, error) {
	terms.Frontier = make(map[string]int, len(terms.Frontier))
	halves := []struct {
		entries []bench.Watched
		only    map[string]bool
	}{{live, nil}, {archive, archiveEvents}}
	for _, half := range halves {
		// The archive is read through the same filter a reporting call reads
		// it through, so the position a mint records can never sit on a line
		// no later call would deliver.
		for _, entry := range half.entries {
			if entry.Journal == "" {
				continue
			}
			key, only := entry.Key, half.only
			l.Bench.JournalLines(entry.Journal, func(index int, ts, event string) {
				if only == nil || only[event] {
					terms.cover(key, index, ts)
				}
			})
		}
	}
	if len(terms.Frontier) == 0 {
		terms.Frontier = nil
	}
	token, err := terms.encode()
	if err != nil {
		return nil, err
	}
	return &ChangeSet{Cursor: token, Changed: false, Affordances: l.changeAffordances()}, nil
}

// changeFilters resolves the two narrowing arguments, refusing over each by
// the name its own check list declares. They narrow what is reported and
// never what is read: the walk is always whole-bench, which is what makes
// "did my card leave the column I was watching" answerable at all.
func (l *Library) changeFilters(req *Request) (card string, column *bench.Column, err error) {
	if req.Card != "" {
		found, resolveErr := l.watchedCard(req.Card)
		if resolveErr != nil {
			return "", nil, resolveErr
		}
		card = found
	}
	if req.Column != "" {
		column = l.Bench.ColumnByRef(req.Column)
		if column == nil {
			return "", nil, contract.Refuse(contract.UnknownColumn, req.Column)
		}
	}
	return card, column, nil
}

// watchedCard resolves the card filter to the identifier the walk keys on,
// which is a wider question than resolving a card to act on.
//
// A card the caller is watching is the card most likely to have left, and the
// departure is the thing the caller was watching for, so a filter that refused
// the moment its subject was archived or deleted would be closed exactly when
// it was wanted. The resolution therefore reads the live half first, then the
// archive mirror, which is anchorOf's own order, and then accepts a
// well-formed identifier that resolves in neither, because a removed entry in
// gone carries an identifier and nothing else and an identifier is all a
// caller can match one by. A number more than one card of a half carries
// refuses dinah.ambiguous-card from that half rather than falling through to
// the other one, because a filter keyed on a card the caller did not name
// watches the wrong card and says nothing. Anything else still refuses
// UnknownCard, so a mistyped reference is caught rather than answered with
// silence.
//
// The mirror is reached only by a reference the live half already failed, so a
// call about a card that is still on the board never pays for it.
func (l *Library) watchedCard(ref string) (string, error) {
	found, err := l.Bench.ResolveCard(ref)
	if err == nil {
		return found.Card.ID, nil
	}
	var refusal *contract.Refusal
	if errors.As(err, &refusal) && refusal.Name == contract.AmbiguousCard {
		return "", refusal
	}
	found, err = l.Bench.ResolveArchivedCard(ref)
	if err == nil {
		return found.Card.ID, nil
	}
	if errors.As(err, &refusal) && refusal.Name == contract.AmbiguousCard {
		return "", refusal
	}
	if trimmed := strings.TrimSpace(ref); bench.IsID(trimmed) {
		return trimmed, nil
	}
	return "", contract.Refuse(contract.UnknownCard, ref)
}

// changedSince builds the answer to a call whose board moved: the events after
// the cursor, the live cards this call has a reason to report, what left, and
// the cursor that covers all of it. day is the request's day, which the
// reported cards' conditions and holds are drawn on.
func (l *Library) changedSince(held, terms cursor, live, archive []bench.Watched, wantedCard string, wantedColumn *bench.Column, day *requestDay) (*ChangeSet, error) {
	var delivered []position
	var unreadable, liveUnreadable []string
	if held.Live != terms.Live {
		read, unread := readHalf(l.Bench, live, held, nil)
		delivered = append(delivered, read...)
		unreadable = append(unreadable, unread...)
		liveUnreadable = append(liveUnreadable, unread...)
	}
	if held.Archive != terms.Archive {
		read, unread := readHalf(l.Bench, archive, held, archiveEvents)
		delivered = append(delivered, read...)
		unreadable = append(unreadable, unread...)
	}
	sort.SliceStable(delivered, func(i, j int) bool { return delivered[i].before(delivered[j]) })

	// The cursor's coverage grows over every line this walk delivered, before
	// any filter narrows what is reported. A cursor that covered only the
	// reported lines would tell a filtered caller about the same change on
	// every call for the rest of the session.
	advanced := terms
	advanced.TS, advanced.Frontier = held.TS, held.Frontier
	advanced = advanced.coverThrough(delivered)
	token, err := advanced.encode()
	if err != nil {
		return nil, err
	}

	answer := &ChangeSet{Cursor: token, Changed: true, Affordances: l.changeAffordances()}
	answer.Gone = l.goneFrom(delivered, wantedCard, wantedColumn)
	if held.Columns != terms.Columns {
		answer.Columns = l.columnChanges()
	}
	// Evidence is counted across every entity the walk delivered, not only
	// across cards. A workbench field rewrite, a workstream act, a deletion
	// and a completed archiving each move the live term and each is a
	// complete explanation of the movement, so none of them is a reason to
	// resync the board. A completed archiving is delivered out of the archive
	// half, so a delivered line counts wherever it was read from.
	//
	// An unreadable journal counts only from the live half. A corrupted
	// archived journal moves the archive term and says nothing whatever about
	// the live one, so letting it stand as the explanation for a moved live
	// term would suppress a resync the live half had earned.
	explained := len(delivered) > 0 || len(liveUnreadable) > 0
	changed, err := l.changedCards(delivered, unreadable, live, held.Live != terms.Live && !explained, wantedCard, wantedColumn, day)
	if err != nil {
		return nil, err
	}
	answer.Cards = changed
	answer.Events = l.eventsFrom(delivered, wantedCard, wantedColumn)
	answer.Unreadable = filterKeys(unreadable, wantedCard)
	return answer, nil
}

// readHalf parses one half of the walk and returns the lines after the cursor
// along with the keys of the journals that would not parse.
//
// bench.ReadJournal tolerates a malformed line only when it is the final one,
// and this design reads every journal on the bench, so an unhandled refusal
// would turn one entity's corruption into a dead checkpoint for the whole
// board. It does not: the entity's events are dropped, its key is named, and
// its fingerprint is still computed from os.Stat, so the terms advance past
// the corruption rather than reporting it forever.
func readHalf(b *bench.Bench, entries []bench.Watched, held cursor, only map[string]bool) (delivered []position, unreadable []string) {
	for _, entry := range entries {
		// An entity carrying no journal is skipped on the empty string
		// rather than on the error reading an empty path gives, which is a
		// fact about the struct rather than about an errno. A column is the
		// entity that reaches this, and reading its absence as corruption
		// would name it unreadable on every call.
		if entry.Journal == "" {
			continue
		}
		events, _, err := b.ReadJournal(entry.Journal)
		if err != nil {
			unreadable = append(unreadable, entry.Key)
			continue
		}
		for index, event := range events {
			if only != nil && !only[event.Event] {
				continue
			}
			at := position{key: entry.Key, index: index, event: event}
			if held.delivered(at) {
				continue
			}
			delivered = append(delivered, at)
		}
	}
	return delivered, unreadable
}

// eventsFrom renders the delivered lines for the answer, narrowed by whichever
// filters the caller named.
func (l *Library) eventsFrom(delivered []position, wantedCard string, wantedColumn *bench.Column) []ChangeEvent {
	var events []ChangeEvent
	for _, at := range delivered {
		scope, id := splitKey(at.key)
		if wantedCard != "" && (scope != ScopeCard || id != wantedCard) {
			continue
		}
		if wantedColumn != nil && !l.inColumn(scope, id, at.event, wantedColumn) {
			continue
		}
		events = append(events, ChangeEvent{Scope: scope, ID: id, Ref: l.entityRef(scope, id), Event: at.event})
	}
	return events
}

// inColumn decides whether a column filter admits one line.
//
// A card the filter admits is one that sits in the named column now, or one
// whose own line names that column on either side of a move. The second half is
// what makes "did my card leave the column I was watching" answerable at all: a
// card that left is no longer in the column, so a rule reading only where the
// card sits now would filter out the very departure the caller asked about.
//
// Nothing outside a card carries a column, so a workbench-scoped or
// workstream-scoped line is not admitted by a filter that asks about one.
func (l *Library) inColumn(scope, id string, event bench.Event, wanted *bench.Column) bool {
	if scope != ScopeCard {
		return false
	}
	if event.From == wanted.ID || event.To == wanted.ID {
		return true
	}
	card, err := l.Bench.LoadCardIn(l.Bench.CardsRoot(), id)
	return err == nil && card.Column == wanted.ID
}

// changedCards reports the live cards this call has a per-card reason to
// report, as they now stand.
//
// Two reasons are per-card and computable: the card's journal carries lines
// after the cursor, or its journal would not parse, which makes a resync the
// only honest answer for it. A card carrying an archived event after the
// cursor is reported in gone instead, never in both, because an archived card
// has no live state a caller would act on even on the interrupted path where
// its directory has not moved yet.
//
// The third case has no evidence anywhere. An anchor rewritten with no
// journal line, which is what dinah edit produces, moves the live term and
// leaves nothing behind that names which entity moved it, and the cursor
// carries digests rather than per-entity state, so no comparison can single
// the card out. Only then, when the live term moved and the walk delivered
// nothing at all to explain it, does the call report every live card, which is
// a resync and is the answer a caller can act on. Attributing that case
// exactly would need the cursor to carry a term per entity, which is the
// token-growth tradeoff this design rejected.
//
// unexplained is that last case and nothing wider. It is decided by the
// caller, over every entity the walk delivered rather than over cards alone,
// because a workbench field rewrite, a workstream act, a deletion and a
// completed archiving all move the live term and all explain it.
func (l *Library) changedCards(delivered []position, unreadable []string, live []bench.Watched, unexplained bool, wantedCard string, wantedColumn *bench.Column, day *requestDay) ([]*CardView, error) {
	named := map[string]bool{}
	departed := map[string]bool{}
	for _, at := range delivered {
		scope, id := splitKey(at.key)
		if scope != ScopeCard {
			continue
		}
		if at.event.Event == contract.EventArchived {
			departed[id] = true
		}
		named[id] = true
	}
	for _, key := range unreadable {
		if scope, id := splitKey(key); scope == ScopeCard {
			named[id] = true
		}
	}
	ids := liveCardIDs(live)
	reported := map[string]bool{}
	for _, id := range ids {
		if named[id] {
			reported[id] = true
		}
	}
	if unexplained {
		for _, id := range ids {
			reported[id] = true
		}
	}
	var views []*CardView
	for _, id := range ids {
		if !reported[id] || departed[id] {
			continue
		}
		if wantedCard != "" && id != wantedCard {
			continue
		}
		card, err := l.Bench.LoadCardIn(l.Bench.CardsRoot(), id)
		if err != nil {
			continue
		}
		if wantedColumn != nil && card.Column != wantedColumn.ID {
			continue
		}
		view, err := l.view(card, day)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// columnChanges reports the flow as it now stands, for a call whose column
// term moved.
//
// It answers every live column rather than the one that moved, and that is
// forced rather than chosen. A column has no journal, so no line names the
// column an edit touched, and the cursor carries one term over the whole
// column half rather than a revision per column, so there is nothing to
// compare a single column against. This is changedCards' own unexplained
// case, reached every time instead of rarely, and it is affordable here for
// the reason that case is expensive there: the flow is a handful of anchors
// the bench has already read, while the cards are the board.
//
// Nothing here reads a card. The entry carries the column's identity and
// which way it holds, and no occupancy, because tallying cards per column is
// a full read of every card anchor and that read is the cost the column
// digest term exists to avoid.
func (l *Library) columnChanges() []ColumnChange {
	var changes []ColumnChange
	for _, column := range l.Bench.Columns {
		changes = append(changes, ColumnChange{
			ID:           column.ID,
			Slug:         column.Slug,
			Title:        column.Title,
			HoldsOnEntry: column.HoldsOnEntry(),
			HoldsOnExit:  column.HoldsOnExit(),
		})
	}
	return changes
}

// goneFrom derives what left from the events this call delivered, never from
// the cursor, which carries digests, and a digest enumerates nothing.
//
// An archived entry is provably a card, because the event was read out of a
// journal whose key is cards/<id>, so the subject comes from the key and the
// event's own note is never consulted. A removed entry comes from a deleted
// event in the workbench journal, which names neither the kind of the thing
// it removed nor a reference to it, so the entry carries the identifier and
// the title and claims nothing more.
func (l *Library) goneFrom(delivered []position, wantedCard string, wantedColumn *bench.Column) []GoneEntity {
	var gone []GoneEntity
	for _, at := range delivered {
		scope, id := splitKey(at.key)
		switch {
		case scope == ScopeCard && at.event.Event == contract.EventArchived:
			if wantedCard != "" && id != wantedCard {
				continue
			}
			entry := GoneEntity{ID: id, Kind: ScopeCard, Fate: FateArchived}
			card := l.anchorOf(id)
			if card != nil {
				entry.Ref = card.Ref(l.Bench.Slug)
				entry.Title = card.Title
			}
			// A column filter reaches an archived entry through the column its
			// surviving anchor records. An entry whose anchor is gone carries
			// no column to match, so a filtered call cannot report it.
			if wantedColumn != nil && (card == nil || card.Column != wantedColumn.ID) {
				continue
			}
			gone = append(gone, entry)
		case scope == ScopeWorkbench && at.event.Event == contract.EventDeleted:
			// A removed entry is exempt from both filters. It carries no
			// column to match and its reference no longer resolves, and a
			// filtered caller told about one identifier it does not
			// recognise is better served than one whose own card's
			// destruction was filtered out for want of a column.
			gone = append(gone, GoneEntity{ID: at.event.Note, Title: at.event.Title, Fate: FateRemoved})
		}
	}
	return gone
}

// anchorOf loads a card's anchor wherever it currently sits, the live half
// first and the mirror second, which is the order linkRef already reads in.
// The order matters on the interrupted-archive path: the event is written
// before the directory moves, so a crash between the two leaves the anchor in
// the live half while the event that names its departure has already been
// delivered.
func (l *Library) anchorOf(id string) *bench.Card {
	if card, err := l.Bench.LoadCardIn(l.Bench.CardsRoot(), id); err == nil {
		return card
	}
	if card, err := l.Bench.LoadCardIn(l.Bench.ArchivedCardsRoot(), id); err == nil {
		return card
	}
	return nil
}

// entityRef composes what a person types to reach the entity a line came from,
// when an anchor still exists to compose it from. It is a convenience, so a
// reference that cannot be composed is left empty rather than guessed at.
func (l *Library) entityRef(scope, id string) string {
	switch scope {
	case ScopeCard:
		if card := l.anchorOf(id); card != nil {
			return card.Ref(l.Bench.Slug)
		}
	case ScopeWorkstream:
		if workstream := l.Bench.Workstream(id); workstream != nil {
			return workstream.Ref()
		}
	}
	return ""
}

// changeAffordances names what a caller may do next after a checkpoint: read
// the board it was just told about.
func (l *Library) changeAffordances() []string {
	return []string{"status", "list", "show"}
}

// liveCardIDs lists the identifiers of the live half of the walk, in key
// order, which is identifier order within the collection.
func liveCardIDs(live []bench.Watched) []string {
	var ids []string
	for _, entry := range live {
		if scope, id := splitKey(entry.Key); scope == ScopeCard {
			ids = append(ids, id)
		}
	}
	return ids
}

// filterKeys narrows a list of entity keys to the card a caller named, so an
// unreadable journal belonging to somebody else's card does not reach a
// filtered answer.
//
// A key outside the cards collection is kept whatever the filter says,
// because the filter cannot rule on it. The workbench journal is the one that
// matters: deleted events are written there, so an unparseable workbench
// journal means gone cannot report a removal in that window, and a caller
// filtered to the very card that was destroyed would otherwise be handed an
// empty answer with nothing in it to say that the journal which would have
// carried the news went unread. That is the silent absence of a departure
// this whole verb exists to prevent, so an unrulable key is reported rather
// than dropped.
func filterKeys(keys []string, wantedCard string) []string {
	if wantedCard == "" {
		return keys
	}
	var kept []string
	for _, key := range keys {
		if scope, id := splitKey(key); scope != ScopeCard || id == wantedCard {
			kept = append(kept, key)
		}
	}
	return kept
}

// splitKey reads an entity key back into the scope it names and the
// identifier under it. The workbench carries no identifier, because there is
// exactly one of it.
func splitKey(key string) (scope, id string) {
	collection, identifier, found := strings.Cut(key, "/")
	if !found {
		return ScopeWorkbench, ""
	}
	switch collection {
	case bench.CardsDir:
		return ScopeCard, identifier
	case bench.WorkstreamsDir:
		return ScopeWorkstream, identifier
	}
	return collection, identifier
}
