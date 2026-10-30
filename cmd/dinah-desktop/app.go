package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"studio/internal/acp"
	"studio/internal/applet/host"
	"studio/internal/bus"
	"studio/internal/bushost"
	"studio/internal/extensions"
	"studio/internal/layout"
	"studio/internal/providers"
	"studio/internal/sessionhost"
	"studio/internal/setup"
	"studio/internal/terminals"
	"studio/internal/workspaces"
)

// App is what the page calls. It runs one Devin process (`devin acp`) and
// any number of sessions in it, each in the workspace it was started in, and
// passes what Devin sends to the page as events. Every event about a session
// carries its sessionId:
//
//	agent:state       {state, detail}                  the Devin process: starting, ready, stopped, failed
//	agent:session     SessionInfo                      a session started, opened, changed title or state
//	agent:update      {sessionId, update}              a session/update notification
//	agent:permission  {sessionId, key, toolCall, options}  a question waiting for the person
//	agent:answered    {sessionId, key}                 the question is settled
//	agent:file        {sessionId, path, bytes}         the agent wrote a file through the app
//	agent:turn        {sessionId, stopReason, error}   a turn ended
//	agent:queue       {sessionId, queued}              the messages waiting for the turn to end
//	agent:sent        {sessionId, text, kind}          a queued or steering message went to the agent
//	agent:options     {sessionId, options}             the session's config options
type App struct {
	startupErr error
	// judging is the set of judges and the person's choices for them
	// (judgements.go).
	judging    judging
	extensions *extensions.Service
	// asks are the questions put to the page and not yet answered (applet_window_commands.go).
	asks             pageAsks
	nativeMu         sync.Mutex
	nativeAPI        nativeWindowAPI
	nativeRestoreMax bool
	nativeWindow     nativeWindowState
	placed           sync.Once
	exitConfirmed    bool
	exitConfirming   bool
	exitRequest      string
	// exitRequestStatus is the status exitRequest was asked with, kept for
	// PendingExitConfirmation.
	exitRequestStatus exitStatus
	exitTimer         *time.Timer
	exitWork          map[string]string
	exitCalls         int
	exitPrompt        func(context.Context, exitStatus) bool
	closeRequest      string
	closeApproved     bool
	closeTimer        *time.Timer
	// folderWatch watches the folders the page shows for changes on disk
	// (workspace_watch.go).
	folderWatch *folderWatch
	ctx         context.Context
	mu          sync.Mutex
	settings    Settings
	// lifecycleOnce constructs the one provider lifecycle, including in fixtures.
	lifecycleOnce sync.Once
	// queueStore keeps the queued messages of chats on disk, and queueRestorer puts
	// them back (queue_restore.go).
	queueStore         *sessionhost.QueueStore
	queueRestorerMu    sync.Mutex
	queueRestorerValue *sessionhost.QueueRestorer
	// teamMu guards the numbering and the sending of the teams' strips (team_views.go).
	teamMu         sync.Mutex
	teamClockValue *sessionhost.StateClock
	teamTimer      *time.Timer
	teamSent       map[string]string
	teamClosed     bool
	teamRun        sync.Mutex
	// teamSendAfter is the number of the newest bus message already looked at for the sends of team chats, and
	// teamSendSet says the first pass, which only finds where the bus is, has been made.
	teamSendMu    sync.Mutex
	teamSendAfter int64
	teamSendSet   bool
	// teamsActive says some team exists or was asked about, so the strips are worth rebuilding; until then
	// nothing reads the bus for them.
	teamsActive       atomic.Bool
	providerLifecycle *sessionhost.ProviderLifecycle
	turnOnce          sync.Once
	turnService       *sessionhost.TurnIO
	// busService owns the bus handle and watcher; busMu guards its creation.
	busMu      sync.Mutex
	busService *bushost.Service
	// personMu guards the person's chosen name, kept in the bus and read
	// from it once (person_name.go).
	personMu     sync.Mutex
	personLoaded bool
	personChosen string
	// devinPrices are Devin's model prices, read from its command line
	// (devin_prices.go).
	devinPrices devinPriceCache

	// agentLogs are the chat logs and the protocol trace (agentlogs.go).
	agentLogs agentLogs

	ledgerOnce sync.Once
	ledger     *sessionhost.SteeringLedger
	journals   sessionhost.Journal // serializes message-time persistence
	cmu        sync.Mutex          // durable visible conversation histories and links
	// runtime operations and their settings snapshots share mu.
	runtime sessionhost.Runtime
	wakeNow func() time.Time
	// connectGraceOverride replaces how long the page has to open a chat it is
	// asked to connect: a test sets a short time, or a negative one to leave
	// the chat to the page alone.
	connectGraceOverride time.Duration
	closedOnce           sync.Once
	closedService        *sessionhost.ClosedSessions
	// commands are the commands Devin last offered, its own and the skills it
	// found, for the skills menu.
	commands []Skill

	busChatsOnce        sync.Once
	busChatsService     *sessionhost.BusChats
	subagentsOnce       sync.Once
	subagentsService    *sessionhost.SubagentService
	continuationOnce    sync.Once
	continuationService *sessionhost.Continuations
	captureOnce         sync.Once
	captureService      *sessionhost.TranscriptCapture
	sessionsOnce        sync.Once
	sessionsService     *sessionhost.SessionLifecycle
	optionsOnce         sync.Once
	optionsService      *sessionhost.Options
	permissionOnce      sync.Once
	permissionService   *sessionhost.Permissions
	qseq                atomic.Int64

	bridgeMu     sync.Mutex
	appletBridge *appletBridgeServer

	// accounts are the accounts providers report, by provider
	// (provider_account.go).
	accounts map[string]ProviderAccount

	// clocks number the state events the window owns (state_versions.go).
	clocks stateClocks

	// terminals owns the running shells (terminal_shells.go).
	terminals *terminals.Service

	// update is the installed build the window watches for Restart to
	// Update (update.go); updateStop ends the watch.
	updMu      sync.Mutex
	update     *updateWatch
	updateStop chan struct{}

	hmu sync.Mutex // the history file

	appletMu   sync.Mutex
	appletHost *host.Service

	// notices are the operating system's notifications (host_notices.go).
	notices hostNotices

	// sink, when set, also receives every event; the live tests use it in
	// place of the window.
	sink func(name string, data any)
	// events carries every event to the page (shell_server.go).
	events interface{ Emit(name string, data any) }
}

func NewApp() *App {
	settings, err := loadSettings()
	if err == nil {
		var migrated Settings
		var changed bool
		migrated, changed, err = migrateWorkspaceSettings(settings)
		if err == nil && settings.registryAbsent && settings.legacyRegistry {
			backUpSettingsBeforeSplit()
		}
		if err == nil && (changed || settings.legacyRegistry) {
			// Saving writes the registry file and leaves it out of settings.json.
			err = saveSettings(migrated)
		} else if err == nil && settings.registryAbsent {
			// Nothing else to save, but the registry file is what tells the
			// command line that the registry has been moved.
			err = workspaces.Save(configDir(), migrated.WorkspaceRecords)
		}
		if err == nil {
			settings = migrated
		} else {
			err = fmt.Errorf("load workspace identities: %w", err)
		}
	}
	native, nativeErr := readNativeWindowState()
	if err == nil {
		err = nativeErr
	}
	setLogLevel(settings.LogLevel)
	openAppLog()
	acp.Logf = func(format string, args ...any) { logf("agent", "info", format, args...) }
	// The folder is there from the start, so it can be found before anything
	// is written to it. Earlier releases kept the protocol trace beside the
	// settings; that copy is out of date and goes.
	_ = os.MkdirAll(logDir(), 0o700)
	_ = os.Remove(filepath.Join(configDir(), "acp-trace.log"))
	if err != nil {
		logf("core", "error", "starting: %v", err)
	}
	a := &App{settings: settings, startupErr: err, nativeWindow: native}
	// Each trim of the chats' event logs is logged, so the bounds can be
	// checked against real use (docs/plans/event-streams.md).
	logs := sessionhost.NewChatLogs(sessionhost.DefaultChatLogLimits)
	logs.Logf = func(level, format string, args ...any) { logf("chat", level, format, args...) }
	a.runtime.SetChatLogsLocked(logs)
	a.queueStore = newQueueStore()
	a.runtime.SetQueueStore(a.queueStore)
	a.useSpeakerNames()
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logf("core", "info", "%s %s started", product.Name, shownVersion())
	logRestartWait()
	a.startHostNotices()
	go a.sweepDrafts()
	go a.installStandardFiles()
	go pruneAgentLogs()
	_ = a.appletHostService().StartMaintenance()
	// The bus is opened here, so that the schema and upgrade checks are a
	// cost of starting and never of a call.
	if _, err := a.busHost().Bus(); err != nil {
		logf("bus", "error", "opening the bus: %v; chats will not be woken by messages", err)
	}
	a.shortenTitles()
	a.nameContinuations()
	// A chat deleted or continued while it was not running has queued messages that
	// cannot wait for it to be opened.
	go a.queueRestorer().Sweep()
	go a.detectTeams()
	a.busHost().StartWatcher(ctx)
	a.updateStop = make(chan struct{})
	go a.watchUpdates(a.updateStop)
	// Packages are looked for only where the build registered a source
	// (package_updates.go); Dinah's own build registers none.
	if len(setup.SourceIDs()) > 0 {
		go a.watchPackageUpdates(a.updateStop)
	}
}

func (a *App) shutdown(ctx context.Context) {
	// The watcher stops before the handle it shares is closed.
	a.busHost().StopWatcher()
	// The watcher admits session-close and fallback provider-stop work. Join
	// its final round before waiting, so no new work can enter the wait group.
	// Keep teardown dependencies available until that tracked work finishes.
	a.closedSessions().Wait()

	a.mu.Lock()
	if a.exitTimer != nil {
		a.exitTimer.Stop()
		a.exitTimer = nil
	}
	if a.closeTimer != nil {
		a.closeTimer.Stop()
		a.closeTimer = nil
	}
	a.closeRequest = ""
	watch := a.folderWatch
	a.mu.Unlock()
	if watch != nil {
		watch.close()
	}
	a.mu.Lock()
	extensions := a.extensions
	a.mu.Unlock()
	if extensions != nil {
		_ = extensions.Close()
	}
	a.appletMu.Lock()
	applets := a.appletHost
	a.appletMu.Unlock()
	if applets != nil {
		applets.Close()
	}
	a.lifecycle().StopAll()
	// A turn that is still ending writes its history and settings, so it is waited for before they are closed.
	a.turnIO().Close(5 * time.Second)
	a.terminalService().CloseAll()
	a.mu.Lock()
	open := make([]*Session, 0, a.runtime.ConversationCountLocked())
	for _, s := range a.runtime.ConversationsLocked() {
		open = append(open, s)
	}
	a.mu.Unlock()
	for _, s := range open {
		a.busCoordinator().LeaveBus(s)
	}
	a.busHost().CloseHandle()
	a.queueStore.Close()
	a.closeJudges()
	a.closeAppletBridge()
	a.closeAgentLogs()
	a.notices.close()
	if a.updateStop != nil {
		close(a.updateStop)
	}
	// Last, once everything this window holds is let go, the new build
	// starts when the window is closing for Restart to Update.
	a.startUpdate()
}

// State is what the page needs on load.
type State struct {
	NewWindowMode string `json:"newWindowMode"`
	// WindowModes is each kind of window's own choice (window_modes.go), and
	// WindowModesShipped the choice each ships with, which Reset returns to.
	WindowModes        map[string]string `json:"windowModes"`
	WindowModesShipped map[string]string `json:"windowModesShipped"`
	// NotifyWhenNeeded shows the operating system's notifications when an
	// agent needs the person (host_notices.go).
	NotifyWhenNeeded bool `json:"notifyWhenNeeded"`
	// PromptSendKey is "enter" or "ctrl-enter" (prompt_keys.go).
	PromptSendKey string `json:"promptSendKey"`
	// PersonName is the name the person chose, AccountName the operating
	// system's name for the account, used when none was chosen, and
	// BusHandle the person's address on the bus, which headings replace
	// with the name (person_name.go).
	PersonName  string `json:"personName"`
	AccountName string `json:"accountName"`
	BusHandle   string `json:"busHandle"`
	// DocumentViews is the view each media type opens in (document_views.go).
	DocumentViews map[string]string `json:"documentViews"`
	// OllamaApprovals is when Ollama's own tools ask the person
	// (ollama_approvals.go).
	OllamaApprovals string `json:"ollamaApprovals"`
	// OllamaFileScope and OllamaFileFolders are where Ollama's file tools
	// may go (ollama_files.go).
	OllamaFileScope   string   `json:"ollamaFileScope"`
	OllamaFileFolders []string `json:"ollamaFileFolders"`
	// OllamaRoundLimit is how many rounds of tool calls an Ollama turn may take
	// (ollama_rounds.go).
	OllamaRoundLimit OllamaRoundLimit `json:"ollamaRoundLimit"`
	// CodexApprovals is how far a Codex chat may go without asking
	// (codex_approvals.go).
	CodexApprovals string `json:"codexApprovals"`
	// FileLimits are each provider's limit on the files in one message,
	// with its documented default and source (file_limits.go).
	FileLimits map[string]ProviderFileLimit `json:"fileLimits"`
	// OptionSubstitutions are the substitutions the person stored for each
	// provider, for a chat whose saved value is not offered
	// (option_substitutions.go).
	OptionSubstitutions  map[string][]OptionSubstitution `json:"optionSubstitutions"`
	DefaultProvider      string                          `json:"defaultProvider"`
	CodexAppletApprovals string                          `json:"codexAppletApprovals"`
	Workspace            string                          `json:"workspace"`
	WorkspaceID          string                          `json:"workspaceId"`
	WorkspaceRecords     []WorkspaceRecord               `json:"workspaceRecords"`
	Workspaces           []string                        `json:"workspaces"`
	Recent               []string                        `json:"recent"`
	// MissingWorkspaces are the workspaces whose main folder is not on the
	// computer, by ID (workspace_export.go).
	MissingWorkspaces []string `json:"missingWorkspaces"`
	Approvals         string   `json:"approvals"`
	Connected         bool     `json:"connected"`

	Titles       map[string]string          `json:"titles"`
	HiddenCount  int                        `json:"hiddenCount"`
	Catalog      []acp.ConfigOption         `json:"catalog"`
	Defaults     map[string]json.RawMessage `json:"defaults"`
	Pinned       []string                   `json:"pinned"`
	ShowThinking bool                       `json:"showThinking"`
	Templates    []Template                 `json:"templates"`
	UsagePage    string                     `json:"usagePage"`
	Version      string                     `json:"version"`
	// LogLevel is how much the log keeps (applog.go).
	LogLevel string `json:"logLevel"`
	// Product is what the page shows of the product this build is
	// (product.go).
	Product ProductInfo `json:"product"`
	// PackageSources is how many package sources the build registered
	// (package_updates.go). With none, the page offers no package updates.
	PackageSources int `json:"packageSources"`
}

// State is what the page needs to draw itself. It is the page's first call
// at startup, so it does no slow work under the lock: finding Devin, which
// starts Devin to check it, is left to the calls that need Devin.
func (a *App) State() State {
	s := a.stateLocked()
	// The name is read outside the lock, since carrying it over saves the
	// settings.
	s.PersonName = a.chosenPersonName()
	// Whether a main folder is there is asked of the disk, so it is asked
	// after the lock is released.
	s.MissingWorkspaces = missingWorkspaces(s.WorkspaceRecords)
	return s
}

func (a *App) stateLocked() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := State{Workspace: a.settings.Workspace, Recent: append([]string{}, a.settings.Recent...), Approvals: a.settings.Approvals,
		WorkspaceID: a.settings.WorkspaceID, WorkspaceRecords: workspaces.Copy(a.settings.WorkspaceRecords),
		NewWindowMode: a.settings.NewWindowMode, WindowModes: effectiveWindowModes(a.settings), WindowModesShipped: effectiveWindowModes(Settings{}), NotifyWhenNeeded: !a.settings.HostNotificationsOff, PromptSendKey: promptSendKey(a.settings),
		AccountName: accountName(), BusHandle: bus.Human,
		DocumentViews: documentViews(a.settings), OllamaApprovals: ollamaApprovals(a.settings), OllamaFileScope: ollamaFileScope(a.settings), OllamaFileFolders: ollamaFileFolders(a.settings), OllamaRoundLimit: ollamaRoundLimit(a.settings), CodexApprovals: codexApprovals(a.settings),
		FileLimits: fileLimitViews(a.settings), OptionSubstitutions: optionSubstitutionViews(a.settings),
		Workspaces: append([]string{}, a.settings.Workspaces...),
		// Copies, since the state is encoded for the page after the lock is
		// released, while a chat may be renamed in the same maps.
		Titles: maps.Clone(a.settings.SessionTitles), HiddenCount: len(a.settings.Hidden), Catalog: append([]acp.ConfigOption(nil), a.settings.Catalog...), Defaults: maps.Clone(a.settings.Defaults), Pinned: append([]string(nil), a.settings.Pinned...), ShowThinking: a.settings.ShowThinking,
		Templates: append([]Template{}, a.settings.Templates...), UsagePage: a.usagePage(), Version: shownVersion(), LogLevel: a.settings.LogLevel,
		Product: productInfo(), PackageSources: len(setup.SourceIDs())}
	s.DefaultProvider = defaultProviderOf(a.settings)
	s.CodexAppletApprovals = codexAppletApprovals(a.settings.CodexAppletApprovals)
	s.Connected = a.runtime.ProviderLocked(s.DefaultProvider) != nil
	if providers.For(s.DefaultProvider).OwnPreferences {
		s.Catalog = a.settings.ProviderCatalogs[s.DefaultProvider]
		s.Defaults = a.settings.ProviderDefaults[s.DefaultProvider]
		s.Pinned = a.settings.ProviderPinned[s.DefaultProvider]
	}
	return s
}

// ChooseWorkspace asks for a folder and, when one is chosen, selects the
// workspace whose main folder it is, making one when there is none. The
// main folder is where the workspace's agents start and its flows, skills
// and document types are kept. Sessions in other workspaces go on; the page
// shows the sessions of the workspace in front.
func (a *App) ChooseWorkspace() (State, error) {
	dir, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Choose the Workspace's Main Folder", DefaultDirectory: a.currentWorkspace()})
	if err != nil || dir == "" {
		return a.State(), err
	}
	return a.SetWorkspace(dir)
}

// ForgetWorkspace takes a folder off the recent list.
func (a *App) ForgetWorkspace(dir string) (State, error) {
	if a.startupErr != nil {
		return a.State(), a.startupErr
	}
	err := a.updateSettings(func(s *Settings) error {
		var out []string
		for _, r := range s.Recent {
			if !layout.SamePath(r, dir) {
				out = append(out, r)
			}
		}
		s.Recent = out
		return nil
	})
	return a.State(), err
}

// SetApprovals sets which of the agent's actions are put to the person
// first. It takes effect when Devin next starts.
func (a *App) SetApprovals(level string) error {
	switch level {
	case ApprovalsAsk, ApprovalsDinah, ApprovalsDevin, ApprovalsAll:
	default:
		return fmt.Errorf("unknown approval level %q", level)
	}
	return a.updateSettings(func(s *Settings) error {
		s.Approvals = level
		return nil
	})
}

func (a *App) emit(name string, data any) {
	// What a team's strip shows changes with its chats' state, so any event of a chat or a notification
	// says it may have; the strips are rebuilt a moment later and sent only when they differ.
	if strings.HasPrefix(name, "agent:") || name == "notifications:added" {
		a.teamsChanged()
	}
	if a.sink != nil {
		a.sink(name, data)
	}
	if a.events != nil {
		a.events.Emit(name, data)
	}
}

// Connect starts the Devin process, if it is not running. It opens no
// session.
func (a *App) Connect() error {
	_, _, err := a.lifecycle().Live(a.defaultProvider())
	return err
}

// Stop ends the Devin process and with it every session in it. The
// sessions stay in Devin's history and can be opened again.
func (a *App) Stop() {
	a.AgentStop(a.defaultProvider())
}

// Restart stops Devin and starts it again, so that it reads its skills, its
// configuration and its MCP servers afresh, as a skill asks after installing
// or updating them. The sessions end with the old process; the page opens the
// one in front again, which replays its history.
func (a *App) Restart() error {
	a.Stop()
	return a.Connect()
}

// Answer settles a question the agent asked, with the option the person chose.
func (a *App) Answer(key, optionID string) { a.permissions().Answer(key, optionID) }

// PendingPermissions is the permission questions waiting for the person,
// each as the agent:permission event carried it. A page that missed the
// event reads them here.
func (a *App) PendingPermissions() []map[string]any { return a.permissions().Pending() }

func (a *App) permissions() *sessionhost.Permissions {
	a.permissionOnce.Do(func() {
		a.permissionService = sessionhost.NewPermissions(sessionhost.PermissionHost{
			NextKey: func() string { return fmt.Sprintf("q%d", a.qseq.Add(1)) },
			Emit:    a.emit, NoticeQuestion: a.noticeQuestion,
		})
	})
	return a.permissionService
}

// ReadFile returns a workspace file's text, for the page to show.
func (a *App) ReadFile(relPath string) (string, error) {
	a.mu.Lock()
	ws := a.settings.Workspace
	a.mu.Unlock()
	p := filepath.Join(ws, filepath.FromSlash(relPath))
	if !layout.Inside(ws, p) {
		return "", errors.New("that file is outside the workspace")
	}
	b, err := os.ReadFile(p)
	return string(b), err
}
