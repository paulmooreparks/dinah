package sessionhost

import (
	"reflect"
	"sync"
	"testing"

	"studio/internal/acp"
)

type subagentNotifyConnection struct {
	Connection
	notify func(string, any) error
}

func (c subagentNotifyConnection) Notify(method string, value any) error {
	return c.notify(method, value)
}

func TestSubagentServicePublishesIndexBeforeEventsAndCancelsQuestionsFirst(t *testing.T) {
	var mu sync.Mutex
	var runtime Runtime
	root := NewConversation(Identity{ID: "root"}, InitialActivity{Open: true}, false, false)
	runtime.PutConversationLocked(root.ID(), root)
	var effects []string
	unlocked := func() {
		t.Helper()
		if !mu.TryLock() {
			t.Fatal("callback held runtime mutex")
		}
		mu.Unlock()
	}
	var service *SubagentService
	service = NewSubagentService(&mu, &runtime, SubagentHost{
		Emit: func(event string, value any) {
			unlocked()
			info := value.(map[string]any)["subagent"].(SubagentInfo)
			owner, found, ok := service.Find(info.ID)
			if !ok || owner != root || found.State != info.State {
				t.Fatal("event preceded root indexing or state mutation")
			}
			mu.Lock()
			prior, seen := runtime.RunSubagentLocked(info.ID)
			mu.Unlock()
			if !seen || prior.Started != info.Started {
				t.Fatal("event preceded run tracking")
			}
			effects = append(effects, event)
		},
		CancelQuestions: func(ids map[string]bool) {
			unlocked()
			if !ids["child"] || !ids["grandchild"] {
				t.Fatalf("cancel family %v", ids)
			}
			effects = append(effects, "questions")
		},
		Connection: func(c *Conversation) Connection {
			unlocked()
			if c != root {
				t.Fatal("wrong provider root")
			}
			return subagentNotifyConnection{notify: func(method string, value any) error {
				unlocked()
				if method != "session/cancel" || value.(acp.CancelParams).SessionID != "child" {
					t.Fatal("cancel notification changed")
				}
				effects = append(effects, "notify")
				return nil
			}}
		},
	})
	if service.mu != &mu || service.runtime != &runtime {
		t.Fatal("service replaced runtime authority")
	}
	service.Spawn(root, root.ID(), "child", "child", "", "", true)
	service.Spawn(root, "child", "grandchild", "grandchild", "", "", true)
	effects = nil
	if err := service.Cancel("child"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(effects, []string{"questions", "notify"}) {
		t.Fatalf("cancellation order %v", effects)
	}
	effects = nil
	service.End(root, "child", "completed")
	if !reflect.DeepEqual(effects, []string{"questions", "agent:subagent"}) {
		t.Fatalf("completion order %v", effects)
	}
}
