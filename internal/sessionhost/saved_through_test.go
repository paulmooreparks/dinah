package sessionhost

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
)

// throughRig is a live conversation with its saved copy on disk, a capture
// that shares one mutex with whoever reads that copy, and the events the chat
// emitted, as the turn loop and the update handler produce them.
type throughRig struct {
	t       *testing.T
	dir     string
	mu      sync.Mutex
	capture *TranscriptCapture
	s       *Conversation
}

func newThroughRig(t *testing.T) *throughRig {
	r := &throughRig{t: t, dir: t.TempDir()}
	r.capture = NewTranscriptCapture(&r.mu, TranscriptCaptureHost{
		StateDir:    func() string { return r.dir },
		VisibleText: func(s string) string { return s },
		RedactText:  func(s string) string { return s },
		Log:         func(string, string, string, ...any) {},
		TurnEnd:     func(string, string, error) {},
	})
	r.s = NewConversation(Identity{ID: "live", Provider: "codex", Cwd: r.dir}, InitialActivity{Open: true}, false, false)
	r.capture.Initialize(r.s, false)
	return r
}

// view is what Saved reads for a live chat: the saved messages and the log
// number they are consistent with, taken under the one mutex.
func (r *throughRig) view() ([]ConversationMessage, uint64, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tr, err := ReadTranscript(r.dir, "live")
	if err != nil {
		r.t.Fatal(err)
	}
	_, through, ok := r.s.HistoryThroughLocked()
	return tr.Messages, through, ok
}

// update is a tool call as the agent sends it, with a title that is its name.
func toolUpdate(title string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"sessionUpdate": "tool_call", "title": title, "status": "completed"})
	return b
}

func (r *throughRig) entries(after uint64) []ChatEvent {
	return r.s.ChatEvents(r.s.log.gen, after).Events
}

// Whatever the saved copy holds, the log's entries up to the number it records
// are exactly the events behind it, and the entries after that number are
// exactly the events it lacks: a prompt, each update of a turn and the turn's
// end are each in one or the other, never both and never neither. A reader
// takes the saved view over and over while a turn is recorded and logged by
// two goroutines, as the turn loop and the update handler do.
func TestTheSavedCopyAndTheLogNumberAreExactAtEveryMoment(t *testing.T) {
	r := newThroughRig(t)
	const turns, perTurn = 6, 40
	done := make(chan struct{})
	var violations []string
	var vmu sync.Mutex
	report := func(format string, args ...any) {
		vmu.Lock()
		violations = append(violations, fmt.Sprintf(format, args...))
		vmu.Unlock()
	}
	var readers sync.WaitGroup
	for range 3 {
		readers.Go(func() {
			for {
				select {
				case <-done:
					return
				default:
				}
				messages, through, ok := r.view()
				if !ok {
					continue
				}
				// What the log holds up to the number and after it, by name.
				inFile := map[string]bool{}
				for _, m := range messages {
					switch m.Role {
					case "you":
						inFile[m.Text] = true
					case "tool":
						inFile[strings.SplitN(m.Text, "\n", 2)[0]] = true
					case "status":
						inFile[m.Text] = true
					}
				}
				for _, e := range r.entries(0) {
					name := entryName(e)
					if name == "" {
						continue
					}
					if before := e.N <= through; before != inFile[name] {
						report("entry %d (%s): up to the number %d is %v, in the saved copy is %v", e.N, name, through, before, inFile[name])
					}
				}
			}
		})
	}
	for turn := 0; turn < turns; turn++ {
		prompt := fmt.Sprintf("prompt %d", turn)
		// A prompt is logged before it is recorded and written, as the turn
		// loop does: the event first, then the record at the start of the turn.
		EmitChat(func(string, any) {}, r.s, EventSent, map[string]any{"sessionId": "live", "text": prompt})
		r.capture.RecordPrompt(r.s, ptr(NewPrompt(prompt, []acp.ContentBlock{{Type: "text", Text: prompt}})))
		var wg sync.WaitGroup
		wg.Go(func() {
			for i := 0; i < perTurn; i++ {
				title := fmt.Sprintf("tool %d.%d", turn, i)
				r.capture.RecordUpdateLogged(r.s, toolUpdate(title), true).Emit(func(string, any) {})
			}
		})
		// The turn ends while its last updates may still be in flight.
		r.capture.RecordTurnEnd(r.s, nil, fmt.Sprintf("end_turn %d", turn))
		r.capture.CommitTurn(r.s, map[string]any{"sessionId": "live", "stopReason": fmt.Sprintf("end_turn %d", turn)}).Emit(func(string, any) {})
		wg.Wait()
		// A turn that finished while its updates were arriving is written
		// again by the next one; commit once more so the last turn is whole.
		r.capture.FinishTurn(r.s, nil, "settled")
	}
	close(done)
	readers.Wait()
	if len(violations) > 0 {
		t.Fatalf("%d moments where the saved copy and the log number disagree, the first: %s", len(violations), violations[0])
	}
}

// disagreements checks the saved view against the log, as the readers of the
// main test do, and returns what disagrees.
func (r *throughRig) disagreements() []string {
	messages, through, ok := r.view()
	if !ok {
		return nil
	}
	inFile := map[string]bool{}
	for _, m := range messages {
		switch m.Role {
		case "you", "status":
			inFile[m.Text] = true
		case "tool":
			inFile[strings.SplitN(m.Text, "\n", 2)[0]] = true
		}
	}
	var out []string
	for _, e := range r.entries(0) {
		name := entryName(e)
		if name == "" {
			continue
		}
		if before := e.N <= through; before != inFile[name] {
			out = append(out, fmt.Sprintf("entry %d (%s): up to the number %d is %v, in the saved copy is %v", e.N, name, through, before, inFile[name]))
		}
	}
	return out
}

// Another writer of the saved copy that gets in between recording an update
// and logging it would write a copy that holds the update, with a number that
// lacks it. The two are one critical section, so a commit that starts in the
// middle waits for the update to be logged. The hook starts the commit there
// and gives it time to get in.
func TestACommitCannotGetBetweenRecordingAnUpdateAndLoggingIt(t *testing.T) {
	r := newThroughRig(t)
	EmitChat(func(string, any) {}, r.s, EventSent, map[string]any{"sessionId": "live", "text": "prompt"})
	r.capture.RecordPrompt(r.s, ptr(NewPrompt("prompt", []acp.ContentBlock{{Type: "text", Text: "prompt"}})))
	committed := make(chan struct{})
	captureAfterRecord = func() {
		go func() {
			r.capture.RecordTurnEnd(r.s, nil, "end_turn")
			r.capture.CommitTurn(r.s, map[string]any{"sessionId": "live", "stopReason": "end_turn"})
			close(committed)
		}()
		time.Sleep(150 * time.Millisecond)
	}
	t.Cleanup(func() { captureAfterRecord = nil })
	r.capture.RecordUpdateLogged(r.s, toolUpdate("late tool"), true).Emit(func(string, any) {})
	<-committed
	if d := r.disagreements(); len(d) > 0 {
		t.Fatalf("the saved copy and the log number disagree: %v", d)
	}
}

// A prompt recorded before its event is logged is the other order, and the
// check sees it: the copy written at the start of the turn holds the prompt
// and the number lacks its event. The turn loop logs the event first.
func TestThePairingCheckSeesAPromptLoggedAfterItIsRecorded(t *testing.T) {
	r := newThroughRig(t)
	r.capture.RecordPrompt(r.s, ptr(NewPrompt("prompt", []acp.ContentBlock{{Type: "text", Text: "prompt"}})))
	EmitChat(func(string, any) {}, r.s, EventSent, map[string]any{"sessionId": "live", "text": "prompt"})
	if d := r.disagreements(); len(d) == 0 {
		t.Fatal("a prompt logged after it was recorded was not seen")
	}
}

// entryName is the saved row an event of the log corresponds to, or empty for
// an event that has no row.
func entryName(e ChatEvent) string {
	var d map[string]any
	if json.Unmarshal(e.Data, &d) != nil {
		return ""
	}
	switch e.Name {
	case EventSent:
		s, _ := d["text"].(string)
		return s
	case EventUpdate:
		u, _ := d["update"].(map[string]any)
		title, _ := u["title"].(string)
		return title
	case EventTurn:
		s, _ := d["stopReason"].(string)
		if s == "" {
			return ""
		}
		return "Turn ended: " + s
	}
	return ""
}

func ptr[T any](v T) *T { return &v }

// The number is the log's head at each commit point, and it is not set before
// the chat has written its copy.
func TestTheLogNumberIsTheHeadAtTheLastWrite(t *testing.T) {
	r := newThroughRig(t)
	if _, through, ok := r.view(); !ok || through != 0 {
		t.Fatalf("a chat opened and written: %d %v", through, ok)
	}
	EmitChat(func(string, any) {}, r.s, EventSent, map[string]any{"sessionId": "live", "text": "first"})
	EmitChat(func(string, any) {}, r.s, EventSent, map[string]any{"sessionId": "live", "text": "second"})
	// Nothing is written yet, so the number has not moved.
	if _, through, _ := r.view(); through != 0 {
		t.Fatalf("the number moved without a write: %d", through)
	}
	r.capture.RecordPrompt(r.s, ptr(NewPrompt("first", []acp.ContentBlock{{Type: "text", Text: "first"}})))
	if _, through, _ := r.view(); through != 2 {
		t.Fatalf("after the prompt was recorded: %d", through)
	}
	r.capture.RecordUpdateLogged(r.s, toolUpdate("a tool"), true)
	if _, through, _ := r.view(); through != 2 {
		t.Fatalf("an update the copy does not hold yet moved the number: %d", through)
	}
	r.capture.RecordTurnEnd(r.s, nil, "end_turn")
	r.capture.CommitTurn(r.s, map[string]any{"sessionId": "live", "stopReason": "end_turn"})
	if _, through, _ := r.view(); through != 4 {
		t.Fatalf("after the turn was committed: %d", through)
	}
}

// A chat that has not written its copy since it was opened, as one replaying
// its history has not, has no number, and neither does a chat the engine does
// not have.
func TestAReplayingChatHasNoLogNumberYet(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	capture := NewTranscriptCapture(&mu, TranscriptCaptureHost{StateDir: func() string { return dir }, VisibleText: func(s string) string { return s }, RedactText: func(s string) string { return s }, Log: func(string, string, string, ...any) {}, TurnEnd: func(string, string, error) {}})
	s := NewConversation(Identity{ID: "replaying", Provider: "codex", Cwd: dir}, InitialActivity{Open: true}, true, false)
	capture.Initialize(s, true)
	mu.Lock()
	_, _, ok := s.HistoryThroughLocked()
	mu.Unlock()
	if ok {
		t.Fatal("a chat that has not written its copy has a number")
	}
}
