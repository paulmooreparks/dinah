package bench

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	// Reason is a block's prose reason, or the reason a raise gave for
	// requiring more of a card than it required a moment ago. Only block and
	// raise populate it: an ordinary per-column tier write carries none, and
	// a reader meeting a tier_overridden line with no reason is meeting one
	// of those rather than a raise that omitted it.
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
	// Scheme and Target are the citation an item_cited event recorded, as the
	// caller typed them. Nothing here resolves either: a citation is taken at
	// its word at write time, and dinah check is what tells a reader it was
	// wrong.
	Scheme string `json:"scheme,omitempty"`
	Target string `json:"target,omitempty"`
	// Note is the human's free prose, unparseable by design.
	Note string `json:"note,omitempty"`
}

// AppendEvent adds one line to a journal, creating it when absent. The write
// is an append rather than a rewrite, so a crash can tear at most the final
// line and never the records already in the file.
func AppendEvent(path string, ev Event) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// ReadJournal reads a journal in file order, which is event order. A torn
// final line left by a crash is skipped rather than failing the read, and an
// absent journal is an empty history.
//
// The second return value reports whether a trailing partial line was found,
// which is what check reports and trims with a witness.
func ReadJournal(path string) ([]Event, bool, error) {
	text, err := ReadText(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
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
			return nil, torn, err
		}
		events = append(events, ev)
	}
	return events, torn, nil
}
