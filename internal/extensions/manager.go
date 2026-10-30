// Package extensions hosts signed native extensions and invocation-scoped host APIs.
package extensions

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	extension "github.com/paulmooreparks/dinah-extension-go"
	"studio/internal/applet"
	"studio/internal/procjob"
)

type Hooks struct {
	Workspace func(string) (extension.Workspace, error)
	Notify    func(context.Context, string, string) error
	Terminal  func(context.Context, extension.TerminalInput) error
	Decorate  func(context.Context, json.RawMessage) error
	Changed   func()
	Review    func(context.Context, string, extension.RunInput) ([]string, error)
}
type process struct {
	session   *mcp.ClientSession
	signature string
	starts    int
	mu        sync.Mutex
}
type scope struct {
	ctx         context.Context
	cancel      context.CancelFunc
	workspace   extension.Workspace
	permissions []string
	extensionID string
	effect      string
}
type Manager struct {
	Store        extension.Store
	Hooks        Hooks
	mu           sync.Mutex
	processes    map[string]*process
	scopes       map[string]scope
	listener     net.Listener
	server       *http.Server
	url          string
	closed       bool
	pauses       map[string]map[string]bool
	controlToken string
	controlFile  string
}

func New(root string, hooks Hooks) *Manager {
	return &Manager{Store: extension.Store{Root: root}, Hooks: hooks, processes: map[string]*process{}, scopes: map[string]scope{}, pauses: map[string]map[string]bool{}}
}
func (m *Manager) serve() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("extension host is closed")
	}
	if m.server != nil {
		return nil
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	m.listener = ln
	m.url = "http://" + ln.Addr().String() + "/host"
	m.server = &http.Server{Handler: http.HandlerFunc(m.hostHTTP), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	if err := m.publishControl(); err != nil {
		ln.Close()
		m.server = nil
		return err
	}
	go m.server.Serve(ln)
	return nil
}
func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	server := m.server
	list := []*process{}
	for _, p := range m.processes {
		list = append(list, p)
	}
	m.scopes = map[string]scope{}
	controlFile := m.controlFile
	m.mu.Unlock()
	if server != nil {
		server.Close()
	}
	if controlFile != "" {
		os.Remove(controlFile)
	}
	for _, p := range list {
		p.mu.Lock()
		s := p.session
		p.session = nil
		p.mu.Unlock()
		if s != nil {
			s.Close()
		}
	}
	return nil
}
func (m *Manager) Stop(id string) {
	m.mu.Lock()
	p := m.processes[id]
	delete(m.processes, id)
	m.mu.Unlock()
	if p != nil {
		p.mu.Lock()
		s := p.session
		p.session = nil
		p.mu.Unlock()
		if s != nil {
			s.Close()
		}
	}
}
func (m *Manager) Installed(id string) (extension.Installation, error) {
	list, err := m.Store.List()
	if err != nil {
		return extension.Installation{}, err
	}
	for _, v := range list {
		if v.Manifest.ID == id {
			if !v.Enabled {
				return v, errors.New("extension is disabled")
			}
			verified, err := extension.Verify(m.Store.Path(v.Manifest))
			if err != nil {
				return v, err
			}
			if verified.Signature != v.Manifest.Signature {
				return v, errors.New("installed manifest differs from the active installation")
			}
			return v, nil
		}
	}
	return extension.Installation{}, errors.New("extension is not installed")
}
func (m *Manager) session(ctx context.Context, v extension.Installation) (*process, *mcp.ClientSession, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, nil, errors.New("extension host is closed")
	}
	if len(m.pauses[v.Manifest.ID]) > 0 {
		m.mu.Unlock()
		return nil, nil, errors.New("extension is paused while its installation changes")
	}
	p := m.processes[v.Manifest.ID]
	if p == nil {
		p = &process{}
		m.processes[v.Manifest.ID] = p
	}
	m.mu.Unlock()
	p.mu.Lock()
	defer p.mu.Unlock()
	m.mu.Lock()
	blocked := m.closed || len(m.pauses[v.Manifest.ID]) > 0
	m.mu.Unlock()
	if blocked {
		return nil, nil, errors.New("extension installation is changing")
	}
	current, err := m.Store.List()
	if err != nil {
		return nil, nil, err
	}
	active := false
	for _, installed := range current {
		if installed.Manifest.ID == v.Manifest.ID && installed.Enabled && installed.Manifest.Signature == v.Manifest.Signature {
			active = true
		}
	}
	if !active {
		return nil, nil, errors.New("extension changed before backend startup")
	}
	if p.session != nil && p.signature != v.Manifest.Signature {
		p.session.Close()
		p.session = nil
		p.starts = 0
	}
	if p.session != nil {
		return p, p.session, nil
	}
	if p.starts >= 2 {
		return p, nil, errors.New("extension backend failed twice; disable and re-enable it to retry")
	}
	p.starts++
	backend := v.Manifest.Backends[runtime.GOOS]
	if backend == "" {
		return p, nil, errors.New("extension has no backend for this operating system")
	}
	cmd := exec.Command(filepath.Join(m.Store.Path(v.Manifest), filepath.FromSlash(backend)))
	cmd.Dir = m.Store.Path(v.Manifest)
	cmd.Env = cleanEnvironment(os.Environ())
	cmd.Stderr = &limitedBuffer{limit: 64 << 10}
	hideProcess(cmd)
	if err := procjob.TieRequired(os.Getpid()); err != nil {
		return p, nil, fmt.Errorf("extension lifetime protection: %w", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "dinah-extension-host", Version: "1.0.0"}, nil)
	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	session, err := client.Connect(connectCtx, &mcp.CommandTransport{Command: cmd, TerminateDuration: time.Second}, nil)
	if err != nil {
		return p, nil, err
	}
	tools, err := session.ListTools(connectCtx, nil)
	if err == nil {
		err = validateTools(v.Manifest, tools)
	}
	if err != nil {
		session.Close()
		return p, nil, err
	}
	p.session = session
	p.signature = v.Manifest.Signature
	return p, session, nil
}
func cleanEnvironment(env []string) []string {
	out := []string{}
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if strings.HasPrefix(key, "DINAH_") || strings.Contains(key, "API_KEY") || strings.HasSuffix(key, "_TOKEN") {
			continue
		}
		out = append(out, entry)
	}
	return out
}
func validateTools(manifest extension.Manifest, result *mcp.ListToolsResult) error {
	if result.NextCursor != "" || len(result.Tools) != len(manifest.Actions) {
		return errors.New("backend tools differ from signed manifest")
	}
	seen := map[string]bool{}
	for _, tool := range result.Tools {
		if seen[tool.Name] {
			return errors.New("duplicate backend tool")
		}
		seen[tool.Name] = true
		found := false
		for _, a := range manifest.Actions {
			if a.Name != tool.Name {
				continue
			}
			found = true
			if canonicalJSON(a) != canonicalJSON(tool.Meta["com.parkscomputing.dinah/action"]) {
				return errors.New("backend action metadata differs from signed declaration")
			}
			for _, pair := range [][2]any{{tool.InputSchema, a.InputSchema}, {tool.OutputSchema, a.OutputSchema}} {
				if canonicalJSON(pair[0]) != canonicalJSON(pair[1]) {
					return errors.New("backend schema differs from signed declaration")
				}
			}
		}
		if !found {
			return errors.New("backend advertised an undeclared tool")
		}
	}
	return nil
}
func canonicalJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "invalid"
	}
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if decoder.Decode(&decoded) != nil {
		return "invalid"
	}
	data, _ = json.Marshal(decoded)
	return string(data)
}
func (m *Manager) Call(ctx context.Context, id, action, workspace string, input json.RawMessage) (json.RawMessage, error) {
	data, _, err := m.callSigned(ctx, id, action, workspace, input, "")
	return data, err
}

// callSigned calls an extension's action and returns its structured result
// and, when the tool gave one, its readable text (resultText).
func (m *Manager) callSigned(ctx context.Context, id, action, workspace string, input json.RawMessage, signature string) (json.RawMessage, string, error) {
	data, result, err := m.callTool(ctx, id, action, workspace, input, signature)
	if err != nil {
		return nil, "", err
	}
	return data, resultText(result, data), nil
}

// resultText is the readable text of a tool's result: its text content
// blocks, in order. The MCP specification says a tool that returns
// structured content should also return it serialized as JSON in a text
// block, for older clients, and the Go SDK's server does so when a tool
// gives no content of its own (CallToolResult.Content). Such a block is the
// structured result again rather than text for a reader, so a block whose
// text is JSON equal to the structured result is left out.
func resultText(result *mcp.CallToolResult, structured json.RawMessage) string {
	if result == nil {
		return ""
	}
	var want any
	_ = json.Unmarshal(structured, &want)
	parts := []string{}
	for _, content := range result.Content {
		block, ok := content.(*mcp.TextContent)
		if !ok || strings.TrimSpace(block.Text) == "" {
			continue
		}
		var got any
		if json.Unmarshal([]byte(block.Text), &got) == nil && reflect.DeepEqual(got, want) {
			continue
		}
		parts = append(parts, block.Text)
	}
	return strings.Join(parts, "\n\n")
}

func (m *Manager) callTool(ctx context.Context, id, action, workspace string, input json.RawMessage, signature string) (json.RawMessage, *mcp.CallToolResult, error) {
	v, err := m.Installed(id)
	if err != nil {
		return nil, nil, err
	}
	if signature != "" && signature != v.Manifest.Signature {
		return nil, nil, errors.New("extension changed since action discovery; discover it again before invoking")
	}
	var spec *extension.Action
	for _, a := range v.Manifest.Actions {
		if a.Name == action {
			copy := a
			spec = &copy
			break
		}
	}
	if spec == nil {
		return nil, nil, errors.New("extension action is not declared")
	}
	if err = applet.Validate(spec.InputSchema, string(input)); err != nil {
		return nil, nil, err
	}
	ws, err := m.Hooks.Workspace(workspace)
	if err != nil {
		return nil, nil, err
	}
	if err = m.serve(); err != nil {
		return nil, nil, err
	}
	p, session, err := m.session(ctx, v)
	if err != nil {
		return nil, nil, err
	}
	token := rand.Text()
	callCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	m.mu.Lock()
	if len(m.pauses[id]) > 0 {
		m.mu.Unlock()
		return nil, nil, errors.New("extension installation is changing")
	}
	m.scopes[token] = scope{ctx: callCtx, cancel: cancel, workspace: ws, permissions: spec.Permissions, extensionID: id, effect: spec.Effect}
	url := m.url
	m.mu.Unlock()
	defer func() { m.mu.Lock(); delete(m.scopes, token); m.mu.Unlock() }()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: action, Arguments: input, Meta: mcp.Meta{extension.ScopeMeta: extension.Scope{URL: url, Token: token, Workspace: ws}}})
	if err != nil {
		p.mu.Lock()
		if p.session == session {
			p.session = nil
			session.Close()
		}
		p.mu.Unlock()
		return nil, nil, fmt.Errorf("extension action failed without replay: %w", err)
	}
	if result.IsError {
		// GetError is server-local in the MCP SDK and always nil on clients.
		messages := []string{}
		for _, content := range result.Content {
			if message, ok := content.(*mcp.TextContent); ok && message.Text != "" {
				messages = append(messages, message.Text)
			}
		}
		if len(messages) == 0 {
			return nil, nil, errors.New("extension action reported an error")
		}
		return nil, nil, errors.New(strings.Join(messages, "\n"))
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return nil, nil, err
	}
	if err = applet.Validate(spec.OutputSchema, string(data)); err != nil {
		return nil, nil, fmt.Errorf("invalid extension output: %w", err)
	}
	return data, result, nil
}
func (m *Manager) Bundles() ([]applet.Bundle, error) {
	list, err := m.Store.List()
	if err != nil {
		return nil, err
	}
	out := []applet.Bundle{}
	for _, v := range list {
		if !v.Enabled {
			continue
		}
		if _, err = m.Installed(v.Manifest.ID); err != nil {
			// The Extensions pane reports integrity errors. One damaged package
			// must not remove healthy extensions or built-in applets.
			continue
		}
		bundle := applet.Bundle{Manifest: applet.Manifest{ID: v.Manifest.ID, Version: v.Manifest.Version, Title: v.Manifest.Title, Description: v.Manifest.Description, APIVersion: 1}, Sources: map[string][]byte{}, Handlers: map[string]applet.Handler{}}
		manifestData, _ := json.Marshal(v.Manifest)
		bundle.Sources[extension.ManifestFile] = manifestData
		for _, spec := range v.Manifest.Actions {
			spec := spec
			ref := v.Manifest.ID + "/" + spec.Name + "@1"
			caps := []string{}
			for _, p := range spec.Permissions {
				if p == "network" || p == "workspace.decorate" {
					continue
				}
				caps = append(caps, p)
			}
			// The signed effect level travels with the action, so consent for
			// agents (applet.Asks) asks only for history and network when the
			// manifest declares one, and keeps the conservative rule otherwise.
			bundle.Manifest.Actions = append(bundle.Manifest.Actions, applet.Action{Ref: ref, Title: spec.Title, Description: spec.Description, InputSchema: spec.InputSchema, OutputSchema: spec.OutputSchema, Capabilities: caps, Replay: spec.Replay, Effect: spec.Effect})
			id := v.Manifest.ID
			bundle.Handlers[ref] = func(ctx context.Context, input string) (applet.Result, error) {
				inv, ok := applet.InvocationFromContext(ctx)
				if !ok {
					return applet.Result{}, errors.New("an invocation scope is required")
				}
				data, text, err := m.callSigned(ctx, id, spec.Name, inv.Workspace, json.RawMessage(input), v.Manifest.Signature)
				return applet.Result{OutputJSON: string(data), Text: text}, err
			}
		}
		out = append(out, bundle)
	}
	return out, nil
}

// Executing counts live signed action scopes, including calls being cancelled,
// that would leave something half done if Dinah exited. A read changes
// nothing, so it is not counted: the Git pane always has one waiting for
// its repository to change, and counting it made every exit and restart
// ask (Paul, 26 October 2026). Native extension actions currently declare
// once-only replay.
func (m *Manager) Executing() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, s := range m.scopes {
		if s.effect != extension.EffectRead {
			n++
		}
	}
	return n
}
