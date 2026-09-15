package mcp

import (
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// TestTheCommentToolsCardDescriptionNamesTheItemToo asserts dinah-502 AC-9's
// schema half. The comment tool's input schema still declares a required
// property named card and declares no new property, and the rendered
// description is the assertion that matters: it names both a card and a
// checklist item, and it no longer carries the shared sentence every other
// plain-card command's schema still does.
//
// Pinning the rendered string rather than the property name is deliberate:
// this criterion fails against a schema whose property is still called card
// and whose description still says the parameter takes a card alone, which a
// check on the property's name would pass.
func TestTheCommentToolsCardDescriptionNamesTheItemToo(t *testing.T) {
	properties := schemaProperties(t, "comment")
	property, ok := properties["card"].(map[string]any)
	if !ok {
		t.Fatalf("the comment tool carries no card property: %v", properties)
	}
	if _, textDeclared := properties["text"]; !textDeclared {
		t.Errorf("the comment tool declares no text property: %v", properties)
	}
	// The parameter table's own two entries, card and text, are what this
	// card touches; the rest are the transport's own injected properties and
	// carry no new parameter either.
	injected := map[string]bool{
		"actor": true, "workbench": true,
		"harness": true, "provider": true, "model": true, "server": true,
	}
	for name := range properties {
		if name != "card" && name != "text" && !injected[name] {
			t.Errorf("the comment tool declares a property this card did not expect: %s", name)
		}
	}

	required, ok := schemaFor(toolNamed(t, "comment"))["required"].([]string)
	if !ok {
		t.Fatalf("the comment tool's schema carries no required list")
	}
	requiredCard := false
	for _, name := range required {
		if name == "card" {
			requiredCard = true
		}
	}
	if !requiredCard {
		t.Errorf("the comment tool's schema does not require card: %v", required)
	}

	description, ok := property["description"].(string)
	if !ok {
		t.Fatalf("the card property carries no description string: %v", property)
	}
	if !strings.Contains(description, "card") {
		t.Errorf("the rendered description does not name a card: %q", description)
	}
	if !strings.Contains(description, "checklist item") {
		t.Errorf("the rendered description does not name a checklist item: %q", description)
	}
	const sharedCardSentence = "the card you are acting on"
	if strings.Contains(description, sharedCardSentence) {
		t.Errorf("the rendered description still carries the shared card-only sentence %q: %q", sharedCardSentence, description)
	}
}

// TestTheCommentToolWritesAnItemComment asserts dinah-502 AC-9's behavioural
// half: a call passing an item reference in the card property writes the
// comment on the item rather than being refused.
func TestTheCommentToolWritesAnItemComment(t *testing.T) {
	library := newLibrary(t)
	filed := library.File(&verb.Request{Verb: "file", Actor: "alka", Card: "fx-1", Kind: "open_question", Text: "does the deadline move?"})
	if filed.Outcome != contract.OutcomeOK {
		t.Fatalf("file: %s %s", filed.Outcome, filed.Refusal)
	}
	response := library.Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: "fx-1/questions/1", Text: "reasoning"})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment on the item: %s %s", response.Outcome, response.Refusal)
	}
}
