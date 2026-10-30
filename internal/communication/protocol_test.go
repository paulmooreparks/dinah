package communication

import (
	"strings"
	"testing"
)

// Every chat is told to mention the person when it needs them, that their
// name in the text does not count, and when not to.
func TestChatsAreToldToMentionThePersonWhenTheyNeedThem(t *testing.T) {
	text := Window("Paul")
	for _, want := range []string{
		"put paul in the send or reply tool's mentions",
		"which notifies Paul",
		"writing Paul's name or an @ and their name in the text mentions nobody",
		"when you need a decision or an answer from them",
		"Do not mention Paul to say thanks, to report progress that needs nothing from them",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the window text lacks %q", want)
		}
	}
}
