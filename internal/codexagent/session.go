package codexagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"studio/internal/acp"
	"studio/internal/profile"
)

// SessionParams adds host-owned Codex preferences to the internal ACP-shaped
// interface. The additional fields are never forwarded to an ACP provider.
type SessionParams struct {
	acp.LoadSessionParams
	AppletApprovals string `json:"dinahAppletApprovals,omitempty"`
	// Approvals is how far Codex may go without asking the person:
	// ApprovalsAsk (or empty), ApprovalsWorkspace or ApprovalsFull.
	Approvals string `json:"dinahApprovals,omitempty"`
}

// How far Codex may go without asking the person, the choice in Settings.
const (
	// ApprovalsAsk puts each escalation to the person: Codex runs in the
	// workspace sandbox and asks before it leaves it.
	ApprovalsAsk = "ask"
	// ApprovalsWorkspace runs commands in the workspace sandbox without
	// asking. A command that would write outside the workspace or reach the
	// network is refused by the sandbox rather than put to the person, so an
	// agent works unattended.
	ApprovalsWorkspace = "workspace"
	// ApprovalsFull runs Codex with no sandbox and no questions.
	ApprovalsFull = "full"
)

// approvalsPolicy is the policy a thread runs under, whatever it was given.
func approvalsPolicy(given string) string {
	switch given {
	case ApprovalsWorkspace, ApprovalsFull:
		return given
	}
	return ApprovalsAsk
}

// sandboxFor is the sandbox and approval policy Codex is started with.
func sandboxFor(policy string) (sandbox, approval string) {
	switch policy {
	case ApprovalsWorkspace:
		return "workspace-write", "never"
	case ApprovalsFull, "all":
		return "danger-full-access", "never"
	}
	return "workspace-write", "on-request"
}

type model struct {
	InputModalities []string `json:"inputModalities"`
	ID              string   `json:"id"`
	Model           string   `json:"model"`
	DisplayName     string   `json:"displayName"`
	DefaultEffort   string   `json:"defaultReasoningEffort"`
	IsDefault       bool     `json:"isDefault"`
	Efforts         []struct {
		Value       string `json:"reasoningEffort"`
		Description string `json:"description"`
	} `json:"supportedReasoningEfforts"`
}
type session struct {
	model, effort, turn string
	approvals           string
	baseApprovals       string
	appletApprovals     string
	completion          chan turn
	cancelRequested     bool
	streamed            map[string]bool
	requestContext      context.Context
	cancelRequests      context.CancelFunc
}
type turn struct {
	ID     string            `json:"id"`
	Status string            `json:"status"`
	Items  []json.RawMessage `json:"items"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}
type thread struct {
	ID        string `json:"id"`
	Cwd       string `json:"cwd"`
	Name      string `json:"name"`
	Preview   string `json:"preview"`
	UpdatedAt int64  `json:"updatedAt"`
	Turns     []turn `json:"turns"`
}
type threadResult struct {
	Thread thread `json:"thread"`
	Model  string `json:"model"`
	Effort string `json:"reasoningEffort"`
}

func native(id string) (string, error) {
	if !strings.HasPrefix(id, "codex:") || len(id) == 6 {
		return "", errors.New("invalid Codex session ID")
	}
	value := strings.TrimPrefix(id, "codex:")
	if len(value) > 200 {
		return "", errors.New("invalid Codex session ID")
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return "", errors.New("invalid Codex session ID")
		}
	}
	return value, nil
}

// Call translates ACP session operations. A small allowlist exposes native
// account and catalog methods to the host's provider settings.
func (c *Conn) Call(ctx context.Context, method string, params, out any) error {
	switch method {
	case "initialize":
		var p acp.InitializeParams
		if err := acp.Decode(params, &p); err != nil {
			return err
		}
		info := p.ClientInfo
		if info == nil {
			info = &acp.Implementation{Name: "dinah", Title: "Dinah", Version: "dev"}
		}
		if err := c.Conn.Call(ctx, "initialize", map[string]any{"clientInfo": info}, nil); err != nil {
			return err
		}
		if err := c.Conn.Notify("initialized", nil); err != nil {
			return err
		}
		r := acp.InitializeResult{ProtocolVersion: p.ProtocolVersion, AgentInfo: &acp.Implementation{Name: "codex", Title: "Codex", Version: "app-server"}}
		r.AgentCapabilities.LoadSession = true
		r.AgentCapabilities.PromptCapabilities = acp.PromptCapabilities{Image: true, EmbeddedContext: true}
		r.AgentCapabilities.SessionCapabilities.List = &struct{}{}
		r.AgentCapabilities.SessionCapabilities.Close = &struct{}{}
		return acp.Result(r, out)
	case "session/new", "session/load":
		return c.open(ctx, method, params, out)
	case "session/close":
		return c.closeSession(ctx, params)
	case "session/list":
		var p acp.ListSessionsParams
		if err := acp.Decode(params, &p); err != nil {
			return err
		}
		q := map[string]any{"limit": 100}
		if p.Cursor != "" {
			q["cursor"] = p.Cursor
		}
		if p.Cwd != "" {
			q["cwd"] = p.Cwd
		}
		var r struct {
			Data []thread `json:"data"`
			Next string   `json:"nextCursor"`
		}
		if err := c.Conn.Call(ctx, "thread/list", q, &r); err != nil {
			return err
		}
		res := acp.ListSessionsResult{Sessions: []acp.SessionInfo{}, NextCursor: r.Next}
		for _, t := range r.Data {
			if _, err := native("codex:" + t.ID); err != nil {
				return err
			}
			title := t.Name
			if title == "" {
				title = t.Preview
			}
			res.Sessions = append(res.Sessions, acp.SessionInfo{SessionID: "codex:" + t.ID, Cwd: t.Cwd, Title: title, UpdatedAt: time.Unix(t.UpdatedAt, 0).UTC().Format(time.RFC3339)})
		}
		return acp.Result(res, out)
	case "session/prompt":
		return c.prompt(ctx, params, out)
	case "session/cancel":
		return c.cancelTurn(ctx, params)
	case "session/set_config_option":
		return c.configure(params, out)
	case "account/read", "account/login/start", "account/login/cancel", "account/rateLimits/read", "model/list":
		return c.Conn.Call(ctx, method, params, out)
	default:
		return &acp.Error{Code: acp.CodeMethodNotFound, Message: "Codex does not support " + method}
	}
}

func (c *Conn) Notify(method string, params any) error {
	if method == "initialized" {
		return nil
	}
	if method != "session/cancel" {
		return &acp.Error{Code: acp.CodeMethodNotFound, Message: "Codex does not support " + method}
	}
	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()
	return c.cancelTurn(ctx, params)
}

func (c *Conn) catalog(ctx context.Context) error {
	var all []model
	cursor := ""
	for {
		p := map[string]any{"limit": 100}
		if cursor != "" {
			p["cursor"] = cursor
		}
		var r struct {
			Data []model `json:"data"`
			Next string  `json:"nextCursor"`
		}
		if err := c.Conn.Call(ctx, "model/list", p, &r); err != nil {
			return err
		}
		all = append(all, r.Data...)
		if r.Next == "" {
			break
		}
		if r.Next == cursor {
			return errors.New("Codex repeated the model page cursor")
		}
		cursor = r.Next
	}
	c.mu.Lock()
	c.models = all
	c.mu.Unlock()
	return nil
}

// Catalog discovers model and reasoning controls without creating a thread.
func (c *Conn) Catalog(ctx context.Context) ([]acp.ConfigOption, error) {
	if err := c.catalog(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	s := &session{}
	if len(c.models) > 0 {
		selected := c.models[0]
		for _, m := range c.models {
			if m.IsDefault {
				selected = m
				break
			}
		}
		s.model = selected.Model
		if s.model == "" {
			s.model = selected.ID
		}
		s.effort = selected.DefaultEffort
	}
	return c.options(s), nil
}

// SetApprovals changes host consent immediately and the sandbox on the next turn.
func (c *Conn) SetApprovals(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.sessions {
		s.appletApprovals = mode
		s.approvals = approvalsPolicy(s.baseApprovals)
		if mode == "all" {
			s.approvals = "all"
		}
	}
}

func (c *Conn) open(ctx context.Context, method string, params, out any) error {
	var p SessionParams
	if err := acp.Decode(params, &p); err != nil {
		return err
	}
	if p.AppletApprovals != "" && p.AppletApprovals != "ask" && p.AppletApprovals != "allow" && p.AppletApprovals != "all" {
		return errors.New("invalid Codex bundled-applet approval preference")
	}
	if method == "session/load" {
		id, err := native(p.SessionID)
		if err != nil {
			return err
		}
		var existing struct {
			Thread thread `json:"thread"`
		}
		if err = c.Conn.Call(ctx, "thread/read", map[string]any{"threadId": id, "includeTurns": false}, &existing); err != nil {
			return err
		}
		if !sameWorkspace(existing.Thread.Cwd, p.Cwd) {
			return errors.New("the Codex conversation belongs to a different workspace")
		}
	}
	config, err := mcpConfig(p.McpServers)
	if err != nil {
		return err
	}
	if p.AppletApprovals == "allow" || p.AppletApprovals == "all" {
		if managed, ok := config["mcp_servers"].(map[string]any)["dinah-applets"].(map[string]any); ok {
			managed["default_tools_approval_mode"] = "approve"
		}
	}
	policy := approvalsPolicy(p.Approvals)
	if p.AppletApprovals == "all" {
		policy = "all"
	}
	if policy != ApprovalsAsk && policy != "all" {
		// Nobody is going to be asked, so the servers Dinah starts must run
		// their tools without asking too. Otherwise every bus call and every
		// applet call raised a question, which is what stopped an unattended
		// Codex chat.
		if servers, ok := config["mcp_servers"].(map[string]any); ok {
			for name, entry := range servers {
				if q, ok := entry.(map[string]any); ok {
					q["default_tools_approval_mode"] = "approve"
					servers[name] = q
				}
			}
		}
	}
	if len(p.McpServers) > 0 {
		// ConfigReadParams.cwd is documented to resolve project layers. Reject
		// a reserved name instead of relying on recursive table merge behavior.
		var effective struct {
			Config struct {
				Servers map[string]json.RawMessage `json:"mcp_servers"`
			} `json:"config"`
		}
		if err = c.Conn.Call(ctx, "config/read", map[string]any{"cwd": p.Cwd, "includeLayers": false}, &effective); err != nil {
			return err
		}
		for name := range config["mcp_servers"].(map[string]any) {
			if _, exists := effective.Config.Servers[name]; exists {
				return fmt.Errorf("Codex configuration already defines the reserved MCP server %q; remove that conflicting entry before using %s", name, profile.Current().Name)
			}
		}
	}
	if err = c.catalog(ctx); err != nil {
		return err
	}
	sandbox, approval := sandboxFor(policy)
	q := map[string]any{"cwd": p.Cwd, "config": config, "sandbox": sandbox, "approvalPolicy": approval, "approvalsReviewer": "user"}
	rpcMethod := "thread/start"
	if method == "session/load" {
		id, err := native(p.SessionID)
		if err != nil {
			return err
		}
		q["threadId"] = id
		rpcMethod = "thread/resume"
	}
	var r threadResult
	if err = c.Conn.Call(ctx, rpcMethod, q, &r); err != nil {
		return err
	}
	if r.Thread.ID == "" {
		return errors.New("Codex returned no thread ID")
	}
	id := "codex:" + r.Thread.ID
	if _, err := native(id); err != nil {
		return err
	}
	if method == "session/load" && id != p.SessionID {
		return errors.New("Codex resumed a different conversation")
	}
	s := &session{model: r.Model, effort: r.Effort, approvals: policy, baseApprovals: approvalsPolicy(p.Approvals), appletApprovals: p.AppletApprovals, streamed: map[string]bool{}}
	c.mu.Lock()
	c.sessions[id] = s
	opts := c.options(s)
	c.mu.Unlock()
	if method == "session/load" {
		for _, t := range r.Thread.Turns {
			for _, item := range t.Items {
				c.item(id, item, true, true)
			}
		}
		return acp.Result(acp.LoadSessionResult{ConfigOptions: opts}, out)
	}
	return acp.Result(acp.NewSessionResult{SessionID: id, ConfigOptions: opts}, out)
}

func sameWorkspace(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (c *Conn) options(s *session) []acp.ConfigOption {
	vals := []map[string]string{}
	efforts := []map[string]string{}
	for _, m := range c.models {
		value := m.Model
		if value == "" {
			value = m.ID
		}
		vals = append(vals, map[string]string{"value": value, "name": m.DisplayName})
		if value == s.model {
			for _, e := range m.Efforts {
				efforts = append(efforts, map[string]string{"value": e.Value, "name": e.Value, "description": e.Description})
			}
		}
	}
	opts := []acp.ConfigOption{{ID: "model", Name: "Model", Category: "model", Type: "select", CurrentValue: acp.Raw(s.model), Options: acp.Raw(vals)}}
	if len(efforts) > 0 {
		opts = append(opts, acp.ConfigOption{ID: "reasoning_effort", Name: "Thinking", Category: "thought_level", Type: "select", CurrentValue: acp.Raw(s.effort), Options: acp.Raw(efforts)})
	}
	return opts
}

func (c *Conn) configure(params, out any) error {
	var p acp.SetConfigOptionParams
	if err := acp.Decode(params, &p); err != nil {
		return err
	}
	value, ok := p.Value.(string)
	if !ok {
		return errors.New("Codex option must be text")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sessions[p.SessionID]
	if s == nil {
		return errors.New("Codex session is not loaded")
	}
	if s.completion != nil {
		return errors.New("wait for the Codex turn to finish before changing its model")
	}
	switch p.ConfigID {
	case "model":
		found := false
		for _, m := range c.models {
			v := m.Model
			if v == "" {
				v = m.ID
			}
			if v == value {
				s.model = value
				s.effort = m.DefaultEffort
				found = true
				break
			}
		}
		if !found {
			return errors.New("model is not in the Codex catalog")
		}
	case "reasoning_effort":
		found := false
		for _, m := range c.models {
			if m.Model == s.model || m.ID == s.model {
				for _, e := range m.Efforts {
					if e.Value == value {
						found = true
					}
				}
			}
		}
		if !found {
			return errors.New("thinking level is not supported by this model")
		}
		s.effort = value
	default:
		return errors.New("unsupported Codex session option")
	}
	return acp.Result(acp.ConfigOptionsResult{ConfigOptions: c.options(s)}, out)
}

func (c *Conn) prompt(ctx context.Context, params, out any) error {
	var p acp.PromptParams
	if err := acp.Decode(params, &p); err != nil {
		return err
	}
	id, err := native(p.SessionID)
	if err != nil {
		return err
	}
	inputs, err := promptInputs(p)
	if err != nil {
		return err
	}
	c.mu.Lock()
	s := c.sessions[p.SessionID]
	if s == nil {
		c.mu.Unlock()
		return errors.New("Codex session is not loaded")
	}
	if s.completion != nil {
		c.mu.Unlock()
		return errors.New("Codex session already has an active turn")
	}
	ch := make(chan turn, 1)
	s.completion = ch
	s.turn = ""
	s.cancelRequested = false
	s.requestContext, s.cancelRequests = context.WithCancel(c.Context())
	s.streamed = map[string]bool{}
	q := map[string]any{"threadId": id, "input": inputs}
	// TurnStartParams documents these overrides for this and subsequent turns:
	// https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/schema/typescript/v2/TurnStartParams.ts
	_, approval := sandboxFor(s.approvals)
	q["approvalPolicy"] = approval
	if s.approvals == "all" || s.approvals == ApprovalsFull {
		q["sandboxPolicy"] = map[string]any{"type": "dangerFullAccess"}
	} else {
		q["sandboxPolicy"] = map[string]any{"type": "workspaceWrite", "writableRoots": []string{}, "networkAccess": false, "excludeTmpdirEnvVar": false, "excludeSlashTmp": false}
	}
	if s.model != "" {
		q["model"] = s.model
	}
	if s.effort != "" {
		q["effort"] = s.effort
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if s.completion == ch {
			s.cancelRequests()
			s.completion = nil
			s.turn = ""
		}
		c.mu.Unlock()
	}()
	var r struct {
		Turn turn `json:"turn"`
	}
	if err = c.Conn.Call(ctx, "turn/start", q, &r); err != nil {
		// A cancelled start may already have reached the server. Its eventual
		// turn/started notification must interrupt the work, or the process ends.
		if ctx.Err() != nil {
			c.Close()
		}
		return err
	}
	c.mu.Lock()
	s.turn = r.Turn.ID
	cancelled := s.cancelRequested
	c.mu.Unlock()
	if cancelled {
		_ = c.Notify("session/cancel", acp.CancelParams{SessionID: p.SessionID})
	}
	select {
	case t := <-ch:
		switch t.Status {
		case "completed":
			return acp.Result(acp.PromptResult{StopReason: "end_turn"}, out)
		case "interrupted":
			return acp.Result(acp.PromptResult{StopReason: "cancelled"}, out)
		default:
			if t.Error != nil {
				return errors.New(t.Error.Message)
			}
			return errors.New("Codex turn failed")
		}
	case <-ctx.Done():
		if err := c.Notify("session/cancel", acp.CancelParams{SessionID: p.SessionID}); err != nil {
			c.Close()
		} else {
			// Do not admit another turn until interruption is confirmed. If the
			// server cannot confirm it, ending the process prevents hidden work.
			timer := time.NewTimer(10 * time.Second)
			defer timer.Stop()
			select {
			case <-ch:
			case <-c.Done():
			case <-timer.C:
				c.Close()
			}
		}
		return ctx.Err()
	case <-c.Done():
		return errors.New("Codex stopped before completing the turn")
	}
}

func (c *Conn) cancelTurn(ctx context.Context, params any) error {
	var p acp.CancelParams
	if err := acp.Decode(params, &p); err != nil {
		return err
	}
	id, err := native(p.SessionID)
	if err != nil {
		return err
	}
	c.mu.Lock()
	s := c.sessions[p.SessionID]
	if s == nil || s.completion == nil {
		c.mu.Unlock()
		return nil
	}
	s.cancelRequested = true
	if s.cancelRequests != nil {
		s.cancelRequests()
	}
	turnID := s.turn
	c.mu.Unlock()
	if turnID == "" {
		return nil
	}
	return c.Conn.Call(ctx, "turn/interrupt", map[string]any{"threadId": id, "turnId": turnID}, nil)
}

// closeSession is ACP's session/close for a Codex thread. Any turn running
// is interrupted as session/cancel does, and the connection unsubscribes
// from the thread with thread/unsubscribe, after which app-server unloads
// the thread once it has had no subscribers and no activity for its grace
// period (https://learn.chatgpt.com/docs/app-server). The thread stays in
// Codex's history, so session/load resumes it with thread/resume, as it
// does for any stored thread. A session this connection has not opened is
// answered with the protocol's error for a session that is not active.
func (c *Conn) closeSession(ctx context.Context, params any) error {
	var p acp.CloseSessionParams
	if err := acp.Decode(params, &p); err != nil {
		return err
	}
	id, err := native(p.SessionID)
	if err != nil {
		return err
	}
	c.mu.Lock()
	s := c.sessions[p.SessionID]
	c.mu.Unlock()
	if s == nil {
		return &acp.Error{Code: acp.CodeResourceNotFound, Message: "the Codex session is not active"}
	}
	if err := c.cancelTurn(ctx, acp.CancelParams{SessionID: p.SessionID}); err != nil {
		return err
	}
	// The thread is unsubscribed only once an interrupted turn has ended,
	// since the turn's end arrives as a notification on the subscription.
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		c.mu.Lock()
		active := s.completion != nil
		c.mu.Unlock()
		if !active {
			break
		}
		select {
		case <-tick.C:
		case <-ctx.Done():
			return ctx.Err()
		case <-c.Done():
			return errors.New("Codex stopped while the session was closing")
		}
	}
	var r struct {
		Status string `json:"status"`
	}
	if err := c.Conn.Call(ctx, "thread/unsubscribe", map[string]any{"threadId": id}, &r); err != nil {
		return err
	}
	c.mu.Lock()
	if c.sessions[p.SessionID] == s {
		delete(c.sessions, p.SessionID)
	}
	c.mu.Unlock()
	return nil
}

func mcpConfig(servers []any) (map[string]any, error) {
	entries := map[string]any{}
	for _, s := range servers {
		var p struct {
			Name    string   `json:"name"`
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
			URL     string   `json:"url"`
			Env     []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"env"`
			Headers []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
		}
		if err := acp.Decode(s, &p); err != nil {
			return nil, err
		}
		if p.Name == "" {
			return nil, errors.New("MCP server requires a name")
		}
		if _, exists := entries[p.Name]; exists {
			return nil, errors.New("duplicate MCP server name")
		}
		q := map[string]any{"enabled": true, "required": true}
		if p.Name == "dinah-applets" {
			// The host owns this reserved server and its read-only annotations.
			// Codex documents writes mode as prompting only for mutating tools:
			// https://learn.chatgpt.com/docs/extend/mcp?surface=cli
			q["default_tools_approval_mode"] = "writes"
			q["tool_timeout_sec"] = 30
		}
		switch p.Type {
		case "", "stdio":
			if p.Command == "" {
				return nil, errors.New("MCP server requires a command")
			}
			q["command"] = p.Command
			q["args"] = p.Args
			env := map[string]string{}
			for _, v := range p.Env {
				env[v.Name] = v.Value
			}
			q["env"] = env
		case "http":
			q["url"] = p.URL
			headers := map[string]string{}
			for _, v := range p.Headers {
				headers[v.Name] = v.Value
			}
			q["http_headers"] = headers
		default:
			return nil, fmt.Errorf("Codex does not support MCP transport %q", p.Type)
		}
		entries[p.Name] = q
	}
	return map[string]any{"mcp_servers": entries}, nil
}

func promptInputs(p acp.PromptParams) ([]map[string]any, error) {
	inputs := []map[string]any{}
	for _, b := range p.Prompt {
		switch b.Type {
		case "text":
			inputs = append(inputs, map[string]any{"type": "text", "text": b.Text})
		case "image":
			inputs = append(inputs, map[string]any{"type": "image", "url": "data:" + b.MimeType + ";base64," + b.Data})
		case "resource_link":
			inputs = append(inputs, map[string]any{"type": "text", "text": b.Name + "\n" + b.URI})
		case "resource":
			if b.Resource == nil || b.Resource.Blob != "" {
				return nil, errors.New("Codex embedded resources must contain text")
			}
			inputs = append(inputs, map[string]any{"type": "text", "text": b.Resource.URI + "\n" + b.Resource.Text})
		default:
			return nil, fmt.Errorf("Codex does not support prompt content %q", b.Type)
		}
	}
	return inputs, nil
}
