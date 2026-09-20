package verb

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// linkTo writes a link into a card's frontmatter by hand, which is one of the
// two ways a link reaches a card; `dinah link` is the other, and planting the
// key here keeps these tests off that verb's own path. The format is a tree of
// plain files a person is meant to be able to edit.
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
			name:     "a card carrying links, attachments, and comments",
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
			detail, _, _, text, err := h.library.Show(&Request{
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
	whole, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: full})
	if err != nil {
		t.Fatalf("show %s: %v", full, err)
	}
	if len(whole.Links) == 0 || len(whole.Attachments) == 0 || len(whole.Comments) == 0 {
		t.Fatalf("the unshaped answer carries no listing, so the shaped one proves nothing: %+v", whole)
	}
	// An unshaped answer carries every member and carries the collections as
	// indexes, so what it announces is the modifier of each collection the
	// card holds. This card holds one comment and no checklist item, so it
	// announces one name and reaches for a reread.
	if want := []string{"comments.full"}; !reflect.DeepEqual(whole.Withheld, want) {
		t.Errorf("an unshaped answer announces the bodies it did not serve, wanted %v, got %v",
			want, whole.Withheld)
	}
	if whole.Reread != full {
		t.Errorf("wanted the reread %q, got %q", full, whole.Reread)
	}
	if body := whole.Comments[0].Body; body != "" {
		t.Errorf("an unshaped answer carried a comment body: %q", body)
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
		if _, ok := payload[member]; !ok {
			t.Errorf("the unshaped payload does not announce %s, and it served no comment body: %s",
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
		{
			name:   "a list written with nothing in it names no member",
			card:   ref,
			fields: ",",
			detail: ",",
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			detail, _, _, text, err := h.library.Show(&Request{
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
			if got := refusal.Extra["fields"]; got != strings.Join(DetailSelectors, ", ") {
				t.Errorf("the refusal does not name the declared set, got %q", got)
			}
		})
	}

	// The control for the last row: an argument that is blank rather than
	// written with a separator in it is an argument the caller did not give,
	// so it reads as the unshaped call it has always been and is not refused
	// alongside the bare comma above.
	if detail, _, _, _, err := h.library.Show(&Request{
		Verb: "show", Actor: "alka", Card: ref, Fields: "   ",
	}); err != nil {
		t.Fatalf("a blank field list was refused: %v", err)
	} else if want := []string{"comments.full"}; !reflect.DeepEqual(detail.Withheld, want) {
		t.Errorf("a blank field list shaped the answer: wanted the unshaped read's %v, got %v",
			want, detail.Withheld)
	}

	// The control for the second row: the same nonexistent card with no
	// field list is refused for being nonexistent, so the row above really
	// does show the field check running ahead of the resolution.
	if _, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: "fx-9999"}); err == nil {
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
	// The vocabulary is the set a caller may write rather than the payload's
	// own members, because two of the nine names are depths on a member
	// rather than members. The reflection half above is what keeps the seven
	// exactly the payload's members while these nine grow.
	if !reflect.DeepEqual(set.Values, DetailSelectors) {
		t.Errorf("the declared vocabulary is %v and DetailSelectors is %v", set.Values, DetailSelectors)
	}
	// The order is the contract as much as the membership is, because
	// withheld reports in it and two runs against one card have to compose
	// one string.
	if !reflect.DeepEqual(DetailFields,
		[]string{"card", "body", "links", "attachments", "comments", "checklist", "path"}) {
		t.Errorf("the declared order moved: %v", DetailFields)
	}

	// The second invariant, in three parts. DetailSelectors is DetailFields
	// with each modifier inserted after the member it names, every modifier
	// names a member this payload has, and no modifier is itself a member,
	// which is what keeps the reflection half above standing while the
	// vocabulary grows.
	woven := make([]string, 0, len(DetailFields)+len(DetailModifiers))
	modifiers := map[string][]string{}
	for _, modifier := range DetailModifiers {
		base := baseOfModifier(modifier)
		modifiers[base] = append(modifiers[base], modifier)
	}
	for _, member := range DetailFields {
		woven = append(woven, member)
		woven = append(woven, modifiers[member]...)
	}
	if !reflect.DeepEqual(DetailSelectors, woven) {
		t.Errorf("DetailSelectors is %v and DetailFields woven with its modifiers is %v", DetailSelectors, woven)
	}
	t.Logf("wove %d members and %d modifiers into %d selectors",
		len(DetailFields), len(DetailModifiers), len(woven))

	member := map[string]bool{}
	for _, name := range DetailFields {
		member[name] = true
	}
	payload := map[string]bool{}
	for _, name := range members {
		payload[name] = true
	}
	if len(DetailModifiers) == 0 {
		t.Fatal("no modifier is declared, so the two checks below read nothing")
	}
	for _, modifier := range DetailModifiers {
		if base := baseOfModifier(modifier); !member[base] {
			t.Errorf("the modifier %s names %q and that is not a member of DetailFields", modifier, base)
		}
		if payload[modifier] {
			t.Errorf("the modifier %s is a JSON member of Detail, and a modifier is a depth rather than a member", modifier)
		}
	}
	t.Logf("checked %d modifiers against %d members and %d payload names",
		len(DetailModifiers), len(DetailFields), len(members))
}

// TestTheDetailPayloadCarriesEveryMemberDetailDeclares stands over the mirror
// struct inside Detail.MarshalJSON. That struct is a second declaration of the
// same wire format, and the language ties it to nothing: a member added to
// Detail and forgotten there is dropped from every payload, shaped and
// unshaped alike, and every other test in this file would still pass.
//
// The guard has two halves and needs both. The literal below is asserted total
// by reflection, so a member added to Detail fails here until somebody fills it
// in, and the payload is then read back for every name the type declares, so a
// member the mirror does not carry fails there. Neither half detects the drift
// on its own, because a literal that leaves a member zero writes an answer that
// omits it for a reason of its own.
//
// The literal fills withheld and reread alongside every member of the card,
// which no live answer does, since an answer that withheld something did not
// carry it. This is a test of the marshaller rather than of Library.Show, and
// the question it asks is whether every name the type declares can reach the
// wire at all.
func TestTheDetailPayloadCarriesEveryMemberDetailDeclares(t *testing.T) {
	whole := Detail{
		Card:        CardView{Ref: "fx-1", Title: "A card"},
		Body:        "The framing prose.",
		Links:       []LinkView{{Kind: "relates_to", To: "0000", Ref: "fx-2"}},
		Attachments: []AttachmentView{{ID: "0001", Filename: "note.txt"}},
		Comments:    []CommentView{{ID: "0002", Body: "A remark."}},
		Checklist: []ItemView{{
			ID: "0003", Ordinal: 1, Ref: "fx-1/questions/1", Kind: "open_question",
			State: "pending", Text: "A question.",
		}},
		Path:     filepath.Join("cards", "fx-1", "card.md"),
		Withheld: []string{"links"},
		Reread:   "fx-1",
	}
	declared := map[string]bool{}
	shape := reflect.TypeOf(whole)
	value := reflect.ValueOf(whole)
	for i := 0; i < shape.NumField(); i++ {
		field := shape.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" {
			continue
		}
		declared[name] = true
		if value.Field(i).IsZero() {
			t.Errorf("the literal leaves %s empty, so this test cannot tell whether the payload carries it", name)
		}
	}
	encoded, err := json.Marshal(whole)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, encoded)
	}
	for name := range declared {
		if _, ok := payload[name]; !ok {
			t.Errorf("Detail declares %s and MarshalJSON does not write it: %s", name, encoded)
		}
	}
	for name := range payload {
		if !declared[name] {
			t.Errorf("the payload carries %s and Detail declares no such member: %s", name, encoded)
		}
	}
}

// indexFixture builds the card every case below reads: links, an attachment,
// three comments one of which carries an attachment of its own, and six
// checklist items standing in six states. One card carries all of it because
// the announcement's claim is about one answer rather than about one member,
// and six calls against six cards would agree with each other whatever the
// rule turned out to be.
func indexFixture(h *harness) (card, other string) {
	h.t.Helper()
	card = h.ready("A card worth reading twice")
	other = h.add("A card to link to")
	linkTo(h, card, "relates_to", h.card(other).ID)
	h.attach(card, "note.txt", "attached bytes\n")
	h.comment(card, "## TRIAGE\n\nThe first remark, and it runs to a second line.\n")
	h.comment(card, "The second remark.\n")
	h.comment(card, "The third remark.\n")
	h.attach(card+"/comments/1", "evidence.txt", "the evidence\n")
	h.item(card, "b00000000001", "kind: open_question\nstate: pending\nordinal: 1\n",
		"Which vendor do we cite?")
	// Three of the six items are settled, and each designates a comment of
	// its own as its answer. The designation is planted rather than settled
	// through a verb because the fixture names the references it asserts
	// against, and a verb would mint identifiers the test cannot spell.
	answered := h.item(card, "b00000000002", "kind: acceptance_criterion\nstate: resolved\nordinal: 2\nresolution: "+card+"/criteria/1/comments/1\n",
		"The endpoint returns 404 for an unknown id.")
	h.plantComment(answered, "c00000000001", 1, "alka", "the endpoint answers")
	measured := h.item(card, "b00000000003", "kind: acceptance_criterion\nstate: verified\nordinal: 3\nresolution: "+card+"/criteria/2/comments/1\n",
		"The listing answers in one round.")
	h.plantComment(measured, "c00000000002", 1, "alka", "measured on the branch")
	red := h.item(card, "b00000000004", "kind: acceptance_criterion\nstate: failed\nordinal: 4\nresolution: "+card+"/criteria/3/comments/1\n",
		"The sweep reports the size of the set it swept.")
	h.plantComment(red, "c00000000003", 1, "alka", "the run came back red")
	h.item(card, "b00000000005", "kind: decision\nstate:\nordinal: 5\n",
		"Whose contract the numbers come from.")
	h.item(card, "b00000000006", "kind: decision\nstate: marinating\nordinal: 6\n",
		"Whether the cursor is opaque.")
	return card, other
}

// showing runs one read of a card and fails the test unless it succeeded.
func showing(h *harness, req *Request) *Detail {
	h.t.Helper()
	req.Verb, req.Actor = "show", "alka"
	detail, _, _, _, err := h.library.Show(req)
	if err != nil {
		h.t.Fatalf("show %s: %v", req.Card, err)
	}
	if detail == nil {
		h.t.Fatalf("show %s answered no card detail", req.Card)
	}
	return detail
}

// TestTheAnnouncementCoversEveryCallShape asserts the whole of withheld over
// seven calls against one fixture, rather than asserting that one name is
// present in each. A check for presence passes on an announcement that names
// everything, which is exactly the build this rule replaced.
//
// Two rows carry the weight. The unshaped call is what fails a build that
// left the `chosen != nil` guard standing, since such a build announces
// nothing at all. The call naming all seven members with both modifiers is
// what fails a build that announces every member on a nil selection, since
// such a build announces the lot.
//
// The specification's own worked table understates three of these rows, and
// the decision filed at Implement records the repair. Two of its rows list
// the collection names alone on a fixture that also holds links, an
// attachment and a path, and one announces a modifier of a member its answer
// carried no entry of, which is the half section 3.6's own note rules out.
// The rule those rows are worked examples of is what this pins.
func TestTheAnnouncementCoversEveryCallShape(t *testing.T) {
	h := newHarness(t)
	card, _ := indexFixture(h)
	bare := h.ready("A card holding nothing below it")

	for _, row := range []struct {
		name     string
		card     string
		fields   string
		since    string
		filtered bool
		withheld []string
	}{
		{
			name:     "no field list at all",
			card:     card,
			withheld: []string{"comments.full", "checklist.full"},
		},
		{
			name:     "a field list naming two members",
			card:     card,
			fields:   "card,body",
			withheld: []string{"links", "attachments", "comments", "checklist", "path"},
		},
		{
			name:     "a field list naming one modifier",
			card:     card,
			fields:   "comments.full",
			withheld: []string{"card", "links", "attachments", "checklist", "path"},
		},
		{
			name:   "every member and both modifiers",
			card:   card,
			fields: "card,body,links,attachments,comments.full,checklist.full,path",
		},
		{
			name:     "an ordinal short of the comments the card holds",
			card:     card,
			since:    "2",
			withheld: []string{"comments.full", "checklist.full"},
		},
		{
			name:     "an ordinal of zero",
			card:     card,
			since:    "0",
			withheld: []string{"checklist.full"},
		},
		{
			name:     "a filter narrowing a member named in full",
			card:     card,
			fields:   "card,checklist.full",
			filtered: true,
			withheld: []string{"links", "attachments", "comments", "checklist", "path"},
		},
		{
			name: "a card holding no comment and no checklist item",
			card: bare,
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			detail := showing(h, &Request{
				Card: row.card, Fields: row.fields,
				SinceComment: row.since, Unresolved: row.filtered,
			})
			if !reflect.DeepEqual(detail.Withheld, row.withheld) {
				t.Errorf("wanted withheld %v, got %v", row.withheld, detail.Withheld)
			}
			want := row.card
			if len(row.withheld) == 0 {
				want = ""
			}
			if detail.Reread != want {
				t.Errorf("wanted the reread %q, got %q", want, detail.Reread)
			}
		})
	}
}

// TestACommentIndexEntryCarriesTheWholeCommentButItsBody asserts what an
// index entry is: every member CommentView declares except the body, with the
// body present and empty so a reader can tell an unserved body from a comment
// nobody wrote.
//
// The member set is asserted as a whole rather than name by name, so a member
// added to CommentView and left out of this contract fails here rather than
// riding onto the wire unnoticed.
func TestACommentIndexEntryCarriesTheWholeCommentButItsBody(t *testing.T) {
	h := newHarness(t)
	card, _ := indexFixture(h)
	detail := showing(h, &Request{Card: card})

	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload struct {
		Comments []map[string]json.RawMessage `json:"comments"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, encoded)
	}
	if len(payload.Comments) != 3 {
		t.Fatalf("wanted the three comments the fixture holds, got %d: %s", len(payload.Comments), encoded)
	}
	withAttachment := []string{"id", "ordinal", "ref", "ts", "author", "subject", "size", "attachments", "body"}
	without := []string{"id", "ordinal", "ref", "ts", "author", "subject", "size", "body"}
	for i, entry := range payload.Comments {
		wanted := without
		if i == 0 {
			wanted = withAttachment
		}
		got := make([]string, 0, len(entry))
		for name := range entry {
			got = append(got, name)
		}
		sort.Strings(got)
		want := append([]string{}, wanted...)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("comment %d carries the members %v, wanted %v", i+1, got, want)
		}
		if string(entry["body"]) != `""` {
			t.Errorf("comment %d carries a body on an index entry: %s", i+1, entry["body"])
		}
	}

	// The comment carrying an attachment carries it here too, with the
	// reference and the path a caller opens it by, because an attachment
	// view is a reference and a path rather than a payload.
	first := detail.Comments[0]
	if len(first.Attachments) != 1 {
		t.Fatalf("the first comment's attachment did not reach the index: %+v", first)
	}
	if got, want := first.Attachments[0].Ref, card+"/comments/1/attachments/1"; got != want {
		t.Errorf("the attachment's reference is %q, wanted %q", got, want)
	}
	if first.Attachments[0].Path == "" {
		t.Error("the attachment reached the index with no path, so a caller cannot open it")
	}

	// The subject drops the heading markup a comment opens with, and the
	// size prices the read that would fetch the body.
	if got, want := first.Subject, "TRIAGE"; got != want {
		t.Errorf("the subject is %q, wanted %q", got, want)
	}
	if got := first.Size; got != len("## TRIAGE\n\nThe first remark, and it runs to a second line.\n") {
		t.Errorf("the size is %d and does not price the body", got)
	}
	for i, comment := range detail.Comments {
		if comment.Ordinal != i+1 {
			t.Errorf("comment %d carries the ordinal %d", i+1, comment.Ordinal)
		}
	}
}

// TestASubjectIsAFirstLineTrimmedAndCapped drives the three rules the
// composition holds to: a Markdown heading reads as its words, a long first
// line is cut at the cap and closed with one ellipsis, and the cut falls on a
// rune boundary rather than in the middle of a multi-byte rune.
//
// The boundary case plants a three-byte rune straddling the limit, so a
// build that sliced bytes would answer a value Go renders as a replacement
// character and this comparison would fail on it.
func TestASubjectIsAFirstLineTrimmedAndCapped(t *testing.T) {
	long := strings.Repeat("a", subjectCap+40)
	straddling := strings.Repeat("a", subjectCap-1) + "\u4e2d\u4e2d\u4e2d"
	for _, row := range []struct {
		name string
		body string
		want string
	}{
		{name: "a plain first line", body: "A remark.\nA second line.\n", want: "A remark."},
		{name: "a Markdown heading", body: "## SPEC HANDOFF\n\nthe rest\n", want: "SPEC HANDOFF"},
		{name: "a deeper heading", body: "#### Findings, round 2\n", want: "Findings, round 2"},
		{name: "a leading blank line", body: "\n\n  A remark.  \n", want: "A remark."},
		{name: "a body of nothing", body: "", want: ""},
		{name: "a line at the cap", body: strings.Repeat("a", subjectCap), want: strings.Repeat("a", subjectCap)},
		{name: "a line past the cap", body: long, want: strings.Repeat("a", subjectCap) + subjectEllipsis},
		{
			name: "a multi-byte rune straddling the cap",
			body: straddling,
			want: strings.Repeat("a", subjectCap-1) + "\u4e2d" + subjectEllipsis,
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			if got := subjectOf(row.body); got != row.want {
				t.Errorf("subjectOf(%q) is %q, wanted %q", row.body, got, row.want)
			}
		})
	}
	if got := len([]rune(subjectOf(straddling))); got != subjectCap+1 {
		t.Errorf("the capped subject runs to %d runes, wanted the cap and one ellipsis", got)
	}
}

// TestTheFullFormsCarryEveryBody asserts that the two modifiers recover what
// the index stopped serving, and that they recover it on the same entries the
// index carried rather than on a second member.
//
// Both halves assert the number of entries and the number of bodies filled,
// because an answer that carried nothing would satisfy a check on either
// count alone.
func TestTheFullFormsCarryEveryBody(t *testing.T) {
	h := newHarness(t)
	card, _ := indexFixture(h)

	comments := showing(h, &Request{Card: card, Fields: "comments.full"})
	if len(comments.Comments) != 3 {
		t.Fatalf("wanted three entries, got %d", len(comments.Comments))
	}
	filled := 0
	for _, comment := range comments.Comments {
		if comment.Body != "" {
			filled++
		}
		if comment.Ref == "" || comment.Ordinal == 0 || comment.Subject == "" || comment.Size == 0 {
			t.Errorf("a full entry lost an index member: %+v", comment)
		}
	}
	if filled != 3 {
		t.Errorf("comments.full filled %d of three bodies", filled)
	}
	for _, name := range comments.Withheld {
		if strings.HasPrefix(name, "comments") {
			t.Errorf("comments.full announced %s: %v", name, comments.Withheld)
		}
	}

	checklist := showing(h, &Request{Card: card, Fields: "checklist.full"})
	if len(checklist.Checklist) != 6 {
		t.Fatalf("wanted six items, got %d", len(checklist.Checklist))
	}
	opened := 0
	for _, item := range checklist.Checklist {
		if item.Designated != nil {
			opened++
		}
	}
	if opened != 3 {
		t.Errorf("checklist.full opened %d of the three designated comments the fixture holds", opened)
	}
	for _, name := range checklist.Withheld {
		if strings.HasPrefix(name, "checklist") {
			t.Errorf("checklist.full announced %s: %v", name, checklist.Withheld)
		}
	}

	// The control. The same members read as an index carry neither, so the
	// two assertions above are about the modifier rather than about the
	// fixture.
	index := showing(h, &Request{Card: card})
	for _, comment := range index.Comments {
		if comment.Body != "" {
			t.Errorf("the index carried a body: %+v", comment)
		}
	}
	for _, item := range index.Checklist {
		if item.Designated != nil {
			t.Errorf("the index opened a designated comment: %+v", item)
		}
	}
}

// TestSinceFillsTheBodiesPastAnOrdinal asserts that the filter fills the
// bodies it names and no others while the index goes on carrying every
// comment, that an ordinal of zero answers what the modifier answers, and
// that an ordinal past the end is an answer rather than a refusal.
//
// The entry count and the filled count are asserted separately, because a
// run that carried nothing would satisfy a check on either one alone.
func TestSinceFillsTheBodiesPastAnOrdinal(t *testing.T) {
	h := newHarness(t)
	card, _ := indexFixture(h)

	detail := showing(h, &Request{Card: card, SinceComment: "2"})
	if len(detail.Comments) != 3 {
		t.Fatalf("the index dropped a comment: wanted three, got %d", len(detail.Comments))
	}
	filled := 0
	for _, comment := range detail.Comments {
		if (comment.Body != "") != (comment.Ordinal > 2) {
			t.Errorf("comment %d carries the body %q, and the ordinal decides", comment.Ordinal, comment.Body)
		}
		if comment.Body != "" {
			filled++
		}
	}
	if filled != 1 {
		t.Errorf("--since 2 filled %d of the one body past the second", filled)
	}

	// An ordinal of zero answers, for this member, what the modifier
	// answers. The member is compared rather than the whole answer, because
	// the two calls announce differently: one announces checklist.full
	// alone and the other announces every member it left out.
	zero := showing(h, &Request{Card: card, Fields: "comments", SinceComment: "0"})
	whole := showing(h, &Request{Card: card, Fields: "comments.full"})
	byOrdinal, err := json.Marshal(zero.Comments)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	byName, err := json.Marshal(whole.Comments)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(byOrdinal, byName) {
		t.Errorf("--since 0 answers\n%s\nand comments.full answers\n%s", byOrdinal, byName)
	}

	// An ordinal past the end is not refused, on the card's own decision: a
	// station polling with an ordinal it remembers must not be turned away
	// for having remembered a comment that has since been deleted.
	past := showing(h, &Request{Card: card, Fields: "comments", SinceComment: "99"})
	if len(past.Comments) != 3 {
		t.Fatalf("an ordinal past the end dropped an entry: got %d", len(past.Comments))
	}
	for _, comment := range past.Comments {
		if comment.Body != "" {
			t.Errorf("an ordinal past the end filled a body: %+v", comment)
		}
	}
}

// TestUnresolvedCarriesWhatStillHoldsTheCard asserts that the filter answers
// the predicate a column hold reads rather than either of the two predicates
// beside it. The fixture stands one item in each of the four declared states
// and two outside them, so an answer built on bench.ItemIsResolved fails on
// the failed item, which is the one state meaning the work is wrong.
//
// Both counts are asserted, four carried and two left out, so a build that
// answered nothing and a build that answered everything each fail.
func TestUnresolvedCarriesWhatStillHoldsTheCard(t *testing.T) {
	h := newHarness(t)
	card, _ := indexFixture(h)

	detail := showing(h, &Request{Card: card, Fields: "checklist", Unresolved: true})
	carried := map[string]string{}
	for _, item := range detail.Checklist {
		carried[item.ID] = item.State
	}
	for _, id := range []string{"b00000000001", "b00000000004", "b00000000005", "b00000000006"} {
		if _, held := carried[id]; !held {
			t.Errorf("the item %s releases no column hold and the filter dropped it: %v", id, carried)
		}
	}
	for _, id := range []string{"b00000000002", "b00000000003"} {
		if state, held := carried[id]; held {
			t.Errorf("the item %s is %s, which lifts a column hold, and the filter carried it", id, state)
		}
	}
	if len(detail.Checklist) != 4 {
		t.Errorf("the filter carried %d items, wanted the four the card holds outstanding", len(detail.Checklist))
	}

	// The control. The same card unfiltered carries all six, so the two
	// counts above are the filter's doing rather than the fixture's.
	all := showing(h, &Request{Card: card, Fields: "checklist"})
	if len(all.Checklist) != 6 {
		t.Fatalf("the unfiltered read carries %d items, so the filtered one proves nothing", len(all.Checklist))
	}
	if len(all.Checklist)-len(detail.Checklist) != 2 {
		t.Errorf("the filter left out %d items, wanted two", len(all.Checklist)-len(detail.Checklist))
	}

	// The filter and the modifier compose rather than collide: the answer
	// carries the outstanding items with their text and their designated
	// answers whole.
	both := showing(h, &Request{Card: card, Fields: "checklist.full", Unresolved: true})
	if len(both.Checklist) != 4 {
		t.Fatalf("the composed call carries %d items, wanted four", len(both.Checklist))
	}
	answers := 0
	for _, item := range both.Checklist {
		if item.Designated != nil {
			answers++
		}
	}
	if answers != 1 {
		t.Errorf("the composed call opened %d answers, and one outstanding item designates one", answers)
	}
}

// TestAFilterWithoutItsMemberIsRefused asserts that a narrowing this answer
// would drop is turned away rather than accepted and ignored, which is the
// rule the guides already give: an argument a tool would accept and then drop
// tells the caller a check ran when none did.
//
// Every refusing row sits beside an accepting one in the test below it, so a
// build that turned every filtered call away fails that one rather than
// passing this.
func TestAFilterWithoutItsMemberIsRefused(t *testing.T) {
	h := newHarness(t)
	card, _ := indexFixture(h)
	before := len(contract.Introduced)

	for _, row := range []struct {
		name       string
		card       string
		fields     string
		since      string
		unresolved bool
		detail     string
	}{
		{
			name:   "an ordinal beside a field list that leaves the comments out",
			card:   card,
			fields: "card,body",
			since:  "4",
			detail: "--since",
		},
		{
			name:       "the filter beside a field list that leaves the checklist out",
			card:       card,
			fields:     "card,body",
			unresolved: true,
			detail:     "--unresolved",
		},
		{
			name:   "an ordinal on a reference that is not a card",
			card:   card + "/comments/1",
			since:  "4",
			detail: "--since",
		},
		{
			name:       "the filter on a reference that is not a card",
			card:       card + "/comments/1",
			unresolved: true,
			detail:     "--unresolved",
		},
		{
			name:   "an ordinal beside the modifier of its own member",
			card:   card,
			fields: "comments.full",
			since:  "4",
			detail: "--since beside comments.full",
		},
		{
			name:   "an ordinal that is not a whole number",
			card:   card,
			since:  "yesterday",
			detail: "--since yesterday",
		},
		{
			name:   "an ordinal below zero",
			card:   card,
			since:  "-1",
			detail: "--since -1",
		},
		{
			// The refusal is raised before anything is resolved, which this
			// row drives by naming a card that does not exist: a build
			// resolving first would answer dinah.unknown-card.
			name:       "a filter on a card that does not exist",
			card:       "fx-9999",
			fields:     "card,body",
			unresolved: true,
			detail:     "--unresolved",
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			detail, _, _, text, err := h.library.Show(&Request{
				Verb: "show", Actor: "alka", Card: row.card, Fields: row.fields,
				SinceComment: row.since, Unresolved: row.unresolved,
			})
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

	// No refusal name is minted for any of this, so the name roster is the
	// length it was.
	if after := len(contract.Introduced); after != before {
		t.Errorf("the introduced names moved from %d to %d, and this change mints none", before, after)
	}
}

// TestAFilteredCallThatNamesItsMemberIsAnswered is the accepting half of the
// refusals above. Without it a build refusing every filtered call would
// satisfy every row of that table.
func TestAFilteredCallThatNamesItsMemberIsAnswered(t *testing.T) {
	h := newHarness(t)
	card, _ := indexFixture(h)

	byOrdinal := showing(h, &Request{Card: card, Fields: "card,comments", SinceComment: "2"})
	if len(byOrdinal.Comments) != 3 {
		t.Errorf("--since 2 --fields card,comments carried %d comments", len(byOrdinal.Comments))
	}
	if byOrdinal.Card.Ref != card {
		t.Errorf("the accepted call lost the card member: %+v", byOrdinal.Card)
	}

	filtered := showing(h, &Request{Card: card, Fields: "card,checklist", Unresolved: true})
	if len(filtered.Checklist) != 4 {
		t.Errorf("--unresolved --fields card,checklist carried %d items", len(filtered.Checklist))
	}
	if filtered.Card.Ref != card {
		t.Errorf("the accepted call lost the card member: %+v", filtered.Card)
	}

	// The modifier beside the filter is neither refused nor a conflict: one
	// chooses which items the answer carries and the other how much of each.
	composed := showing(h, &Request{Card: card, Fields: "checklist.full", Unresolved: true})
	if len(composed.Checklist) != 4 {
		t.Errorf("--unresolved --fields checklist.full carried %d items", len(composed.Checklist))
	}
}

// TestAnItemsOwnReadKeepsEveryBody asserts that the two reads this change
// leaves alone still answer in full: an item's own reference answers every
// comment written on it, and a comment's own reference answers that comment.
//
// The byte lengths are asserted against what was written rather than the
// bodies being asserted non-empty, because a build that cleared Body in
// commentViews rather than in detailOf would still answer a comment view and
// would answer it hollow.
func TestAnItemsOwnReadKeepsEveryBody(t *testing.T) {
	h := newHarness(t)
	card := h.ready("A card carrying an argued question")
	h.item(card, "b00000000001", "kind: open_question\nstate: pending\nordinal: 1\n",
		"Which vendor do we cite?")
	first := "The contract names two, and the later one supersedes.\n"
	second := "## Ruling\n\nThe operator settled on the later one.\n"
	h.comment(card+"/questions/1", first)
	h.comment(card+"/questions/1", second)
	h.comment(card, "A remark on the card itself, which runs to some length.\n")

	_, _, item, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: card + "/questions/1"})
	if err != nil {
		t.Fatalf("show the item: %v", err)
	}
	if item == nil {
		t.Fatal("the item's own reference answered no item detail")
	}
	if len(item.Comments) != 2 {
		t.Fatalf("wanted the two comments the item holds, got %d", len(item.Comments))
	}
	for i, want := range []string{first, second} {
		got := item.Comments[i]
		if got.Body != want {
			t.Errorf("comment %d answers %q, wanted %q", i+1, got.Body, want)
		}
		if len(got.Body) != len(want) {
			t.Errorf("comment %d answers %d bytes, wanted %d", i+1, len(got.Body), len(want))
		}
		if got.Size != len(want) {
			t.Errorf("comment %d prices itself at %d bytes, wanted %d", i+1, got.Size, len(want))
		}
		if got.Ordinal != i+1 {
			t.Errorf("comment %d carries the ordinal %d", i+1, got.Ordinal)
		}
	}
	if got, want := item.Comments[1].Subject, "Ruling"; got != want {
		t.Errorf("the item's second comment reads %q as its subject, wanted %q", got, want)
	}

	// A comment's own reference is the recovery path, and it answers the
	// whole of the comment it names.
	_, _, _, text, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: card + "/comments/1"})
	if err != nil {
		t.Fatalf("show the comment: %v", err)
	}
	if !strings.Contains(text, "A remark on the card itself") {
		t.Errorf("a comment's own reference answered %q", text)
	}

	// The control: the same card's own read carries that comment as an
	// index, so the two assertions above are about the reference rather
	// than about the fixture.
	detail := showing(h, &Request{Card: card})
	if len(detail.Comments) != 1 {
		t.Fatalf("the card holds one comment of its own, got %d", len(detail.Comments))
	}
	if detail.Comments[0].Body != "" {
		t.Errorf("the card's own read carried a comment body: %q", detail.Comments[0].Body)
	}
}

// TestTheIndexedReadRewritesNothingOnDisk drives the two halves one run has
// to carry: the read answered an index, and the read left the files alone.
//
// A build that answers nothing fails the first half, and a build that
// rewrites an anchor fails the second. The number of files compared is
// reported, so a run that walked an empty tree says so rather than passing.
func TestTheIndexedReadRewritesNothingOnDisk(t *testing.T) {
	h := newHarness(t)
	card, _ := indexFixture(h)

	before := treeDigest(t, h.root)
	if len(before) == 0 {
		t.Fatal("the fixture wrote no file, so the comparison below reads nothing")
	}

	detail := showing(h, &Request{Card: card})
	if len(detail.Comments) != 3 || len(detail.Checklist) != 6 {
		t.Fatalf("the read answered %d comments and %d items, so it proves nothing about the files",
			len(detail.Comments), len(detail.Checklist))
	}
	for _, comment := range detail.Comments {
		if comment.Body != "" {
			t.Fatalf("the read served a body, so it is not the indexed read this asserts about: %+v", comment)
		}
	}

	after := treeDigest(t, h.root)
	if len(after) != len(before) {
		t.Errorf("the read changed the file count from %d to %d", len(before), len(after))
	}
	changed := 0
	for path, digest := range before {
		if after[path] != digest {
			changed++
			t.Errorf("the read rewrote %s", path)
		}
	}
	t.Logf("%d files compared, %d changed", len(before), changed)
}

// treeDigest reads every file under a root and returns its bytes by path, so
// two calls either side of an act say what the act wrote.
func treeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	digest := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest[path] = string(bytes)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return digest
}
