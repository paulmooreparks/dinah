package sessionhost

import (
	"reflect"
	"studio/internal/acp"
	"sync"
	"testing"
	"time"
)

func TestClosedSessionCleanupOrderKeepsReplacementIdentity(t *testing.T) {
	var mu sync.Mutex
	var runtime Runtime
	s := NewConversation(Identity{ID: "chat", Provider: "codex"}, InitialActivity{Open: true}, false, false)
	replacement := NewConversation(Identity{ID: "chat", Provider: "codex"}, InitialActivity{Open: true}, false, false)
	runtime.PutConversationLocked("chat", s)
	s.ClosedDue(false, time.Now(), ClosedSessionEnd)
	var order []string
	var done *SessionEnd
	host := ClosedSessionsHost{
		LeaveBus: func(*Conversation) { order = append(order, "leave") },
		Emit:     func(string, any) { order = append(order, "event") },
		CallWithin: func(_ Connection, d time.Duration, method string, p, _ any) error {
			if d != 30*time.Second || method != "session/close" || p.(acp.CloseSessionParams).SessionID != "chat" {
				t.Fatal("close contract changed")
			}
			order = append(order, "call")
			mu.Lock()
			runtime.PutConversationLocked("chat", replacement)
			mu.Unlock()
			return nil
		},
		RevokeSession:       func(string) { order = append(order, "revoke") },
		CloseChatLog:        func(string) { order = append(order, "log-close") },
		CancelQuestions:     func(string) { order = append(order, "questions") },
		DisconnectSubagents: func(*Conversation) { order = append(order, "subagents") },
		Log: func(string, string, ...any) {
			select {
			case <-done.done:
			default:
				t.Fatal("final log preceded end completion")
			}
			if s.Ending() != nil {
				t.Fatal("end state not cleared")
			}
			order = append(order, "log")
		},
	}
	service := NewClosedSessions(&mu, &runtime, host)
	done = service.beginEnd(s)
	if done == nil {
		t.Fatal("end not admitted")
	}
	service.closeSession(nil, s, done)
	if runtime.ConversationLocked("chat") != replacement {
		t.Fatal("close removed replacement")
	}
	if !reflect.DeepEqual(order, []string{"leave", "event", "call", "revoke", "log-close", "questions", "subagents", "log", "event"}) {
		t.Fatal(order)
	}
}
