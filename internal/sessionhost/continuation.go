package sessionhost

import (
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"studio/internal/acp"
	"studio/internal/applet/broker"
	"studio/internal/layout"
	"sync"
)

//go:embed continuation.go
var ContinuationSource []byte

// Recovery serialization is process-wide, including independent shell instances.
var sessionRecoveryMu sync.Mutex

// Continuations shares the shell transcript and runtime locks. Host callbacks
// preserve the original call positions; callbacks used under mu must not re-enter it.
type Continuations struct {
	mu, stateMu *sync.Mutex
	runtime     *Runtime
	host        ContinuationHost
}
type ContinuationHost struct {
	StateDir          func() string
	SupportedProvider func(string) bool
	NormalizeProvider func(string) string
	Session           func(string) *Conversation
	Invocations       func(string, string) ([]broker.Invocation, error)
	HasPending        func(string) bool
	// ProviderName and WorkspaceName are the names a person reads for a provider and
	// for a workspace's folder, for the facts a confirmation shows and the label of
	// each stage. Either may be nil, and the ID or the folder is shown.
	ProviderName  func(string) string
	WorkspaceName func(string) string
	// ApplyOptions gives a chat these settings, as the person had chosen them in the
	// chat it continues, and returns the names of those the provider refused.
	ApplyOptions func(id string, options []acp.ConfigOption) []string
	// BusIdentity is the bus session ID a chat answers to, or empty when it
	// has not used the bus.
	BusIdentity func(*Conversation) string
	// NewSession starts the session a switch continues in, under the given
	// bus identity when it is not empty (SessionLifecycle.NewAdopting).
	NewSession func(workspace, provider, adopt string) (Info, error)
	// MoveBusIdentity records in the settings that the identity now belongs
	// to the chat "to" and not to "from", in one write.
	MoveBusIdentity func(from, to, id string) error
	HideSession     func(string) error
	RevokeSession   func(string)
	Emit            func(string, any)
	Title           func(string) string
	RenameSession   func(string, string) (map[string]string, error)
	ListPreferences func() (map[string]bool, map[string]string)
	Budget          func(string, []acp.ConfigOption) int
	Redact          func(string) string
	Formatter       HandoffFormatter
	InterruptedNote string
	SummaryBudget   int
}

func NewContinuations(mu, stateMu *sync.Mutex, runtime *Runtime, host ContinuationHost) *Continuations {
	return &Continuations{mu: mu, stateMu: stateMu, runtime: runtime, host: host}
}
func InterruptedHistory(tr *Transcript) bool {
	return tr.CaptureError == InterruptedTurn || (!tr.Complete && tr.CaptureError == "")
}
func (c *Continuations) readTranscript(id string) (*Transcript, error) {
	return ReadTranscript(c.host.StateDir(), id)
}
func (c *Continuations) child(id string) string {
	links, _ := ConversationLinks(c.host.StateDir())
	return links[id]
}

type SessionContinuationView struct {
	ParentSessionID string                `json:"parentSessionId"`
	History         []ConversationMessage `json:"history"`
}

type SessionContinuationPreview struct {
	// options is the settings snapshot covered by Revision. It also supplies the
	// displayed authority facts, so a concurrent update cannot mix revisions.
	options         []acp.ConfigOption
	invocations     []broker.Invocation
	SourceSessionID string                `json:"sourceSessionId"`
	Provider        string                `json:"provider"`
	History         []ConversationMessage `json:"history"`
	Context         string                `json:"context"`
	Revision        string                `json:"revision"`
	Warnings        []string              `json:"warnings"`
	// Dropped is how many earlier messages the handoff summarises.
	Dropped int `json:"dropped"`
	// Facts is what the confirmation shows (switch.go). Blocked is set, and the
	// history and context are empty, when a turn or something the chat is waiting on
	// must finish before it can continue, which Facts.Running says.
	Facts   SwitchFacts `json:"facts"`
	Blocked string      `json:"blocked,omitempty"`
}

type SessionContinuationResult struct {
	Session         Info                  `json:"session"`
	ParentSessionID string                `json:"parentSessionId"`
	History         []ConversationMessage `json:"history"`
	// OptionsNotCarried names the settings of a chat moved to another workspace that
	// the new chat's provider refused.
	OptionsNotCarried []string `json:"optionsNotCarried,omitempty"`
}

type SavedConversationView struct {
	Available bool `json:"available"`
	Complete  bool `json:"complete"`
	// Recoverable is whether the chat can be continued in a fresh chat
	// from this copy: a complete copy, or one whose last turn was cut off.
	Recoverable bool                  `json:"recoverable"`
	Warning     string                `json:"warning,omitempty"`
	Messages    []ConversationMessage `json:"messages"`
	// StreamGen and StreamThrough are set for a chat that is live in the
	// engine: the generation of its event log and the number of the entry the
	// saved messages are consistent with. The messages hold every entry up to
	// that number and none after it, so a page that has drawn them asks the
	// log for what follows and sees the turn in progress
	// (docs/plans/running-chat-catch-up.md). They are absent for a chat that
	// is not live.
	StreamGen     string `json:"streamGen,omitempty"`
	StreamThrough uint64 `json:"streamThrough,omitempty"`
}

func TranscriptHistory(stateDir func() string, id, workspace string, seen map[string]bool) ([]ConversationMessage, error) {
	if seen[id] || len(seen) >= 100 {
		return nil, errors.New("the chat continuation links contain a cycle or exceed 100 segments")
	}
	seen[id] = true
	tr, err := ReadTranscript(stateDir(), id)
	if err != nil {
		return nil, err
	}
	// The chat being continued may have had its last turn cut off, which its
	// handoff then says (continuationPreview); an earlier chat it continues
	// must be whole.
	if !tr.Complete && !(len(seen) == 1 && InterruptedHistory(tr)) {
		return nil, errors.New("the complete chat history has not finished loading")
	}
	if !layout.SamePath(tr.Workspace, workspace) {
		return nil, errors.New("chat continuation cannot cross workspaces")
	}
	history := []ConversationMessage{}
	if tr.ParentSessionID != "" {
		// A chat moved to another workspace records where it came from, so the link
		// to the chat it continues is one that was chosen and not an accident.
		parentWorkspace := workspace
		if tr.ParentWorkspace != "" {
			parentWorkspace = tr.ParentWorkspace
		}
		history, err = TranscriptHistory(stateDir, tr.ParentSessionID, parentWorkspace, seen)
		if err != nil {
			return nil, err
		}
		if tr.ParentHistory != nil {
			history = append([]ConversationMessage{}, tr.ParentHistory...)
		} else {
			return nil, errors.New("the saved continuation history is incomplete")
		}
	}
	return append(history, tr.Messages...), nil
}

func SavedSummary(stateDir func() string, id string) string {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		tr, err := ReadTranscript(stateDir(), id)
		if err != nil || seen[id] {
			return ""
		}
		seen[id] = true
		if tr.Summary != "" {
			return tr.Summary
		}
		if tr.ParentSessionID == "" {
			return ""
		}
		id = tr.ParentSessionID
	}
	return ""
}

func (c *Continuations) Guard(s *Conversation, provider string, switching bool) error {
	return c.GuardMode(s, provider, switching, false)
}

func (c *Continuations) GuardMode(s *Conversation, provider string, switching, recovery bool) error {
	return c.guard(s, provider, switching, recovery, false)
}

// guard refuses a continuation the chat is not ready for. A compaction is a
// continuation with the chat's own provider, which a switch refuses.
func (c *Continuations) guard(s *Conversation, provider string, switching, recovery, compaction bool) error {
	if !c.host.SupportedProvider(provider) {
		return errors.New("that provider does not support chat continuation")
	}
	if c.host.NormalizeProvider(s.Provider()) == provider && !recovery && !compaction {
		return errors.New("the chat already uses that provider")
	}
	if recovery && c.host.Session(s.ID()) != nil {
		return errors.New("this chat is connected or reconnecting; finish that operation before recovering its saved history")
	}
	busy := s.ContinuationBusy(switching)
	if busy {
		return &BlockedError{Reason: "finish or cancel the current turn and clear queued messages before changing provider"}
	}
	invocations, err := c.host.Invocations(s.Cwd(), s.ID())
	if err != nil {
		return err
	}
	for _, v := range invocations {
		if v.Status == "running" || v.Status == "waiting" {
			return &BlockedError{Reason: "finish or cancel this chat's pending applet work in Notifications before changing provider"}
		}
	}
	if c.host.HasPending(s.ID()) {
		return &BlockedError{Reason: "answer or dismiss pending approvals before changing provider"}
	}
	return nil
}

// Preview is what a continuation would hand the new chat.
func (c *Continuations) Preview(s *Conversation, provider string) (SessionContinuationPreview, error) {
	return c.PreviewFor(s, provider, "")
}

// PreviewFor is Preview for a continuation into the workspace folder cwd, or the chat's
// own when cwd is empty. A judge may write the summary of the older messages, which
// takes seconds, so that is done after the lock is released, and the text is the
// rule's when the judge has none.
func (c *Continuations) PreviewFor(s *Conversation, provider, cwd string) (SessionContinuationPreview, error) {
	return c.prepareFor(s, provider, cwd, "")
}

// prepareFor checks an approved revision before reading full message bodies.
// An empty revision is reserved for internal preparation and compaction.
func (c *Continuations) prepareFor(s *Conversation, provider, cwd, revision string) (SessionContinuationPreview, error) {
	c.mu.Lock()
	confirmation, err := c.confirmationLocked(s, provider, cwd)
	var p SessionContinuationPreview
	if err == nil && revision != "" && confirmation.Revision != revision {
		err = errors.New("the chat changed; review the provider switch again")
	}
	if err == nil {
		p, err = c.previewLocked(s, provider, cwd, confirmation.invocations)
	}
	c.mu.Unlock()
	if err != nil {
		return p, err
	}
	p.Revision = confirmation.Revision
	p.options = confirmation.options
	budget := c.host.Budget(provider, nil)
	res := c.host.Formatter.compact(s.ID(), p.History, budget, c.host.Formatter.Digest != nil)
	p.Context, p.Dropped = res.Text, res.Dropped
	if res.Dropped > 0 {
		p.Warnings = append(p.Warnings, fmt.Sprintf("%d earlier message(s) do not fit the context of the provider this chat moves to, so they are summarised in the handoff. The full history stays in the earlier chat, and the chat you continue in keeps its link to it.", res.Dropped))
	}
	p.Facts = c.facts(s, provider, cwd, p, res, budget)
	return p, nil
}

func (c *Continuations) previewLocked(s *Conversation, provider, cwd string, invocations []broker.Invocation) (SessionContinuationPreview, error) {
	if s.HistoryErrorLocked() != nil {
		return SessionContinuationPreview{}, fmt.Errorf("the chat history could not be saved: %w", s.HistoryErrorLocked())
	}
	tr, err := c.readTranscript(s.ID())
	if err != nil {
		return SessionContinuationPreview{}, err
	}
	if tr.CaptureError != "" && tr.CaptureError != InterruptedTurn {
		return SessionContinuationPreview{}, errors.New(tr.CaptureError)
	}
	if tr.Deleted {
		return SessionContinuationPreview{}, errors.New("this conversation was deleted")
	}
	// The guard has already refused a chat with a turn running, so a copy
	// still marked incomplete here was cut off.
	cutOff := InterruptedHistory(tr)
	if child := c.child(s.ID()); child != "" {
		return SessionContinuationPreview{}, errors.New("this chat already continues in another session")
	}
	// A switch always keeps a deterministic summary beside the new chat, so
	// that a later switch can be made when the earlier chats, or their
	// providers, are gone. Here it also stands in when the earlier history
	// cannot be read at all.
	unreadable, summary := "", ""
	history, err := TranscriptHistory(c.host.StateDir, s.ID(), s.Cwd(), map[string]bool{})
	if err != nil {
		saved := SavedSummary(c.host.StateDir, s.ID())
		if saved == "" {
			return SessionContinuationPreview{}, err
		}
		unreadable = "The earlier chats could not be read in full, so the summary saved beside this chat is handed over instead. " + err.Error()
		summary = "What came before this chat, from the summary saved beside it:\n" + saved
		history = []ConversationMessage{{Role: "limitation", Provider: tr.Provider, SessionID: s.ID(), Text: summary}}
		history = append(history, tr.Messages...)
	}
	if cutOff {
		history = append(history, ConversationMessage{Role: "limitation", Provider: tr.Provider, SessionID: s.ID(), Text: c.host.InterruptedNote})
	}
	for _, v := range invocations {
		if v.Status == "running" || v.Status == "waiting" {
			return SessionContinuationPreview{}, errors.New("finish or cancel this chat's pending applet work before changing provider")
		}
		if len(v.Artifacts) > 0 {
			history = append(history, ConversationMessage{Role: "limitation", Provider: tr.Provider, SessionID: s.ID(), Text: "Applet artifacts remain owned by the earlier chat. Their binary contents and access permissions are not transferred. Open the result in Notifications or attach it to the continued chat when needed."})
		}
		history = append(history, ConversationMessage{Role: "tool", Provider: tr.Provider, SessionID: s.ID(), Text: c.host.Redact("Applet " + v.Action + "\nStatus: " + v.Status + "\n" + v.OutputJSON + "\n" + v.Error)})
	}
	warnings := []string{}
	// The saved summary has a warning of its own below, so its whole text is
	// left out of the warnings.
	seenWarnings := map[string]bool{summary: true}
	for _, m := range history {
		if m.Role == "limitation" && !seenWarnings[m.Text] {
			warnings = append(warnings, m.Text)
			seenWarnings[m.Text] = true
		}
	}
	if unreadable != "" {
		warnings = append(warnings, unreadable)
	}
	return SessionContinuationPreview{SourceSessionID: s.ID(), Provider: provider, History: history, Warnings: warnings}, nil
}

func (c *Continuations) PreviewSession(id, provider string) (SessionContinuationPreview, error) {
	s := c.host.Session(id)
	if s == nil {
		return SessionContinuationPreview{}, errors.New("open this chat before changing provider")
	}
	if err := c.Guard(s, provider, false); err != nil {
		var blocked *BlockedError
		if errors.As(err, &blocked) {
			return c.blockedPreview(s, provider, "", blocked.Reason), nil
		}
		return SessionContinuationPreview{}, err
	}
	return c.confirmation(s, provider, "")
}

// PreviewWorkspace is what continuing the chat in the workspace folder cwd would hand
// the new chat: the same provider, in another workspace. A turn that is running is
// a fact the confirmation shows, as it is for a provider switch.
func (c *Continuations) PreviewWorkspace(id, cwd string) (SessionContinuationPreview, error) {
	s := c.host.Session(id)
	if s == nil {
		return SessionContinuationPreview{}, errors.New("open this chat before moving it to another workspace")
	}
	provider := c.host.NormalizeProvider(s.Provider())
	if err := c.checkWorkspace(s, cwd); err != nil {
		return SessionContinuationPreview{}, err
	}
	if err := c.guard(s, provider, false, false, true); err != nil {
		var blocked *BlockedError
		if errors.As(err, &blocked) {
			return c.blockedPreview(s, provider, cwd, blocked.Reason), nil
		}
		return SessionContinuationPreview{}, err
	}
	return c.confirmation(s, provider, cwd)
}

func (c *Continuations) checkWorkspace(s *Conversation, cwd string) error {
	if cwd == "" || !filepath.IsAbs(cwd) {
		return errors.New("choose an absolute workspace path first")
	}
	if layout.SamePath(s.Cwd(), cwd) {
		return errSameWorkspace
	}
	return nil
}

// blockedPreview is the preview of a continuation that cannot start yet: the facts
// the confirmation shows, with what must finish, and nothing to hand over.
func (c *Continuations) blockedPreview(s *Conversation, provider, cwd, reason string) SessionContinuationPreview {
	p := SessionContinuationPreview{SourceSessionID: s.ID(), Provider: provider, History: []ConversationMessage{}, Warnings: []string{reason}, Blocked: reason}
	p.Facts = c.facts(s, provider, cwd, p, compactResult{}, c.host.Budget(provider, nil))
	return p
}

// facts builds what the confirmation shows, from the chat and the handoff.
func (c *Continuations) facts(s *Conversation, provider, cwd string, p SessionContinuationPreview, res compactResult, budget int) SwitchFacts {
	own := c.host.NormalizeProvider(s.Provider())
	kind := SwitchProvider
	switch {
	case cwd != "":
		kind = SwitchWorkspace
	case provider == own:
		kind = SwitchCompaction
	}
	options := p.options
	if options == nil {
		options = s.Options()
	}
	model := ""
	for _, o := range options {
		if o.Category == "model" {
			model = optionValueName(o)
		}
	}
	f := SwitchFacts{Kind: kind, Carried: []OptionFact{}, NotCarried: []OptionFact{}}
	f.From = SwitchEnd{Provider: own, ProviderName: c.providerName(own), Model: model, Workspace: s.Cwd(), WorkspaceName: c.workspaceName(s.Cwd())}
	f.To = SwitchEnd{Provider: provider, ProviderName: c.providerName(provider), Workspace: s.Cwd(), WorkspaceName: c.workspaceName(s.Cwd())}
	if cwd != "" {
		f.To.Workspace, f.To.WorkspaceName = cwd, c.workspaceName(cwd)
		f.To.Model = model
	}
	if kind == SwitchWorkspace {
		carried, notCarried := splitOptions(options)
		f.Carried, f.NotCarried = optionFacts(carried), optionFacts(notCarried)
	} else {
		f.NotCarried = optionFacts(options)
	}
	if budget > 0 {
		f.Handoff = &HandoffFacts{Messages: len(p.History), Whole: len(p.History) - res.Dropped, Summarised: res.Dropped, Chars: len(res.Text), Budget: budget, Judged: res.Judged}
	}
	f.Running = c.runningFacts(s)
	return f
}

func (c *Continuations) providerName(id string) string {
	if c.host.ProviderName != nil {
		if name := c.host.ProviderName(id); name != "" {
			return name
		}
	}
	return id
}

func (c *Continuations) workspaceName(cwd string) string {
	if c.host.WorkspaceName != nil {
		if name := c.host.WorkspaceName(cwd); name != "" {
			return name
		}
	}
	return cwd
}

// runningFacts is what must finish before a continuation can start.
func (c *Continuations) runningFacts(s *Conversation) RunningFacts {
	st := s.State()
	r := RunningFacts{Turn: st.Info.Working, Queued: len(st.Info.Queued), Approvals: c.host.HasPending(s.ID())}
	r.CanStop = r.Turn
	if invocations, err := c.host.Invocations(s.Cwd(), s.ID()); err == nil {
		for _, v := range invocations {
			if v.Status == "running" || v.Status == "waiting" {
				r.Applets++
			}
		}
	}
	return r
}

func (c *Continuations) ContinueSession(id, provider, revision string) (SessionContinuationResult, error) {
	s := c.host.Session(id)
	if s == nil {
		return SessionContinuationResult{}, errors.New("open this chat before changing provider")
	}
	return c.Continue(s, provider, revision, false)
}

func (c *Continuations) Continue(s *Conversation, provider, revision string, recovery bool) (SessionContinuationResult, error) {
	return c.continueWith(s, continueOpts{provider: provider, revision: revision, recovery: recovery})
}

// ContinueInWorkspace moves a chat to another workspace by continuing it there: a new
// chat with the same provider in the folder cwd, given the summary and the recent
// turns, and linked to the old one, which stays where it was. The agent's history and
// the provider's reopening both depend on a chat's folder, so the folder of an
// existing chat is never changed.
func (c *Continuations) ContinueInWorkspace(id, cwd, revision string) (SessionContinuationResult, error) {
	s := c.host.Session(id)
	if s == nil {
		return SessionContinuationResult{}, errors.New("open this chat before moving it to another workspace")
	}
	if err := c.checkWorkspace(s, cwd); err != nil {
		return SessionContinuationResult{}, err
	}
	return c.continueWith(s, continueOpts{provider: c.host.NormalizeProvider(s.Provider()), revision: revision, cwd: cwd})
}

// Compact continues a chat with its own provider in a new session that is given
// the history to the provider's budget, as a handoff, so that a chat whose
// context is full goes on without its earliest turns. Nothing is shown for the
// person to review first: the person chose this course, and the full history
// stays in the earlier chat, which the new one links to.
func (c *Continuations) Compact(s *Conversation) (SessionContinuationResult, error) {
	return c.continueWith(s, continueOpts{provider: c.host.NormalizeProvider(s.Provider()), compaction: true})
}

// continueOpts say what a continuation is for. cwd is the workspace folder the new
// chat is in, and empty for the chat's own.
type continueOpts struct {
	provider, revision, cwd string
	recovery, compaction    bool
}

// kind is what the continuation is called in the chat's information.
func (o continueOpts) kind(own string) string {
	switch {
	case o.cwd != "":
		return SwitchWorkspace
	case o.compaction || o.provider == own:
		return SwitchCompaction
	}
	return SwitchProvider
}

// setStage records the stage a switch is in, in the chat's numbered state, and tells
// the page.
func (c *Continuations) setStage(s *Conversation, kind, to, stage, label string) {
	s.SetSwitch(&SwitchState{Kind: kind, To: to, Stage: stage, Label: label})
	EmitSession(c.host.Emit, s)
}

func (c *Continuations) continueWith(s *Conversation, o continueOpts) (result SessionContinuationResult, err error) {
	id := s.ID()
	provider, revision := o.provider, o.revision
	// A workspace move is a continuation with the chat's own provider, which a switch
	// refuses, and a compaction is not reviewed first, which a move is.
	if err := c.guard(s, provider, false, o.recovery, o.compaction || o.cwd != ""); err != nil {
		return SessionContinuationResult{}, err
	}
	if !s.BeginContinuation() {
		return SessionContinuationResult{}, errors.New("the chat changed; review the provider switch again")
	}
	kind := o.kind(c.host.NormalizeProvider(s.Provider()))
	to := c.providerName(provider)
	startingLabel := "Starting " + to
	if o.cwd != "" {
		to = c.workspaceName(o.cwd)
		startingLabel = "Starting the chat in " + to
	}
	defer func() {
		s.SetSwitching(false)
		if err != nil {
			// The chat is as it was; the information says why in plain words, until the
			// person dismisses it.
			s.SetSwitch(&SwitchState{Kind: kind, To: to, Failed: err.Error()})
		} else {
			s.SetSwitch(nil)
		}
		EmitSession(c.host.Emit, s)
	}()
	c.setStage(s, kind, to, StageSummary, "Building the summary of the conversation")
	if revision == "" && !o.compaction {
		return SessionContinuationResult{}, errors.New("the chat changed; review the provider switch again")
	}
	preview, err := c.prepareFor(s, provider, o.cwd, revision)
	if err != nil {
		return SessionContinuationResult{}, err
	}
	if o.compaction {
		revision = preview.Revision
	}
	busID := c.host.BusIdentity(s)
	cwd := s.Cwd()
	if o.cwd != "" {
		cwd = o.cwd
	}
	c.setStage(s, kind, to, StageStarting, startingLabel)
	info, err := c.host.NewSession(cwd, provider, busID)
	if err != nil {
		return SessionContinuationResult{}, err
	}
	next := c.host.Session(info.ID)
	if next == nil {
		return SessionContinuationResult{}, errors.New("the new provider disconnected")
	}
	cleanup := func() {
		c.stateMu.Lock()
		c.runtime.RemoveConversationLocked(info.ID)
		c.stateMu.Unlock()
		_ = c.host.HideSession(info.ID)
		c.host.RevokeSession(info.ID)
		next.Close()
		EmitSession(c.host.Emit, next)
	}
	// A move to another workspace keeps the chat's settings that grant no authority.
	var notCarried []string
	if o.cwd != "" && c.host.ApplyOptions != nil {
		carried, _ := splitOptions(preview.options)
		notCarried = c.host.ApplyOptions(next.ID(), carried)
	}
	c.setStage(s, kind, to, StageHandover, "Handing the conversation over")
	latest, err := c.confirmation(s, provider, o.cwd)
	if err != nil || latest.Revision != revision {
		cleanup()
		return SessionContinuationResult{}, errors.New("the chat changed while connecting; review the provider switch again")
	}
	if err := c.guard(s, provider, true, o.recovery, o.compaction || o.cwd != ""); err != nil {
		cleanup()
		return SessionContinuationResult{}, err
	}
	c.mu.Lock()
	parentHistory := append(append([]ConversationMessage{}, preview.History...), ConversationMessage{Role: "provider_switch", Text: switchNote(kind, provider, o.cwd), Provider: provider, SessionID: info.ID})
	err = next.HistoryBeginHandoffLocked(c.host.StateDir(), id, s.Cwd(), preview.Context, c.host.Formatter.Summary(preview.History, c.host.SummaryBudget), parentHistory)
	c.mu.Unlock()
	if err != nil {
		cleanup()
		return SessionContinuationResult{}, err
	}
	if busID != "" {
		if err := c.moveBusIdentity(s, next, busID); err != nil {
			cleanup()
			return SessionContinuationResult{}, err
		}
	}
	c.setStage(s, kind, to, StageOpening, "Opening the new chat")
	c.host.RevokeSession(s.ID())
	s.SetSuperseded(true)
	next.SetSwitching(false)
	title := c.host.Title(id)
	if title == "" {
		title = s.Info().Title
	}
	if title != "" {
		_, _ = c.host.RenameSession(info.ID, title)

		next.UpdateDisplay(&title, nil)
	}
	history := append(append([]ConversationMessage{}, preview.History...), ConversationMessage{Role: "provider_switch", Text: switchNote(kind, provider, o.cwd), Provider: provider, SessionID: info.ID})
	return SessionContinuationResult{Session: next.InfoVersioned(), ParentSessionID: id, History: history, OptionsNotCarried: notCarried}, nil
}

// switchNote is the line the conversation keeps where the chat was continued.
func switchNote(kind, provider, cwd string) string {
	if kind == SwitchWorkspace {
		return "Continued in " + cwd
	}
	return "Continued with " + provider
}

// moveBusIdentity gives the new chat the old chat's place on the bus. The old
// chat lets go first, so that no moment has both holding the identity (a bus
// message in between waits on the bus), and it lets go without ending the
// identity, which the new chat now uses. If the settings write fails, the old
// chat gets the identity back and the new one lets go of it.
func (c *Continuations) moveBusIdentity(old, next *Conversation, id string) error {
	hadIt := old.LeaveBus() != ""
	next.JoinBus(id)
	if err := c.host.MoveBusIdentity(old.ID(), next.ID(), id); err != nil {
		next.LeaveBus()
		if hadIt {
			old.JoinBus(id)
		}
		return fmt.Errorf("keep the chat's place on the bus: %w", err)
	}
	return nil
}

func (c *Continuations) View(id string) (SessionContinuationView, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tr, err := c.readTranscript(id)
	if err != nil {
		return SessionContinuationView{History: []ConversationMessage{}}, nil
	}
	v := SessionContinuationView{ParentSessionID: tr.ParentSessionID, History: []ConversationMessage{}}
	if tr.ParentSessionID == "" {
		return v, nil
	}
	if tr.ParentHistory == nil {
		return v, errors.New("the saved continuation history is incomplete")
	}
	v.History = append([]ConversationMessage{}, tr.ParentHistory...)
	return v, nil
}

func (c *Continuations) Saved(id string) SavedConversationView {
	// The file is read, and the log number it is consistent with taken, under
	// the transcript mutex that every write of the file holds.
	var s *Conversation
	if c.host.Session != nil {
		s = c.host.Session(id)
	}
	c.mu.Lock()
	tr, err := c.readTranscript(id)
	var gen string
	var through uint64
	var live bool
	if s != nil {
		gen, through, live = s.HistoryThroughLocked()
	}
	c.mu.Unlock()
	if err != nil || tr.Deleted {
		return SavedConversationView{Messages: []ConversationMessage{}}
	}
	messages := append(append([]ConversationMessage{}, tr.ParentHistory...), tr.Messages...)
	warning, cutOff := "", InterruptedHistory(tr)
	switch {
	case cutOff:
		warning = "The last turn was cut off before it finished, so its end may be missing from this saved copy."
	case !tr.Complete || tr.CaptureError != "":
		warning = "This saved history may be incomplete. " + tr.CaptureError
	}
	missingEarlier := tr.ParentSessionID != "" && len(tr.ParentHistory) == 0
	if missingEarlier {
		warning += " Earlier conversation history is missing from this saved copy."
		if tr.Summary != "" {
			messages = append([]ConversationMessage{{Role: "limitation", Text: "Saved summary of earlier history:\n" + tr.Summary}}, messages...)
		}
	}
	complete := warning == ""
	recoverable := len(messages) > 0 && (complete || cutOff && !missingEarlier)
	view := SavedConversationView{Available: len(messages) > 0, Complete: complete, Recoverable: recoverable, Warning: strings.TrimSpace(warning), Messages: messages}
	if live {
		view.StreamGen, view.StreamThrough = gen, through
	}
	return view
}

func (c *Continuations) PendingContext(s *Conversation) (string, error) {
	provider, options := s.Provider(), append([]acp.ConfigOption{}, s.Options()...)

	c.mu.Lock()
	defer c.mu.Unlock()
	tr := s.HistorySavedLocked()
	if tr == nil || !tr.PendingHandoff {
		return "", nil
	}
	if tr.Handoff == "" {
		return "", errors.New("the saved provider handoff is unavailable")
	}
	budget := c.host.Budget(provider, options)
	if len(tr.Handoff) <= budget {
		return tr.Handoff, nil
	}
	// The session's own window is smaller than the one the handoff was built
	// for, as an Ollama model's context length can be, so the handoff is
	// built again for it from the history kept beside it.
	if text, _ := c.host.Formatter.Compact(tr.ParentHistory, budget); text != "" {
		return text, nil
	}
	return ClipText(tr.Handoff, budget), nil
}

func (c *Continuations) Resolve(id string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	links, records := ConversationLinks(c.host.StateDir())
	seen := map[string]bool{}
	for {
		if seen[id] || len(seen) >= 100 {
			return "", errors.New("the chat continuation links contain a cycle")
		}
		seen[id] = true
		if tr := records[id]; tr != nil && tr.Deleted && links[id] == "" {
			return "", errors.New("this conversation was deleted; its old links cannot reopen it")
		}
		child := links[id]
		if child == "" {
			return id, nil
		}
		id = child
	}
}

func (c *Continuations) FilterAncestors(in []Info, workspace string) []Info {
	hidden, titles := c.host.ListPreferences()
	c.mu.Lock()
	defer c.mu.Unlock()
	links, records := ConversationLinks(c.host.StateDir())
	seen := map[string]bool{}
	out := make([]Info, 0, len(in))
	for _, s := range in {
		seen[s.ID] = true
		tr := records[s.ID]
		if links[s.ID] == "" && !hidden[s.ID] && (tr == nil || !tr.Deleted) {
			out = append(out, s)
		}
	}
	for id, tr := range records {
		if tr.ParentSessionID != "" && !tr.Deleted && links[id] == "" && !seen[id] && !hidden[id] && layout.SamePath(tr.Workspace, workspace) {
			out = append(out, Info{ID: id, Provider: tr.Provider, Cwd: tr.Workspace, Title: titles[id]})
		}
	}
	return out
}

func (c *Continuations) RecoverySession(id string) (*Conversation, error) {
	c.mu.Lock()
	tr, err := c.readTranscript(id)
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	// A copy whose last turn was cut off holds everything before it, and the
	// handoff says the turn's end is missing (continuationPreview).
	if tr.Deleted || ((!tr.Complete || tr.CaptureError != "") && !InterruptedHistory(tr)) {
		return nil, errors.New("the saved history is incomplete; save the available history before starting a new chat")
	}
	title := c.host.Title(id)
	s := NewConversation(Identity{ID: id, Provider: tr.Provider, Cwd: tr.Workspace, Title: title}, InitialActivity{Open: true}, false, false)
	c.mu.Lock()
	s.HistoryRestoreSavedLocked(tr)
	c.mu.Unlock()
	return s, nil
}

func (c *Continuations) PreviewRecovery(id, provider string) (SessionContinuationPreview, error) {
	c.mu.Lock()
	tr, _, err := readConfirmation(TranscriptPath(c.host.StateDir(), id), id)
	c.mu.Unlock()
	if err != nil {
		return SessionContinuationPreview{}, err
	}
	if tr.Deleted || tr.CaptureError != "" && tr.CaptureError != InterruptedTurn {
		return SessionContinuationPreview{}, errors.New("the saved history is incomplete; save the available history before starting a new chat")
	}
	s := NewConversation(Identity{ID: id, Provider: tr.Provider, Cwd: tr.Workspace}, InitialActivity{Open: true}, false, false)
	if err := c.GuardMode(s, provider, false, true); err != nil {
		return SessionContinuationPreview{}, err
	}
	return c.confirmation(s, provider, "")
}

func (c *Continuations) Recover(id, provider, revision string) (SessionContinuationResult, error) {
	sessionRecoveryMu.Lock()
	defer sessionRecoveryMu.Unlock()
	s, err := c.RecoverySession(id)
	if err != nil {
		return SessionContinuationResult{}, err
	}
	return c.Continue(s, provider, revision, true)
}
