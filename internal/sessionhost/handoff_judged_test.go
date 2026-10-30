package sessionhost

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"studio/internal/acp"
	"studio/internal/applet/broker"
)

func longHistory(n int) []ConversationMessage {
	var h []ConversationMessage
	for i := 0; i < n; i++ {
		h = append(h, ConversationMessage{Role: "agent", Text: fmt.Sprintf("%d %s", i, strings.Repeat("x", 200))})
	}
	return h
}

func judgedFormatter(calls *[]int, answer string, shownAll bool) HandoffFormatter {
	return HandoffFormatter{ProductName: "Host", Speaker: func(m ConversationMessage) string { return m.Role },
		Digest: func(chat string, lines []string, limit int) (string, int, bool) {
			*calls = append(*calls, limit)
			shown := len(lines)
			if !shownAll {
				shown = len(lines) / 2
			}
			return answer, shown, answer != ""
		}}
}

// A judge's summary stands in for the digest of the older messages and takes no more
// room than the digest would, a quarter of the budget, so the text stays within the
// budget and the newest messages are still whole. It is given one line for each
// older message and the room it has.
func TestAJudgedSummaryReplacesTheDigestWithinTheSameRoom(t *testing.T) {
	history := longHistory(200)
	var limits []int
	f := judgedFormatter(&limits, "The person asked for an exporter and the agent built it.", true)
	budget := MinimumHandoffBudget
	rule, ruleDropped := f.Compact(history, budget)
	got, dropped := f.CompactJudged("chat", history, budget)
	if dropped != ruleDropped || dropped == 0 || len(limits) != 1 || limits[0] <= 0 || limits[0] >= budget/4 {
		t.Fatalf("dropped %d (rule %d), limits %v", dropped, ruleDropped, limits)
	}
	if !strings.Contains(got, "What came before, summarised:\nThe person asked for an exporter and the agent built it.\n\nThe most recent messages follow in full.") ||
		strings.Contains(got, "one line for each message") || !strings.Contains(got, "[agent]\n199 ") || len(got) > budget {
		t.Fatalf("the judged handoff: %q", got[:min(len(got), 600)])
	}
	if strings.Contains(got, "more earlier message(s)") {
		t.Fatal("a count was added though the judge covered every line")
	}
	if len(got) > len(rule)+budget/4 {
		t.Fatal("the judged handoff is larger than the rule's by more than the room it is given")
	}
	// Compact never asks the judge.
	limits = nil
	f.Compact(history, budget)
	if len(limits) != 0 {
		t.Fatal("Compact asked the judge")
	}
}

// When the judge covered only some of the lines, the rest are counted as the rule
// counts them.
func TestTheLinesAJudgeWasNotShownAreCounted(t *testing.T) {
	var limits []int
	f := judgedFormatter(&limits, "A summary of the first half.", false)
	got, dropped := f.CompactJudged("chat", longHistory(200), MinimumHandoffBudget)
	want := fmt.Sprintf("- and %d more earlier message(s), kept in Host\n", dropped-dropped/2)
	if !strings.Contains(got, want) {
		t.Fatalf("the count %q is missing from %q", want, got[:min(len(got), 400)])
	}
}

// With no summary, one that is too long for its room, one that covers no line, or
// a history that fits, the text is the rule's, byte for byte, and a history that
// fits does not ask the judge.
func TestWithoutAUsableSummaryTheHandoffIsTheRules(t *testing.T) {
	history := longHistory(200)
	for name, answer := range map[string]string{"none": "", "too long": strings.Repeat("y", MinimumHandoffBudget)} {
		var limits []int
		f := judgedFormatter(&limits, answer, true)
		rule, _ := f.Compact(history, MinimumHandoffBudget)
		if got, _ := f.CompactJudged("chat", history, MinimumHandoffBudget); got != rule {
			t.Errorf("%s: the judged handoff differs from the rule's", name)
		}
	}
	zero := HandoffFormatter{ProductName: "Host", Speaker: func(m ConversationMessage) string { return m.Role },
		Digest: func(string, []string, int) (string, int, bool) { return "A summary", 0, true }}
	rule, _ := zero.Compact(history, MinimumHandoffBudget)
	if got, _ := zero.CompactJudged("chat", history, MinimumHandoffBudget); got != rule {
		t.Error("a summary that covers no line was used")
	}
	var limits []int
	f := judgedFormatter(&limits, "A summary", true)
	short := longHistory(3)
	rule, dropped := f.Compact(short, MinimumHandoffBudget)
	if got, d := f.CompactJudged("chat", short, MinimumHandoffBudget); got != rule || d != 0 || dropped != 0 || len(limits) != 0 {
		t.Errorf("a history that fits: %d calls", len(limits))
	}
}

// The preview of a continuation asks the judge only after the transcript mutex and
// the state mutex are released, since it may take seconds, and what it shows is the
// judged text.
func TestThePreviewAsksTheJudgeWithNoLockHeld(t *testing.T) {
	var mu, stateMu sync.Mutex
	var runtime Runtime
	dir := t.TempDir()
	source := NewConversation(Identity{ID: "source", Provider: "old", Cwd: dir}, InitialActivity{Open: true}, false, false)
	tr := &Transcript{SessionID: source.ID(), Provider: "old", Workspace: dir, Complete: true, Messages: longHistory(200)}
	if err := WriteTranscript(dir, tr); err != nil {
		t.Fatal(err)
	}
	runtime.PutConversationLocked(source.ID(), source)
	asked := 0
	f := HandoffFormatter{ProductName: "Host", Speaker: func(m ConversationMessage) string { return m.Role },
		Digest: func(chat string, lines []string, limit int) (string, int, bool) {
			asked++
			for _, lock := range []*sync.Mutex{&mu, &stateMu} {
				if !lock.TryLock() {
					t.Fatal("the judge was asked with a lock held")
				}
				lock.Unlock()
			}
			if chat != "source" {
				t.Fatalf("the judge was told the chat is %q", chat)
			}
			return "Judged summary of the older messages.", len(lines), true
		}}
	c := NewContinuations(&mu, &stateMu, &runtime, ContinuationHost{
		StateDir:          func() string { return dir },
		SupportedProvider: func(string) bool { return true }, NormalizeProvider: func(s string) string { return s },
		Session: func(id string) *Conversation {
			stateMu.Lock()
			defer stateMu.Unlock()
			return runtime.ConversationLocked(id)
		},
		Invocations: func(string, string) ([]broker.Invocation, error) { return nil, nil }, HasPending: func(string) bool { return false },
		Budget:      func(string, []acp.ConfigOption) int { return MinimumHandoffBudget },
		Formatter:   f,
		BusIdentity: func(*Conversation) string { return "" },
	})
	preview, err := c.PreviewSession(source.ID(), "new")
	if err != nil {
		t.Fatal(err)
	}
	if asked != 1 || !strings.Contains(preview.Context, "Judged summary of the older messages.") || len(preview.Context) > MinimumHandoffBudget {
		t.Fatalf("asked %d times; context %q", asked, preview.Context[:min(len(preview.Context), 300)])
	}
}
