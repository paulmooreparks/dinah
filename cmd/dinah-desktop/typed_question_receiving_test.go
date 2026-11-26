//go:build quality_typed_questions && quality_task_host

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/records"
	"studio/internal/services"
	"studio/internal/taskhost"
)

type typedEntry struct {
	scope  taskhost.Scope
	op     taskhost.Operation
	handle *taskhost.Handle
	key    string
	done   chan struct{}
	once   sync.Once
}

type typedQuestionFixture struct {
	f         *frozenTaskHost
	client    taskhost.Client
	mu        sync.Mutex
	entries   map[string]*typedEntry
	keys      map[string]*typedEntry
	events    []map[string]any
	changed   chan struct{}
	trace     testCollector[map[string]any]
	sequence  int
	artifact  string
	protected string
	barrier   atomic.Pointer[typedAnswerBarrier]
	replays   atomic.Int32
}

const typedFixtureService = "a-typed-question-consumer"

var typedRegistration sync.Once
var typedConsumerClient taskhost.Client

type typedPublicService struct{ client taskhost.Client }

func (s *typedPublicService) Inspect(ctx context.Context, scope taskhost.Scope) (taskhost.Inspection, error) {
	return s.client.Inspect(ctx, scope)
}

type typedAnswerBarrier struct {
	entered atomic.Int32
	ready   chan struct{}
}

type typedNativeTrace struct {
	*frozenNativePeer
	fixture *typedQuestionFixture
}

func (p *typedNativeTrace) Call(ctx context.Context, method string, in, out any) error {
	p.fixture.trace.Add(map[string]any{"method": method, "input": in})
	return p.frozenNativePeer.Call(ctx, method, in, out)
}
func (p *typedNativeTrace) Notify(method string, in any) error {
	p.fixture.trace.Add(map[string]any{"method": method, "input": in})
	return p.frozenNativePeer.Notify(method, in)
}
func (p *typedNativeTrace) PromptWithDispatch(ctx context.Context, in acp.PromptParams, out any, dispatched func()) error {
	p.fixture.trace.Add(map[string]any{"method": "session/prompt", "input": in})
	return p.frozenNativePeer.PromptWithDispatch(ctx, in, out, dispatched)
}

func newTypedQuestionFixture(t *testing.T) *typedQuestionFixture {
	typedRegistration.Do(func() {
		services.RegisterWindowNamed(typedFixtureService, "Invented Typed Questions", nil, func(c taskhost.Client) (services.Service, services.Participant, error) {
			typedConsumerClient = c
			return services.Object(&typedPublicService{client: c}), frozenConsumer, nil
		})
	})
	f := newFrozenTaskHost(t).(*frozenTaskHost)
	x := &typedQuestionFixture{f: f, entries: map[string]*typedEntry{}, keys: map[string]*typedEntry{}, events: []map[string]any{}, changed: make(chan struct{}, 1), artifact: os.Getenv("QUALITY_TYPED_ARTIFACT")}
	if x.artifact == "" {
		x.artifact = t.TempDir()
	}
	if err := os.MkdirAll(x.artifact, 0700); err != nil {
		t.Fatal(err)
	}
	x.protected = f.protectedSnapshot()
	x.client = typedConsumerClient
	x.configure()
	return x
}

func (x *typedQuestionFixture) configure() {
	a := x.f.a
	prior := a.runtime.ProviderLocked("codex")
	p := newTestProvider(&typedNativeTrace{frozenNativePeer: x.f.peer, fixture: x}, prior.Initialization())
	a.runtime.PutProviderLocked("codex", p)
	go a.lifecycle().Watch("codex", p)
	a.taskQuestions.check = func(ctx context.Context, s taskhost.Scope) error {
		if err := a.checkTaskIdentity(ctx, s); err != nil {
			return err
		}
		if b := x.barrier.Load(); b != nil {
			if b.entered.Add(1) == 2 {
				close(b.ready)
			}
			select {
			case <-b.ready:
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(testWait):
				return errors.New("concurrent answers did not overlap at authority check")
			}
		}
		return nil
	}
	a.sink = func(name string, value any) {
		x.mu.Lock()
		x.events = append(x.events, map[string]any{"name": name, "value": value})
		if name == "task:permission" {
			q := value.(taskhost.Question)
			if !x.f.consent.Load() {
				if e := x.entries[q.Task]; e != nil {
					if e.key != "" {
						x.replays.Add(1)
					}
					e.key = q.Key
					x.keys[q.Key] = e
				}
			}
		}
		if name == "task:receipt" {
			r := value.(taskhost.Receipt)
			if e := x.entries[r.Scope.Task]; e != nil && r.State != "prepared" && r.State != "in_progress" {
				e.once.Do(func() { close(e.done) })
			}
			if r.State != "prepared" && r.State != "in_progress" {
				x.f.terminal <- r
			}
		}
		x.mu.Unlock()
		if name == "task:permission" && x.f.consent.Load() {
			if err := a.Answer(value.(taskhost.Question).Key, "allow_once"); err != nil {
				x.f.t.Error(err)
			}
		}
		select {
		case x.changed <- struct{}{}:
		default:
		}
	}
}

func typedResponse(err error) map[string]any {
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	return map[string]any{"ok": true}
}

func (x *typedQuestionFixture) ask(raw json.RawMessage) any {
	x.mu.Lock()
	x.sequence++
	id := fmt.Sprintf("typed-%d", x.sequence)
	x.mu.Unlock()
	scope := taskhost.Scope{Service: typedFixtureService, Workspace: "workspace-a", Task: id, Generation: "generation-1"}
	op := taskhost.Operation{ID: "ask", Kind: "AskPerson", Input: raw}
	r, err := x.client.Review(context.Background(), taskhost.Plan{Scope: scope, Permits: []taskhost.Permit{{Kind: op.Kind, Input: raw}}, Deadline: time.Now().Add(time.Hour)})
	if err != nil {
		return typedResponse(err)
	}
	x.f.consent.Store(true)
	h, err := x.client.Begin(context.Background(), r.ID)
	x.f.consent.Store(false)
	if err != nil {
		return typedResponse(err)
	}
	e := &typedEntry{scope: scope, op: op, handle: h, done: make(chan struct{})}
	x.mu.Lock()
	x.entries[id] = e
	x.mu.Unlock()
	if _, err = x.client.Start(context.Background(), h, op); err != nil {
		return typedResponse(err)
	}
	deadline := time.NewTimer(testWait)
	defer deadline.Stop()
	for {
		x.mu.Lock()
		key := e.key
		x.mu.Unlock()
		if key != "" {
			return map[string]any{"ok": true, "key": key}
		}
		select {
		case <-x.changed:
		case <-e.done:
			r, err := x.receipt(e)
			if err != nil {
				return typedResponse(err)
			}
			return typedResponse(fmt.Errorf("question ended before asking: %s %s", r.State, r.Reason))
		case <-deadline.C:
			return typedResponse(errors.New("question did not become pending"))
		}
	}
}

func (x *typedQuestionFixture) entry(key string) *typedEntry {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.keys[key]
}
func (x *typedQuestionFixture) receipt(e *typedEntry) (taskhost.Receipt, error) {
	key, _ := json.Marshal([]string{e.scope.Service, e.scope.Workspace, e.scope.Task})
	sum := sha256.Sum256(key)
	data, err := (records.Store{Dir: filepath.Join(configDir(), "task-operations")}).Read(hex.EncodeToString(sum[:]))
	if err != nil {
		return taskhost.Receipt{}, err
	}
	var record struct {
		Receipts map[string]taskhost.Receipt `json:"receipts"`
	}
	if err = json.Unmarshal(data, &record); err != nil {
		return taskhost.Receipt{}, err
	}
	return record.Receipts[e.op.ID], nil
}
func (x *typedQuestionFixture) wait(e *typedEntry) error {
	if e == nil {
		return errors.New("unknown fixture key")
	}
	select {
	case <-e.done:
		return nil
	case <-time.After(testWait):
		return errors.New("durable operation did not settle")
	}
}
func (x *typedQuestionFixture) answer(key, raw string, legacy bool) any {
	var err error
	if legacy {
		err = x.f.a.Answer(key, raw)
	} else {
		err = x.f.a.AnswerTaskQuestion(key, raw)
	}
	if err == nil {
		err = x.wait(x.entry(key))
	}
	return typedResponse(err)
}
func (x *typedQuestionFixture) pending() any {
	rows := []map[string]any{}
	for _, q := range x.f.a.PendingTaskPermissions() {
		data, _ := json.Marshal(q)
		var row map[string]any
		_ = json.Unmarshal(data, &row)
		row["owner"] = q.QuestionOwner
		row["text"] = q.Question
		if options, ok := row["options"].([]any); ok {
			for _, o := range options {
				option := o.(map[string]any)
				if option["kind"] == "" {
					delete(option, "kind")
				}
			}
		}
		rows = append(rows, row)
	}
	return rows
}
func (x *typedQuestionFixture) snapshot() any {
	pending := x.pending()
	x.mu.Lock()
	entries := make([]*typedEntry, 0, len(x.entries))
	for _, e := range x.entries {
		entries = append(entries, e)
	}
	events := []map[string]any{}
	for _, event := range x.events {
		if name := event["name"].(string); name == "task:permission" || name == "task:answered" || name == "task:receipt" {
			events = append(events, event)
		}
	}
	x.mu.Unlock()
	receipts := map[string]any{}
	for _, e := range entries {
		r, err := x.receipt(e)
		if err != nil {
			receipts[e.scope.Task] = err.Error()
		} else {
			receipts[e.scope.Task] = r
		}
	}
	generation, err := os.ReadFile(filepath.Join(x.f.store.path, "generation"))
	if err != nil {
		generation = []byte(err.Error())
	}
	return map[string]any{"pending": pending, "receipts": receipts, "events": events, "host": x.f.a.taskServices.host.Snapshot(), "generation": string(generation), "protected": x.f.protectedSnapshot(), "providerRequests": x.providerRequests(), "actions": x.actionCount(), "remoteWrites": x.f.docPeer.writes.Load()}
}
func (x *typedQuestionFixture) actionCount() int {
	x.f.mu.Lock()
	defer x.f.mu.Unlock()
	return x.f.counts.Actions
}
func (x *typedQuestionFixture) settlement(key string) any {
	e := x.entry(key)
	results := []any{}
	events := []any{}
	if e != nil {
		r, err := x.receipt(e)
		if err == nil && len(r.Result) > 0 {
			var result any
			_ = json.Unmarshal(r.Result, &result)
			results = append(results, result)
		}
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	for _, event := range x.events {
		if event["name"] == "task:answered" && event["value"].(taskhost.QuestionEnd).Key == key {
			events = append(events, event)
		}
	}
	return map[string]any{"durableResults": results, "answeredEvents": events}
}
func (x *typedQuestionFixture) observations() any {
	provider := x.providerRequests()
	actions := []any{}
	if n := x.actionCount(); n != 0 {
		actions = append(actions, map[string]any{"attempts": n})
	}
	return map[string]any{"artifactDirectory": x.artifact, "providerRequests": provider, "actionRequests": actions, "remoteWriteAttempts": x.f.docPeer.writes.Load(), "protectedBefore": x.protected, "protectedAfter": x.f.protectedSnapshot(), "questionReplayCount": x.replays.Load()}
}

func (x *typedQuestionFixture) providerRequests() []map[string]any {
	requests := x.trace.Snapshot()
	if requests == nil {
		requests = []map[string]any{}
	}
	x.f.mu.Lock()
	starts := x.f.counts.Starts
	x.f.mu.Unlock()
	if starts != 0 {
		requests = append(requests, map[string]any{"method": "provider preparation or executable discovery", "attempts": starts})
	}
	return requests
}

func (x *typedQuestionFixture) call(method string, args []json.RawMessage) any {
	str := func(i int) string {
		var s string
		if i < len(args) {
			_ = json.Unmarshal(args[i], &s)
		}
		return s
	}
	switch method {
	case "ask":
		return x.ask(args[0])
	case "pending":
		return x.pending()
	case "answer":
		return x.answer(str(0), str(1), false)
	case "answerLegacy":
		return x.answer(str(0), str(1), true)
	case "answerBytes":
		var bytes []int
		_ = json.Unmarshal(args[1], &bytes)
		raw := make([]byte, len(bytes))
		for i, b := range bytes {
			raw[i] = byte(b)
		}
		return x.answer(str(0), string(raw), false)
	case "concurrentAnswers":
		var calls [][]string
		_ = json.Unmarshal(args[0], &calls)
		barrier := &typedAnswerBarrier{ready: make(chan struct{})}
		x.barrier.Store(barrier)
		defer x.barrier.Store(nil)
		results := make([]any, len(calls))
		var wg sync.WaitGroup
		for i, c := range calls {
			wg.Go(func() { results[i] = x.answer(c[0], c[1], false) })
		}
		wg.Wait()
		return results
	case "result":
		e := x.entry(str(0))
		if err := x.wait(e); err != nil {
			return typedResponse(err)
		}
		r, err := x.receipt(e)
		if err != nil {
			return typedResponse(err)
		}
		return r.Result
	case "snapshot":
		return x.snapshot()
	case "settlement":
		return x.settlement(str(0))
	case "observations":
		return x.observations()
	case "mutatePendingCopy":
		var patch struct {
			Options                      []taskhost.QuestionOption
			MinSelections, MaxSelections int
		}
		_ = json.Unmarshal(args[1], &patch)
		for _, q := range x.f.a.PendingTaskPermissions() {
			if q.Key == str(0) {
				if len(q.Options) > 0 && len(patch.Options) > 0 {
					q.Options[0] = patch.Options[0]
				}
				q.MinSelections = patch.MinSelections
				q.MaxSelections = patch.MaxSelections
			}
		}
		return typedResponse(nil)
	case "callService":
		payload, _ := json.Marshal([]json.RawMessage{args[1]})
		_, err := x.f.a.CallService(typedFixtureService, str(0), string(payload))
		return typedResponse(err)
	case "withdraw":
		e := x.entry(str(1))
		if e == nil {
			return typedResponse(errors.New("unknown question"))
		}
		var err error
		switch str(0) {
		case "cancel":
			err = x.client.Cancel(e.handle)
		case "generation-change":
			err = os.WriteFile(filepath.Join(x.f.store.path, "generation"), []byte("generation-2"), 0600)
			_ = x.f.a.PendingTaskPermissions()
		case "removal":
			err = os.Remove(filepath.Join(x.f.store.path, "generation"))
			_ = x.f.a.PendingTaskPermissions()
		case "shutdown":
			x.f.a.closeTaskServices()
		default:
			err = errors.New("unknown withdrawal")
		}
		if err == nil {
			err = x.wait(e)
		}
		return typedResponse(err)
	case "restart":
		x.f.Restart()
		x.client = typedConsumerClient
		x.configure()
		// Rebinding reconciles actual saved receipts. Starting the same reviewed
		// operation again must return that receipt without asking another question.
		x.mu.Lock()
		entries := make([]*typedEntry, 0, len(x.entries))
		for _, e := range x.entries {
			entries = append(entries, e)
		}
		x.mu.Unlock()
		for _, e := range entries {
			_, _ = x.client.Rebind(context.Background(), e.scope)
			r, err := x.client.Review(context.Background(), taskhost.Plan{Scope: e.scope, Permits: []taskhost.Permit{{Kind: e.op.Kind, Input: e.op.Input}}, Deadline: time.Now().Add(time.Hour)})
			if err != nil {
				continue
			}
			x.f.consent.Store(true)
			h, err := x.client.Begin(context.Background(), r.ID)
			x.f.consent.Store(false)
			if err == nil {
				_, _ = x.client.Start(context.Background(), h, e.op)
			}
		}
		return typedResponse(nil)
	case "close":
		x.f.a.shutdown(context.Background())
		return typedResponse(nil)
	}
	return typedResponse(fmt.Errorf("unknown fixture operation %s", method))
}

func TestTypedQuestionAcceptanceServer(t *testing.T) {
	ready := os.Getenv("QUALITY_TYPED_READY")
	if ready == "" {
		t.Skip("requires an isolated receiving driver")
	}
	x := newTypedQuestionFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan struct{})
	var once sync.Once
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Method string
			Args   []json.RawMessage
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&in); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		out := x.call(in.Method, in.Args)
		_ = json.NewEncoder(w).Encode(out)
		// Retain exact supplied raw bytes as a base64 JSON byte slice as well as
		// the canonical response and the unfiltered persisted task generations.
		x.mu.Lock()
		defer x.mu.Unlock()
		log, err := os.OpenFile(filepath.Join(x.artifact, "requests.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err == nil {
			_ = json.NewEncoder(log).Encode(map[string]any{"request": in, "response": out})
			_ = log.Close()
		}
		if in.Method == "snapshot" || in.Method == "close" {
			if in.Method == "close" {
				data, _ := json.Marshal(map[string]any{"events": x.events, "providerRequests": x.providerRequests(), "observations": x.observations()})
				_ = os.WriteFile(filepath.Join(x.artifact, "final-observations.json"), data, 0600)
			}
			_ = filepath.WalkDir(filepath.Join(configDir(), "task-operations"), func(path string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					if data, e := os.ReadFile(path); e == nil {
						_ = os.WriteFile(filepath.Join(x.artifact, filepath.Base(path)), data, 0600)
					}
				}
				return nil
			})
		}
		if in.Method == "close" {
			once.Do(func() { close(closed) })
		}
	})
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	}()
	if err := os.WriteFile(ready, []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Minute):
		t.Error("receiving driver did not close")
	}
	ctx, cancel := context.WithTimeout(context.Background(), testWait)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func TestTypedQuestionBindingCountersObserveRealEffects(t *testing.T) {
	x := newTypedQuestionFixture(t)
	input, _ := json.Marshal([]taskhost.Scope{{Workspace: "workspace-a", Task: "control", Generation: "generation-1"}})
	if _, err := x.f.a.CallService(typedFixtureService, "Inspect", string(input)); err != nil {
		t.Fatal("registered service positive control", err)
	}
	for _, name := range []string{"Answer", "AnswerTyped", "AnswerTaskQuestion"} {
		if _, ok := reflect.TypeFor[taskhost.Client]().MethodByName(name); ok {
			t.Fatal("service received answering authority", name)
		}
	}
	if _, err := x.f.a.NewSessionAt(x.f.workspace); err != nil {
		t.Fatal(err)
	}
	if len(x.trace.Snapshot()) == 0 {
		t.Fatal("provider counter missed a real allocation")
	}
	r := Request{Task: "control", Generation: "generation-1", Operation: "effect", Kind: "InvokeAction", Input: "invented", Digest: "digest-1"}
	review := x.f.Review(r)
	begin := x.f.Begin(review, "trusted-person")
	x.f.Execute(begin.Handle, r)
	if x.actionCount() != 1 {
		t.Fatal("action counter missed real dispatch")
	}
	v, err := x.f.a.GetDraft(x.f.remoteDraft)
	if err != nil {
		t.Fatal(err)
	}
	x.f.docPeer.refuse.Store(true)
	_, _ = x.f.a.SaveDraft(v.ID, v.Revision)
	if x.f.docPeer.writes.Load() != 1 {
		t.Fatal("remote counter missed refused attempt")
	}
	x.f.docPeer.refuse.Store(false)
	if _, err := x.f.a.SaveDraft(v.ID, v.Revision); err != nil {
		t.Fatal(err)
	}
	if x.f.docPeer.writes.Load() != 2 || reflect.DeepEqual(x.protected, x.f.protectedSnapshot()) {
		t.Fatal("remote counter missed successful write")
	}
}
