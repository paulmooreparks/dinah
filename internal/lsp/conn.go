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
}

// newConn wraps a reader and a writer in the framing.
func newConn(in io.Reader, out io.Writer) *conn {
	body := bufio.NewReader(in)
	return &conn{reader: textproto.NewReader(body), body: body, out: out}
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

// request sends a request this server originates. The answer is not waited
// for: the two requests this server originates, the inlay-hint refresh and
// the configuration pull, are both acted on by what the client does next
// rather than by what it answers, and a server blocking on a client's reply
// inside its own poll loop would stop reading the stream that reply arrives
// on.
func (c *conn) request(method string, params any) error {
	encoded, err := json.Marshal(params)
	if err != nil {
		return err
	}
	c.write.Lock()
	c.nextID++
	id := json.RawMessage(strconv.Itoa(c.nextID))
	c.write.Unlock()
	return c.send(message{JSONRPC: jsonrpcVersion, ID: id, Method: method, Params: encoded})
}
