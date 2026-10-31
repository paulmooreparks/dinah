package sessionhost

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/applet/broker"
)

func confirmationFixture(t testing.TB) (*Continuations, *Conversation, *Transcript) {
	return confirmationFixtureSize(t, 2000)
}

func confirmationFixtureSize(t testing.TB, messages int) (*Continuations, *Conversation, *Transcript) {
	t.Helper()
	dir := t.TempDir()
	var mu, stateMu sync.Mutex
	runtime := &Runtime{}
	s := NewConversation(Identity{ID: "source", Provider: "old", Cwd: dir}, InitialActivity{Open: true}, false, false)
	runtime.PutConversationLocked(s.ID(), s)
	tr := &Transcript{SessionID: s.ID(), Provider: "old", Workspace: dir, Complete: true}
	for range messages {
		tr.Messages = append(tr.Messages, ConversationMessage{Role: "agent", Text: strings.Repeat("saved conversation ", 100)})
	}
	tr.Messages = append(tr.Messages, ConversationMessage{Role: "limitation", Text: "Images are not transferred."})
	if err := WriteTranscript(dir, tr); err != nil {
		t.Fatal(err)
	}
	c := NewContinuations(&mu, &stateMu, runtime, ContinuationHost{
		StateDir:          func() string { return dir },
		SupportedProvider: func(string) bool { return true }, NormalizeProvider: func(s string) string { return s },
		Session: func(id string) *Conversation {
			stateMu.Lock()
			defer stateMu.Unlock()
			return runtime.ConversationLocked(id)
		},
		Invocations: func(string, string) ([]broker.Invocation, error) { return nil, nil },
		HasPending:  func(string) bool { return false }, Budget: func(string, []acp.ConfigOption) int { return MinimumHandoffBudget },
		Formatter:   HandoffFormatter{ProductName: "Dinah", Speaker: func(m ConversationMessage) string { return m.Role }},
		BusIdentity: func(*Conversation) string { return "" }, Title: func(string) string { return "" },
		RevokeSession: func(string) {}, HideSession: func(string) error { return nil }, Emit: func(string, any) {},
	})
	c.host.NewSession = func(workspace, provider, adopt string) (Info, error) {
		next := NewConversation(Identity{ID: "next", Provider: provider, Cwd: workspace}, InitialActivity{Open: true}, false, false)
		mu.Lock()
		next.HistoryRestoreSavedLocked(&Transcript{SessionID: next.ID(), Provider: provider, Workspace: workspace, Complete: true})
		mu.Unlock()
		stateMu.Lock()
		runtime.PutConversationLocked(next.ID(), next)
		stateMu.Unlock()
		return next.Info(), nil
	}
	return c, s, tr
}

func BenchmarkConfirmationLongHistory(b *testing.B) {
	for _, cold := range []bool{false, true} {
		name := "warm-metadata"
		if cold {
			name = "cold-metadata"
		}
		b.Run(name, func(b *testing.B) {
			c, s, _ := confirmationFixtureSize(b, 20000)
			path := TranscriptPath(c.host.StateDir(), s.ID())
			stat, _ := os.Stat(path)
			if _, err := c.PreviewSession(s.ID(), "new"); err != nil {
				b.Fatal(err)
			}
			b.SetBytes(stat.Size())
			b.ResetTimer()
			for range b.N {
				if cold {
					confirmationFiles.Lock()
					delete(confirmationFiles.byPath, path)
					confirmationFiles.Unlock()
				}
				p, err := c.PreviewSession(s.ID(), "new")
				if err != nil || p.Facts.Handoff != nil || len(p.Warnings) != 1 {
					b.Fatalf("confirmation: %v", err)
				}
			}
		})
	}
}

func TestConfirmationDoesNotPrepareAndContinuationPreparesOnce(t *testing.T) {
	c, s, _ := confirmationFixture(t)
	var lines, digests atomic.Int32
	stageTimes := map[string]time.Time{}
	c.host.Emit = func(_ string, value any) {
		info := value.(Info)
		if info.ID == s.ID() && info.Switch != nil && info.Switch.Stage != "" {
			stageTimes[info.Switch.Stage] = time.Now()
		}
	}
	entered, release := make(chan struct{}), make(chan struct{})
	c.host.Formatter.Speaker = func(m ConversationMessage) string { lines.Add(1); return m.Role }
	c.host.Formatter.Digest = func(string, []string, int) (string, int, bool) {
		digests.Add(1)
		close(entered)
		<-release
		return "Earlier conversation summary", 1, true
	}
	start := time.Now()
	p, err := c.PreviewSession(s.ID(), "new")
	previewTime := time.Since(start)
	if err != nil || p.Revision == "" || p.Context != "" || len(p.History) != 0 || p.Facts.Handoff != nil || len(p.Warnings) != 1 || lines.Load() != 0 || digests.Load() != 0 {
		t.Fatalf("confirmation prepared content or lost warnings: %+v, %v", p, err)
	}
	b, _ := json.Marshal(p.Facts)
	if strings.Contains(string(b), `"handoff"`) {
		t.Fatal("confirmation advertised handoff counts")
	}
	done := make(chan error, 1)
	start = time.Now()
	go func() { _, err := c.ContinueSession(s.ID(), "new", p.Revision); done <- err }()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("preparation did not start")
	}
	if st := s.SwitchState(); st == nil || st.Stage != StageSummary {
		t.Fatalf("preparation lacks visible state: %+v", st)
	}
	if !c.mu.TryLock() {
		t.Fatal("summary holds the shared history lock")
	}
	c.mu.Unlock()
	if !c.stateMu.TryLock() {
		t.Fatal("summary holds the shared runtime lock")
	}
	c.stateMu.Unlock()
	if _, err := c.ContinueSession(s.ID(), "new", p.Revision); err == nil {
		t.Fatal("duplicate approval started another continuation")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if digests.Load() != 1 {
		t.Fatalf("summary generated %d times", digests.Load())
	}
	t.Logf("long history confirmation=%s, approved preparation/startup/validation=%s, judge calls=%d", previewTime, time.Since(start), digests.Load())
	t.Logf("stages preparation=%s, provider startup=%s, final validation/storage=%s", stageTimes[StageStarting].Sub(stageTimes[StageSummary]), stageTimes[StageHandover].Sub(stageTimes[StageStarting]), stageTimes[StageOpening].Sub(stageTimes[StageHandover]))
}

func TestConfirmationCacheDetectsExternalEditsAndCorruptHistory(t *testing.T) {
	c, s, _ := confirmationFixture(t)
	p, err := c.PreviewSession(s.ID(), "new")
	if err != nil {
		t.Fatal(err)
	}
	path := TranscriptPath(c.host.StateDir(), s.ID())
	data, _ := os.ReadFile(path)
	data = []byte(strings.Replace(string(data), "saved conversation", "other conversation", 1))
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	next, err := c.PreviewSession(s.ID(), "new")
	if err != nil || next.Revision == p.Revision {
		t.Fatalf("cached revision survived external content change: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"sessionId":"source", "messages":[`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PreviewSession(s.ID(), "new"); err == nil {
		t.Fatal("cached metadata hid corrupt history")
	}
}

func TestConfirmationRejectsSameSizeHistoryChangesBeforePreparation(t *testing.T) {
	c, s, tr := confirmationFixture(t)
	p, err := c.PreviewSession(s.ID(), "new")
	if err != nil {
		t.Fatal(err)
	}
	path := TranscriptPath(c.host.StateDir(), s.ID())
	before, _ := os.Stat(path)
	tr.Messages[0].Text = strings.Replace(tr.Messages[0].Text, "saved", "other", 1)
	if err := WriteTranscript(c.host.StateDir(), tr); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	c.host.Formatter.Digest = func(string, []string, int) (string, int, bool) {
		t.Error("stale confirmation invoked the judge")
		return "", 0, false
	}
	if _, err := c.ContinueSession(s.ID(), "new", p.Revision); err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("same-size edit accepted: %v", err)
	}
}

func TestConfirmationAuthorityFactsUseTheApprovedOptionsSnapshot(t *testing.T) {
	c, s, _ := confirmationFixture(t)
	s.UpdateOptions([]acp.ConfigOption{{ID: "permission", Name: "Permissions", Category: "mode", CurrentValue: json.RawMessage(`"safe"`)}})
	c.mu.Lock()
	p, err := c.confirmationLocked(s, "new", "")
	c.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	s.UpdateOptions([]acp.ConfigOption{{ID: "permission", Name: "Permissions", Category: "mode", CurrentValue: json.RawMessage(`"bypass"`)}})
	f := c.facts(s, "new", "", p, compactResult{}, 0)
	if len(f.NotCarried) != 1 || f.NotCarried[0].Value != "safe" {
		t.Fatalf("facts mixed settings revisions: %+v", f.NotCarried)
	}
	if _, err := c.ContinueSession(s.ID(), "new", p.Revision); err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("changed authority settings accepted: %v", err)
	}
}
