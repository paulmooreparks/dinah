package codexagent

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"studio/internal/acp"
)

func TestNativeAllocationReceiptPrecedesFurtherPreparation(t *testing.T) {
	for _, mode := range []string{"inject failure", "receipt persistence failure"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			var order []string
			record := func(value string) { mu.Lock(); defer mu.Unlock(); order = append(order, value) }
			c := fake(t, &testHandler{}, func(m message, send func(message)) {
				switch m.Method {
				case "model/list":
					send(answer(m, catalogResult()))
				case "thread/start":
					record("start")
					send(answer(m, map[string]any{"thread": map[string]string{"id": "allocated"}, "model": "test"}))
				case "thread/inject_items":
					record("inject")
					send(message{ID: m.ID, Error: &acp.Error{Code: -1, Message: "invented preparation error"}})
				case "thread/unsubscribe":
					record("close")
					send(answer(m, map[string]string{"status": "unsubscribed"}))
				default:
					t.Errorf("unexpected operation %s", m.Method)
					send(answer(m, struct{}{}))
				}
			})
			var out acp.NewSessionResult
			err := c.NewSessionWithReceipt(context.Background(), SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &out, func(id string) error {
				if id != "codex:allocated" {
					t.Fatalf("wrong receipt %s", id)
				}
				record("receipt")
				if mode == "receipt persistence failure" {
					return errors.New("invented disk failure")
				}
				return nil
			})
			var receipt *acp.SessionAllocationError
			if !errors.As(err, &receipt) || receipt.SessionID != "codex:allocated" {
				t.Fatalf("lost native receipt: %v", err)
			}
			want := []string{"start", "receipt", "inject", "close"}
			if mode == "receipt persistence failure" {
				want = []string{"start", "receipt", "close"}
			}
			mu.Lock()
			defer mu.Unlock()
			if !reflect.DeepEqual(order, want) {
				t.Fatalf("receipt order %v, want %v", order, want)
			}
		})
	}
}
