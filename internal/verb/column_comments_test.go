package verb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestAColumnTakesACommentBySlugByTitleAndByIdentifier asserts the accepting
// half of dinah-518/criteria/1. The three spellings ColumnByRef resolves each
// reach the same column, and the three comments read back in creation order
// at that column's own reference.
//
// All three spellings run against one workbench, because a build resolving
// the identifier alone passes a test that types only the identifier.
func TestAColumnTakesACommentBySlugByTitleAndByIdentifier(t *testing.T) {
	h := newHarness(t)
	column := h.library.Bench.Columns[0]
	spellings := []string{column.Slug, column.Title, column.ID}
	for i, spelling := range spellings {
		if spelling == "" {
			t.Fatalf("spelling %d is empty, so this run would prove nothing", i+1)
		}
	}
	texts := []string{"the first note", "the second note", "the third note"}
	for i, spelling := range spellings {
		response := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: spelling, Text: texts[i]})
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("comment on %q: %s %s", spelling, response.Outcome, response.Refusal)
		}
		h.reopen()
	}

	comments, err := bench.Comments(h.library.Bench.ColumnDir(column.ID))
	if err != nil {
		t.Fatalf("read the column's comments: %v", err)
	}
	if len(comments) != len(texts) {
		t.Fatalf("the column carries %d comments, wanted %d", len(comments), len(texts))
	}
	for i, comment := range comments {
		if comment.Body != texts[i] {
			t.Errorf("comment %d reads %q, wanted %q", i+1, comment.Body, texts[i])
		}
		if comment.Ordinal != i+1 {
			t.Errorf("comment %d carries ordinal %d, wanted %d", i+1, comment.Ordinal, i+1)
		}
	}

	// Each printed position resolves back to the comment that stands there,
	// which is what makes the position a spelling a reader can type.
	for i, comment := range comments {
		ref := fmt.Sprintf("%s/%s/%d", column.Ref(), bench.CommentsDir, i+1)
		entity, err := h.library.Bench.ResolveEntity(ref)
		if err != nil {
			t.Fatalf("resolve %q: %v", ref, err)
		}
		if entity.Kind != bench.KindComment {
			t.Errorf("%q resolves to a %s, wanted a %s", ref, entity.Kind, bench.KindComment)
		}
		if entity.ID != comment.ID {
			t.Errorf("%q resolves to %s, wanted %s", ref, entity.ID, comment.ID)
		}
	}
}

// TestEverySpellingReachingTheWorkbenchIsRefusedACommentByName asserts the
// refusing half of dinah-518/criteria/1 and the whole of
// dinah-518/criteria/2. The three accepting kinds run beside the five
// refusing references against one workbench, so a build that refuses
// everything fails on the accepting half and a build that accepts everything
// fails on the refusing half.
//
// Each refusal's own kind is read back, so a build refusing the literal
// strings "workbench" and "." ahead of resolution fails: it would carry no
// kind where the resolver's own answer is the workbench.
func TestEverySpellingReachingTheWorkbenchIsRefusedACommentByName(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card")
	item := h.file(card, "open_question", "a question")
	h.comment(card, "a remark")
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("bytes"), 0o644); err != nil {
		t.Fatalf("write the attachment source: %v", err)
	}
	if response := h.library.Attach(&Request{Verb: "attach", Actor: "alka", Ref: card, File: source}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("attach: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if response := h.library.NewWorkstream(&Request{Verb: "workstream", Actor: "alka", Workstream: "Addressing"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("workstream: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	streams, err := h.library.Bench.Workstreams()
	if err != nil {
		t.Fatalf("read the workstreams: %v", err)
	}
	if len(streams) == 0 {
		t.Fatal("the workbench carries no workstream, so the workstream half would assert nothing")
	}
	stream := streams[0]

	accepts := []struct {
		name string
		ref  string
	}{
		{"a card", card},
		{"a checklist item", item},
		{"a column", h.library.Bench.Columns[0].Slug},
	}
	for _, c := range accepts {
		t.Run(c.name, func(t *testing.T) {
			response := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: c.ref, Text: "text"})
			if response.Outcome != contract.OutcomeOK {
				t.Fatalf("comment on %s: wanted ok, got %s %s", c.ref, response.Outcome, response.Refusal)
			}
			h.reopen()
		})
	}

	refuses := []struct {
		name string
		ref  string
		kind string
	}{
		{"an attachment", card + "/attachments/1", bench.KindAttachment},
		{"a comment", card + "/comments/1", bench.KindComment},
		{"a workstream", "workstream/" + stream.Slug, bench.KindWorkstream},
		{"the workbench by name", "workbench", bench.KindWorkbench},
		{"the workbench by dot", ".", bench.KindWorkbench},
	}
	for _, c := range refuses {
		t.Run(c.name, func(t *testing.T) {
			response := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: c.ref, Text: "text"})
			if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.NotCommentable {
				t.Fatalf("comment on %s: wanted %s, got %s %s", c.ref, contract.NotCommentable, response.Outcome, response.Refusal)
			}
			if response.Context["kind"] != c.kind {
				t.Errorf("the refusal carries kind %q, wanted %q", response.Context["kind"], c.kind)
			}
		})
	}

	// The refusal token is asserted verbatim: dinah.not-commentable carries
	// its layer prefix and unknown-card does not, which is a difference an
	// earlier round of this card's contract got wrong.
	if contract.NotCommentable != "dinah.not-commentable" {
		t.Errorf("contract.NotCommentable is %q, and this test asserts the prefixed spelling", contract.NotCommentable)
	}
	if contract.UnknownCard != "unknown-card" {
		t.Errorf("contract.UnknownCard is %q, and this test asserts the bare spelling", contract.UnknownCard)
	}

	// The workbench's own slug is not a spelling the resolver answers the
	// workbench for, and a blank reference never reaches the mount check.
	bare := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: h.library.Bench.Slug, Text: "text"})
	if bare.Outcome != contract.OutcomeRefused || bare.Refusal != contract.UnknownCard {
		t.Errorf("comment on the workbench slug: wanted %s, got %s %s", contract.UnknownCard, bare.Outcome, bare.Refusal)
	}
	empty := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: "", Text: "text"})
	if empty.Outcome != contract.OutcomeRefused || empty.Refusal != contract.UnknownCard {
		t.Errorf("comment on an empty reference: wanted %s, got %s %s", contract.UnknownCard, empty.Outcome, empty.Refusal)
	}
}

// TestAColumnCommentLandsInTheWorkbenchJournalWithItsColumn asserts
// dinah-518/criteria/3. A column comment appends one commented line to the
// workbench journal carrying the column and its title and no item, and a card
// comment and an item comment are pinned beside it in the same run: each
// lands in the card's own journal, neither reaches the workbench journal, and
// neither carries a column.
//
// Both halves are needed. The column half alone passes against a build that
// sends every comment to the workbench journal, and the card half alone
// passes against a build that writes a column onto every commented line.
func TestAColumnCommentLandsInTheWorkbenchJournalWithItsColumn(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card")
	item := h.file(card, "open_question", "a question")
	column := h.library.Bench.Columns[1]

	before := len(h.benchEvents())
	h.comment(card, "a card remark")
	h.comment(item, "an item remark")
	if response := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: column.Slug, Text: "a column remark"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment on the column: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()

	var commented []bench.Event
	for _, ev := range h.benchEvents()[before:] {
		if ev.Event == contract.EventCommented {
			commented = append(commented, ev)
		}
	}
	if len(commented) != 1 {
		t.Fatalf("the workbench journal gained %d commented lines, wanted 1", len(commented))
	}
	line := commented[0]
	if line.Column != column.ID {
		t.Errorf("the line carries column %q, wanted %q", line.Column, column.ID)
	}
	if line.ColumnTitle != column.Title {
		t.Errorf("the line carries column_title %q, wanted %q", line.ColumnTitle, column.Title)
	}
	if line.Item != "" {
		t.Errorf("the line carries item %q, wanted none", line.Item)
	}
	if line.Comment == "" {
		t.Error("the line carries no comment identifier")
	}

	var cardLines []bench.Event
	for _, ev := range h.events(card) {
		if ev.Event == contract.EventCommented {
			cardLines = append(cardLines, ev)
		}
	}
	if len(cardLines) != 2 {
		t.Fatalf("the card's journal carries %d commented lines, wanted 2", len(cardLines))
	}
	if cardLines[0].Item != "" {
		t.Errorf("the card comment's line carries item %q, wanted none", cardLines[0].Item)
	}
	if cardLines[1].Item == "" {
		t.Error("the item comment's line carries no item")
	}
	for i, ev := range cardLines {
		if ev.Column != "" || ev.ColumnTitle != "" {
			t.Errorf("card journal line %d carries column %q and column_title %q, wanted neither", i+1, ev.Column, ev.ColumnTitle)
		}
	}
}

// TestAColumnViewPublishesItsCommentCount asserts dinah-518/criteria/4 on the
// library's own surface. The member rides beside the attachment count, it is
// omitted where the count is zero, and it differs between columns, so a build
// reading one column's count for every column fails.
func TestAColumnViewPublishesItsCommentCount(t *testing.T) {
	h := newHarness(t)
	first := h.library.Bench.Columns[0]
	second := h.library.Bench.Columns[1]
	third := h.library.Bench.Columns[2]
	wanted := map[string]int{first.ID: 2, second.ID: 1, third.ID: 0}
	for _, spelling := range []string{first.Slug, first.Slug, second.Slug} {
		if response := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: spelling, Text: "a note"}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("comment on %q: %s %s", spelling, response.Outcome, response.Refusal)
		}
		h.reopen()
	}

	views, err := h.library.Columns()
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	seen := 0
	for _, view := range views {
		want, declared := wanted[view.ID]
		if !declared {
			continue
		}
		seen++
		if view.CommentCount != want {
			t.Errorf("the column %s publishes comment_count %d, wanted %d", view.Title, view.CommentCount, want)
		}
		encoded, err := json.Marshal(view)
		if err != nil {
			t.Fatalf("encode the column %s: %v", view.Title, err)
		}
		switch want {
		case 0:
			// The zero is omitted rather than emitted, which is what a client
			// reads to tell an empty column from one it was told nothing
			// about.
			if strings.Contains(string(encoded), "comment_count") {
				t.Errorf("the column %s carries no comments and encodes %s", view.Title, encoded)
			}
		default:
			member := fmt.Sprintf("\"comment_count\":%d", want)
			if !strings.Contains(string(encoded), member) {
				t.Errorf("the column %s encodes %s, wanted %s in it", view.Title, encoded, member)
			}
		}
	}
	if seen != len(wanted) {
		t.Fatalf("the run read %d of the %d columns it names, so it asserted less than it says", seen, len(wanted))
	}
}
