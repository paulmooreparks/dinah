package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"studio/internal/acp"
	"studio/internal/sessionhost"
)

func TestRecordedAllocationUsesTheRealHostReceiptBridge(t *testing.T) {
	for _, mode := range []string{"success", "receipt failure", "finished failure"} {
		t.Run(mode, func(t *testing.T) {
			a, conn := providerTestApp(t)
			dir := t.TempDir()
			intentPath, receiptPath := filepath.Join(dir, "intent"), filepath.Join(dir, "receipt")
			if err := os.WriteFile(intentPath, []byte("task:operation"), 0600); err != nil {
				t.Fatal(err)
			}
			dispatched, received, finished := false, false, 0
			conn.onNew = func() {
				if !dispatched {
					t.Fatal("native request preceded durable dispatch")
				}
			}
			callbacks := sessionhost.AllocationCallbacks{
				Dispatched: func() error {
					if _, err := os.Stat(intentPath); err != nil {
						return err
					}
					dispatched = true
					return nil
				},
				Received: func(id string) error {
					received = true
					if a.session(id) != nil {
						t.Fatal("runtime publication preceded receipt")
					}
					if mode == "receipt failure" {
						return errors.New("invented receipt failure")
					}
					return os.WriteFile(receiptPath, []byte(id), 0600)
				},
				Finished: func(id string, outcome error) error {
					finished++
					if !received || id != "codex:created" {
						t.Fatal("finish lost received identity", id, outcome)
					}
					if mode == "finished failure" {
						return errors.New("invented finish failure")
					}
					return nil
				},
			}
			info, err := a.sessionsLifecycle().NewRecorded(sessionhost.AllocationIntent{Source: "task:operation", Kind: "task_binding", Provider: "codex", Workspace: dir}, callbacks)
			if !received || finished != 1 || info.ID != "codex:created" {
				t.Fatal("host receipt was not retained", info, err, finished)
			}
			if mode == "success" {
				if err != nil || a.session(info.ID) == nil {
					t.Fatal(info, err)
				}
			} else {
				var receipt *acp.SessionAllocationError
				if !errors.As(err, &receipt) || receipt.SessionID != info.ID || a.session(info.ID) != nil {
					t.Fatal("host setup error lost identity or retained runtime", info, err)
				}
			}
			calls := 0
			for _, call := range conn.calls {
				if call.method == "session/new" {
					calls++
				}
				if call.method == "session/delete" {
					t.Fatal("allocation rollback deleted native copy")
				}
			}
			if calls != 1 {
				t.Fatal("allocation replayed", calls)
			}
		})
	}
}
