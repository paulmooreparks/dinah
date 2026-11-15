//go:build quality_task_host

package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"studio/internal/taskhost"
)

func TestTaskHostModelReceiptAndRuntimeRelease(t *testing.T) {
	for _, personChoice := range []bool{false, true} {
		t.Run(map[bool]string{false: "restore", true: "person-changed"}[personChoice], func(t *testing.T) {
			f := newFrozenTaskHost(t).(*frozenTaskHost)
			scope := f.scope(req())
			ops := []taskhost.Operation{
				{ID: "open", Kind: "OpenChat", Target: "codex:chat-a"},
				{ID: "model", Kind: "UseModel", Target: "codex:chat-a", Input: json.RawMessage(`{"option":"model","value":"changed"}`)},
				{ID: "restore", Kind: "UseModel", Target: "codex:chat-a", Input: json.RawMessage(`{"restoreOperation":"model"}`)},
				{ID: "release", Kind: "ReleaseChat", Target: "codex:chat-a", Input: json.RawMessage(`{"operation":"open"}`)},
				{ID: "forged-restore", Kind: "UseModel", Target: "codex:chat-a", Input: json.RawMessage(`{"restoreOperation":"foreign-operation","value":"initial","revision":"invented"}`)},
			}
			p := taskhost.Plan{Scope: scope, Deadline: time.Now().Add(time.Minute)}
			for _, op := range ops {
				p.Permits = append(p.Permits, taskhost.Permit{Kind: op.Kind, Target: op.Target, Input: op.Input})
			}
			review, err := f.store.client.Review(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			f.consent.Store(true)
			h, err := f.store.client.Begin(context.Background(), review.ID)
			f.consent.Store(false)
			if err != nil {
				t.Fatal(err)
			}
			run := func(op taskhost.Operation) taskhost.Receipt {
				t.Helper()
				if _, err := f.store.client.Start(context.Background(), h, op); err != nil {
					t.Fatal(err)
				}
				select {
				case receipt := <-f.terminal:
					return receipt
				case <-time.After(5 * time.Second):
					t.Fatal("operation did not settle")
					return taskhost.Receipt{}
				}
			}
			if r := run(ops[0]); r.State != "succeeded" {
				t.Fatal(r)
			}
			f.a.taskServices.mu.Lock()
			held := len(f.a.taskServices.interests)
			f.a.taskServices.mu.Unlock()
			if held != 1 {
				t.Fatal("open did not retain a runtime interest")
			}
			changed := run(ops[1])
			if changed.State != "succeeded" || len(changed.Result) == 0 {
				t.Fatal(changed)
			}
			if r := run(ops[4]); r.State != "failed" {
				t.Fatal("caller invented a restoration receipt", r)
			}
			if personChoice {
				if _, err := f.a.optionService().Set("codex:chat-a", "model", json.RawMessage(`"changed"`)); err != nil {
					t.Fatal(err)
				}
			}
			restored := run(ops[2])
			if personChoice && restored.State != "failed" || !personChoice && restored.State != "succeeded" {
				t.Fatal(restored)
			}
			if r := run(ops[3]); r.State != "succeeded" {
				t.Fatal(r)
			}
			f.a.taskServices.mu.Lock()
			held = len(f.a.taskServices.interests)
			f.a.taskServices.mu.Unlock()
			if held != 0 || f.a.session("codex:chat-a") == nil {
				t.Fatal("release lost the chat or retained its interest")
			}
			f.peer.mu.Lock()
			defer f.peer.mu.Unlock()
			for _, call := range f.peer.calls.Snapshot() {
				if call.method == "session/delete" {
					t.Fatal("runtime release deleted native history")
				}
			}
		})
	}
}
