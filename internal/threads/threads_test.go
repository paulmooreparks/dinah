package threads

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const doc = "# Notes\n\nThe POS sends the sale to the server.\n\nThe server answers.\n"

// A record kept in the state folder moves to the place Place gives once
// that place is there, the state folder's copy is emptied, and the place
// holds no lock file.
func TestRecordMovesToItsPlace(t *testing.T) {
	state, folder := t.TempDir(), t.TempDir()
	b := Open(state)
	key := DocumentKey("local-folder", "w1", "s1", "plan/plan.md")
	start := strings.Index(doc, "sends the sale")
	if _, _, err := b.Add(key, New{Text: doc, Start: start, End: start + 4, Message: Message{By: ByPerson, Text: "Which server?"}}); err != nil {
		t.Fatal(err)
	}
	place := filepath.Join(folder, "comments.json")
	b.Place = func(string) string { return place }
	list, err := b.List(key)
	if err != nil || len(list) != 1 {
		t.Fatalf("a record not yet moved is read from the state folder: %d %v", len(list), err)
	}
	if _, err := b.Reply(key, list[0].ID, Message{By: ByAgent, Text: "The store server."}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(place)
	if err != nil || !strings.Contains(string(data), "Which server?") || !strings.Contains(string(data), "The store server.") {
		t.Fatalf("the record was not moved: %s %v", data, err)
	}
	b.Place = nil
	if left, _ := b.List(key); len(left) != 0 {
		t.Fatalf("the state folder still has the moved record: %d", len(left))
	}
	entries, _ := os.ReadDir(folder)
	if len(entries) != 1 {
		t.Fatalf("the place holds more than the record: %v", entries)
	}
}

func TestOpenReplyResolveReopen(t *testing.T) {
	b := Open(t.TempDir())
	key := DocumentKey("wiki-page", "w1", "s1", "100")
	if key != "wiki-page:w1/s1/100" {
		t.Fatalf("key %q", key)
	}
	start := strings.Index(doc, "sends the sale")
	th, created, err := b.Add(key, New{Key: "Q1", Kind: KindQuestion, Text: doc, Start: start, End: start + len("sends the sale"),
		Message: Message{Author: "Devin", By: ByAgent, Text: "Which server?"}})
	if err != nil || !created || th.Quote != "sends the sale" || !th.Waiting() || th.Due() {
		t.Fatalf("add %+v %v %v", th, created, err)
	}
	again, created, _ := b.Add(key, New{Key: "Q1", Text: doc, Start: 0, End: 1, Message: Message{By: ByAgent, Text: "again"}})
	if created || again.ID != th.ID {
		t.Fatal("a repeated open made a second thread")
	}
	th, _ = b.Reply(key, th.ID, Message{Author: "Paul", By: ByPerson, Text: "The store server."})
	if !th.Due() {
		t.Fatal("an answered question is not due")
	}
	th, _ = b.Resolve(key, th.ID, Message{Author: "Devin", By: ByAgent, Text: "Named the store server."})
	if !th.Resolved || th.Due() || th.Waiting() {
		t.Fatalf("resolve %+v", th)
	}
	th, _ = b.Reply(key, th.ID, Message{Author: "Paul", By: ByPerson, Text: "Not quite."})
	if th.Resolved || th.Messages[len(th.Messages)-1].Kind != MessageReopen {
		t.Fatalf("a person's reply did not reopen: %+v", th)
	}
	if _, err := b.Reply(key, "T-none", Message{By: ByPerson, Text: "x"}); !errors.Is(err, ErrNoThread) {
		t.Fatalf("missing thread: %v", err)
	}
	list, _ := b.List(key)
	if len(list) != 1 || len(list[0].Messages) != 4 {
		t.Fatalf("list %+v", list)
	}
}

// A copy that says a thread and its messages were sent to the site, as a
// colleague's computer records it, makes them sent here too, so they are
// not sent twice; a message the copy has unsent stays unsent.
func TestMergeTakesWhatWasSent(t *testing.T) {
	b := Open(t.TempDir())
	key := DocumentKey("wiki-page", "w1", "s1", "100")
	start := strings.Index(doc, "sends the sale")
	th, _, err := b.Add(key, New{Text: doc, Start: start, End: start + 4, Message: Message{Author: "Paul", By: ByPerson, Text: "Which server?"}})
	if err != nil {
		t.Fatal(err)
	}
	th, _ = b.Reply(key, th.ID, Message{Author: "Paul", By: ByPerson, Text: "The store server?"})
	if !th.Unsent() {
		t.Fatal("nothing to send")
	}
	copy := th
	copy.Messages = append([]Message(nil), th.Messages...)
	copy.Native = "500"
	copy.Messages[0].Native = "500"
	copy.Messages[1].Native = "501"
	_, n, err := b.Merge(key, []Thread{copy})
	if err != nil || n != 2 {
		t.Fatalf("merge %d %v", n, err)
	}
	list, _ := b.List(key)
	if len(list) != 1 || list[0].Native != "500" || list[0].Messages[1].Native != "501" || list[0].Unsent() || len(list[0].Messages) != 2 {
		t.Fatalf("after merge %+v", list)
	}
	copy.Messages[1].Native = ""
	if _, n, _ := b.Merge(key, []Thread{copy}); n != 0 {
		t.Fatalf("an unsent copy changed a sent message: %d", n)
	}
}

func TestPlaceFollowsTheWords(t *testing.T) {
	start := strings.Index(doc, "server answers")
	th := Thread{Quote: "server answers", Prefix: "\n\nThe ", Suffix: ".\n", Start: start}
	moved := "# Notes\n\nA new paragraph first.\n\nThe POS sends the sale to the server.\n\nThe server answers.\n"
	p := Place([]Thread{th}, moved)[0]
	if p.Lost || moved[p.At:p.End] != "server answers" {
		t.Fatalf("not followed: %+v", p)
	}
	gone := "# Notes\n\nThe POS sends the sale.\n"
	p = Place([]Thread{th}, gone)[0]
	if !p.Lost || p.At > len(gone) {
		t.Fatalf("lost words: %+v", p)
	}
}

func TestMoveToANewHome(t *testing.T) {
	b := Open(t.TempDir())
	from, to := DraftKey("d1"), DocumentKey("local-folder", "w1", "s1", "notes.md")
	if _, _, err := b.Add(from, New{Key: "k", Text: doc, Start: 0, End: 7, Message: Message{By: ByPerson, Text: "Title?"}}); err != nil {
		t.Fatal(err)
	}
	if err := b.Move(from, to); err != nil {
		t.Fatal(err)
	}
	if err := b.Move(from, to); err != nil {
		t.Fatal(err)
	}
	moved, _ := b.List(to)
	left, _ := b.List(from)
	if len(moved) != 1 || len(left) != 0 {
		t.Fatalf("moved %d, left %d", len(moved), len(left))
	}
}
