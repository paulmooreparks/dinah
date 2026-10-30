package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestTraceChannelCredentials(t *testing.T) {
	var out bytes.Buffer
	c := &Conn{trace: &out}
	c.RegisterTraceSecret("secret-channel-value")
	c.traceLine("send ", []byte(`{"env":[{"name":"TOKEN","value":"secret-channel-value"}]}`))
	c.traceLine("recv ", []byte(`{"text":"secret-channel-value"}`))
	stderr := &traceLines{conn: c}
	stderr.Write([]byte("initial secret-channel-"))
	stderr.Write([]byte("value\n"))
	stderr.Write([]byte(strings.Repeat("x", 65536) + "secret-channel-value\n"))
	if strings.Contains(out.String(), "secret-channel-value") || strings.Contains(out.String(), "initial secret-channel-") {
		t.Fatal("trace exposed credential")
	}
	if strings.Count(out.String(), "[redacted]") != 3 {
		t.Fatalf("missing redaction: %s", out.String())
	}
	if !strings.Contains(out.String(), "oversized diagnostic") {
		t.Fatal("oversized line was not suppressed")
	}
}

func TestTraceHeadersOnly(t *testing.T) {
	var out bytes.Buffer
	c := &Conn{trace: &out, headersOnly: true}
	id := json.RawMessage(`"r1"`)
	c.traceMessage("recv", message{ID: &id, Method: "thread/start", Params: json.RawMessage(`{"token":"secret"}`)}, []byte(`{"token":"secret"}`))
	c.traceMessage("send", message{Method: "initialized"}, []byte(`{"method":"initialized"}`))
	if out.String() != "recv thread/start \"r1\"\nsend initialized \n" {
		t.Fatalf("headers-only trace: %q", out.String())
	}
}

type notifyFunc func(method string, params json.RawMessage)

func (f notifyFunc) Notify(method string, params json.RawMessage) { f(method, params) }
func (f notifyFunc) Request(context.Context, string, json.RawMessage) (any, *Error) {
	return nil, &Error{Code: CodeMethodNotFound, Message: "none"}
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func TestConnectSkipsLinesThatAreNotProtocol(t *testing.T) {
	var notes []string
	old := Logf
	Logf = func(format string, args ...any) { notes = append(notes, fmt.Sprintf(format, args...)) }
	defer func() { Logf = old }()
	r, w := io.Pipe()
	got := make(chan string, 2)
	c := Connect(r, nopCloser{io.Discard}, notifyFunc(func(method string, _ json.RawMessage) { got <- method }), Options{Agent: "Test"})
	if _, err := w.Write([]byte("not protocol\n{\"method\":\"session/update\",\"params\":{}}\nstill not\n")); err != nil {
		t.Fatal(err)
	}
	if m := <-got; m != "session/update" {
		t.Fatalf("notified %q", m)
	}
	w.Close()
	<-c.Done()
	if len(notes) != 1 || !strings.Contains(notes[0], "Test wrote a line that is not protocol") || !strings.Contains(notes[0], "not protocol") {
		t.Fatalf("notes: %q", notes)
	}
	if c.Err() != nil {
		t.Fatalf("a reader ending cleanly is an error: %v", c.Err())
	}
}

func TestTraceRedactorCoversReplayedValues(t *testing.T) {
	var out bytes.Buffer
	c := &Conn{trace: &out}
	c.RegisterTraceRedactor(func(line string) string { return strings.ReplaceAll(line, "old-session-secret", "[redacted]") })
	c.traceLine("recv ", []byte(`{"text":"old-session-secret"}`))
	if strings.Contains(out.String(), "old-session-secret") || !strings.Contains(out.String(), "[redacted]") {
		t.Fatal("replayed credential escaped redaction")
	}
}
