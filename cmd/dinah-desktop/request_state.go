package main

import (
	"context"
	"errors"
	"strings"

	"studio/internal/bus"
	"studio/internal/requestsettlement"
	"studio/internal/taskhost"
)

type RequestDetails struct {
	ID       string `json:"id"`
	Revision int    `json:"revision"`
	Kind     string `json:"kind"`
	requestsettlement.Disposition
	Commands   []requestsettlement.Command `json:"commands"`
	Request    any                         `json:"request"`
	Wording    string                      `json:"wording,omitempty"`
	Projection *Notification               `json:"projection,omitempty"`
}

func personRequestCommands(id, status, answerMethod string) []requestsettlement.Command {
	names := []string{}
	if answerMethod != "" {
		names = append(names, "Answer")
	}
	out := requestsettlement.Commands(id, status, true, false, names)
	for i := range out {
		if out[i].Enabled {
			switch out[i].Name {
			case "Dismiss":
				out[i].Invocation = "DismissRequest"
			case "Answer":
				out[i].Invocation = answerMethod
			}
		}
	}
	return out
}

func disableRequestCommands(commands []requestsettlement.Command, reason string) {
	for i := range commands {
		commands[i].Enabled = false
		commands[i].Reason = reason
		commands[i].Invocation = ""
	}
}

// RequestState reads authoritative disposition and command availability for
// the person. The original source remains separate from its card projection.
func (a *App) RequestState(id string) (RequestDetails, error) {
	if strings.HasPrefix(id, "permission:") {
		key := strings.TrimPrefix(id, "permission:")
		for _, pending := range a.PendingPermissions() {
			if pending["key"] != key {
				continue
			}
			out := RequestDetails{ID: id, Revision: 1, Kind: "provider-permission", Disposition: requestsettlement.Disposition{Status: "open"}, Request: pending, Commands: personRequestCommands(id, "open", "Answer")}
			for i := range out.Commands {
				if out.Commands[i].Name != "Answer" {
					out.Commands[i].Enabled = false
					out.Commands[i].Invocation = ""
					out.Commands[i].Reason = "Dismiss and Withdraw are unavailable until this provider has an approved, documented cancellation adapter."
				}
			}
			return out, nil
		}
		return RequestDetails{}, errors.New("This provider permission is no longer pending.")
	}
	if strings.HasPrefix(id, "applet:") {
		key := strings.TrimPrefix(id, "applet:")
		pending, err := a.AppletRequests()
		if err != nil {
			return RequestDetails{}, err
		}
		matches := []AppletInvocation{}
		for _, request := range pending {
			if request.ID == key {
				matches = append(matches, request)
			}
		}
		if len(matches) != 1 {
			return RequestDetails{}, errors.New("The applet request is unavailable or ambiguous.")
		}
		out := RequestDetails{ID: id, Revision: 1, Kind: "applet-request", Disposition: requestsettlement.Disposition{Status: "open"}, Request: matches[0], Commands: personRequestCommands(id, "open", "AnswerAppletRequest")}
		for i := range out.Commands {
			if out.Commands[i].Name != "Answer" {
				out.Commands[i].Enabled = false
				out.Commands[i].Invocation = ""
				out.Commands[i].Reason = "Dismiss and Withdraw are unavailable until the applet request settlement adapter is approved."
			}
		}
		return out, nil
	}
	if strings.HasPrefix(id, "notification:") {
		source, err := a.NotificationRequest(id)
		if err != nil {
			return RequestDetails{}, err
		}
		item := source.Report
		if strings.HasPrefix(item.PermissionKey, "task-q-") {
			out, err := a.RequestState(item.PermissionKey)
			out.Projection = &item
			return out, err
		}
		if item.Kind == "mention" && item.Message != 0 {
			var original bus.Decision
			err := a.busHost().With(func(b *bus.Bus) error {
				var err error
				original, err = b.DecisionForMessage(context.Background(), item.Message)
				return err
			})
			if err != nil {
				return RequestDetails{}, err
			}
			out, err := a.RequestState(original.ID)
			out.Projection = &item
			return out, err
		}
		out := RequestDetails{ID: id, Revision: source.Revision, Kind: "attention-report", Disposition: source.Disposition, Request: item, Wording: source.Completion.Wording, Commands: personRequestCommands(id, source.Status, "")}
		if item.PermissionKey != "" {
			disableRequestCommands(out.Commands, "Dismiss is unavailable until this provider permission request has an approved, documented cancellation adapter.")
		} else if item.RequestOwner == "" && source.Status == "open" {
			for i := range out.Commands {
				if out.Commands[i].Name == "Withdraw" {
					out.Commands[i].Reason = "This report has no asking chat."
				}
			}
		}
		return out, nil
	}
	if strings.HasPrefix(id, "task-q-") {
		asking := false
		for _, pending := range a.PendingTaskPermissions() {
			asking = asking || pending.Key == id
		}
		source, err := taskhost.ReadQuestionRecord(configDir(), id)
		if err != nil {
			return RequestDetails{}, err
		}
		answerMethod := "Answer"
		if source.Question.AnswerType != "" {
			answerMethod = "AnswerTaskQuestion"
		}
		out := RequestDetails{ID: id, Revision: source.Revision, Kind: "task-question", Disposition: source.Disposition, Request: source.Question, Commands: personRequestCommands(id, source.Status, answerMethod)}
		if source.Completion != nil {
			out.Wording = source.Completion.Wording
		}
		if source.Status == "open" {
			a.taskServices.mu.Lock()
			h := a.taskServices.host
			a.taskServices.mu.Unlock()
			if h == nil || h.Snapshot().AdmissionClosed || !asking {
				disableRequestCommands(out.Commands, "The task host is unavailable or is shutting down.")
			} else if err := a.checkTaskIdentity(context.Background(), source.Scope); err != nil {
				disableRequestCommands(out.Commands, "The task's original authority is no longer current.")
			}
		}
		return out, nil
	}
	if !strings.HasPrefix(id, "request:") && !strings.HasPrefix(id, "mention:") {
		return RequestDetails{}, errors.New("This request source is unavailable.")
	}
	var detail bus.DecisionDetail
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		detail, err = b.DecisionByID(context.Background(), id)
		return err
	})
	if err != nil {
		return RequestDetails{}, err
	}
	d := detail.Decision
	out := RequestDetails{ID: d.ID, Revision: d.Revision, Kind: d.Kind, Disposition: d.Disposition, Request: d, Commands: personRequestCommands(id, d.Status, "AnswerDecision")}
	if d.Status == "dismissed" {
		out.Wording = "The person dismissed this request without answering."
	}
	if d.Status == "withdrawn" {
		out.Wording = "The asking chat withdrew this request without answering."
	}
	if d.Status == "answered" {
		out.Wording = "The person answered this request."
	}
	if d.Status == "open" {
		var current bus.Decision
		err = a.busHost().With(func(b *bus.Bus) error {
			var err error
			current, err = b.DecisionForMessage(context.Background(), d.MessageID)
			return err
		})
		if err != nil {
			return RequestDetails{}, err
		}
		if current.DismissalReason != "" {
			for i := range out.Commands {
				if out.Commands[i].Name == "Answer" {
					out.Commands[i].Enabled = false
					out.Commands[i].Reason = current.DismissalReason
					out.Commands[i].Invocation = ""
				}
			}
		}
	}
	return out, nil
}
