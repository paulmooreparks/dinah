package documents

import (
	"context"
	"sort"
	"strings"
	"time"

	"studio/internal/drafts"
	"studio/internal/resource"
	"studio/internal/threads"
)

// A colleague comments on a document where it lives, on its site, and those
// comments are brought into the document's threads record, so they are
// answered and resolved where the person's and the agents' are; and the
// threads made here go to the site as its own comments when the document
// is saved or published, with their replies, resolutions and reopenings
// (docs/design/document-comments.md). The source's provider does both
// through resource.CommentReader and resource.CommentWriter; a document
// whose source has neither keeps its threads here alone. Each comment and
// reply keeps the site's ID, so bringing the comments in again adds only
// new ones and nothing is sent twice.

// commentReaderFor is the reader of the comments on a draft's document, or
// nil when its source keeps none.
func (o Ops) commentReaderFor(d drafts.Draft) resource.CommentReader {
	if d.Origin.Kind == "" || d.Origin.Kind == drafts.OriginNew || d.Origin.Path == "" {
		return nil
	}
	p, err := o.Drafts.Providers.Provider(o.ctx(), d.Origin)
	if err != nil {
		return nil
	}
	r, _ := p.(resource.CommentReader)
	return r
}

// commentWriterFor is the writer of comments on a draft's document, or nil
// when its source takes none.
func (o Ops) commentWriterFor(d drafts.Draft) resource.CommentWriter {
	if d.Origin.Kind == "" || d.Origin.Kind == drafts.OriginNew || d.Origin.Path == "" {
		return nil
	}
	p, err := o.Drafts.Providers.Provider(o.ctx(), d.Origin)
	if err != nil {
		return nil
	}
	w, _ := p.(resource.CommentWriter)
	return w
}

func (o Ops) ctx() context.Context {
	if o.Ctx == nil {
		return context.Background()
	}
	return o.Ctx
}

// BringInComments brings the comments on a draft's document, from its
// source, into its threads record and returns how many comments and replies
// were new. A draft whose source keeps no comments has none.
func (o Ops) BringInComments(id string) (int, error) {
	d, err := o.Draft(id)
	if err != nil {
		return 0, err
	}
	src := o.commentReaderFor(d)
	if src == nil {
		return 0, nil
	}
	all, err := src.ReadComments(o.ctx(), d.Origin.Path)
	if err != nil {
		return 0, err
	}
	book, key := o.threadBook(), o.threadKey(d)
	kept, err := book.List(key)
	if err != nil {
		return 0, err
	}
	known := map[string]bool{}
	for _, t := range kept {
		for _, m := range t.Messages {
			if m.Native != "" {
				known[m.Native] = true
			}
		}
		if t.Native != "" {
			known[t.Native] = true
		}
	}
	byParent := map[string][]resource.Comment{}
	tops := []resource.Comment{}
	for _, c := range all {
		if c.ParentID == "" {
			tops = append(tops, c)
		} else {
			byParent[c.ParentID] = append(byParent[c.ParentID], c)
		}
	}
	sortComments(tops)
	added, changed := 0, false
	for _, top := range tops {
		replies := allReplies(byParent, top.ID)
		sortComments(replies)
		start, end := commentAnchor(d.Text, top.Selection)
		first := commentMessage(top)
		t, created, err := book.Add(key, threads.New{Native: top.ID, Text: d.Text, Start: start, End: end, Message: first})
		if err != nil {
			if strings.Contains(err.Error(), "needs words") {
				continue
			}
			return added, err
		}
		if created {
			added++
		}
		for _, r := range replies {
			if known[r.ID] {
				continue
			}
			m := commentMessage(r)
			if strings.TrimSpace(m.Text) == "" {
				continue
			}
			if t, err = book.Reply(key, t.ID, m); err != nil {
				return added, err
			}
			added++
		}
		moved, err := followResolution(book, key, t, top)
		if err != nil {
			return added, err
		}
		changed = changed || moved
	}
	if added > 0 || changed {
		o.emit("threads:changed", ThreadsChange{ID: id})
	}
	return added, nil
}

// followResolution resolves or reopens a thread as its comment is on the
// site, when that happened after the thread was last resolved or reopened
// here, so a change made on either side is kept until the other side
// changes it again. It reports whether the thread changed.
func followResolution(book *threads.Book, key string, t threads.Thread, c resource.Comment) (bool, error) {
	if c.ResolvedAt.IsZero() || c.Resolved == t.Resolved {
		return false, nil
	}
	for _, m := range t.Messages {
		if (m.Kind == threads.MessageResolve || m.Kind == threads.MessageReopen) && !c.ResolvedAt.After(m.At) {
			return false, nil
		}
	}
	m := threads.Message{Author: c.ResolvedBy, By: threads.ByPerson, At: c.ResolvedAt, Native: c.ID + "@" + c.ResolvedAt.UTC().Format(time.RFC3339)}
	if m.Author == "" {
		m.Author = "The site"
	}
	var err error
	switch {
	case c.Dangling:
		m.Text = "Resolved on the site, because a change to the document removed the words this comment was on."
		_, err = book.Resolve(key, t.ID, m)
	case c.Resolved:
		m.Text = "Resolved on the site."
		_, err = book.Resolve(key, t.ID, m)
	default:
		m.Text = "Reopened on the site."
		_, err = book.Reopen(key, t.ID, m)
	}
	return err == nil, err
}

// allReplies are a comment's replies and theirs, by their parents.
func allReplies(byParent map[string][]resource.Comment, id string) []resource.Comment {
	out := []resource.Comment{}
	for _, r := range byParent[id] {
		out = append(out, r)
		out = append(out, allReplies(byParent, r.ID)...)
	}
	return out
}

func sortComments(cs []resource.Comment) {
	sort.SliceStable(cs, func(i, j int) bool {
		if !cs[i].Created.Equal(cs[j].Created) {
			return cs[i].Created.Before(cs[j].Created)
		}
		return cs[i].ID < cs[j].ID
	})
}

// commentMessage is a site's comment as a person's message.
func commentMessage(c resource.Comment) threads.Message {
	m := threads.Message{Author: c.Author, By: threads.ByPerson, Text: strings.TrimSpace(c.Text), Native: c.ID, At: c.Created}
	if m.Author == "" {
		m.Author = "Someone"
	}
	return m
}

// commentAnchor is where a site's comment is shown: on the words it was made
// on, when the site gave them and the text still has them, and otherwise on
// the document's first line of text, as a comment on the whole document.
func commentAnchor(text, selection string) (int, int) {
	if s := strings.TrimSpace(selection); s != "" {
		if i := strings.Index(text, s); i >= 0 {
			return i, i + len(s)
		}
	}
	at := 0
	lines := strings.SplitAfter(text, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			at += len(lines[i-1])
			if strings.TrimSpace(lines[i]) == "---" {
				at += len(lines[i])
				break
			}
		}
	}
	for _, line := range strings.SplitAfter(text[at:], "\n") {
		if t := strings.TrimSpace(line); t != "" {
			i := at + strings.Index(line, t)
			return i, i + len(t)
		}
		at += len(line)
	}
	return 0, 0
}

// UnsentComments reports whether a draft's document has threads with
// something its site has not been sent.
func (o Ops) UnsentComments(id string) bool {
	d, err := o.Draft(id)
	if err != nil {
		return false
	}
	return o.unsentComments(d)
}

func (o Ops) unsentComments(d drafts.Draft) bool {
	if o.commentWriterFor(d) == nil {
		return false
	}
	list, err := o.threadBook().List(o.threadKey(d))
	if err != nil {
		return false
	}
	for _, t := range list {
		if t.Unsent() {
			return true
		}
	}
	return false
}

// sendComments sends a draft's unsent threads to its document's site, and
// returns notes on what could not be sent; what was not sent is sent at the
// next save or publish.
func (o Ops) sendComments(d drafts.Draft) []string {
	w := o.commentWriterFor(d)
	if w == nil || !o.unsentComments(d) {
		return nil
	}
	ctx := o.ctx()
	book, key := o.threadBook(), o.threadKey(d)
	list, err := book.List(key)
	if err != nil {
		return []string{"The comments could not be read, so none were sent: " + err.Error()}
	}
	var notes []string
	changed := false
	for _, p := range threads.Place(list, d.Text) {
		t := p.Thread
		if !t.Unsent() {
			continue
		}
		var sent []threads.Message
		native := t.Native
		gone := false
		if native == "" {
			first := t.Messages[0]
			occurrence := strings.Count(d.Text[:min(p.At, len(d.Text))], t.Quote)
			made, err := w.AddComment(ctx, d.Origin.Path, resource.NewComment{Quote: t.Quote, Occurrence: occurrence, Text: first.Text, FromAgent: first.By == threads.ByAgent,
				Thread: map[string]string{"thread": t.ID, "key": t.Key, "kind": t.Kind, "severity": t.Severity}})
			if err != nil {
				notes = append(notes, "The comment on \""+clip(t.Quote)+"\" could not be sent: "+ErrorText(err).Error())
				continue
			}
			native = made.ID
			if made.Note != "" {
				notes = append(notes, made.Note)
			}
			first.Native = native
			sent = append(sent, first)
			t.Messages[0].Native = native
		}
		for _, m := range t.Messages {
			if m.Native != "" {
				continue
			}
			if gone {
				m.Native = native + "@" + m.At.UTC().Format("20060102T150405.000000000Z")
				sent = append(sent, m)
				continue
			}
			if strings.TrimSpace(m.Text) != "" {
				id, err := w.ReplyComment(ctx, d.Origin.Path, native, m.Text, m.By == threads.ByAgent)
				if err != nil {
					// A comment that is no longer on the site, because
					// someone deleted it, takes no more replies. What was
					// added here is kept here, marked as dealt with so that
					// no later save tries it again.
					if isNotFound(err) {
						gone = true
						notes = append(notes, "The comment on \""+clip(t.Quote)+"\" is no longer on the site, so what was added to it here is kept here alone.")
						m.Native = native + "@" + m.At.UTC().Format("20060102T150405.000000000Z")
						sent = append(sent, m)
						continue
					}
					notes = append(notes, "A reply on \""+clip(t.Quote)+"\" could not be sent: "+ErrorText(err).Error())
					break
				}
				m.Native = id
			}
			if m.Kind == threads.MessageResolve || m.Kind == threads.MessageReopen {
				if err := w.ResolveComment(ctx, d.Origin.Path, native, m.Kind == threads.MessageResolve); err != nil {
					notes = append(notes, "Resolving or reopening \""+clip(t.Quote)+"\" could not be sent: "+ErrorText(err).Error())
					break
				}
			}
			if m.Native == "" {
				// A resolve or reopen without words is sent as a change of
				// the comment, which has no ID of its own.
				m.Native = native + "@" + m.At.UTC().Format("20060102T150405.000000000Z")
			}
			sent = append(sent, m)
		}
		if len(sent) > 0 {
			if _, err := book.Sent(key, t.ID, native, sent); err != nil {
				notes = append(notes, "What was sent could not be recorded: "+err.Error())
				continue
			}
			changed = true
		}
	}
	if changed {
		o.emit("threads:changed", ThreadsChange{ID: d.ID})
	}
	return notes
}

func isNotFound(err error) bool {
	for e := err; e != nil; {
		if e == resource.ErrNotFound {
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

// clip is the start of a quote, for a note.
func clip(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 60 {
		return string(r[:60]) + "…"
	}
	return s
}
