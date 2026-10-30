package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/tailscale/hujson"
	"studio/internal/acp"
	"studio/internal/agents"
	"studio/internal/profile"
)

// Devin's own work in the window: finding its command line among the ones on
// the computer, signing in through the browser, and the registration and hooks
// it reads from a workspace.

// devinID is Devin's ID, for the files of Devin's own features.
const devinID = "devin"

func init() {
	registerProviderModule(devinID, providerModule{
		Info:           devinInfoOf,
		SignIn:         (*App).signInDevin,
		AuthRequired:   devinAuthRequired,
		PrepareApplets: devinPrepareApplets,
		Adapter:        devinAdapter,
		Prices:         devinModelPrices,
	})
}

// devinInfoOf says whether Devin's command line is found, which of the ones on
// the computer works, and where. Its command line is checked as it is found
// (agents.FindDevin); a path the person chose is used as chosen.
func devinInfoOf(a *App, p *AgentProviderInfo, workspaces []string) {
	a.mu.Lock()
	p.ExecutableOverride = a.settings.ProviderExecutables[p.ID]
	p.Pinned = append([]string{}, a.settings.Pinned...)
	a.mu.Unlock()
	var found []agents.DevinCandidate
	if p.ExecutableOverride == "" {
		p.Path, found = agents.FindDevin(workspaces)
	}
	var overrideError error
	if p.ExecutableOverride != "" {
		p.Path, overrideError = agents.CheckExecutable(p.ExecutableOverride)
	}
	p.Installed = p.Path != ""
	p.Available = p.Installed
	p.Status = "not-installed"
	if p.Installed {
		p.Status = "detected"
	}
	if p.Available {
		p.Detail = "Installation found. Check Connection verifies the agent without sending a prompt."
		if len(found) > 0 {
			p.Detail = "The Devin command line at " + p.Path + " was found and answered. Check Connection verifies the agent without sending a prompt."
			if len(found) > 1 {
				p.Detail += " Passed over:"
				for _, c := range found[:len(found)-1] {
					p.Detail += " " + c.Path + ", because " + c.Problem + "."
				}
			}
		}
	}
	if !p.Installed && p.ExecutableOverride == "" {
		if choices := agents.DevinChoices(found); len(choices) > 1 {
			p.Status, p.Choices = "choose", choices
			if i := agents.LikelyCurrent(choices); i >= 0 {
				p.Likely = choices[i].Path
			}
			p.Detail = agents.DevinMissing(found, "")
		} else {
			p.Detail = agents.DevinMissing(found, agents.DevinDesktop())
		}
	}
	if overrideError != nil {
		p.Status = "invalid-path"
		p.Detail = overrideError.Error() + ". Choose another executable or use automatic discovery."
	}
}

// devinPrepareApplets writes the registration and the steering hooks Devin
// reads from a workspace.
func devinPrepareApplets(workspace string) error {
	if err := ensureAppletMCPRegistration(workspace); err != nil {
		return err
	}
	return ensureDevinSteeringHooks(workspace)
}

// devinAuthRequired is what a request Devin refused for want of a sign-in
// tells the page.
func devinAuthRequired(a *App) error {
	a.emit("agent:auth", map[string]any{"state": "required", "method": a.signInName()})
	return errSignIn
}

// signInMethod is the first way of signing in that Devin offered which the
// app can carry out: one of the default type, "agent", where the agent does
// the work. The protocol's other types expect the client to collect a secret
// or run a terminal command, which the app does not do.
func signInMethod(methods []acp.AuthMethod) *acp.AuthMethod {
	for i, m := range methods {
		if m.Type == "" || m.Type == "agent" {
			return &methods[i]
		}
	}
	return nil
}

func (a *App) signInName() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, init := a.providerInitLocked("devin")
	if m := signInMethod(init.AuthMethods); m != nil {
		return m.Name
	}
	return ""
}

// signInDevin asks Devin to sign the person in, starting Devin first if it is
// not running. It returns when Devin reports the sign-in done or failed, which
// takes as long as the person needs in the browser.
func (a *App) signInDevin() error {
	if _, _, err := a.lifecycle().Live("devin"); err != nil {
		return err
	}
	a.mu.Lock()
	conn, init := a.providerInitLocked("devin")
	a.mu.Unlock()
	if conn == nil {
		return errors.New("Devin is not running")
	}
	m := signInMethod(init.AuthMethods)
	if m == nil {
		err := errors.New("Devin offers no way to sign in from an app; run devin auth login in a terminal window")
		a.emit("agent:auth", map[string]any{"state": "failed", "error": err.Error()})
		return err
	}
	a.emit("agent:auth", map[string]any{"state": "signing-in", "method": m.Name})
	if err := conn.Call(context.Background(), "authenticate", acp.AuthenticateParams{MethodID: m.ID}, nil); err != nil {
		a.emit("agent:auth", map[string]any{"state": "failed", "error": err.Error()})
		return err
	}
	a.emit("agent:auth", map[string]any{"state": "signed-in"})
	return nil
}

// The adapter that starts and watches the provider (provider_lifecycle.go).
var devinAdapter = &providerAdapter{
	id:       "devin",
	starting: "Starting Devin",
	stopped:  "Devin stopped",
	readyDetail: func(init acp.InitializeResult) string {
		name := "Devin"
		if init.AgentInfo != nil && init.AgentInfo.Title != "" {
			name = init.AgentInfo.Title
		}
		return name + " is ready"
	},
	// A Devin that never answers would leave every chat waiting with no
	// word; a minute is far more than a start takes.
	initWithin: time.Minute,
	traces:     true,
	prepare: func(a *App) (providerStart, error) {
		a.mu.Lock()
		ws := a.settings.Workspace
		level := a.settings.Approvals
		managedRoots := managedExecutionRoots(a.settings.WorkspaceRecords)
		a.mu.Unlock()
		if ws == "" {
			return providerStart{}, errors.New("choose a workspace first")
		}
		found := a.providerInfoOf("devin")
		devin, missing := found.Path, found.Detail
		if devin == "" {
			if missing == "" {
				missing = "the Devin command line was not found outside workspace folders"
			}
			return providerStart{}, errors.New(missing)
		}
		if err := prepareProviderApplets("devin", ws); err != nil {
			return providerStart{}, fmt.Errorf("prepare the managed applet integration: %w", err)
		}
		args := []string{"acp"}
		cfg, err := devinConfig(level, managedRoots...)
		if err != nil {
			return providerStart{}, err
		}
		if cfg != "" {
			args = []string{"--config", cfg, "acp"}
		}
		return providerStart{
			launch: func(handler acp.Handler) (agentConnection, error) {
				trace := a.traceWriter()
				conn, err := acp.Start(devin, args, ws, handler, acp.Options{Env: append(os.Environ(), product.HomeVar()+"="+configDir()), Trace: trace})
				if err != nil {
					_ = trace.Flush()
					return nil, err
				}
				return conn, nil
			},
			ready: func(init acp.InitializeResult) {
				version := ""
				if init.AgentInfo != nil {
					version = init.AgentInfo.Version
				}
				a.refreshDevinPrices(devin, version)
			},
		}, nil
	},
}

var appletRegistrationMu sync.Mutex

type appletRegistrationEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// Devin documents this configuration file and its stdio server schema at
// https://docs.devin.ai/cli/extensibility/mcp/configuration.
func ensureAppletMCPRegistration(workspace string) error {
	executable, err := appletHelperExecutable()
	if err != nil {
		return fmt.Errorf("locate applet helper: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return err
	}
	var path string
	if configOverride != "" || !personsOwnStateFolder() {
		// A test, or a trial copy run with a state folder of its own, never writes
		// Devin's user-wide settings, which are the person's own Dinah's to keep. It
		// registers its helper in the workspace's local file, but only in a workspace
		// that is throwaway: a local file in a real project would outrank the
		// installed Dinah's entry there and stop its Devin chats the same way.
		if workspace == "" || !filepath.IsAbs(workspace) {
			return errors.New("applet registration for a state folder that is not yours requires an absolute workspace")
		}
		if !throwawayWorkspace(workspace) {
			return errors.New("this copy of " + product.Name + " is not the installed one, so it will not change Devin's settings for a workspace that is not a throwaway one: " + workspace)
		}
		path = filepath.Join(workspace, ".devin", "mcp_config.local.json")
	} else {
		base, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("locate Devin configuration: %w", err)
		}
		path = filepath.Join(base, "devin", "mcp_config.json")
	}
	entry := appletRegistrationEntry{Command: executable, Args: []string{"--applet-mcp", "--state-dir", configDir()}}
	if err := checkAppletRegistrationSources(workspace, path, entry); err != nil {
		return err
	}
	return registerAppletMCPAt(path, entry)
}

// personsOwnStateFolder says this Dinah keeps its state in the person's own folder,
// the default one in their home, and not in one a trial copy was given.
func personsOwnStateFolder() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	own := filepath.Join(home, profile.Current().HomeFolder)
	return strings.EqualFold(filepath.Clean(configDir()), filepath.Clean(own))
}

// throwawayWorkspace says a workspace is inside this copy's own state folder or the
// system's temporary folder, which is where a trial copy may leave a file.
func throwawayWorkspace(workspace string) bool {
	for _, root := range []string{configDir(), os.TempDir()} {
		if insideFolder(root, workspace) {
			return true
		}
	}
	return false
}

// insideFolder says path is root or below it, comparing real paths where they
// resolve and ignoring case, as the file system of the platform does.
func insideFolder(root, path string) bool {
	resolve := func(p string) string {
		p = filepath.Clean(p)
		if real, err := filepath.EvalSymlinks(p); err == nil {
			return real
		}
		return p
	}
	root, path = resolve(root), resolve(path)
	if runtime.GOOS == "windows" {
		root, path = strings.ToLower(root), strings.ToLower(path)
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// describeEntry says what a dinah-applets entry in a file runs, for an error.
func describeEntry(e appletRegistrationEntry) string {
	if e.Command == "" {
		return "which runs nothing " + product.Name + " could read"
	}
	return "which runs " + strings.TrimSpace(e.Command+" "+strings.Join(e.Args, " "))
}

// Check the documented native and imported configuration sources before the
// model receives an authorization. A higher-precedence server can otherwise
// receive a context intended for the managed helper.
func checkAppletRegistrationSources(workspace, managedPath string, entry appletRegistrationEntry) error {
	projectFiles := []string{
		".devin/mcp_config.json", ".devin/mcp_config.local.json", ".devin/config.json", ".devin/config.local.json",
		".cursor/mcp.json", ".mcp.json", ".claude/settings.json", ".claude/settings.local.json",
		"opencode.json", ".vscode/mcp.json", ".zed/settings.json",
	}
	var paths []string
	for dir := filepath.Clean(workspace); ; dir = filepath.Dir(dir) {
		for _, name := range projectFiles {
			paths = append(paths, filepath.Join(dir, filepath.FromSlash(name)))
		}
		_, gitErr := os.Stat(filepath.Join(dir, ".git"))
		_, jjErr := os.Stat(filepath.Join(dir, ".jj"))
		if configOverride != "" || gitErr == nil || jjErr == nil || filepath.Dir(dir) == dir {
			break
		}
	}
	if configOverride == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		for _, name := range []string{
			".cursor/mcp.json",
			".claude.json", ".claude/settings.json", ".claude/settings.local.json", ".claude/mcp_servers.json",
			".codeium/windsurf/mcp_config.json", ".codeium/windsurf-next/mcp_config.json", ".codeium/windsurf-insiders/mcp_config.json",
			".config/opencode/opencode.json", ".config/zed/settings.json",
		} {
			paths = append(paths, filepath.Join(home, filepath.FromSlash(name)))
		}
		base, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		paths = append(paths, filepath.Join(base, "devin", "config.json"))
	}
	for _, path := range paths {
		if strings.EqualFold(filepath.Clean(path), filepath.Clean(managedPath)) {
			continue
		}
		if err := checkAppletRegistrationSource(path, entry); err != nil {
			return err
		}
	}
	return nil
}

// A source that cannot be read or parsed is passed over: it is another
// program's file, and one its own program cannot load defines no server
// that could take the helper's place (a consultant's Windsurf file that did
// not parse kept every chat from starting, 14 October 2026). Duplicate
// properties are no bar either, since every member is inspected.
func checkAppletRegistrationSource(path string, entry appletRegistrationEntry) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	doc, err := hujson.Parse(withoutBOM(data))
	if err != nil {
		return nil
	}
	var inspect func(*hujson.Value) error
	inspect = func(v *hujson.Value) error {
		switch value := v.Value.(type) {
		case *hujson.Object:
			for i := range value.Members {
				member := &value.Members[i]
				if member.Name.Value.(hujson.Literal).String() == "dinah-applets" {
					standard := member.Value.Clone()
					standard.Standardize()
					var candidate appletRegistrationEntry
					var fields map[string]json.RawMessage
					if json.Unmarshal(standard.Pack(), &candidate) != nil || json.Unmarshal(standard.Pack(), &fields) != nil || len(fields) != 2 || !reflect.DeepEqual(candidate, entry) {
						return fmt.Errorf("%s has a dinah-applets entry %s that conflicts with %s's own, since %s did not write it. Remove that entry from the file and %s will register its own", path, describeEntry(candidate), product.Name, product.Name, product.Name)
					}
				}
				if err := inspect(&member.Value); err != nil {
					return err
				}
			}
		case *hujson.Array:
			for i := range value.Elements {
				if err := inspect(&value.Elements[i]); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return inspect(&doc)
}

// ownStateDir reports whether an existing helper entry's state folder is
// Dinah's: the current one, or the folder it kept its state in before the
// move (state_directory.go). A build from before the move wrote the old
// folder into Devin's configuration, and that entry is Dinah's own to
// replace, not another program's.
func ownStateDir(existing, current, legacy string) bool {
	same := func(a, b string) bool { return b != "" && strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }
	return same(existing, current) || same(existing, legacy)
}

// legacyStateDir is the folder Dinah kept its state in before the
// move, or "" when tests run with a throwaway state folder.
func legacyStateDir() string {
	// Dinah has no earlier state folder of its own.
	return ""
}

func registerAppletMCPAt(path string, entry appletRegistrationEntry) error {
	appletRegistrationMu.Lock()
	defer appletRegistrationMu.Unlock()
	original, err := os.ReadFile(path)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return fmt.Errorf("read Devin MCP configuration: %w", err)
	}
	input := withoutBOM(original)
	if missing {
		input = []byte("{}\n")
	}
	doc, err := hujson.Parse(input)
	if err != nil {
		return errors.New("Devin MCP configuration is not valid JSON with comments")
	}
	if err := uniqueAppletConfigObjects(&doc); err != nil {
		return err
	}
	if _, ok := doc.Value.(*hujson.Object); !ok {
		return errors.New("Devin MCP configuration must be an object")
	}
	patches := []map[string]any{}
	servers := doc.Find("/mcpServers")
	if servers == nil {
		patches = append(patches, map[string]any{"op": "add", "path": "/mcpServers", "value": map[string]any{}})
	} else if _, ok := servers.Value.(*hujson.Object); !ok {
		return errors.New("Devin MCP configuration mcpServers must be an object")
	}
	const pointer = "/mcpServers/dinah-applets"
	if existing := doc.Find(pointer); existing != nil {
		standard := existing.Clone()
		standard.Standardize()
		var old appletRegistrationEntry
		var fields map[string]json.RawMessage
		if json.Unmarshal(standard.Pack(), &old) != nil || json.Unmarshal(standard.Pack(), &fields) != nil || fields == nil {
			return fmt.Errorf("%s has a dinah-applets entry that is not a %s helper and conflicts with its own. Remove that entry from the file and %s will register its own", path, product.Name, product.Name)
		}
		owned := filepath.IsAbs(old.Command) && len(old.Args) == 3 && old.Args[0] == "--applet-mcp" && old.Args[1] == "--state-dir" && ownStateDir(old.Args[2], entry.Args[2], legacyStateDir())
		if !owned || len(fields) != 2 {
			return fmt.Errorf("%s has a dinah-applets entry %s that conflicts with %s's own, since %s did not write it (it is another copy's or another program's). Remove that entry from the file and %s will register its own", path, describeEntry(old), product.Name, product.Name, product.Name)
		}
		if reflect.DeepEqual(old, entry) {
			return nil
		}
	}
	patches = append(patches, map[string]any{"op": "add", "path": pointer, "value": entry})
	patch, _ := json.Marshal(patches)
	if err := doc.Patch(patch); err != nil {
		return errors.New("could not update the managed applet registration")
	}
	out := doc.Pack()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".dinah-mcp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// Serialize this process and reject changes observed from another writer.
	// The rename is a replacement; its atomicity is not assumed on Windows.
	current, readErr := os.ReadFile(path)
	if (!missing && readErr != nil) || (missing && !errors.Is(readErr, os.ErrNotExist)) || !bytes.Equal(current, original) {
		return errors.New("Devin MCP configuration changed during registration; retry the connection")
	}
	return os.Rename(tmp.Name(), path)
}

// A byte order mark, which an editor on Windows may write at the front of a
// JSON file, is not part of the text (RFC 8259, section 8.1 allows a parser
// to ignore it).
func withoutBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
}

// Reject ambiguous duplicate properties before applying JSON Pointer updates.
func uniqueAppletConfigObjects(v *hujson.Value) error {
	switch value := v.Value.(type) {
	case *hujson.Object:
		seen := map[string]bool{}
		for i := range value.Members {
			member := &value.Members[i]
			key := member.Name.Value.(hujson.Literal).String()
			if seen[key] {
				return errors.New("Devin MCP configuration contains duplicate object properties")
			}
			seen[key] = true
			if err := uniqueAppletConfigObjects(&member.Value); err != nil {
				return err
			}
		}
	case *hujson.Array:
		for i := range value.Elements {
			if err := uniqueAppletConfigObjects(&value.Elements[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

// Devin's model prices are read from its command line in the background and
// shown in the model list (DESIGN.md, Devin Model Prices). They are read once
// a run, again when Devin starts as another executable or version, and again
// when the person opens the model list an hour or more after the last read.
// Nothing waits for them: the page draws the list at once and draws it again
// when prices arrive. When the read fails in any way there are no prices,
// and the list looks as it did before prices existed.

// devinPriceAge is how long prices are kept before opening the model list
// reads them again.
var devinPriceAge = time.Hour

// readDevinPrices reads the prices from the Devin command line at path;
// tests replace it, so that they never start Devin.
var readDevinPrices = agents.DevinModelPrices

type devinPriceCache struct {
	mu sync.Mutex
	// path and version are the Devin the prices were last read from, or are
	// being read from, and at when that read finished.
	path, version string
	at            time.Time
	reading       bool
	prices        map[string]agents.DevinModelPrice
}

// refreshDevinPrices reads the prices again, in the background, unless a
// read is under way or the prices already came from this Devin within the
// hour. An empty version is a Devin not yet started.
func (a *App) refreshDevinPrices(path, version string) {
	if path == "" {
		return
	}
	c := &a.devinPrices
	c.mu.Lock()
	fresh := c.path == path && c.version == version && !c.at.IsZero() && time.Since(c.at) < devinPriceAge
	if c.reading || fresh {
		c.mu.Unlock()
		return
	}
	c.reading, c.path, c.version = true, path, version
	c.mu.Unlock()
	go func() {
		prices, err := readDevinPrices(path)
		if err != nil {
			logf("core", "info", "Devin's model prices are not shown: %v", err)
			prices = nil
		}
		c.mu.Lock()
		changed := !sameJSON(c.prices, prices)
		c.prices, c.at, c.reading = prices, time.Now(), false
		c.mu.Unlock()
		if changed {
			a.emit("models:prices", map[string]any{"provider": devinID, "prices": a.matchedDevinPrices()})
		}
	}()
}

// matchedDevinPrices are the prices of the models Devin offers in its open
// sessions and in the catalog Settings keeps, by the model option's value.
func (a *App) matchedDevinPrices() map[string]agents.DevinModelPrice {
	c := &a.devinPrices
	c.mu.Lock()
	prices := c.prices
	c.mu.Unlock()
	if len(prices) == 0 {
		return map[string]agents.DevinModelPrice{}
	}
	a.mu.Lock()
	offered := [][]acp.ConfigOption{a.settings.Catalog}
	var open []*Session
	for _, s := range a.runtime.ConversationsLocked() {
		if providerRegistry[normalizedProvider(s.Provider())].Prices {
			open = append(open, s)
		}
	}
	a.mu.Unlock()
	for _, s := range open {
		offered = append(offered, s.Options())
	}
	out := map[string]agents.DevinModelPrice{}
	for _, opts := range offered {
		for value, p := range agents.MatchDevinPrices(opts, prices) {
			out[value] = p
		}
	}
	return out
}

// devinModelPrices are the prices known now for Devin's models. The page asks
// as the person opens the model list, and that may start a read in the
// background, whose prices arrive as models:prices.
func devinModelPrices(a *App) map[string]agents.DevinModelPrice {
	a.mu.Lock()
	conn, init := a.providerInitLocked(devinID)
	running := conn != nil
	version := ""
	if init.AgentInfo != nil {
		version = init.AgentInfo.Version
	}
	a.mu.Unlock()
	c := &a.devinPrices
	c.mu.Lock()
	path := c.path
	if !running {
		version = c.version
	}
	c.mu.Unlock()
	if path != "" {
		a.refreshDevinPrices(path, version)
	} else {
		// Finding Devin can start it to check it, so it is done in the
		// background too.
		go func() { a.refreshDevinPrices(a.providerInfoOf(devinID).Path, "") }()
	}
	return a.matchedDevinPrices()
}
