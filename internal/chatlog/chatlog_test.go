package chatlog

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func up(s string) []byte { return []byte(s) }

func TestJoinsPiecesIntoBlocks(t *testing.T) {
	b := New(t.TempDir(), func(s string) string { return strings.ReplaceAll(s, "SECRET", "[redacted]") })
	b.Begin("s1", "devin", `C:\ws`, "Flow: Design Doc")
	b.Note("s1", "Flow run r1, step research.")
	b.Prompt("s1", "Design Doc: Research", "Research the plan. SECRET")
	b.Update("s1", up(`{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"I should search "}}`))
	b.Update("s1", up(`{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"the space first."}}`))
	b.Update("s1", up(`{"sessionUpdate":"tool_call","toolCallId":"t1","title":"Ran command","kind":"execute","status":"in_progress","rawInput":{"command":"git status"}}`))
	b.Update("s1", up(`{"sessionUpdate":"tool_call_update","toolCallId":"t1","status":"completed","content":[{"type":"content","content":{"type":"text","text":"{\"results\":[]}"}}]}`))
	b.Update("s1", up(`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Nothing "}}`))
	b.Update("s1", up(`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"was found."}}`))
	b.TurnEnd("s1", "end_turn", nil)
	b.Close("s1")
	raw, err := os.ReadFile(b.Path("s1"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{
		"# Chat s1", "- Agent: devin", "Flow run r1, step research.",
		"Prompt: Design Doc: Research", "Research the plan. [redacted]",
		"Thinking\n\nI should search the space first.",
		"Tool: Ran command (completed, execute)", "git status", `{"results":[]}`,
		"Agent\n\nNothing was found.", "The turn ended", "end_turn",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the log lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "SECRET") {
		t.Error("a secret reached the log")
	}
	if strings.Index(got, "Thinking") > strings.Index(got, "Tool: Ran command") || strings.Index(got, "Tool: Ran command") > strings.Index(got, "Nothing was found") {
		t.Errorf("the blocks are out of order:\n%s", got)
	}
}

func TestFailedTurnAndUnfinishedTool(t *testing.T) {
	b := New(t.TempDir(), nil)
	b.Prompt("s2", "", "go")
	b.Update("s2", up(`{"sessionUpdate":"tool_call","toolCallId":"t9","title":"Read file","status":"in_progress"}`))
	b.TurnEnd("s2", "", errors.New("the agent stopped"))
	b.CloseAll()
	raw, _ := os.ReadFile(b.Path("s2"))
	got := string(raw)
	if !strings.Contains(got, "Tool: Read file (in_progress)") || !strings.Contains(got, "The turn failed") {
		t.Fatalf("got:\n%s", got)
	}
}

func TestReopenedChatAppends(t *testing.T) {
	dir := t.TempDir()
	b := New(dir, nil)
	b.Begin("s3", "devin", "", "")
	b.Prompt("s3", "", "first")
	b.CloseAll()
	b = New(dir, nil)
	b.Begin("s3", "devin", "", "")
	b.Prompt("s3", "", "second")
	b.CloseAll()
	raw, _ := os.ReadFile(b.Path("s3"))
	got := string(raw)
	if strings.Count(got, "# Chat s3") != 1 || !strings.Contains(got, "Opened again") || !strings.Contains(got, "first") || !strings.Contains(got, "second") {
		t.Fatalf("got:\n%s", got)
	}
}

func TestToolTextIsFencedSafely(t *testing.T) {
	b := New(t.TempDir(), nil)
	b.Update("s4", up(`{"sessionUpdate":"tool_call","toolCallId":"t","title":"Read","status":"completed","content":[{"type":"content","content":{"type":"text","text":"a\n~~~~\nb"}}]}`))
	b.CloseAll()
	raw, _ := os.ReadFile(b.Path("s4"))
	if !strings.Contains(string(raw), "~~~~~text\n") {
		t.Fatalf("the fence can be closed by the text:\n%s", raw)
	}
}
