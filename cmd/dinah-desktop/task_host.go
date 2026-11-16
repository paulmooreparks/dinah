package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"studio/internal/agents"
	"studio/internal/layout"
	"studio/internal/services"
	"studio/internal/sessionhost"
	"studio/internal/taskhost"
	"studio/internal/workspaces"
)

type taskServiceState struct {
	once      sync.Once
	mu        sync.Mutex
	host      *taskhost.Window
	window    *services.Window
	err       error
	turns     map[string]taskOwnedTurn
	now       func() time.Time
	rooms     taskhost.RoomAdapter
	interests map[string]func()
}

type taskOwnedTurn struct {
	cancel context.CancelFunc
	done   <-chan struct{}
}

func (a *App) taskRooms() taskhost.RoomAdapter {
	a.taskServices.mu.Lock()
	defer a.taskServices.mu.Unlock()
	return a.taskServices.rooms
}

func taskRoomKind(kind string) bool {
	return slices.Contains([]string{"EnsureRoom", "ClaimRoom", "PostRoom", "ObserveRoom", "ReleaseRoom", "RecoverRoom"}, kind)
}

func taskRefusal(code, reason string) error { return &taskhost.Error{Code: code, Reason: reason} }

func (a *App) startTaskServices() error {
	if !services.HasWindowServices() {
		return nil
	}
	a.taskServices.once.Do(func() {
		h, err := taskhost.New(taskhost.Options{
			State: configDir(), Resolve: a.taskWorkspace, Check: a.checkTaskIdentity,
			HoldTask: a.holdTaskRecord, Admit: a.admitTaskOperation, Confirm: a.confirmTaskReview, TargetRevision: a.taskTargetRevision,
			Resource:   a.taskResource,
			Operations: a.taskHostOperations, DescribeChats: a.describeTaskChats, CheckProgram: a.checkTaskProgram, DescribeAction: a.describeTaskAction,
			PendingQuestions: a.pendingServiceQuestions,
			SavedChange:      func(change taskhost.Change) { a.emit("service:"+change.Service+":changed", change) },
			Backend:          taskBackend{a}, RoomsAvailable: func() bool { return a.taskRooms() != nil },
			Now: func() time.Time {
				a.taskServices.mu.Lock()
				now := a.taskServices.now
				a.taskServices.mu.Unlock()
				if now != nil {
					return now()
				}
				return time.Now()
			},
			Track: func(s taskhost.Scope, o taskhost.Operation) (func(), error) {
				token, err := a.BeginExitWork(s.Service + ": " + o.Kind)
				if err != nil {
					return nil, err
				}
				return func() { a.EndExitWork(token) }, nil
			},
			Emit:      func(r taskhost.Receipt) { a.emit("task:receipt", r) },
			Lifecycle: func(phase string) { a.emit("task:host", map[string]string{"phase": phase}) },
		})
		var window *services.Window
		if err == nil {
			window, err = services.NewWindow(h)
		}
		a.taskServices.mu.Lock()
		a.taskServices.host = h
		a.taskServices.window = window
		a.taskServices.err = err
		a.taskServices.mu.Unlock()
	})
	a.taskServices.mu.Lock()
	defer a.taskServices.mu.Unlock()
	return a.taskServices.err
}

func (a *App) closeTaskServices() {
	a.taskServices.mu.Lock()
	h, w := a.taskServices.host, a.taskServices.window
	a.taskServices.mu.Unlock()
	if h == nil {
		return
	}
	h.CloseAdmission()
	a.closeTaskQuestions()
	// Cleanup has its own bounded context after execution and page calls stop.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.Shutdown(ctx); err != nil {
		logf("core", "error", "task services did not finish checkpointing: %v", err)
	}
	if err := h.Shutdown(ctx); err != nil {
		logf("core", "error", "task receipt cleanup did not settle: %v", err)
	}
	a.taskServices.mu.Lock()
	interests := make([]func(), 0, len(a.taskServices.interests))
	for _, release := range a.taskServices.interests {
		interests = append(interests, release)
	}
	a.taskServices.mu.Unlock()
	for _, release := range interests {
		release()
	}
}

func (a *App) taskRecordPort(service string) (services.TaskRecords, error) {
	a.taskServices.mu.Lock()
	w := a.taskServices.window
	a.taskServices.mu.Unlock()
	p, ok := w.Records(service)
	if !ok {
		return nil, taskRefusal("unavailable", "this service provides no durable task store")
	}
	return p, nil
}

func (a *App) checkTaskIdentity(ctx context.Context, s taskhost.Scope) error {
	p, err := a.taskRecordPort(s.Service)
	if err != nil {
		return err
	}
	g, err := p.TaskGeneration(ctx, s.Workspace, s.Task)
	if err != nil {
		return err
	}
	if g != s.Generation {
		return taskRefusal("stale", "the task record generation changed")
	}
	if s.OwnerChat != "" {
		_, err := a.taskTargetRevision(ctx, s, taskhost.Permit{Kind: "ReadOptions", Target: s.OwnerChat})
		return err
	}
	return nil
}

func (a *App) holdTaskRecord(ctx context.Context, s taskhost.Scope) (func() error, error) {
	p, err := a.taskRecordPort(s.Service)
	if err != nil {
		return nil, err
	}
	return p.HoldTask(ctx, s.Workspace, s.Task)
}

func (a *App) taskWorkspace(ctx context.Context, s taskhost.Scope) (taskhost.Binding, error) {
	if err := ctx.Err(); err != nil {
		return taskhost.Binding{}, err
	}
	a.mu.Lock()
	records := workspaces.Copy(a.settings.WorkspaceRecords)
	a.mu.Unlock()
	var matches []WorkspaceRecord
	for _, r := range records {
		if r.ID == s.Workspace {
			matches = append(matches, r)
		}
	}
	if len(matches) != 1 {
		return taskhost.Binding{}, taskRefusal("workspace_changed", "the saved workspace identity is missing or ambiguous")
	}
	r := matches[0]
	path := workspaces.ExecutionPath(r)
	resolved, err := agents.ResolvePath(path)
	if err != nil {
		return taskhost.Binding{}, taskRefusal("workspace_changed", "the workspace folder is inaccessible")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return taskhost.Binding{}, taskRefusal("workspace_changed", "the workspace folder is unavailable")
	}
	root, err := os.OpenRoot(resolved)
	if err != nil {
		return taskhost.Binding{}, taskRefusal("workspace_changed", "the workspace folder cannot be opened")
	}
	_ = root.Close()
	b, _ := json.Marshal(struct {
		Record   WorkspaceRecord
		Resolved string
	}{r, resolved})
	sum := sha256.Sum256(b)
	return taskhost.Binding{Path: resolved, Generation: hex.EncodeToString(sum[:])}, nil
}

func (a *App) checkTaskChat(id string, turn bool) error {
	a.mu.Lock()
	s := a.settings
	members, err := logicalChatMembersFromSettings(s, id)
	if err == nil {
		for _, member := range members.Members {
			if _, held := s.RemovedChats[member.SessionID]; held {
				err = taskRefusal("removed", "the chat has a removal reservation")
				break
			}
		}
	}
	chat := a.runtime.ConversationLocked(members.Head)
	_, transferring := a.transferRunning.Load(members.Head)
	a.mu.Unlock()
	if err != nil {
		return err
	}
	if transferring || chat != nil && chat.Switching() {
		return taskRefusal("busy", "the chat is switching programs")
	}
	if turn && chat != nil && chat.State().WorkActivity().Busy() {
		return taskRefusal("busy", "the chat is already working")
	}
	return nil
}

func (a *App) taskResource(ctx context.Context, scope taskhost.Scope, operation taskhost.Operation) (string, error) {
	if operation.Kind != "TakeTurn" && operation.Kind != "UseModel" {
		return "", nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	a.mu.Lock()
	members, err := logicalChatMembersFromSettings(a.settings, operation.Target)
	a.mu.Unlock()
	if err != nil {
		return "", err
	}
	return members.ChatID, nil
}

func (a *App) admitTaskOperation(ctx context.Context, s taskhost.Scope, o taskhost.Operation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if taskRoomKind(o.Kind) {
		rooms := a.taskRooms()
		if rooms == nil {
			return taskRefusal("unsupported", "the complete room protocol is unavailable")
		}
		_, err := rooms.Review(ctx, s, taskhost.Permit{Kind: o.Kind, Target: o.Target, Program: o.Program, Digest: o.Digest, Input: o.Input})
		return err
	}
	if o.Program != "" && !product.Offers(o.Program) {
		return taskRefusal("unsupported", "the configured host does not offer this program")
	}
	if o.Kind == "OpenChat" && o.Target == "" {
		if o.Program == "" {
			return taskRefusal("unsupported", "fresh allocation requires a selected program")
		}
		return nil
	}
	if !slices.Contains([]string{"OpenChat", "TakeTurn", "ReadOptions", "UseModel", "CancelTurn", "ReleaseChat", "InvokeAction", "CancelAction", "AskPerson"}, o.Kind) {
		return taskRefusal("unsupported", "this operation's host adapter is unavailable")
	}
	switch o.Kind {
	case "OpenChat", "TakeTurn", "ReadOptions", "UseModel", "ReleaseChat", "CancelTurn":
		if err := a.checkTaskChat(o.Target, o.Kind == "TakeTurn" || o.Kind == "UseModel"); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) taskTargetRevision(ctx context.Context, scope taskhost.Scope, p taskhost.Permit) (string, error) {
	if taskRoomKind(p.Kind) {
		rooms := a.taskRooms()
		if rooms == nil {
			return "", taskRefusal("unsupported", "the complete room protocol is unavailable")
		}
		return rooms.Review(ctx, scope, p)
	}
	if p.Kind == "AskPerson" {
		if _, err := decodeTypedTaskQuestion(p.Input); err != nil {
			return "", err
		}
		return p.Target, nil
	}
	if p.Kind == "InvokeAction" {
		r, err := a.appletRegistry()
		if err != nil {
			return "", err
		}
		digest, err := r.Digest(p.Target)
		if err != nil {
			return "", err
		}
		if digest != p.Digest {
			return "", taskRefusal("stale", "the reviewed action digest changed")
		}
		return digest, nil
	}
	if p.Kind == "CancelAction" {
		return p.Target, nil
	}
	if p.Target == "" {
		if p.Kind != "OpenChat" || p.Program == "" || !product.Offers(p.Program) {
			return "", taskRefusal("unsupported", "fresh allocation requires an offered program")
		}
		return "fresh:" + p.Program, nil
	}
	if err := a.checkTaskChat(p.Target, false); err != nil {
		return "", err
	}
	a.mu.Lock()
	members, err := logicalChatMembersFromSettings(a.settings, p.Target)
	chat := a.runtime.ConversationLocked(members.Head)
	a.mu.Unlock()
	if err != nil {
		return "", err
	}
	var path, program string
	if chat != nil {
		path = chat.Cwd()
		program = chat.Provider()
	} else {
		tr, err := sessionhost.ReadTranscript(configDir(), members.Head)
		if err != nil {
			return "", taskRefusal("ambiguous", err.Error())
		}
		path = tr.Workspace
		program = tr.Provider
	}
	ws, err := a.taskWorkspace(ctx, scope)
	if err != nil {
		return "", err
	}
	if !layout.SamePath(path, ws.Path) {
		return "", taskRefusal("not_authorized", "the chat belongs to another workspace")
	}
	if p.Program != "" && p.Program != program {
		return "", taskRefusal("stale", "the chat's selected program changed")
	}
	if !product.Offers(program) {
		return "", taskRefusal("unsupported", "the chat's program is unavailable in this host")
	}
	b, _ := json.Marshal([]string{members.ChatID, members.Head, members.Generation, path, program})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func taskQuestionOwner(s taskhost.Scope) string {
	b, _ := json.Marshal([]string{s.Service, s.Workspace, s.Task})
	sum := sha256.Sum256(b)
	return "task-" + hex.EncodeToString(sum[:16])
}

func (a *App) confirmTaskReview(ctx context.Context, r taskhost.Review) error {
	// This question is answered through the existing person-only permission path.
	// No request body field can confirm a review or impersonate an invocation grant.
	questionCtx, cancel := context.WithDeadline(ctx, r.Expires)
	defer cancel()
	var summary strings.Builder
	fmt.Fprintf(&summary, "Run the operations reviewed on this task's page before %s?", r.Plan.Deadline.Format(time.RFC3339))
	for _, permit := range r.Plan.Permits {
		fingerprint := sha256.Sum256(permit.Input)
		fmt.Fprintf(&summary, "\n%s; target %q; program %q; action digest %q; input fingerprint %x.", permit.Kind, permit.Target, permit.Program, permit.Digest, fingerprint)
	}
	choice, err := a.askTaskQuestion(questionCtx, r.Plan.Scope, summary.String(), []taskhost.QuestionOption{{OptionID: "allow_once", Name: "Run the Reviewed Task", Kind: "allow_once"}, {OptionID: "reject_once", Name: "Don't Run", Kind: "reject_once"}})
	if err != nil || choice != "allow_once" {
		return taskRefusal("not_authorized", "the reviewed task was not confirmed by the person")
	}
	return nil
}

// There is no native work here until an explicit reviewed operation dispatches.
type taskBackend struct{ app *App }

func taskFailure(err error) taskhost.Outcome {
	var typed *taskhost.Error
	if errors.As(err, &typed) {
		return taskhost.Outcome{State: "failed", Code: typed.Code, Reason: typed.Reason}
	}
	return taskhost.Outcome{State: "failed", Code: "unavailable", Reason: err.Error()}
}
