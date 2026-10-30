// Package acp is a client for the Agent Client Protocol (ACP), the JSON-RPC
// 2.0 protocol over stdio that `devin acp` speaks
// (https://agentclientprotocol.com, https://docs.devin.ai/cli/reference/commands).
//
// The client starts the agent as a subprocess, sends it requests, and passes
// the agent's own requests and notifications to a Handler. Each message is one
// line of JSON. Only the protocol's documented methods and fields are relied
// on; the agent's vendor extensions under _meta are passed through untouched
// and never decide anything here.
//
// Conn is also the transport of the other agents that speak JSON-RPC over a
// process's pipes (codexagent); Options holds what differs between them.
package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"studio/internal/profile"
)

// Handler receives what the agent sends on its own initiative.
type Handler interface {
	// Notify is called for each notification, such as session/update, in the
	// order the agent sent them.
	Notify(method string, params json.RawMessage)
	// Request is called, on its own goroutine, for each request the agent
	// makes of the client, such as session/request_permission or
	// fs/write_text_file. It may block until the person answers.
	Request(ctx context.Context, method string, params json.RawMessage) (any, *Error)
}

// Error is a JSON-RPC error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error is the agent's message alone, since the page shows it to the person;
// the code is in the trace log.
func (e *Error) Error() string { return e.Message }

// The JSON-RPC codes the client uses in its answers, and CodeAuthRequired,
// which the agent answers with when the person has to sign in first
// (ACP schema, ErrorCode: "-32000 Authentication required").
// CodeResourceNotFound is the answer to session/close for a session that
// does not exist or is not active (ACP schema, ErrorCode: "-32002 Resource
// not found").
const (
	CodeMethodNotFound   = -32601
	CodeInternal         = -32603
	CodeAuthRequired     = -32000
	CodeResourceNotFound = -32002
)

// IsAuthRequired reports whether an agent's answer says that the person has
// to sign in before the request can be carried out.
func IsAuthRequired(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == CodeAuthRequired
}

// Logf is where the package notes what is worth a line in the app's log,
// such as an agent writing something that is not protocol; the app sets it.
var Logf = func(format string, args ...any) {}

type message struct {
	JSONRPC string           `json:"jsonrpc,omitempty"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *Error           `json:"error,omitempty"`
}

type reply struct {
	result json.RawMessage
	err    *Error
}

// Options is what differs between the agents the transport runs.
type Options struct {
	// Agent is the agent's name in the errors the transport returns, such as
	// when it stops; empty is "the agent".
	Agent string
	// Env is the process's environment; nil gives it this process's own.
	// Either way the process is told it runs in Dinah's window
	// (profile.HostedVar).
	Env []string
	// Command builds the process to start, for an agent whose launcher needs
	// more than exec.Command gives; nil is exec.CommandContext. The window is
	// hidden and the process tied to the app after it is built.
	Command func(ctx context.Context, name string, args []string) (*exec.Cmd, error)
	// Trace, when not nil, receives every line sent and received and the
	// process's stderr, for diagnosis.
	Trace io.Writer
	// TraceHeadersOnly traces each message's method and ID alone and
	// discards the process's stderr, for an agent whose message bodies and
	// diagnostics can hold credentials, as Codex's can.
	TraceHeadersOnly bool
	// OmitVersion leaves the "jsonrpc" member off the messages sent, as the
	// Codex app-server protocol's messages are written.
	OmitVersion bool
}

func (o Options) agent() string {
	if o.Agent == "" {
		return "the agent"
	}
	return o.Agent
}

// Conn is one running agent process, or a connection over given pipes
// (Connect).
type Conn struct {
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	handler        Handler
	agent          string
	omitVersion    bool
	trace          io.Writer
	headersOnly    bool
	traceMu        sync.Mutex
	traceSecrets   []string
	traceRedactors []func(string) string

	wmu     sync.Mutex
	nextID  atomic.Int64
	pmu     sync.Mutex
	pending map[int64]chan reply

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	exit   error
	// warned is whether the agent has already been noted in the log for
	// writing a line that is not protocol; one note per connection is enough.
	warned bool
}

// Start runs the agent command in dir, hidden and tied to the app, and
// begins reading its output.
func Start(name string, args []string, dir string, h Handler, o Options) (*Conn, error) {
	ctx, cancel := context.WithCancel(context.Background())
	build := o.Command
	if build == nil {
		build = func(ctx context.Context, name string, args []string) (*exec.Cmd, error) {
			return exec.CommandContext(ctx, name, args...), nil
		}
	}
	cmd, err := build(ctx, name, args)
	if err != nil {
		cancel()
		return nil, err
	}
	cmd.Dir = dir
	env := o.Env
	if env == nil {
		env = cmd.Environ()
	}
	// The agent may load the person's own tools and hooks, Dinah's among
	// them, so it is told it runs in Dinah's window.
	cmd.Env = append(env, profile.Current().HostedVar()+"=1")
	hideWindow(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		_ = stdin.Close()
		return nil, err
	}
	c := newConn(ctx, cancel, stdin, h, o)
	c.cmd = cmd
	switch {
	case o.TraceHeadersOnly:
		cmd.Stderr = io.Discard
	case o.Trace != nil:
		cmd.Stderr = &traceLines{conn: c}
	}
	if err := cmd.Start(); err != nil {
		cancel()
		_ = stdin.Close()
		return nil, err
	}
	tieToApp(cmd)
	go c.read(stdout)
	return c, nil
}

// Connect is a connection over r and w in place of a process's pipes, for
// tests and for an agent in this process. Done closes when r ends.
func Connect(r io.Reader, w io.WriteCloser, h Handler, o Options) *Conn {
	ctx, cancel := context.WithCancel(context.Background())
	c := newConn(ctx, cancel, w, h, o)
	go c.read(r)
	return c
}

func newConn(ctx context.Context, cancel context.CancelFunc, w io.WriteCloser, h Handler, o Options) *Conn {
	return &Conn{stdin: w, handler: h, agent: o.agent(), omitVersion: o.OmitVersion, trace: o.Trace, headersOnly: o.TraceHeadersOnly,
		pending: map[int64]chan reply{}, ctx: ctx, cancel: cancel, done: make(chan struct{})}
}

// Done is closed when the agent process has ended.
func (c *Conn) Done() <-chan struct{} { return c.done }

// Err is why the process ended, once Done is closed.
func (c *Conn) Err() error { return c.exit }

// Context ends when the connection is closed; the agent's requests are
// answered under it.
func (c *Conn) Context() context.Context { return c.ctx }

// Close ends the agent process.
func (c *Conn) Close() {
	c.cancel()
	_ = c.stdin.Close()
}

// Call sends a request and waits for its answer. params of nil sends none.
func (c *Conn) Call(ctx context.Context, method string, params any, out any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id := c.nextID.Add(1)
	ch := make(chan reply, 1)
	c.pmu.Lock()
	c.pending[id] = ch
	c.pmu.Unlock()
	defer func() {
		c.pmu.Lock()
		delete(c.pending, id)
		c.pmu.Unlock()
	}()
	raw, _ := json.Marshal(id)
	rid := json.RawMessage(raw)
	p, err := encodeParams(params)
	if err != nil {
		return err
	}
	if err := c.send(message{ID: &rid, Method: method, Params: p}); err != nil {
		return err
	}
	select {
	case r := <-ch:
		if r.err != nil {
			return r.err
		}
		if out != nil && len(r.result) > 0 {
			return json.Unmarshal(r.result, out)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return errors.New(c.agent + " stopped")
	}
}

// Notify sends a notification, which has no answer. params of nil sends
// none.
func (c *Conn) Notify(method string, params any) error {
	p, err := encodeParams(params)
	if err != nil {
		return err
	}
	return c.send(message{Method: method, Params: p})
}

// encodeParams is params as JSON, or nothing for nil, since JSON-RPC lets a
// message leave its params out but not set them to null.
func encodeParams(params any) (json.RawMessage, error) {
	if params == nil {
		return nil, nil
	}
	return json.Marshal(params)
}

func (c *Conn) send(m message) error {
	if !c.omitVersion {
		m.JSONRPC = "2.0"
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.traceMessage("send", m, b)
	_, err = c.stdin.Write(append(b, '\n'))
	return err
}

func (c *Conn) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	defer func() {
		if c.cmd != nil {
			c.exit = c.cmd.Wait()
		}
		// A reader that ends on an error, rather than on the process's end,
		// is why the connection ended when the process says nothing itself.
		if c.exit == nil {
			c.exit = sc.Err()
		}
		close(c.done)
	}()
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var m message
		if err := json.Unmarshal(line, &m); err != nil {
			if !c.headersOnly {
				c.traceLine("recv ", line)
			}
			if !c.warned {
				c.warned = true
				Logf("%s wrote a line that is not protocol, which is skipped; later ones are not noted: %.200s", c.agent, line)
			}
			continue
		}
		c.traceMessage("recv", m, line)
		switch {
		case m.Method != "" && m.ID != nil:
			go c.answer(m)
		case m.Method != "":
			c.handler.Notify(m.Method, m.Params)
		case m.ID != nil:
			var id int64
			if json.Unmarshal(*m.ID, &id) != nil {
				continue
			}
			c.pmu.Lock()
			ch := c.pending[id]
			c.pmu.Unlock()
			if ch != nil {
				// A second answer to the same request is dropped rather than
				// stopping the reader.
				select {
				case ch <- reply{result: m.Result, err: m.Error}:
				default:
				}
			}
		}
	}
}

func (c *Conn) answer(m message) {
	result, rerr := c.handler.Request(c.ctx, m.Method, m.Params)
	out := message{ID: m.ID}
	if rerr != nil {
		out.Error = rerr
	} else {
		b, err := json.Marshal(result)
		if err != nil {
			out.Error = &Error{Code: CodeInternal, Message: err.Error()}
		} else {
			out.Result = b
		}
	}
	_ = c.send(out)
}

// RegisterTraceSecret excludes a channel credential from protocol and stderr
// diagnostics, including later agent messages that repeat the credential.
func (c *Conn) RegisterTraceSecret(secret string) {
	if secret == "" {
		return
	}
	c.traceMu.Lock()
	c.traceSecrets = append(c.traceSecrets, secret)
	c.traceMu.Unlock()
}

// RegisterTraceRedactor also covers credentials replayed from earlier sessions.
// A redactor must be pure and must not call back into the connection.
func (c *Conn) RegisterTraceRedactor(redact func(string) string) {
	c.traceMu.Lock()
	c.traceRedactors = append(c.traceRedactors, redact)
	c.traceMu.Unlock()
}

// traceMessage traces a message sent or received: the whole line, or its
// method and ID alone when only headers are traced.
func (c *Conn) traceMessage(direction string, m message, line []byte) {
	if !c.headersOnly {
		c.traceLine(direction+" ", line)
		return
	}
	id := ""
	if m.ID != nil {
		id = string(*m.ID)
	}
	c.traceLine("", []byte(direction+" "+m.Method+" "+id))
}

func (c *Conn) traceLine(prefix string, b []byte) {
	c.traceMu.Lock()
	defer c.traceMu.Unlock()
	if c.trace == nil {
		return
	}
	s := string(b)
	for _, redact := range c.traceRedactors {
		s = redact(s)
	}
	for _, secret := range c.traceSecrets {
		s = strings.ReplaceAll(s, secret, "[redacted]")
	}
	fmt.Fprintf(c.trace, "%s%s\n", prefix, s)
}

// stderr writes can split a credential between chunks. Buffer complete lines
// before redaction and suppress oversized diagnostic lines rather than leak.
type traceLines struct {
	conn      *Conn
	pending   []byte
	oversized bool
}

func (w *traceLines) Write(b []byte) (int, error) {
	for _, c := range b {
		if c == '\n' {
			if !w.oversized {
				w.conn.traceLine("stderr ", w.pending)
			}
			w.pending = w.pending[:0]
			w.oversized = false
		} else if !w.oversized {
			w.pending = append(w.pending, c)
			if len(w.pending) > 64<<10 {
				w.conn.traceLine("stderr ", []byte("[oversized diagnostic line omitted]"))
				w.pending = w.pending[:0]
				w.oversized = true
			}
		}
	}
	return len(b), nil
}

// Decode converts a value to another through JSON, as an adapter does to
// read the parameters the host gives as any.
func Decode(in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// Result puts an answer into out, when the caller gave one.
func Result(v, out any) error {
	if out == nil {
		return nil
	}
	return Decode(v, out)
}

// Raw is a value as JSON, for a message built in place.
func Raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
