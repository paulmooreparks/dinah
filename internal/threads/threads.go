// Package threads keeps a document's comment threads apart from its text
// (docs/design/document-comments.md). A thread written into a document's
// body would be seen by every reader of the document, so a thread is kept in
// a record of its own and anchored by the words it is about, with the words
// around them, never by a marker in the text.
//
// A record belongs to a document, not to a working copy, so that it outlives
// the working copies the drafts forget: a document by the identity its
// source gives it (a page on a site by the site and the page's ID, a file by
// its workspace, source and path), and a document not yet saved anywhere by
// its draft. Every change is made under the file's lock (package statefile),
// so more than one program may write records.
package threads

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"studio/internal/statefile"
)

// A thread's kinds: a comment a person opened, a question an agent asks the
// person, and a finding of a review.
const (
	KindComment  = "comment"
	KindQuestion = "question"
	KindFinding  = "finding"
)

// Who writes a message: a person or an agent.
const (
	ByPerson = "person"
	ByAgent  = "agent"
)

// What a message does to its thread.
const (
	MessageOpen    = "open"
	MessageReply   = "reply"
	MessageResolve = "resolve"
	MessageReopen  = "reopen"
)

// Message is one entry of a thread.
type Message struct {
	Author string `json:"author"`
	By     string `json:"by"`
	Kind   string `json:"kind"`
	Text   string `json:"text"`
	// At is when the message was written; one given without a time, as is
	// usual, is given the time it is kept.
	At time.Time `json:"at"`
	// Native is the comment on the document's own site a message was
	// brought in from, or sent as, so that bringing the site's comments in
	// again adds only new ones and nothing is sent twice.
	Native string `json:"native,omitempty"`
	// Brought marks a message brought in from the document's own site, where
	// a reader of the site wrote it. A message written here and sent to the
	// site is not marked.
	Brought bool `json:"brought,omitempty"`
}

// Thread is a discussion about a passage of a document.
type Thread struct {
	ID string `json:"id"`
	// Key is the opener's own stable name for the thread, such as a review
	// finding's fingerprint, so that opening it again finds this thread
	// rather than making a second one.
	Key      string `json:"key,omitempty"`
	Kind     string `json:"kind"`
	Severity string `json:"severity,omitempty"`
	// Quote is the text the thread is about, Prefix and Suffix the text
	// around it when it was opened, and Start where it was last found, as a
	// byte offset in the document's text.
	Quote    string    `json:"quote"`
	Prefix   string    `json:"prefix,omitempty"`
	Suffix   string    `json:"suffix,omitempty"`
	Start    int       `json:"start"`
	Resolved bool      `json:"resolved,omitempty"`
	Messages []Message `json:"messages"`
	// Native is the comment on the document's own site the thread was
	// brought in from, or sent as.
	Native  string    `json:"native,omitempty"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// Due reports whether the thread waits on an agent: it is open and its last
// word is a person's, a person's own comment or a reply to an agent.
func (t Thread) Due() bool {
	return !t.Resolved && len(t.Messages) > 0 && t.Messages[len(t.Messages)-1].By == ByPerson
}

// Waiting reports whether the thread waits on a person: it is open and its
// last word is an agent's.
func (t Thread) Waiting() bool {
	return !t.Resolved && len(t.Messages) > 0 && t.Messages[len(t.Messages)-1].By == ByAgent
}

// record is a document's file.
type record struct {
	Document string   `json:"document"`
	Threads  []Thread `json:"threads"`
}

// Book is the records in a folder.
type Book struct {
	dir string
	now func() time.Time
	// Place, when set, says where a document's record is kept in place of
	// the state folder: the comments file of its Working Material, or "" for
	// the state folder. A record kept in the state folder before is moved
	// there the first time it is changed. The lock is the state folder's,
	// so the person's folder holds no lock file.
	Place func(doc string) string
}

// where is the file a document's record is kept in, and the file whose
// lock guards it.
func (b *Book) where(doc string) (file, lock string) {
	state := b.path(doc)
	if b.Place != nil {
		if p := b.Place(doc); p != "" {
			return p, state
		}
	}
	return state, state
}

// Open returns the records kept in the state folder state.
func Open(state string) *Book {
	return &Book{dir: filepath.Join(state, "threads"), now: time.Now}
}

// DocumentKey is the identity of a document whose threads are kept, when
// its source gives it none of its own: its kind, workspace, source and
// path. A source whose documents have a stable identity of their own, such
// as a page's ID on its site, which stays when the page is renamed or moved,
// gives that instead (workspaces.SourceKind.DocumentKey).
func DocumentKey(kind, workspaceID, sourceID, path string) string {
	return kind + ":" + workspaceID + "/" + sourceID + "/" + path
}

// DraftKey is the identity of a document that exists only as a draft.
func DraftKey(draftID string) string { return "draft:" + draftID }

func (b *Book) path(doc string) string {
	sum := sha256.Sum256([]byte(doc))
	return filepath.Join(b.dir, hex.EncodeToString(sum[:16])+".json")
}

// ErrNoThread means no thread of that ID is kept for the document.
var ErrNoThread = errors.New("there is no such thread on this document")

// List returns a document's threads, oldest first.
func (b *Book) List(doc string) ([]Thread, error) {
	file, lock := b.where(doc)
	data, err := statefile.ReadSharedWithLock(file, lock)
	if errors.Is(err, os.ErrNotExist) && file != lock {
		// Not moved yet: the record is still in the state folder.
		data, err = statefile.ReadShared(lock)
	}
	if errors.Is(err, os.ErrNotExist) {
		return []Thread{}, nil
	}
	if err != nil {
		return nil, err
	}
	var r record
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("the document's threads could not be read: %w", err)
	}
	if r.Threads == nil {
		r.Threads = []Thread{}
	}
	return r.Threads, nil
}

func (b *Book) change(doc string, fn func(r *record) error) error {
	if err := os.MkdirAll(b.dir, 0o700); err != nil {
		return err
	}
	file, lock := b.where(doc)
	moved := false
	err := statefile.UpdateWithLock(file, lock, func(previous []byte) ([]byte, error) {
		r := record{Document: doc}
		if previous == nil && file != lock {
			// The record moves from the state folder to its place.
			if old, err := statefile.ReadRegular(lock); err == nil {
				previous, moved = old, true
			}
		}
		if previous != nil {
			if err := json.Unmarshal(previous, &r); err != nil {
				return nil, fmt.Errorf("the document's threads could not be read: %w", err)
			}
		}
		if err := fn(&r); err != nil {
			return nil, err
		}
		return json.MarshalIndent(r, "", "  ")
	})
	if err != nil || !moved {
		return err
	}
	// The state folder's copy is emptied once the record is in its place,
	// so that it is never read again in place of the moved one.
	return statefile.Update(lock, func([]byte) ([]byte, error) {
		return json.Marshal(record{Document: doc, Threads: []Thread{}})
	})
}

// New is what opening a thread needs: the text of the document and the span
// of it the thread is about, and its first message.
type New struct {
	Key      string
	Kind     string
	Severity string
	Text     string
	Start    int
	End      int
	Message  Message
	Native   string
}

// Add opens a thread. When a thread with the same key is already kept, it
// is returned and nothing changes, so an opener may repeat itself; created
// says whether a thread was made.
func (b *Book) Add(doc string, n New) (Thread, bool, error) {
	if n.Start < 0 || n.End < n.Start || n.End > len(n.Text) {
		return Thread{}, false, errors.New("the passage is not inside the document")
	}
	if strings.TrimSpace(n.Message.Text) == "" {
		return Thread{}, false, errors.New("a thread needs words")
	}
	kind := n.Kind
	if kind == "" {
		kind = KindComment
	}
	var out Thread
	created := false
	err := b.change(doc, func(r *record) error {
		for _, t := range r.Threads {
			if n.Key != "" && t.Key == n.Key || n.Native != "" && t.Native == n.Native {
				out = t
				return nil
			}
		}
		now := b.now().UTC()
		m := n.Message
		m.Kind = MessageOpen
		if m.At.IsZero() {
			m.At = now
		}
		quote, prefix, suffix := span(n.Text, n.Start, n.End)
		out = Thread{ID: newID(), Key: n.Key, Kind: kind, Severity: n.Severity, Quote: quote, Prefix: prefix, Suffix: suffix, Start: n.Start,
			Messages: []Message{m}, Native: n.Native, Created: now, Updated: now}
		r.Threads = append(r.Threads, out)
		created = true
		return nil
	})
	return out, created, err
}

// Reply adds a message to a thread. A person's reply to a resolved thread
// opens it again (Paul, 17 October 2026: the agent resolves what it acted on,
// and the person reopens a thread by replying).
func (b *Book) Reply(doc, id string, m Message) (Thread, error) {
	if strings.TrimSpace(m.Text) == "" {
		return Thread{}, errors.New("a reply needs words")
	}
	return b.update(doc, id, func(t *Thread, now time.Time) {
		m.Kind = MessageReply
		if m.At.IsZero() {
			m.At = now
		}
		if t.Resolved && m.By == ByPerson {
			m.Kind, t.Resolved = MessageReopen, false
		}
		t.Messages = append(t.Messages, m)
	})
}

// Resolve closes a thread, with an optional last word.
func (b *Book) Resolve(doc, id string, m Message) (Thread, error) {
	return b.update(doc, id, func(t *Thread, now time.Time) {
		if t.Resolved {
			return
		}
		m.Kind = MessageResolve
		if m.At.IsZero() {
			m.At = now
		}
		t.Messages = append(t.Messages, m)
		t.Resolved = true
	})
}

// Unsent reports whether a thread has something the document's site has
// not been sent: an open thread never sent, or a message of a sent thread
// that was not. A resolved thread never sent stays here alone.
func (t Thread) Unsent() bool {
	if t.Native == "" {
		return !t.Resolved
	}
	for _, m := range t.Messages {
		if m.Native == "" {
			return true
		}
	}
	return false
}

// Sent records that a thread was sent to the document's site: the comment
// it became, when it had none, and for each message sent, what it became
// there. A message is told by its kind, time, author and words.
func (b *Book) Sent(doc, id, native string, sent []Message) (Thread, error) {
	return b.update(doc, id, func(t *Thread, now time.Time) {
		if t.Native == "" {
			t.Native = native
		}
		for _, s := range sent {
			for i := range t.Messages {
				m := &t.Messages[i]
				if m.Native == "" && m.Kind == s.Kind && m.At.Equal(s.At) && m.Author == s.Author && m.Text == s.Text {
					m.Native = s.Native
					break
				}
			}
		}
	})
}

// MarkBrought marks the messages that were brought in from the document's
// site before messages were marked as they arrived. written gives, for each
// comment on the site, the time the site says it was written. A message
// kept for one of those comments with that same time was brought in; one
// written here and sent has the time it was written here, and stays
// unmarked. It reports whether a message was marked, and writes nothing
// when none was.
func (b *Book) MarkBrought(doc string, written map[string]time.Time) (bool, error) {
	kept, err := b.List(doc)
	if err != nil {
		return false, err
	}
	late := func(m Message) bool {
		at, ok := written[m.Native]
		return ok && !m.Brought && m.By == ByPerson && !at.IsZero() && m.At.Equal(at)
	}
	found := false
	for _, t := range kept {
		for _, m := range t.Messages {
			found = found || late(m)
		}
	}
	if !found {
		return false, nil
	}
	return true, b.change(doc, func(r *record) error {
		for i := range r.Threads {
			for j := range r.Threads[i].Messages {
				if late(r.Threads[i].Messages[j]) {
					r.Threads[i].Messages[j].Brought = true
				}
			}
		}
		return nil
	})
}

// Reopen opens a resolved thread again, with an optional word, as when a
// colleague reopens its comment on the document's site.
func (b *Book) Reopen(doc, id string, m Message) (Thread, error) {
	return b.update(doc, id, func(t *Thread, now time.Time) {
		if !t.Resolved {
			return
		}
		m.Kind = MessageReopen
		if m.At.IsZero() {
			m.At = now
		}
		t.Messages = append(t.Messages, m)
		t.Resolved = false
	})
}

func (b *Book) update(doc, id string, fn func(t *Thread, now time.Time)) (Thread, error) {
	var out Thread
	err := b.change(doc, func(r *record) error {
		for i := range r.Threads {
			if r.Threads[i].ID == id {
				now := b.now().UTC()
				fn(&r.Threads[i], now)
				r.Threads[i].Updated = now
				out = r.Threads[i]
				return nil
			}
		}
		return ErrNoThread
	})
	return out, err
}

// Move gives a document's threads to its new identity, as when a draft is
// saved for the first time or a document is saved under a new home with
// Save As. Threads the new identity already has are kept, and a thread of
// the same key or the same comment on the site is not added twice.
func (b *Book) Move(from, to string) error {
	if from == to {
		return nil
	}
	if err := b.Copy(from, to); err != nil {
		return err
	}
	return b.change(from, func(r *record) error {
		r.Threads = []Thread{}
		return nil
	})
}

// Merge brings a copy of a document's threads kept elsewhere, such as a
// record kept beside the document that colleagues change, into the
// document's record: a thread the record lacks is added, and one it has
// gains the messages it lacks, matched by their time, author and words, in
// the order they were written. A thread or message the copy says was sent to
// the site is recorded as sent here too. Whether a thread is resolved
// follows its messages, the last resolve or reopen deciding. It returns how
// many threads and messages were new or newly recorded as sent.
func (b *Book) Merge(doc string, incoming []Thread) (threads, messages int, err error) {
	if len(incoming) == 0 {
		return 0, 0, nil
	}
	err = b.change(doc, func(r *record) error {
		for _, in := range incoming {
			at := -1
			for i, have := range r.Threads {
				if have.ID == in.ID || in.Key != "" && have.Key == in.Key || in.Native != "" && have.Native == in.Native {
					at = i
					break
				}
			}
			if at < 0 {
				in.Resolved = resolvedBy(in.Messages)
				r.Threads = append(r.Threads, in)
				threads++
				messages += len(in.Messages)
				continue
			}
			have := &r.Threads[at]
			seen := map[string]int{}
			for i, m := range have.Messages {
				seen[messageKey(m)] = i
			}
			added := false
			for _, m := range in.Messages {
				i, ok := seen[messageKey(m)]
				if !ok {
					have.Messages = append(have.Messages, m)
					seen[messageKey(m)] = len(have.Messages) - 1
					messages++
					added = true
					continue
				}
				// A message another computer has sent to the site is sent;
				// it is not sent again from here.
				if have.Messages[i].Native == "" && m.Native != "" {
					have.Messages[i].Native = m.Native
					messages++
				}
			}
			if have.Native == "" && in.Native != "" {
				have.Native = in.Native
			}
			if added {
				sort.SliceStable(have.Messages, func(i, j int) bool { return have.Messages[i].At.Before(have.Messages[j].At) })
				have.Resolved = resolvedBy(have.Messages)
				if in.Updated.After(have.Updated) {
					have.Updated = in.Updated
				}
			}
		}
		return nil
	})
	return threads, messages, err
}

func messageKey(m Message) string {
	return m.At.UTC().Format(time.RFC3339Nano) + "\x00" + m.Author + "\x00" + m.Text
}

// resolvedBy says whether messages leave their thread resolved: the last
// resolve or reopen among them decides.
func resolvedBy(ms []Message) bool {
	resolved := false
	for _, m := range ms {
		switch m.Kind {
		case MessageResolve:
			resolved = true
		case MessageReopen:
			resolved = false
		}
	}
	return resolved
}

// Copy gives a second document the threads of the first, as when a
// document is saved as a copy under a new home; the first keeps its own.
func (b *Book) Copy(from, to string) error {
	if from == to {
		return nil
	}
	moving, err := b.List(from)
	if err != nil || len(moving) == 0 {
		return err
	}
	return b.change(to, func(r *record) error {
		for _, t := range moving {
			dup := false
			for _, have := range r.Threads {
				if have.ID == t.ID || t.Key != "" && have.Key == t.Key || t.Native != "" && have.Native == t.Native {
					dup = true
				}
			}
			if !dup {
				r.Threads = append(r.Threads, t)
			}
		}
		return nil
	})
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "T-" + hex.EncodeToString(b)
}
