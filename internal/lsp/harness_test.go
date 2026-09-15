package lsp

import (
	"bufio"
	"encoding/json"
	"io"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// harness is an in-process editor driving one server over the framed base
// protocol, which is the same wire a real client speaks.
//
// The two loops a running server has are both live here: the request loop
// reads the pipe this writes into, and the poll loop originates traffic with
// no request of its own to answer. So the reader runs on its own goroutine
// and files everything it sees, and a test asks for the response to an
// identifier or for the notifications of a method rather than reading the
// next frame and hoping.
type harness struct {
	t *testing.T

	toServer *io.PipeWriter
	server   *Server
	served   chan error

	mu        sync.Mutex
	responses map[string]*message
	incoming  []*message
	arrived   chan struct{}

	nextID int
}

// start builds a server over a pair of pipes and runs it.
func start(t *testing.T, opts Options) *harness {
	t.Helper()
	serverReads, clientWrites := io.Pipe()
	clientReads, serverWrites := io.Pipe()
	if opts.Messages == nil {
		opts.Messages = msg.For(msg.Base)
	}
	h := &harness{
		t:         t,
		toServer:  clientWrites,
		server:    New(opts, serverReads, serverWrites),
		served:    make(chan error, 1),
		responses: map[string]*message{},
		arrived:   make(chan struct{}, 1024),
	}
	go func() { h.served <- h.server.Serve() }()
	go h.read(clientReads)
	t.Cleanup(func() {
		clientWrites.Close()
		<-h.served
		serverWrites.Close()
	})
	return h
}

// read files every frame the server writes, so nothing is lost between the
// moment it arrives and the moment a test asks for it.
func (h *harness) read(from io.Reader) {
	body := bufio.NewReader(from)
	reader := textproto.NewReader(body)
	for {
		header, err := reader.ReadMIMEHeader()
		if err != nil {
			return
		}
		length, err := strconv.Atoi(strings.TrimSpace(header.Get("Content-Length")))
		if err != nil {
			return
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(body, payload); err != nil {
			return
		}
		var read message
		if err := json.Unmarshal(payload, &read); err != nil {
			return
		}
		h.mu.Lock()
		if read.Method == "" && read.ID != nil {
			h.responses[string(read.ID)] = &read
		} else {
			h.incoming = append(h.incoming, &read)
		}
		h.mu.Unlock()
		select {
		case h.arrived <- struct{}{}:
		default:
		}
	}
}

// send writes one request and waits for its response.
func (h *harness) send(method string, params any) json.RawMessage {
	h.t.Helper()
	h.nextID++
	id := strconv.Itoa(h.nextID)
	h.write(message{JSONRPC: jsonrpcVersion, ID: json.RawMessage(id), Method: method, Params: encode(h.t, params)})
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		found, ok := h.responses[id]
		h.mu.Unlock()
		if ok {
			if found.Error != nil {
				h.t.Fatalf("%s answered the error %d %s", method, found.Error.Code, found.Error.Message)
			}
			return found.Result
		}
		h.wait()
	}
	h.t.Fatalf("%s never answered", method)
	return nil
}

// answer replies to a request the server originated, under the identifier
// that request went out on. A client that never answers is the ordinary case
// for the inlay-hint refresh; the configuration pull is the one the server
// asks a question with.
func (h *harness) answer(asked *message, result any) {
	h.t.Helper()
	h.write(message{JSONRPC: jsonrpcVersion, ID: asked.ID, Result: encode(h.t, result)})
}

// notify writes one notification, which is never answered.
func (h *harness) notify(method string, params any) {
	h.t.Helper()
	h.write(message{JSONRPC: jsonrpcVersion, Method: method, Params: encode(h.t, params)})
}

// write puts one frame on the wire.
func (h *harness) write(payload message) {
	h.t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		h.t.Fatalf("encode: %v", err)
	}
	if _, err := io.WriteString(h.toServer, "Content-Length: "+strconv.Itoa(len(encoded))+"\r\n\r\n"); err != nil {
		h.t.Fatalf("write header: %v", err)
	}
	if _, err := h.toServer.Write(encoded); err != nil {
		h.t.Fatalf("write body: %v", err)
	}
}

// wait blocks until another frame arrives or a short grace period passes, so
// a test polling for a notification does not spin.
func (h *harness) wait() {
	select {
	case <-h.arrived:
	case <-time.After(10 * time.Millisecond):
	}
}

// sent lists every message the server originated under one method, in order.
func (h *harness) sent(method string) []*message {
	h.mu.Lock()
	defer h.mu.Unlock()
	var found []*message
	for _, read := range h.incoming {
		if read.Method == method {
			found = append(found, read)
		}
	}
	return found
}

// await polls until the server has originated at least count messages under a
// method, and fails when it never does.
func (h *harness) await(method string, count int) []*message {
	h.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if found := h.sent(method); len(found) >= count {
			return found
		}
		h.wait()
	}
	h.t.Fatalf("the server sent %d %s, wanted %d", len(h.sent(method)), method, count)
	return nil
}

// quiet reports how many messages of a method the server has sent, for an
// assertion that it sent none.
func (h *harness) quiet(method string) int {
	return len(h.sent(method))
}

// initialize sends the one request that resolves the workbench and starts the
// poll loop, declaring inlay-hint refresh support unless told otherwise.
func (h *harness) initialize(params map[string]any) {
	h.t.Helper()
	if params == nil {
		params = clientDeclaring(true)
	}
	h.send(methodInitialize, params)
}

// clientDeclaring builds the initialize params of a client that does or does
// not declare inlay-hint refresh support.
func clientDeclaring(refresh bool) map[string]any {
	return map[string]any{
		"capabilities": map[string]any{
			"workspace": map[string]any{
				"inlayHint": map[string]any{"refreshSupport": refresh},
			},
		},
	}
}

// open sends didOpen for a file on disk, reading its text off the filesystem
// so the fixture and the document the server models are the same bytes.
func (h *harness) open(path string) string {
	h.t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		h.t.Fatalf("read %s: %v", path, err)
	}
	return h.openText(path, string(text))
}

// openText sends didOpen for a path with text the caller composed.
func (h *harness) openText(path, text string) string {
	h.t.Helper()
	uri := fileURI(path)
	h.notify(methodDidOpen, map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": languageMarkdown, "version": 1, "text": text},
	})
	h.settle()
	return uri
}

// settle waits until the server has drained the notifications sent so far, by
// asking it a question whose answer it cannot give until it has. The request
// loop is sequential, so an answer to this one cannot arrive before the
// notifications ahead of it were handled.
func (h *harness) settle() {
	h.t.Helper()
	h.send(methodAnnotations, map[string]any{"textDocument": map[string]any{"uri": "file:///settle"}})
}

// encode marshals a value for a frame.
func encode(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return raw
}

// decode reads a response body into a value.
func decode[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var read T
	if len(raw) == 0 || string(raw) == "null" {
		return read
	}
	if err := json.Unmarshal(raw, &read); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return read
}

// fixture is a throwaway workbench with everything the annotation set needs:
// a flow, two workstreams, cards carrying links and overrides, a checklist, a
// comment and an attachment.
type fixture struct {
	root    string
	library *verb.Library
	bench   *bench.Bench
	// card is the identifier of the card the tests annotate references to.
	card string
	// other is a second card, so a links[].to has something to name.
	other string
	// item is the reference of the first checklist item on card.
	item string
	// attachment is the reference of the attachment on card.
	attachment string
	// comment is the reference of the comment on card.
	comment string
	// workstream is the identifier of the first workstream.
	workstream string
	// workstreamSlug is that workstream's handle.
	workstreamSlug string
}

// build makes a fixture workbench under a fresh temporary directory.
func build(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "wb")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	written, err := verb.Init(root, "fx", "alka", "", "", "")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	f := &fixture{root: written}
	f.reopen(t)

	f.card = f.add(t, "The first card")
	f.other = f.add(t, "The second card")
	f.run(t, "file", &verb.Request{Card: f.card, Kind: "open_question", Text: "Is this settled?", Owner: "operator"})
	f.run(t, "comment", &verb.Request{Card: f.card, Text: "A remark"})
	payload := filepath.Join(base, "note.md")
	if err := os.WriteFile(payload, []byte("a payload\n"), 0o644); err != nil {
		t.Fatalf("payload: %v", err)
	}
	f.run(t, "attach", &verb.Request{Ref: f.card, File: payload})
	f.run(t, "workstream", &verb.Request{Action: "new", Workstream: "The vsix stream", Slug: "vsix"})
	f.reopen(t)
	workstreams, err := f.bench.Workstreams()
	if err != nil || len(workstreams) == 0 {
		t.Fatalf("workstreams: %v %d", err, len(workstreams))
	}
	f.workstream, f.workstreamSlug = workstreams[0].ID, workstreams[0].Slug
	f.run(t, "join", &verb.Request{Card: f.card, Workstream: f.workstream})
	f.run(t, "link", &verb.Request{Card: f.card, Kind: "relates_to", LinkTo: f.other})
	f.item = "fx-1/questions/1"
	f.attachment = "fx-1/attachments/1"
	f.comment = "fx-1/comments/1"
	f.reopen(t)
	return f
}

// reopen rebuilds the fixture's own view of the workbench, which a test does
// after writing to it.
func (f *fixture) reopen(t *testing.T) {
	t.Helper()
	opened, err := bench.Open(f.root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	f.bench = opened
	f.library = verb.New(opened, "")
}

// add files a card and answers its identifier.
func (f *fixture) add(t *testing.T, title string) string {
	t.Helper()
	answer := f.library.Add(&verb.Request{Verb: "add", Actor: "alka", Title: title})
	if answer.Outcome != "ok" {
		t.Fatalf("add %s: %s %s", title, answer.Outcome, answer.Refusal)
	}
	f.reopen(t)
	return answer.Card.ID
}

// run drives one verb against the fixture, failing on a refusal.
func (f *fixture) run(t *testing.T, name string, req *verb.Request) {
	t.Helper()
	req.Verb, req.Actor = name, "alka"
	var answer *verb.Response
	switch name {
	case "file":
		answer = f.library.File(req)
	case "comment":
		answer = f.library.Comment(req)
	case "attach":
		answer = f.library.Attach(req)
	case "link":
		answer = f.library.Link(req)
	case "workstream":
		answer = f.library.NewWorkstream(req)
	default:
		// join and move are card verbs, which Library.Do dispatches by the
		// verb name the request carries.
		answer = f.library.Do(req)
	}
	if answer.Outcome != "ok" {
		t.Fatalf("%s: %s %s", name, answer.Outcome, answer.Refusal)
	}
	f.reopen(t)
}

// cardAnchor is the path of one card's anchor.
func (f *fixture) cardAnchor(id string) string {
	return filepath.Join(f.bench.CardsRoot(), id, bench.CardAnchor)
}

// columnAnchor is the path of one column's anchor.
func (f *fixture) columnAnchor(id string) string {
	return f.bench.ColumnAnchorPath(id)
}

// serve starts a server bound to the fixture by the explicit rung of the
// ladder, with the poll loop parked until a test releases it.
func (f *fixture) serve(t *testing.T) (*harness, *ticker) {
	t.Helper()
	return f.serveWith(t, nil)
}

// serveWith is serve with a chance to shape the options first, which is what
// a test comparing a server told something on the command line against one
// told the same thing later needs. Both go through the one construction site,
// so the two servers differ in exactly what the shaping function changed.
func (f *fixture) serveWith(t *testing.T, shape func(*Options)) (*harness, *ticker) {
	t.Helper()
	tick := newTicker()
	opts := Options{
		Workbench: f.root,
		Sleep:     tick.sleep,
		Now:       tick.now,
		Since:     tick.since,
	}
	if shape != nil {
		shape(&opts)
	}
	return start(t, opts), tick
}

// ticker is a clock a test drives by hand, so a poll loop runs exactly as
// many turns as the test asks for and no test waits on a real interval.
type ticker struct {
	release chan struct{}
	slept   chan time.Duration
	// walk is what the clock reports every completed walk took, which a test
	// raises to drive the slow-walk state.
	mu   sync.Mutex
	walk time.Duration
}

// newTicker builds a parked clock.
func newTicker() *ticker {
	return &ticker{release: make(chan struct{}), slept: make(chan time.Duration, 64)}
}

// sleep parks the poll loop until a test releases it.
func (c *ticker) sleep(d time.Duration) {
	c.slept <- d
	<-c.release
}

// now answers a fixed instant, the elapsed time being reported by since.
func (c *ticker) now() time.Time { return time.Unix(0, 0) }

// since answers whatever the test says a walk took.
func (c *ticker) since(time.Time) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.walk
}

// takes sets how long the clock reports the next completed walk took.
func (c *ticker) takes(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.walk = d
}

// baseline drives one whole walk, so the change cursor has been minted over
// the workbench as it stands before the test edits it.
//
// The first checkpoint a server runs mints a cursor and reports nothing,
// which is what a first call is specified to do. A test that edits the
// workbench before that first walk has run therefore has its edit absorbed
// into the baseline and never reported, and the tick it drives afterwards
// finds nothing changed. Which of the two happened first is a race between
// the poll goroutine, which starts at initialize, and the test. Driving one
// walk first settles it rather than making the wrong order unlikely.
func (c *ticker) baseline(t *testing.T) {
	t.Helper()
	c.step(t)
}

// step lets exactly one more walk run and returns once that walk has
// finished, answering what the loop then slept for.
//
// The loop parks in sleep after every completed walk, so a value on the slept
// channel is the proof that the walk before it finished. The second value is
// put back, so the next step finds the loop parked exactly as this one did.
func (c *ticker) step(t *testing.T) time.Duration {
	t.Helper()
	select {
	case <-c.slept:
	case <-time.After(10 * time.Second):
		t.Fatal("the poll loop never parked")
	}
	select {
	case c.release <- struct{}{}:
	case <-time.After(10 * time.Second):
		t.Fatal("the poll loop never woke")
	}
	select {
	case slept := <-c.slept:
		c.slept <- slept
		return slept
	case <-time.After(10 * time.Second):
		t.Fatal("the walk never finished")
	}
	return 0
}
