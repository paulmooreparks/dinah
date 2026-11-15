package codexagent

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"

	"studio/internal/acp"
)

func TestDeleteUsesPermanentNativeOperationAfterDescendantGuard(t *testing.T) {
	for _, refused := range []bool{false, true} {
		t.Run(map[bool]string{false: "allowed", true: "independent descendant"}[refused], func(t *testing.T) {
			var deletes atomic.Int32
			c := fake(t, &testHandler{}, func(m message, send func(message)) {
				switch m.Method {
				case "thread/list":
					var p struct {
						Archived    bool
						SourceKinds []string
						Cursor      string
					}
					if err := json.Unmarshal(m.Params, &p); err != nil {
						t.Error(err)
					}
					if len(p.SourceKinds) != 10 {
						t.Error("inventory omitted source kinds")
					}
					if p.Archived {
						send(answer(m, map[string]any{"data": []any{map[string]string{"id": "grandchild", "parentThreadId": "child"}}}))
					} else if p.Cursor == "" {
						send(answer(m, map[string]any{"data": []any{map[string]string{"id": "root"}}, "nextCursor": "next"}))
					} else {
						send(answer(m, map[string]any{"data": []any{map[string]string{"id": "child", "parentThreadId": "root"}, map[string]string{"id": "fork", "forkedFromId": "root"}}}))
					}
				case "thread/delete":
					var p map[string]string
					_ = json.Unmarshal(m.Params, &p)
					if p["threadId"] != "root" {
						t.Error("wrong native ID")
					}
					deletes.Add(1)
					send(answer(m, struct{}{}))
				default:
					t.Errorf("unexpected operation %s", m.Method)
					send(answer(m, struct{}{}))
				}
			})
			c.sessions["codex:root"] = &session{}
			c.SetDeletionGuard(func(_ context.Context, root string, children []string) error {
				if root != "codex:root" || !reflect.DeepEqual(children, []string{"codex:child", "codex:grandchild"}) {
					t.Errorf("wrong deletion scope %s %v", root, children)
				}
				if refused {
					return errors.New("independent descendant")
				}
				return nil
			})
			err := c.Call(context.Background(), "session/delete", acp.DeleteSessionParams{SessionID: "codex:root"}, nil)
			if (err != nil) != refused {
				t.Fatalf("delete result %v", err)
			}
			if deletes.Load() != map[bool]int32{false: 1, true: 0}[refused] {
				t.Fatal("wrong destructive request count")
			}
			c.mu.Lock()
			retained := c.sessions["codex:root"] != nil
			c.mu.Unlock()
			if retained != refused {
				t.Fatal("local identity removed before native success")
			}
		})
	}
}

func TestDeleteFailuresPreserveIdentity(t *testing.T) {
	for _, failure := range []string{"no guard", "busy", "inventory error", "native error", "repeated cursor", "cycle"} {
		t.Run(failure, func(t *testing.T) {
			var deletes atomic.Int32
			c := fake(t, &testHandler{}, func(m message, send func(message)) {
				if m.Method == "thread/delete" {
					deletes.Add(1)
				}
				if failure == "inventory error" || m.Method == "thread/delete" {
					send(message{ID: m.ID, Error: &acp.Error{Code: -1, Message: "stand-in refusal"}})
					return
				}
				if failure == "repeated cursor" {
					send(answer(m, map[string]any{"data": []any{}, "nextCursor": "same"}))
					return
				}
				if failure == "cycle" {
					send(answer(m, map[string]any{"data": []any{map[string]string{"id": "root", "parentThreadId": "child"}, map[string]string{"id": "child", "parentThreadId": "root"}}}))
					return
				}
				send(answer(m, map[string]any{"data": []any{}}))
			})
			c.sessions["codex:root"] = &session{}
			if failure == "busy" {
				c.sessions["codex:root"].completion = make(chan turn)
			}
			if failure != "no guard" {
				c.SetDeletionGuard(func(context.Context, string, []string) error { return nil })
			}
			if err := c.Call(context.Background(), "session/delete", acp.DeleteSessionParams{SessionID: "codex:root"}, nil); err == nil {
				t.Fatal("unsafe delete accepted")
			}
			want := int32(0)
			if failure == "native error" {
				want = 1
			}
			if deletes.Load() != want {
				t.Fatal("unexpected destructive request")
			}
			c.mu.Lock()
			retained := c.sessions["codex:root"] != nil
			c.mu.Unlock()
			if !retained {
				t.Fatal("failed deletion forgot identity")
			}
		})
	}
}
