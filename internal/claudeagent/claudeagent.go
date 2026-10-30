// Package claudeagent reaches Claude through the Claude Agent ACP adapter
// (https://github.com/agentclientprotocol/claude-agent-acp), which implements
// an ACP agent with Anthropic's Claude Agent SDK. The adapter is a Node.js
// program: Dinah installs one pinned release of it in its state
// folder and runs it with the Node.js the person has installed.
//
// The adapter is a plain ACP agent, so the connection is package acp's. This
// package adds three things: the adapter's environment, which decides who
// pays (the person's own Claude Code sign-in, or an API key they gave); the
// prefix "claude:" on every session ID, by which Dinah tells a Claude
// chat from the chats of its other agents; and the installation.
package claudeagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"studio/internal/acp"
)

// Package and Version are the adapter release Dinah runs and is tested
// against. A release is pinned, never the preview channel, and is at least
// two weeks old when chosen.
const (
	Package = "@agentclientprotocol/claude-agent-acp"
	Version = "0.87.0"
)

// MinimumNode is the oldest Node.js major version the pinned adapter
// supports, from its package.json's "engines".
const MinimumNode = 22

// Prefix begins the ID Dinah gives each Claude session.
const Prefix = "claude:"

// credentialVariables are the environment variables that choose how Claude
// Code authenticates, ahead of the person's own sign-in
// (https://code.claude.com/docs/en/authentication). They are removed from the
// adapter's environment so that a key left in the person's profile is never
// used without their choosing it.
var credentialVariables = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"}

// Dir is the folder the pinned adapter is installed in, under the state
// folder given.
func Dir(state string) string { return filepath.Join(state, "claude-agent-acp", Version) }

// Entry is the adapter's program, the package's "bin" entry.
func Entry(state string) string {
	return filepath.Join(Dir(state), "node_modules", "@agentclientprotocol", "claude-agent-acp", "dist", "index.js")
}

// Installed says whether the pinned adapter is installed under the state
// folder.
func Installed(state string) bool {
	data, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(Entry(state))), "package.json"))
	if err != nil {
		return false
	}
	var p struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &p) != nil || p.Version != Version {
		return false
	}
	_, err = os.Stat(Entry(state))
	return err == nil
}

// Install installs the pinned adapter under the state folder with npm. It
// installs into a new folder and moves it into place when npm succeeds, so an
// install that fails leaves nothing half done. Package scripts are not run:
// the adapter needs none, and the SDK's Claude Code executable comes as a
// platform package.
func Install(ctx context.Context, npm, state string) error {
	if Installed(state) {
		return nil
	}
	final := Dir(state)
	if err := os.MkdirAll(filepath.Dir(final), 0o700); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(filepath.Dir(final), "installing-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, npm, "install", "--prefix", temp, "--ignore-scripts", "--no-audit", "--no-fund", "--save-exact", Package+"@"+Version)
	cmd.Dir = temp
	cmd.Stdout, cmd.Stderr = &out, &out
	acp.HideWindow(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm could not install the Claude adapter %s: %v\n%s", Version, err, lastLines(out.String(), 12))
	}
	if _, err := os.Stat(filepath.Join(temp, "node_modules", "@agentclientprotocol", "claude-agent-acp", "dist", "index.js")); err != nil {
		return fmt.Errorf("npm finished, but the Claude adapter's program is not where its package says: %w", err)
	}
	_ = os.RemoveAll(final)
	return os.Rename(temp, final)
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// NPM is the npm that came with the Node.js given: npm.cmd beside node.exe
// on Windows, npm beside node elsewhere, as Node.js's installers place it.
// It is "" when there is none there.
func NPM(node string) string {
	name := "npm"
	if runtime.GOOS == "windows" {
		name = "npm.cmd"
	}
	p := filepath.Join(filepath.Dir(node), name)
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p
	}
	return ""
}

var nodeVersionRe = regexp.MustCompile(`^v(\d+)\.`)

// CheckNode runs node --version and says whether it is new enough for the
// adapter. It runs the program, so it is for a connection check or a start,
// never for discovery.
func CheckNode(ctx context.Context, node string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "--version")
	acp.HideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Node.js at %s did not run: %w", node, err)
	}
	version := strings.TrimSpace(string(out))
	m := nodeVersionRe.FindStringSubmatch(version)
	if m == nil {
		return version, fmt.Errorf("Node.js at %s gave an unexpected version, %q", node, version)
	}
	if major, _ := strconv.Atoi(m[1]); major < MinimumNode {
		return version, fmt.Errorf("Claude needs Node.js %d or later, and %s is %s; install a current Node.js", MinimumNode, node, version)
	}
	return version, nil
}

// Env is the adapter's environment: the one given without the credential
// variables, and with ANTHROPIC_API_KEY when apiKey is not empty. Without a
// key the SDK uses the person's Claude Code sign-in, the last and default
// entry of its documented precedence.
func Env(base []string, apiKey string) []string {
	out := make([]string, 0, len(base)+1)
	for _, kv := range base {
		name, _, _ := strings.Cut(kv, "=")
		drop := false
		for _, v := range credentialVariables {
			if name == v || (runtime.GOOS == "windows" && strings.EqualFold(name, v)) {
				drop = true
			}
		}
		if !drop {
			out = append(out, kv)
		}
	}
	if apiKey != "" {
		out = append(out, "ANTHROPIC_API_KEY="+apiKey)
	}
	return out
}

// Conn is a running adapter. It is an acp.Conn whose session IDs carry
// Prefix on Dinah's side and not on the adapter's.
type Conn struct {
	*acp.Conn
}

// Start runs the installed adapter with node in dir. apiKey, when not empty,
// is the person's Claude API key; empty uses their Claude Code sign-in.
func Start(node, state, dir, apiKey string, h acp.Handler, trace io.Writer) (*Conn, error) {
	if !Installed(state) {
		return nil, errors.New("the Claude adapter is not installed; install it in Settings > Agents")
	}
	c, err := acp.Start(node, []string{Entry(state)}, dir, prefixHandler{h}, acp.Options{Env: Env(os.Environ(), apiKey), Trace: trace})
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		c.RegisterTraceSecret(apiKey)
	}
	return &Conn{c}, nil
}

// Call sends a request with the prefix taken off its session IDs, and puts
// the prefix on the session IDs of the answer.
func (c *Conn) Call(ctx context.Context, method string, params, out any) error {
	p, err := rewrite(params, strip)
	if err != nil {
		return err
	}
	var raw json.RawMessage
	if err := c.Conn.Call(ctx, method, p, &raw); err != nil {
		return err
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	fixed, err := rewrite(raw, add)
	if err != nil {
		return err
	}
	return json.Unmarshal(fixed, out)
}

// Notify sends a notification with the prefix taken off its session IDs.
func (c *Conn) Notify(method string, params any) error {
	p, err := rewrite(params, strip)
	if err != nil {
		return err
	}
	return c.Conn.Notify(method, p)
}

// prefixHandler puts the prefix on the session IDs of what the adapter sends.
type prefixHandler struct{ h acp.Handler }

func (p prefixHandler) Notify(method string, params json.RawMessage) {
	if fixed, err := rewrite(params, add); err == nil {
		params = fixed
	}
	// The adapter's account report is about the adapter's own sign-in, not
	// a session's, so the app is told it came from Claude.
	if method == AuthStatusMethod {
		method = AuthStatusFromClaude
	}
	p.h.Notify(method, params)
}

// AuthStatusMethod is the adapter's account report: which identity it runs
// on, such as a Claude subscription or an API key. The adapter advertises it
// with agentCapabilities._meta.authStatus and pushes it whenever it changes,
// as its auth-status module describes; it is an interim extension until ACP
// has first-class fields for it. AuthStatusFromClaude is the method the app
// receives it under.
const (
	AuthStatusMethod     = "_auth/status_update"
	AuthStatusFromClaude = "claude/_auth/status_update"
)

func (p prefixHandler) Request(ctx context.Context, method string, params json.RawMessage) (any, *acp.Error) {
	if fixed, err := rewrite(params, add); err == nil {
		params = fixed
	}
	return p.h.Request(ctx, method, params)
}

func add(id string) string {
	if strings.HasPrefix(id, Prefix) {
		return id
	}
	return Prefix + id
}

func strip(id string) string { return strings.TrimPrefix(id, Prefix) }

// rewrite applies change to every "sessionId" string in a JSON value,
// however deep, and to every "subagentSessionId", which the ACP subagent
// RFD's subagent_spawned and subagent_state_update use for a child session
// whose own updates then carry it as their "sessionId". It returns the
// value as JSON. Numbers keep their text.
func rewrite(v any, change func(string) string) (json.RawMessage, error) {
	data, ok := v.(json.RawMessage)
	if !ok {
		var err error
		if data, err = json.Marshal(v); err != nil {
			return nil, err
		}
	}
	if len(data) == 0 || (!bytes.Contains(data, []byte(`"sessionId"`)) && !bytes.Contains(data, []byte(`"subagentSessionId"`))) {
		return data, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	walk(tree, change)
	return json.Marshal(tree)
}

func walk(v any, change func(string) string) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if s, ok := child.(string); ok && (k == "sessionId" || k == "subagentSessionId") {
				x[k] = change(s)
				continue
			}
			walk(child, change)
		}
	case []any:
		for _, child := range x {
			walk(child, change)
		}
	}
}
