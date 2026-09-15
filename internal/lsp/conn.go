// Package lsp serves one Dinah workbench to an editor over the Language
// Server Protocol on stdio. It reads and never writes: no verb runs here, no
// lock is taken, and every answer is built from a read of the workbench as it
// stands at the moment the question was asked.
//
// The head answers hover, document links, definition, completion and inlay
// hints over the references a workbench's own files carry, and publishes one
// namespaced request and one namespaced notification carrying the structured
// annotation a client draws its own chip from.
package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
)

// jsonrpcVersion is the version field every message on this wire carries.
const jsonrpcVersion = "2.0"

// message is one frame in either direction. A request carries an identifier
// and a method, a notification carries a method and no identifier, and a
// response carries an identifier and one of a result or an error. The four
// shapes travel as one struct because the base protocol does not tell them
// apart by anything but which members are filled.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

// responseError is the error member of a response.
type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// The two error codes this server raises. Nothing else here fails a request:
// a question about a workbench that has not resolved answers an empty result
// rather than an error, which is what keeps an editor usable in a folder
// whose workbench is one minute away from existing.
const (
	codeParseError     = -32700
	codeMethodNotFound = -32601
)

// conn is the framed base protocol: Content-Length, a blank line, and a JSON
// body. It is not internal/mcp's transport and does not reuse it. That head
// reads line-delimited JSON and answers strictly one response per request,
// while this one is framed and bidirectional, originating a refresh request
// and a log notification with no request of its own to answer.
type conn struct {
	reader *textproto.Reader
	body   *bufio.Reader

	// write serialises the whole of one frame, because the poll loop and the
	// request loop both originate traffic and a half-written header would
	// leave the client parsing a body as a header.
	write sync.Mutex
	out   io.Writer

	// nextID mints the identifiers of the requests this server originates.
	// It is guarded by write, which every originating path already holds.
	nextID int

	// pendingMu guards pending. It is separate from write because a
	// response arrives on the read loop while the poll loop may be
	// originating a request, and because a handler runs outside it.
	pendingMu sync.Mutex
	// pending is what is waiting on an answer, keyed by the identifier the
	// request went out under. A request whose answer nothing reads records
	// nothing here, so the map holds only what somebody asked a question
	// for.
	pending map[string]func(json.RawMessage, *responseError)
}

// newConn wraps a reader and a writer in the framing.
func newConn(in io.Reader, out io.Writer) *conn {
	body := bufio.NewReader(in)
	return &conn{
		reader:  textproto.NewReader(body),
		body:    body,
		out:     out,
		pending: map[string]func(json.RawMessage, *responseError){},
	}
}

// read takes the next frame off the wire. It answers io.EOF when the client
// has closed the stream, which is the ordinary way an editor stops a server.
func (c *conn) read() (*message, error) {
	header, err := c.reader.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	declared := header.Get("Content-Length")
	if declared == "" {
		return nil, errors.New("lsp: a frame arrived with no Content-Length header")
	}
	length, err := strconv.Atoi(strings.TrimSpace(declared))
	if err != nil || length < 0 {
		return nil, fmt.Errorf("lsp: a frame declared the length %q", declared)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(c.body, payload); err != nil {
		return nil, err
	}
	var read message
	if err := json.Unmarshal(payload, &read); err != nil {
		return nil, err
	}
	return &read, nil
}

// send writes one frame.
func (c *conn) send(payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	c.write.Lock()
	defer c.write.Unlock()
	return c.writeFrame(encoded)
}

// writeFrame puts the header and the body on the wire. The caller holds the
// write lock.
func (c *conn) writeFrame(encoded []byte) error {
	if _, err := fmt.Fprintf(c.out, "Content-Length: %d\r\n\r\n", len(encoded)); err != nil {
		return err
	}
	_, err := c.out.Write(encoded)
	return err
}

// respond answers a request with a result.
func (c *conn) respond(id json.RawMessage, result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return c.send(message{JSONRPC: jsonrpcVersion, ID: id, Result: encoded})
}

// refuse answers a request with an error.
func (c *conn) refuse(id json.RawMessage, code int, text string) error {
	return c.send(message{JSONRPC: jsonrpcVersion, ID: id, Error: &responseError{Code: code, Message: text}})
}

// notify sends a notification, which carries no identifier and is never
// answered.
func (c *conn) notify(method string, params any) error {
	encoded, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.send(message{JSONRPC: jsonrpcVersion, Method: method, Params: encoded})
}

// request sends a request this server originates and reads no answer to it.
// The inlay-hint refresh is the one such request. What the client does next
// is to ask for the hints again, and that is the whole of the answer it
// needs.
func (c *conn) request(method string, params any) error {
	return c.requestWith(method, params, nil)
}

// requestWith sends a request this server originates and records who is
// waiting for its answer, so a reply reaches the code that asked the
// question.
//
// The wait is not a blocking one. The handler runs on the read loop when the
// reply arrives, because a server blocking on a client's reply would stop
// reading the stream that reply arrives on.
func (c *conn) requestWith(method string, params any, handle func(json.RawMessage, *responseError)) error {
	encoded, err := json.Marshal(params)
	if err != nil {
		return err
	}
	c.write.Lock()
	c.nextID++
	id := json.RawMessage(strconv.Itoa(c.nextID))
	c.write.Unlock()
	if handle != nil {
		c.pendingMu.Lock()
		c.pending[string(id)] = handle
		c.pendingMu.Unlock()
	}
	if err := c.send(message{JSONRPC: jsonrpcVersion, ID: id, Method: method, Params: encoded}); err != nil {
		// A request that never reached the wire will never be answered, so
		// what was recorded for it is dropped rather than left waiting.
		c.pendingMu.Lock()
		delete(c.pending, string(id))
		c.pendingMu.Unlock()
		return err
	}
	return nil
}

// deliver hands a response to whatever originated the request it answers, and
// reports whether anything was waiting on it. A response nothing waits on is
// dropped, which is what the inlay-hint refresh's own reply gets.
func (c *conn) deliver(read *message) bool {
	if len(read.ID) == 0 {
		return false
	}
	key := strings.TrimSpace(string(read.ID))
	c.pendingMu.Lock()
	handle, waiting := c.pending[key]
	delete(c.pending, key)
	c.pendingMu.Unlock()
	if !waiting {
		return false
	}
	handle(read.Result, read.Error)
	return true
}
