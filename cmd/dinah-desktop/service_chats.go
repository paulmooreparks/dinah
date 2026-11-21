package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"studio/internal/agents"
	"studio/internal/applet"
	"studio/internal/applet/broker"
	"studio/internal/layout"
	"studio/internal/records"
	"studio/internal/servicechats"
	"studio/internal/sessionhost"
	"studio/internal/workspaces"
)

/* A product's own service works in chats through the plain port of
   internal/servicechats (docs/design/service-chats.md). This file is the
   window's side of that port. Each method uses the functions the App's own
   methods use: a chat is opened as New Chat opens one, a turn is the owned
   turn the task host takes (sessionhost.TurnIO.TakeTurn), a model is set
   through the chat's options service, and an action runs in the applet
   broker. The port has no review and no confirmation of a run, which the
   task host has (task_host.go). */

type serviceChatState struct {
	mu     sync.Mutex
	window *servicechats.Window
	// holds has the release of each chat kept loaded for a service.
	holds map[serviceChatKey]func()
	// file serializes the changes of the saved records.
	file sync.Mutex
}

type serviceChatKey struct{ service, chat string }

// serviceChatRecord is what the window keeps of a chat a service opened.
type serviceChatRecord struct {
	Workspace string    `json:"workspace"`
	Program   string    `json:"program"`
	Opened    time.Time `json:"opened"`
	Released  bool      `json:"released,omitempty"`
}

// serviceChatRecords is one service's saved record. ID is the service's
// name, which the record store requires of every record.
type serviceChatRecords struct {
	ID    string                       `json:"id"`
	Chats map[string]serviceChatRecord `json:"chats"`
}

type serviceChatBackend struct{ app *App }

// serviceTurnSettle is how long a turn that was asked to stop may take to
// end before the port gives it up.
const serviceTurnSettle = 2 * time.Second

func serviceChatStore() records.Store {
	return records.Store{Dir: filepath.Join(configDir(), "service-chats")}
}

func readServiceChats(service string) (serviceChatRecords, error) {
	out := serviceChatRecords{Chats: map[string]serviceChatRecord{}}
	data, err := serviceChatStore().Read(service)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	if out.Chats == nil {
		out.Chats = map[string]serviceChatRecord{}
	}
	return out, nil
}

// changeServiceChats saves a change of the chats a service opened.
func (a *App) changeServiceChats(service string, change func(*serviceChatRecords)) error {
	a.serviceChats.file.Lock()
	defer a.serviceChats.file.Unlock()
	saved, err := readServiceChats(service)
	if err != nil {
		return err
	}
	change(&saved)
	saved.ID = service
	data, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	return serviceChatStore().Write(service, data)
}

// closeServiceChats lets go of the chats the services kept loaded.
func (a *App) closeServiceChats() {
	a.serviceChats.mu.Lock()
	holds := a.serviceChats.holds
	a.serviceChats.holds = nil
	a.serviceChats.mu.Unlock()
	for _, release := range holds {
		release()
	}
}

// serviceChatWorkspace resolves the folder a service gave to the folder of
// exactly one saved workspace.
func (a *App) serviceChatWorkspace(folder string) (string, error) {
	if folder == "" || !filepath.IsAbs(folder) {
		return "", fmt.Errorf("%w: a workspace is given by its folder's full path", servicechats.ErrNotWorkspace)
	}
	a.mu.Lock()
	saved := workspaces.Copy(a.settings.WorkspaceRecords)
	a.mu.Unlock()
	var matches []string
	for _, record := range saved {
		if path := workspaces.ExecutionPath(record); path != "" && layout.SamePath(path, folder) {
			matches = append(matches, path)
		}
	}
	if len(matches) == 0 {
		return "", servicechats.ErrNotWorkspace
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("%w: more than one saved workspace has this folder", servicechats.ErrNotWorkspace)
	}
	if info, err := os.Stat(matches[0]); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: the workspace's folder is unavailable", servicechats.ErrNotWorkspace)
	}
	return matches[0], nil
}

// serviceChatCheck refuses a chat that is deleted or switching programs
// and, for a turn, one that is working. It is the check the task host makes
// of a chat (checkTaskChat), with the port's errors.
func (a *App) serviceChatCheck(id string, turn bool) (LogicalChatMembers, error) {
	a.mu.Lock()
	s := a.settings
	members, err := logicalChatMembersFromSettings(s, id)
	_, removed := s.RemovedChats[id]
	for _, member := range members.Members {
		if _, held := s.RemovedChats[member.SessionID]; held {
			removed = true
		}
	}
	head := members.Head
	if head == "" {
		head = id
	}
	chat := a.runtime.ConversationLocked(head)
	_, transferring := a.transferRunning.Load(head)
	a.mu.Unlock()
	if removed {
		return members, servicechats.ErrRemoved
	}
	if err != nil {
		return members, err
	}
	if transferring || chat != nil && chat.Switching() {
		return members, servicechats.ErrSwitching
	}
	if turn && chat != nil && chat.State().WorkActivity().Busy() {
		return members, servicechats.ErrBusy
	}
	return members, nil
}

// serviceChatOpened finds the record of a chat among those the service
// opened. A chat that was continued with another program is the same chat,
// so the record is found by any of its stages.
func (a *App) serviceChatOpened(service, id string, members LogicalChatMembers) (string, serviceChatRecord, error) {
	a.serviceChats.file.Lock()
	saved, err := readServiceChats(service)
	a.serviceChats.file.Unlock()
	if err != nil {
		return "", serviceChatRecord{}, err
	}
	ids := []string{id, members.ChatID, members.Head}
	for _, member := range members.Members {
		ids = append(ids, member.SessionID)
	}
	for _, candidate := range ids {
		if record, ok := saved.Chats[candidate]; ok && candidate != "" {
			if record.Released {
				return candidate, record, servicechats.ErrReleased
			}
			return candidate, record, nil
		}
	}
	return "", serviceChatRecord{}, servicechats.ErrNotOpened
}

// serviceChatFolder is the workspace folder of a chat the person has.
func (a *App) serviceChatFolder(head string) (string, error) {
	if chat := a.session(head); chat != nil {
		return chat.Cwd(), nil
	}
	saved, err := sessionhost.ReadTranscript(configDir(), head)
	if err != nil {
		return "", err
	}
	return saved.Workspace, nil
}

// holdServiceChat keeps a chat a service opened loaded until the service
// releases it or the window closes.
func (a *App) holdServiceChat(service, id string, chat *Session) error {
	key := serviceChatKey{service, id}
	a.serviceChats.mu.Lock()
	defer a.serviceChats.mu.Unlock()
	if a.serviceChats.holds[key] != nil {
		return nil
	}
	release, err := chat.HoldRuntime()
	if err != nil {
		return err
	}
	if a.serviceChats.holds == nil {
		a.serviceChats.holds = map[serviceChatKey]func(){}
	}
	a.serviceChats.holds[key] = release
	return nil
}

// trackServiceWork counts a call of a service among the unfinished work the
// person is told of when the window is closed.
func (a *App) trackServiceWork(service, kind string) (func(), error) {
	token, err := a.BeginExitWork(service + ": " + kind)
	if err != nil {
		return nil, err
	}
	return func() { a.EndExitWork(token) }, nil
}

func (b serviceChatBackend) OpenChat(ctx context.Context, service string, request servicechats.OpenRequest) (servicechats.Chat, error) {
	a := b.app
	path, err := a.serviceChatWorkspace(request.Workspace)
	if err != nil {
		return servicechats.Chat{}, err
	}
	if !product.Offers(request.Program) {
		return servicechats.Chat{}, servicechats.ErrProgram
	}
	done, err := a.trackServiceWork(service, "open a chat")
	if err != nil {
		return servicechats.Chat{}, err
	}
	defer done()
	if err := ctx.Err(); err != nil {
		return servicechats.Chat{}, err
	}
	chat := servicechats.Chat{Workspace: path, Program: request.Program}
	info, err := a.NewSessionWithProviderAt(path, request.Program)
	chat.ID = info.ID
	record := func() error {
		return a.changeServiceChats(service, func(saved *serviceChatRecords) {
			saved.Chats[info.ID] = serviceChatRecord{Workspace: path, Program: request.Program, Opened: time.Now().UTC()}
		})
	}
	if err != nil {
		var notCreated *sessionhost.AllocationNotCreatedError
		if errors.As(err, &notCreated) {
			return servicechats.Chat{}, err
		}
		// The agent may have made the chat. When it gave an identity, the
		// chat is recorded as the service's, so the service can still use it.
		if info.ID != "" {
			err = errors.Join(err, record())
		}
		return chat, fmt.Errorf("%w: %w", servicechats.ErrUnknownOutcome, err)
	}
	if err := record(); err != nil {
		return chat, fmt.Errorf("the chat was opened and could not be recorded as the service's: %w", err)
	}
	if opened := a.session(info.ID); opened != nil {
		if err := a.holdServiceChat(service, info.ID, opened); err != nil {
			return chat, fmt.Errorf("the chat was opened and closed again at once: %w", err)
		}
	}
	if request.Title != "" {
		titles, err := a.RenameSession(info.ID, request.Title)
		if err != nil {
			return chat, fmt.Errorf("the chat was opened and could not be given its title: %w", err)
		}
		chat.Title = titles[info.ID]
	}
	return chat, nil
}

func (b serviceChatBackend) TakeTurn(ctx context.Context, service, chat string, prompt servicechats.Prompt, opened bool) (servicechats.Reply, error) {
	a := b.app
	members, err := a.serviceChatCheck(chat, true)
	if err != nil {
		return servicechats.Reply{}, err
	}
	var folder string
	if opened {
		_, record, err := a.serviceChatOpened(service, chat, members)
		if err != nil {
			return servicechats.Reply{}, err
		}
		folder = record.Workspace
	} else if folder, err = a.serviceChatFolder(members.Head); err != nil {
		return servicechats.Reply{}, fmt.Errorf("%w: %w", servicechats.ErrNotSent, err)
	}
	done, err := a.trackServiceWork(service, "a turn")
	if err != nil {
		return servicechats.Reply{}, fmt.Errorf("%w: %w", servicechats.ErrNotSent, err)
	}
	defer done()
	info, err := a.OpenSessionAt(folder, chat)
	if err != nil {
		return servicechats.Reply{}, fmt.Errorf("%w: the chat could not be opened: %w", servicechats.ErrNotSent, err)
	}
	s := a.session(info.ID)
	if s == nil {
		return servicechats.Reply{}, fmt.Errorf("%w: the opened chat is no longer available", servicechats.ErrNotSent)
	}
	if opened {
		err = a.holdServiceChat(service, info.ID, s)
	} else {
		var release func()
		if release, err = s.HoldRuntime(); err == nil {
			defer release()
		}
	}
	if err != nil {
		return servicechats.Reply{}, fmt.Errorf("%w: %w", servicechats.ErrNotSent, err)
	}
	label := prompt.Label
	if label == "" {
		label = service
	}
	// The flag is set only after the last check before the prompt is handed
	// to the agent's connection, so a failure without it sent nothing.
	var handed atomic.Bool
	// The turn runs under a context of its own. When the caller's context
	// ends, a prompt that was handed over is stopped through the agent, and
	// the turn's own end is waited for, so that a turn the agent ends is
	// reported as cancelled. The turn is given up only when the agent does
	// not end it in time, and its outcome is then unknown.
	turnCtx, abandon := context.WithCancel(context.Background())
	defer abandon()
	stopTurn := make(chan struct{})
	type outcome struct {
		words string
		end   sessionhost.End
	}
	finished := make(chan outcome, 1)
	go func() {
		words, end := a.turnIO().TakeTurnStoppable(turnCtx, stopTurn, s, prompt.Text, label, func() error {
			if _, err := a.serviceChatCheck(chat, false); err != nil {
				return err
			}
			handed.Store(true)
			return nil
		})
		finished <- outcome{words, end}
	}()
	var out outcome
	select {
	case out = <-finished:
	case <-ctx.Done():
		if handed.Load() {
			close(stopTurn)
		} else {
			abandon()
		}
		settle := time.NewTimer(serviceTurnSettle)
		select {
		case out = <-finished:
		case <-settle.C:
			abandon()
			out = <-finished
		}
		settle.Stop()
	}
	words, end := out.words, out.end
	// A turn given up reports the end of its own context. The caller is told
	// how its context ended.
	if end.Err != nil && ctx.Err() != nil && errors.Is(end.Err, context.Canceled) {
		end.Err = ctx.Err()
	}
	if end.Err != nil {
		switch {
		case errors.Is(end.Err, sessionhost.ErrOwnedTurnBusy):
			return servicechats.Reply{}, servicechats.ErrBusy
		case handed.Load():
			return servicechats.Reply{Text: words}, fmt.Errorf("%w: %w", servicechats.ErrUnknownOutcome, end.Err)
		case errors.Is(end.Err, servicechats.ErrRemoved), errors.Is(end.Err, servicechats.ErrSwitching):
			return servicechats.Reply{}, end.Err
		}
		return servicechats.Reply{}, fmt.Errorf("%w: %w", servicechats.ErrNotSent, end.Err)
	}
	return servicechats.Reply{Text: words, StopReason: end.StopReason}, nil
}

// serviceChatSession loads a chat the service opened and returns it.
func (a *App) serviceChatSession(service, chat string) (*Session, error) {
	members, err := a.serviceChatCheck(chat, false)
	if err != nil {
		return nil, err
	}
	_, record, err := a.serviceChatOpened(service, chat, members)
	if err != nil {
		return nil, err
	}
	info, err := a.OpenSessionAt(record.Workspace, chat)
	if err != nil {
		return nil, err
	}
	s := a.session(info.ID)
	if s == nil {
		return nil, errors.New("the opened chat is no longer available")
	}
	if err := a.holdServiceChat(service, info.ID, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (b serviceChatBackend) Model(ctx context.Context, service, chat string) (servicechats.Model, error) {
	s, err := b.app.serviceChatSession(service, chat)
	if err != nil {
		return servicechats.Model{}, err
	}
	option, ok := agents.ModelOption(s.Options())
	if !ok {
		return servicechats.Model{}, servicechats.ErrModel
	}
	model := servicechats.Model{Offered: []servicechats.ModelChoice{}}
	_ = json.Unmarshal(option.CurrentValue, &model.Value)
	model.Name = agents.ValueName(option.Options, model.Value)
	for _, value := range agents.OptionValues(option) {
		model.Offered = append(model.Offered, servicechats.ModelChoice{Value: value, Name: agents.ValueName(option.Options, value)})
	}
	return model, nil
}

func (b serviceChatBackend) UseModel(ctx context.Context, service, chat, model string) (func(context.Context) error, error) {
	a := b.app
	s, err := a.serviceChatSession(service, chat)
	if err != nil {
		return nil, err
	}
	option, ok := agents.ModelOption(s.Options())
	value, _ := json.Marshal(model)
	if !ok || !agents.Allowed(option, value) {
		return nil, servicechats.ErrModel
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// A recording setter is the exclusive one: it refuses a chat that is
	// working, and its change can be put back only while nobody else has
	// changed the chat's settings. The port keeps the change in memory.
	keep := func(sessionhost.OptionChange) error { return nil }
	_, change, err := a.optionService().SetRecorded(s.ID(), option.ID, value, keep)
	if err != nil {
		switch {
		case errors.Is(err, sessionhost.ErrOptionBusy):
			return nil, servicechats.ErrBusy
		case change.Revision == "" || errors.Is(err, sessionhost.ErrOptionsChanged):
			return nil, err
		}
		return nil, fmt.Errorf("%w: %w", servicechats.ErrUnknownOutcome, err)
	}
	var once sync.Once
	return func(context.Context) error {
		var err error
		once.Do(func() {
			_, _, err = a.optionService().RestoreRecorded(change, keep)
			switch {
			case errors.Is(err, sessionhost.ErrOptionsChanged):
				err = servicechats.ErrModelChanged
			case errors.Is(err, sessionhost.ErrOptionBusy):
				err = servicechats.ErrBusy
			}
		})
		return err
	}, nil
}

func (b serviceChatBackend) RenameChat(ctx context.Context, service, chat, title string) error {
	a := b.app
	members, err := a.serviceChatCheck(chat, false)
	if err != nil {
		return err
	}
	if _, _, err := a.serviceChatOpened(service, chat, members); err != nil {
		return err
	}
	head := members.Head
	if head == "" {
		head = chat
	}
	_, err = a.RenameSession(head, title)
	return err
}

func (b serviceChatBackend) ReleaseChat(ctx context.Context, service, chat string) error {
	a := b.app
	a.mu.Lock()
	members, _ := logicalChatMembersFromSettings(a.settings, chat)
	a.mu.Unlock()
	key, _, err := a.serviceChatOpened(service, chat, members)
	if errors.Is(err, servicechats.ErrReleased) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := a.changeServiceChats(service, func(saved *serviceChatRecords) {
		record := saved.Chats[key]
		record.Released = true
		saved.Chats[key] = record
	}); err != nil {
		return err
	}
	ids := []string{chat, key, members.Head}
	a.serviceChats.mu.Lock()
	var releases []func()
	for _, id := range ids {
		held := serviceChatKey{service, id}
		if release := a.serviceChats.holds[held]; release != nil {
			releases = append(releases, release)
			delete(a.serviceChats.holds, held)
		}
	}
	a.serviceChats.mu.Unlock()
	for _, release := range releases {
		release()
	}
	return nil
}

func (b serviceChatBackend) SkillAvailable(ctx context.Context, workspace, program, skill string) (bool, error) {
	a := b.app
	path, err := a.serviceChatWorkspace(workspace)
	if err != nil {
		return false, err
	}
	if !product.Offers(program) {
		return false, servicechats.ErrProgram
	}
	if skill == "" {
		return false, nil
	}
	for _, offered := range a.SkillsForProviderAt(path, program) {
		if offered.Name == skill {
			return true, nil
		}
	}
	// A skill the window supplies as text, such as a built-in one, is not in
	// that list. It is available when the preparation finds its instructions.
	command := "/" + skill
	prepared, err := a.skillPromptFor(program, path, command)
	return err == nil && prepared != command, err
}

func (b serviceChatBackend) SkillPrompt(ctx context.Context, workspace, program, text string) (string, error) {
	a := b.app
	path, err := a.serviceChatWorkspace(workspace)
	if err != nil {
		return "", err
	}
	if !product.Offers(program) {
		return "", servicechats.ErrProgram
	}
	return a.skillPromptFor(program, path, text)
}

func (b serviceChatBackend) Programs(context.Context) ([]servicechats.Program, error) {
	out := make([]servicechats.Program, 0, len(product.Providers))
	for _, id := range product.Providers {
		out = append(out, servicechats.Program{ID: id, Name: providerName(id)})
	}
	return out, nil
}

func (b serviceChatBackend) CheckProgram(ctx context.Context, program string) (servicechats.ProgramState, error) {
	state := servicechats.ProgramState{Program: program, Offered: program != "" && product.Offers(program)}
	if !state.Offered {
		state.Status, state.Reason = "unsupported", "This program is not offered by this product."
		return state, nil
	}
	if err := ctx.Err(); err != nil {
		return state, err
	}
	info := b.app.AgentCheck(program)
	state.Installed, state.Status = info.Installed, info.Status
	state.Reachable = info.Available && info.Status == "connected"
	if !state.Reachable {
		state.Reason = info.StatusWords
		if state.Reason == "" {
			state.Reason = info.Detail
		}
	}
	return state, nil
}

func (b serviceChatBackend) PublishChange(ctx context.Context, change servicechats.Change) error {
	if change.Workspace == "" || change.Key == "" || len(change.Workspace) > 1024 || len(change.Key) > 256 {
		return errors.New("a change notice needs a workspace and a key")
	}
	b.app.emit("service:"+change.Service+":changed", change)
	return nil
}

func (b serviceChatBackend) DescribeAction(ctx context.Context, reference string) (servicechats.Action, error) {
	registry, err := b.app.appletRegistry()
	if err != nil {
		return servicechats.Action{}, err
	}
	action, err := registry.Describe(reference)
	if err != nil {
		return servicechats.Action{}, err
	}
	digest, err := registry.Digest(reference)
	if err != nil {
		return servicechats.Action{}, err
	}
	return servicechats.Action{Reference: reference, Title: action.Title, Description: action.Description, InputSchema: action.InputSchema, OutputSchema: action.OutputSchema,
		Replay: action.Replay, Effect: action.Effect, Capabilities: action.Capabilities, Digest: digest, NeedsPerson: applet.RequiresFreshApproval(action)}, nil
}

func (b serviceChatBackend) RunAction(ctx context.Context, service string, run servicechats.ActionRun) (servicechats.ActionResult, error) {
	a := b.app
	path, err := a.serviceChatWorkspace(run.Workspace)
	if err != nil {
		return servicechats.ActionResult{}, err
	}
	host, err := a.appletHostService().Broker()
	if err != nil {
		return servicechats.ActionResult{}, err
	}
	action, err := host.Registry.Describe(run.Reference)
	if err != nil {
		return servicechats.ActionResult{}, err
	}
	if applet.RequiresFreshApproval(action) {
		return servicechats.ActionResult{}, servicechats.ErrNeedsPerson
	}
	id := run.ID
	if id == "" {
		if id, err = broker.NewID(); err != nil {
			return servicechats.ActionResult{}, err
		}
	}
	done, err := a.trackServiceWork(service, "an action")
	if err != nil {
		return servicechats.ActionResult{}, err
	}
	defer done()
	result := servicechats.ActionResult{ID: id}
	// The broker cancels the invocation when the context ends.
	out, err := host.Run(ctx, path, "service", service, id, run.Reference, string(run.Input), run.Digest, run.Deadline)
	if err == nil {
		result.Output = json.RawMessage(out)
		return result, nil
	}
	saved, readErr := host.Read(path, id)
	switch {
	case errors.Is(readErr, os.ErrNotExist):
		// Nothing was recorded, so the action did not start.
		return result, err
	case readErr == nil && saved.Status == "done":
		result.Output = json.RawMessage(saved.OutputJSON)
		return result, nil
	case readErr == nil && saved.Status == "failed" && action.Replay != "once" && !errors.Is(err, broker.ErrRecovery):
		return result, err
	}
	return result, fmt.Errorf("%w: %w", servicechats.ErrUnknownOutcome, err)
}

func (b serviceChatBackend) CancelAction(ctx context.Context, service, workspace, id string) error {
	path, err := b.app.serviceChatWorkspace(workspace)
	if err != nil {
		return err
	}
	return b.app.appletHostService().CancelKnown(path, "service", service, id)
}
