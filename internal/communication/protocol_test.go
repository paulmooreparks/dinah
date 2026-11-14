package communication

import (
	"strings"
	"testing"
)

// Every chat distinguishes person decisions from results and notices.
func TestChatsAreToldHowToAskForPersonDecisions(t *testing.T) {
	text := Window("Paul")
	for _, want := range []string{
		"For a decision only Paul can make, use the ask_person tool of the dinah-bus server with a recommendation for each question",
		"Put paul in actionRecipients for results and notices that need Paul's attention",
		"Writing a name or @ in the body routes nothing",
		"Address a group post to the member it is for",
		"A handoff, a request or a question always names its recipient",
		"never the whole group",
		"end your turn with one short line in your own chat that says so",
		"Never end a turn with no words at all",
		"mentions is a legacy alias and cannot be combined",
		"routine internal traffic needs no narration",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the window text lacks %q", want)
		}
	}
}
