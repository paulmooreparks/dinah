// Package bridge carries a page's calls to a Go object, and the object's
// events back to the page, over HTTP. It keeps the shape Wails gives a
// page, window.go.main.App with one function per exported method and
// window.runtime.EventsOn, so a front end written against Wails' bindings
// runs unchanged, but it does not depend on Wails: a call is a POST of the
// method's name and arguments, and events arrive over Server-Sent Events.
//
// The script that defines window.go.main.App names exactly the methods the
// bridge offers, generated from its method table, so a front end that tests
// whether a method exists before calling it keeps its meaning.
package bridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"studio/internal/profile"
)

// The replay ring keeps this many of the most recent events, or this many
// bytes of them, whichever is reached first. They are the defaults of
// Bridge.ReplayEvents and Bridge.ReplayBytes. A ring that is too small costs
// a page a resync, never an event it cannot learn it lost, so they are
// options in code and not a setting.
const (
	DefaultReplayEvents = 8192
	DefaultReplayBytes  = 16 << 20
)

const (
	// subscriberQueue is how many events a page's stream may be behind
	// before the bridge closes it.
	subscriberQueue = 256
	// reconnectDelay is sent to the browser as the stream's retry field, so
	// a dropped stream is reopened after this long and not the browser's own
	// default.
	reconnectDelay = time.Second
	// ResyncEvent is what a page that reconnects with a cursor the bridge
	// cannot replay from is sent first: some of what it missed is gone.
	ResyncEvent = "dinah:resync"
)

// Bridge serves one object's methods and events.
type Bridge struct {
	// ObserveCall records a validated object call until its method returns.
	// Hosts may use it to account for work that would be interrupted by exit.
	ObserveCall func(string) func()

	// ReplayEvents and ReplayBytes bound the ring of recent events kept for
	// a page that reconnects. Zero selects DefaultReplayEvents and
	// DefaultReplayBytes. They are set before the first event is emitted.
	ReplayEvents int
	ReplayBytes  int

	target  reflect.Value
	methods map[string]reflect.Method
	runtime map[string]RuntimeFunc

	// mu guards everything below. An event is numbered, stored in the ring
	// and queued to every subscriber under it, and a subscriber is registered
	// and handed its replay under it, so an event is in a subscriber's replay
	// or in its live queue and never in both or neither.
	mu   sync.Mutex
	subs map[chan []byte]struct{}
	// epoch names this process's numbering. Sequence numbers begin again at
	// 1 in a new process, so a cursor from another epoch means nothing here.
	epoch string
	seq   uint64
	ring  []replayed
	// ringBytes is the size of the frames in the ring, and floor the highest
	// sequence number the ring has dropped: a page whose cursor is below it
	// has missed an event that is gone.
	ringBytes int
	floor     uint64
}

// replayed is one event kept for replay, as the frame the stream writes.
type replayed struct {
	seq   uint64
	frame []byte
}

// RuntimeFunc is one of the window.runtime functions a page may call, such
// as BrowserOpenURL, carried out in Go with the page's arguments.
type RuntimeFunc func(ctx context.Context, args []json.RawMessage) error

var (
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
)

// New returns a bridge to target's exported methods. A method may take a
// context.Context first, which receives the request's, and may return
// nothing, a value, an error, or a value and an error, as Wails allows.
// Methods of any other shape are not offered.
func New(target any) *Bridge {
	v := reflect.ValueOf(target)
	b := &Bridge{target: v, methods: map[string]reflect.Method{}, runtime: map[string]RuntimeFunc{}, subs: map[chan []byte]struct{}{}, epoch: newEpoch()}
	t := v.Type()
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		if callable(m.Type) {
			b.methods[m.Name] = m
		}
	}
	return b
}

// NewRestricted returns a bridge exposing only methods marked true in policy.
// Entries marked false are validated but excluded, as are unlisted methods.
// Every named entry must identify a callable exported method on target.
// New retains its unrestricted behavior for existing clients.
func NewRestricted(target any, policy map[string]bool) (*Bridge, error) {
	if target == nil {
		return nil, errors.New("the bridge target is nil")
	}
	b := New(target)
	for name := range policy {
		if _, ok := b.methods[name]; !ok {
			return nil, fmt.Errorf("bridge policy names missing or uncallable method %q", name)
		}
	}
	for name := range b.methods {
		if !policy[name] {
			delete(b.methods, name)
		}
	}
	return b, nil
}

func callable(t reflect.Type) bool {
	switch t.NumOut() {
	case 0:
	case 1:
		if t.Out(0) != errorType && !encodable(t.Out(0)) {
			return false
		}
	case 2:
		if !encodable(t.Out(0)) || t.Out(1) != errorType {
			return false
		}
	default:
		return false
	}
	// In(0) is the receiver.
	for i := 1; i < t.NumIn(); i++ {
		if i == 1 && t.In(i) == contextType {
			continue
		}
		if !encodable(t.In(i)) {
			return false
		}
	}
	return true
}

func encodable(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return false
	}
	return true
}

// Methods returns the names of the methods offered, in order.
func (b *Bridge) Methods() []string {
	names := make([]string, 0, len(b.methods))
	for n := range b.methods {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Runtime offers a window.runtime function to the page.
func (b *Bridge) Runtime(name string, f RuntimeFunc) { b.runtime[name] = f }

// call is what the page posts.
type call struct {
	Method string            `json:"method"`
	Args   []json.RawMessage `json:"args"`
}

// answer is what the page receives: the result, or the error's message,
// which the page's promise is rejected with, as a Wails binding's is. An
// error that has more for the page than its message gives it as Detail, and
// the page's promise is then rejected with an Error whose detail it is.
type answer struct {
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error,omitempty"`
	Detail json.RawMessage `json:"detail,omitempty"`
}

// DetailedError is an error with something more for the page than its
// message, such as the choices it leaves the person.
type DetailedError interface {
	error
	ErrorDetail() any
}

// Call invokes a method by name with arguments in JSON.
func (b *Bridge) Call(ctx context.Context, name string, args []json.RawMessage) (any, error) {
	if rest, ok := strings.CutPrefix(name, "runtime."); ok {
		f := b.runtime[rest]
		if f == nil {
			return nil, fmt.Errorf("the page asked for runtime.%s, which %s does not offer", rest, profile.Current().Name)
		}
		return nil, f(ctx, args)
	}
	m, ok := b.methods[name]
	if !ok {
		return nil, fmt.Errorf("there is no method %s", name)
	}
	t := m.Type
	in := []reflect.Value{b.target}
	next := 1
	if t.NumIn() > 1 && t.In(1) == contextType {
		in = append(in, reflect.ValueOf(ctx))
		next = 2
	}
	want := t.NumIn() - next
	if len(args) > want {
		return nil, fmt.Errorf("%s takes %d arguments, and %d were given", name, want, len(args))
	}
	for i := 0; i < want; i++ {
		p := reflect.New(t.In(next + i))
		if i < len(args) && len(args[i]) > 0 && string(args[i]) != "null" {
			if err := json.Unmarshal(args[i], p.Interface()); err != nil {
				return nil, fmt.Errorf("%s's argument %d: %w", name, i+1, err)
			}
		}
		in = append(in, p.Elem())
	}
	if b.ObserveCall != nil {
		if done := b.ObserveCall(name); done != nil {
			defer done()
		}
	}
	out := m.Func.Call(in)
	switch len(out) {
	case 0:
		return nil, nil
	case 1:
		if t.Out(0) == errorType {
			return nil, asError(out[0])
		}
		return out[0].Interface(), nil
	default:
		return out[0].Interface(), asError(out[1])
	}
}

func asError(v reflect.Value) error {
	if v.IsNil() {
		return nil
	}
	return v.Interface().(error)
}

// newEpoch returns a random name for this process's sequence numbers.
func newEpoch() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// The clock still tells two processes apart.
		return fmt.Sprintf("t%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}

// frame is one event as the stream writes it. The id is the cursor a
// browser sends back as Last-Event-ID when it reconnects. The payload is
// compact JSON, which has no line breaks.
func (b *Bridge) frame(seq uint64, payload []byte) []byte {
	return fmt.Appendf(nil, "id: %s.%d\ndata: %s\n\n", b.epoch, seq, payload)
}

func (b *Bridge) limits() (events, bytes int) {
	events, bytes = b.ReplayEvents, b.ReplayBytes
	if events <= 0 {
		events = DefaultReplayEvents
	}
	if bytes <= 0 {
		bytes = DefaultReplayBytes
	}
	return events, bytes
}

// Emit sends an event to every page listening, and keeps it for a page that
// reconnects. Every event gets the next sequence number.
func (b *Bridge) Emit(name string, data any) {
	payload, err := json.Marshal(struct {
		Name string `json:"name"`
		Data any    `json:"data"`
	}{name, data})
	if err != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	frame := b.frame(b.seq, payload)
	b.keep(b.seq, frame)
	for ch := range b.subs {
		select {
		case ch <- frame:
		default:
			// A page that has stopped reading is not waited for; its stream
			// is closed, and it comes back with its cursor and is either
			// replayed what it missed or told to resync.
			close(ch)
			delete(b.subs, ch)
		}
	}
}

// keep adds a frame to the ring and drops the oldest while either bound is
// exceeded, so a single frame larger than the byte bound is not kept. The
// caller holds b.mu.
func (b *Bridge) keep(seq uint64, frame []byte) {
	b.ring = append(b.ring, replayed{seq, frame})
	b.ringBytes += len(frame)
	maxEvents, maxBytes := b.limits()
	for len(b.ring) > 0 && (len(b.ring) > maxEvents || b.ringBytes > maxBytes) {
		b.floor = b.ring[0].seq
		b.ringBytes -= len(b.ring[0].frame)
		b.ring[0] = replayed{}
		b.ring = b.ring[1:]
	}
}

// parseCursor reads a cursor of the form <epoch>.<sequence>.
func parseCursor(s string) (epoch string, seq uint64, ok bool) {
	i := strings.LastIndexByte(s, '.')
	if i <= 0 {
		return "", 0, false
	}
	seq, err := strconv.ParseUint(s[i+1:], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return s[:i], seq, true
}

// subscribe registers a stream and returns its queue, which already holds
// what the cursor says the page missed. With no cursor the page gets live
// events only. With a cursor from this epoch that the ring still reaches, it
// gets every event after the cursor. With any other cursor (another epoch, a
// form that is not a cursor, one ahead of the numbering, or a gap the ring
// has dropped) it gets a resync event first, which has a sequence number of
// its own. Registering and filling the queue happen under one lock, so no
// event falls between the replay and the live events or appears in both.
func (b *Bridge) subscribe(cursor string) chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	var replay [][]byte
	resync := false
	if cursor != "" {
		epoch, seq, ok := parseCursor(cursor)
		if !ok || epoch != b.epoch || seq > b.seq || seq < b.floor {
			resync = true
		} else {
			for _, r := range b.ring {
				if r.seq > seq {
					replay = append(replay, r.frame)
				}
			}
		}
	}
	size := subscriberQueue + len(replay)
	if resync {
		size++
	}
	ch := make(chan []byte, size)
	if resync {
		b.seq++
		ch <- b.frame(b.seq, []byte(`{"name":"`+ResyncEvent+`","data":null}`))
	}
	for _, frame := range replay {
		ch <- frame
	}
	b.subs[ch] = struct{}{}
	return ch
}

// Handler serves the bridge at prefix, such as "/bridge": POST call, GET
// app.js and GET events beneath it.
func (b *Bridge) Handler(prefix string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+prefix+"/call", b.serveCall)
	mux.HandleFunc("GET "+prefix+"/app.js", func(w http.ResponseWriter, r *http.Request) { b.serveScript(w, prefix) })
	mux.HandleFunc("GET "+prefix+"/events", b.serveEvents)
	return mux
}

func (b *Bridge) serveCall(w http.ResponseWriter, r *http.Request) {
	var c call
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<20)).Decode(&c); err != nil {
		http.Error(w, "the call could not be read: "+err.Error(), http.StatusBadRequest)
		return
	}
	result, err := b.Call(r.Context(), c.Method, c.Args)
	var a answer
	if err != nil {
		a.Error = err.Error()
		var detailed DetailedError
		if errors.As(err, &detailed) {
			if b, encErr := json.Marshal(detailed.ErrorDetail()); encErr == nil {
				a.Detail = b
			}
		}
	} else if a.Result, err = json.Marshal(result); err != nil {
		a.Error = "the result could not be encoded: " + err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(a)
}

func (b *Bridge) serveEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	// A browser sends the id of the last event it received as Last-Event-ID
	// when it reconnects. A client that is not a browser passes the same
	// value as the after parameter; the header wins when both are present.
	cursor := r.Header.Get("Last-Event-ID")
	if cursor == "" {
		cursor = r.URL.Query().Get("after")
	}
	ch := b.subscribe(cursor)
	defer func() {
		b.mu.Lock()
		if _, open := b.subs[ch]; open {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, ": connected\nretry: %d\n\n", reconnectDelay.Milliseconds())
	fl.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case frame, open := <-ch:
			if !open {
				return
			}
			w.Write(frame)
			fl.Flush()
		}
	}
}

func (b *Bridge) serveScript(w http.ResponseWriter, prefix string) {
	names, _ := json.Marshal(b.Methods())
	runtime := make([]string, 0, len(b.runtime))
	for n := range b.runtime {
		runtime = append(runtime, n)
	}
	sort.Strings(runtime)
	rt, _ := json.Marshal(runtime)
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, script, prefix, names, rt)
}

// script defines window.go.main.App and window.runtime. Its three blanks
// are the bridge's address, the methods and the runtime functions.
const script = `// Generated by Dinah's bridge: the app's methods and events over HTTP.
(function () {
  'use strict';
  var base = %q, methods = %s, runtimeNames = %s;

  function call(method, args) {
    return fetch(base + '/call', {
      method: 'POST', credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ method: method, args: args })
    }).then(function (r) {
      if (!r.ok) return r.text().then(function (t) { throw t || ('The call to ' + method + ' failed with ' + r.status); });
      return r.json();
    }).then(function (a) {
      if (a.error && a.detail) { var e = new Error(a.error); e.detail = a.detail; throw e; }
      if (a.error) throw a.error;
      return a.result;
    });
  }

  var App = {};
  methods.forEach(function (name) {
    App[name] = function () { return call(name, Array.prototype.slice.call(arguments)); };
  });
  window.go = { main: { App: App } };

  var listeners = {};
  function on(name, fn, times) {
    (listeners[name] = listeners[name] || []).push({ fn: fn, times: times });
    return function () { off(name, fn); };
  }
  function off(name, fn) {
    if (!listeners[name]) return;
    listeners[name] = fn ? listeners[name].filter(function (l) { return l.fn !== fn; }) : [];
  }
  function dispatch(name, data) {
    (listeners[name] || []).slice().forEach(function (l) {
      if (l.times > 0 && --l.times === 0) off(name, l.fn);
      try { l.fn(data); } catch (e) { console.error(e); }
    });
  }
  var runtime = {
    EventsOn: function (name, fn) { return on(name, fn, -1); },
    EventsOnce: function (name, fn) { return on(name, fn, 1); },
    EventsOnMultiple: function (name, fn, n) { return on(name, fn, n); },
    EventsOff: function (name) { for (var i = 0; i < arguments.length; i++) off(arguments[i]); },
    EventsEmit: function (name) { dispatch(name, arguments.length > 1 ? arguments[1] : undefined); }
  };
  runtimeNames.forEach(function (name) {
    runtime[name] = function () { return call('runtime.' + name, Array.prototype.slice.call(arguments)); };
  });
  window.runtime = runtime;

  var events = new EventSource(base + '/events');
  events.onmessage = function (e) {
    var m;
    try { m = JSON.parse(e.data); } catch (err) { return; }
    dispatch(m.name, m.data);
  };
})();
`
