package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

type panicRequestHandler struct{}

func (panicRequestHandler) Notify(string, json.RawMessage) {}
func (panicRequestHandler) Request(_ context.Context, method string, _ json.RawMessage) (any, *Error) {
	if method == "panic" {
		panic("sensitive-canary")
	}
	return map[string]string{"chat": method}, nil
}

type panicNotificationHandler struct {
	panicRequestHandler
	notifications chan string
}

func (h panicNotificationHandler) Notify(method string, _ json.RawMessage) {
	if method == "panic" {
		panic("sensitive-notification-canary")
	}
	h.notifications <- method
}

func TestNotificationPanicKeepsConnectionServingChats(t *testing.T) {
	notes := make(chan string, 4)
	previous := Logf
	Logf = func(format string, args ...any) { notes <- fmt.Sprintf(format, args...) }
	defer func() { Logf = previous }()
	h := panicNotificationHandler{notifications: make(chan string, 2)}
	in, peer := io.Pipe()
	out, client := io.Pipe()
	c := Connect(in, client, h, Options{})
	defer c.Close()
	defer peer.Close()
	defer out.Close()
	responses := make(chan string, 2)
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			responses <- sc.Text()
		}
	}()
	write := func(v any) {
		t.Helper()
		b, _ := json.Marshal(v)
		if _, err := peer.Write(append(b, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	for i, chat := range []string{"chat-one", "chat-two"} {
		write(map[string]any{"method": "panic", "params": map[string]string{"secret": "sensitive-notification-canary"}})
		write(map[string]any{"id": i + 1, "method": chat, "params": map[string]any{}})
		select {
		case raw := <-responses:
			var m message
			if json.Unmarshal([]byte(raw), &m) != nil || m.Error != nil || !strings.Contains(string(m.Result), chat) {
				t.Fatalf("request after notification fault: %s", raw)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("request after notification fault stopped")
		}
		write(map[string]any{"method": chat, "params": map[string]any{}})
		select {
		case got := <-h.notifications:
			if got != chat {
				t.Fatalf("notification went to %s, want %s", got, chat)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("notification after fault stopped")
		}
	}
	if len(notes) != 1 {
		t.Fatalf("got %d fault logs, want one per connection", len(notes))
	}
	if got := <-notes; got != "ignored a provider notification after a client handler fault" {
		t.Fatalf("fault log was not generic: %q", got)
	}
	select {
	case <-c.Done():
		t.Fatal("connection ended after a notification fault")
	default:
	}
}

func TestReverseRequestPanicKeepsConnectionServingOtherChats(t *testing.T) {
	in, peer := io.Pipe()
	out, client := io.Pipe()
	c := Connect(in, client, panicRequestHandler{}, Options{})
	defer c.Close()
	defer peer.Close()
	defer out.Close()
	responses := make(chan string, 3)
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			responses <- sc.Text()
		}
	}()
	for i, method := range []string{"panic", "chat-one", "chat-two"} {
		req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": i + 1, "method": method, "params": map[string]any{}})
		if _, err := peer.Write(append(req, '\n')); err != nil {
			t.Fatal(err)
		}
		var raw string
		select {
		case raw = <-responses:
		case <-time.After(3 * time.Second):
			t.Fatal("connection stopped answering")
		}
		var m message
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatal(err)
		}
		if m.ID == nil || string(*m.ID) != strconv.Itoa(i+1) {
			t.Fatalf("wrong request ID: %s", raw)
		}
		if i == 0 {
			if m.Error == nil || m.Error.Code != CodeInternal || strings.Contains(raw, "sensitive-canary") {
				t.Fatalf("panic response: %s", raw)
			}
		} else if m.Error != nil || !strings.Contains(string(m.Result), method) {
			t.Fatalf("other chat failed: %s", raw)
		}
		select {
		case <-c.Done():
			t.Fatal("connection ended")
		default:
		}
	}
}
