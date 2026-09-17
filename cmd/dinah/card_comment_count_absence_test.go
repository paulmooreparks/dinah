package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"dinah/internal/verb"
)

// dinah-518 section 6.2 rules that a card view publishes no comment count. A
// card already publishes attachment_count and checklist_count beside it, so
// the member that would sit there is one line of struct and one line of fill
// away, and nothing in the tool would refuse it. Agent Code Review round three
// wrote exactly that build, found it compiled, and found the whole suite green
// over it, so the ruling was a sentence in a specification and nothing else.
//
// This file is the assertion that was missing. It reads the card's own JSON on
// every surface that publishes one, and it reads the type behind those
// surfaces, because the two catch different builds. A member filled from a
// count is invisible on a card holding no comments, since the tag carries
// omitempty, and a member minted and never filled is invisible on every card
// there is; the fixture below carries comments so the first build is seen, and
// the type assertion sees the second.
//
// It asserts the absence of a member rather than the presence of one, so the
// count of card objects it read is asserted too. A surface that stopped
// emitting a card object at all would otherwise pass this for free.

// cardCommentCountMember is the JSON member no card view may carry.
const cardCommentCountMember = "comment_count"

// TestNoCardViewPublishesACommentCount asserts dinah-518's card half at the
// command surface.
func TestNoCardViewPublishesACommentCount(t *testing.T) {
	root := newBench(t)
	mustRun(t, root, "add", "a card carrying comments")
	mustRun(t, root, "comment", "fx-1", "a first remark")
	mustRun(t, root, "comment", "fx-1", "a second remark")

	surfaces := [][]string{
		{"status", "--json"},
		// `ls` retired into `list cards` on dinah-523, which answers the
		// same card views under the same envelope.
		{"list", "cards", "--json"},
		{"show", "fx-1", "--json"},
	}
	total := 0
	for _, argv := range surfaces {
		got := runCLI(t, root, argv...)
		if got.code != 0 {
			t.Fatalf("%s: %d %s", strings.Join(argv, " "), got.code, got.errw)
		}
		var decoded any
		if err := json.Unmarshal([]byte(got.out), &decoded); err != nil {
			t.Fatalf("%s: decode the payload: %v\n%s", strings.Join(argv, " "), err, got.out)
		}
		objects := objectsCarryingRef(decoded, "fx-1")
		for _, object := range objects {
			if _, carried := object[cardCommentCountMember]; carried {
				t.Errorf("dinah %s publishes %s on the card fx-1, and section 6.2 rules that member out", strings.Join(argv, " "), cardCommentCountMember)
			}
		}
		total += len(objects)
		t.Logf("dinah %s carried %d object(s) for the card fx-1", strings.Join(argv, " "), len(objects))
	}
	// dinah status publishes column views rather than card views, so two of
	// the three surfaces are expected to answer and the third to answer
	// nothing. What is asserted is that the two did answer, because a read
	// of no card object reports success exactly as a clean one does.
	if total < 2 {
		t.Fatalf("the three surfaces carried %d objects for the card fx-1 between them, so this check read almost nothing", total)
	}
}

// TestTheCardViewTypeDeclaresNoCommentCount reads the type the three surfaces
// encode, which catches the member a fixture cannot: one minted and left
// unfilled is omitted from every payload by its own omitempty tag.
func TestTheCardViewTypeDeclaresNoCommentCount(t *testing.T) {
	view := reflect.TypeOf(verb.CardView{})
	countingMembers := 0
	for i := 0; i < view.NumField(); i++ {
		name := jsonMemberName(view.Field(i).Tag.Get("json"))
		if name == cardCommentCountMember {
			t.Errorf("verb.CardView declares the field %s, which encodes as %s, and section 6.2 rules that member out of a card view", view.Field(i).Name, cardCommentCountMember)
		}
		if strings.HasSuffix(name, "_count") {
			countingMembers++
		}
	}
	// The arming of the reading itself. A tag spelling this walk could not
	// read would report no comment count on a type that declared one, so the
	// walk is held to finding the counting members the card does publish.
	if countingMembers < 2 {
		t.Fatalf("verb.CardView publishes %d members ending in _count, and it carries attachment_count and checklist_count at least, so this walk is not reading the tags", countingMembers)
	}
}

// jsonMemberName returns the member name a struct tag encodes as, dropping the
// options that follow it.
func jsonMemberName(tag string) string {
	name, _, _ := strings.Cut(tag, ",")
	return name
}

// objectsCarryingRef returns every JSON object below the decoded payload whose
// ref member is the given reference, which is how a card view identifies
// itself on each of the three surfaces.
func objectsCarryingRef(node any, ref string) []map[string]any {
	var found []map[string]any
	switch value := node.(type) {
	case map[string]any:
		if carried, ok := value["ref"].(string); ok && carried == ref {
			found = append(found, value)
		}
		for _, member := range value {
			found = append(found, objectsCarryingRef(member, ref)...)
		}
	case []any:
		for _, member := range value {
			found = append(found, objectsCarryingRef(member, ref)...)
		}
	}
	return found
}
