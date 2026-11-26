package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/records"
	"studio/internal/taskhost"
)

// This fixture binds the actual task operation store and App allocation bridge.
// Consumer task-generation and invocation-grant acceptance are separate gates.
func TestTaskHostAllocationPersistsRealAppBoundaries(t *testing.T) {
	for _, mode := range []string{"success", "dispatch persistence failure", "identity persistence failure", "finish persistence failure"} {
		t.Run(mode, func(t *testing.T) {
			a, conn := providerTestApp(t)
			root := t.TempDir()
			a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "workspace-a", ExecutionPath: root}}
			a.taskQuestions.check = func(context.Context, taskhost.Scope) error { return nil }
			a.taskQuestions.describe = func(ctx context.Context, s taskhost.Scope) (taskhost.QuestionOwner, error) {
				return taskhost.QuestionOwner{Service: s.Service, ServiceName: "Allocation Fixture", Workspace: s.Workspace, WorkspaceName: "Invented Workspace", Task: s.Task, Generation: s.Generation, TaskTitle: "Allocate an invented chat", Target: taskhost.QuestionTarget{Reason: "This fixture has no task page."}}, nil
			}
			var questions atomic.Int32
			a.sink = func(name string, data any) {
				if name == "task:permission" {
					questions.Add(1)
					if err := a.Answer(data.(taskhost.Question).Key, "allow_once"); err != nil {
						t.Error(err)
					}
				}
			}
			terminal := make(chan taskhost.Receipt, 2)
			injected := false
			h, err := taskhost.New(taskhost.Options{State: configDir(), Resolve: a.taskWorkspace,
				Check: func(context.Context, taskhost.Scope) error { return nil },
				Admit: a.admitTaskOperation, TargetRevision: a.taskTargetRevision, Confirm: a.confirmTaskReview,
				Backend: taskBackend{a}, Track: func(taskhost.Scope, taskhost.Operation) (func(), error) { return func() {}, nil },
				Save: func(r taskhost.Receipt) error {
					fail := mode == "dispatch persistence failure" && r.Dispatched || mode == "identity persistence failure" && r.Identity != "" || mode == "finish persistence failure" && r.SetupFinished
					if fail && !injected {
						injected = true
						return errors.New("invented task-store failure")
					}
					return nil
				},
				Emit: func(r taskhost.Receipt) {
					if r.State != "prepared" && r.State != "in_progress" {
						terminal <- r
					}
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := h.Shutdown(context.Background()); err != nil && mode == "success" {
					t.Error(err)
				}
			})
			scope := taskhost.Scope{Service: "allocation-fixture", Workspace: "workspace-a", Task: "task-a", Generation: "1"}
			conn.onNew = func() {
				stored, err := h.Read(context.Background(), scope, "allocate")
				if err != nil || !stored.Dispatched || stored.State != "in_progress" {
					t.Errorf("native allocation preceded task receipt: %+v %v", stored, err)
				}
			}
			p := taskhost.Plan{Scope: scope, Permits: []taskhost.Permit{{Kind: "OpenChat", Program: "codex"}}, Deadline: time.Now().Add(time.Minute)}
			review, err := h.Review(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			if len(conn.calls.Snapshot()) != 0 {
				t.Fatal("review contacted provider")
			}
			handle, err := h.Begin(context.Background(), review.ID)
			if err != nil || questions.Load() != 1 {
				t.Fatalf("trusted review confirmation: %v, questions %d", err, questions.Load())
			}
			op := taskhost.Operation{ID: "allocate", Kind: "OpenChat", Program: "codex"}
			if _, err := h.Start(context.Background(), handle, op); err != nil {
				t.Fatal(err)
			}
			var receipt taskhost.Receipt
			select {
			case receipt = <-terminal:
			case <-time.After(testWait):
				t.Fatal("allocation did not settle")
			}
			if err := h.Shutdown(context.Background()); err != nil && mode == "success" {
				t.Fatal(err)
			}
			allocations := 0
			for _, call := range conn.calls.Snapshot() {
				if call.method == "session/new" {
					allocations++
				}
				if call.method == "session/delete" {
					t.Fatal("task allocation rolled back through delete")
				}
			}
			if mode == "dispatch persistence failure" {
				if allocations != 0 || receipt.Identity != "" {
					t.Fatalf("dispatch after failed persistence: %d %+v", allocations, receipt)
				}
			} else if allocations != 1 || receipt.Identity != "codex:created" {
				t.Fatalf("allocation identity lost: %d %+v", allocations, receipt)
			}
			if mode == "success" && (receipt.State != "succeeded" || !receipt.SetupFinished) {
				t.Fatalf("success: %+v", receipt)
			}
			if mode != "success" && receipt.State != "unknown" {
				t.Fatalf("failed checkpoint claimed known outcome: %+v", receipt)
			}
			key, _ := json.Marshal([]string{scope.Service, scope.Workspace, scope.Task})
			sum := sha256.Sum256(key)
			bytes, err := (records.Store{Dir: filepath.Join(configDir(), "task-operations")}).Read(hex.EncodeToString(sum[:]))
			if err != nil {
				t.Fatal(err)
			}
			var persisted struct {
				Receipts map[string]taskhost.Receipt `json:"receipts"`
			}
			if err := json.Unmarshal(bytes, &persisted); err != nil {
				t.Fatal(err)
			}
			if persisted.Receipts["allocate"].Identity != receipt.Identity || persisted.Receipts["allocate"].State != receipt.State {
				t.Fatal("durable receipt differs from event")
			}
		})
	}
}
