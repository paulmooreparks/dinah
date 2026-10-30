package sessionhost

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"studio/internal/applet/broker"
	"studio/internal/bus"
)

//go:embed bus_results.go
var BusResultsSource []byte

const ChatAskAction = "dinah.chat/ask@2"

func ChatResultKey(workspace, id string) string {
	return "chat-result:" + bus.WorkspaceKey(workspace) + ":" + id
}

// reconcileChatResults moves saved completions to the bus. The index avoids
// rereading unrelated records; reported avoids rereading results this process
// already queued. The bus key also deduplicates across process restarts.
func (a *BusChats) ReconcileResults(ctx context.Context, b *bus.Bus, reported map[string]bool) {
	host, err := a.host.ResultBroker()
	if err != nil {
		a.host.Log("error", "reading completed chat requests: %v", err)
		return
	}
	results, err := host.Attention(func(s broker.Summary) bool {
		return s.Action == ChatAskAction && s.OwnerKind == "chat" && broker.TerminalStatus(s.Status) && !reported[ChatResultKey(s.Workspace, s.ID)]
	})
	if err != nil {
		a.host.Log("error", "recovering chat request results: %v", err)
		return
	}
	for _, v := range results {
		if err := a.EnqueueResult(ctx, b, v); err != nil {
			a.host.Log("error", "queueing chat request %s's result: %v", v.ID, err)
			continue
		}
		reported[ChatResultKey(v.Workspace, v.ID)] = true
	}
}

// enqueueChatResult addresses only the caller's saved bus identity. A closed
// caller keeps its result until it reopens; another chat in that workspace
// must never consume it.
func (a *BusChats) EnqueueResult(ctx context.Context, b *bus.Bus, v broker.Invocation) error {
	if v.Action != ChatAskAction || v.OwnerKind != "chat" || !broker.TerminalStatus(v.Status) {
		return errors.New("only a completed asynchronous chat request has a result to deliver")
	}
	id := a.host.SavedBusIdentity(v.OwnerID)
	if id == "" {
		return errors.New("the caller has no saved bus identity; its result remains saved for retry")
	}
	report := bus.HostReport{InvocationID: v.ID, Request: json.RawMessage(v.InputJSON), Status: v.Status, Output: json.RawMessage(v.OutputJSON), Error: v.Error}
	// The report's identity is its JSON (bus.EnqueueResult), so a result
	// saved before results had text must make the report it made then.
	// Its text and the answering chat's title are therefore taken only
	// from a result that has text; one saved earlier has neither, and is
	// shown as its output.
	if v.Text != "" {
		var out struct {
			Title string `json:"title"`
		}
		_ = json.Unmarshal([]byte(v.OutputJSON), &out)
		report.Text, report.From = v.Text, out.Title
	}
	text, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("reading the saved chat result: %w", err)
	}
	_, err = b.EnqueueResult(ctx, ChatResultKey(v.Workspace, v.ID), id, string(text))
	return err
}
