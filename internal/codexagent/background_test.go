package codexagent

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"studio/internal/acp"
)

func TestBackgroundSnapshotCountsDescendantsAndTerminalsWithoutReadingOutput(t *testing.T) {
	h := &testHandler{}
	var phase atomic.Int32
	c := fake(t, h, func(m message, send func(message)) {
		step := phase.Load()
		var p struct{ AncestorThreadID, ThreadID, Cursor string }
		if err := json.Unmarshal(m.Params, &p); err != nil {
			t.Error(err)
		}
		switch m.Method {
		case "thread/list":
			if p.AncestorThreadID != "root" {
				t.Errorf("unscoped descendant read: %s", m.Params)
			}
			if step == 2 {
				send(message{ID: m.ID, Error: &acp.Error{Code: -32601, Message: "unsupported"}})
				return
			}
			if step == 3 {
				send(answer(m, map[string]any{}))
				return
			}
			data := []any{}
			if step == 0 || step == 1 || step == 4 || step == 5 {
				parent := "root"
				if step == 4 {
					parent = "foreign-root"
				}
				status := "active"
				if step == 1 {
					status = "idle"
				}
				if step == 5 {
					status = "notLoaded"
				}
				data = append(data, map[string]any{"id": "child", "parentThreadId": parent, "status": map[string]string{"type": status}})
			}
			send(answer(m, map[string]any{"data": data, "nextCursor": nil}))
		case "thread/backgroundTerminals/list":
			data := []any{}
			next := ""
			if step == 0 && p.ThreadID == "child" && p.Cursor == "" {
				data = append(data, map[string]any{"processId": "42", "command": "invented command"})
				next = "more"
			}
			send(answer(m, map[string]any{"data": data, "nextCursor": next}))
		default:
			t.Errorf("unexpected observation request %s", m.Method)
			send(answer(m, map[string]any{}))
		}
	})
	c.sessions["codex:root"] = &session{}
	refresh := func(wantEvents int, wantError bool) {
		t.Helper()
		err := c.RefreshBackgroundWork(context.Background(), "codex:root")
		if (err != nil) != wantError {
			t.Fatalf("refresh error = %v", err)
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if len(h.events) != wantEvents {
			t.Fatalf("events = %d, want %d: %s", len(h.events), wantEvents, h.events)
		}
	}
	refresh(2, false)
	refresh(2, false) // An unchanged observation produces no duplicate tasks.
	phase.Store(2)
	refresh(2, true) // A failed read cannot clear either running record.
	phase.Store(3)
	refresh(2, true) // Missing data is not an empty authoritative snapshot.
	phase.Store(4)
	refresh(2, true) // A server ignoring the ancestor filter cannot lend foreign work.
	phase.Store(5)
	refresh(2, false) // An unloaded child cannot certify its previous work stopped.
	phase.Store(6)
	refresh(2, false) // Removing a thread from the catalog is not a completion event.
	phase.Store(1)
	refresh(4, false)
	for _, raw := range h.events[2:] {
		var event struct {
			Update struct{ SessionUpdate, State string }
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatal(err)
		}
		if event.Update.SessionUpdate != "async_task_state_update" || event.Update.State != "completed" {
			t.Fatalf("end = %s", raw)
		}
	}
}

func TestBackgroundObservationRequiresHostAdmissionOfEachLoadedThread(t *testing.T) {
	c := newConn(nil)
	c.sessions["codex:replaying"] = &session{}
	c.sessions["codex:ready"] = &session{}
	c.sessions["codex:temporary"] = &session{ephemeral: true}
	if c.enableBackgroundObservation("codex:unknown") || c.enableBackgroundObservation("codex:temporary") {
		t.Fatal("unknown or ephemeral thread admitted")
	}
	if !c.enableBackgroundObservation("codex:ready") {
		t.Fatal("loaded ready thread not admitted")
	}
	if c.sessions["codex:replaying"].observeBackground || !c.sessions["codex:ready"].observeBackground {
		t.Fatal("one admitted chat enabled observation of another loading chat")
	}
}
