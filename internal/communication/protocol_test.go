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
		"Put paul in actionRecipients when Paul needs to decide or answer",
		"Writing a name or @ in the body routes nothing",
		"Informational posts never wake, reopen or steer a chat",
		"mentions is a legacy alias and cannot be combined",
		"routine internal traffic needs no narration",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the window text lacks %q", want)
		}
	}
}
