// Package servicechats is the plain port through which a product's own
// service works in chats (docs/design/service-chats.md). A product built
// from this source registers a window service whose factory receives a Port
// (services.RegisterChatWindow). Through it the service opens chats, takes
// exclusive turns in them, and uses the few things around a turn that the
// window already has: the chat's model, skills, the agent programs, applet
// actions and the service's change notice.
//
// This port has no review and no confirmation of a run. The task host
// (internal/taskhost) has both, and it stays the port for work whose every
// operation is reviewed before it starts. A service that uses this port is
// responsible for asking the person before it starts work. The owner decided
// on 21 November 2026 that the base offers this second port.
//
// The package has the port, its errors and the window that admits, tracks
// and joins the calls. What a call does to a chat is the Backend's, which
// the desktop program implements with the functions its own methods use.
package servicechats

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
)

// The refusals of a chat. Each of them means that nothing was sent to the
// agent.
var (
	// ErrBusy refuses a chat that is working: a turn of the person's, of
	// another chat's message or of this port is running in it, or work the
	// agent left in the background is. The prompt is not queued and is not
	// steered into the running turn.
	ErrBusy = errors.New("the chat is working")
	// ErrSwitching refuses a chat that is changing its agent program.
	ErrSwitching = errors.New("the chat is switching programs")
	// ErrRemoved refuses a chat that is in the Recycle Bin or deleted.
	ErrRemoved = errors.New("the chat is deleted")
	// ErrReleased refuses a chat the service released through this port.
	ErrReleased = errors.New("the service released the chat")
	// ErrNotOpened refuses a chat the service did not open through this
	// port, for a method that acts only on such chats.
	ErrNotOpened = errors.New("the service did not open this chat")
	// ErrClosed refuses every call once the window is shutting down.
	ErrClosed = errors.New("the window is closing")
)

// ErrNotSent marks a failure of a turn before the prompt was handed to the
// agent's connection. The turn may be tried again.
var ErrNotSent = errors.New("the prompt was not sent")

// ErrUnknownOutcome marks a turn, an opening or an action whose outcome
// could not be established: the request may or may not have reached the
// agent or the action. The port never sends it again, and a service must
// not either without the person's word.
var ErrUnknownOutcome = errors.New("the outcome is unknown")

// Other refusals.
var (
	// ErrNotWorkspace refuses a folder that is not a saved workspace.
	ErrNotWorkspace = errors.New("the folder is not a workspace")
	// ErrProgram refuses an agent program this product does not offer.
	ErrProgram = errors.New("the agent program is not offered")
	// ErrModel refuses a model the chat's agent does not offer, and a chat
	// whose agent offers no choice of model.
	ErrModel = errors.New("the agent does not offer this model")
	// ErrModelChanged refuses the restoring of a model after somebody else
	// changed the chat's settings.
	ErrModelChanged = errors.New("the chat's settings changed after the model was set")
	// ErrNeedsPerson refuses an applet action that requires the person's
	// approval of every invocation, which this port cannot ask for.
	ErrNeedsPerson = errors.New("the action requires the person's approval of every invocation")
)

// OpenRequest says which chat to open.
type OpenRequest struct {
	// Workspace is the folder of a saved workspace.
	Workspace string
	// Program is the identity of an agent program this product offers.
	Program string
	// Title is the chat's name in the lists. An empty title leaves the
	// agent's own.
	Title string
}

// Chat is a chat the port opened.
type Chat struct {
	ID        string `json:"id"`
	Workspace string `json:"workspace"`
	Program   string `json:"program"`
	Title     string `json:"title"`
}

// Prompt is what a turn sends. Label is the short name the transcript shows
// for the sender in place of the person.
type Prompt struct {
	Text  string
	Label string
}

// Reply is the end of a turn. StopReason is the agent's own, such as
// "end_turn", "max_tokens", "refusal" or "cancelled".
type Reply struct {
	Text       string `json:"text"`
	StopReason string `json:"stopReason"`
}

// Model is a chat's choice of model.
type Model struct {
	// Value is the identity of the model in use, and Name is its name.
	Value string `json:"value"`
	Name  string `json:"name"`
	// Offered are the identities the agent offers for this chat.
	Offered []ModelChoice `json:"offered"`
}

// ModelChoice is a model the agent offers.
type ModelChoice struct {
	Value string `json:"value"`
	Name  string `json:"name"`
}

// Program is an agent program this product offers.
type Program struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ProgramState says whether an agent program can be reached.
type ProgramState struct {
	Program string `json:"program"`
	// Offered says whether the product offers the program at all.
	Offered bool `json:"offered"`
	// Installed says whether the program was found on this computer.
	Installed bool `json:"installed"`
	// Reachable says whether the program answered the window's check.
	Reachable bool `json:"reachable"`
	// Status is the window's status code for the program, and Reason says
	// in plain words what is wrong when something is.
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// Change is the content of the event "service:<name>:changed".
type Change struct {
	Service   string `json:"service"`
	Workspace string `json:"workspace"`
	Key       string `json:"key"`
}

// Action describes a registered applet action.
type Action struct {
	Reference    string          `json:"reference"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema"`
	Replay       string          `json:"replay"`
	Effect       string          `json:"effect"`
	Capabilities []string        `json:"capabilities"`
	Digest       string          `json:"digest"`
	// NeedsPerson says that RunAction refuses the action (ErrNeedsPerson).
	NeedsPerson bool `json:"needsPerson"`
}

// ActionRun is one invocation of an applet action.
type ActionRun struct {
	// ID is the invocation's identity, 32 lowercase hexadecimal digits. With
	// an empty ID the port makes one. A service that saves the identity
	// before the call can read the same invocation again after a restart by
	// running it with the same identity and the same other fields.
	ID string
	// Workspace is the folder of a saved workspace.
	Workspace string
	Reference string
	Input     json.RawMessage
	// Digest is the digest the service read with DescribeAction. An action
	// whose implementation changed since is refused. An empty digest accepts
	// the action as it is now.
	Digest string
	// Deadline is required, and is at most 24 hours ahead.
	Deadline time.Time
}

// ActionResult is the end of an invocation.
type ActionResult struct {
	ID     string          `json:"id"`
	Output json.RawMessage `json:"output"`
}

// Backend does the work of the port's calls in the window. Every method is
// called with the registered name of the service, which the service cannot
// choose. The methods return the errors of this package for the refusals
// they name.
type Backend interface {
	OpenChat(ctx context.Context, service string, request OpenRequest) (Chat, error)
	// TakeTurn takes one exclusive turn. With opened set, the chat must be
	// one the service opened through the port.
	TakeTurn(ctx context.Context, service, chat string, prompt Prompt, opened bool) (Reply, error)
	Model(ctx context.Context, service, chat string) (Model, error)
	UseModel(ctx context.Context, service, chat, model string) (restore func(context.Context) error, err error)
	RenameChat(ctx context.Context, service, chat, title string) error
	ReleaseChat(ctx context.Context, service, chat string) error
	SkillAvailable(ctx context.Context, workspace, program, skill string) (bool, error)
	SkillPrompt(ctx context.Context, workspace, program, text string) (string, error)
	Programs(ctx context.Context) ([]Program, error)
	CheckProgram(ctx context.Context, program string) (ProgramState, error)
	PublishChange(ctx context.Context, change Change) error
	DescribeAction(ctx context.Context, reference string) (Action, error)
	RunAction(ctx context.Context, service string, run ActionRun) (ActionResult, error)
	CancelAction(ctx context.Context, service, workspace, id string) error
}

// Window admits the calls of every service's port, knows the turns that are
// running, and joins them all at shutdown.
type Window struct {
	backend Backend
	ctx     context.Context
	stop    context.CancelFunc

	mu     sync.Mutex
	closed bool
	active int
	idle   chan struct{}
	turns  map[turnKey]*runningTurn
}

type turnKey struct{ service, chat string }

type runningTurn struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// New makes the window's side of the port.
func New(backend Backend) *Window {
	ctx, stop := context.WithCancel(context.Background())
	return &Window{backend: backend, ctx: ctx, stop: stop, idle: make(chan struct{}), turns: map[turnKey]*runningTurn{}}
}

var serviceName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ForService returns the port of the service registered under the name.
func (w *Window) ForService(name string) Port {
	if !serviceName.MatchString(name) {
		panic("servicechats: invalid service name " + name)
	}
	return Port{window: w, service: name}
}

// CloseAdmission refuses every later call and cancels the context of every
// call that is running, which stops the turns among them.
func (w *Window) CloseAdmission() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		if w.active == 0 {
			close(w.idle)
		}
	}
	w.mu.Unlock()
	w.stop()
}

// Shutdown closes admission and waits for the running calls to return. It
// starts no goroutine. An error says that calls were still running when the
// context ended.
func (w *Window) Shutdown(ctx context.Context) error {
	if w == nil {
		return nil
	}
	w.CloseAdmission()
	select {
	case <-w.idle:
		return nil
	case <-ctx.Done():
		return errors.New("calls of a service's chat port were still running at the end of the shutdown budget")
	}
}

// Running is the number of calls that have not returned.
func (w *Window) Running() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.active
}

// begin admits one call. The context it returns ends with the caller's and
// with the window's admission.
func (w *Window) begin(ctx context.Context) (context.Context, func(), error) {
	if w == nil {
		return nil, nil, ErrClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil, nil, ErrClosed
	}
	w.active++
	w.mu.Unlock()
	callCtx, cancel := context.WithCancel(ctx)
	unhook := context.AfterFunc(w.ctx, cancel)
	return callCtx, func() {
		unhook()
		cancel()
		w.mu.Lock()
		w.active--
		if w.closed && w.active == 0 {
			close(w.idle)
		}
		w.mu.Unlock()
	}, nil
}

// Port is one service's access to chats. The zero Port refuses every call.
type Port struct {
	window  *Window
	service string
}

// Service is the registered name the port acts under.
func (p Port) Service() string { return p.service }

// OpenChat opens a new chat in a workspace with an agent program and gives
// it a title. The chat is an ordinary chat: it is in the Chats list, and the
// person can open, read, rename and delete it. A failure that wraps
// ErrUnknownOutcome may have left a chat with the agent, whose identity the
// returned Chat has when the agent gave one.
func (p Port) OpenChat(ctx context.Context, request OpenRequest) (Chat, error) {
	ctx, end, err := p.window.begin(ctx)
	if err != nil {
		return Chat{}, err
	}
	defer end()
	if request.Workspace == "" || request.Program == "" {
		return Chat{}, errors.New("a chat needs a workspace folder and an agent program")
	}
	return p.window.backend.OpenChat(ctx, p.service, request)
}

// TakeTurn sends a prompt to a chat the service opened through this port
// and waits for the end of the agent's turn. The turn is exclusive: a chat
// that is working, switching programs, deleted or released is refused at
// once with ErrBusy, ErrSwitching, ErrRemoved or ErrReleased, and nothing is
// sent, queued or steered.
//
// A failure that wraps ErrNotSent, like every refusal, means the agent did
// not receive the prompt. A failure that wraps ErrUnknownOutcome means it
// may have, and the prompt is not sent again. When the context ends or
// CancelTurn is called, the agent is asked to stop its turn. A turn the
// agent then ends has the stop reason "cancelled" and what it had written,
// with the context's error when the context ended and ErrClosed when the
// window closed.
func (p Port) TakeTurn(ctx context.Context, chat string, prompt Prompt) (Reply, error) {
	return p.turn(ctx, chat, prompt, true)
}

// TakeTurnInExistingChat takes the same exclusive turn in any chat of the
// person's, by its identity. It is for a chat the person chose for the
// service's work. The refusals and the outcomes are those of TakeTurn. The
// other methods of the port do not act on such a chat, except CancelTurn
// for the turn this method started.
func (p Port) TakeTurnInExistingChat(ctx context.Context, chat string, prompt Prompt) (Reply, error) {
	return p.turn(ctx, chat, prompt, false)
}

func (p Port) turn(ctx context.Context, chat string, prompt Prompt, opened bool) (Reply, error) {
	caller := ctx
	ctx, end, err := p.window.begin(ctx)
	if err != nil {
		return Reply{}, err
	}
	defer end()
	if chat == "" || strings.TrimSpace(prompt.Text) == "" {
		return Reply{}, errors.New("a turn needs a chat and a prompt")
	}
	w := p.window
	key := turnKey{p.service, chat}
	turnCtx, cancel := context.WithCancel(ctx)
	running := &runningTurn{cancel: cancel, done: make(chan struct{})}
	w.mu.Lock()
	if w.turns[key] != nil {
		w.mu.Unlock()
		cancel()
		return Reply{}, ErrBusy
	}
	w.turns[key] = running
	w.mu.Unlock()
	defer func() {
		cancel()
		w.mu.Lock()
		delete(w.turns, key)
		w.mu.Unlock()
		close(running.done)
	}()
	reply, err := w.backend.TakeTurn(turnCtx, p.service, chat, prompt, opened)
	if err == nil && reply.StopReason == "cancelled" {
		if caller != nil && caller.Err() != nil {
			return reply, caller.Err()
		}
		if w.ctx.Err() != nil {
			return reply, ErrClosed
		}
	}
	return reply, err
}

// CancelTurn asks the agent to stop the turn this service's port started in
// the chat, and waits until that turn has returned or the context ends. It
// reports whether such a turn was running. It never stops a turn the person
// or anything else started.
func (p Port) CancelTurn(ctx context.Context, chat string) (bool, error) {
	w := p.window
	if w == nil {
		return false, ErrClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	w.mu.Lock()
	running := w.turns[turnKey{p.service, chat}]
	w.mu.Unlock()
	if running == nil {
		return false, nil
	}
	running.cancel()
	select {
	case <-running.done:
		return true, nil
	case <-ctx.Done():
		return true, ctx.Err()
	}
}

// Model reads the model of a chat the service opened.
func (p Port) Model(ctx context.Context, chat string) (Model, error) {
	ctx, end, err := p.window.begin(ctx)
	if err != nil {
		return Model{}, err
	}
	defer end()
	return p.window.backend.Model(ctx, p.service, chat)
}

// UseModel sets the model of a chat the service opened, by the identity the
// agent gives it. A chat that is working is refused with ErrBusy. The
// function returned puts the earlier model back. It refuses with
// ErrModelChanged when the chat's settings were changed by somebody else in
// between, so that the person's later choice is never undone.
func (p Port) UseModel(ctx context.Context, chat, model string) (restore func(context.Context) error, err error) {
	ctx, end, err := p.window.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer end()
	if model == "" {
		return nil, ErrModel
	}
	undo, err := p.window.backend.UseModel(ctx, p.service, chat, model)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) error {
		ctx, end, err := p.window.begin(ctx)
		if err != nil {
			return err
		}
		defer end()
		return undo(ctx)
	}, nil
}

// RenameChat gives a chat the service opened another title.
func (p Port) RenameChat(ctx context.Context, chat, title string) error {
	ctx, end, err := p.window.begin(ctx)
	if err != nil {
		return err
	}
	defer end()
	if strings.TrimSpace(title) == "" {
		return errors.New("a chat's title is not empty")
	}
	return p.window.backend.RenameChat(ctx, p.service, chat, title)
}

// ReleaseChat ends the service's use of a chat it opened. The window no
// longer keeps the chat loaded for the service, and later turns of the
// service in it are refused with ErrReleased. The chat and its history stay
// with the person. A chat with a turn of this port running is refused with
// ErrBusy.
func (p Port) ReleaseChat(ctx context.Context, chat string) error {
	ctx, end, err := p.window.begin(ctx)
	if err != nil {
		return err
	}
	defer end()
	p.window.mu.Lock()
	running := p.window.turns[turnKey{p.service, chat}]
	p.window.mu.Unlock()
	if running != nil {
		return ErrBusy
	}
	return p.window.backend.ReleaseChat(ctx, p.service, chat)
}

// SkillAvailable says whether a skill of that name can be used with the
// agent program in the workspace.
func (p Port) SkillAvailable(ctx context.Context, workspace, program, skill string) (bool, error) {
	ctx, end, err := p.window.begin(ctx)
	if err != nil {
		return false, err
	}
	defer end()
	return p.window.backend.SkillAvailable(ctx, workspace, program, strings.TrimPrefix(skill, "/"))
}

// SkillPrompt turns text that starts with "/" and a skill's name into the
// prompt the agent program needs. A program that reads skills itself gets
// the text as it is, and another program gets the skill's instructions with
// the text. Text that starts with no skill's name comes back unchanged. A turn prepares
// its prompt in the same way, so a service needs this method only to see or
// keep the prepared text.
func (p Port) SkillPrompt(ctx context.Context, workspace, program, text string) (string, error) {
	ctx, end, err := p.window.begin(ctx)
	if err != nil {
		return "", err
	}
	defer end()
	return p.window.backend.SkillPrompt(ctx, workspace, program, text)
}

// Programs lists the agent programs this product offers.
func (p Port) Programs(ctx context.Context) ([]Program, error) {
	ctx, end, err := p.window.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer end()
	return p.window.backend.Programs(ctx)
}

// CheckProgram says whether an agent program can be reached, by the check
// the window's settings use. The check starts the program when it is
// installed and not running. It sends no prompt.
func (p Port) CheckProgram(ctx context.Context, program string) (ProgramState, error) {
	ctx, end, err := p.window.begin(ctx)
	if err != nil {
		return ProgramState{}, err
	}
	defer end()
	return p.window.backend.CheckProgram(ctx, program)
}

// PublishChange sends the event "service:<name>:changed" for this service,
// with a workspace folder and a key of the service's own, so that its page
// reads the changed record again. The event has no content of the record.
func (p Port) PublishChange(ctx context.Context, workspace, key string) error {
	ctx, end, err := p.window.begin(ctx)
	if err != nil {
		return err
	}
	defer end()
	return p.window.backend.PublishChange(ctx, Change{Service: p.service, Workspace: workspace, Key: key})
}

// DescribeAction describes a registered applet action and gives its digest.
func (p Port) DescribeAction(ctx context.Context, reference string) (Action, error) {
	ctx, end, err := p.window.begin(ctx)
	if err != nil {
		return Action{}, err
	}
	defer end()
	return p.window.backend.DescribeAction(ctx, reference)
}

// ActionDigest gives the digest of a registered applet action as it is now.
func (p Port) ActionDigest(ctx context.Context, reference string) (string, error) {
	action, err := p.DescribeAction(ctx, reference)
	return action.Digest, err
}

// RunAction runs a registered applet action for the service and waits for
// its end. The invocation is recorded as the service's own. When the context
// ends, the invocation is cancelled. A failure that wraps ErrUnknownOutcome
// means the action may have had its effect.
func (p Port) RunAction(ctx context.Context, run ActionRun) (ActionResult, error) {
	ctx, end, err := p.window.begin(ctx)
	if err != nil {
		return ActionResult{}, err
	}
	defer end()
	if run.Deadline.IsZero() {
		return ActionResult{}, errors.New("an action needs a deadline")
	}
	ctx, cancel := context.WithDeadline(ctx, run.Deadline)
	defer cancel()
	return p.window.backend.RunAction(ctx, p.service, run)
}

// CancelAction cancels an invocation this service started, by its identity.
func (p Port) CancelAction(ctx context.Context, workspace, id string) error {
	w := p.window
	if w == nil {
		return ErrClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return w.backend.CancelAction(ctx, p.service, workspace, id)
}
