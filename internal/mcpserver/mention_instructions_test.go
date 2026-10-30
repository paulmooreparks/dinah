package mcpserver

import (
	"reflect"
	"strings"
	"testing"
)

// The bus server's own rules and the field description of send tell a chat to
// notify the person by mentioning them, that their name in the text notifies
// nobody, and when not to; and no longer describe the mention as only an inbox.
func TestTheBusServerTellsChatsHowToReachThePerson(t *testing.T) {
	text := rules("Paul")
	for _, want := range []string{
		"to notify Paul: put paul in the mentions when you need a decision or an answer from Paul",
		"notifies nobody",
		"Do not mention paul to say thanks",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the rules lack %q", want)
		}
	}
	field, _ := reflect.TypeOf(sendIn{}).FieldByName("Mentions")
	description := field.Tag.Get("jsonschema")
	if !strings.Contains(description, "to notify the person and put the message in their inbox") || strings.Contains(description, "to put the message in the person's inbox") {
		t.Errorf("the mentions description: %q", description)
	}
}
