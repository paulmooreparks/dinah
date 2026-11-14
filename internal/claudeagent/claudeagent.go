// Package claudeagent reaches Claude through the Claude Agent ACP adapter
// (https://github.com/agentclientprotocol/claude-agent-acp), which implements
// an ACP agent with Anthropic's Claude Agent SDK. The adapter is a Node.js
// program: Dinah installs one pinned release of it in its state
// folder and runs it with the Node.js the person has installed. The person
// may choose npm's newest release over the pinned one
// (docs/proposals/claude-adapter-updates.md).
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
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"studio/internal/acp"
)

// Package and Version are the adapter release Dinah runs and is tested
// against. A release is pinned, never the preview channel, and is at least
// one week old when chosen and has passed the live test
// (claude_live_test.go). A new model is a reason to move the pin as soon as
// both are true. Paul set the week on 10 November 2026, where the rule had
// been two weeks, and waived it for 0.101.0, which was five days old that
// day, because it is the first release whose Claude Code offers Fable 5.2
// (docs/proposals/claude-adapter-updates.md).
const (
	Package = "@agentclientprotocol/claude-agent-acp"
	Version = "0.101.0"
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

// releasePattern is a release's number as npm gives it: three numbers and no
// pre-release part. A number becomes a folder's name and an argument of npm,
// so nothing else is taken, and a preview release never matches.
var releasePattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})[.](0|[1-9][0-9]{0,8})[.](0|[1-9][0-9]{0,8})$`)

// ValidRelease says whether version is a release's number.
func ValidRelease(version string) bool { return releasePattern.MatchString(version) }

// Newer says whether release a is later than release b. A number that is not
// a release's is later than nothing.
func Newer(a, b string) bool {
	x, y := releasePattern.FindStringSubmatch(a), releasePattern.FindStringSubmatch(b)
	if x == nil || y == nil {
		return false
	}
	for i := 1; i <= 3; i++ {
		m, _ := strconv.Atoi(x[i])
		n, _ := strconv.Atoi(y[i])
		if m != n {
			return m > n
		}
	}
	return false
}

// Dir is the folder the pinned adapter is installed in, under the state
// folder given. DirOf is the folder of any release: each release has a folder
// of its own, so that one can run while another is installed.
func Dir(state string) string { return DirOf(state, Version) }

func DirOf(state, version string) string {
	return filepath.Join(state, "claude-agent-acp", version)
}

// Entry is the pinned adapter's program, the package's "bin" entry, and
// EntryOf that of any release.
func Entry(state string) string { return EntryOf(state, Version) }

func EntryOf(state, version string) string {
	return filepath.Join(packageDir(DirOf(state, version)), "dist", "index.js")
}

// packageDir is the adapter's package in the folder npm installed it under.
func packageDir(dir string) string {
	return filepath.Join(dir, "node_modules", "@agentclientprotocol", "claude-agent-acp")
}

// manifest is what a release's package.json says that Dinah reads.
type manifest struct {
	Version string `json:"version"`
	Engines struct {
		Node string `json:"node"`
	} `json:"engines"`
}

func readManifest(state, version string) (manifest, bool) {
	var m manifest
	data, err := os.ReadFile(filepath.Join(packageDir(DirOf(state, version)), "package.json"))
	if err != nil || json.Unmarshal(data, &m) != nil {
		return m, false
	}
	return m, true
}

// Installed says whether the pinned adapter is installed under the state
// folder, and InstalledRelease whether the release given is.
func Installed(state string) bool { return InstalledRelease(state, Version) }

func InstalledRelease(state, version string) bool {
	if !ValidRelease(version) {
		return false
	}
	if m, ok := readManifest(state, version); !ok || m.Version != version {
		return false
	}
	_, err := os.Stat(EntryOf(state, version))
	return err == nil
}

// Releases are the releases installed under the state folder, the latest
// first.
func Releases(state string) []string {
	entries, _ := os.ReadDir(filepath.Join(state, "claude-agent-acp"))
	out := []string{}
	for _, e := range entries {
		if e.IsDir() && InstalledRelease(state, e.Name()) {
			out = append(out, e.Name())
		}
	}
	sort.Slice(out, func(i, j int) bool { return Newer(out[i], out[j]) })
	return out
}

// InUse is the release a start of Claude runs, or "" when none is installed.
// It is the release the person chose, while that one is installed and later
// than the pinned one; otherwise the pinned one; and, while the pinned one is
// not installed, the latest earlier release that is. The last case is the
// time after an update of Dinah that moved the pin: Claude runs on the release
// it had until the pinned one is in place. A choice the pin has caught up
// with counts for nothing, so the tested release wins from then on.
func InUse(state, chosen string) string {
	if Newer(chosen, Version) && InstalledRelease(state, chosen) {
		return chosen
	}
	if Installed(state) {
		return Version
	}
	for _, v := range Releases(state) {
		if Newer(Version, v) {
			return v
		}
	}
	return ""
}

// nodeRange reads the form the adapter's releases give their oldest Node.js
// in, ">=22".
var nodeRange = regexp.MustCompile(`^>= ?([0-9]{1,4})`)

func nodeOf(engines string) int {
	if m := nodeRange.FindStringSubmatch(strings.TrimSpace(engines)); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// NodeNeeded is the oldest Node.js major version an installed release
// supports, from its package.json's "engines". It is MinimumNode for the
// pinned release and for a release that gives the range in another form.
func NodeNeeded(state, version string) int {
	if version != Version {
		if m, ok := readManifest(state, version); ok {
			if n := nodeOf(m.Engines.Node); n > 0 {
				return n
			}
		}
	}
	return MinimumNode
}

// Install installs the pinned adapter under the state folder with npm.
func Install(ctx context.Context, npm, state string) error {
	return InstallRelease(ctx, npm, state, Version)
}

// InstallRelease installs one release under the state folder with npm. It
// installs into a new folder and moves it into place when npm succeeds, so an
// install that fails leaves nothing half done, and a release that is running
// from its own folder is not touched. Package scripts are not run: the
// adapter needs none, and the SDK's Claude Code executable comes as a
// platform package.
func InstallRelease(ctx context.Context, npm, state, version string) error {
	if !ValidRelease(version) {
		return fmt.Errorf("%q is not a release of the Claude adapter", version)
	}
	if InstalledRelease(state, version) {
		return nil
	}
	final := DirOf(state, version)
	if err := os.MkdirAll(filepath.Dir(final), 0o700); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(filepath.Dir(final), "installing-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, npm, "install", "--prefix", temp, "--ignore-scripts", "--no-audit", "--no-fund", "--save-exact", Package+"@"+version)
	cmd.Dir = temp
	cmd.Stdout, cmd.Stderr = &out, &out
	acp.HideWindow(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm could not install the Claude adapter %s: %v\n%s", version, err, lastLines(out.String(), 12))
	}
	if _, err := os.Stat(filepath.Join(packageDir(temp), "dist", "index.js")); err != nil {
		return fmt.Errorf("npm finished, but the Claude adapter's program is not where its package says: %w", err)
	}
	_ = os.RemoveAll(final)
	return os.Rename(temp, final)
}

// removingPrefix begins the name of a folder Remove moved aside.
const removingPrefix = "removing-"

// Remove removes one release's folder. The folder is moved aside whole before
// anything in it is deleted. A removal that cannot finish, because a program
// that is ending still has a file of the release open, therefore never leaves
// a folder that looks like an installed release and lacks files: either the
// release stays as it was and the error says so, or it is gone, and what
// could not be deleted waits under another name for Prune.
func Remove(state, version string) error {
	if !ValidRelease(version) {
		return fmt.Errorf("%q is not a release of the Claude adapter", version)
	}
	dir := DirOf(state, version)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	aside := filepath.Join(filepath.Dir(dir), fmt.Sprintf("%s%s-%d", removingPrefix, version, time.Now().UnixNano()))
	if err := os.Rename(dir, aside); err != nil {
		return err
	}
	_ = os.RemoveAll(aside)
	return nil
}

// Prune removes every release's folder under the state folder except those
// of the releases to keep, and what an earlier Remove left behind. Only a
// folder named as a release, or one Remove moved aside, is removed. The
// caller sees to it that none of the removed releases is running.
func Prune(state string, keep ...string) {
	parent := filepath.Join(state, "claude-agent-acp")
	entries, _ := os.ReadDir(parent)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), removingPrefix) {
			_ = os.RemoveAll(filepath.Join(parent, e.Name()))
			continue
		}
		if !ValidRelease(e.Name()) {
			continue
		}
		kept := false
		for _, k := range keep {
			kept = kept || k == e.Name()
		}
		if !kept {
			_ = Remove(state, e.Name())
		}
	}
}

// Release is a release of the adapter as npm's registry describes it: its
// number, when it was published, and the oldest Node.js major version it
// supports, which is 0 when the registry gives the range in a form other than
// ">=22".
type Release struct {
	Version  string
	Released time.Time
	Node     int
}

// Latest asks npm which release its "latest" tag is on, which is the newest
// one that is not a preview. It runs npm view, whose --json output and whose
// version, time and engines fields npm documents
// (https://docs.npmjs.com/cli/commands/npm-view), in the adapters' folder
// under the state folder, so that no project's npm settings apply.
func Latest(ctx context.Context, npm, state string) (Release, error) {
	dir := filepath.Join(state, "claude-agent-acp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Release{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	var out, problems bytes.Buffer
	cmd := exec.CommandContext(ctx, npm, "view", Package+"@latest", "version", "time", "engines", "--json")
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = &out, &problems
	acp.HideWindow(cmd)
	if err := cmd.Run(); err != nil {
		return Release{}, fmt.Errorf("npm could not say which Claude adapter is the newest: %v\n%s", err, lastLines(problems.String()+out.String(), 6))
	}
	return parseLatest(out.Bytes())
}

// parseLatest reads what npm view printed.
func parseLatest(data []byte) (Release, error) {
	var v struct {
		Version string            `json:"version"`
		Time    map[string]string `json:"time"`
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return Release{}, fmt.Errorf("npm's answer about the Claude adapter could not be read: %w", err)
	}
	if !ValidRelease(v.Version) {
		return Release{}, fmt.Errorf("npm gave %q as the newest Claude adapter, which is not a release's number", v.Version)
	}
	r := Release{Version: v.Version, Node: nodeOf(v.Engines.Node)}
	if t, err := time.Parse(time.RFC3339, v.Time[v.Version]); err == nil {
		r.Released = t
	}
	return r, nil
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
// pinned adapter. It runs the program, so it is for a connection check or a
// start, never for discovery.
func CheckNode(ctx context.Context, node string) (string, error) {
	return CheckNodeFor(ctx, node, MinimumNode)
}

// CheckNodeFor is CheckNode for a release whose oldest Node.js major version
// is minimum.
func CheckNodeFor(ctx context.Context, node string, minimum int) (string, error) {
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
	if major, _ := strconv.Atoi(m[1]); major < minimum {
		return version, fmt.Errorf("Claude needs Node.js %d or later, and %s is %s; install a current Node.js", minimum, node, version)
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
	steering atomic.Bool
}

// Start runs the pinned adapter with node in dir. apiKey, when not empty, is
// the person's Claude API key; empty uses their Claude Code sign-in.
func Start(node, state, dir, apiKey string, h acp.Handler, trace io.Writer) (*Conn, error) {
	return StartRelease(node, state, Version, dir, apiKey, h, trace)
}

// StartRelease is Start for the installed release given, which InUse chooses.
func StartRelease(node, state, version, dir, apiKey string, h acp.Handler, trace io.Writer) (*Conn, error) {
	if !InstalledRelease(state, version) {
		return nil, errors.New("the Claude adapter is not installed; install it in Settings > Agents")
	}
	c, err := acp.Start(node, []string{EntryOf(state, version)}, dir, prefixHandler{h}, acp.Options{Env: Env(os.Environ(), apiKey), Trace: trace})
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		c.RegisterTraceSecret(apiKey)
	}
	return &Conn{Conn: c}, nil
}

// Call sends a request with the prefix taken off its session IDs, and puts
// the prefix on the session IDs of the answer.
func (c *Conn) Call(ctx context.Context, method string, params, out any) error {
	if method == "initialize" {
		c.steering.Store(false)
	}
	p, err := rewrite(params, strip)
	if err != nil {
		return err
	}
	var raw json.RawMessage
	if err := c.Conn.Call(ctx, method, p, &raw); err != nil {
		return err
	}
	if method == "initialize" {
		var offered struct {
			Meta struct {
				Steering struct {
					Supported bool `json:"supported"`
				} `json:"steering"`
			} `json:"_meta"`
		}
		err := json.Unmarshal(raw, &offered)
		c.steering.Store(err == nil && offered.Meta.Steering.Supported)
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
