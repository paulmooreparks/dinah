package verb

import (
	"encoding/json"
	"reflect"
	"testing"

	"dinah/internal/contract"
)

// TestTheHandoffIsWhatTheLastStationSaid is dinah-648's rule for the handoff
// member, read off one card that crossed four columns. The filer's remark,
// the two comments the first station wrote, a column crossed in silence and
// a remark the stay under way wrote all stand on the one card, because the
// rule is about which stay's comments are chosen, and a card carrying a
// single stay's comments would pass whatever the rule turned out to be.
func TestTheHandoffIsWhatTheLastStationSaid(t *testing.T) {
	h := newHarness(t)
	card := h.add("A card handed from station to station")
	h.comment(card, "## INTAKE\n\nThe filer's remark.\n")
	h.at(card, doing)
	h.comment(card, "## FINDINGS\n\nWhat the first station found.\n")
	h.comment(card, "## HANDOFF\n\nWhat the first station handed on.\n")
	h.at(card, review)
	h.at(card, aftercare)
	h.comment(card, "A remark the stay under way wrote.\n")

	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: card, Fields: "handoff"})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	var subjects []string
	for _, entry := range detail.Handoff {
		subjects = append(subjects, entry.Subject)
		if entry.Body == "" {
			t.Errorf("the handoff entry %s carries no body, and a handoff is served in full", entry.Ref)
		}
	}
	if want := []string{"FINDINGS", "HANDOFF"}; !reflect.DeepEqual(subjects, want) {
		t.Errorf("wanted the first station's two comments %v, got %v", want, subjects)
	}
	// The announcement names the members the card holds and this answer left
	// out, so it names the comments, which the card holds four of, and never
	// the handoff, which the answer carried.
	if !reflect.DeepEqual(detail.Withheld, []string{"card", "comments", "path"}) {
		t.Errorf("wanted withheld card, comments, path; got %v", detail.Withheld)
	}

	// The control. A card that moved before anybody wrote on it, and was
	// written on only since, has no handoff, and the announcement does not
	// name one: the comment it holds is the stay under way's own.
	quiet := h.add("A card nobody wrote on before it moved")
	h.at(quiet, doing)
	h.comment(quiet, "Written after the only move.\n")
	detail, _, _, _, err = h.library.Show(&Request{Verb: "show", Actor: "alka", Card: quiet, Fields: "handoff"})
	if err != nil {
		t.Fatalf("show %s: %v", quiet, err)
	}
	if len(detail.Handoff) != 0 {
		t.Errorf("a card written on only since its last move carries a handoff: %+v", detail.Handoff)
	}
	if !reflect.DeepEqual(detail.Withheld, []string{"card", "comments", "path"}) {
		t.Errorf("wanted withheld card, comments, path on the quiet card; got %v", detail.Withheld)
	}
}

// TestBriefIsTheCardAStationOpensOn asserts what --brief carries and what it
// withholds, on the payload as much as on the Go value, since a member left
// out and a member carried empty are the same value and only the marshalled
// answer tells them apart. The card holds every member the flag decides
// about: a handoff and a later remark, an attachment, and a pending item
// beside a verified one.
func TestBriefIsTheCardAStationOpensOn(t *testing.T) {
	h := newHarness(t)
	card := h.add("A card a station opens on")
	h.comment(card, "## HANDOFF\n\nRead this first.\n")
	h.at(card, doing)
	h.comment(card, "The stay under way.\n")
	h.attach(card, "spec.md", "the contract\n")
	h.item(card, "b00000000001", "kind: acceptance_criterion\nstate: pending\nordinal: 1\n", "Still to verify.")
	h.item(card, "b00000000002", "kind: acceptance_criterion\nstate: verified\nordinal: 2\n", "Already verified.")

	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: card, Brief: true})
	if err != nil {
		t.Fatalf("show --brief: %v", err)
	}
	for _, name := range []string{"card", "body", "links", "attachments", "handoff", "checklist"} {
		if !detail.Carries(name) {
			t.Errorf("the brief does not carry %s", name)
		}
	}
	for _, name := range []string{"comments", "path"} {
		if detail.Carries(name) {
			t.Errorf("the brief carries %s, which a station asks for when it needs it", name)
		}
	}
	if len(detail.Handoff) != 1 || detail.Handoff[0].Subject != "HANDOFF" || detail.Handoff[0].Body == "" {
		t.Errorf("wanted the one handoff comment in full, got %+v", detail.Handoff)
	}
	if len(detail.Checklist) != 1 || detail.Checklist[0].State != "pending" {
		t.Errorf("wanted the one pending item alone, got %+v", detail.Checklist)
	}
	if len(detail.Attachments) != 1 {
		t.Errorf("wanted the one attachment, got %+v", detail.Attachments)
	}
	if !reflect.DeepEqual(detail.Withheld, []string{"comments", "checklist", "path"}) {
		t.Errorf("wanted withheld comments, checklist, path; got %v", detail.Withheld)
	}
	// The recovery names the flag the caller wrote rather than the filter it
	// implies, since a reader told to drop --unresolved never typed it.
	if !reflect.DeepEqual(detail.NarrowedBy(), []string{flagBrief}) {
		t.Errorf("wanted the checklist narrowed by %s, got %v", flagBrief, detail.NarrowedBy())
	}

	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, encoded)
	}
	for _, member := range []string{"card", "body", "attachments", "handoff", "checklist", "withheld", "reread"} {
		if _, ok := payload[member]; !ok {
			t.Errorf("the payload does not carry %s: %s", member, encoded)
		}
	}
	for _, member := range []string{"comments", "path"} {
		if _, ok := payload[member]; ok {
			t.Errorf("the payload carries %s, which the brief leaves out: %s", member, encoded)
		}
	}

	// A caller who wrote both narrowing flags is told to drop both, since
	// dropping one leaves the other narrowing.
	detail, _, _, _, err = h.library.Show(&Request{Verb: "show", Actor: "alka", Card: card, Brief: true, Unresolved: true})
	if err != nil {
		t.Fatalf("show --brief --unresolved: %v", err)
	}
	if !reflect.DeepEqual(detail.NarrowedBy(), []string{flagBrief, flagUnresolved}) {
		t.Errorf("wanted both flags named, got %v", detail.NarrowedBy())
	}
}

// TestBriefNamesAFieldSetOfItsOwn asserts the two refusals: --brief beside
// --fields and --brief beside --all each name two field sets on one call, and
// the refusal names --brief and the flag it met. Nothing is read before either
// is raised, so the card need not exist.
func TestBriefNamesAFieldSetOfItsOwn(t *testing.T) {
	h := newHarness(t)
	for _, row := range []struct {
		name   string
		req    Request
		detail string
	}{
		{"beside --fields", Request{Fields: "card"}, flagBrief + " conflicts with --fields"},
		{"beside --all", Request{All: true}, flagBrief + " conflicts with " + flagAll},
	} {
		t.Run(row.name, func(t *testing.T) {
			req := row.req
			req.Verb, req.Actor, req.Card, req.Brief = "show", "alka", "fx-99", true
			detail, _, _, text, err := h.library.Show(&req)
			if err == nil {
				t.Fatalf("wanted a refusal, got %+v %q", detail, text)
			}
			refusal, ok := err.(*contract.Refusal)
			if !ok {
				t.Fatalf("wanted a refusal, got %T %v", err, err)
			}
			if refusal.Name != contract.Usage {
				t.Fatalf("wanted %s, got %s", contract.Usage, refusal.Name)
			}
			if refusal.Detail != row.detail {
				t.Errorf("wanted the detail %q, got %q", row.detail, refusal.Detail)
			}
		})
	}
}
