package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"studio/internal/acp"
	"studio/internal/agents"
	"studio/internal/claudeagent"
	"studio/internal/credentials"
)

// Claude's own work in the window: its Node.js and adapter, its API key, and
// the sign-in it leaves to the person's own Claude Code.

func init() {
	registerProviderModule("claude", providerModule{
		Info:    func(a *App, p *AgentProviderInfo, workspaces []string) { a.claudeInfo(p, workspaces) },
		Adapter: claudeAdapter,
		Picture: claudePicture,
		Judge:   acpJudgeModule(),
		SignIn: func(*App) error {
			// Dinah never offers a Claude login of its own
			// (docs/proposals/claude-agent-integration.md, Authentication).
			return errors.New(product.Name + " never signs you in to Claude itself. Run claude in a terminal and sign in there with /login, or store an API key here")
		},
	})
}

// claudeKeys keeps the person's Claude API key, in the system keychain;
// tests replace it, so that they never touch the person's keychain.
var claudeKeys = struct {
	get    func() (string, error)
	set    func(string) error
	remove func() error
}{
	get:    func() (string, error) { return credentials.Get(credentials.KeyClaudeAPIKey) },
	set:    func(key string) error { return credentials.Set(credentials.KeyClaudeAPIKey, key) },
	remove: func() error { return credentials.Remove(credentials.KeyClaudeAPIKey) },
}

// claudeTools are the adapter's operations that run a program: npm, which
// finds and installs a release, and Node.js, which says its version. Tests
// replace them, so that they run neither.
var claudeTools = struct {
	latest    func(ctx context.Context, npm, state string) (claudeagent.Release, error)
	install   func(ctx context.Context, npm, state, version string) error
	checkNode func(ctx context.Context, node string, minimum int) (string, error)
}{
	latest:    claudeagent.Latest,
	install:   claudeagent.InstallRelease,
	checkNode: claudeagent.CheckNodeFor,
}

// claudeInstallWithin is how long npm may take to install a release.
const claudeInstallWithin = 10 * time.Minute

// claudeAdapterWork is the installing of the adapter's releases
// (docs/proposals/claude-adapter-updates.md). One install runs at a time, so
// that the one the app starts by itself and one the person asks for never
// write the same folder together.
type claudeAdapterWork struct {
	installs sync.Mutex

	mu sync.Mutex
	// installing says the app is installing the tested release by itself,
	// after an update that moved it, and failed is why that install failed.
	installing bool
	failed     string
}

func (w *claudeAdapterWork) set(installing bool, failed string) {
	w.mu.Lock()
	w.installing, w.failed = installing, failed
	w.mu.Unlock()
}

// claudeInfo fills in Claude's entry: Node.js, which runs the adapter, the
// adapter, and the billing. It runs nothing.
func (a *App) claudeInfo(p *AgentProviderInfo, workspaces []string) {
	a.mu.Lock()
	p.ExecutableOverride = a.settings.ProviderExecutables["claude"]
	p.Pinned = append([]string{}, a.settings.ProviderPinned["claude"]...)
	chosen := a.settings.ClaudeAdapter
	a.mu.Unlock()
	var overrideError error
	if p.ExecutableOverride != "" {
		p.Path, overrideError = agents.CheckExecutable(p.ExecutableOverride)
	} else {
		p.Path = agents.FindExecutable("node", workspaces)
	}
	work := &a.claudeAdapters
	work.mu.Lock()
	installing, failed := work.installing, work.failed
	work.mu.Unlock()
	_, keyErr := claudeKeys.get()
	state, tested := configDir(), claudeagent.Version
	inUse := claudeagent.InUse(state, chosen)
	setup := &ClaudeSetup{
		AdapterVersion: tested, AdapterInstalled: claudeagent.Installed(state),
		AdapterInUse: inUse, AdapterNewer: claudeagent.Newer(inUse, tested), AdapterInstalling: installing,
		APIKey: keyErr == nil,
	}
	setup.CanInstall = p.Path != "" && claudeagent.NPM(p.Path) != ""
	p.Claude = setup
	p.Installed = p.Path != "" && inUse != ""
	p.Available = p.Installed
	billing := "Claude uses your own Claude Code sign-in; sign in by running claude in a terminal once."
	if setup.APIKey {
		billing = "Claude is billed to the API key you stored."
	}
	switch {
	case overrideError != nil:
		p.Status, p.Detail = "invalid-path", overrideError.Error()+". Choose another Node.js or use automatic discovery."
	case p.Path == "":
		p.Status = "runtime-missing"
		p.Detail = fmt.Sprintf("Claude runs on Node.js %d or later, which was not found. Install Node.js, or choose its executable.", claudeagent.MinimumNode)
	case inUse == "":
		p.Status = "adapter-missing"
		p.Detail = "Node.js was found. Install the Claude adapter " + tested + " to use Claude."
		if !setup.CanInstall {
			p.Detail = "Node.js was found, but npm, which installs the Claude adapter, is not beside it. Reinstall Node.js with npm."
		}
	case setup.AdapterNewer:
		p.Status = "detected"
		p.Detail = "Node.js and the Claude adapter " + inUse + " are installed. You chose that release, which is newer than " + tested + ", the one " + product.Name + " was tested with. " + billing
	case inUse != tested:
		// An update moved the tested release, and Claude runs on the one it
		// had until the tested one is in place.
		p.Status = "detected"
		switch {
		case installing:
			p.Detail = "Claude runs on the adapter " + inUse + " while " + product.Name + " installs " + tested + ", which Claude uses from its next start. " + billing
		case failed != "":
			p.Detail = "Claude runs on the adapter " + inUse + ", because " + tested + " could not be installed: " + failed + " Install it here to try again."
		case !setup.CanInstall:
			p.Detail = "Claude runs on the adapter " + inUse + ". npm, which installs " + tested + ", is not beside Node.js; reinstall Node.js with npm."
		default:
			p.Detail = "Claude runs on the adapter " + inUse + ". Install " + tested + " to bring it up to date."
		}
	default:
		p.Status = "detected"
		p.Detail = "Node.js and the Claude adapter " + tested + " are installed. " + billing
	}
}

// claudeRow is Claude's entry in the list of agents as it is now.
func (a *App) claudeRow() AgentProviderInfo { return a.providerInfoOf("claude") }

// claudeNode is the Node.js Claude runs on and the npm beside it, or why a
// release cannot be installed.
func (a *App) claudeNode() (node, npm string, err error) {
	info := a.claudeRow()
	if info.Path == "" || info.Claude == nil {
		return "", "", fmt.Errorf("install Node.js %d or later first", claudeagent.MinimumNode)
	}
	if npm = claudeagent.NPM(info.Path); npm == "" {
		return "", "", errors.New("npm was not found beside Node.js; reinstall Node.js with npm")
	}
	return info.Path, npm, nil
}

// installClaudeRelease installs one release of the adapter, after checking
// that Node.js is new enough for it. A release that is already installed
// costs nothing.
func (a *App) installClaudeRelease(ctx context.Context, node, npm, version string, minimum int) error {
	a.claudeAdapters.installs.Lock()
	defer a.claudeAdapters.installs.Unlock()
	return a.installClaudeReleaseLocked(ctx, node, npm, version, minimum)
}

// installClaudeReleaseLocked is installClaudeRelease for a caller that has
// the installs lock, and keeps it until it has saved what it installed.
func (a *App) installClaudeReleaseLocked(ctx context.Context, node, npm, version string, minimum int) error {
	if _, err := claudeTools.checkNode(ctx, node, minimum); err != nil {
		return err
	}
	return claudeTools.install(ctx, npm, configDir(), version)
}

// InstallClaudeAdapter installs the pinned Claude adapter with the npm that
// came with Node.js. The person asks for it in Settings > Agents.
func (a *App) InstallClaudeAdapter() (AgentProviderInfo, error) {
	node, npm, err := a.claudeNode()
	if err != nil {
		return a.claudeRow(), err
	}
	ctx, cancel := context.WithTimeout(context.Background(), claudeInstallWithin)
	defer cancel()
	if err := a.installClaudeRelease(ctx, node, npm, claudeagent.Version, claudeagent.MinimumNode); err != nil {
		return a.claudeRow(), err
	}
	a.claudeAdapters.set(false, "")
	return a.claudeRow(), nil
}

// catchUpClaudeAdapter brings the adapter up to the tested release by itself
// after an update of the app that moved it, so that the person does not have
// to set Claude up again (docs/proposals/claude-adapter-updates.md). Claude
// runs on the release it had meanwhile (claudeagent.InUse) and takes the
// tested one at its next start. Nothing is installed for a person who never
// set Claude up. It runs once, as the app starts.
func (a *App) catchUpClaudeAdapter() {
	if !agentProviderKnown("claude") {
		return
	}
	state, tested := configDir(), claudeagent.Version
	a.mu.Lock()
	chosen := a.settings.ClaudeAdapter
	a.mu.Unlock()
	if chosen != "" && !claudeagent.Newer(chosen, tested) {
		// The tested release caught up with the one the person chose.
		if err := a.updateSettings(func(s *Settings) error { s.ClaudeAdapter = ""; return nil }); err != nil {
			logf("core", "warning", "the choice of a Claude adapter could not be cleared: %v", err)
		}
	}
	if claudeagent.Installed(state) {
		// Nothing has started an earlier release in this run, since a start
		// takes the tested release or the chosen one when the tested one is
		// installed, so the others can go. The lock keeps a release the
		// person is choosing at this moment out of it.
		a.claudeAdapters.installs.Lock()
		a.mu.Lock()
		chosen = a.settings.ClaudeAdapter
		a.mu.Unlock()
		claudeagent.Prune(state, tested, chosen)
		a.claudeAdapters.installs.Unlock()
		return
	}
	if len(claudeagent.Releases(state)) == 0 {
		return
	}
	node, npm, err := a.claudeNode()
	if err != nil {
		return
	}
	a.claudeAdapters.set(true, "")
	ctx, cancel := context.WithTimeout(context.Background(), claudeInstallWithin)
	defer cancel()
	if err := a.installClaudeRelease(ctx, node, npm, tested, claudeagent.MinimumNode); err != nil {
		a.claudeAdapters.set(false, firstSentence(err.Error()))
		logf("core", "error", "the Claude adapter %s could not be installed after the update: %v", tested, err)
		return
	}
	a.claudeAdapters.set(false, "")
	logf("core", "info", "installed the Claude adapter %s after the update", tested)
}

// firstSentence is the first line of a message, ending in a full stop.
func firstSentence(message string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(message), "\n")
	line = strings.TrimSpace(line)
	if line != "" && !strings.HasSuffix(line, ".") {
		line += "."
	}
	return line
}

// ClaudeAdapterRelease is npm's newest release of the Claude adapter, as
// Settings > Agents shows it.
type ClaudeAdapterRelease struct {
	Version string `json:"version"`
	// Released is when npm published the release, in RFC 3339, or "" when
	// npm did not say.
	Released string `json:"released,omitempty"`
	// Tested is the release the app was tested with, and Newer whether the
	// newest one is later, so that the person can choose it.
	Tested string `json:"tested"`
	Newer  bool   `json:"newer"`
}

// NewestClaudeAdapter asks npm which release of the adapter is its newest.
// That is the release under npm's "latest" tag, never a preview. The person
// asks for it in Settings > Agents; the app never asks by itself.
func (a *App) NewestClaudeAdapter() (ClaudeAdapterRelease, error) {
	_, npm, err := a.claudeNode()
	if err != nil {
		return ClaudeAdapterRelease{}, err
	}
	r, err := claudeTools.latest(context.Background(), npm, configDir())
	if err != nil {
		return ClaudeAdapterRelease{}, err
	}
	out := ClaudeAdapterRelease{Version: r.Version, Tested: claudeagent.Version, Newer: claudeagent.Newer(r.Version, claudeagent.Version)}
	if !r.Released.IsZero() {
		out.Released = r.Released.UTC().Format(time.RFC3339)
	}
	return out, nil
}

// UseClaudeAdapter installs npm's newest release of the adapter and runs
// Claude on it from then on, in place of the tested release. version is the
// release the person was shown; it is used only while it is still npm's
// newest and is later than the tested one, so the page can ask for no other
// release. Claude is stopped so that its next start takes the change, which
// ends its open chats' connections.
func (a *App) UseClaudeAdapter(version string) (AgentProviderInfo, error) {
	version = strings.TrimSpace(version)
	if !claudeagent.ValidRelease(version) {
		return a.claudeRow(), fmt.Errorf("%q is not a release of the Claude adapter", version)
	}
	if !claudeagent.Newer(version, claudeagent.Version) {
		return a.claudeRow(), fmt.Errorf("the Claude adapter %s is not newer than %s, the release %s was tested with", version, claudeagent.Version, product.Name)
	}
	node, npm, err := a.claudeNode()
	if err != nil {
		return a.claudeRow(), err
	}
	ctx, cancel := context.WithTimeout(context.Background(), claudeInstallWithin)
	defer cancel()
	newest, err := claudeTools.latest(ctx, npm, configDir())
	if err != nil {
		return a.claudeRow(), err
	}
	if newest.Version != version {
		return a.claudeRow(), fmt.Errorf("npm's newest Claude adapter is now %s and not %s; look for the newest release again", newest.Version, version)
	}
	minimum := newest.Node
	if minimum == 0 {
		minimum = claudeagent.MinimumNode
	}
	// The lock is kept until the choice is saved, so that the tidying at the
	// app's start never takes the release for one nobody uses.
	previous := ""
	a.claudeAdapters.installs.Lock()
	err = a.installClaudeReleaseLocked(ctx, node, npm, version, minimum)
	if err == nil {
		err = a.updateSettings(func(s *Settings) error { previous, s.ClaudeAdapter = s.ClaudeAdapter, version; return nil })
	}
	a.claudeAdapters.installs.Unlock()
	if err != nil {
		return a.claudeRow(), err
	}
	a.AgentStop("claude")
	a.dropClaudeRelease(previous)
	logf("core", "info", "the person chose the Claude adapter %s over the tested %s", version, claudeagent.Version)
	return a.claudeRow(), nil
}

// UseTestedClaudeAdapter returns Claude to the tested release of the adapter,
// installing it first when it is not there. Claude is stopped so that its
// next start takes the change.
func (a *App) UseTestedClaudeAdapter() (AgentProviderInfo, error) {
	if !claudeagent.Installed(configDir()) {
		if _, err := a.InstallClaudeAdapter(); err != nil {
			return a.claudeRow(), err
		}
	}
	previous := ""
	if err := a.updateSettings(func(s *Settings) error { previous, s.ClaudeAdapter = s.ClaudeAdapter, ""; return nil }); err != nil {
		return a.claudeRow(), err
	}
	a.AgentStop("claude")
	a.dropClaudeRelease(previous)
	logf("core", "info", "the person returned to the tested Claude adapter %s", claudeagent.Version)
	return a.claudeRow(), nil
}

// dropClaudeRelease removes a release the person chose before and no longer
// uses. Claude was stopped first, and a file its ending process still has
// open may be left behind; the next start of the app removes the rest
// (catchUpClaudeAdapter).
func (a *App) dropClaudeRelease(version string) {
	a.mu.Lock()
	chosen := a.settings.ClaudeAdapter
	a.mu.Unlock()
	if version == "" || version == chosen || version == claudeagent.Version {
		return
	}
	_ = claudeagent.Remove(configDir(), version)
}

// SetClaudeAPIKey stores the person's Claude API key, which bills Claude to
// it from then on; an empty key removes it, so that Claude uses their own
// Claude Code sign-in. Claude is stopped so that its next start takes the
// change, which ends its open chats' connections.
func (a *App) SetClaudeAPIKey(key string) (AgentProviderInfo, error) {
	key = strings.TrimSpace(key)
	var err error
	if key == "" {
		err = claudeKeys.remove()
	} else {
		err = claudeKeys.set(key)
	}
	if err != nil {
		return AgentProviderInfo{}, err
	}
	a.AgentStop("claude")
	return a.claudeRow(), nil
}

// The adapter that starts and watches the provider (provider_lifecycle.go).
var claudeAdapter = &providerAdapter{
	id:       "claude",
	starting: "Starting Claude",
	stopped:  "Claude stopped",
	// The adapter reports its own release as the agent's version, so the
	// connection says which release a problem was seen on.
	readyDetail: func(init acp.InitializeResult) string {
		if init.AgentInfo != nil && claudeagent.ValidRelease(init.AgentInfo.Version) {
			return "Claude is connected, on the adapter " + init.AgentInfo.Version
		}
		return "Claude is connected"
	},
	initWithin: time.Minute,
	initError:  "start Claude",
	prepare: func(a *App) (providerStart, error) {
		info := a.claudeRow()
		if !info.Available {
			return providerStart{}, errors.New(info.Detail)
		}
		state, version := configDir(), info.Claude.AdapterInUse
		if _, err := claudeTools.checkNode(context.Background(), info.Path, claudeagent.NodeNeeded(state, version)); err != nil {
			return providerStart{}, err
		}
		key := ""
		if info.Claude.APIKey {
			var err error
			if key, err = claudeKeys.get(); err != nil {
				return providerStart{}, fmt.Errorf("the Claude API key could not be read from the keychain: %w", err)
			}
		}
		neutral, err := agentStartupDir()
		if err != nil {
			return providerStart{}, err
		}
		return providerStart{
			launch: func(handler acp.Handler) (agentConnection, error) {
				return claudeagent.StartRelease(info.Path, state, version, neutral, key, handler, nil)
			},
			ready: func(acp.InitializeResult) {
				logf("core", "info", "Claude started on the adapter %s", version)
			},
		}, nil
	},
}

func (a *App) claudeAccountReport(params json.RawMessage) {
	var n struct {
		AuthStatus struct {
			Kind  string `json:"kind"`
			Label string `json:"label"`
		} `json:"authStatus"`
	}
	if json.Unmarshal(params, &n) != nil || n.AuthStatus.Kind == "" {
		return
	}
	acct := ProviderAccount{Provider: "claude", Kind: n.AuthStatus.Kind, Label: strings.TrimSpace(n.AuthStatus.Label),
		Subscription: n.AuthStatus.Kind == "account"}
	a.reportAccount(acct)
}
