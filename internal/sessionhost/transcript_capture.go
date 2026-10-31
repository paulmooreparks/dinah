package sessionhost

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"

	"studio/internal/acp"
	"studio/internal/providers"
)

//go:embed transcript_capture.go
var TranscriptCaptureSource []byte

// TranscriptCapture shares the shell transcript mutex with continuation work.
// Its callbacks under that mutex must not acquire it again. Conversation history
// operations acquire the conversation mutex after this shared mutex.
type TranscriptCapture struct {
	mu   *sync.Mutex
	host TranscriptCaptureHost
}

type TranscriptCaptureHost struct {
	// Session and FlushReplay run outside the shared transcript mutex.
	Session     func(string) *Conversation
	FlushReplay func(*Conversation)
	ReplayBatch int
	StateDir    func() string
	VisibleText func(string) string
	RedactText  func(string) string
	Log         func(string, string, string, ...any)
	TurnEnd     func(string, string, error)
	Emit        func(string, any)
}

func NewTranscriptCapture(mu *sync.Mutex, host TranscriptCaptureHost) *TranscriptCapture {
	return &TranscriptCapture{mu: mu, host: host}
}

func (c *TranscriptCapture) Initialize(s *Conversation, replay bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s.InitializeHistoryLocked(c.host.StateDir(), replay)
}

// Resume opens the chat's saved copy for a reopen without a replay and returns
// why the copy could not be used, or "" when it was (history.go).
func (c *TranscriptCapture) Resume(s *Conversation) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return s.ResumeHistoryLocked(c.host.StateDir())
}

func (c *TranscriptCapture) FinishReplay(s *Conversation, succeeded bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if succeeded {
		if m, replace := s.HistoryPrepareReplayLocked(); replace {
			c.host.Log("chat", "warning", "chat %s replayed a %s message its saved copy lacked, so the replay is now its saved copy: %.300q", s.ID(), m.Role, m.Text)
		}
	}
	s.HistoryFinishReplayLocked(c.host.StateDir(), succeeded)
}

func (c *TranscriptCapture) RecordMessage(s *Conversation, role, text string, unsupported bool, merge bool) {
	replay := s.Replaying()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordMessageLocked(s, replay, role, text, unsupported, merge)
}

func (c *TranscriptCapture) recordMessageLocked(s *Conversation, replay bool, role, text string, unsupported bool, merge bool) {
	s.HistoryRecordMessageLocked(replay, s.ID(), role, c.host.VisibleText(text), unsupported, merge)
}

// RecordUpdate records an update of the conversation in the saved history.
func (c *TranscriptCapture) RecordUpdate(s *Conversation, update json.RawMessage) {
	if s == nil {
		return
	}
	replay := s.Replaying()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordUpdateLocked(s, replay, update)
}

// RecordUpdateLogged records a live update in the saved history and, when
// logIt is set, appends it to the chat's event log in the same critical
// section, so that a saved copy written at any moment holds exactly the
// updates the log has numbered up to the head it records. It returns the
// event to emit once the section is left, or nil when nothing was logged.
// Updates of a replay are recorded and numbered by their own path.
func (c *TranscriptCapture) RecordUpdateLogged(s *Conversation, update json.RawMessage, logIt bool) *LoggedEvent {
	if s == nil {
		return nil
	}
	replay := s.Replaying()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordUpdateLocked(s, replay, update)
	if captureAfterRecord != nil {
		captureAfterRecord()
	}
	if !logIt {
		return nil
	}
	return AppendChat(s, EventUpdate, map[string]any{"sessionId": s.ID(), "update": update})
}

// captureAfterRecord is a test hook, called between recording a live update
// and appending it, which is where another writer of the saved copy would
// have to get in for the two to disagree.
var captureAfterRecord func()

func (c *TranscriptCapture) recordUpdateLocked(s *Conversation, replay bool, update json.RawMessage) {
	var u struct {
		Type       string          `json:"sessionUpdate"`
		ToolCallID string          `json:"toolCallId"`
		Title      string          `json:"title"`
		Status     string          `json:"status"`
		Content    json.RawMessage `json:"content"`
	}
	if json.Unmarshal(update, &u) != nil {
		return
	}
	switch u.Type {
	case "dinah_status":
		// A line Dinah's own code says in the chat, such as turns set aside to
		// make room in a full window. A replay does not have it, so it is kept
		// only as it happens.
		var line struct {
			Text string `json:"text"`
			Kind string `json:"kind"`
		}
		if json.Unmarshal(update, &line) == nil && line.Text != "" && !replay {
			s.HistoryRecordStatusLocked(replay, s.ID(), c.host.VisibleText(line.Text), line.Kind)
		}
	case "user_message_chunk", "agent_message_chunk":
		if u.Type == "user_message_chunk" && !replay {
			return
		}
		var content acp.ContentBlock
		if json.Unmarshal(u.Content, &content) != nil {
			return
		}
		role := "agent"
		if u.Type == "user_message_chunk" {
			role = "you"
		}
		if role == "you" && content.Type == "image" {
			c.recordMessageLocked(s, replay, role, "", true, false)
			c.recordImageLocked(s, replay, content)
		} else {
			c.recordMessageLocked(s, replay, role, content.Text, content.Type != "text", u.Type == "agent_message_chunk")
		}
	case "tool_call", "tool_call_update":
		var parts []struct {
			Type    string           `json:"type"`
			Content acp.ContentBlock `json:"content"`
			Path    string           `json:"path"`
			OldText *string          `json:"oldText"`
			NewText string           `json:"newText"`
		}
		_ = json.Unmarshal(u.Content, &parts)
		var content []string
		unsupported := false
		for _, part := range parts {
			if part.Type == "diff" {
				old := ""
				if part.OldText != nil {
					old = *part.OldText
				}
				content = append(content, "File: "+part.Path+"\nPrevious text:\n"+old+"\nUpdated text:\n"+part.NewText)
			} else if part.Type == "content" && part.Content.Type == "text" {
				content = append(content, part.Content.Text)
			} else if part.Type == "content" && part.Content.Type == "resource" && part.Content.Resource != nil && part.Content.Resource.Blob == "" {
				content = append(content, part.Content.Resource.URI+"\n"+part.Content.Resource.Text)
			} else {
				unsupported = true
			}
		}
		if u.ToolCallID != "" && providers.For(s.Provider()).SnapshotToolCalls {
			s.HistoryRecordSnapshotToolLocked(replay, s.ID(), u.ToolCallID, u.Type == "tool_call" || terminalToolStatus(u.Status), c.host.VisibleText(u.Title), u.Status, content, unsupported, c.host.VisibleText)
			return
		}
		// The Claude adapter streams a call as updates that its history
		// replay has merged, so its calls are recorded as the replay has
		// them (history.go, RecordToolLocked). Measured on 29 October 2026
		// with the adapter's own protocol traffic; the other providers are
		// recorded one row for each event as before.
		if u.ToolCallID != "" && providers.For(s.Provider()).MergedToolCalls {
			s.HistoryRecordToolLocked(replay, s.ID(), u.ToolCallID, u.Type == "tool_call", u.Title, u.Status, content, unsupported)
			return
		}
		var lines []string
		if u.Title != "" {
			lines = append(lines, u.Title)
		}
		if u.Status != "" {
			lines = append(lines, "Status: "+u.Status)
		}
		c.recordMessageLocked(s, replay, "tool", strings.Join(append(lines, content...), "\n"), unsupported, false)
	}
}

func (c *TranscriptCapture) ConsumeHandoff(s *Conversation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s.HistoryConsumeHandoffLocked(c.host.StateDir())
}

func (c *TranscriptCapture) RecordPrompt(s *Conversation, p *Prompt) {
	unsupported := false
	var text []string
	for _, b := range p.Blocks() {
		if b.Type == "text" {
			text = append(text, b.Text)
		} else if b.Type == "resource" && b.Resource != nil && b.Resource.Blob == "" {
			text = append(text, b.Resource.URI+"\n"+b.Resource.Text)
		} else {
			unsupported = true
		}
	}
	if len(text) == 0 {
		text = append(text, p.Text())
	}
	if len(p.Delivered()) > 0 {
		// Messages the bus delivered are their senders' words, not the
		// person's, and are kept as entries of their own (delivered.go).
		c.RecordDelivered(s, strings.Join(text, "\n\n"), p.Delivered())
	} else {
		c.RecordMessage(s, "you", strings.Join(text, "\n\n"), unsupported, false)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	s.HistoryFinishPromptLocked(c.host.StateDir(), p.Blocks(), p.qid)
}

// FinishTurn ends a turn in the saved history: it records how the turn ended
// and writes the saved copy.
func (c *TranscriptCapture) FinishTurn(s *Conversation, err error, reason string) {
	c.RecordTurnEnd(s, err, reason)
	c.mu.Lock()
	defer c.mu.Unlock()
	s.HistoryFinishTurnLocked(c.host.StateDir())
}

// RecordTurnEnd records how a turn ended without writing the saved copy yet.
// The turn loop writes it with CommitTurn, once the turn's event is known.
func (c *TranscriptCapture) RecordTurnEnd(s *Conversation, err error, reason string) {
	status := "Turn ended: " + reason
	if err != nil {
		status = "Turn failed; its effects may need verification before retry: " + err.Error()
	}
	c.RecordMessage(s, "status", status, false, false)
	c.host.TurnEnd(s.ID(), reason, err)
}

// CommitTurn appends the turn's agent:turn event to the chat's log and writes
// the saved copy in one critical section, so that the copy, which holds how
// the turn ended, and the log number it records, which includes the event that
// says so, agree. It returns the event to emit once the section is left.
func (c *TranscriptCapture) CommitTurn(s *Conversation, turn map[string]any) *LoggedEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	logged := AppendChat(s, EventTurn, turn)
	s.HistoryFinishTurnLocked(c.host.StateDir())
	return logged
}

func (c *TranscriptCapture) RecordImage(s *Conversation, image acp.ContentBlock) {
	if image.Data == "" {
		return
	}
	replay := s.Replaying()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordImageLocked(s, replay, image)
}

func (c *TranscriptCapture) recordImageLocked(s *Conversation, replay bool, image acp.ContentBlock) {
	if image.Data == "" {
		return
	}
	s.HistoryRecordImageLocked(replay, s.ID(), image)
}

func (c *TranscriptCapture) RecordDelivered(s *Conversation, promptText string, entries []ConversationMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cleanPrompt := c.host.VisibleText(promptText)
	cleaned := make([]ConversationMessage, len(entries))
	for i, e := range entries {
		cleaned[i] = e
		cleaned[i].Text = c.host.RedactText(e.Text)
	}
	s.HistoryRecordDeliveredLocked(s.ID(), cleanPrompt, cleaned)
}

// RecordSent records a message the chat sent in a team's group chat and says so to the page, once for each message; the entry
// keeps the group and the message's number, so a window folds a run of team traffic without reading tool calls or prose.
func (c *TranscriptCapture) RecordSent(s *Conversation, d DeliveredMessage, text string) bool {
	c.mu.Lock()
	clean := c.host.RedactText(text)
	added := s.HistoryRecordSentLocked(c.host.StateDir(), s.ID(), d, clean)
	c.mu.Unlock()
	if added {
		// The event carries the redacted text, as the saved entry does, so an export made before a reload keeps it.
		EmitChat(c.host.Emit, s, EventTeamSent, map[string]any{"sessionId": s.ID(), "message": d, "text": clean})
	}
	return added
}

func (c *TranscriptCapture) ReplayBoundary(s *Conversation, kind string) {
	state := s.State()
	replaying, holding := state.Replaying, state.HoldingReplay

	if !replaying {
		return
	}
	if kind != "user_message_chunk" {
		if holding {
			c.SettleReplay(s)
		}
		return
	}
	if holding {
		return
	}
	c.mu.Lock()
	start := s.HistoryReplayLengthLocked()
	c.mu.Unlock()
	s.BeginHeldReplay(start)
}

func (c *TranscriptCapture) SettleReplay(s *Conversation) {
	held, start := s.DrainHeldReplay()
	if len(held) == 0 {
		return
	}
	out := held
	c.mu.Lock()
	out = s.HistoryRestoreDeliveryLocked(s.ID(), held, start)
	c.mu.Unlock()
	full := s.QueueReplay(out, c.host.ReplayBatch)
	if full {
		c.host.FlushReplay(s)
	}
}

func (c *TranscriptCapture) Tombstone(id string) error {
	s := c.host.Session(id)
	c.mu.Lock()
	defer c.mu.Unlock()
	tr, err := ReadTranscript(c.host.StateDir(), id)
	if err != nil {
		return nil
	}
	tr.Deleted = true
	tr.Messages, tr.ParentHistory = nil, nil
	tr.Handoff, tr.Summary, tr.CaptureError = "", "", ""
	tr.PendingHandoff, tr.Unsupported = false, false
	if s != nil {
		s.HistoryReplaceDeletedLocked(tr)
	}
	return WriteTranscript(c.host.StateDir(), tr)
}
