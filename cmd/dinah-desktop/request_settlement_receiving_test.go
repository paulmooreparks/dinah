//go:build quality_request_settlement && quality_typed_questions && quality_task_host

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/applet"
	"studio/internal/bus"
	"studio/internal/requestsettlement"
	"studio/internal/taskhost"
)

// This receiving host exposes raw real-App operations. Canonical acceptance
// observations are assembled by the binding from these retained responses.
type requestSettlementFixture struct {
	t        *testing.T
	typed    *typedQuestionFixture
	owner    string
	group    string
	artifact string
	mu       sync.Mutex
	requests []map[string]any
}

func newRequestSettlementFixture(t *testing.T) *requestSettlementFixture {
	x := newTypedQuestionFixture(t)
	f := &requestSettlementFixture{t: t, typed: x, owner: "codex:owner-chat", artifact: x.artifact, requests: []map[string]any{}}
	refs := []BusChatRef{}
	for _, id := range []string{f.owner, "codex:chat-a"} {
		s := x.f.a.session(id)
		if s == nil {
			t.Fatal("the seeded group member is missing", id)
		}
		if _, err := x.f.a.RenameSession(id, "Invented "+id); err != nil {
			t.Fatal(err)
		}
		refs = append(refs, BusChatRef{ID: id, Cwd: s.Cwd(), Title: id})
	}
	group, err := x.f.a.BusStartGroup("Invented settlement group", refs)
	if err != nil {
		t.Fatal(err)
	}
	f.group = group.Name
	return f
}

func (x *requestSettlementFixture) startQuestion(source string) (string, error) {
	q := map[string]any{"title": "Review an invented request."}
	switch source {
	case "task-single":
		q["choices"] = []string{"Proceed", "Wait"}
	case "task-text":
		q["answer"] = taskhost.QuestionSpec{Type: "text", MinLength: 1, MaxLength: 32}
	case "task-multiple":
		q["answer"] = taskhost.QuestionSpec{Type: "multiple", Options: []taskhost.QuestionOption{{OptionID: "beta", Name: "Beta"}, {OptionID: "alpha", Name: "Alpha"}}, MinSelections: 1, MaxSelections: 2}
	default:
		return "", errors.New("unknown question source")
	}
	raw, _ := json.Marshal(q)
	f := x.typed
	f.mu.Lock()
	f.sequence++
	id := fmt.Sprintf("settlement-%d", f.sequence)
	f.mu.Unlock()
	scope := taskhost.Scope{Service: typedFixtureService, Workspace: "workspace-a", Task: id, Generation: "generation-1", OwnerChat: x.owner}
	op := taskhost.Operation{ID: "ask", Kind: "AskPerson", Input: raw}
	review, err := f.client.Review(context.Background(), taskhost.Plan{Scope: scope, Permits: []taskhost.Permit{{Kind: op.Kind, Input: raw}}, Deadline: time.Now().Add(time.Minute)})
	if err != nil {
		return "", err
	}
	f.f.consent.Store(true)
	h, err := f.client.Begin(context.Background(), review.ID)
	f.f.consent.Store(false)
	if err != nil {
		return "", err
	}
	e := &typedEntry{scope: scope, op: op, handle: h, done: make(chan struct{})}
	f.mu.Lock()
	f.entries[id] = e
	f.mu.Unlock()
	if _, err = f.client.Start(context.Background(), h, op); err != nil {
		return "", err
	}
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		f.mu.Lock()
		key := e.key
		f.mu.Unlock()
		if key != "" {
			return key, nil
		}
		select {
		case <-f.changed:
		case <-e.done:
			r, readErr := f.receipt(e)
			return "", fmt.Errorf("question ended before admission: %+v; %v", r, readErr)
		case <-timer.C:
			return "", errors.New("question did not become pending")
		}
	}
}

func (x *requestSettlementFixture) seed(source string) (any, error) {
	a := x.typed.f.a
	if strings.HasPrefix(source, "task-") {
		key, err := x.startQuestion(source)
		if err != nil {
			return nil, err
		}
		return a.RequestState(key)
	}
	if source == "attention-report" || source == "ownerless-report" {
		owner := x.owner
		kind, severity := "turn-failure", "error"
		if source == "ownerless-report" {
			owner = ""
			kind, severity = "app", "warning"
		}
		item, err := a.SaveNotification(Notification{ID: "invented-settlement-report", Kind: kind, Severity: severity, Title: "Invented report", Body: "The result is unknown.", SessionID: owner, Workspace: x.typed.f.workspace})
		if err != nil {
			return nil, err
		}
		return a.RequestState(notificationRequestID(item))
	}
	var decision bus.Decision
	err := a.busHost().With(func(b *bus.Bus) error {
		owner, err := b.Session(context.Background(), a.knownBusID(x.owner))
		if err != nil {
			return err
		}
		if source != "request" && source != "partial-request" {
			return errors.New("unknown source")
		}
		questions := []bus.PersonQuestion{{Number: 1, Text: "Proceed with the invented work?", Recommendation: "Proceed."}}
		if source == "partial-request" {
			questions = append(questions, bus.PersonQuestion{Number: 2, Text: "Keep the invented record?", Recommendation: "Keep it."})
		}
		decision, err = b.AskPerson(context.Background(), owner, bus.AskPersonInput{OperationID: "seed-request", Questions: questions})
		return err
	})
	if err != nil {
		return nil, err
	}
	if source == "partial-request" {
		_, err = a.AnswerDecision(bus.AnswerDecisionInput{ID: decision.ID, Revision: decision.Revision, OperationID: "seed-prior-answer", Answers: []bus.DecisionAnswer{{Number: 1, Text: "The prior answer."}}})
		if err != nil {
			return nil, err
		}
	}
	return a.RequestState(decision.ID)
}

func (x *requestSettlementFixture) call(method string, raw json.RawMessage) (any, error) {
	a := x.typed.f.a
	switch method {
	case "seed":
		var in struct {
			Source string `json:"source"`
		}
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, err
		}
		return x.seed(in.Source)
	case "state":
		var in struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, err
		}
		return a.RequestState(in.ID)
	case "dismiss":
		var in requestsettlement.Input
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, err
		}
		return a.DismissRequest(in)
	case "withdraw", "withdrawal-outcome":
		bundle, err := a.requestSettlementBundle()
		if err != nil {
			return nil, err
		}
		action := withdrawRequestAction
		if method == "withdrawal-outcome" {
			action = withdrawalOutcomeAction
		}
		ctx := asChat(a, a.session(x.owner))
		inv, _ := applet.InvocationFromContext(ctx)
		x.mu.Lock()
		x.requests = append(x.requests, map[string]any{"authenticatedContext": inv, "input": json.RawMessage(raw), "action": action})
		x.mu.Unlock()
		return bundle.Handlers[action](ctx, string(raw))
	case "raw":
		return x.typed.snapshot(), nil
	case "observations":
		return x.typed.observations(), nil
	case "close":
		a.shutdown(context.Background())
		return true, nil
	default:
		return nil, fmt.Errorf("unsupported receiving operation %q", method)
	}
}

func TestRequestSettlementAcceptanceServer(t *testing.T) {
	ready := os.Getenv("QUALITY_REQUEST_SETTLEMENT_READY")
	if ready == "" {
		t.Skip("requires the isolated receiving driver")
	}
	if dir := os.Getenv("QUALITY_REQUEST_SETTLEMENT_ARTIFACT"); dir != "" {
		t.Setenv("QUALITY_TYPED_ARTIFACT", dir)
	}
	x := newRequestSettlementFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan struct{})
	var once sync.Once
	var serial sync.Mutex
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Method string          `json:"method"`
			Input  json.RawMessage `json:"input"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&in); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		serial.Lock()
		defer serial.Unlock()
		result, err := x.call(in.Method, in.Input)
		response := map[string]any{"result": result}
		if err != nil {
			response["error"] = err.Error()
		}
		log, logErr := os.OpenFile(filepath.Join(x.artifact, "request-settlement-raw.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if logErr == nil {
			logErr = json.NewEncoder(log).Encode(map[string]any{"request": in, "response": response})
			logErr = errors.Join(logErr, log.Close())
		}
		if logErr != nil {
			t.Error(logErr)
			http.Error(w, logErr.Error(), 500)
			return
		}
		_ = json.NewEncoder(w).Encode(response)
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
	case <-time.After(5 * time.Minute):
		t.Error("receiving driver did not close")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Error(err)
	}
}

func TestRequestSettlementReceivingSeedsActualSources(t *testing.T) {
	for _, source := range []string{"request", "attention-report", "partial-request", "task-single", "task-text", "task-multiple"} {
		t.Run(source, func(t *testing.T) {
			x := newRequestSettlementFixture(t)
			seed, err := x.seed(source)
			if err != nil {
				t.Fatal(err)
			}
			before := seed.(RequestDetails)
			if before.Status != "open" {
				t.Fatal("the real source was not open", before)
			}
			r, err := x.typed.f.a.DismissRequest(requestsettlement.Input{ID: before.ID, Revision: before.Revision, OperationID: "positive-dismiss"})
			if err != nil || r.State != "committed" || r.Outcome != "dismissed" {
				t.Fatal(r, err)
			}
			after, err := x.typed.f.a.RequestState(before.ID)
			if err != nil || after.Status != "dismissed" {
				t.Fatal(after, err)
			}
			if strings.HasPrefix(source, "task-") {
				entry := x.typed.entry(before.ID)
				if err := x.typed.wait(entry); err != nil {
					t.Fatal(err)
				}
				raw, err := x.typed.receipt(entry)
				if err != nil || raw.Disposition != "dismissed" || len(raw.Result) != 0 {
					t.Fatal("actual waiter did not settle without an answer", raw, err)
				}
			}
			repeat, err := x.typed.f.a.DismissRequest(requestsettlement.Input{ID: before.ID, Revision: before.Revision, OperationID: "positive-dismiss"})
			if err != nil || !reflect.DeepEqual(repeat, r) {
				t.Fatal("the original dismissal receipt changed", repeat, r, err)
			}
			lookup, err := x.typed.f.a.RequestOperation(before.ID, "positive-dismiss")
			if err != nil || !reflect.DeepEqual(lookup, r) {
				t.Fatal("passive lookup lost the dismissal receipt", lookup, r, err)
			}
			x.requireQuiet()
		})
	}
}

func TestRequestSettlementReceivingWithdrawsAsActualOwner(t *testing.T) {
	for _, source := range []string{"request", "attention-report", "partial-request", "task-single", "task-text", "task-multiple"} {
		t.Run(source, func(t *testing.T) {
			x := newRequestSettlementFixture(t)
			seed, err := x.seed(source)
			if err != nil {
				t.Fatal(err)
			}
			before := seed.(RequestDetails)
			in := requestsettlement.Input{ID: before.ID, Revision: before.Revision, OperationID: "positive-withdraw"}
			raw, _ := json.Marshal(in)
			result, err := x.call("withdraw", raw)
			if err != nil {
				t.Fatal(err)
			}
			var receipt requestsettlement.Receipt
			if err := json.Unmarshal([]byte(result.(applet.Result).OutputJSON), &receipt); err != nil || receipt.State != "committed" || receipt.Outcome != "withdrawn" {
				t.Fatal("actual authenticated owner did not withdraw", result, err)
			}
			after, err := x.typed.f.a.RequestState(before.ID)
			if err != nil || after.Status != "withdrawn" {
				t.Fatal(after, err)
			}
			if strings.HasPrefix(source, "task-") {
				entry := x.typed.entry(before.ID)
				if err := x.typed.wait(entry); err != nil {
					t.Fatal(err)
				}
				raw, err := x.typed.receipt(entry)
				if err != nil || raw.Disposition != "withdrawn" || len(raw.Result) != 0 {
					t.Fatal("withdrawal delivered an answer", raw, err)
				}
			}
			lookup, err := x.call("withdrawal-outcome", raw)
			if err != nil || lookup.(applet.Result).OutputJSON != result.(applet.Result).OutputJSON {
				t.Fatal("passive owner lookup lost the original receipt", lookup, err)
			}
			repeat, err := x.call("withdraw", raw)
			if err != nil || repeat.(applet.Result).OutputJSON != result.(applet.Result).OutputJSON {
				t.Fatal("the owner withdrawal replayed or changed its receipt", repeat, err)
			}
			x.requireQuiet()
		})
	}
}

func (x *requestSettlementFixture) requireQuiet() {
	x.t.Helper()
	if requests := x.typed.providerRequests(); len(requests) != 0 {
		x.t.Fatal("settlement contacted an agent program", requests)
	}
	if x.typed.actionCount() != 0 || x.typed.f.docPeer.writes.Load() != 0 || x.typed.f.protectedSnapshot() != x.typed.protected {
		x.t.Fatal("settlement changed protected content or dispatched an unrelated action", x.typed.observations())
	}
}
