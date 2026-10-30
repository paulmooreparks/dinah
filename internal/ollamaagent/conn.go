package ollamaagent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"studio/internal/acp"
	"studio/internal/appletcontext"
	"studio/internal/profile"
	"studio/internal/statefile"
)

// SessionParams adds the host's approval preferences to a session's opening,
// as for Codex: one for the applet tools, and one for the commands and writes
// Dinah runs itself.
type SessionParams struct {
	acp.LoadSessionParams
	AppletApprovals string `json:"dinahAppletApprovals,omitempty"`
	// CommandApprovals says when the person is asked before a command, a
	// write outside the workspace or an applet action that changes something:
	// ApprovalsEvery for each command line, ApprovalsChat once in each chat,
	// or ApprovalsNone not at all. Empty is ApprovalsChat.
	CommandApprovals string `json:"dinahCommandApprovals,omitempty"`
}

// When Ollama's own tools ask the person, the choice in Settings.
const (
	// ApprovalsEvery asks before each command line, and remembers only the
	// exact line the person allowed.
	ApprovalsEvery = "ask"
	// ApprovalsChat asks the first time in each chat, and then runs what the
	// person allowed there: a command, a write outside the workspace and an
	// applet action are each asked once in the chat. This is the default,
	// because a question per command line made a chat with an agent unusable.
	ApprovalsChat = "chat"
	// ApprovalsNone asks nothing: the tools run as the model asks them.
	ApprovalsNone = "allow"
	// ApprovalsAll allows commands, outside writes and applet calls.
	ApprovalsAll = "all"
)

// commandsPolicy is the policy a session runs under, whatever it was given.
func commandsPolicy(given string) string {
	switch given {
	case ApprovalsEvery, ApprovalsNone, ApprovalsAll:
		return given
	}
	return ApprovalsChat
}

// RoundLimit says the most model answers one turn may take, each followed by
// the tools it asked for, before the turn stops; 0 means no limit. It is asked
// before each round, so a change the person makes in Settings applies to a
// turn that is running.
type RoundLimit func() int

// SetRoundLimit gives the connection the function that says how many rounds a
// turn may take. Without one a turn has no limit.
func (c *Conn) SetRoundLimit(limit RoundLimit) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rounds = limit
}

// roundLimit is the limit now, or 0 for none.
func (c *Conn) roundLimit() int {
	c.mu.Lock()
	limit := c.rounds
	c.mu.Unlock()
	if limit == nil {
		return 0
	}
	return max(limit(), 0)
}

// defaultContext is the length every conversation was given before a
// conversation could use Ollama's own setting. A record saved with it and
// without a revision is taken to be on that default, and becomes one that
// leaves the length to Ollama (contextRevision).
const defaultContext = 65536

// OldDefaultContext is defaultContext for the window, which releases it from the
// options it saved for a chat while it was the default.
const OldDefaultContext = defaultContext

// contextRevision marks a record made, or settled, when a context of 0 meant
// the length set in Ollama (its app's slider or OLLAMA_CONTEXT_LENGTH), for
// which Dinah sends no num_ctx. A record with an older revision that holds the
// old default has it replaced by 0 once; a length the person chose is kept.
//
// Revision 2 added the flags and costs of the full-window action (Turn, Cost and
// Aside on a message, and Used and Choice on the record), which an older record
// simply lacks.
//
// Revision 3 released the old default again, for the records that had it put back.
// The window saves a chat's options and applies them when the chat is opened, so
// a context of 65536 saved while it was the default was applied as if the person
// had chosen it, to a record that had already been released, and the chat was
// back on 64K (30 October 2026). A record with an older revision that holds the
// old default has it released once more. The window releases the saved options
// the same way (provider_ollama.go).
const contextRevision = 3

// record is a conversation as Dinah keeps it, since Ollama keeps
// none.
type record struct {
	ID    string `json:"id"`
	Cwd   string `json:"cwd"`
	Title string `json:"title"`
	Model string `json:"model"`
	Think string `json:"think,omitempty"`
	// Context is the length the person chose, in tokens, which Dinah sends as
	// num_ctx; 0 leaves it to Ollama's own setting, and nothing is sent.
	Context int `json:"context"`
	// ContextRev says how Context is read (contextRevision).
	ContextRev int `json:"contextRev,omitempty"`
	// Used is the tokens in the conversation as Ollama last counted them,
	// and Choice the course the person chose for a full window
	// (context_window.go).
	Used     int       `json:"used,omitempty"`
	Choice   string    `json:"choice,omitempty"`
	Updated  time.Time `json:"updated"`
	Messages []message `json:"messages"`
}

type session struct {
	rec          record
	approvals    string
	commands     string
	servers      []server
	allowed      map[string]bool
	cancel       context.CancelFunc
	busy         bool
	steeringOpen bool
	steers       []steeringInput
	// warned is whether the person has been told the window is nearly full
	// since it last was not.
	warned bool
}

// Conn is a connection to Ollama with the conversations it holds.
type Conn struct {
	api     *client
	handler acp.Handler
	dir     string
	ctx     context.Context
	stop    context.CancelFunc
	done    chan struct{}

	mu       sync.Mutex
	sessions map[string]*session
	// kept is what the person allowed in each conversation closed in this
	// run, for when it is opened again (closeSession).
	kept    map[string]map[string]bool
	models  []tagged
	shown   map[string]shown
	applets map[string]*appletServer
	redact  func(string) string
	// scope says where the file tools may go (files.go); nil is anywhere.
	scope FileScope
	// rounds says how many rounds a turn may take; nil is no limit.
	rounds RoundLimit
}

// Start connects to Ollama at base, keeping conversations in stateDir.
func Start(base, stateDir string, h acp.Handler) (*Conn, error) {
	if _, err := probe(base); err != nil {
		return nil, err
	}
	dir := filepath.Join(stateDir, "ollama", "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	ctx, stop := context.WithCancel(context.Background())
	c := &Conn{api: newClient(base), handler: h, dir: dir, ctx: ctx, stop: stop, done: make(chan struct{}),
		sessions: map[string]*session{}, shown: map[string]shown{}, applets: map[string]*appletServer{}}
	c.stripStored()
	return c, nil
}

func (c *Conn) Done() <-chan struct{} { return c.done }
func (c *Conn) Err() error            { return nil }
func (c *Conn) Close() {
	c.mu.Lock()
	select {
	case <-c.done:
		c.mu.Unlock()
		return
	default:
	}
	c.stop()
	close(c.done)
	servers := c.applets
	c.applets = map[string]*appletServer{}
	c.mu.Unlock()
	for _, s := range servers {
		s.close()
	}
}
func (c *Conn) RegisterTraceSecret(string) {}
func (c *Conn) RegisterTraceRedactor(f func(string) string) {
	c.mu.Lock()
	c.redact = f
	c.mu.Unlock()
}

// valid says whether id is a session ID this package made.
func valid(id string) bool {
	v, ok := strings.CutPrefix(id, "ollama:")
	if !ok || len(v) != 32 {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil
}

func (c *Conn) update(id string, u any) {
	if c.handler != nil {
		c.handler.Notify("session/update", acp.Raw(map[string]any{"sessionId": id, "update": u}))
	}
}

func (c *Conn) text(id, kind, text string) {
	if text != "" {
		c.update(id, map[string]any{"sessionUpdate": kind, "content": map[string]string{"type": "text", "text": text}})
	}
}

// Call answers the ACP-shaped requests the workbench makes of a provider.
func (c *Conn) Call(ctx context.Context, method string, params, out any) error {
	switch method {
	case "initialize":
		var p acp.InitializeParams
		if err := acp.Decode(params, &p); err != nil {
			return err
		}
		version, err := c.api.version(ctx)
		if err != nil {
			return err
		}
		r := acp.InitializeResult{ProtocolVersion: p.ProtocolVersion, AgentInfo: &acp.Implementation{Name: "ollama", Title: "Ollama", Version: version}}
		r.AgentCapabilities.LoadSession = true
		r.AgentCapabilities.PromptCapabilities = acp.PromptCapabilities{Image: true, EmbeddedContext: true}
		r.AgentCapabilities.SessionCapabilities.List = &struct{}{}
		r.AgentCapabilities.SessionCapabilities.Delete = &struct{}{}
		r.AgentCapabilities.SessionCapabilities.Close = &struct{}{}
		return acp.Result(r, out)
	case "session/close":
		return c.closeSession(params)
	case "session/delete":
		var p acp.DeleteSessionParams
		if err := acp.Decode(params, &p); err != nil {
			return err
		}
		if !valid(p.SessionID) {
			return errors.New("invalid Ollama session ID")
		}
		c.mu.Lock()
		if s := c.sessions[p.SessionID]; s != nil && s.busy {
			c.mu.Unlock()
			return errors.New("the Ollama conversation has a turn going")
		}
		delete(c.sessions, p.SessionID)
		c.mu.Unlock()
		if err := os.Remove(c.file(p.SessionID)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	case "session/new", "session/load":
		return c.open(ctx, method, params, out)
	case "session/list":
		var p acp.ListSessionsParams
		if err := acp.Decode(params, &p); err != nil {
			return err
		}
		return acp.Result(c.list(p.Cwd), out)
	case "session/prompt":
		return c.prompt(ctx, params, out)
	case "session/cancel":
		return c.cancelTurn(params)
	case "session/set_config_option":
		return c.configure(params, out)
	case ImageInputMethod:
		return c.imageInput(ctx, params, out)
	case ContextMethod, SetAsideMethod, ChoiceMethod, TurnsMethod:
		return c.contextRequest(ctx, method, params, out)
	default:
		return &acp.Error{Code: acp.CodeMethodNotFound, Message: "Ollama does not support " + method}
	}
}

func (c *Conn) Notify(method string, params any) error {
	switch method {
	case "initialized":
		return nil
	case "session/cancel":
		return c.cancelTurn(params)
	}
	return &acp.Error{Code: acp.CodeMethodNotFound, Message: "Ollama does not support " + method}
}

// catalog reads the installed models.
func (c *Conn) catalog(ctx context.Context) error {
	models, err := c.api.tags(ctx)
	if err != nil {
		return err
	}
	sort.SliceStable(models, func(i, j int) bool {
		// Models on this computer come before those in Ollama's cloud.
		li, lj := models[i].RemoteHost == "", models[j].RemoteHost == ""
		if li != lj {
			return li
		}
		return models[i].Name < models[j].Name
	})
	c.mu.Lock()
	c.models = models
	c.mu.Unlock()
	return nil
}

// details are a model's capabilities, read once.
func (c *Conn) details(ctx context.Context, model string) (shown, error) {
	c.mu.Lock()
	s, ok := c.shown[model]
	c.mu.Unlock()
	if ok {
		return s, nil
	}
	s, err := c.api.show(ctx, model)
	if err != nil {
		return shown{}, err
	}
	c.mu.Lock()
	c.shown[model] = s
	c.mu.Unlock()
	return s, nil
}

// Catalog gives the model options without opening a conversation.
func (c *Conn) Catalog(ctx context.Context) ([]acp.ConfigOption, error) {
	if err := c.catalog(ctx); err != nil {
		return nil, err
	}
	rec := record{}
	c.mu.Lock()
	if len(c.models) > 0 {
		rec.Model = c.models[0].Name
	}
	c.mu.Unlock()
	c.settle(ctx, &rec)
	return c.options(rec), nil
}

// settle gives a record a thinking level and a context length that its
// model supports.
func (c *Conn) settle(ctx context.Context, rec *record) {
	if rec.Model == "" {
		return
	}
	s, err := c.details(ctx, rec.Model)
	if err != nil {
		return
	}
	levels := thinkLevels(s)
	if len(levels) == 0 {
		rec.Think = ""
	} else if !contains(levels, rec.Think) {
		rec.Think = defaultThink(s, levels)
	}
	// A length of 0 stays, which leaves the size to Ollama's own setting.
	limit := s.contextLength()
	if limit > 0 && rec.Context > limit {
		rec.Context = limit
	}
}

// thinkLevels are the thinking settings a model offers, as text: "true"
// and "false" for on and off, or its named levels.
func thinkLevels(s shown) []string {
	if s.Thinking != nil {
		var out []string
		for _, v := range s.Thinking.Values {
			out = append(out, fmt.Sprint(v))
		}
		if len(out) == 1 && out[0] == "false" {
			return nil
		}
		return out
	}
	if s.can("thinking") {
		return []string{"true", "false"}
	}
	return nil
}

func defaultThink(s shown, levels []string) string {
	if s.Thinking != nil && s.Thinking.Default != nil {
		return fmt.Sprint(s.Thinking.Default)
	}
	return levels[0]
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// think is a thinking setting as /api/chat takes it.
func think(v string) any {
	switch v {
	case "":
		return nil
	case "true":
		return true
	case "false":
		return false
	}
	return v
}

func (c *Conn) options(rec record) []acp.ConfigOption {
	c.mu.Lock()
	models := append([]tagged(nil), c.models...)
	s := c.shown[rec.Model]
	c.mu.Unlock()
	vals := []map[string]string{}
	for _, m := range models {
		name := m.Name
		if m.RemoteHost != "" {
			name += " (Ollama cloud)"
		} else if m.Details.ParameterSize != "" {
			name += " (" + m.Details.ParameterSize + ", on this computer)"
		}
		vals = append(vals, map[string]string{"value": m.Name, "name": name})
	}
	opts := []acp.ConfigOption{{ID: "model", Name: "Model", Category: "model", Type: "select", CurrentValue: acp.Raw(rec.Model), Options: acp.Raw(vals)}}
	if levels := thinkLevels(s); len(levels) > 0 {
		var choices []map[string]string
		for _, l := range levels {
			name := l
			switch l {
			case "true":
				name = "On"
			case "false":
				name = "Off"
			}
			choices = append(choices, map[string]string{"value": l, "name": name})
		}
		opts = append(opts, acp.ConfigOption{ID: "think", Name: "Thinking", Category: "thought_level", Type: "select", CurrentValue: acp.Raw(rec.Think), Options: acp.Raw(choices)})
	}
	limit := s.contextLength()
	sizes := []map[string]string{{"value": "0", "name": "Ollama's setting"}}
	for _, n := range contextLadder(limit) {
		sizes = append(sizes, map[string]string{"value": strconv.Itoa(n), "name": sizeName(n)})
	}
	description := "How much of the conversation the model holds at once. Ollama's setting leaves the size to Ollama, which is its app's slider or OLLAMA_CONTEXT_LENGTH. A size chosen here is sent with every request, and more needs more memory on the computer that runs the model."
	if limit == 0 {
		description += " Ollama did not report this model's own limit, so only a few sizes are offered."
	}
	opts = append(opts, acp.ConfigOption{ID: "context", Name: "Context", Description: description, Type: "select", CurrentValue: acp.Raw(strconv.Itoa(rec.Context)), Options: acp.Raw(sizes)})
	return opts
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "ollama:" + hex.EncodeToString(b)
}

func (c *Conn) file(id string) string {
	return filepath.Join(c.dir, strings.TrimPrefix(id, "ollama:")+".json")
}

// save keeps a conversation on disk without the applet contexts in it: each
// prompt carries the chat's private authorization, and so do the tool calls
// that use it, and the record is resent to the model every turn and may be read
// by others. The turn that is running keeps the current context in memory, so
// the model loses nothing (appletcontext).
func (c *Conn) save(rec record) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return statefile.Write(c.file(rec.ID), appletcontext.RedactBytes(b))
}

// stripStored removes the contexts that records saved before they were
// stripped still hold. It runs when the connection starts, and changes only a
// record that holds one.
func (c *Conn) stripStored() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(c.dir, e.Name())
		b, err := statefile.Read(path)
		if err != nil {
			continue
		}
		fixed := appletcontext.RedactBytes(b)
		fixed = repairTitle(fixed)
		if string(fixed) != string(b) {
			_ = statefile.Write(path, fixed)
		}
	}
}

// repairTitle replaces the title of a stored record that was made from text
// Dinah injected into the prompt, as one was when the first message had no text
// of the person's, with a title from the person's own words, or none.
func repairTitle(b []byte) []byte {
	var rec record
	if json.Unmarshal(b, &rec) != nil || !(strings.HasPrefix(rec.Title, "[Dinah ") || strings.Contains(rec.Title, appletcontext.Redacted)) {
		return b
	}
	rec.Title = ""
	for _, m := range rec.Messages {
		if m.Role == "user" {
			rec.Title = ownTitle(m.Content)
			break
		}
	}
	// Only the title changes, so the record is written as it was read.
	var all map[string]json.RawMessage
	if json.Unmarshal(b, &all) != nil {
		return b
	}
	all["title"], _ = json.Marshal(rec.Title)
	out, err := json.Marshal(all)
	if err != nil {
		return b
	}
	return out
}

func (c *Conn) load(id string) (record, error) {
	b, err := statefile.Read(c.file(id))
	if err != nil {
		return record{}, errors.New("the Ollama conversation is not on this computer")
	}
	var rec record
	if err := json.Unmarshal(b, &rec); err != nil {
		return record{}, fmt.Errorf("the Ollama conversation could not be read: %w", err)
	}
	if rec.ContextRev < 3 && rec.Context == defaultContext {
		rec.Context = 0
	}
	rec.ContextRev = contextRevision
	return rec, nil
}

func sameFolder(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (c *Conn) list(cwd string) acp.ListSessionsResult {
	res := acp.ListSessionsResult{Sessions: []acp.SessionInfo{}}
	entries, _ := os.ReadDir(c.dir)
	var recs []record
	for _, e := range entries {
		id := "ollama:" + strings.TrimSuffix(e.Name(), ".json")
		if !strings.HasSuffix(e.Name(), ".json") || !valid(id) {
			continue
		}
		rec, err := c.load(id)
		if err != nil || (cwd != "" && !sameFolder(rec.Cwd, cwd)) {
			continue
		}
		recs = append(recs, rec)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].Updated.After(recs[j].Updated) })
	for _, rec := range recs {
		res.Sessions = append(res.Sessions, acp.SessionInfo{SessionID: rec.ID, Cwd: rec.Cwd, Title: rec.Title, UpdatedAt: rec.Updated.UTC().Format(time.RFC3339)})
	}
	return res
}

func (c *Conn) open(ctx context.Context, method string, params, out any) error {
	var p SessionParams
	if err := acp.Decode(params, &p); err != nil {
		return err
	}
	if p.AppletApprovals != "" && p.AppletApprovals != "ask" && p.AppletApprovals != "allow" && p.AppletApprovals != "all" {
		return errors.New("invalid applet approval preference")
	}
	if p.CommandApprovals != "" && p.CommandApprovals != ApprovalsEvery && p.CommandApprovals != ApprovalsChat && p.CommandApprovals != ApprovalsNone && p.CommandApprovals != ApprovalsAll {
		return errors.New("invalid command approval preference")
	}
	servers, err := serversOf(p.McpServers)
	if err != nil {
		return err
	}
	if err := c.catalog(ctx); err != nil {
		return err
	}
	var rec record
	if method == "session/load" {
		if !valid(p.SessionID) {
			return errors.New("invalid Ollama session ID")
		}
		if rec, err = c.load(p.SessionID); err != nil {
			return err
		}
		if !sameFolder(rec.Cwd, p.Cwd) {
			return errors.New("the Ollama conversation belongs to a different workspace")
		}
	} else {
		rec = record{ID: newID(), Cwd: p.Cwd, ContextRev: contextRevision, Updated: time.Now()}
		c.mu.Lock()
		if len(c.models) > 0 {
			rec.Model = c.models[0].Name
		}
		c.mu.Unlock()
	}
	c.settle(ctx, &rec)
	c.mu.Lock()
	if old := c.sessions[rec.ID]; old != nil && old.busy {
		c.mu.Unlock()
		return errors.New("the Ollama conversation is busy")
	}
	// A conversation reopened in this run keeps what the person already
	// allowed in it, so a chat that reconnects does not ask again.
	allowed := map[string]bool{}
	if old := c.sessions[rec.ID]; old != nil {
		allowed = old.allowed
	} else if kept := c.kept[rec.ID]; kept != nil {
		allowed = kept
	}
	delete(c.kept, rec.ID)
	c.sessions[rec.ID] = &session{rec: rec, approvals: p.AppletApprovals, commands: commandsPolicy(p.CommandApprovals), servers: servers, allowed: allowed}
	c.mu.Unlock()
	opts := c.options(rec)
	if method == "session/load" {
		for _, m := range rec.Messages {
			switch m.Role {
			case "user":
				c.text(rec.ID, "user_message_chunk", m.Content)
			case "assistant":
				c.text(rec.ID, "agent_message_chunk", m.Content)
			}
		}
		return acp.Result(acp.LoadSessionResult{ConfigOptions: opts}, out)
	}
	return acp.Result(acp.NewSessionResult{SessionID: rec.ID, ConfigOptions: opts}, out)
}

func (c *Conn) configure(params, out any) error {
	var p acp.SetConfigOptionParams
	if err := acp.Decode(params, &p); err != nil {
		return err
	}
	value, ok := p.Value.(string)
	if !ok {
		return errors.New("an Ollama option must be text")
	}
	c.mu.Lock()
	s := c.sessions[p.SessionID]
	if s == nil {
		c.mu.Unlock()
		return errors.New("the Ollama conversation is not open")
	}
	if s.busy {
		c.mu.Unlock()
		return errors.New("wait for the turn to finish before changing its options")
	}
	rec := s.rec
	models := c.models
	c.mu.Unlock()
	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()
	switch p.ConfigID {
	case "model":
		found := false
		for _, m := range models {
			found = found || m.Name == value
		}
		if !found {
			return errors.New("that model is not installed in Ollama")
		}
		rec.Model = value
	case "think":
		sh, err := c.details(ctx, rec.Model)
		if err != nil {
			return err
		}
		if !contains(thinkLevels(sh), value) {
			return errors.New("that thinking setting is not one the model offers")
		}
		rec.Think = value
	case "context":
		n, err := strconv.Atoi(value)
		if err != nil || (n != 0 && n < 2048) {
			return errors.New("the context length must be a number of tokens, or 0 for Ollama's setting")
		}
		rec.Context = n
	default:
		return errors.New("unsupported Ollama option")
	}
	c.settle(ctx, &rec)
	c.mu.Lock()
	s.rec = rec
	c.mu.Unlock()
	if len(rec.Messages) > 0 {
		if err := c.save(rec); err != nil {
			return err
		}
	}
	return acp.Result(acp.ConfigOptionsResult{ConfigOptions: c.options(rec)}, out)
}

func (c *Conn) cancelTurn(params any) error {
	var p acp.CancelParams
	if err := acp.Decode(params, &p); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if s := c.sessions[p.SessionID]; s != nil && s.cancel != nil {
		s.steeringOpen = false
		s.cancel()
	}
	return nil
}

// closeSession is ACP's session/close. Any turn running is cancelled as
// session/cancel does, and the conversation is let go: its record is on
// disk already, saved after every turn, so session/load reads it again.
// What the person allowed in it is kept for this run, as for a chat that
// reconnects. The applet servers that no other open conversation uses are
// stopped.
func (c *Conn) closeSession(params any) error {
	var p acp.CloseSessionParams
	if err := acp.Decode(params, &p); err != nil {
		return err
	}
	if !valid(p.SessionID) {
		return errors.New("invalid Ollama session ID")
	}
	c.mu.Lock()
	s := c.sessions[p.SessionID]
	if s == nil {
		c.mu.Unlock()
		return &acp.Error{Code: acp.CodeResourceNotFound, Message: "the Ollama conversation is not active"}
	}
	if s.cancel != nil {
		s.steeringOpen = false
		s.cancel()
	}
	delete(c.sessions, p.SessionID)
	if c.kept == nil {
		c.kept = map[string]map[string]bool{}
	}
	c.kept[p.SessionID] = s.allowed
	used := map[string]bool{}
	for _, other := range c.sessions {
		for _, sv := range other.servers {
			used[sv.key()] = true
		}
	}
	var unused []*appletServer
	for _, sv := range s.servers {
		if a := c.applets[sv.key()]; a != nil && !used[sv.key()] {
			unused = append(unused, a)
			delete(c.applets, sv.key())
		}
	}
	c.mu.Unlock()
	for _, a := range unused {
		a.close()
	}
	return nil
}

// SetApprovals applies the Settings choice to every open chat.
func (c *Conn) SetApprovals(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.sessions {
		s.commands = commandsPolicy(mode)
		s.approvals = mode
	}
}

// system is the instruction an Ollama model works under in the workspace.
func system(cwd string, tools bool, approvals string) string {
	b := "You are an agent working with a person in " + profile.Current().Name + ". The workspace folder is " + cwd + ", on " + runtime.GOOS + ". Today is " + time.Now().Format("2 January 2006") + ". Answer in Markdown."
	if tools {
		b += " Use the tools to read, find and search files, write files and run commands rather than guessing what they contain; a relative path is relative to the workspace folder."
		if approvals != ApprovalsAll && approvals != ApprovalsNone {
			b += " The person is asked before a command runs or a file outside the workspace is written, and may refuse."
		}
		b += " When you have finished, reply with your answer and make no further tool call."
	} else {
		b += " This model cannot use tools, so you cannot read or change files or run commands; say so when a request needs them."
	}
	return b
}

func (c *Conn) prompt(ctx context.Context, params, out any) error {
	var p acp.PromptParams
	if err := acp.Decode(params, &p); err != nil {
		return err
	}
	user, err := promptMessage(p)
	if err != nil {
		return err
	}
	c.mu.Lock()
	s := c.sessions[p.SessionID]
	if s == nil {
		c.mu.Unlock()
		return errors.New("the Ollama conversation is not open")
	}
	if s.busy {
		c.mu.Unlock()
		return errors.New("the Ollama conversation already has a turn going")
	}
	turnCtx, cancel := context.WithCancel(ctx)
	s.busy, s.cancel, s.steeringOpen = true, cancel, true
	rec := s.rec
	commands, approvals, scope := s.commands, s.approvals, c.scope
	if approvals == ApprovalsAll {
		commands = ApprovalsAll
	}
	c.mu.Unlock()
	defer func() {
		cancel()
		c.mu.Lock()
		s.busy, s.cancel, s.steers, s.steeringOpen = false, nil, nil, false
		c.mu.Unlock()
	}()
	if rec.Model == "" {
		return errors.New("no Ollama model is installed; pull one with ollama pull")
	}
	sh, err := c.details(turnCtx, rec.Model)
	if err != nil {
		return err
	}
	if len(user.Images) > 0 && !sh.can("vision") {
		return errors.New(rec.Model + " cannot read images; choose a model with vision")
	}
	useTools := sh.can("tools")
	var tools []any
	if useTools {
		tools = ownTools(commands)
		applets, err := c.appletTools(turnCtx, s.servers)
		if err != nil {
			return err
		}
		tools = append(tools, applets...)
	}
	if rec.Title == "" {
		rec.Title = ownTitle(user.Content)
	}
	user.Turn = true
	turnStart, usedBefore := len(rec.Messages), rec.Used
	rec.Messages = append(rec.Messages, user)
	keep := func() error {
		rec.Updated = time.Now()
		if err := c.save(rec); err != nil {
			return fmt.Errorf("save conversation: %w", err)
		}
		c.mu.Lock()
		s.rec = rec
		c.mu.Unlock()
		return nil
	}
	if err := keep(); err != nil {
		return err
	}
	// What the turn costs is what Ollama's count of the conversation grew by,
	// recorded on the turn's first message when the turn ends, however it ends.
	measured := false
	defer func() {
		if !measured || turnStart >= len(rec.Messages) {
			return
		}
		rec.Messages[turnStart].Cost = max(rec.Used-usedBefore, 0)
		_ = keep()
	}()
	var refusal string
	tt := turnTools{c: c, session: rec.ID, cwd: rec.Cwd, commands: commands, allowed: s.allowed, mu: &c.mu, refusal: &refusal, scope: scope}
	// Input is inserted only between complete model exchanges, never into a
	// tool-call/result sequence. The mutex closes the race with final output.
	insertSteers := func(finish bool) (bool, error) {
		c.mu.Lock()
		pending := s.steers
		s.steers = nil
		if finish && len(pending) == 0 {
			s.steeringOpen = false
		}
		c.mu.Unlock()
		if err := persistSteeringMessages(&rec, pending, keep); err != nil {
			return false, err
		}
		return len(pending) > 0, nil
	}
	for round := 0; ; round++ {
		if limit := c.roundLimit(); limit > 0 && round >= limit {
			c.text(rec.ID, "agent_message_chunk", fmt.Sprintf("\n\nThe turn stopped at the limit of %d rounds of tool calls that you set in Settings. Send a message to continue.", limit))
			return acp.Result(acp.PromptResult{StopReason: "max_turn_requests"}, out)
		}
		if _, err := insertSteers(false); err != nil {
			return err
		}
		req := chatRequest{Model: rec.Model, Messages: append([]message{{Role: "system", Content: system(rec.Cwd, useTools, commands)}}, visibleMessages(rec.Messages)...), Tools: tools, Think: think(rec.Think)}
		if rec.Context > 0 {
			req.Options = map[string]any{"num_ctx": rec.Context}
		}
		answer, last, err := c.api.chat(turnCtx, req, func(ch chunk) {
			c.text(rec.ID, "agent_thought_chunk", ch.Message.Thinking)
			c.text(rec.ID, "agent_message_chunk", ch.Message.Content)
		})
		if err != nil {
			if turnCtx.Err() != nil {
				if err := keep(); err != nil {
					return err
				}
				return acp.Result(acp.PromptResult{StopReason: "cancelled"}, out)
			}
			return err
		}
		used := last.PromptEvalCount + last.EvalCount
		rec.Used, measured = used, true
		c.reportUsage(turnCtx, rec, used)
		size, _, known := c.window(turnCtx, rec)
		c.noteWindow(s, rec.ID, used, size, known)
		rec.Messages = append(rec.Messages, answer)
		if err := keep(); err != nil {
			return err
		}
		// A turn cancelled while the reply was arriving can still have its
		// reply come back whole. It ends here, before the steering messages
		// queued for it are saved and reported as delivered, since the
		// model never read them.
		if turnCtx.Err() != nil {
			return acp.Result(acp.PromptResult{StopReason: "cancelled"}, out)
		}
		if len(answer.ToolCalls) == 0 {
			inserted, err := insertSteers(true)
			if err != nil {
				return err
			}
			if inserted {
				continue
			}
			if last.DoneReason == "length" {
				return acp.Result(acp.PromptResult{StopReason: "max_tokens"}, out)
			}
			return acp.Result(acp.PromptResult{StopReason: "end_turn"}, out)
		}
		for i, call := range answer.ToolCalls {
			id := fmt.Sprintf("%s-%d-%d", strings.TrimPrefix(rec.ID, "ollama:")[:8], len(rec.Messages), i)
			var args map[string]any
			if len(call.Function.Arguments) > 0 && json.Unmarshal(call.Function.Arguments, &args) != nil {
				// Some models send the arguments as a JSON string.
				var s string
				if json.Unmarshal(call.Function.Arguments, &s) == nil {
					_ = json.Unmarshal([]byte(s), &args)
				}
			}
			if args == nil {
				args = map[string]any{}
			}
			heading, kind := title(call.Function.Name, args)
			c.update(rec.ID, map[string]any{"sessionUpdate": "tool_call", "toolCallId": id, "title": heading, "kind": kind, "status": "in_progress", "rawInput": args})
			tctx, tcancel := context.WithTimeout(turnCtx, toolTimeout)
			text, ok := tt.run(tctx, id, call.Function.Name, args)
			tcancel()
			status := "completed"
			if !ok {
				status = "failed"
			}
			shownText := text
			if len(shownText) > 4000 {
				shownText = shownText[:4000] + "…"
			}
			c.update(rec.ID, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": id, "status": status,
				"content": []any{map[string]any{"type": "content", "content": map[string]string{"type": "text", "text": shownText}}}})
			rec.Messages = append(rec.Messages, message{Role: "tool", ToolName: call.Function.Name, Content: text})
			if err := keep(); err != nil {
				return err
			}
			if turnCtx.Err() != nil {
				return acp.Result(acp.PromptResult{StopReason: "cancelled"}, out)
			}
		}
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	if r := []rune(line); len(r) > 80 {
		line = string(r[:80]) + "…"
	}
	return line
}

// ask asks the person whether a tool call may go ahead. always offers to
// allow the same call for the rest of the chat; the second result is
// "always" or "once" when allowed, and the reason to give the model when not.
func (c *Conn) ask(ctx context.Context, session, id, heading string, input any, always bool) (bool, string) {
	if c.handler == nil {
		return false, "Nobody could be asked, so it was not done."
	}
	options := []acp.PermissionOption{{OptionID: "once", Name: "Allow Once", Kind: "allow_once"}}
	if always {
		options = append(options, acp.PermissionOption{OptionID: "always", Name: "Allow in This Chat", Kind: "allow_always"})
	}
	options = append(options, acp.PermissionOption{OptionID: "reject", Name: "Reject", Kind: "reject_once"})
	answer, err := c.handler.Request(ctx, "session/request_permission", acp.Raw(acp.RequestPermissionParams{SessionID: session, ToolCall: acp.Raw(map[string]any{"toolCallId": id, "title": heading, "rawInput": input}), Options: options}))
	if err != nil {
		return false, "It was not done: " + err.Message
	}
	var o acp.PermissionOutcome
	if acp.Decode(answer, &o) != nil || o.Outcome.Outcome != "selected" || o.Outcome.OptionID == "reject" {
		return false, "The person did not allow it, so it was not done. Do not try it again unless they ask."
	}
	return true, o.Outcome.OptionID
}

// ownTitle makes a conversation's title from the person's own words: the first
// line of the message that is not text Dinah injected into the prompt. A first
// message of attachments only has none, so the title is empty and the chat is
// named later.
func ownTitle(content string) string {
	for _, block := range strings.Split(content, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		if strings.HasPrefix(block, "[Dinah ") || appletcontext.Contains(block) {
			continue
		}
		return firstLine(block)
	}
	return ""
}

func promptMessage(p acp.PromptParams) (message, error) {
	user := message{Role: "user"}
	var parts []string
	for _, b := range p.Prompt {
		switch b.Type {
		case "text":
			parts = append(parts, b.Text)
		case "image":
			user.Images = append(user.Images, b.Data)
		case "resource_link":
			parts = append(parts, b.Name+"\n"+b.URI)
		case "resource":
			if b.Resource == nil || b.Resource.Blob != "" {
				return message{}, errors.New("Ollama embedded resources must contain text")
			}
			parts = append(parts, b.Resource.URI+"\n"+b.Resource.Text)
		default:
			return message{}, fmt.Errorf("Ollama does not support prompt content %q", b.Type)
		}
	}
	user.Content = strings.Join(parts, "\n\n")
	return user, nil
}

// reportUsage tells the page how much of the context the last request used. The
// size and whether it is known are the conversation's window (window): a chosen
// size is known, and so is a cloud model's own limit on Ollama's setting, while
// a local model's limit is shown as the model's limit, which the server's
// setting may be below. Nothing is reported when no size is known at all.
func (c *Conn) reportUsage(ctx context.Context, rec record, used int) {
	// What the request used, which a team's budget counts (a turn_cost update is not shown).
	c.update(rec.ID, map[string]any{"sessionUpdate": "turn_cost", "tokens": used})
	size, kind, known := c.window(ctx, rec)
	if size <= 0 {
		return
	}
	c.update(rec.ID, map[string]any{"sessionUpdate": "usage_update", "used": used, "size": size, "sizeKind": kind, "windowKnown": known})
}

// contextLadder is the sizes offered for a model whose own limit is limit:
// doubling sizes from 8K, with the limit itself always the top entry. A model
// that reports no limit gets a short range up to 32K.
func contextLadder(limit int) []int {
	const first = 8192
	if limit <= 0 {
		return []int{first, 2 * first, 4 * first}
	}
	var out []int
	for n := first; n < limit; n *= 2 {
		out = append(out, n)
	}
	return append(out, limit)
}

// sizeName is a size as the option shows it.
func sizeName(n int) string {
	switch {
	case n%(1<<20) == 0:
		return strconv.Itoa(n>>20) + "M tokens"
	case n%1024 == 0:
		return strconv.Itoa(n>>10) + "K tokens"
	}
	return strconv.Itoa(n) + " tokens"
}

// ImageInputMethod is Dinah's own request for whether a conversation's model
// can read pictures, which the protocol cannot say per model.
const ImageInputMethod = "_dinah/imageInput"

// ImageInputResult says "yes" when the model's capabilities list vision, "no"
// when they list others and not vision, and "unknown" when Ollama could not
// say, which includes a server too old to list capabilities at all.
type ImageInputResult struct {
	ImageInput string `json:"imageInput"`
}

// imageInput reads the conversation's model's capabilities from /api/show,
// which documents "vision" among them.
func (c *Conn) imageInput(ctx context.Context, params, out any) error {
	var p acp.CancelParams
	if err := acp.Decode(params, &p); err != nil {
		return err
	}
	c.mu.Lock()
	s := c.sessions[p.SessionID]
	model := ""
	if s != nil {
		model = s.rec.Model
	}
	c.mu.Unlock()
	answer := "unknown"
	if model != "" {
		if shown, err := c.details(ctx, model); err == nil && len(shown.Capabilities) > 0 {
			answer = "no"
			if shown.can("vision") {
				answer = "yes"
			}
		}
	}
	return acp.Result(ImageInputResult{ImageInput: answer}, out)
}
