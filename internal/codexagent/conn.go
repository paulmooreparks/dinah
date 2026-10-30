// Package codexagent adapts the documented Codex app-server protocol to the
// session interface used by Dinah. Protocol reference:
// https://learn.chatgpt.com/docs/app-server.
package codexagent

import (
	"context"
	"encoding/json"
	"io"
	"sync"

	"studio/internal/acp"
)

// Conn owns one app-server subprocess and its loaded conversations. The
// process's JSON-RPC transport is acp.Conn; this type translates between
// the app-server's methods and events and the ACP-shaped ones the host
// speaks.
type Conn struct {
	*acp.Conn
	handler  acp.Handler
	mu       sync.Mutex
	sessions map[string]*session
	models   []model
}

// options is how the transport runs Codex: its message bodies and
// diagnostics can contain credentials, so only method names and request IDs
// enter the host trace and stderr is never logged; and its messages carry
// no "jsonrpc" member.
func options(trace io.Writer) acp.Options {
	return acp.Options{Agent: "Codex", Command: command, Trace: trace, TraceHeadersOnly: true, OmitVersion: true}
}

// Start starts the supplied command, normally codex app-server, in dir.
func Start(name string, args []string, dir string, h acp.Handler, trace io.Writer) (*Conn, error) {
	c := newConn(h)
	t, err := acp.Start(name, args, dir, translator{c}, options(trace))
	if err != nil {
		return nil, err
	}
	c.Conn = t
	return c, nil
}

// connect is a Conn over given pipes in place of the process's, for tests.
func connect(r io.Reader, w io.WriteCloser, h acp.Handler, trace io.Writer) *Conn {
	c := newConn(h)
	c.Conn = acp.Connect(r, w, translator{c}, options(trace))
	return c
}

func newConn(h acp.Handler) *Conn {
	return &Conn{handler: h, sessions: map[string]*session{}}
}

// translator is the handler the transport calls with what the app-server
// sends on its own initiative.
type translator struct{ c *Conn }

func (t translator) Notify(method string, params json.RawMessage) { t.c.notification(method, params) }
func (t translator) Request(ctx context.Context, method string, params json.RawMessage) (any, *acp.Error) {
	return t.c.request(ctx, method, params)
}
