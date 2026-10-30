package sessionhost

import (
	"encoding/json"
	"studio/internal/acp"
	"sync"
	"testing"
)

func TestDetailsOptionsAndReplayCopies(t *testing.T) {
	var mu sync.Mutex
	d := NewDetails(&mu)
	mu.Lock()
	defer mu.Unlock()
	if d.OptionsLocked() != nil {
		t.Fatal("nil options became empty")
	}
	d.UpdateOptionsLocked([]acp.ConfigOption{})
	if d.OptionsLocked() == nil {
		t.Fatal("empty options became nil")
	}
	original := json.RawMessage(`"old"`)
	replacement := json.RawMessage(`"new"`)
	d.SetOverridesLocked(map[string]OptionOverride{"model": {From: original, To: replacement}})
	original[1], replacement[1] = 'X', 'X'
	opts := []acp.ConfigOption{{ID: "model", CurrentValue: json.RawMessage(`"new"`), Options: json.RawMessage(`["new"]`)}}
	d.SetRestoringLocked(true)
	restoring, kept := d.UpdateOptionsLocked(opts)
	if !restoring || string(kept["model"]) != `"old"` {
		t.Fatalf("override: %v %s", restoring, kept["model"])
	}
	opts[0].CurrentValue[1], opts[0].Options[2], kept["model"][1] = 'X', 'X', 'X'
	view := d.OptionsLocked()
	if string(view[0].CurrentValue) != `"new"` || string(view[0].Options) != `["new"]` {
		t.Fatal("input aliases owned options")
	}
	view[0].CurrentValue[1], view[0].Options[2] = 'X', 'X'
	_, kept = d.UpdateOptionsLocked(d.OptionsLocked())
	if string(kept["model"]) != `"old"` {
		t.Fatal("returned override aliases owner")
	}
	if string(d.OptionsLocked()[0].CurrentValue) != `"new"` {
		t.Fatal("view aliases owner")
	}
	d.UpdateOptionsLocked([]acp.ConfigOption{{ID: "model", CurrentValue: json.RawMessage(`"own"`)}})
	_, kept = d.UpdateOptionsLocked([]acp.ConfigOption{{ID: "model", CurrentValue: json.RawMessage(`"new"`)}})
	if len(kept) != 0 {
		t.Fatal("changed choice retained override")
	}
	raw := json.RawMessage(`{"value":1}`)
	d.SetReplayingLocked(true)
	d.BeginHeldReplayLocked(7)
	if !d.HoldReplayLocked(raw) {
		t.Fatal("replay not held")
	}
	raw[9] = '2'
	held, start := d.DrainHeldReplayLocked()
	if start != 7 || string(held[0]) != `{"value":1}` || d.HoldingReplayLocked() {
		t.Fatal("held replay state or copy changed")
	}
	// The queue owns what it is given; numbering copies the updates first
	// (TestReplayNumbersCopyTheirUpdates).
	if !d.QueueReplayLocked([]NumberedUpdate{{N: 1, Update: held[0]}}, 1) {
		t.Fatal("batch boundary lost")
	}
	batch := d.DrainReplayLocked()
	if string(batch[0].Update) != `{"value":1}` || batch[0].N != 1 || d.DrainReplayLocked() != nil {
		t.Fatal("queue lost the update or did not drain")
	}
	d.EndReplayLocked()
	if d.HoldReplayLocked(raw) || !d.UpdateKindLocked("agent_message_chunk") || d.UpdateKindLocked("agent_message_chunk") {
		t.Fatal("replay/journal transition changed")
	}
}

func TestDetailsSubagentViewsAreIndependent(t *testing.T) {
	var mu sync.Mutex
	d := NewDetails(&mu)
	mu.Lock()
	defer mu.Unlock()
	d.SpawnSubagentLocked("root", "root", "child", "name", "task", "prompt", true, false, "", "")
	raw := json.RawMessage(`{"sessionUpdate":"tool_call","title":"tool"}`)
	d.KeepSubagentUpdateLocked("child", "tool_call", raw)
	raw[0] = 'X'
	rec, found := d.SubagentRecordLocked("child")
	if !found || !json.Valid(rec.Updates[0]) {
		t.Fatal("input aliases subagent buffer")
	}
	rec.Updates[0][0] = 'X'
	rec.Subagent.Name = "changed"
	family := d.SubagentFamilyLocked("child")
	delete(family, "child")
	view := d.SubagentListLocked()
	view[0].Name = "changed"
	again, _ := d.SubagentRecordLocked("child")
	if !json.Valid(again.Updates[0]) || again.Subagent.Name != "name" || !d.SubagentFamilyLocked("child")["child"] {
		t.Fatal("returned subagent data aliases owner")
	}
}

func TestHistoryCopiesNestedMessages(t *testing.T) {
	// History uses the caller's transcript lock, independently of the session lock.
	var cmu sync.Mutex
	cmu.Lock()
	defer cmu.Unlock()
	message := ConversationMessage{Role: "you", Images: []acp.ContentBlock{{Type: "resource", Resource: &acp.EmbeddedResource{Text: "image"}}}, Delivered: &DeliveredMessage{Prompt: "prompt"}}
	input := &Transcript{SessionID: "chat", Messages: []ConversationMessage{message}, ParentHistory: []ConversationMessage{message}}
	var h History
	h.RestoreSavedLocked(input)
	input.Messages[0].Images[0].Resource.Text = "changed"
	input.ParentHistory[0].Delivered.Prompt = "changed"
	for i := 0; i < 2; i++ {
		view := h.SavedLocked()
		for _, messages := range [][]ConversationMessage{view.Messages, view.ParentHistory} {
			if messages[0].Images[0].Resource.Text != "image" || messages[0].Delivered.Prompt != "prompt" {
				t.Fatal("nested transcript data aliases caller")
			}
			messages[0].Images[0].Resource.Text = "changed"
			messages[0].Delivered.Prompt = "changed"
		}
	}
	h.RestoreSavedLocked(&Transcript{Messages: []ConversationMessage{}})
	if h.SavedLocked().Messages == nil || h.SavedLocked().ParentHistory != nil {
		t.Fatal("nil and empty transcript fields changed")
	}
}

func TestHistoryReplayWithoutSavedCopySharesTombstone(t *testing.T) {
	dir := t.TempDir()
	var h History
	h.InitializeLocked(dir, "chat", "codex", dir, true)
	h.RecordMessageLocked(true, "chat", "you", "before", false, false)
	h.ReplaceDeletedLocked(&Transcript{SessionID: "chat", Provider: "codex", Deleted: true})
	h.RecordMessageLocked(true, "chat", "you", "after", false, false)
	h.FinishReplayLocked(dir, true)
	tr, err := ReadTranscript(dir, "chat")
	if err != nil || !tr.Deleted || len(tr.Messages) != 0 {
		t.Fatalf("shared replay tombstone changed: %+v %v", tr, err)
	}
}
