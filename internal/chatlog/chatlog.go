// Package chatlog writes a readable log of each chat with an agent: what the
// agent was asked, what it thought, each tool it used with its input and its
// result, what it answered, and how each turn ended, as one Markdown file per
// chat in the logs folder. The Agent Client Protocol streams a turn in small
// pieces (https://agentclientprotocol.com/protocol/prompt-turn); the log joins
// consecutive pieces of one kind into one block, so a reader follows the chat
// as it happened. Flow runs say in the chat's log which run and step a turn
// belongs to, and the run's own log points back at the chat (package runlog),
// so the whole chain from a flow's step to the agent's reasoning and output
// can be read back.
//
// The files are written through package rotlog: buffered, and rotated when a
// file reaches its size limit, so a long chat never makes an unbounded file.
// Prune removes old logs.
package chatlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"studio/internal/rotlog"
)

// Limits of what one block of the log keeps. A block past its limit is cut,
// with a line saying how much was left out.
const (
	maxBlock      = 256 << 10
	maxToolInput  = 4 << 10
	maxToolResult = 16 << 10
)

// Book is the chat logs of one program. The zero value is not usable; use New.
type Book struct {
	dir    string
	opt    rotlog.Options
	redact func(string) string

	mu    sync.Mutex
	chats map[string]*chat
	// closed says CloseAll was called, as the program ends: a chat still running
	// is then given a log that takes nothing, so no file is made after it.
	closed bool
}

type chat struct {
	w      *rotlog.Writer
	kind   string // the open block: "you", "thought", "agent", or ""
	at     time.Time
	text   strings.Builder
	cut    int
	tools  map[string]*tool
	turnAt time.Time
}

type tool struct {
	title, kind, status string
	input               string
	result              strings.Builder
	at                  time.Time
	reported            bool
}

// New returns the chat logs kept in dir. redact, when not nil, is applied to
// every text before it is written, to keep credentials out of the log.
func New(dir string, redact func(string) string) *Book {
	return &Book{dir: dir, redact: redact, chats: map[string]*chat{},
		opt: rotlog.Options{MaxSize: 16 << 20, Keep: 3}}
}

// Dir is the folder of the logs.
func (b *Book) Dir() string { return b.dir }

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Path is the log file of a chat.
func (b *Book) Path(session string) string {
	return filepath.Join(b.dir, unsafeName.ReplaceAllString(session, "_")+".md")
}

func (b *Book) clean(s string) string {
	if b.redact != nil {
		s = b.redact(s)
	}
	return s
}

func clock(t time.Time) string { return t.Format("15:04:05") }

// get returns the open chat, opening its log when it is not open yet.
func (b *Book) get(session string) *chat {
	c := b.chats[session]
	if c == nil && b.closed {
		c = &chat{w: rotlog.Open(b.Path(session), b.opt), tools: map[string]*tool{}}
		_ = c.w.Close()
		return c
	}
	if c == nil {
		c = &chat{w: rotlog.Open(b.Path(session), b.opt), tools: map[string]*tool{}}
		b.chats[session] = c
	}
	return c
}

// Begin opens a chat's log and writes its heading when the file is new: the
// chat, the agent, the folder it works in and its title.
func (b *Book) Begin(session, provider, workspace, title string) {
	if b == nil || session == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	path := b.Path(session)
	_, statErr := os.Stat(path)
	c := b.get(session)
	if os.IsNotExist(statErr) {
		fmt.Fprintf(c.w, "# Chat %s\n\n", session)
		if title != "" {
			fmt.Fprintf(c.w, "- Title: %s\n", b.clean(title))
		}
		if provider != "" {
			fmt.Fprintf(c.w, "- Agent: %s\n", provider)
		}
		if workspace != "" {
			fmt.Fprintf(c.w, "- Folder: %s\n", workspace)
		}
		fmt.Fprintf(c.w, "- Opened: %s\n\n", time.Now().Format(time.RFC3339))
	} else {
		fmt.Fprintf(c.w, "\n---\n\n%s Opened again.\n\n", time.Now().Format(time.RFC3339))
	}
}

// Note adds a line of the program's own to a chat's log, such as which flow
// run and step the next turn belongs to.
func (b *Book) Note(session, text string) {
	if b == nil || session == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.get(session)
	b.endBlock(c)
	fmt.Fprintf(c.w, "> %s %s\n\n", clock(time.Now()), strings.ReplaceAll(b.clean(strings.TrimSpace(text)), "\n", "\n> "))
}

// Prompt records what the agent was asked, with the label the program gave
// the turn, such as a flow step's name.
func (b *Book) Prompt(session, label, text string) {
	if b == nil || session == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.get(session)
	b.endBlock(c)
	now := time.Now()
	c.turnAt = now
	heading := "Prompt"
	if label != "" {
		heading += ": " + b.clean(label)
	}
	fmt.Fprintf(c.w, "## %s %s\n\n", clock(now), heading)
	writeCut(c.w, b.clean(text), maxBlock)
	c.w.WriteString("\n\n")
}

// Update records one session/update of the protocol: a piece of the agent's
// message or thinking, a replayed piece of the person's message, or a tool
// call and its progress. Other updates are not logged.
func (b *Book) Update(session string, update json.RawMessage) {
	if b == nil || session == "" {
		return
	}
	var u struct {
		Type       string          `json:"sessionUpdate"`
		Content    json.RawMessage `json:"content"`
		ToolCallID string          `json:"toolCallId"`
		Title      string          `json:"title"`
		Kind       string          `json:"kind"`
		Status     string          `json:"status"`
		RawInput   json.RawMessage `json:"rawInput"`
	}
	if json.Unmarshal(update, &u) != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	switch u.Type {
	case "agent_message_chunk", "agent_thought_chunk", "user_message_chunk":
		var content struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(u.Content, &content) != nil || content.Type != "text" {
			return
		}
		kind := map[string]string{"agent_message_chunk": "agent", "agent_thought_chunk": "thought", "user_message_chunk": "you"}[u.Type]
		c := b.get(session)
		if c.kind != kind {
			b.endBlock(c)
			c.kind, c.at = kind, time.Now()
		}
		if room := maxBlock - c.text.Len(); room > 0 {
			if len(content.Text) > room {
				c.text.WriteString(content.Text[:room])
				c.cut += len(content.Text) - room
			} else {
				c.text.WriteString(content.Text)
			}
		} else {
			c.cut += len(content.Text)
		}
	case "tool_call", "tool_call_update":
		c := b.get(session)
		b.endBlock(c)
		t := c.tools[u.ToolCallID]
		if t == nil {
			t = &tool{at: time.Now()}
			c.tools[u.ToolCallID] = t
		}
		if u.Title != "" {
			t.title = u.Title
		}
		if u.Kind != "" {
			t.kind = u.Kind
		}
		if u.Status != "" {
			t.status = u.Status
		}
		if t.input == "" {
			t.input = toolInput(u.RawInput)
		}
		if text := toolContent(u.Content); text != "" && t.result.Len() < maxToolResult {
			if t.result.Len() > 0 {
				t.result.WriteString("\n")
			}
			t.result.WriteString(text)
		}
		if t.status == "completed" || t.status == "failed" {
			b.writeTool(c, u.ToolCallID, t)
			delete(c.tools, u.ToolCallID)
		}
	}
}

// TurnEnd records how a turn ended, and writes what is still held.
func (b *Book) TurnEnd(session, stopReason string, err error) {
	if b == nil || session == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.get(session)
	b.endBlock(c)
	for id, t := range c.tools {
		b.writeTool(c, id, t)
		delete(c.tools, id)
	}
	now := time.Now()
	took := ""
	if !c.turnAt.IsZero() {
		took = fmt.Sprintf(" after %s", now.Sub(c.turnAt).Round(time.Second))
	}
	switch {
	case err != nil:
		fmt.Fprintf(c.w, "---\n\n%s The turn failed%s: %s\n\n", clock(now), took, b.clean(err.Error()))
	default:
		if stopReason == "" {
			stopReason = "end_turn"
		}
		fmt.Fprintf(c.w, "---\n\n%s The turn ended%s: %s\n\n", clock(now), took, stopReason)
	}
	c.turnAt = time.Time{}
	_ = c.w.Flush()
}

// Close writes what a chat's log holds and closes it; it opens again when
// the chat is written to.
func (b *Book) Close(session string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if c := b.chats[session]; c != nil {
		b.endBlock(c)
		_ = c.w.Close()
		delete(b.chats, session)
	}
}

// CloseAll closes every open log, as the program ends.
func (b *Book) CloseAll() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for id, c := range b.chats {
		b.endBlock(c)
		_ = c.w.Close()
		delete(b.chats, id)
	}
}

// Flush writes what every open log holds, without closing them.
func (b *Book) Flush() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, c := range b.chats {
		_ = c.w.Flush()
	}
}

// endBlock writes the open block of joined pieces.
func (b *Book) endBlock(c *chat) {
	if c.kind == "" {
		return
	}
	heading := map[string]string{"you": "You (replayed)", "thought": "Thinking", "agent": "Agent"}[c.kind]
	fmt.Fprintf(c.w, "### %s %s\n\n", clock(c.at), heading)
	text := strings.TrimSpace(b.clean(c.text.String()))
	c.w.WriteString(text)
	if c.cut > 0 {
		fmt.Fprintf(c.w, "\n\n[%d more characters of this block are not kept]", c.cut)
	}
	c.w.WriteString("\n\n")
	c.kind, c.cut = "", 0
	c.text.Reset()
}

func (b *Book) writeTool(c *chat, id string, t *tool) {
	title := t.title
	if title == "" {
		title = "a tool"
	}
	kind := ""
	if t.kind != "" {
		kind = ", " + t.kind
	}
	status := t.status
	if status == "" {
		status = "no result reported"
	}
	fmt.Fprintf(c.w, "### %s Tool: %s (%s%s)\n\n", clock(t.at), b.clean(title), status, kind)
	if t.input != "" {
		c.w.WriteString("Input:\n\n")
		fence(c.w, b.clean(t.input), maxToolInput)
	}
	if t.result.Len() > 0 {
		c.w.WriteString("Result:\n\n")
		fence(c.w, b.clean(t.result.String()), maxToolResult)
	}
	_ = id
}

func toolInput(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var in struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(raw, &in) == nil && in.Command != "" {
		return in.Command
	}
	return string(raw)
}

func toolContent(raw json.RawMessage) string {
	var parts []struct {
		Type    string `json:"type"`
		Path    string `json:"path"`
		Content struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Resource *struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"resource"`
		} `json:"content"`
		NewText string `json:"newText"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var out []string
	for _, p := range parts {
		switch {
		case p.Type == "diff":
			out = append(out, "Changed "+p.Path+":\n"+p.NewText)
		case p.Type == "content" && p.Content.Type == "text":
			out = append(out, p.Content.Text)
		case p.Type == "content" && p.Content.Resource != nil:
			out = append(out, p.Content.Resource.URI+"\n"+p.Content.Resource.Text)
		}
	}
	return strings.Join(out, "\n")
}

func writeCut(w *rotlog.Writer, s string, limit int) {
	if len(s) > limit {
		w.WriteString(s[:limit])
		fmt.Fprintf(w, "\n\n[%d more characters are not kept]", len(s)-limit)
		return
	}
	w.WriteString(s)
}

// fence writes text as a block a Markdown reader shows as it is, with a fence
// no line of the text can close.
func fence(w *rotlog.Writer, s string, limit int) {
	marks := "~~~~"
	for strings.Contains(s, marks) {
		marks += "~"
	}
	w.WriteString(marks + "text\n")
	writeCut(w, strings.TrimRight(s, "\n"), limit)
	w.WriteString("\n" + marks + "\n\n")
}
