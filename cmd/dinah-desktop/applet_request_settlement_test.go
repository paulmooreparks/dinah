package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"studio/internal/applet"
	"studio/internal/applet/broker"
	"studio/internal/applet/builtin"
	"studio/internal/bus"
	"studio/internal/requestsettlement"
)

func TestWithdrawalProviderPreservesPendingHumanRequestContract(t *testing.T) {
	a, _ := providerTestApp(t)
	legacyBundle, err := builtin.Notifications()
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := applet.New(legacyBundle)
	if err != nil {
		t.Fatal(err)
	}
	bundles, err := a.builtinAppletBundles()
	if err != nil {
		t.Fatal(err)
	}
	current, err := applet.New(bundles...)
	if err != nil {
		t.Fatal(err)
	}
	const action = "dinah.notifications/request@1"
	digest, err := legacy.Digest(action)
	if err != nil {
		t.Fatal(err)
	}
	got, err := current.Digest(action)
	if err != nil || got != digest {
		t.Fatal("the new withdrawal provider changed the pending human request contract", got, digest, err)
	}
	state, workspace := t.TempDir(), t.TempDir()
	waiting := make(chan broker.Invocation, 1)
	oldHost := &broker.Broker{Registry: legacy, State: state, Emit: func(event string, data any) {
		if event == "applet:invocation" {
			if v, ok := data.(broker.Invocation); ok && v.Status == "waiting" {
				waiting <- v
			}
		}
	}}
	t.Cleanup(func() { _ = oldHost.Close() })
	_, err = oldHost.StartID(workspace, "chat", "invented-owner", "kept-human-request", action, `{"title":"Invented request","body":"Keep this request answerable."}`, digest, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	var pending broker.Invocation
	select {
	case pending = <-waiting:
	case <-time.After(testWait):
		t.Fatal("the original human request did not reach waiting")
	}
	// Get takes the original broker's record lock after the worker publishes
	// waiting, so its durable write and execution hold are released first.
	if _, err := oldHost.Get(workspace, "chat", "invented-owner", pending.ID); err != nil {
		t.Fatal(err)
	}
	newHost := &broker.Broker{Registry: current, State: state}
	t.Cleanup(func() { _ = newHost.Close() })
	answered, err := newHost.Answer(workspace, pending.ID, pending.Version, `{"answer":"The person answered."}`)
	if err != nil || answered.Status != "done" || answered.Digest != digest {
		t.Fatal("the pending human request could not be answered after adding withdrawal", answered, err)
	}
}

func TestRequestWithdrawalUsesTheAuthenticatedChatAndKeptRecipient(t *testing.T) {
	a, native := providerTestApp(t)
	native.newIDs = []string{"codex:owner", "codex:other"}
	workspace := t.TempDir()
	owner, err := a.NewSessionAt(workspace)
	if err != nil {
		t.Fatal(err)
	}
	other, err := a.NewSessionAt(workspace)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := a.requestSettlementBundle()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applet.New(bundle); err != nil {
		t.Fatal("the actual bundled withdrawal schema is invalid", err)
	}
	saved, err := a.SaveNotification(Notification{ID: "owned-report", Kind: "turn-failure", Severity: "error", Title: "Invented report", SessionID: owner.ID, Workspace: workspace})
	if err != nil {
		t.Fatal(err)
	}
	in := requestsettlement.Input{ID: notificationRequestID(saved), Revision: 1, OperationID: "withdraw-once"}
	raw, _ := json.Marshal(in)
	path := filepath.Join(configDir(), "notifications.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeNative := native.calls.Snapshot()
	contexts := []context.Context{
		context.Background(),
		asChat(a, a.session(other.ID)),
		applet.WithInvocation(context.Background(), applet.Invocation{Workspace: workspace, OwnerKind: "applet", OwnerID: owner.ID}),
		applet.WithInvocation(context.Background(), applet.Invocation{Workspace: t.TempDir(), OwnerKind: "chat", OwnerID: owner.ID}),
		applet.WithInvocation(context.Background(), applet.Invocation{Workspace: workspace, OwnerKind: "chat", OwnerID: "unresolvable"}),
	}
	for _, ctx := range contexts {
		for _, action := range []string{withdrawRequestAction, withdrawalOutcomeAction} {
			result, err := bundle.Handlers[action](ctx, string(raw))
			var r requestsettlement.Receipt
			if err != nil || json.Unmarshal([]byte(result.OutputJSON), &r) != nil || r.State != "refused" {
				t.Fatal("untrusted context used report withdrawal authority", action, result, err)
			}
		}
		after, err := os.ReadFile(path)
		if err != nil || !reflect.DeepEqual(after, before) {
			t.Fatal("refused withdrawal changed persisted report", err)
		}
	}
	unknown, err := bundle.Handlers[withdrawalOutcomeAction](asChat(a, a.session(owner.ID)), string(raw))
	var notCommitted requestsettlement.Receipt
	if err != nil || json.Unmarshal([]byte(unknown.OutputJSON), &notCommitted) != nil || notCommitted.State != "unknown" {
		t.Fatal("an uncommitted lookup claimed a result", unknown, err)
	}
	if after, err := os.ReadFile(path); err != nil || !reflect.DeepEqual(after, before) {
		t.Fatal("passive unknown lookup changed the source", err)
	}
	result, err := bundle.Handlers[withdrawRequestAction](asChat(a, a.session(owner.ID)), string(raw))
	var receipt requestsettlement.Receipt
	if err != nil || json.Unmarshal([]byte(result.OutputJSON), &receipt) != nil || receipt.State != "committed" || receipt.Outcome != "withdrawn" {
		t.Fatal(result, err)
	}
	source, err := a.NotificationRequest(in.ID)
	if err != nil || source.Status != "withdrawn" || source.Completion.MessageID == 0 || source.Completion.Recipient != a.knownBusID(owner.ID) {
		t.Fatal("completion did not retain and reach the original owner", source, err)
	}
	var notice bus.Message
	if err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		notice, err = b.Message(context.Background(), source.Completion.MessageID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var report bus.HostReport
	if err := json.Unmarshal([]byte(notice.Text), &report); err != nil || notice.Kind != bus.ResultKind {
		t.Fatal(notice, err)
	}
	var output map[string]string
	if err := json.Unmarshal(report.Output, &output); err != nil || output["outcome"] != "withdrawn" || len(output) != 2 {
		t.Fatal("withdrawal notice contained an answer", output, err)
	}
	repeat, err := bundle.Handlers[withdrawRequestAction](asChat(a, a.session(owner.ID)), string(raw))
	if err != nil || repeat.OutputJSON != result.OutputJSON || !reflect.DeepEqual(native.calls.Snapshot(), beforeNative) {
		t.Fatal("withdrawal repeated an effect or changed its receipt", repeat, err)
	}
	lookup, err := bundle.Handlers[withdrawalOutcomeAction](asChat(a, a.session(owner.ID)), string(raw))
	if err != nil || lookup.OutputJSON != result.OutputJSON {
		t.Fatal("passive lookup changed the original receipt", lookup, err)
	}
	changed := in
	changed.Revision++
	changedJSON, _ := json.Marshal(changed)
	lookup, err = bundle.Handlers[withdrawalOutcomeAction](asChat(a, a.session(owner.ID)), string(changedJSON))
	var refused requestsettlement.Receipt
	if err != nil || json.Unmarshal([]byte(lookup.OutputJSON), &refused) != nil || refused.State != "refused" || refused.Code != "operation_changed" {
		t.Fatal("lookup accepted a changed original input", lookup, err)
	}
}
