package main

import (
	"context"
	"encoding/json"
	"errors"

	"studio/internal/bus"
	"studio/internal/taskhost"
)

func dispatchRequestCompletion(ctx context.Context, b *bus.Bus, id, recipient, outcome, wording string) (int64, error) {
	if recipient == "" {
		return 0, nil
	}
	key := "request-settlement:" + id
	output, _ := json.Marshal(map[string]string{"id": id, "outcome": outcome})
	report, _ := json.Marshal(bus.HostReport{InvocationID: key, Request: json.RawMessage(`{}`), Status: "done", Output: output, Text: wording})
	message, err := b.FindResult(ctx, key, recipient, string(report))
	if err != nil || message != 0 {
		return message, err
	}
	available, err := b.RequestCompletionAvailable(ctx, recipient)
	if err != nil || !available {
		return 0, err
	}
	m, err := b.EnqueueResult(ctx, key, recipient, string(report))
	return m.ID, err
}

func (a *App) flushRequestCompletions() error {
	// An ownerless question has no chat notice. Its lifecycle must not create
	// a bus connection or a consumer merely to observe that absence.
	notificationStoreMu.Lock()
	store, err := readNotifications()
	notificationStoreMu.Unlock()
	if err != nil {
		return err
	}
	needed := false
	for _, source := range store.ReportSettlements {
		needed = needed || source.Completion.Recipient != "" && source.Completion.MessageID == 0
	}
	questions, err := taskhost.QuestionRecords(configDir())
	if err != nil {
		return err
	}
	for _, source := range questions {
		c := source.Completion
		needed = needed || c != nil && c.Recipient != "" && c.MessageID == 0 && c.Outcome != "answered"
	}
	if !needed {
		return nil
	}
	return a.busHost().With(func(b *bus.Bus) error { return a.reconcileRequestCompletions(context.Background(), b) })
}

func (a *App) reconcileRequestCompletions(ctx context.Context, b *bus.Bus) error {
	if err := a.repairRequestProjections(); err != nil {
		return err
	}
	if err := b.DispatchRequestCompletions(ctx); err != nil {
		return err
	}
	notificationStoreMu.Lock()
	store, err := readNotifications()
	notificationStoreMu.Unlock()
	if err != nil {
		return err
	}
	var failures []error
	for id, source := range store.ReportSettlements {
		c := source.Completion
		if c.Recipient == "" || c.MessageID != 0 {
			continue
		}
		message, err := dispatchRequestCompletion(ctx, b, id, c.Recipient, c.Outcome, c.Wording)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if message == 0 {
			continue
		}
		notificationStoreMu.Lock()
		current, err := readNotifications()
		if err == nil {
			item, ok := current.ReportSettlements[id]
			if !ok || item.Status != source.Status || item.Completion.Recipient != c.Recipient {
				err = errors.New("The report completion no longer matches its source.")
			} else if item.Completion.MessageID == 0 {
				item.Completion.MessageID = message
				current.ReportSettlements[id] = item
				err = writeNotifications(current)
			} else if item.Completion.MessageID != message {
				err = errors.New("The report already has another completion notice.")
			}
		}
		notificationStoreMu.Unlock()
		if err != nil {
			failures = append(failures, err)
		}
	}
	questions, err := taskhost.QuestionRecords(configDir())
	if err != nil {
		failures = append(failures, err)
	} else {
		for _, source := range questions {
			c := source.Completion
			if c == nil || c.Recipient == "" || c.MessageID != 0 || c.Outcome == "answered" {
				continue
			}
			message, err := dispatchRequestCompletion(ctx, b, source.Question.Key, c.Recipient, c.Outcome, c.Wording)
			if err == nil && message != 0 {
				a.taskServices.mu.Lock()
				h := a.taskServices.host
				a.taskServices.mu.Unlock()
				if h != nil {
					err = h.RecordQuestionNotice(source.Question.Key, message)
				}
			}
			if err != nil {
				failures = append(failures, err)
			}
		}
	}
	return errors.Join(failures...)
}
