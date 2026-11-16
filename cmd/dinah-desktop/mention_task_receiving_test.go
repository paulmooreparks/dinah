//go:build org_chart_acceptance && startup_page_acceptance && quality_task_host && quality_typed_questions

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
	"testing"
	"time"

	"studio/internal/services"
	"studio/internal/taskhost"
)

const mentionTaskService = "mention-receiving-tasks"

var mentionTaskStores sync.Map
var mentionTaskSequence atomic.Int64

func init() {
	services.RegisterWindowNamed(mentionTaskService, "Invented receiving tasks", nil, func(client taskhost.Client) (services.Service, services.Participant, error) {
		dir := filepath.Join(configDir(), "mention-receiving-consumer")
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, nil, err
		}
		path := filepath.Join(dir, "generation")
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err = os.WriteFile(path, []byte("generation-1"), 0600); err != nil {
				return nil, nil, err
			}
		}
		store := &frozenTaskStore{path: dir, client: client}
		mentionTaskStores.Store(configDir(), store)
		return frozenPage{}, store, nil
	})
}

// OrgChartHostFixtureMentionTask reuses the frozen consumer's actual durable
// generation and hold, and the typed fixture's Review/Begin/Start sequence.
// The setup approves only its review consent through the real window Answer.
func (a *App) OrgChartHostFixtureMentionTask(kind string) (taskhost.Question, error) {
	f, err := orgChartFixture(a)
	if err != nil {
		return taskhost.Question{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.a.startTaskServices(); err != nil {
		return taskhost.Question{}, err
	}
	value, ok := mentionTaskStores.Load(configDir())
	if !ok {
		return taskhost.Question{}, errors.New("receiving task consumer is absent")
	}
	store := value.(*frozenTaskStore)
	var raw json.RawMessage
	switch kind {
	case "legacy":
		raw = json.RawMessage(`{"title":"Invented legacy question","choices":["Beta","Alpha"]}`)
	case "text":
		raw = json.RawMessage(`{"title":"Invented text question","answer":{"type":"text","minLength":1,"maxLength":12}}`)
	case "multiple":
		raw = json.RawMessage(`{"title":"Invented multiple question","answer":{"type":"multiple","options":[{"optionId":"beta","name":"Beta"},{"optionId":"alpha","name":"Alpha"}],"minSelections":1,"maxSelections":2}}`)
	default:
		return taskhost.Question{}, errors.New("unknown receiving task question type")
	}
	spaces := f.manifest["workspaces"].([]WorkspaceRecord)
	task := fmt.Sprintf("mention-source-%d", mentionTaskSequence.Add(1))
	scope := taskhost.Scope{Service: mentionTaskService, Workspace: spaces[0].ID, Task: task, Generation: "generation-1"}
	op := taskhost.Operation{ID: "ask", Kind: "AskPerson", Input: raw}
	var consent atomic.Bool
	arrived := make(chan taskhost.Question, 1)
	prior := f.a.sink
	f.a.sink = func(name string, value any) {
		if prior != nil {
			prior(name, value)
		}
		if name == "task:permission" {
			q := value.(taskhost.Question)
			if consent.Load() {
				if err := f.a.Answer(q.Key, "allow_once"); err != nil {
					logf("core", "error", "receiving consent: %v", err)
				}
			} else if q.Task == task {
				select {
				case arrived <- q:
				default:
				}
			}
		}
	}
	defer func() { f.a.sink = prior }()
	review, err := store.client.Review(context.Background(), taskhost.Plan{Scope: scope, Permits: []taskhost.Permit{{Kind: op.Kind, Input: raw}}, Deadline: time.Now().Add(time.Hour)})
	if err != nil {
		return taskhost.Question{}, err
	}
	consent.Store(true)
	handle, err := store.client.Begin(context.Background(), review.ID)
	consent.Store(false)
	if err != nil {
		return taskhost.Question{}, err
	}
	if _, err = store.client.Start(context.Background(), handle, op); err != nil {
		return taskhost.Question{}, err
	}
	select {
	case q := <-arrived:
		return q, nil
	case <-time.After(5 * time.Second):
		return taskhost.Question{}, errors.New("reviewed task did not expose its actual pending question")
	}
}

func TestMentionReceivingActualTaskSources(t *testing.T) {
	orgChartForbidProviderStarts(t)
	a := newFileAccessFixtureApp(t)
	a.ctx = context.Background()
	newOrgChartHostFixture(t, a)
	for _, kind := range []string{"legacy", "text", "multiple"} {
		q, err := a.OrgChartHostFixtureMentionTask(kind)
		if err != nil || q.Key == "" {
			t.Fatal("reviewed task seed failed", kind, q, err)
		}
	}
	pending := a.PendingTaskPermissions()
	if len(pending) != 3 {
		t.Fatal("actual App did not retain all task sources", pending)
	}
}
