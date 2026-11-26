//go:build quality_task_host

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/applet"
	appletHost "studio/internal/applet/host"
	"studio/internal/records"
	"studio/internal/services"
	"studio/internal/taskhost"
)

const frozenService = "frozen-task-consumer"
const frozenAction = "fixture.task/effect@1"

var frozenRegistration sync.Once
var frozenConsumer *frozenTaskStore

func init() { Factory = newFrozenTaskHost }

type frozenTaskStore struct {
	path   string
	client taskhost.Client
}

func (s *frozenTaskStore) TaskGeneration(context.Context, string, string) (string, error) {
	data, err := os.ReadFile(filepath.Join(s.path, "generation"))
	return string(data), err
}
func (s *frozenTaskStore) HoldTask(context.Context, string, string) (func() error, error) {
	return (records.Store{Dir: s.path}).Hold("consumer")
}
func (s *frozenTaskStore) Shutdown(context.Context) error { return nil }
func (s *frozenTaskStore) DescribeTask(_ context.Context, workspace, task string) (services.TaskPresentation, error) {
	return services.TaskPresentation{Workspace: workspace, Task: task, Title: "Invented task", Unavailable: "No fixture page."}, nil
}

type frozenPage struct{}

func (frozenPage) Call(context.Context, string, json.RawMessage) (any, error) {
	return nil, errors.New("fixture has no page calls")
}

type frozenTaskHost struct {
	t              *testing.T
	a              *App
	store          *frozenTaskStore
	peer           *frozenNativePeer
	mu             sync.Mutex
	counts         Counts
	events         []string
	clock          time.Time
	handles        map[string]*taskhost.Handle
	reviews        map[string]taskhost.Review
	scopes         map[string]taskhost.Scope
	sequence       int
	lost           atomic.Bool
	block          atomic.Bool
	consent        atomic.Bool
	question       chan struct{}
	terminal       chan taskhost.Receipt
	workspace      string
	protected      map[string]string
	docPeer        *frozenDocumentPeer
	remoteDraft    string
	roomPeer       *frozenRoomPeer
	observePending bool
	lastBinned     string
	joined         bool
	digest         string
}

func newFrozenTaskHost(t *testing.T) Host {
	f := &frozenTaskHost{t: t, clock: time.Now(), handles: map[string]*taskhost.Handle{}, reviews: map[string]taskhost.Review{}, scopes: map[string]taskhost.Scope{}, question: make(chan struct{}, 8), terminal: make(chan taskhost.Receipt, 32), workspace: t.TempDir()}
	f.store = &frozenTaskStore{path: t.TempDir()}
	// Any unexpected provider preparation or executable discovery is counted
	// and refused before it can touch an account or launch a real program.
	originalModules := make(map[string]providerModule, len(providerModules))
	for id, module := range providerModules {
		originalModules[id] = module
		module.Info = func(_ *App, info *AgentProviderInfo, _ []string) {
			f.mu.Lock()
			f.counts.Starts++
			f.mu.Unlock()
			info.Status = "unavailable"
		}
		if module.Adapter != nil {
			adapter := *module.Adapter
			adapter.prepare = func(*App) (providerStart, error) {
				f.mu.Lock()
				f.counts.Starts++
				f.mu.Unlock()
				return providerStart{}, errors.New("invented provider preparation refuses external launch")
			}
			module.Adapter = &adapter
		}
		providerModules[id] = module
	}
	t.Cleanup(func() {
		for id, module := range originalModules {
			providerModules[id] = module
		}
	})
	if err := os.WriteFile(filepath.Join(f.store.path, "generation"), []byte("generation-1"), 0600); err != nil {
		t.Fatal(err)
	}
	frozenRegistration.Do(func() {
		services.RegisterWindowNamed(frozenService, "Invented Tasks", nil, func(client taskhost.Client) (services.Service, services.Participant, error) {
			frozenConsumer.client = client
			return frozenPage{}, frozenConsumer, nil
		})
	})
	frozenConsumer = f.store
	a, c := providerTestApp(t)
	f.a = a
	f.configure(c)
	if err := a.updateSettings(func(s *Settings) error {
		s.Workspace = f.workspace
		record := newWorkspaceRecord(f.workspace)
		record.ID, record.Name = "workspace-a", "Invented workspace"
		s.WorkspaceRecords = []WorkspaceRecord{record}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.seedProtectedDocuments()
	f.observePending = strings.Contains(t.Name(), "TaskHostCancellationAndShutdown/") || strings.Contains(t.Name(), "TaskHostBinCancelsPendingWork") || strings.Contains(t.Name(), "TaskHostRepeatedPendingOperationReturnsReceipt") || strings.Contains(t.Name(), "TaskHostPendingObservationBinding/")
	if f.observePending {
		f.enableRoomPeer()
	}
	c.newIDs = []string{"codex:chat-a", "codex:chat-b", "codex:owner-chat"}
	for range 3 {
		if _, err := a.NewSessionAt(f.workspace); err != nil {
			t.Fatal(err)
		}
	}
	c.newIDs = []string{"codex:new-task"}
	f.mu.Lock()
	f.counts = Counts{}
	f.mu.Unlock()
	if err := a.startTaskServices(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.consentCleanup() })
	return f
}

func (f *frozenTaskHost) consentCleanup() {
	f.a.shutdown(context.Background())
}

func (f *frozenTaskHost) configure(c *testProviderConnection) {
	f.peer = &frozenNativePeer{testProviderConnection: c, host: f, cancel: make(chan struct{})}
	prior := f.a.runtime.ProviderLocked("codex")
	p := newTestProvider(f.peer, prior.Initialization())
	f.a.runtime.PutProviderLocked("codex", p)
	go f.a.lifecycle().Watch("codex", p)
	f.a.taskServices.now = func() time.Time { f.mu.Lock(); defer f.mu.Unlock(); return f.clock }
	f.a.sink = func(name string, value any) {
		switch name {
		case "task:permission":
			q := value.(taskhost.Question)
			if f.consent.Load() {
				if err := f.a.Answer(q.Key, "allow_once"); err != nil {
					f.t.Error(err)
				}
			}
		case "agent:permission":
			f.question <- struct{}{}
		case "task:receipt":
			r := value.(taskhost.Receipt)
			if r.State != "prepared" && r.State != "in_progress" {
				f.terminal <- r
			}
		case "task:host":
			f.event(value.(map[string]string)["phase"])
		}
	}
	bundle := applet.Bundle{Manifest: applet.Manifest{ID: "fixture.task", Version: "1.0.0", Title: "Invented peer", APIVersion: 1, Actions: []applet.Action{{Ref: frozenAction, Title: "Invented effect", Description: "Consumes invented input", Replay: "once", Effect: "read", Capabilities: []string{}, InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"],"additionalProperties":false}`), OutputSchema: json.RawMessage(`{"type":"object"}`)}}}, Handlers: map[string]applet.Handler{frozenAction: func(context.Context, string) (applet.Result, error) {
		f.mu.Lock()
		f.counts.Actions++
		f.mu.Unlock()
		if f.lost.Load() {
			return applet.Result{}, errors.New("invented peer consumed request and lost response")
		}
		return applet.Result{OutputJSON: `{}`}, nil
	}}, Sources: map[string][]byte{"peer.go": []byte("invented action peer version one")}}
	f.a.appletHost = appletHost.New(appletHost.Options{State: configDir(), Builtins: func() ([]applet.Bundle, error) { return []applet.Bundle{bundle}, nil }, Extensions: func() ([]applet.Bundle, error) { return nil, nil }, Revision: func() string { return "fixture" }, Emit: func(string, any) {}, Log: func(string, string, ...any) {}})
	b, err := f.a.appletHost.Broker()
	if err != nil {
		f.t.Fatal(err)
	}
	f.digest, err = b.Registry.Digest(frozenAction)
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *frozenTaskHost) event(event string) {
	f.mu.Lock()
	f.events = append(f.events, event)
	f.mu.Unlock()
}
func (f *frozenTaskHost) scope(r Request) taskhost.Scope {
	s := taskhost.Scope{Service: frozenService, Workspace: "workspace-a", Task: r.Task, Generation: r.Generation, OwnerChat: "codex:owner-chat"}
	if strings.HasSuffix(f.t.Name(), "/bin-referenced-chat") {
		s.OwnerChat = ""
	}
	return s
}
func (f *frozenTaskHost) operation(r Request) taskhost.Operation {
	target, digest := r.Target, r.Digest
	if taskRoomKind(r.Kind) {
		target = "invented-room"
	}
	if r.Kind == "OpenChat" && target == "chat-a" {
		target = ""
	}
	if r.Kind == "InvokeAction" {
		target = frozenAction
		if digest == "digest-1" {
			digest = f.digest
		}
	}
	input, _ := json.Marshal(map[string]string{"text": r.Input, "label": "Invented task"})
	if r.Kind == "InvokeAction" {
		input, _ = json.Marshal(map[string]string{"text": r.Input})
	}
	if r.Kind == "OpenChat" {
		input = nil
	}
	if target == "chat-a" || target == "chat-b" {
		target = "codex:" + target
	}
	program := ""
	if r.Kind == "OpenChat" {
		program = "codex"
	}
	return taskhost.Operation{ID: r.Operation, Kind: r.Kind, Target: target, Program: program, Digest: digest, Input: input}
}
func frozenResult(r taskhost.Receipt, err error) Result {
	if err != nil {
		code := "refused"
		var typed *taskhost.Error
		if errors.As(err, &typed) {
			code = typed.Code
		}
		return Result{Code: code}
	}
	return Result{Receipt: r.ID, State: r.State, Identity: r.Identity, Code: r.Code}
}
func (f *frozenTaskHost) Review(r Request) Result {
	op := f.operation(r)
	scope := f.scope(r)
	f.mu.Lock()
	deadline := f.clock.Add(time.Hour)
	f.mu.Unlock()
	permits := []taskhost.Permit{{Kind: op.Kind, Target: op.Target, Program: op.Program, Digest: op.Digest, Input: op.Input}}
	if f.observePending {
		observer := f.observation(r)
		permits = append(permits, taskhost.Permit{Kind: observer.Kind, Target: observer.Target, Input: observer.Input})
	}
	review, err := f.store.client.Review(context.Background(), taskhost.Plan{Scope: scope, Permits: permits, Deadline: deadline, Rooms: r.Rooms || f.observePending})
	if err != nil {
		return frozenResult(taskhost.Receipt{}, err)
	}
	f.reviews[review.ID] = review
	f.scopes[r.Task] = scope
	return Result{Receipt: review.ID}
}
func (f *frozenTaskHost) Begin(review Result, caller string) Result {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if caller == "foreign-invocation" {
		ctx = applet.WithInvocation(ctx, applet.Invocation{Workspace: f.t.TempDir(), OwnerKind: "chat", OwnerID: "foreign-chat", ID: "foreign-invocation"})
	}
	if caller == "forged-person" {
		ctx = context.WithValue(ctx, frozenCallerKey{}, map[string]any{"caller": "person", "confirmed": true})
	}
	f.consent.Store(caller == "trusted-person")
	defer f.consent.Store(false)
	h, err := f.store.client.Begin(ctx, review.Receipt)
	if err != nil {
		return frozenResult(taskhost.Receipt{}, err)
	}
	f.sequence++
	key := string(rune(f.sequence + 64))
	f.handles[key] = h
	return Result{Handle: key}
}

type frozenCallerKey struct{}

func (f *frozenTaskHost) Execute(handle string, r Request) Result {
	before := f.Snapshot().Counts
	receipt, err := f.store.client.Start(context.Background(), f.handles[handle], f.operation(r))
	if err != nil {
		return frozenResult(receipt, err)
	}
	if receipt.State != "prepared" && receipt.State != "in_progress" {
		return frozenResult(receipt, nil)
	}
	if f.block.Load() {
		if _, err := f.store.client.Start(context.Background(), f.handles[handle], f.observation(r)); err != nil {
			f.t.Fatal("start owned room observation", err)
		}
		select {
		case <-f.roomPeer.entered:
		case <-time.After(testWait):
			f.t.Fatal("room observation did not become pending")
		}
		select {
		case <-f.question:
		case <-time.After(testWait):
			f.t.Fatal("native permission did not become pending")
		}
		stored, err := f.a.taskServices.host.Read(context.Background(), f.scope(r), r.Operation)
		return frozenResult(stored, err)
	}
	select {
	case ended := <-f.terminal:
		if f.lost.Load() {
			after := f.Snapshot().Counts
			count := map[string]int{"OpenChat": after.Allocations - before.Allocations, "TakeTurn": after.Prompts - before.Prompts, "InvokeAction": after.Actions - before.Actions, "PostRoom": after.Posts - before.Posts}[r.Kind]
			if count != 1 {
				f.t.Fatalf("lost-response fixture did not consume one %s request: before %+v after %+v receipt %+v", r.Kind, before, after, ended)
			}
		}
		if ended.State == "failed" {
			f.t.Logf("host operation failed: %+v", ended)
		}
		return frozenResult(ended, nil)
	case <-time.After(testWait):
		f.t.Fatal("task operation did not settle")
	}
	return Result{}
}
func (f *frozenTaskHost) Rebind(task, generation, caller string) Result {
	scope := f.scopes[task]
	scope.Generation = generation
	_, err := f.store.client.Rebind(context.Background(), scope)
	if err != nil {
		return frozenResult(taskhost.Receipt{}, err)
	}
	return Result{Handle: "recovery-only"}
}
func (f *frozenTaskHost) Inspect() {
	_, _ = f.store.client.Inspect(context.Background(), taskhost.Scope{Workspace: "workspace-a", Task: "task-a", Generation: "generation-1"})
}
func (f *frozenTaskHost) Snapshot() Snapshot {
	f.mu.Lock()
	s := Snapshot{Counts: f.counts, Events: append([]string(nil), f.events...), Joined: f.joined}
	f.mu.Unlock()
	state := f.a.taskServices.host.Snapshot()
	s.Workers = state.Workers
	s.AdmissionClosed = state.AdmissionClosed
	s.HandlesRevoked = state.Handles == 0
	s.CancelRequests = int(f.peer.cancels.Load())
	s.Questions = len(f.a.PendingTaskPermissions()) + len(f.a.PendingPermissions())
	if f.roomPeer != nil {
		s.Observers = int(f.roomPeer.live.Load())
	}
	s.Receipts = map[string]Result{}
	key, _ := json.Marshal([]string{frozenService, "workspace-a", "task-a"})
	sum := sha256.Sum256(key)
	data, err := (records.Store{Dir: filepath.Join(configDir(), "task-operations")}).Read(hex.EncodeToString(sum[:]))
	if err == nil {
		var record struct{ Receipts map[string]taskhost.Receipt }
		if err := json.Unmarshal(data, &record); err != nil {
			f.t.Fatal(err)
		}
		s.Checkpointed = len(record.Receipts) > 0
		for id, r := range record.Receipts {
			s.Receipts[id] = frozenResult(r, nil)
			if r.State == "prepared" || r.State == "in_progress" {
				s.Checkpointed = false
			}
		}
	} else if !os.IsNotExist(err) {
		f.t.Fatal(err)
	}
	s.ProtectedRecords = f.protectedSnapshot()
	return s
}
func (f *frozenTaskHost) Advance(d time.Duration) {
	f.mu.Lock()
	f.clock = f.clock.Add(d)
	f.mu.Unlock()
}
func (f *frozenTaskHost) Cancel(handle string) {
	if err := f.store.client.Cancel(f.handles[handle]); err != nil {
		f.t.Fatal(err)
	}
}
func (f *frozenTaskHost) StartShutdown() { f.a.taskServices.host.CloseAdmission() }
func (f *frozenTaskHost) FinishShutdown(budget time.Duration) {
	f.Advance(budget)
	f.a.shutdown(context.Background())
	f.mu.Lock()
	f.joined = true
	f.mu.Unlock()
}
func (f *frozenTaskHost) Restart() {
	f.a.shutdown(context.Background())
	openFolder()
	a := NewApp()
	f.a = a
	frozenConsumer = f.store
	c := newTestProviderConnection()
	a.runtime.PutProviderLocked("codex", newTestProvider(c, acp.InitializeResult{AgentCapabilities: acp.AgentCapabilities{LoadSession: true}}))
	f.configure(c)
	if err := a.startTaskServices(); err != nil {
		f.t.Fatal(err)
	}
}
func (f *frozenTaskHost) Change(change string) {
	switch change {
	case "advance-task-generation":
		if err := os.WriteFile(filepath.Join(f.store.path, "generation"), []byte("generation-2"), 0600); err != nil {
			f.t.Fatal(err)
		}
	case "workspace-remount", "workspace-missing", "workspace-ambiguous", "workspace-inaccessible":
		err := f.a.updateSettings(func(s *Settings) error {
			switch change {
			case "workspace-remount":
				s.WorkspaceRecords[0].ExecutionPath = f.t.TempDir()
			case "workspace-missing":
				s.WorkspaceRecords = nil
			case "workspace-ambiguous":
				s.WorkspaceRecords = append(s.WorkspaceRecords, s.WorkspaceRecords[0])
			case "workspace-inaccessible":
				s.WorkspaceRecords[0].ExecutionPath = filepath.Join(f.t.TempDir(), "absent")
			}
			return nil
		})
		if err != nil {
			f.t.Fatal(err)
		}
	case "bin-owning-chat", "bin-referenced-chat":
		id := "codex:owner-chat"
		if change == "bin-referenced-chat" {
			id = "codex:chat-a"
		}
		if err := f.a.RemoveChat(id); err != nil {
			f.t.Fatal(err)
		}
		f.lastBinned = id
	case "restore-chat":
		if result := f.a.RestoreRemovedChat(f.lastBinned); !result.OK {
			f.t.Fatal(result)
		}
	case "lose-response-after-dispatch":
		f.lost.Store(true)
	case "block-owned-work-and-register-question-observer":
		f.block.Store(true)
	case "settle-owned-cancellation":
		deadline := time.After(testWait)
		for f.a.taskServices.host.Snapshot().Workers != 0 {
			select {
			case <-f.terminal:
			case <-deadline:
				f.t.Fatal("owned cancellation did not settle")
			}
		}
	case "enable-complete-room-peer":
		f.enableRoomPeer()
	default:
		f.t.Fatal("unknown fixture control", change)
	}
}

type frozenNativePeer struct {
	*testProviderConnection
	host    *frozenTaskHost
	mu      sync.Mutex
	cancel  chan struct{}
	once    sync.Once
	cancels atomic.Int32
}

func (p *frozenNativePeer) Call(ctx context.Context, method string, in, out any) error {
	p.host.mu.Lock()
	if method == "session/new" {
		p.host.counts.Allocations++
	}
	if method == "session/load" || method == "session/resume" {
		p.host.counts.Loads++
	}
	p.host.mu.Unlock()
	if method == "session/new" && p.host.lost.Load() {
		return errors.New("invented allocation request consumed; response lost")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.testProviderConnection.Call(ctx, method, in, out)
}
func (p *frozenNativePeer) PromptWithDispatch(_ context.Context, params acp.PromptParams, out any, dispatched func()) error {
	p.host.mu.Lock()
	p.host.counts.Prompts++
	p.host.mu.Unlock()
	dispatched()
	if p.host.lost.Load() {
		return errors.New("invented prompt request consumed; response lost")
	}
	if p.host.block.Load() {
		questionDone := make(chan struct{})
		go func() {
			defer close(questionDone)
			p.host.a.permissions().Ask(context.Background(), params.SessionID, nil, json.RawMessage(`{"title":"Invented pending tool"}`), []acp.PermissionOption{{OptionID: "allow_once", Name: "Allow", Kind: "allow_once"}})
		}()
		select {
		case <-p.cancel:
		case <-p.Done():
		}
		<-questionDone
		return acp.Result(acp.PromptResult{StopReason: "cancelled"}, out)
	}
	return acp.Result(acp.PromptResult{StopReason: "end_turn"}, out)
}
func (p *frozenNativePeer) Notify(method string, in any) error {
	if method == "session/cancel" {
		p.cancels.Add(1)
		p.host.event("cancel-requested")
		p.once.Do(func() { close(p.cancel) })
		return nil
	}
	return nil
}

type frozenRoomPeer struct {
	host      *frozenTaskHost
	roots     map[string]string
	mu        sync.Mutex
	claimed   bool
	phase     int
	entered   chan struct{}
	live      atomic.Int32
	cancelled atomic.Int32
	completed atomic.Int32
}

func (f *frozenTaskHost) enableRoomPeer() {
	f.roomPeer = &frozenRoomPeer{host: f, roots: map[string]string{}, claimed: true, phase: 1, entered: make(chan struct{}, 4)}
	f.a.taskServices.mu.Lock()
	f.a.taskServices.rooms = f.roomPeer
	f.a.taskServices.mu.Unlock()
}

func (f *frozenTaskHost) observation(r Request) taskhost.Operation {
	return taskhost.Operation{ID: r.Operation + "-observe", Kind: "ObserveRoom", Target: "invented-room", Input: json.RawMessage(`{"wait":true}`)}
}

func (p *frozenRoomPeer) Review(_ context.Context, s taskhost.Scope, permit taskhost.Permit) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.review(s, permit)
}
func (p *frozenRoomPeer) review(s taskhost.Scope, permit taskhost.Permit) (string, error) {
	if s.Workspace != "workspace-a" || s.Task != "task-a" || s.Generation != "generation-1" || s.OwnerChat != "codex:owner-chat" || permit.Target != "invented-room" {
		return "", errors.New("invented dedicated room has a different workspace, canonical lead, task claim or phase")
	}
	if (permit.Kind == "PostRoom" || permit.Kind == "ObserveRoom") && !p.claimed {
		return "", errors.New("invented dedicated room has no current phase claim")
	}
	return fmt.Sprintf("invented-room-phase-%d-claimed-%t", p.phase, p.claimed), nil
}
func (p *frozenRoomPeer) Ensure(ctx context.Context, s taskhost.Scope, o taskhost.Operation, _ taskhost.Capture) (taskhost.RoomResult, error) {
	if _, err := p.Review(ctx, s, taskhost.Permit{Kind: o.Kind, Target: o.Target}); err != nil {
		return taskhost.RoomResult{}, err
	}
	return taskhost.RoomResult{Identity: "invented-room"}, nil
}
func (p *frozenRoomPeer) Claim(_ context.Context, s taskhost.Scope, o taskhost.Operation, _ taskhost.Capture) (taskhost.RoomResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.review(s, taskhost.Permit{Kind: o.Kind, Target: o.Target}); err != nil {
		return taskhost.RoomResult{}, err
	}
	if !p.claimed {
		p.phase++
		p.claimed = true
	}
	return taskhost.RoomResult{Identity: fmt.Sprintf("invented-room-claim-%d", p.phase)}, nil
}
func (p *frozenRoomPeer) Post(_ context.Context, s taskhost.Scope, o taskhost.Operation, capture taskhost.Capture) (taskhost.RoomResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := taskOperationKey(s, o.ID)
	p.host.mu.Lock()
	p.host.counts.Posts++
	p.host.mu.Unlock()
	if _, err := p.review(s, taskhost.Permit{Kind: "PostRoom", Target: o.Target}); err != nil {
		return taskhost.RoomResult{}, err
	}
	if p.roots[key] != "" {
		return taskhost.RoomResult{}, errors.New("invented peer received a duplicate post")
	}
	root := "root-" + o.ID
	p.roots[key] = root
	if p.host.lost.Load() {
		return taskhost.RoomResult{}, errors.New("invented post consumed; response lost")
	}
	return taskhost.RoomResult{Identity: root}, nil
}
func (p *frozenRoomPeer) Observe(ctx context.Context, s taskhost.Scope, o taskhost.Operation, _ taskhost.Capture) (taskhost.RoomResult, error) {
	if err := ctx.Err(); err != nil {
		return taskhost.RoomResult{}, err
	}
	if _, err := p.Review(ctx, s, taskhost.Permit{Kind: "ObserveRoom", Target: o.Target}); err != nil {
		return taskhost.RoomResult{}, err
	}
	if string(o.Input) == `{"wait":true}` {
		p.live.Add(1)
		defer func() { p.live.Add(-1); p.completed.Add(1) }()
		p.entered <- struct{}{}
		<-ctx.Done()
		p.cancelled.Add(1)
		return taskhost.RoomResult{}, ctx.Err()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	rows := map[string]string{}
	for key, root := range p.roots {
		var identity []string
		if json.Unmarshal([]byte(key), &identity) == nil && len(identity) == 4 && identity[0] == s.Service && identity[1] == s.Workspace && identity[2] == s.Task {
			rows[identity[3]] = root
		}
	}
	result, _ := json.Marshal(map[string]any{"phase": p.phase, "roots": rows, "cursor": len(rows)})
	return taskhost.RoomResult{Identity: "invented-root-cursor", Result: result}, nil
}
func (p *frozenRoomPeer) Release(_ context.Context, s taskhost.Scope, o taskhost.Operation, _ taskhost.Capture) (taskhost.RoomResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.review(s, taskhost.Permit{Kind: o.Kind, Target: o.Target}); err != nil {
		return taskhost.RoomResult{}, err
	}
	p.claimed = false
	return taskhost.RoomResult{Identity: "invented-room", Result: json.RawMessage(`{"released":true}`)}, nil
}
func (p *frozenRoomPeer) Recover(ctx context.Context, s taskhost.Scope, o taskhost.Operation, capture taskhost.Capture) (taskhost.RoomResult, error) {
	return p.Observe(ctx, s, o, capture)
}
