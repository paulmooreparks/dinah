package mcpserver

import (
	"reflect"
	"strings"
	"testing"
)

// The bus server separates person decisions from notices and explains routing.
func TestTheBusServerTellsChatsHowToReachThePerson(t *testing.T) {
	text := rules("Paul")
	for _, want := range []string{
		"For a decision only Paul can make, use ask_person with a recommendation for each question",
		"Put paul in actionRecipients for results and notices that need Paul's attention",
		"routes nothing",
		"Do not notify Paul for thanks",
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
