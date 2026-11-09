package documents

import (
	"errors"
	"strings"

	"studio/internal/drafts"
	"studio/internal/threads"
	"studio/internal/workspaces"
)

// A document's comment threads are kept apart from its text, in a record of
// the document's own (package threads), so that no thread is ever saved to
// the document's home where its readers would see it
// (docs/design/document-comments.md). The person writes here through the
// window's comment margin; agents write through the Documents applet.

// ThreadsChange is emitted as "threads:changed" after a draft's document's
// threads change, so that its views draw them again.
type ThreadsChange struct {
	ID string `json:"id"`
}

// threadBook is the threads records, kept in the state folder.
func (o Ops) threadBook() *threads.Book {
	return threads.Open(o.Resolver.Layout.State)
}

// threadKey is the identity a draft's threads are kept under: its document's
// when it has a home, its own while it is only a draft.
func (o Ops) threadKey(d drafts.Draft) string {
	if d.Origin.Kind == "" || d.Origin.Kind == drafts.OriginNew {
		return threads.DraftKey(d.ID)
	}
	if source, err := o.Resolver.Source(workspaces.Ref{WorkspaceID: d.Origin.WorkspaceID, SourceID: d.Origin.SourceID}); err == nil {
		return workspaces.DocumentKey(source, d.Origin.Path)
	}
	return threads.DocumentKey(d.Origin.Kind, d.Origin.WorkspaceID, d.Origin.SourceID, d.Origin.Path)
}

// PlacedThread is a thread as a view draws it: where its words are in the
// draft's text, in UTF-16 code units as a JavaScript editor counts them.
type PlacedThread struct {
	threads.Placed
	From    int  `json:"from"`
	To      int  `json:"to"`
	Due     bool `json:"due"`
	Waiting bool `json:"waiting"`
	// Unsent says the thread has something the document's site has not
	// been sent yet, which the next save or publish sends.
	Unsent bool `json:"unsent"`
}

// Threads returns a draft's document's threads, placed in the draft's text.
func (o Ops) Threads(id string) ([]PlacedThread, error) {
	d, err := o.Draft(id)
	if err != nil {
		return nil, err
	}
	list, err := o.threadBook().List(o.threadKey(d))
	if err != nil {
		return nil, err
	}
	sends := o.commentWriterFor(d) != nil
	out := []PlacedThread{}
	for _, p := range threads.Place(list, d.Text) {
		out = append(out, o.placedOf(d, p, sends))
	}
	return out, nil
}

func (o Ops) placedOf(d drafts.Draft, p threads.Placed, sends bool) PlacedThread {
	return PlacedThread{Placed: p, From: utf16Len(d.Text[:p.At]), To: utf16Len(d.Text[:p.End]), Due: p.Due(), Waiting: p.Waiting(), Unsent: sends && p.Unsent()}
}

// ThreadOpen is a new thread on a draft's document. Its passage is given as
// words of the text, Quote, the Nth time they occur (1 is the first), as an
// agent gives it, or as a UTF-16 span [From, To) of the draft's text, as the
// window gives it.
type ThreadOpen struct {
	Quote    string          `json:"quote,omitempty"`
	Nth      int             `json:"nth,omitempty"`
	From     *int            `json:"from,omitempty"`
	To       *int            `json:"to,omitempty"`
	Key      string          `json:"key,omitempty"`
	Kind     string          `json:"kind,omitempty"`
	Severity string          `json:"severity,omitempty"`
	Message  threads.Message `json:"message"`
}

// OpenThread opens a thread on a draft's document. A thread whose key is
// already kept is returned unchanged.
func (o Ops) OpenThread(id string, in ThreadOpen) (PlacedThread, error) {
	d, err := o.Draft(id)
	if err != nil {
		return PlacedThread{}, err
	}
	start, end, err := passage(d.Text, in)
	if err != nil {
		return PlacedThread{}, err
	}
	t, _, err := o.threadBook().Add(o.threadKey(d), threads.New{Key: in.Key, Kind: in.Kind, Severity: in.Severity, Text: d.Text, Start: start, End: end, Message: in.Message})
	if err != nil {
		return PlacedThread{}, err
	}
	o.emit("threads:changed", ThreadsChange{ID: id})
	return o.placed(d, t), nil
}

// ReplyThread adds a reply to a thread of a draft's document.
func (o Ops) ReplyThread(id, thread string, m threads.Message) (PlacedThread, error) {
	return o.changeThread(id, func(b *threads.Book, key string) (threads.Thread, error) { return b.Reply(key, thread, m) })
}

// ResolveThread closes a thread of a draft's document.
func (o Ops) ResolveThread(id, thread string, m threads.Message) (PlacedThread, error) {
	return o.changeThread(id, func(b *threads.Book, key string) (threads.Thread, error) { return b.Resolve(key, thread, m) })
}

func (o Ops) changeThread(id string, fn func(b *threads.Book, key string) (threads.Thread, error)) (PlacedThread, error) {
	d, err := o.Draft(id)
	if err != nil {
		return PlacedThread{}, err
	}
	t, err := fn(o.threadBook(), o.threadKey(d))
	if err != nil {
		return PlacedThread{}, err
	}
	o.emit("threads:changed", ThreadsChange{ID: id})
	return o.placed(d, t), nil
}

func (o Ops) placed(d drafts.Draft, t threads.Thread) PlacedThread {
	p := threads.Place([]threads.Thread{t}, d.Text)[0]
	return o.placedOf(d, p, o.commentWriterFor(d) != nil)
}

// passage finds a thread's span in text, in bytes.
func passage(text string, in ThreadOpen) (int, int, error) {
	if in.From != nil && in.To != nil {
		start, end := byteOffset(text, *in.From), byteOffset(text, *in.To)
		if start < 0 || end < start {
			return 0, 0, errors.New("the passage is not inside the document")
		}
		return start, end, nil
	}
	if in.Quote == "" {
		return 0, 0, errors.New("say which words of the document the thread is about")
	}
	nth := max(in.Nth, 1)
	from := 0
	for n := 1; ; n++ {
		i := strings.Index(text[from:], in.Quote)
		if i < 0 {
			return 0, 0, errors.New("the document does not contain those words" + nthText(nth) + "; quote them exactly as the document has them")
		}
		if n == nth {
			return from + i, from + i + len(in.Quote), nil
		}
		from += i + len(in.Quote)
	}
}

func nthText(n int) string {
	if n <= 1 {
		return ""
	}
	return " that many times"
}

// byteOffset turns a UTF-16 offset into a byte offset of text, or -1 when it
// is past the end.
func byteOffset(text string, units int) int {
	n := 0
	for i, r := range text {
		if n >= units {
			return i
		}
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	if n >= units {
		return len(text)
	}
	return -1
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// moveThreads keeps a draft's threads with it when it is saved under a new
// home: a draft saved for the first time gives its threads to the new
// document, and a document saved as a copy gives the copy its threads too,
// keeping its own.
func (o Ops) moveThreads(before, after drafts.Draft) {
	from, to := o.threadKey(before), o.threadKey(after)
	if from == to {
		return
	}
	b := o.threadBook()
	if before.Origin.Kind == "" || before.Origin.Kind == drafts.OriginNew {
		_ = b.Move(from, to)
		return
	}
	_ = b.Copy(from, to)
}
