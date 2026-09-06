package verb

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// linkTo writes a link into a card's frontmatter by hand, which is how a
// link reaches a card in this package's tests: no verb writes one, and the
// format is a tree of plain files a person is meant to be able to edit.
func linkTo(h *harness, ref, kind, to string) {
	h.t.Helper()
	path := h.card(ref).AnchorPath()
	text, err := os.ReadFile(path)
	if err != nil {
		h.t.Fatalf("read %s: %v", path, err)
	}
	closing := strings.LastIndex(string(text), "\n---\n")
	if closing < 0 {
		h.t.Fatalf("card %s carries no closing frontmatter fence", path)
	}
	block := "\nlinks:\n  - kind: " + kind + "\n    to: " + to
	edited := string(text[:closing]) + block + string(text[closing:])
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		h.t.Fatalf("write %s: %v", path, err)
	}
	h.reopen()
}

// TestShowCarriesTheFieldsTheCallerNamed asserts dinah-383 AC-3. Two cards
// stand in one table because the announcement's whole claim is that it names
// what the card holds rather than what the caller left out, and one card
// cannot show that: the card carrying comments must name comments and the card
// carrying none must not.
func TestShowCarriesTheFieldsTheCallerNamed(t *testing.T) {
	h := newHarness(t)
	full := h.add("A card with everything below it")
	other := h.add("A card to link to")
	linkTo(h, full, "relates_to", h.card(other).ID)
	h.attach(full, "note.txt", "attached bytes\n")
	h.comment(full, "A remark worth keeping.")
	bare := h.add("A card with nothing below it")

	for _, row := range []struct {
		name     string
		ref      string
		fields   string
		withheld []string
	}{
		{
			name:     "a card carrying links, attachments and comments",
			ref:      full,
			fields:   "card,body",
			withheld: []string{"links", "attachments", "comments", "path"},
		},
		{
			name:     "a card carrying no comments",
			ref:      bare,
			fields:   "card,body",
			withheld: []string{"path"},
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			detail, text, err := h.library.Show(&Request{
				Verb: "show", Actor: "alka", Card: row.ref, Fields: row.fields,
			})
			if err != nil {
				t.Fatalf("show %s: %v", row.ref, err)
			}
			if text != "" {
				t.Fatalf("a card came back as text: %q", text)
			}
			if detail.Card.Ref != row.ref {
				t.Errorf("wanted the card member, got %+v", detail.Card)
			}
			if len(detail.Links) != 0 || len(detail.Attachments) != 0 ||
				len(detail.Comments) != 0 || detail.Path != "" {
				t.Errorf("the answer carried a member the field list left out: %+v", detail)
			}
			if !reflect.DeepEqual(detail.Withheld, row.withheld) {
				t.Errorf("wanted withheld %v, got %v", row.withheld, detail.Withheld)
			}
			if detail.Reread != row.ref {
				t.Errorf("wanted reread %q, got %q", row.ref, detail.Reread)
			}
			// The payload is where the criterion is written, because a
			// member left out and a member carried empty are the same Go
			// value and only the marshalled answer tells them apart.
			encoded, err := json.Marshal(detail)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatalf("decode: %v\n%s", err, encoded)
			}
			for _, member := range []string{"card", "body", "withheld", "reread"} {
				if _, ok := payload[member]; !ok {
					t.Errorf("the payload does not carry %s: %s", member, encoded)
				}
			}
			for _, member := range []string{"links", "attachments", "comments", "path"} {
				if _, ok := payload[member]; ok {
					t.Errorf("the payload carries %s, which the field list left out: %s",
						member, encoded)
				}
			}
		})
	}

	// The control. Without it, an answer that carried nothing at all would
	// satisfy every assertion above about what the shaped answer omits.
	whole, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: full})
	if err != nil {
		t.Fatalf("show %s: %v", full, err)
	}
	if len(whole.Links) == 0 || len(whole.Attachments) == 0 || len(whole.Comments) == 0 {
		t.Fatalf("the unshaped answer carries no listing, so the shaped one proves nothing: %+v", whole)
	}
	if whole.Withheld != nil || whole.Reread != "" {
		t.Errorf("an unshaped answer withholds nothing and should announce nothing: %v %q",
			whole.Withheld, whole.Reread)
	}
	// The unshaped payload carries every member the type declares, which is
	// what makes the shaped payload's omissions the caller's doing rather
	// than this type's.
	encoded, err := json.Marshal(whole)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, encoded)
	}
	for _, member := range []string{"card", "body", "links", "attachments", "comments", "path"} {
		if _, ok := payload[member]; !ok {
			t.Errorf("the unshaped payload does not carry %s: %s", member, encoded)
		}
	}
	for _, member := range []string{"withheld", "reread"} {
		if _, ok := payload[member]; ok {
			t.Errorf("the unshaped payload announces %s and it withheld nothing: %s",
				member, encoded)
		}
	}
}

// TestShowRefusesAFieldItDoesNotCarry asserts dinah-383 AC-4. Every
// unrecognised name is reported rather than the first one reached, the
// declared set rides in the refusal, and the refusal is raised before the
// reference is resolved, which the second row drives by naming a card that
// does not exist and still getting the field refusal rather than the
// unknown-card one.
func TestShowRefusesAFieldItDoesNotCarry(t *testing.T) {
	h := newHarness(t)
	ref := h.add("A card")
	h.comment(ref, "A remark.")

	for _, row := range []struct {
		name   string
		card   string
		fields string
		detail string
	}{
		{
			name:   "two unrecognised names, both reported and sorted",
			card:   ref,
			fields: "comments,bogus,alsobogus",
			detail: "alsobogus, bogus",
		},
		{
			name:   "the refusal is raised before any card is read",
			card:   "fx-9999",
			fields: "bogus",
			detail: "bogus",
		},
		{
			name:   "a reference that is not a card has no members to select",
			card:   "intake",
			fields: "card,body",
			detail: "card,body",
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			detail, text, err := h.library.Show(&Request{
				Verb: "show", Actor: "alka", Card: row.card, Fields: row.fields,
			})
			if err == nil {
				t.Fatalf("wanted a refusal, got %+v %q", detail, text)
			}
			refusal, ok := err.(*contract.Refusal)
			if !ok {
				t.Fatalf("wanted a refusal, got %T %v", err, err)
			}
			if refusal.Name != contract.UnknownField {
				t.Fatalf("wanted %s, got %s", contract.UnknownField, refusal.Name)
			}
			if refusal.Detail != row.detail {
				t.Errorf("wanted the detail %q, got %q", row.detail, refusal.Detail)
			}
			if got := refusal.Extra["fields"]; got != strings.Join(DetailFields, ", ") {
				t.Errorf("the refusal does not name the declared set, got %q", got)
			}
		})
	}

	// The control for the second row: the same nonexistent card with no
	// field list is refused for being nonexistent, so the row above really
	// does show the field check running ahead of the resolution.
	if _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: "fx-9999"}); err == nil {
		t.Fatal("a nonexistent card resolved")
	} else if refusal, ok := err.(*contract.Refusal); !ok || refusal.Name != contract.UnknownCard {
		t.Fatalf("wanted %s on a card that does not exist, got %v", contract.UnknownCard, err)
	}
}

// TestTheDetailVocabularyIsTheDetailItself asserts that the closed set the
// help table, the refusal sentence and Library.Show all read is the JSON
// member names of Detail, so a member added to the payload cannot be left out
// of the set a caller may name.
//
// Withheld and Reread are excluded by name, because neither is a member of the
// card: both are statements about the answer, and neither is something a
// caller may ask to be served.
func TestTheDetailVocabularyIsTheDetailItself(t *testing.T) {
	var members []string
	value := reflect.TypeOf(Detail{})
	for i := 0; i < value.NumField(); i++ {
		name, _, _ := strings.Cut(value.Field(i).Tag.Get("json"), ",")
		if name == "" || name == "withheld" || name == "reread" {
			continue
		}
		members = append(members, name)
	}
	declared := append([]string{}, DetailFields...)
	sort.Strings(members)
	sort.Strings(declared)
	if !reflect.DeepEqual(members, declared) {
		t.Errorf("Detail carries the members %v and DetailFields declares %v", members, declared)
	}
	set, ok := VocabularyFor("show", "fields")
	if !ok {
		t.Fatal("show's fields argument declares no vocabulary")
	}
	if !reflect.DeepEqual(set.Values, DetailFields) {
		t.Errorf("the declared vocabulary is %v and DetailFields is %v", set.Values, DetailFields)
	}
	// The order is the contract as much as the membership is, because
	// withheld reports in it and two runs against one card have to compose
	// one string.
	if !reflect.DeepEqual(DetailFields,
		[]string{"card", "body", "links", "attachments", "comments", "path"}) {
		t.Errorf("the declared order moved: %v", DetailFields)
	}
}
