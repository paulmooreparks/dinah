package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"studio/internal/acp"
	"studio/internal/bus"
	"studio/internal/layout"
	"studio/internal/workspaces"
)

// Settings are the person's choices that outlast a restart.
type Settings struct {
	NewWindowMode string `json:"newWindowMode,omitempty"`
	// HostNotificationsOff turns off the operating system's notifications
	// when an agent needs the person (host_notices.go), which are
	// on unless it is set.
	HostNotificationsOff bool `json:"hostNotificationsOff,omitempty"`
	// PromptSendKey is the key that sends a chat's prompt: "ctrl-enter",
	// with Enter starting a new line, or empty for Enter, with Shift+Enter
	// starting a new line (prompt_keys.go).
	PromptSendKey string `json:"promptSendKey,omitempty"`
	// LegacyPersonName is where earlier releases kept the person's name. The
	// name now lives in the bus (person_name.go); this is read once, to carry
	// it over, and cleared so that a save no longer writes it.
	LegacyPersonName string `json:"personName,omitempty"`
	// DocumentViews are the views documents open in, by media type, where
	// the person chose other than the default (document_views.go).
	DocumentViews map[string]string `json:"documentViews,omitempty"`
	// OllamaApprovals is when Ollama's own tools ask the person: "ask" for
	// each command line, "chat" once in each chat, or "all" for everything.
	// The older "allow" value also asks nothing. Empty is "chat".
	OllamaApprovals string `json:"ollamaApprovals,omitempty"`
	// OllamaFileScope is where Ollama's file tools may go: "workspace",
	// "folders" (the workspace and OllamaFileFolders) or "anywhere". Empty is
	// anywhere, so an install from before the choice keeps what it had; a
	// settings file that does not exist yet starts with "workspace"
	// (ollama_files.go).
	OllamaFileScope string `json:"ollamaFileScope,omitempty"`
	// OllamaRounds is how many rounds of tool calls an Ollama turn may take;
	// 0, or absent, is no limit (ollama_rounds.go).
	OllamaRounds int `json:"ollamaRounds,omitempty"`
	// DefaultContextReleased is set once the saved options that held the old
	// default context length for a chat have been released (provider_ollama.go).
	DefaultContextReleased bool `json:"defaultContextReleased,omitempty"`
	// JudgedTitles are the titles a judge wrote for chats, by chat, with the first
	// line each came from (judge_titles.go).
	JudgedTitles      map[string]JudgedTitle `json:"judgedTitles,omitempty"`
	OllamaFileFolders []string               `json:"ollamaFileFolders,omitempty"`
	// MentionBaseline is the number of the newest bus message when the window
	// first looked for mentions of the person; only later ones make cards
	// (mention_notices.go). Nil until then.
	MentionBaseline *int64 `json:"mentionBaseline,omitempty"`
	// CodexApprovals is how far a Codex chat may go without asking: "ask",
	// "workspace" or "full" (codex_approvals.go). Empty is "ask".
	CodexApprovals string `json:"codexApprovals,omitempty"`
	// FileLimits are the limits the person chose on the files in one
	// message, by provider, where they differ from the documented default;
	// 0 is no limit (file_limits.go).
	FileLimits map[string]int `json:"fileLimits,omitempty"`
	// WindowModes are the person's choices for the kinds of window that open
	// their own way, by kind (window_modes.go).
	WindowModes          map[string]string `json:"windowModes,omitempty"`
	ProviderExecutables  map[string]string `json:"providerExecutables,omitempty"`
	DefaultProvider      string            `json:"defaultProvider,omitempty"`
	CodexAppletApprovals string            `json:"codexAppletApprovals,omitempty"`
	// AppletGrants are the applet actions the person allowed always, by
	// workspace path, so a chat there runs them without asking
	// (applet_consent.go).
	AppletGrants     map[string][]string                   `json:"appletGrants,omitempty"`
	ProviderDefaults map[string]map[string]json.RawMessage `json:"providerDefaults,omitempty"`
	ProviderCatalogs map[string][]acp.ConfigOption         `json:"providerCatalogs,omitempty"`
	ProviderPinned   map[string][]string                   `json:"providerPinned,omitempty"`
	Workspace        string                                `json:"workspace"`
	WorkspaceID      string                                `json:"workspaceId,omitempty"`
	// WorkspaceRecords is the workspace registry. It is kept in its own file
	// (workspaces.json, internal/workspaces), where the command line reads
	// it too, and not in settings.json; loadSettings and saveSettings move
	// it between the two. It is tagged out of the JSON form, which clone
	// also uses, so clone copies it by hand.
	WorkspaceRecords []WorkspaceRecord `json:"-"`
	// LegacyWorkspaceRecords is where earlier releases kept the registry. It
	// is read once, to carry the registry over, and never written.
	LegacyWorkspaceRecords []WorkspaceRecord `json:"workspaceRecords,omitempty"`
	Workspaces             []string          `json:"workspaces,omitempty"`
	// registryAbsent and legacyRegistry say what loadSettings found: no
	// registry file yet, and a registry still in settings.json. They are
	// not saved, and clone does not keep them.
	registryAbsent, legacyRegistry bool
	// HomeName is the name of the person's home folder, /home/<name>, when
	// they want one other than their account's (home.go).
	HomeName string `json:"homeName,omitempty"`
	// Approvals decides which of the agent's actions are put to the person
	// first: ApprovalsAsk, ApprovalsDinah or ApprovalsDevin.
	Approvals string `json:"approvals"`
	// Recent are the workspaces used before, most recent first.
	Recent []string `json:"recent,omitempty"`
	// Defaults are the person's choices among the config options the agent
	// offers (its model and mode, say), by option id, for every new session,
	// where the agent still offers the value.
	Defaults map[string]json.RawMessage `json:"defaults,omitempty"`
	// SessionOptions are the choices made in each session, by session id and
	// then option id, given to the session again when it is reopened.
	SessionOptions   map[string]map[string]json.RawMessage `json:"sessionOptions,omitempty"`
	SessionProviders map[string]string                     `json:"sessionProviders,omitempty"`
	// OptionSubstitutions are the values the person chose, by provider, to
	// reopen a chat with when its saved value of an option is not offered
	// (option_substitutions.go).
	OptionSubstitutions OptionSubstitutions `json:"optionSubstitutions,omitempty"`
	// SessionTitles are the names the person gave sessions, by session id.
	// The protocol defines no way to rename a session in the agent, so the
	// app keeps them and shows them in place of the agent's titles.
	SessionTitles map[string]string `json:"sessionTitles,omitempty"`
	// BusSessions are each chat's session ID on the bus, by chat, so a chat
	// keeps its name on the bus when it is reopened (bus_chats.go).
	BusSessions map[string]string `json:"busSessions,omitempty"`
	// TerminalShell is the shell a new Terminal runs (terminal_shells.go).
	TerminalShell    string            `json:"terminalShell,omitempty"`
	TerminalProfiles []TerminalProfile `json:"terminalProfiles,omitempty"`
	// Hidden are sessions the person took off the list without the agent
	// deleting them, because it could not or does not delete sessions.
	Hidden []string `json:"hidden,omitempty"`
	// Catalog is the list of config options the agent offered most
	// recently, so that Settings can show them before any session is open.
	Catalog []acp.ConfigOption `json:"catalog,omitempty"`
	// Pinned are the person's favourite values, such as their usual models,
	// offered first wherever a list is long.
	Pinned []string `json:"pinned,omitempty"`
	// ShowThinking shows the agent's thinking in the run.
	ShowThinking bool `json:"showThinking"`
	// Templates are the person's own prompts, offered in the "/" menu.
	Templates []Template `json:"templates,omitempty"`
	// UsagePage is the web page that shows the person's Devin usage and
	// limit. Devin documents no API through which a member reads their own
	// usage (https://docs.devin.ai/enterprise/security-access/personal-analytics),
	// so the app links to the page instead of showing the figures.
	UsagePage string `json:"usagePage,omitempty"`
	// LogLevel is how much the log keeps (applog.go): LogOff, LogProblems
	// or LogAll; empty is LogProblems.
	LogLevel string `json:"logLevel,omitempty"`
}

// Template is a prompt the person saved under a name.
type Template struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// The approval levels. Each but ApprovalsDevin starts Devin with a
// configuration file of the app's own (the documented --config flag), whose
// permission rules are documented at https://docs.devin.ai/cli/reference/permissions.
// A project's own .devin/config.json still applies on top, as Devin's
// precedence rules say (https://docs.devin.ai/cli/reference/configuration/global-vs-local).
const (
	// ApprovalsDevin uses the person's own Devin configuration unchanged, so
	// Devin asks exactly what it asks in Devin Desktop. It is the default.
	ApprovalsDevin = "devin"
	// ApprovalsAll allows every action without asking, and refuses, silently,
	// only what the setup reserves for the person.
	ApprovalsAll = "all"
	// ApprovalsDinah allows what only reads, read-only shell commands and
	// making folders, and asks before anything else.
	ApprovalsDinah = "dinah"
	// ApprovalsAsk allows nothing in advance: every action that is not
	// read-only is put to the person.
	ApprovalsAsk = "ask"
)

// configOverride, when set, is the folder the app keeps its settings in
// instead of the person's own. Only the tests set it, so that a test never
// reads or writes the settings of the app the person is using.
var configOverride string

func configDir() string {
	if configOverride != "" {
		_ = os.MkdirAll(configOverride, 0o700)
		return configOverride
	}
	if applicationStateDir != "" {
		return applicationStateDir
	}
	// Production initializes and migrates the directory before creating App.
	// A missing home must never redirect writes to the working directory.
	home, err := bus.Home()
	if err != nil {
		panic(fmt.Errorf("%s's home folder is unavailable: %w", product.Name, err))
	}
	return home
}

func loadSettings() (Settings, error) {
	s := Settings{Approvals: ApprovalsDevin, NewWindowMode: "floating"}
	b, err := readStateFile(filepath.Join(configDir(), "settings.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return s, err
	}
	if err == nil {
		if !bytes.HasPrefix(bytes.TrimSpace(b), []byte("{")) {
			return s, errors.New("settings must contain a JSON object")
		}
		if err := json.Unmarshal(b, &s); err != nil {
			return s, fmt.Errorf("read settings: %w", err)
		}
		releaseOldDefaultContext(&s)
	} else {
		// An install that starts without settings limits Ollama's file
		// tools to the workspace folders; one that has settings and no
		// choice keeps going anywhere (ollama_files.go).
		s.OllamaFileScope = ollamaScopeWorkspace
	}
	// The registry is in its own file. Where that file is not there yet, an
	// older settings.json may still hold it; where both exist, the file is
	// the newer and the old copy is ignored, then dropped by the next save.
	records, regErr := workspaces.Load(configDir())
	switch {
	case regErr == nil:
		s.WorkspaceRecords = records
	case errors.Is(regErr, workspaces.ErrNoRegistry):
		s.registryAbsent = true
		s.WorkspaceRecords = s.LegacyWorkspaceRecords
	default:
		return s, regErr
	}
	s.legacyRegistry = len(s.LegacyWorkspaceRecords) > 0
	s.LegacyWorkspaceRecords = nil
	if s.NewWindowMode == "" {
		s.NewWindowMode = "floating"
	}
	if s.NewWindowMode != "floating" && s.NewWindowMode != "maximized" {
		return s, errors.New("settings contain an unsupported new window mode")
	}
	if err := checkStoredWindowModes(s.WindowModes); err != nil {
		return s, err
	}
	if s.CodexAppletApprovals == "" {
		s.CodexAppletApprovals = "ask"
	}
	if s.CodexAppletApprovals != "ask" && s.CodexAppletApprovals != "allow" && s.CodexAppletApprovals != "all" {
		return s, errors.New("settings contain an unsupported Codex applet approval preference")
	}
	switch s.OllamaApprovals {
	case "", "ask", "chat", "allow", "all":
	default:
		s.OllamaApprovals = ""
	}
	switch s.OllamaFileScope {
	case "", ollamaScopeWorkspace, ollamaScopeFolders, ollamaScopeAnywhere:
	default:
		s.OllamaFileScope = ""
	}
	s.OllamaFileFolders = slices.DeleteFunc(s.OllamaFileFolders, func(f string) bool { return !filepath.IsAbs(f) })
	switch s.CodexApprovals {
	case "", "ask", "workspace", "full":
	default:
		s.CodexApprovals = ""
	}
	checkStoredFileLimits(&s)
	checkStoredSubstitutions(&s)
	if s.LogLevel == "" || !validLogLevel(s.LogLevel) {
		s.LogLevel = LogProblems
	}
	switch s.Approvals {
	case ApprovalsAsk, ApprovalsDinah, ApprovalsDevin, ApprovalsAll:
	default:
		s.Approvals = ApprovalsDevin
	}
	if s.Workspace != "" && len(s.Recent) == 0 {
		s.Recent = []string{s.Workspace}
	}
	if s.Workspace != "" && len(s.Workspaces) == 0 {
		s.Workspaces = []string{s.Workspace}
	}
	return s, nil
}

// addRecent puts a workspace first among the recent ones, keeping eight.
func (s *Settings) addRecent(ws string) {
	out := []string{ws}
	for _, r := range s.Recent {
		if !layout.SamePath(r, ws) && len(out) < 8 {
			out = append(out, r)
		}
	}
	s.Recent = out
}

// saveRegistry writes the workspace registry. It is a variable only so that
// a test can watch when saveSettings writes it, and make it fail.
var saveRegistry = workspaces.Save

// backUpSettingsBeforeSplit keeps a copy of settings.json as it was before
// the workspace registry left it, once, in settings.json.before-split. No
// part of Dinah reads the copy. It is there for the person to restore by
// hand if the move goes wrong; an older release started afterwards would
// not read it, and is not supported.
func backUpSettingsBeforeSplit() {
	path := filepath.Join(configDir(), "settings.json")
	backup := path + ".before-split"
	if _, err := os.Stat(backup); err == nil {
		return
	}
	data, err := readStateFile(path)
	if err != nil {
		return
	}
	// Written as the state files are, so the copy is complete or missing: a
	// torn copy would look finished to the check above and stay that way.
	if err := writeStateFile(backup, data); err != nil {
		logf("core", "error", "keep a copy of the settings before the registry moves: %v", err)
	}
}

// saveSettings saves the settings in settings.json and the workspace
// registry in workspaces.json. The two files cannot be replaced together,
// so the order keeps every workspace the settings refer to in the registry
// at each moment. First the registry is saved with its new records and the
// ones about to go, then settings.json, then the registry again without
// the ones that went. A failure between the steps leaves, at most, a record
// nothing refers to, which loading accepts. A failed settings write puts the
// registry back as it was. A failed last step is logged and the save still
// succeeds, since settings.json already stopped referring to the workspace;
// the next save in the same run drops the leftover record, but after a
// restart it is loaded as a registered workspace, which shows in the list
// again and can be removed again.
func saveSettings(s Settings) error {
	s.LegacyWorkspaceRecords = nil
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := configDir()
	old, err := workspaces.Load(dir)
	existed := err == nil
	if err != nil && !errors.Is(err, workspaces.ErrNoRegistry) {
		return err
	}
	next := s.WorkspaceRecords
	grown := workspaces.Grown(old, next)
	growing := !existed || !workspaces.Same(grown, old)
	if growing {
		if err := saveRegistry(dir, grown); err != nil {
			return err
		}
	}
	if err := writeStateFile(filepath.Join(dir, "settings.json"), b); err != nil {
		if growing && existed {
			if undo := saveRegistry(dir, old); undo != nil {
				logf("core", "error", "put the workspace registry back after a failed save: %v", undo)
			}
		}
		return err
	}
	if !workspaces.Same(grown, next) {
		if err := saveRegistry(dir, next); err != nil {
			logf("core", "error", "drop removed workspaces from the registry: %v", err)
		}
	}
	return nil
}

// updateSettings changes the settings through change, on a deep copy, and
// saves the copy; only when the save succeeds does the App take it, so the
// settings in memory never differ from those on disk, and a map in the old
// settings is never changed under a reader. It is the one way to change
// settings; the places that copy, save and assign by hand move to it as
// they are touched.
// errSettingsUnchanged is what a change returns when it found nothing to
// change, so that the settings are not written; the caller treats it as
// success.
var errSettingsUnchanged = errors.New("the settings are unchanged")

func (a *App) updateSettings(change func(*Settings) error) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.startupErr != nil {
		return a.startupErr
	}
	updated, err := a.settings.clone()
	if err != nil {
		return err
	}
	if err := change(&updated); err != nil {
		return err
	}
	if err := saveSettings(updated); err != nil {
		return err
	}
	a.settings = updated
	return nil
}

// clone is a deep copy of the settings, made through their JSON form, which
// is the form they are saved in, so nothing the save keeps is shared. The
// registry is not in that form, so it is copied on its own.
func (s Settings) clone() (Settings, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return Settings{}, err
	}
	var out Settings
	if err = json.Unmarshal(b, &out); err != nil {
		return Settings{}, err
	}
	if len(s.WorkspaceRecords) > 0 {
		out.WorkspaceRecords = workspaces.Copy(s.WorkspaceRecords)
	}
	return out, nil
}

type permissions struct {
	Allow []string `json:"allow"`
	Ask   []string `json:"ask,omitempty"`
	Deny  []string `json:"deny"`
}

// dinahRules are the rules of ApprovalsDinah: what only reads and looks is
// allowed, and so is making folders inside the workspace; every other
// action is put to the person.
var dinahRules = permissions{
	Allow: []string{
		"read", "grep", "glob",
		// Shell commands that only look.
		"Exec(ls)", "Exec(dir)", "Exec(cat)", "Exec(type)", "Exec(head)", "Exec(tail)", "Exec(wc)",
		"Exec(find)", "Exec(grep)", "Exec(rg)", "Exec(pwd)", "Exec(echo)",
		"Exec(Get-ChildItem)", "Exec(Get-Content)", "Exec(Select-String)", "Exec(Test-Path)", "Exec(Get-Item)", "Exec(Resolve-Path)",
		"Exec(git status)", "Exec(git log)", "Exec(git diff)", "Exec(git show)",
		// Folders for the skills' output inside the workspace.
		"Exec(mkdir)", "Exec(New-Item)",
	},
	Deny: []string{},
}

// allRules are the rules of ApprovalsAll, every documented tool and pattern
// allowed: the bare tool names (read, edit, grep, glob, exec), every path for
// reading and writing, every web address, and every MCP tool.
var allRules = permissions{
	Allow: []string{
		"read", "edit", "grep", "glob", "exec",
		"Read(**)", "Write(**)",
		"Fetch(https://*)", "Fetch(http://*)",
		"mcp__*",
	},
	Deny: []string{},
}

// devinConfig writes the configuration file for an approval level and
// returns its path, or "" for ApprovalsDevin, which uses none.
//
// Devin writes into this file too: an answer such as "Yes, always allow
// Write-Output commands in all projects" is saved in the configuration file
// Devin was started with. The file is therefore merged, not replaced: the
// level's own rules are written, and every rule Devin added since is kept,
// so that an answer is asked once and not again after every restart. A rule
// Devin added to allow something the level asks about or denies is dropped,
// so that no saved answer can lift the level's own asks and denies.
func devinConfig(level string, managedRoots ...string) (string, error) {
	var p permissions
	switch level {
	case ApprovalsDevin:
		return "", nil
	case ApprovalsAsk:
		p = permissions{Allow: []string{}, Deny: []string{}}
	case ApprovalsAll:
		p = allRules
	default:
		p = dinahRules
	}
	if level == ApprovalsAll || level == ApprovalsDinah {
		// The person's own flows are kept outside every workspace, and the
		// agent is asked to change them; writing there is allowed as writing
		// in the workspace is. The rule takes the form Devin itself saves for
		// a folder.
		p.Allow = append(append([]string{}, p.Allow...), flowsWriteRule())
		for _, root := range managedRoots {
			if !managedExecutionRoot(root) || strings.ContainsAny(root, "*?[](){}\r\n") {
				return "", errors.New("the managed workspace path cannot be represented safely in agent permissions")
			}
			p.Allow = append(p.Allow, "Write("+root+string(filepath.Separator)+"**)")
		}
	}
	path := filepath.Join(configDir(), "devin-"+level+".json")
	file := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		var saved struct {
			Permissions permissions `json:"permissions"`
		}
		if json.Unmarshal(b, &saved) == nil {
			// Rebuild generated workspace permissions from the current registry.
			// Removing a workspace must not retain its previous generated rule.
			var allow []string
			for _, rule := range saved.Permissions.Allow {
				path := strings.TrimSuffix(strings.TrimPrefix(rule, "Write("), string(filepath.Separator)+"**)")
				if !managedExecutionRoot(path) {
					allow = append(allow, rule)
				}
			}
			saved.Permissions.Allow = allow
			p = mergeSaved(p, saved.Permissions)
		}
		// Anything else Devin keeps in the file stays as it was.
		_ = json.Unmarshal(b, &file)
	}
	file["permissions"] = p
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(b, '\n'), 0o600)
}

// flowsWriteRule allows writing in the person's own flows folder and below.
func flowsWriteRule() string {
	return "Write(" + filepath.Join(configDir(), "flows") + string(filepath.Separator) + "**)"
}

// mergeSaved adds to a level's rules the rules a saved file holds beyond
// them, leaving out a saved allow on anything the level asks about or denies.
func mergeSaved(base, saved permissions) permissions {
	guarded := map[string]bool{}
	for _, r := range append(append([]string{}, base.Ask...), base.Deny...) {
		guarded[r] = true
	}
	add := func(into, from []string, skip map[string]bool) []string {
		have := map[string]bool{}
		for _, r := range into {
			have[r] = true
		}
		out := append([]string{}, into...)
		for _, r := range from {
			if !have[r] && !skip[r] {
				have[r] = true
				out = append(out, r)
			}
		}
		return out
	}
	return permissions{
		Allow: add(base.Allow, saved.Allow, guarded),
		Ask:   add(base.Ask, saved.Ask, nil),
		Deny:  add(base.Deny, saved.Deny, nil),
	}
}
