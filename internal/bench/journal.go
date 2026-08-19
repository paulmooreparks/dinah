package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

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
	// Actor is who acted, self-declared attribution rather than authority.
	Actor string `json:"actor"`
	// Title is the card's own title, carried by the created event.
	Title string `json:"title,omitempty"`
	// From and To are the state identifiers a move left and entered, and the
	// values a workbench_updated event rewrote a field from and to.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// FromTitle and ToTitle are those states' titles as of the move.
	FromTitle string `json:"from_title,omitempty"`
	ToTitle   string `json:"to_title,omitempty"`
	// Override marks the one act a move admitted under CORE-MOVE-9 records.
	Override bool `json:"override,omitempty"`
	// Reason is a block's prose reason.
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
