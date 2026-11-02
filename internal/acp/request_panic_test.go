package acp

import (
	"bufio"
	"context"
	"encoding/json"
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
