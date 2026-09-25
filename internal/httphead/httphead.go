// Package httphead is the HTTP head: it answers one workbench over HTTP on
// the loopback interface, projecting the same library calls the MCP head
// projects and publishing the same canonical JSON.
//
// The head writes no payload of its own. Every body a route answers with is
// composed in package answer, which the MCP head calls too, so the HTTP body
// and the MCP text content are one byte sequence. What this package owns is
// the transport: which request is admitted, which route and method it names,
// how a URL, a query string and a body become a library request, and which
// status and headers an answer travels with.
package httphead

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"dinah/internal/answer"
	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// maxBody is the largest request body the head reads.
const maxBody = 1 << 20

// Config is what a head serves with.
type Config struct {
	// Root is the absolute path of the workbench the head serves, opened
	// afresh for every request.
	Root string
	// Home is the user base the library reads the caller's settings from.
	Home string
	// DefaultActor is who a request naming no actor acts as, which is the
	// actor the process that started the head resolved.
	DefaultActor string
	// Agent is what a request naming none of the four declared facts falls
	// back to, member by member.
	Agent bench.Agent
	// Host is the literal address the head is bound to, without brackets.
	Host string
	// Port is the port the head is bound to.
	Port int
	// BeforeRun, when set, is called after a request's workbench is opened
	// and its library request built, and before the library runs it. It is a
	// test seam, nil in production.
	BeforeRun func()
	// Interleave, when set, is assigned to each request's library, which
	// calls it inside a mutation while holding the card lock. It is a test
	// seam, nil in production.
	Interleave func()
	// observe, when set, is called with each library request just before
	// the library runs it. It is unexported, so only this package's own
	// tests can set it, and they use it to hold what a request carried
	// against what the library received.
	observe func(command string, req *verb.Request)
}

// head is the handler Handler returns.
type head struct {
	cfg Config
	mux *http.ServeMux
}

// Handler returns the HTTP head for one workbench. It binds nothing: the
// caller listens and serves, so the loopback rule is the caller's and every
// admission check is the handler's.
func Handler(cfg Config) http.Handler {
	h := &head{cfg: cfg, mux: http.NewServeMux()}
	for _, r := range routes {
		matched := r
		h.mux.HandleFunc(matched.pattern, func(w http.ResponseWriter, req *http.Request) {
			h.serveRoute(matched, w, req)
		})
	}
	h.mux.HandleFunc("/", h.serveUnmatched)
	return h
}

// exchange is one request in flight.
type exchange struct {
	w http.ResponseWriter
	r *http.Request
	// route is the matched route, nil before matching and on the catch-all.
	route *route
	// verb is the command the request resolves to as far as it is known,
	// and serve where no route matched.
	verb string
	// method is the method the request is dispatched as, which the form
	// tunnel may change from the method it arrived with.
	method string
	// served is the representation negotiation chose.
	served string
	// library is the request's own library, once the workbench is opened.
	library *verb.Library
}

// ServeHTTP runs the admission steps that read nothing of the route, then
// hands the request to the route table.
func (h *head) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Cache-Control", "no-store")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Vary", "Accept")
	x := &exchange{w: w, r: r, verb: "serve", method: r.Method}
	if !h.admittedHost(r.Host) {
		h.refuse(x, contract.ForeignHost, r.Host)
		return
	}
	if unsafeMethod(r.Method) {
		switch site := r.Header.Get("Sec-Fetch-Site"); site {
		case "cross-site", "same-site":
			h.refuse(x, contract.ForeignOrigin, site)
			return
		}
		if origin, present := originOf(r); present && !h.admittedOrigin(origin) {
			h.refuse(x, contract.ForeignOrigin, origin)
			return
		}
	}
	h.mux.ServeHTTP(w, r)
}

// serveUnmatched answers a path no route matches.
func (h *head) serveUnmatched(w http.ResponseWriter, r *http.Request) {
	x := &exchange{w: w, r: r, verb: "serve", method: r.Method}
	h.refuse(x, contract.UnknownResource, r.URL.Path)
}

// serveRoute runs the admission steps that read the route, negotiates, and
// hands the request to the route's read or to its acts.
func (h *head) serveRoute(rt *route, w http.ResponseWriter, r *http.Request) {
	x := &exchange{w: w, r: r, route: rt, verb: rt.command(), method: r.Method}
	if !rt.takes(r.Method) {
		h.notAllowed(x)
		return
	}
	if unsafeMethod(r.Method) {
		if refused := h.admitBody(x); refused {
			return
		}
	}
	if r.ContentLength > maxBody {
		h.refuse(x, contract.BodyTooLarge, strconv.FormatInt(r.ContentLength, 10))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	served, ok := negotiate(r.Header.Get("Accept"), rt.offered)
	if !ok {
		h.refuse(x, contract.NotAcceptable, r.Header.Get("Accept"))
		return
	}
	x.served = served
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		rt.read(h, x)
		return
	}
	h.act(x)
}

// admitBody runs steps 5 and 6 of admission for an unsafe request: the
// body's type must be one the route takes for the method as received, and a
// body a page on another site could send without asking must prove it came
// from this server's own pages. It reports whether it refused.
func (h *head) admitBody(x *exchange) bool {
	m := x.route.method(x.r.Method)
	declared, present := x.r.Header["Content-Type"]
	empty := x.r.ContentLength == 0
	var mediaType string
	switch {
	case present:
		raw := strings.Join(declared, ", ")
		parsed, ok := parseMediaType(raw)
		if !ok || !listed(m.accepts, parsed) {
			h.refuseType(x, m, raw)
			return true
		}
		mediaType = parsed
	case !empty:
		h.refuseType(x, m, "")
		return true
	case !m.empty:
		h.refuseType(x, m, "")
		return true
	}
	needsProof := mediaType == typeForm || (mediaType == "" && x.r.Method == http.MethodPost)
	if needsProof && !h.provesSameOrigin(x.r) {
		h.refuse(x, contract.OriginRequired, "Origin")
		return true
	}
	return false
}

// refuseType answers 415 with the header naming what the method as received
// takes on this route.
func (h *head) refuseType(x *exchange, m *method, detail string) {
	if name := acceptHeader(m.name); name != "" && len(m.accepts) > 0 {
		x.w.Header().Set(name, strings.Join(m.accepts, ", "))
	}
	h.refuse(x, contract.UnsupportedMediaType, detail)
}

// notAllowed answers 405 with the route's Allow header.
func (h *head) notAllowed(x *exchange) {
	x.w.Header().Set("Allow", strings.Join(x.route.allow(), ", "))
	h.refuse(x, contract.MethodNotAllowed, x.method)
}

// unsafeMethod reports whether a method is anything other than GET and HEAD.
func unsafeMethod(name string) bool {
	return name != http.MethodGet && name != http.MethodHead
}

// open opens the workbench for one request and returns its library. A
// workbench that will not open is answered, and the caller stops.
func (h *head) open(x *exchange, req *verb.Request) bool {
	opened, err := bench.Open(h.cfg.Root)
	if err != nil {
		// A library over no bench composes the answer to the open's own
		// error; no act or read runs on it.
		h.write(x, answer.FromErrorSealed(verb.New(nil, h.cfg.Home), req, err), 0)
		return false
	}
	x.library = verb.New(opened, h.cfg.Home)
	x.library.Interleave = h.cfg.Interleave
	return true
}

// execute hands a built request to the library and returns what it
// answered, sealed, for the caller to write.
func (h *head) execute(x *exchange, command string, req *verb.Request) answer.Sealed {
	if h.cfg.observe != nil {
		h.cfg.observe(command, req)
	}
	if h.cfg.BeforeRun != nil {
		h.cfg.BeforeRun()
	}
	return answer.RunSealed(command, x.library, req)
}

// refuse answers a refusal the head raised itself.
func (h *head) refuse(x *exchange, name, detail string) {
	req := &verb.Request{Verb: x.verb}
	h.write(x, answer.RefusalSealed(req, contract.Refuse(name, detail)), 0)
}

// refuseFor answers a refusal the head raised over a built request.
func (h *head) refuseFor(x *exchange, req *verb.Request, name, detail string) {
	h.write(x, answer.RefusalSealed(req, contract.Refuse(name, detail)), 0)
}

// failed answers an error the head met while resolving something on the
// library's behalf.
func (h *head) failed(x *exchange, req *verb.Request, err error) {
	h.write(x, answer.FromErrorSealed(x.library, req, err), 0)
}

// write encodes an answer package answer composed and writes it with its
// status and headers. An answer with no outcome is a read's own answer, which
// a runner produces only on success; success is the route's success status,
// and zero means 200.
func (h *head) write(x *exchange, answered answer.Sealed, success int) {
	if success == 0 {
		success = http.StatusOK
	}
	status := success
	if outcome := answered.Outcome(); outcome != "" {
		status = statusFor(outcome, answered.Refusal())
		if outcome == contract.OutcomeOK {
			status = success
		}
	}
	encoded, err := answered.Encode()
	if err != nil {
		http.Error(x.w, err.Error(), http.StatusInternalServerError)
		return
	}
	header := x.w.Header()
	if revision := answered.Revision(); revision != "" {
		header.Set("ETag", strconv.Quote(revision))
	}
	h.acceptHeaders(x, status)
	header.Set("Content-Type", servedJSON(x.served))
	x.w.WriteHeader(status)
	x.w.Write(encoded)
}

// acceptHeaders sets Accept-Patch and Accept-Post where the route answers
// with them: on a card's GET and PATCH, on the cards collection's GET and
// POST, and on every 415, which refuseType has already set.
func (h *head) acceptHeaders(x *exchange, status int) {
	if x.route == nil || status == http.StatusUnsupportedMediaType {
		return
	}
	header := x.w.Header()
	read := x.r.Method == http.MethodGet || x.r.Method == http.MethodHead
	switch x.route.pattern {
	case "/cards/{card}":
		if read || x.method == http.MethodPatch {
			header.Set("Accept-Patch", strings.Join(patchTypes, ", "))
		}
	case "/cards":
		if read || x.method == http.MethodPost {
			header.Set("Accept-Post", strings.Join(createTypes, ", "))
		}
	}
}

// servedJSON is the Content-Type a body in the canonical JSON travels with:
// the negotiated type when it is one of the two JSON types, and the vendor
// type otherwise, which is a refusal on a route whose one representation is
// not JSON.
func servedJSON(served string) string {
	if served == typeJSON {
		return typeJSON
	}
	return typeVendor
}

// profileVersion is the contract version the vendor type's profile names.
func profileVersion() string {
	return bench.ProfileVersion
}

// tooLarge reports whether an error is the body limit being reached.
func tooLarge(err error) bool {
	var limit *http.MaxBytesError
	return errors.As(err, &limit)
}
