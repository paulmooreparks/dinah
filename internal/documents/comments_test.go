package documents

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"studio/internal/layout"
	"studio/internal/resource"
	"studio/internal/threads"
	"studio/internal/workspaces"
)

// sitePages is a provider of a fake site: pages read as snapshots, written
// back whole, and commented on, with every comment and reply kept so that
// a test can see what was sent.
type sitePages struct {
	text     map[string]string
	comments []resource.Comment
	writes   []string
	resolves []string
	gone     map[string]bool
}

func (p *sitePages) Snapshot(_ context.Context, path string) (resource.Snapshot, error) {
	t, ok := p.text[path]
	if !ok {
		return resource.Snapshot{}, resource.ErrNotFound
	}
	return resource.Snapshot{Text: t, Revision: "1", Format: "markdown", Title: "Page " + path}, nil
}
func (p *sitePages) WriteDocument(_ context.Context, path string, base resource.Snapshot, edit resource.Edit) (resource.Written, error) {
	p.text[path] = edit.Text
	return resource.Written{Snapshot: resource.Snapshot{Text: edit.Text, Revision: "2", Format: "markdown", Title: edit.Title}}, nil
}
func (p *sitePages) Publishes() bool { return true }
func (p *sitePages) ReadComments(context.Context, string) ([]resource.Comment, error) {
	return p.comments, nil
}
func (p *sitePages) AddComment(_ context.Context, path string, c resource.NewComment) (resource.Made, error) {
	id := "c" + string(rune('1'+len(p.writes)))
	p.writes = append(p.writes, "add "+c.Quote+"#"+string(rune('0'+c.Occurrence))+": "+c.Text)
	p.comments = append(p.comments, resource.Comment{ID: id, Author: "me", Text: c.Text, Inline: true, Selection: c.Quote})
	return resource.Made{ID: id}, nil
}
func (p *sitePages) ReplyComment(_ context.Context, path, parent, text string, fromAgent bool) (string, error) {
	if p.gone[parent] {
		return "", resource.ErrNotFound
	}
	id := "r" + string(rune('1'+len(p.writes)))
	p.writes = append(p.writes, "reply "+parent+": "+text)
	return id, nil
}
func (p *sitePages) ResolveComment(_ context.Context, path, id string, resolved bool) error {
	p.resolves = append(p.resolves, id+"="+map[bool]string{true: "resolved", false: "open"}[resolved])
	return nil
}

func siteOps(t *testing.T, site *sitePages) Ops {
	t.Helper()
	state := t.TempDir()
	workspaces.RegisterSourceKind("test-site-"+strings.ToLower(t.Name()[4:10]), workspaces.SourceKind{
		Provider:    func(workspaces.Resolver, context.Context, workspaces.Source) (any, error) { return site, nil },
		DocumentKey: func(s workspaces.Source, path string) string { return "site:" + path },
	})
	kind := "test-site-" + strings.ToLower(t.Name()[4:10])
	record := workspaces.Record{ID: "w", Name: "Customer", Sources: []workspaces.Source{{ID: "pages", Kind: kind, Name: "Pages"}}}
	resolver := workspaces.Resolver{Layout: layout.Layout{State: state}, Records: func() []workspaces.Record { return []workspaces.Record{record} }}
	service, err := Service(resolver.Layout, resolver)
	if err != nil {
		t.Fatal(err)
	}
	return Ops{Ctx: context.Background(), Drafts: service, Resolver: resolver}
}

// A colleague's comments on the document's site come into the threads as a
// person's, an inline one on the words it was made on and one on the whole
// document on its first line, with their replies; bringing them in again
// adds only what is new, and a new reply reopens a resolved thread.
func TestBringInComments(t *testing.T) {
	when := func(h int) time.Time { return time.Date(2026, 10, 16, h, 0, 0, 0, time.UTC) }
	site := &sitePages{text: map[string]string{"123": "---\ntitle: Plan\n---\n# Plan\n\nThe receipt prints twice on Mondays.\n"}, comments: []resource.Comment{
		{ID: "11", Text: "Which lane?", Inline: true, Selection: "prints twice", Author: "jvychopen", Created: when(9)},
		{ID: "12", Text: "Looks good overall.", Author: "lmueller", Created: when(10)},
		{ID: "13", ParentID: "11", Text: "Lane 4.", Author: "pparks", Created: when(11)},
	}}
	o := siteOps(t, site)
	v, err := o.Open(workspaces.Ref{WorkspaceID: "w", SourceID: "pages", Path: "123"})
	if err != nil {
		t.Fatal(err)
	}
	n, err := o.BringInComments(v.ID)
	if err != nil || n != 3 {
		t.Fatalf("brought in %d %v", n, err)
	}
	list, _ := o.Threads(v.ID)
	if len(list) != 2 || list[0].Quote != "prints twice" || len(list[0].Messages) != 2 || list[0].Messages[1].Text != "Lane 4." ||
		list[0].Messages[0].By != "person" || list[0].Messages[0].Author != "jvychopen" || list[1].Quote != "# Plan" || list[1].Messages[0].Text != "Looks good overall." || list[0].Unsent {
		t.Fatalf("threads %+v", list)
	}
	if !list[0].Messages[0].Brought || !list[0].Messages[1].Brought || !list[1].Messages[0].Brought {
		t.Fatalf("comments from the site are not marked as brought in: %+v", list)
	}
	if n, _ := o.BringInComments(v.ID); n != 0 {
		t.Fatalf("brought the same comments in again: %d", n)
	}
	// Resolved on the site after it was opened here: followed.
	site.comments[0].Resolved, site.comments[0].ResolvedBy, site.comments[0].ResolvedAt = true, "jvychopen", when(12)
	if _, err := o.BringInComments(v.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = o.Threads(v.ID)
	if !list[0].Resolved || !strings.Contains(list[0].Messages[len(list[0].Messages)-1].Text, "Resolved on the site") {
		t.Fatalf("resolution not followed: %+v", list[0])
	}
}

// A thread opened here goes to the site as a comment on its words when the
// document is saved, its replies and its resolution follow, and nothing is
// sent twice; a reply to a comment the site no longer has is kept here.
func TestSendCommentsWithSave(t *testing.T) {
	site := &sitePages{text: map[string]string{"7": "# Plan\n\nThe receipt prints twice. It prints twice on Mondays.\n"}, gone: map[string]bool{}}
	o := siteOps(t, site)
	v, err := o.Open(workspaces.Ref{WorkspaceID: "w", SourceID: "pages", Path: "7"})
	if err != nil {
		t.Fatal(err)
	}
	me := threads.Message{Author: "Paul", By: threads.ByPerson, Text: "Twice? Which lane?"}
	from, to := strings.Index(v.Text, "It prints twice"), strings.Index(v.Text, "It prints twice")+len("It prints twice")
	// The second "prints twice" in the text is the passage.
	start, end := strings.Index(v.Text, "prints twice on"), strings.Index(v.Text, "prints twice on")+len("prints twice")
	_ = from
	_ = to
	th, err := o.OpenThread(v.ID, ThreadOpen{From: &start, To: &end, Kind: threads.KindComment, Message: me})
	if err != nil || !th.Unsent {
		t.Fatalf("open: %+v %v", th, err)
	}
	if !o.UnsentComments(v.ID) {
		t.Fatal("the thread is not counted as unsent")
	}
	saved, err := o.Save(v.ID, v.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if len(site.writes) != 1 || site.writes[0] != "add prints twice#1: Twice? Which lane?" {
		t.Fatalf("sent: %v (notes %v)", site.writes, saved.Notes)
	}
	list, _ := o.Threads(v.ID)
	if list[0].Native != "c1" || list[0].Unsent || list[0].Messages[0].Native != "c1" {
		t.Fatalf("not recorded as sent: %+v", list[0])
	}
	// A reply and a resolution, sent with the next save; the save itself
	// changes nothing.
	if _, err := o.ReplyThread(v.ID, th.ID, threads.Message{Author: "Devin", By: threads.ByAgent, Text: "Lane 4 prints the copy."}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.ResolveThread(v.ID, th.ID, threads.Message{Author: "Paul", By: threads.ByPerson}); err != nil {
		t.Fatal(err)
	}
	latest, _ := o.Get(v.ID)
	if _, err := o.Save(v.ID, latest.Revision); err != nil {
		t.Fatal(err)
	}
	if len(site.writes) != 2 || site.writes[1] != "reply c1: Lane 4 prints the copy." || len(site.resolves) != 1 || site.resolves[0] != "c1=resolved" {
		t.Fatalf("follow-ups: %v %v", site.writes, site.resolves)
	}
	if o.UnsentComments(v.ID) {
		t.Fatal("something is still unsent")
	}
	// The comment is deleted on the site; a further reply stays here.
	site.gone["c1"] = true
	if _, err := o.ReplyThread(v.ID, th.ID, threads.Message{Author: "Paul", By: threads.ByPerson, Text: "Reopening."}); err != nil {
		t.Fatal(err)
	}
	latest, _ = o.Get(v.ID)
	saved, err = o.Save(v.ID, latest.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Notes) == 0 || !strings.Contains(saved.Notes[0], "no longer on the site") || o.UnsentComments(v.ID) {
		t.Fatalf("a gone comment: notes %v unsent %v", saved.Notes, o.UnsentComments(v.ID))
	}
}

// A document whose source keeps no comments has threads here alone, never
// marked unsent, and bringing comments in does nothing.
func TestThreadsWithoutASite(t *testing.T) {
	o, main := renameOps(t)
	write(t, main+"/notes.md", "# Notes\n\nA line.\n")
	v, err := o.Open(workspaces.Ref{WorkspaceID: "w", SourceID: "main", Path: "notes.md"})
	if err != nil {
		t.Fatal(err)
	}
	th, err := o.OpenThread(v.ID, ThreadOpen{Quote: "A line", Message: threads.Message{Author: "Paul", By: threads.ByPerson, Text: "Say more."}})
	if err != nil || th.Unsent || th.From != 9 || th.To != 15 {
		t.Fatalf("open: %+v %v", th, err)
	}
	if n, err := o.BringInComments(v.ID); err != nil || n != 0 {
		t.Fatalf("bring in: %d %v", n, err)
	}
	if _, err := o.OpenThread(v.ID, ThreadOpen{Quote: "Nowhere", Message: threads.Message{Text: "x"}}); err == nil || !errors.Is(err, err) {
		t.Fatal("words the text lacks were accepted")
	}
}
