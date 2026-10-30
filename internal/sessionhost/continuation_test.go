package sessionhost

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"studio/internal/acp"
	"studio/internal/applet/broker"
)

func TestContinuationRollbackKeepsEffectOrderAndSharedLocks(t *testing.T) {
	var mu, stateMu sync.Mutex
	var runtime Runtime
	dir := t.TempDir()
	source := NewConversation(Identity{ID: "source", Provider: "old", Cwd: dir}, InitialActivity{Open: true}, false, false)
	next := NewConversation(Identity{ID: "next", Provider: "new", Cwd: dir}, InitialActivity{Open: true}, false, false)
	tr := &Transcript{SessionID: source.ID(), Provider: "old", Workspace: dir, Complete: true, Messages: []ConversationMessage{{Role: "you", Text: "original"}}}
	if err := WriteTranscript(dir, tr); err != nil {
		t.Fatal(err)
	}
	runtime.PutConversationLocked(source.ID(), source)
	var effects []string
	var sourceEvents []*SwitchState
	unlocked := func() {
		t.Helper()
		for _, lock := range []*sync.Mutex{&mu, &stateMu} {
			if !lock.TryLock() {
				t.Fatal("host callback held an outer lock")
			}
			lock.Unlock()
		}
	}
	c := NewContinuations(&mu, &stateMu, &runtime, ContinuationHost{
		StateDir: func() string {
			if mu.TryLock() {
				mu.Unlock()
				t.Fatal("file access outside transcript mutex")
			}
			return dir
		},
		SupportedProvider: func(string) bool { return true }, NormalizeProvider: func(s string) string { return s },
		Session: func(id string) *Conversation {
			stateMu.Lock()
			defer stateMu.Unlock()
			return runtime.ConversationLocked(id)
		},
		Invocations: func(string, string) ([]broker.Invocation, error) { return nil, nil }, HasPending: func(string) bool { return false },
		Budget:      func(string, []acp.ConfigOption) int { return MinimumHandoffBudget },
		Formatter:   HandoffFormatter{ProductName: "Host", Speaker: func(m ConversationMessage) string { return m.Role }},
		BusIdentity: func(*Conversation) string { return "" },
		NewSession: func(string, string, string) (Info, error) {
			unlocked()
			stateMu.Lock()
			runtime.PutConversationLocked(next.ID(), next)
			stateMu.Unlock()
			mu.Lock()
			tr.Messages[0].Text = "changed while connecting"
			err := WriteTranscript(dir, tr)
			mu.Unlock()
			return next.Info(), err
		},
		HideSession: func(id string) error {
			unlocked()
			stateMu.Lock()
			found := runtime.ConversationLocked(id)
			stateMu.Unlock()
			if found != nil {
				t.Fatal("hide preceded registry removal")
			}
			effects = append(effects, "hide")
			return errors.New("ignored settings failure")
		},
		RevokeSession: func(id string) {
			unlocked()
			effects = append(effects, "revoke")
			if !next.Info().Open {
				t.Fatal("close preceded revocation")
			}
		},
		Emit: func(event string, value any) {
			unlocked()
			// The source chat's own events are its stages and its failure, which say
			// where the switch is and why it stopped.
			if info := value.(Info); info.ID == source.ID() {
				sourceEvents = append(sourceEvents, info.Switch)
				return
			}
			effects = append(effects, event)
			if value.(Info).Open {
				t.Fatal("event preceded close")
			}
		},
	})
	if c.mu != &mu || c.stateMu != &stateMu || c.runtime != &runtime {
		t.Fatal("owner copied synchronization or registry")
	}
	preview, err := c.PreviewSession(source.ID(), "new")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.ContinueSession(source.ID(), "new", preview.Revision)
	if err == nil || !strings.Contains(err.Error(), "changed while connecting") {
		t.Fatalf("revision result %v", err)
	}
	if !reflect.DeepEqual(effects, []string{"hide", "revoke", "agent:session"}) {
		t.Fatalf("rollback order %v", effects)
	}
	if source.State().Switching || !source.Info().Open || runtime.ConversationLocked(source.ID()) != source {
		t.Fatal("rollback damaged source or retained switching flag")
	}
	// The source chat's state went through the stages up to the one that failed, and
	// ends with the failure and no stage.
	var stages []string
	for _, st := range sourceEvents {
		if st != nil && st.Stage != "" {
			stages = append(stages, st.Stage)
		}
	}
	last := sourceEvents[len(sourceEvents)-1]
	if !reflect.DeepEqual(stages, []string{"summary", "starting", "handover"}) || last == nil || last.Stage != "" || !strings.Contains(last.Failed, "changed while connecting") || source.SwitchState() == nil {
		t.Fatalf("stages %v, last %+v", stages, last)
	}
}

func TestRecoveryUsesProcessMutexAndReleasesOnReadFailure(t *testing.T) {
	for i := 0; i < 2; i++ {
		var mu, stateMu sync.Mutex
		var runtime Runtime
		dir := t.TempDir()
		checked := false
		c := NewContinuations(&mu, &stateMu, &runtime, ContinuationHost{StateDir: func() string {
			if sessionRecoveryMu.TryLock() {
				sessionRecoveryMu.Unlock()
				t.Fatal("recovery did not hold process mutex")
			}
			checked = true
			return dir
		}})
		if _, err := c.Recover("missing", "provider", "revision"); err == nil || !checked {
			t.Fatal("missing saved history was not read and refused")
		}
		if !sessionRecoveryMu.TryLock() {
			t.Fatal("failed recovery retained process mutex")
		}
		sessionRecoveryMu.Unlock()
	}
}
