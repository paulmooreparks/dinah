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
		"Address a group post to the member it is for",
		"A handoff, a request or a question always names its recipient",
		"never the whole group",
		"if it does not, end your turn without replying",
		"mentions is a legacy alias and cannot be combined",
		"routine internal traffic needs no narration",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the window text lacks %q", want)
		}
	}
}
