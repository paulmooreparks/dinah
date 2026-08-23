package verb

import (
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"

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
	// stand, so a caller learns the new state without a second call.
	Cards []*CardView `json:"cards,omitempty"`
	// Gone reports what left, one entry per archived or deleted event after
	// the cursor's position. Read Kind before assuming an entry is a card.
	Gone []GoneEntity `json:"gone,omitempty"`
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
// the departure and nothing else. Nothing appends to an archived journal in
// any case, since no verb takes an archived card as its subject.
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
const cursorVersion = 1

// cursor is what a caller hands back, rendered as base64url of this object.
//
// The two digest terms answer "did anything change" against file bytes, and
// the ts/entity/index triple is the position of the last event delivered
// under the total order the merged read imposes. The triple is needed
// because bench.TimeFormat is second resolution, so two events sharing a
// timestamp are ordinary rather than exotic and a bare timestamp would drop
// one of them.
//
// index is the index into the slice bench.ReadJournal returns rather than a
// physical line number. That reader skips blank lines and a torn final line,
// so the two differ, and the parsed index is the one that stays stable when a
// torn tail is later appended past or trimmed by check.
type cursor struct {
	Version int `json:"v"`
	// Workbench is the slug of the workbench that issued the token. The
	// member is spelled in full rather than shortened, which is the product's
	// own word everywhere a reader meets one.
	Workbench string `json:"workbench"`
	Live      string `json:"live"`
	Archive   string `json:"archive"`
	TS        string `json:"ts,omitempty"`
	Entity    string `json:"entity,omitempty"`
	Index     int    `json:"index,omitempty"`
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
	if read.Version != cursorVersion || read.Live == "" || read.Archive == "" {
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
	mine, theirs := bench.ParseStamp(p.event.TS), bench.ParseStamp(other.event.TS)
	if !mine.Equal(theirs) {
		return mine.Before(theirs)
	}
	if p.event.TS != other.event.TS {
		return p.event.TS < other.event.TS
	}
	if p.key != other.key {
		return p.key < other.key
	}
	return p.index < other.index
}

// after reports whether a position falls after the cursor's, which is what
// decides whether a line is delivered. A cursor carrying no position covers
// nothing, so every line is after it.
func (p position) after(c cursor) bool {
	if c.TS == "" {
		return true
	}
	mark := position{key: c.Entity, index: c.Index, event: bench.Event{TS: c.TS}}
	return mark.before(p)
}

// Changes answers one checkpoint: what has happened on this workbench since
// the caller's cursor, and a fresh cursor to ask with next time.
//
// It reads and never writes. No lock is taken, no journal is appended, no
// anchor is saved and no basis is consumed, which is why it does not lapse an
// expired claim the way the other reads of the bench do: reporting a card as
// stored is the honest answer from a call that is not allowed to change it.
func (l *Library) Changes(req *Request) (*ChangeSet, error) {
	live, archive := l.Bench.WatchedEntities()
	terms := cursor{
		Version:   cursorVersion,
		Workbench: l.Bench.Slug,
		Live:      bench.Digest(live),
		Archive:   bench.Digest(archive),
	}
	// The cursor is read before the two filters, which is the order the
	// command's own check list declares: a call carrying a bad token is not a
	// call about a card or a state yet. A call carrying no token is still a
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
	wantedCard, wantedState, err := l.changeFilters(req)
	if err != nil {
		return nil, err
	}
	if minting {
		return l.mintedChangeSet(terms, live, archive)
	}
	if held.Live == terms.Live && held.Archive == terms.Archive {
		// The token comes back byte for byte rather than re-encoded, so a
		// caller comparing two answers compares tokens without decoding one.
		return &ChangeSet{Cursor: req.Since, Changed: false, Affordances: l.changeAffordances()}, nil
	}
	return l.changedSince(held, terms, live, archive, wantedCard, wantedState)
}

// mintedChangeSet is the answer to a first call: a cursor and nothing else.
// A fresh session is asking what happens from now, and what ever happened is
// what log answers, per card.
//
// This is the one call that parses the whole bench and reports nothing. The
// position the token carries has to name the end of the total order as it
// stands, and only a parse can find it, because the design reads no clock and
// "everything up to now" is therefore a place in the journals rather than a
// moment. A cursor left without a position would replay the whole board on
// the next call that found the digest moved, which is exactly what a first
// call is specified not to do.
func (l *Library) mintedChangeSet(terms cursor, live, archive []bench.Watched) (*ChangeSet, error) {
	var last position
	found := false
	halves := []struct {
		entries []bench.Watched
		only    map[string]bool
	}{{live, nil}, {archive, archiveEvents}}
	for _, half := range halves {
		// The archive is read through the same filter a reporting call reads
		// it through, so the position a mint records can never sit on a line
		// no later call would deliver.
		delivered, _ := readHalf(half.entries, cursor{}, half.only)
		for _, at := range delivered {
			if !found || last.before(at) {
				last, found = at, true
			}
		}
	}
	if found {
		terms.TS, terms.Entity, terms.Index = last.event.TS, last.key, last.index
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
// "did my card leave the state I was watching" answerable at all.
func (l *Library) changeFilters(req *Request) (card string, state *bench.State, err error) {
	if req.Card != "" {
		found, resolveErr := l.watchedCard(req.Card)
		if resolveErr != nil {
			return "", nil, resolveErr
		}
		card = found
	}
	if req.State != "" {
		state = l.Bench.StateByRef(req.State)
		if state == nil {
			return "", nil, contract.Refuse(contract.UnknownState, req.State)
		}
	}
	return card, state, nil
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
// caller can match one by. Anything else still refuses UnknownCard, so a
// mistyped reference is caught rather than answered with silence.
//
// The mirror is reached only by a reference the live half already failed, so a
// call about a card that is still on the board never pays for it.
func (l *Library) watchedCard(ref string) (string, error) {
	if found, err := l.Bench.ResolveCard(ref); err == nil {
		return found.Card.ID, nil
	}
	if found, err := l.Bench.ResolveArchivedCard(ref); err == nil {
		return found.Card.ID, nil
	}
	if trimmed := strings.TrimSpace(ref); bench.IsID(trimmed) {
		return trimmed, nil
	}
	return "", contract.Refuse(contract.UnknownCard, ref)
}

// changedSince builds the answer to a call whose board moved: the events after
// the cursor, the live cards this call has a reason to report, what left, and
// the cursor that covers all of it.
func (l *Library) changedSince(held, terms cursor, live, archive []bench.Watched, wantedCard string, wantedState *bench.State) (*ChangeSet, error) {
	var delivered []position
	var unreadable []string
	if held.Live != terms.Live {
		read, unread := readHalf(live, held, nil)
		delivered = append(delivered, read...)
		unreadable = append(unreadable, unread...)
	}
	if held.Archive != terms.Archive {
		read, unread := readHalf(archive, held, archiveEvents)
		delivered = append(delivered, read...)
		unreadable = append(unreadable, unread...)
	}
	sort.SliceStable(delivered, func(i, j int) bool { return delivered[i].before(delivered[j]) })

	// The cursor advances over every line this walk delivered, before any
	// filter narrows what is reported. A cursor that advanced only over the
	// reported lines would tell a filtered caller about the same change on
	// every call for the rest of the session.
	advanced := terms
	if len(delivered) > 0 {
		last := delivered[len(delivered)-1]
		advanced.TS, advanced.Entity, advanced.Index = last.event.TS, last.key, last.index
	} else {
		advanced.TS, advanced.Entity, advanced.Index = held.TS, held.Entity, held.Index
	}
	token, err := advanced.encode()
	if err != nil {
		return nil, err
	}

	answer := &ChangeSet{Cursor: token, Changed: true, Affordances: l.changeAffordances()}
	answer.Gone = l.goneFrom(delivered, wantedCard, wantedState)
	// Evidence is counted across every entity the walk delivered, not only
	// across cards. A workbench field rewrite, a workstream act, a deletion
	// and a completed archiving each move the live term and each is a
	// complete explanation of the movement, so none of them is a reason to
	// resync the board.
	explained := len(delivered) > 0 || len(unreadable) > 0
	answer.Cards = l.changedCards(delivered, unreadable, live, held.Live != terms.Live && !explained, wantedCard, wantedState)
	answer.Events = l.eventsFrom(delivered, wantedCard, wantedState)
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
func readHalf(entries []bench.Watched, held cursor, only map[string]bool) (delivered []position, unreadable []string) {
	for _, entry := range entries {
		events, _, err := bench.ReadJournal(entry.Journal)
		if err != nil {
			unreadable = append(unreadable, entry.Key)
			continue
		}
		for index, event := range events {
			if only != nil && !only[event.Event] {
				continue
			}
			at := position{key: entry.Key, index: index, event: event}
			if !at.after(held) {
				continue
			}
			delivered = append(delivered, at)
		}
	}
	return delivered, unreadable
}

// eventsFrom renders the delivered lines for the answer, narrowed by whichever
// filters the caller named.
func (l *Library) eventsFrom(delivered []position, wantedCard string, wantedState *bench.State) []ChangeEvent {
	var events []ChangeEvent
	for _, at := range delivered {
		scope, id := splitKey(at.key)
		if wantedCard != "" && (scope != ScopeCard || id != wantedCard) {
			continue
		}
		if wantedState != nil && !l.inState(scope, id, at.event, wantedState) {
			continue
		}
		events = append(events, ChangeEvent{Scope: scope, ID: id, Ref: l.entityRef(scope, id), Event: at.event})
	}
	return events
}

// inState decides whether a state filter admits one line.
//
// A card the filter admits is one that sits in the named state now, or one
// whose own line names that state on either side of a move. The second half is
// what makes "did my card leave the state I was watching" answerable at all: a
// card that left is no longer in the state, so a rule reading only where the
// card sits now would filter out the very departure the caller asked about.
//
// Nothing outside a card carries a state, so a workbench-scoped or
// workstream-scoped line is not admitted by a filter that asks about one.
func (l *Library) inState(scope, id string, event bench.Event, wanted *bench.State) bool {
	if scope != ScopeCard {
		return false
	}
	if event.From == wanted.ID || event.To == wanted.ID {
		return true
	}
	card, err := bench.LoadCard(l.Bench.CardsRoot(), id)
	return err == nil && card.State == wanted.ID
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
func (l *Library) changedCards(delivered []position, unreadable []string, live []bench.Watched, unexplained bool, wantedCard string, wantedState *bench.State) []*CardView {
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
		card, err := bench.LoadCard(l.Bench.CardsRoot(), id)
		if err != nil {
			continue
		}
		if wantedState != nil && card.State != wantedState.ID {
			continue
		}
		views = append(views, l.view(card))
	}
	return views
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
func (l *Library) goneFrom(delivered []position, wantedCard string, wantedState *bench.State) []GoneEntity {
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
			// A state filter reaches an archived entry through the state its
			// surviving anchor records. An entry whose anchor is gone carries
			// no state to match, so a filtered call cannot report it.
			if wantedState != nil && (card == nil || card.State != wantedState.ID) {
				continue
			}
			gone = append(gone, entry)
		case scope == ScopeWorkbench && at.event.Event == contract.EventDeleted:
			// A removed entry is exempt from both filters. It carries no
			// state to match and its reference no longer resolves, and a
			// filtered caller told about one identifier it does not
			// recognise is better served than one whose own card's
			// destruction was filtered out for want of a state.
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
	if card, err := bench.LoadCard(l.Bench.CardsRoot(), id); err == nil {
		return card
	}
	if card, err := bench.LoadCard(l.Bench.ArchivedCardsRoot(), id); err == nil {
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
	return []string{"status", "ls", "show", "log"}
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
func filterKeys(keys []string, wantedCard string) []string {
	if wantedCard == "" {
		return keys
	}
	var kept []string
	for _, key := range keys {
		if scope, id := splitKey(key); scope == ScopeCard && id == wantedCard {
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
