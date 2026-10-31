package sessionhost

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"studio/internal/acp"
	"studio/internal/layout"
	"studio/internal/profile"
)

//go:embed history.go
var HistorySource []byte

// History is serialized by the host's existing transcript mutex (App.cmu).
// Every method requires that mutex held, including disk reads and writes.
// It never acquires the live session mutex or calls the host.
type History struct {
	saved, replay *Transcript
	err           error

	// head says the highest number in the chat's event log, and through is its
	// value when the transcript file was last written, so that a reader of the
	// file knows which log entries it already holds: every one up to through,
	// and none after. That holds because the events the history records are
	// appended to the log in the critical section that records them, which is
	// this mutex (docs/plans/running-chat-catch-up.md). through is set only
	// once a write has been made since the chat was opened.
	head       func() uint64
	through    uint64
	throughSet bool

	// tools are the first rows of the tool calls recorded in each transcript
	// the history writes to, by call (RecordToolLocked).
	tools map[*Transcript]map[string]*toolRow
}

// write writes the transcript file and notes where the chat's log was.
func (h *History) write(dir string, tr *Transcript) error {
	err := WriteTranscript(dir, tr)
	if err == nil && h.head != nil {
		h.through, h.throughSet = h.head(), true
	}
	return err
}

// ThroughLocked is the log number the transcript file is consistent with, and
// false before the chat has written it. The caller holds the history's mutex.
func (h *History) ThroughLocked() (uint64, bool) { return h.through, h.throughSet }

const deliveredRole = "delivered"

// sentRole is the role of an entry for a message the chat sent in a team's group chat and the bus took, with the
// same fields as a delivered one: the group and the message's number.
const sentRole = "sent"
const deliveredUpdate = "dinah_delivered"
const maxConversationToolText = 16 << 10
const OutsideHistory = "the provider history contains messages outside Dinah's saved transcript; keep the current provider or start a separate chat with the required context"
const InterruptedTurn = "a previous turn ended before its history was completely captured; keep this provider or start a separate chat with the required context"

// Provider replay may omit compacted turns. Every remaining visible message
// must match a complete captured message in the same order. A delivery is
// compared as the prompt the agent was given, which is what it replays when
// the replay did not keep it as its entries (delivered.go).
func ReplayRepresented(saved, replayed []ConversationMessage) bool {
	_, ok := firstUnrepresented(saved, replayed)
	return ok
}

// firstUnrepresented returns the first replayed message that the saved
// messages do not hold in order, and false, or true when they hold them all.
func firstUnrepresented(saved, replayed []ConversationMessage) (ConversationMessage, bool) {
	normalize := func(messages []ConversationMessage) []ConversationMessage {
		var out []ConversationMessage
		for _, m := range messages {
			if m.Role == deliveredRole {
				prompt, ok := promptPart(m)
				if !ok {
					continue
				}
				m = ConversationMessage{Role: "you", Text: prompt}
			}
			if m.Role != "you" && m.Role != "agent" && m.Role != "tool" {
				continue
			}
			last := len(out) - 1
			if last >= 0 && out[last].Role == m.Role && m.Role != "tool" {
				separator := ""
				if m.Role == "you" {
					separator = "\n\n"
				}
				out[last].Text += separator + m.Text
			} else {
				out = append(out, m)
			}
		}
		for i := range out {
			out[i].Text = strings.Join(strings.Fields(out[i].Text), " ")
		}
		return out
	}
	before, after := normalize(saved), normalize(replayed)
	at := 0
	for _, m := range before {
		if at < len(after) && m.Role == after[at].Role && m.Text == after[at].Text {
			at++
		}
	}
	if at < len(after) {
		return after[at], false
	}
	return ConversationMessage{}, true
}

// heldText joins the text of held user updates, or reports false when one
// of them is neither text nor an image, or none is text. An image is passed
// over, since a delivery gives the person's pictures as image blocks after
// its text (busPictures), and Dinah's record of the delivery is its text.
func heldText(held []json.RawMessage) (string, bool) {
	var parts []string
	for _, raw := range held {
		var u struct {
			Content struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if json.Unmarshal(raw, &u) != nil {
			return "", false
		}
		switch u.Content.Type {
		case "text":
			parts = append(parts, u.Content.Text)
		case "image":
		default:
			return "", false
		}
	}
	return strings.Join(parts, "\n\n"), len(parts) > 0
}

// deliveredGroup finds the run of the person's words and deliveries in the
// saved messages whose prompt text is text, among the runs that hold a
// delivery, compared as the replay check compares messages (words, not
// spacing).
func deliveredGroup(saved []ConversationMessage, text string) []ConversationMessage {
	want := strings.Join(strings.Fields(text), " ")
	for i := 0; i < len(saved); {
		if saved[i].Role != "you" && saved[i].Role != deliveredRole {
			i++
			continue
		}
		j := i
		for j < len(saved) && (saved[j].Role == "you" || saved[j].Role == deliveredRole) {
			j++
		}
		group := saved[i:j]
		i = j
		var parts []string
		delivered := false
		for _, m := range group {
			if t, ok := promptPart(m); ok {
				parts = append(parts, t)
				delivered = delivered || m.Role == deliveredRole
			}
		}
		if delivered && strings.Join(strings.Fields(strings.Join(parts, "\n\n")), " ") == want {
			return group
		}
	}
	return nil
}

// promptPart is what a saved message contributed to the text of the prompt
// the agent was given: the person's words, or a delivery's prompt on its
// first entry. The other entries of a delivery contributed nothing more.
func promptPart(m ConversationMessage) (string, bool) {
	switch m.Role {
	case "you":
		return m.Text, true
	case deliveredRole:
		if m.Delivered != nil && m.Delivered.Prompt != "" {
			return m.Delivered.Prompt, true
		}
	}
	return "", false
}

func (h *History) InitializeLocked(dir, id, provider, workspace string, replay bool) {
	h.tools = nil
	tr := &Transcript{SessionID: id, Provider: provider, Workspace: workspace, Complete: !replay, Messages: []ConversationMessage{}}
	if old, err := ReadTranscript(dir, id); err == nil {
		tr.ParentSessionID, tr.PendingHandoff = old.ParentSessionID, old.PendingHandoff
		tr.Handoff, tr.ParentHistory = old.Handoff, old.ParentHistory
		tr.Summary = old.Summary
		tr.Unsupported = old.Unsupported
		tr.CaptureError = old.CaptureError
		tr.Deleted = old.Deleted
		if replay {
			if !old.Complete {
				old.CaptureError = InterruptedTurn
			}
			h.saved, h.replay, h.err = old, tr, nil
			return
		}
	}
	h.saved, h.err = tr, nil
	if replay {
		h.replay = tr
	}
	if !replay {
		h.err = h.write(dir, tr)
	}
}

// ResumeLocked opens the saved copy as the chat's transcript without a replay
// and returns why it could not, or "" when it did. It can when the copy is whole: the file exists, is
// not deleted, belongs to this provider and workspace, is marked complete and
// has no capture error. A copy that may lack a turn cut off by a crash is not
// resumed, because only the agent's replay has that turn. The copy keeps its
// messages and flags and is written again, so the log position it is
// consistent with is the new log's, which is 0
// (docs/plans/session-resume.md).
func (h *History) ResumeLocked(dir, id, provider, workspace string) string {
	old, err := ReadTranscript(dir, id)
	switch {
	case err != nil:
		return "there is no saved copy"
	case old.Deleted:
		return "the saved copy is marked deleted"
	case !old.Complete:
		return "the saved copy is not complete"
	case old.CaptureError != "":
		return "the saved copy has a capture error"
	}
	saved := profile.ProviderOrLegacy(old.Provider)
	if saved != provider {
		return "the saved copy is another provider's"
	}
	if !layout.SamePath(old.Workspace, workspace) {
		return "the saved copy is another workspace's"
	}
	h.tools = nil
	h.saved, h.replay = old, nil
	h.err = h.write(dir, old)
	return ""
}

// PrepareReplayLocked clears the obsolete limitation before the host logs a
// replacement. FinishReplayLocked completes the same transcript transaction.
func (h *History) PrepareReplayLocked() (ConversationMessage, bool) {
	if h.saved != nil && h.saved.CaptureError == OutsideHistory {
		h.saved.CaptureError = ""
	}
	if h.saved == nil || h.replay == nil || h.saved == h.replay {
		return ConversationMessage{}, false
	}
	m, represented := firstUnrepresented(h.saved.Messages, h.replay.Messages)
	return copyMessages([]ConversationMessage{m})[0], !represented
}
func (h *History) FinishReplayLocked(dir string, succeeded bool) {
	if h.saved == nil {
		return
	}
	if !succeeded {
		h.err = errors.New("the provider did not finish replaying the chat history")
		return
	}
	// A whole replay is the agent's own history of the chat, so it settles
	// an earlier finding that the agent had messages the copy lacked. A cut-off
	// turn stays noted: a replay that brings nothing new does not show the
	// turn's end was captured.
	if h.saved.CaptureError == OutsideHistory {
		h.saved.CaptureError = ""
	}
	if h.replay != nil && h.replay != h.saved && !ReplayRepresented(h.saved.Messages, h.replay.Messages) {
		// The replay becomes the saved copy, so the chat reopens from it next
		// time and a handoff leaves out nothing the agent has. Turns the agent
		// compacted away, which the old copy still held, go with the old copy.

		h.saved.Messages = h.replay.Messages
		// The saved copy is now the replay's rows, and so are its tool calls'.
		if h.tools != nil {
			h.tools[h.saved] = h.tools[h.replay]
		}
		for _, m := range h.saved.Messages {
			h.saved.Unsupported = h.saved.Unsupported || m.Role == "limitation"
		}
		h.saved.Complete, h.saved.CaptureError = true, ""
		h.replay = nil
		h.err = h.write(dir, h.saved)
		return
	}
	h.saved.Complete = true
	if h.replay != nil && h.replay != h.saved {
		for _, m := range h.replay.Messages {
			if m.Role == "limitation" {
				h.saved.Messages = append(h.saved.Messages, m)
				h.saved.Unsupported = true
			}
		}
	}
	h.replay = nil
	h.err = h.write(dir, h.saved)
}

// RecordStatusLocked records a line Dinah says in the chat, with the kind it
// was said as.
func (h *History) RecordStatusLocked(replay bool, id, text, kind string) {
	tr := h.saved
	if replay && h.replay != nil {
		tr = h.replay
	}
	if tr == nil || tr.Deleted || text == "" {
		return
	}
	tr.Messages = append(tr.Messages, ConversationMessage{Role: "status", Text: text, Kind: kind, Provider: tr.Provider, SessionID: id})
}

func (h *History) RecordMessageLocked(replay bool, id, role, text string, unsupported, merge bool) {
	tr := h.saved
	if replay && h.replay != nil {
		tr = h.replay
	}
	if tr == nil || tr.Deleted {
		return
	}
	tr.Unsupported = tr.Unsupported || unsupported
	if unsupported {
		tr.Messages = append(tr.Messages, ConversationMessage{Role: "limitation", Text: "Image, binary attachment, or non-text result contents are not included in this handoff. The original remains in the earlier chat; attach any required content in the continued chat.", Provider: tr.Provider, SessionID: id})
	}
	// A tool's result is kept to its first part: the chat's log has the rest,
	// and a long chat of large results must not fill memory. The cut is the
	// same whenever the same result is recorded, so a replayed history still
	// matches the saved one.
	if role == "tool" && len(text) > maxConversationToolText {
		text = text[:maxConversationToolText] + fmt.Sprintf("\n[%d more characters of this result are in the chat's log]", len(text)-maxConversationToolText)
	}
	if text != "" {
		last := len(tr.Messages) - 1
		if merge && last >= 0 && tr.Messages[last].Role == role {
			tr.Messages[last].Text += text
		} else {
			tr.Messages = append(tr.Messages, ConversationMessage{Role: role, Text: text, Provider: tr.Provider, SessionID: id})
		}
	}
}

// toolRow is the first row of a tool call in a transcript, kept apart from its
// text so that later updates can change its title and add to its content.
type toolRow struct {
	// index is the call's first row and last the last row of the call's own,
	// which is where its ending goes: a replay lists each call's rows
	// together, one call after another, though the calls overlapped when they
	// were live.
	index, last   int
	title, status string
	content       []string
	outputBytes   int
}

// RecordSnapshotToolLocked keeps one row per call. The local Codex adapter
// emits raw output deltas and replaces them with an authoritative item result
// at completion or replay (codexagent/events.go). Partial output is bounded
// while its byte count preserves the usual truncation disclosure.
func (h *History) RecordSnapshotToolLocked(replay bool, id, callID string, snapshot bool, title, status string, content []string, unsupported bool, visible func(string) string) {
	tr := h.saved
	if replay && h.replay != nil {
		tr = h.replay
	}
	if tr == nil || tr.Deleted {
		return
	}
	if unsupported {
		h.RecordMessageLocked(replay, id, "tool", "", true, false)
	}
	if h.tools == nil {
		h.tools = map[*Transcript]map[string]*toolRow{}
	}
	if h.tools[tr] == nil {
		h.tools[tr] = map[string]*toolRow{}
	}
	r := h.tools[tr][callID]
	if r == nil {
		r = &toolRow{index: len(tr.Messages), content: []string{""}}
		h.tools[tr][callID] = r
		tr.Messages = append(tr.Messages, ConversationMessage{Role: "tool", Provider: tr.Provider, SessionID: id})
	}
	if title != "" {
		r.title = title
	}
	if status != "" {
		r.status = status
	}
	if snapshot {
		r.content[0], r.outputBytes = "", 0
	}
	output := strings.Join(content, "\n")
	if snapshot {
		output = visible(output)
	}
	r.outputBytes += len(output)
	room := maxConversationToolText - len(r.content[0])
	if room > len(output) {
		room = len(output)
	}
	r.content[0] += output[:room]
	text := r.text()
	if r.outputBytes == 0 {
		view := *r
		view.content = nil
		text = view.text()
	}
	// Deltas can split a private token anywhere. Keep only a bounded raw
	// prefix in memory, and redact the reconstructed row before saving it.
	// Redacting fragments separately cannot protect an interrupted call.
	text = visible(text)
	// text includes the title and status; account for those bytes when
	// computing the same cut that RecordMessageLocked applies to replay.
	total := len(text) + r.outputBytes - len(r.content[0])
	if total > maxConversationToolText {
		if len(text) > maxConversationToolText {
			text = text[:maxConversationToolText]
		}
		text += fmt.Sprintf("\n[%d more characters of this result are in the chat's log]", total-maxConversationToolText)
	}
	tr.Messages[r.index].Text = text
}

func (r *toolRow) text() string {
	var lines []string
	if r.title != "" {
		lines = append(lines, r.title)
	}
	if r.status != "" {
		lines = append(lines, "Status: "+r.status)
	}
	return strings.Join(append(lines, r.content...), "\n")
}

// terminalToolStatus is a status that ends a tool call.
func terminalToolStatus(s string) bool { return s == "completed" || s == "failed" }

// RecordToolLocked records a tool_call or tool_call_update of the Claude
// adapter as its history replay rebuilds a call, which is the same whichever
// way it arrives: one row for the call, with its final title, the status it
// began with and the content it was given; and one row for how it ended, with
// the status and the result. The adapter streams a call as a tool_call with a
// generic title and then updates that change the title and add content, which
// the replay of its history has already merged into the tool_call; updates
// that do not end the call are folded into the call's first row in the same
// way, so that a chat's saved copy and the replay of its history agree
// (docs/plans/group-chat.md, the replay disagreement). An update for a call
// that has no row yet, such as the first one seen after the chat was opened,
// starts one.
func (h *History) RecordToolLocked(replay bool, id, callID string, isCall bool, title, status string, content []string, unsupported bool) {
	tr := h.saved
	if replay && h.replay != nil {
		tr = h.replay
	}
	if tr == nil || tr.Deleted {
		return
	}
	if unsupported {
		h.RecordMessageLocked(replay, id, "tool", "", true, false)
	}
	if h.tools == nil {
		h.tools = map[*Transcript]map[string]*toolRow{}
	}
	rows := h.tools[tr]
	if rows == nil {
		rows = map[string]*toolRow{}
		h.tools[tr] = rows
	}
	first := rows[callID]
	if first != nil && !isCall && !terminalToolStatus(status) {
		if title != "" {
			first.title = title
		}
		first.content = append(first.content, content...)
		h.rewriteToolRow(tr, first)
		return
	}
	if first != nil && !isCall {
		// The call ends: its title, if it has a new one, is the first row's,
		// and this row says how it ended.
		if title != "" {
			first.title = title
			h.rewriteToolRow(tr, first)
		}
		h.insertToolEnding(tr, rows, first, id, strings.Join(append([]string{"Status: " + status}, content...), "\n"))
		return
	}
	row := &toolRow{title: title, status: status, content: content}
	h.RecordMessageLocked(replay, id, "tool", row.text(), false, false)
	row.index = len(tr.Messages) - 1
	row.last = row.index
	if isCall {
		rows[callID] = row
	} else if !terminalToolStatus(status) {
		rows[callID] = row
	}
}

// insertToolEnding puts the row that says how a call ended right after the
// call's own rows, ahead of whatever was recorded since, and moves the later
// calls' rows along.
func (h *History) insertToolEnding(tr *Transcript, rows map[string]*toolRow, first *toolRow, id, text string) {
	if len(text) > maxConversationToolText {
		text = text[:maxConversationToolText] + fmt.Sprintf("\n[%d more characters of this result are in the chat's log]", len(text)-maxConversationToolText)
	}
	at := first.last + 1
	if at > len(tr.Messages) {
		at = len(tr.Messages)
	}
	tr.Messages = slices.Insert(tr.Messages, at, ConversationMessage{Role: "tool", Text: text, Provider: tr.Provider, SessionID: id})
	for _, r := range rows {
		if r == first {
			continue
		}
		if r.index >= at {
			r.index++
		}
		if r.last >= at {
			r.last++
		}
	}
	first.last = at
}

// rewriteToolRow puts a tool call's first row's text back after its title or
// content changed, with the same cut a long result gets when it is recorded.
func (h *History) rewriteToolRow(tr *Transcript, r *toolRow) {
	if r.index < 0 || r.index >= len(tr.Messages) {
		return
	}
	text := r.text()
	if len(text) > maxConversationToolText {
		text = text[:maxConversationToolText] + fmt.Sprintf("\n[%d more characters of this result are in the chat's log]", len(text)-maxConversationToolText)
	}
	tr.Messages[r.index].Text = text
}

func (h *History) RecordImageLocked(replay bool, id string, image acp.ContentBlock) {
	tr := h.saved
	if replay && h.replay != nil {
		tr = h.replay
	}
	if tr == nil || tr.Deleted {
		return
	}
	if len(tr.Messages) == 0 || tr.Messages[len(tr.Messages)-1].Role != "you" {
		tr.Messages = append(tr.Messages, ConversationMessage{Role: "you", Provider: tr.Provider, SessionID: id})
	}
	last := len(tr.Messages) - 1
	tr.Messages[last].Images = append(tr.Messages[last].Images, copyContent([]acp.ContentBlock{image})[0])
}
func (h *History) RestoreDeliveryLocked(id string, held []json.RawMessage, start int) []json.RawMessage {
	out := copyUpdates(held)
	saved, replay := h.saved, h.replay
	if saved != nil && replay != nil && saved != replay && start <= len(replay.Messages) {
		if text, ok := heldText(held); ok {
			if group := deliveredGroup(saved.Messages, text); group != nil {
				kept := make([]ConversationMessage, len(group))
				out = make([]json.RawMessage, 0, len(group))
				for i, m := range group {
					m.Provider, m.SessionID = replay.Provider, id
					kept[i] = m
					var u any
					if m.Role == deliveredRole {
						page := m
						if page.Delivered != nil {
							d := *page.Delivered
							d.Prompt = ""
							page.Delivered = &d
						}
						u = map[string]any{"sessionUpdate": deliveredUpdate, "message": page}
					} else {
						u = map[string]any{"sessionUpdate": "user_message_chunk", "content": map[string]string{"type": "text", "text": m.Text}}
					}
					data, _ := json.Marshal(u)
					out = append(out, data)
				}
				replay.Messages = append(replay.Messages[:start:start], kept...)
			}
		}
	}
	return out
}
func copyTranscript(in *Transcript) *Transcript {
	if in == nil {
		return nil
	}
	out := *in
	out.Messages = copyMessages(in.Messages)
	out.ParentHistory = copyMessages(in.ParentHistory)
	return &out
}
func (h *History) SavedLocked() *Transcript { return copyTranscript(h.saved) }
func (h *History) ErrorLocked() error       { return h.err }
func (h *History) ReplayLengthLocked() int {
	if h.replay == nil {
		return 0
	}
	return len(h.replay.Messages)
}
func (h *History) RestoreSavedLocked(tr *Transcript) { h.saved = copyTranscript(tr) }
func (h *History) ReplaceDeletedLocked(tr *Transcript) {
	if h.saved != nil {
		*h.saved = *copyTranscript(tr)
	}
}
func (h *History) ConsumeHandoffLocked(dir string) {
	if h.saved != nil && h.saved.PendingHandoff {
		h.saved.PendingHandoff = false
		h.err = h.write(dir, h.saved)
	}
}
func (h *History) FinishPromptLocked(dir string, blocks []acp.ContentBlock, queueID string) {
	if h.saved == nil {
		return
	}
	if queueID != "" {
		h.saved.StartedQueueIDs = append(h.saved.StartedQueueIDs, queueID)
		if extra := len(h.saved.StartedQueueIDs) - startedQueueIDsKept; extra > 0 {
			h.saved.StartedQueueIDs = append([]string(nil), h.saved.StartedQueueIDs[extra:]...)
		}
	}
	if len(h.saved.Messages) > 0 {
		for _, block := range blocks {
			if block.Type == "image" {
				last := len(h.saved.Messages) - 1
				h.saved.Messages[last].Images = append(h.saved.Messages[last].Images, copyContent([]acp.ContentBlock{block})[0])
			}
		}
	}
	h.saved.Complete = false
	h.err = h.write(dir, h.saved)
}
func (h *History) FinishTurnLocked(dir string) {
	if h.saved != nil {
		h.saved.Complete = true
		h.err = h.write(dir, h.saved)
	}
}

// RecordSentLocked records a message the chat sent in a team's group, once for each message number, and says whether it
// was new.
func (h *History) RecordSentLocked(dir, id string, d DeliveredMessage, text string) bool {
	tr := h.saved
	if tr == nil || tr.Deleted {
		return false
	}
	for _, m := range tr.Messages {
		if m.Role == sentRole && m.Delivered != nil && m.Delivered.ID == d.ID {
			return false
		}
	}
	tr.Messages = append(tr.Messages, ConversationMessage{Role: sentRole, Text: text, Delivered: &d, Provider: tr.Provider, SessionID: id})
	// The mark is saved with the history, so it is there when the chat is reopened.
	h.err = h.write(dir, tr)
	return true
}

func (h *History) RecordDeliveredLocked(id, prompt string, entries []ConversationMessage) {
	tr := h.saved
	if tr == nil || tr.Deleted || len(entries) == 0 {
		return
	}
	for i, e := range entries {
		d := *e.Delivered
		d.Prompt = ""
		if i == 0 {
			d.Prompt = prompt
		}
		tr.Messages = append(tr.Messages, ConversationMessage{Role: deliveredRole, Text: e.Text, Delivered: &d, Provider: tr.Provider, SessionID: id})
	}
}

func (h *History) BeginHandoffLocked(dir, parent, parentWorkspace, context, summary string, history []ConversationMessage) error {
	if h.saved == nil || h.err != nil {
		return errors.New("the new chat history could not be saved")
	}
	h.saved.ParentSessionID, h.saved.PendingHandoff = parent, true
	h.saved.ParentWorkspace = ""
	if parentWorkspace != "" && !layout.SamePath(parentWorkspace, h.saved.Workspace) {
		h.saved.ParentWorkspace = parentWorkspace
	}
	h.saved.Handoff, h.saved.Summary = context, summary
	h.saved.ParentHistory = copyMessages(history)
	return h.write(dir, h.saved)
}
