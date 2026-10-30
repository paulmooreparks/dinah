package ollamaagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"studio/internal/acp"
)

// server is an MCP server the host gives a conversation, such as Dinah's
// own applet server. Only servers run as a command are
// supported, as Dinah gives no other kind.
type server struct {
	Name    string
	Command string
	Args    []string
	Env     []string
}

func (s server) key() string {
	return s.Name + "\x00" + s.Command + "\x00" + strings.Join(s.Args, "\x00")
}

var serverName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)

func serversOf(list []any) ([]server, error) {
	var out []server
	for _, v := range list {
		var p struct {
			Name    string   `json:"name"`
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
			Env     []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"env"`
		}
		if err := acp.Decode(v, &p); err != nil {
			return nil, err
		}
		if !serverName.MatchString(p.Name) || strings.Contains(p.Name, "__") {
			return nil, fmt.Errorf("the MCP server name %q cannot name an Ollama tool", p.Name)
		}
		if p.Type != "" && p.Type != "stdio" {
			return nil, fmt.Errorf("Ollama conversations support only command MCP servers, not %q", p.Type)
		}
		if p.Command == "" {
			return nil, errors.New("an MCP server needs a command")
		}
		s := server{Name: p.Name, Command: p.Command, Args: p.Args}
		for _, e := range p.Env {
			s.Env = append(s.Env, e.Name+"="+e.Value)
		}
		out = append(out, s)
	}
	return out, nil
}

// appletServer is a running MCP server with its tools.
type appletServer struct {
	mu       sync.Mutex
	process  *appletProcess
	tools    []*mcp.Tool
	readOnly map[string]bool
}

func (a *appletServer) close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.process != nil {
		_ = a.process.close()
		a.process = nil
	}
}

// running starts an MCP server the first time it is needed and keeps it for
// every conversation that is given the same one.
func (c *Conn) running(ctx context.Context, s server) (*appletServer, error) {
	c.mu.Lock()
	a := c.applets[s.key()]
	if a == nil {
		a = &appletServer{}
		c.applets[s.key()] = a
	}
	c.mu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.process != nil {
		return a, nil
	}
	p, err := startAppletProcess(s, closePhases)
	if err != nil {
		return nil, fmt.Errorf("the applet server %s could not start: %w", s.Name, err)
	}
	tools, err := p.connect(ctx, s.Name, appletStartWithin)
	if err != nil {
		return nil, err
	}
	a.process, a.tools, a.readOnly = p, tools, map[string]bool{}
	for _, t := range tools {
		a.readOnly[t.Name] = t.Annotations != nil && t.Annotations.ReadOnlyHint
	}
	return a, nil
}

// appletTools are the tools of a conversation's MCP servers, each named
// <server>__<tool>.
func (c *Conn) appletTools(ctx context.Context, servers []server) ([]any, error) {
	var out []any
	for _, s := range servers {
		a, err := c.running(ctx, s)
		if err != nil {
			return nil, err
		}
		a.mu.Lock()
		for _, t := range a.tools {
			schema, err := json.Marshal(t.InputSchema)
			if err != nil {
				continue
			}
			out = append(out, map[string]any{"type": "function", "function": map[string]any{"name": s.Name + "__" + t.Name, "description": t.Description, "parameters": json.RawMessage(schema)}})
		}
		a.mu.Unlock()
	}
	return out, nil
}

// callApplet runs one applet tool. A tool that changes something asks the
// person first, unless the person allowed applet actions in Settings; one
// marked read-only does not.
func (c *Conn) callApplet(ctx context.Context, t turnTools, id, name, tool string, args map[string]any) (string, bool) {
	c.mu.Lock()
	s := c.sessions[t.session]
	var found *server
	approvals := ""
	if s != nil {
		approvals = s.approvals
		for i := range s.servers {
			if s.servers[i].Name == name {
				found = &s.servers[i]
			}
		}
	}
	c.mu.Unlock()
	if found == nil {
		return "There is no applet server named " + name + ".", false
	}
	a, err := c.running(ctx, *found)
	if err != nil {
		return err.Error(), false
	}
	a.mu.Lock()
	readOnly, known := a.readOnly[tool]
	p := a.process
	a.mu.Unlock()
	if !known || p == nil {
		return "The applet server " + name + " has no tool named " + tool + ".", false
	}
	// A tool that changes something asks, unless applet actions were allowed
	// in Settings or the person's answer in this chat already covers it.
	if !readOnly && approvals != "allow" && approvals != "all" {
		heading, _ := title(name+"__"+tool, args)
		key := "applet\x00" + name + "__" + tool
		if t.commands == ApprovalsChat {
			key = "applet"
		}
		if ok, why := t.allow(ctx, id, key, heading, args); !ok {
			return why, false
		}
	}
	res, err := p.call(ctx, tool, args)
	if err != nil {
		return err.Error(), false
	}
	var b strings.Builder
	for _, content := range res.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n"), !res.IsError
}
