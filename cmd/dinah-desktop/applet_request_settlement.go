package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"strings"

	"studio/internal/applet"
	"studio/internal/bus"
	"studio/internal/layout"
	"studio/internal/requestsettlement"
	"studio/internal/taskhost"
)

//go:embed applet_request_settlement.go notification_settlement.go notification_dismissal.go notification_projection.go notifications.go request_completions.go request_operations.go task_questions.go task_host.go decisions.go session.go team_views.go bus_chats.go
var requestAppletSources embed.FS

const withdrawRequestAction = "dinah.requests/withdraw@1"
const withdrawalOutcomeAction = "dinah.requests/withdrawal-outcome@1"

func (a *App) requestSettlementBundle() (applet.Bundle, error) {
	bundle := applet.Bundle{Manifest: applet.Manifest{ID: "dinah.requests", Version: "1.0.0", APIVersion: 1, Title: "Requests", Description: "Withdraw the invoking chat's own request without answering it."}, Handlers: map[string]applet.Handler{}}
	bundle.Manifest.Actions = append(bundle.Manifest.Actions, applet.Action{
		Ref: withdrawRequestAction, Title: "Withdraw Request", Description: "Withdraw your own open request to the person without answering it. Ownership comes from the invoking chat.", Replay: "once", Capabilities: []string{"request.withdraw"},
		InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["operationId","id","revision"],"properties":{"operationId":{"type":"string","minLength":1,"maxLength":128},"id":{"type":"string","minLength":1,"maxLength":512},"revision":{"type":"integer","minimum":1}}}`),
		OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["operationId","id","revision","state"],"properties":{"operationId":{"type":"string"},"id":{"type":"string"},"revision":{"type":"integer"},"state":{"type":"string","enum":["committed","refused"]},"outcome":{"type":"string"},"settledAt":{"type":"string"},"code":{"type":"string"},"text":{"type":"string"}}}`),
	})
	bundle.Handlers[withdrawRequestAction] = func(ctx context.Context, raw string) (applet.Result, error) {
		var in requestsettlement.Input
		if err := json.Unmarshal([]byte(raw), &in); err != nil {
			return builtinJSON(requestsettlement.Refuse(in, "invalid_request", "The withdrawal input is invalid."))
		}
		r, err := a.withdrawRequest(ctx, in)
		if err != nil {
			return applet.Result{}, err
		}
		return builtinJSON(r)
	}
	read := bundle.Manifest.Actions[0]
	read.Ref, read.Title, read.Description = withdrawalOutcomeAction, "Read Withdrawal Outcome", "Read your original withdrawal receipt without settling or replaying a request. An unknown result does not authorize a retry."
	read.Replay, read.Capabilities = "pure", []string{"request.withdraw"}
	read.OutputSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"required":["operationId","id","revision","state"],"properties":{"operationId":{"type":"string"},"id":{"type":"string"},"revision":{"type":"integer"},"state":{"type":"string","enum":["committed","refused","unknown"]},"outcome":{"type":"string"},"settledAt":{"type":"string"},"code":{"type":"string"},"text":{"type":"string"}}}`)
	bundle.Manifest.Actions = append(bundle.Manifest.Actions, read)
	bundle.Handlers[withdrawalOutcomeAction] = func(ctx context.Context, raw string) (applet.Result, error) {
		var in requestsettlement.Input
		if err := json.Unmarshal([]byte(raw), &in); err != nil {
			return builtinJSON(requestsettlement.Refuse(in, "invalid_request", "The withdrawal lookup input is invalid."))
		}
		r, err := a.withdrawalOutcome(ctx, in)
		if err != nil {
			return applet.Result{}, err
		}
		return builtinJSON(r)
	}
	if bundle.Sources == nil {
		bundle.Sources = map[string][]byte{}
	}
	entries, err := requestAppletSources.ReadDir(".")
	if err != nil {
		return applet.Bundle{}, err
	}
	for _, entry := range entries {
		data, err := requestAppletSources.ReadFile(entry.Name())
		if err != nil {
			return applet.Bundle{}, err
		}
		bundle.Sources["host/"+entry.Name()] = data
	}
	dependencies := []struct {
		prefix string
		read   func() (map[string][]byte, error)
	}{{"bus/", bus.SettlementSources}, {"taskhost/", taskhost.SettlementSources}, {"requestsettlement/", requestsettlement.SettlementSources}}
	for _, dependency := range dependencies {
		sources, err := dependency.read()
		if err != nil {
			return applet.Bundle{}, err
		}
		for name, data := range sources {
			bundle.Sources[dependency.prefix+name] = data
		}
	}
	return bundle, nil
}

func (a *App) requestChatIdentity(ctx context.Context) (applet.Invocation, string, string, error) {
	inv, ok := applet.InvocationFromContext(ctx)
	if !ok || inv.OwnerKind != "chat" || inv.OwnerID == "" {
		return inv, "", "", errors.New("Only the asking chat can use its withdrawal receipt.")
	}
	workspace, err := builtinWorkspace(ctx)
	if err != nil {
		return inv, "", "", errors.New("The invoking chat has no authenticated workspace.")
	}
	caller := a.session(inv.OwnerID)
	if caller == nil || !layout.SamePath(caller.Cwd(), workspace) {
		return inv, "", "", errors.New("The invoking chat does not belong to this workspace.")
	}
	canonical := a.knownBusID(inv.OwnerID)
	if canonical == "" {
		return inv, "", "", errors.New("The asking chat has no known identity.")
	}
	return inv, workspace, canonical, nil
}

// withdrawalOutcome reads only the authenticated actor's original receipt.
// Source receipts remain readable after admission authority has been revoked.
func (a *App) withdrawalOutcome(ctx context.Context, in requestsettlement.Input) (requestsettlement.Receipt, error) {
	inv, workspace, canonical, err := a.requestChatIdentity(ctx)
	if err != nil {
		return requestsettlement.Refuse(in, "not_authorized", err.Error()), nil
	}
	out := requestsettlement.Receipt{ID: in.ID, OperationID: in.OperationID, Revision: in.Revision, State: "unknown"}
	switch {
	case strings.HasPrefix(in.ID, "notification:"):
		source, err := a.NotificationRequest(in.ID)
		if err != nil {
			return requestsettlement.Receipt{}, err
		}
		if source.Report.RequestOwner != canonical || source.Report.SessionID != inv.OwnerID || source.Report.Workspace != "" && !layout.SamePath(source.Report.Workspace, workspace) {
			return requestsettlement.Refuse(in, "not_authorized", "This report belongs to another asking chat or workspace."), nil
		}
		out, err = a.reportOperation(canonical, "Withdraw", in)
	case strings.HasPrefix(in.ID, "task-q-"):
		var source taskhost.QuestionRecord
		source, err = taskhost.ReadQuestionRecord(configDir(), in.ID)
		if err == nil {
			binding, bindErr := a.taskWorkspace(ctx, source.Scope)
			if bindErr != nil {
				return requestsettlement.Receipt{}, bindErr
			}
			if source.Recipient != canonical || source.Scope.OwnerChat != inv.OwnerID || !layout.SamePath(binding.Path, workspace) {
				return requestsettlement.Refuse(in, "not_authorized", "This question belongs to another asking chat."), nil
			}
			if previous, ok := source.Receipts[requestsettlement.Key(requestsettlement.ChatActor(canonical), "withdrawn", in.OperationID)]; ok {
				out = previous
			}
		}
	default:
		err = a.busHost().With(func(b *bus.Bus) error {
			source, err := b.DecisionByID(ctx, in.ID)
			if err != nil {
				return err
			}
			if source.Decision.AskerSession != canonical {
				out = requestsettlement.Refuse(in, "not_authorized", "This request belongs to another asking chat.")
				return nil
			}
			out, err = b.RequestOperation(ctx, canonical, "Withdraw", in.OperationID)
			return err
		})
	}
	if err != nil {
		return requestsettlement.Receipt{}, err
	}
	if out.State == "committed" && (out.ID != in.ID || out.Revision != in.Revision+1) {
		return requestsettlement.Refuse(in, "operation_changed", "This operation ID describes different input."), nil
	}
	if out.State == "unknown" {
		out.ID, out.Revision = in.ID, in.Revision
	}
	return out, nil
}

// withdrawRequest receives only host-authenticated context. It is not a window
// method and is never offered to a service as an answering or dismissal port.
func (a *App) withdrawRequest(ctx context.Context, in requestsettlement.Input) (requestsettlement.Receipt, error) {
	refuse := func(text string) (requestsettlement.Receipt, error) {
		return requestsettlement.Refuse(in, "not_authorized", text), nil
	}
	inv, workspace, canonical, err := a.requestChatIdentity(ctx)
	if err != nil {
		return refuse(err.Error())
	}
	previous, err := a.withdrawalOutcome(ctx, in)
	if err != nil || previous.State != "unknown" {
		return previous, err
	}
	if a.removedChat(inv.OwnerID) {
		return refuse("The asking chat is unavailable in this workspace.")
	}
	var out requestsettlement.Receipt
	err = a.busHost().With(func(b *bus.Bus) error {
		available, err := b.RequestOwnerAvailable(ctx, canonical)
		if err != nil {
			return err
		}
		if !available {
			out = requestsettlement.Refuse(in, "not_authorized", "The original asking chat is unavailable.")
			return nil
		}
		me, err := b.Session(ctx, canonical)
		if err != nil {
			return err
		}
		if !layout.SamePath(me.WorkspacePath, workspace) {
			out = requestsettlement.Refuse(in, "not_authorized", "The invoking workspace does not match the asking chat.")
			return nil
		}
		switch {
		case strings.HasPrefix(in.ID, "notification:"):
			source, err := a.NotificationRequest(in.ID)
			if err != nil {
				return err
			}
			if source.Report.SessionID != inv.OwnerID || source.Report.RequestOwner != canonical || source.Report.Workspace != "" && !layout.SamePath(source.Report.Workspace, workspace) {
				out = requestsettlement.Refuse(in, "not_authorized", "This report belongs to another asking chat or workspace.")
				return nil
			}
			out, err = a.settleNotification(in, canonical, "Withdraw")
			return err
		case strings.HasPrefix(in.ID, "task-q-"):
			a.taskServices.mu.Lock()
			h := a.taskServices.host
			a.taskServices.mu.Unlock()
			if h == nil {
				out = requestsettlement.Refuse(in, "request_unavailable", "The task host is unavailable.")
				return nil
			}
			source, err := h.QuestionRecord(in.ID)
			if err != nil {
				return err
			}
			binding, err := a.taskWorkspace(ctx, source.Scope)
			if err != nil {
				return err
			}
			if source.Scope.OwnerChat != inv.OwnerID || source.Recipient != canonical || !layout.SamePath(binding.Path, workspace) {
				out = requestsettlement.Refuse(in, "not_authorized", "This question belongs to another asking chat or workspace.")
				return nil
			}
			previous, err := h.QuestionOperationByChat(ctx, source.Scope, in.ID, in.OperationID, canonical)
			if err != nil {
				return err
			}
			if previous.State == "committed" {
				if previous.Revision != in.Revision+1 {
					out = requestsettlement.Refuse(in, "operation_changed", "This operation ID describes different input.")
				} else {
					out = previous
				}
				return nil
			}
			if source.Status == "open" && source.Revision != in.Revision {
				out = requestsettlement.Refuse(in, "revision_changed", "This question changed.")
				return nil
			}
			out, err = h.WithdrawQuestionByChat(ctx, source.Scope, in.ID, in.OperationID, canonical)
			return err
		default:
			out, err = b.WithdrawRequest(ctx, me, in)
			return err
		}
	})
	if err == nil && out.State == "committed" {
		if dispatchErr := a.flushRequestCompletions(); dispatchErr != nil {
			logf("core", "error", "dispatching withdrawal completion: %v", dispatchErr)
		}
	}
	return out, err
}
