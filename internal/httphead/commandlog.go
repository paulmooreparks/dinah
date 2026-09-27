package httphead

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"dinah/internal/answer"
	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/pages"
	"dinah/internal/verb"
)

// logCapacity is how many entries the command log holds. It lives in the
// handler's memory, is lost when the process stops, and is written nowhere.
const logCapacity = 500

// TypedLine is a line typed into the command log, parsed by the terminal's
// own parser into the shape answer.Build takes.
type TypedLine struct {
	// Command is the command the line names.
	Command string
	// Arguments are the line's values keyed by parameter name.
	Arguments map[string]any
	// Actor is what the line gave --actor, and empty when it gave none.
	Actor string
}

// LogEntry is one act the pages performed, or one line typed into the log,
// recorded as the command line that would have done it.
type LogEntry struct {
	// Seq is the entry's number, one-based, per process and never reused.
	Seq int
	// At is when the entry was recorded.
	At time.Time
	// Source is form for an act posted from a page's form, and typed for a
	// line typed into the log or run again from it.
	Source string
	// Verb is the command, and empty when no request was built.
	Verb string
	// Args are the derived arguments, followed by --actor and the actor when
	// the act's actor differs from the handler's default.
	Args []string
	// Line is "dinah " and the derived command line, and empty when no
	// request was built.
	Line string
	// Typed is a typed entry's line as the reader typed it, trimmed, and
	// empty on a form entry.
	Typed string
	// Basis is the basis the act's request carried, and empty when none.
	Basis string
	// Outcome is ok, refused, stale or unreachable, or read for a typed read.
	Outcome string
	// Refusal, Detail and Context are the refusal the answer carried.
	Refusal string
	Detail  string
	Context map[string]string
	// Card, Column and State are the card the answer carried, as it stood
	// when the answer was given, which a stale entry names.
	Card, Column, State string
	// Target is the path of the card or comment the answer carried.
	Target string
}

// commandLog is the ring of recent entries.
type commandLog struct {
	mu      sync.Mutex
	entries []LogEntry
	seq     int
}

// record appends an entry, numbering it and evicting the oldest once the
// ring is full.
func (l *commandLog) record(entry LogEntry) LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seq++
	entry.Seq = l.seq
	entry.At = time.Now()
	l.entries = append(l.entries, entry)
	if len(l.entries) > logCapacity {
		l.entries = append([]LogEntry(nil), l.entries[len(l.entries)-logCapacity:]...)
	}
	return entry
}

// find returns the entry a number names while the ring still holds it.
func (l *commandLog) find(seq int) (LogEntry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, entry := range l.entries {
		if entry.Seq == seq {
			return entry, true
		}
	}
	return LogEntry{}, false
}

// snapshot returns the entries newest first.
func (l *commandLog) snapshot() []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]LogEntry, len(l.entries))
	for i, entry := range l.entries {
		out[len(l.entries)-1-i] = entry
	}
	return out
}

// pageEntries returns at most most of the entries, newest first, with each
// refusal's and each stale answer's sentence rendered.
func (l *commandLog) pageEntries(r *msg.Renderer, most int) []pages.LogEntry {
	var out []pages.LogEntry
	for _, entry := range l.snapshot() {
		if len(out) == most {
			break
		}
		drawn := pages.LogEntry{
			Seq: entry.Seq, At: entry.At, Outcome: entry.Outcome, Verb: entry.Verb,
			Line: entry.Line, Rerun: entry.Line != "", Guarded: entry.Basis != "", Target: entry.Target,
		}
		drawn.Card = entry.Card
		if drawn.Line == "" {
			drawn.Line = "dinah " + entry.Typed
		}
		switch entry.Outcome {
		case contract.OutcomeRefused:
			refused := contract.RefuseWith(entry.Refusal, entry.Detail, entry.Context)
			composed := answer.RefusalSentence(r, entry.Verb, entry.Card, refused)
			drawn.Sentence = entry.Refusal + " " + composed.Sentence + composed.Tail(true)
		case contract.OutcomeStale:
			drawn.Sentence = r.T("page.log.stale", "card", entry.Card, "column", entry.Column, "state", entry.State)
		}
		out = append(out, drawn)
	}
	return out
}

// answered is the members of an answer the log reads off its bytes.
type answered struct {
	Refusal string            `json:"refusal"`
	Detail  string            `json:"detail"`
	Context map[string]string `json:"context"`
	Card    *struct {
		Ref         string `json:"ref"`
		ColumnTitle string `json:"column_title"`
		State       string `json:"state"`
	} `json:"card"`
}

// entryFor composes the log entry for an act the head ran: the derived line,
// the outcome, and what the answer carried.
func (h *head) entryFor(source, typed string, req *verb.Request, result answer.Sealed) LogEntry {
	entry := LogEntry{Source: source, Typed: typed, Verb: req.Verb, Basis: req.Basis, Outcome: result.Outcome()}
	if cmd, ok, reason := verb.DeriveCommand(req); ok {
		entry.Args = append([]string(nil), cmd.Args...)
		if req.Actor != "" && req.Actor != h.cfg.DefaultActor {
			entry.Args = append(entry.Args, "--actor", req.Actor)
		}
		entry.Line = "dinah " + verb.Command{Verb: cmd.Verb, Args: entry.Args}.Line()
	} else {
		entry.Detail = reason
	}
	encoded, _ := result.Encode()
	var carried answered
	json.Unmarshal(encoded, &carried)
	if entry.Outcome != contract.OutcomeOK {
		entry.Refusal, entry.Context = carried.Refusal, carried.Context
		if carried.Detail != "" {
			entry.Detail = carried.Detail
		}
	}
	if carried.Card != nil {
		entry.Card, entry.Column, entry.State = carried.Card.Ref, carried.Card.ColumnTitle, carried.Card.State
		entry.Target = pathForRef(pathCard, carried.Card.Ref)
	}
	if req.Verb == "comment" && result.Outcome() == contract.OutcomeOK && result.Detail() != "" {
		entry.Target = pathForRef(pathCard, result.Detail())
	}
	return entry
}

// localPath is the one rule every Location the pages write satisfies: exactly
// "/", or "/" followed by a byte other than "/" and "\", with no control byte
// anywhere. A reference beginning "//" or "/\" is one a browser reads as
// naming another host, and the rule refuses both by its own test on the
// string written, not by relying on path cleaning.
func localPath(s string) bool {
	if s == "" || s[0] != '/' {
		return false
	}
	if len(s) > 1 && (s[1] == '/' || s[1] == '\\') {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return false
		}
	}
	return true
}

// returnPath is where an act posted from a page returns to: the path and
// query of a Referer naming one of this server's admitted origins, escaped as
// url.URL writes them, when that satisfies localPath, and the fallback
// otherwise. The fragment is dropped.
func (h *head) returnPath(r *http.Request, fallback string) string {
	referer, err := url.Parse(r.Header.Get("Referer"))
	if err != nil || !referer.IsAbs() || referer.Scheme != "http" || referer.User != nil || !h.admittedHost(referer.Host) {
		return fallback
	}
	candidate := referer.EscapedPath()
	if referer.RawQuery != "" {
		candidate += "?" + referer.RawQuery
	}
	if !localPath(candidate) {
		return fallback
	}
	return candidate
}

// withWindowOpen opens a card as the top window on a return URL, keeping the
// URL's other parameters and its window state.
func withWindowOpen(location, key string) string {
	path, raw, _ := strings.Cut(location, "?")
	values := url.Values{}
	var rest []pages.Param
	for _, pair := range strings.Split(raw, "&") {
		if pair == "" {
			continue
		}
		rawName, rawValue, _ := strings.Cut(pair, "=")
		name, _ := url.QueryUnescape(rawName)
		value, _ := url.QueryUnescape(rawValue)
		if pages.IsPageParameter(name) {
			values.Set(name, value)
			continue
		}
		rest = append(rest, pages.Param{Name: name, Value: value})
	}
	return pages.URLFor(path, rest, pages.ParseWindows(values).Raised(key))
}

// redirectAfter answers an act the pages performed: 303 to the return URL,
// with the resulting card opened as the top window after an add or a pull
// that answered ok. A return URL that fails localPath is a defect answered
// 500, never a redirect.
func (h *head) redirectAfter(x *exchange, command string, result answer.Sealed) {
	fallback := "/"
	if card := result.CardRef(); card != "" {
		fallback = pathForRef(pathCard, card)
	}
	if !localPath(fallback) {
		h.defectPage(x, "Location "+fallback)
		return
	}
	location := h.returnPath(x.r, fallback)
	if (command == "add" || command == verb.Pull) && result.Outcome() == contract.OutcomeOK && result.CardRef() != "" {
		location = withWindowOpen(location, result.CardRef())
	}
	if !localPath(location) {
		h.defectPage(x, "Location "+location)
		return
	}
	h.sendHTML(x, http.StatusSeeOther, location, nil)
}

// defectPage answers 500 as the HTML error page, naming the fault. A defect
// is the head's own and no refusal, so the page carries no refusal name.
func (h *head) defectPage(x *exchange, detail string) {
	body, err := pages.ErrorPage(h.pageContext(x), pages.Refusal{Status: http.StatusInternalServerError, Detail: detail, Sentence: h.renderer().T("page.defect")})
	if err != nil {
		h.defect(x, err)
		return
	}
	h.sendHTML(x, http.StatusInternalServerError, "", body)
}

// pageAct finishes an act a page's form posted: the library has run it, so
// the log records the request and the head answers 303 whatever the outcome.
func (h *head) pageAct(x *exchange, command string, req *verb.Request, result answer.Sealed) {
	h.log.record(h.entryFor("form", "", req, result))
	h.redirectAfter(x, command, result)
}

// readCommands answers GET /commands: the whole command log as a page.
func readCommands(h *head, x *exchange) {
	if x.r.URL.RawQuery != "" {
		h.refuse(x, contract.Usage, firstQueryName(x.r.URL.RawQuery))
		return
	}
	body, err := pages.CommandLog(h.pageContext(x))
	if err != nil {
		h.defect(x, err)
		return
	}
	h.sendHTML(x, http.StatusOK, "", body)
}

// postCommands answers POST /commands, which performs a typed line, or runs
// an earlier entry again, as the page would have. Admission has already
// admitted a same-origin form; this is section 10.3 of the specification, in
// its order.
func postCommands(h *head, x *exchange) {
	if h.cfg.ParseLine == nil {
		h.refuse(x, contract.NotImplemented, x.r.URL.Path)
		return
	}
	if x.r.URL.RawQuery != "" {
		h.refuse(x, contract.Usage, firstQueryName(x.r.URL.RawQuery))
		return
	}
	if err := x.r.ParseForm(); err != nil {
		h.bodyFailed(x, err)
		return
	}
	form := x.r.PostForm
	for _, name := range sortedNames(form) {
		if len(form[name]) != 1 {
			h.refuse(x, contract.Usage, name)
			return
		}
		if name != "line" && name != "entry" {
			h.refuse(x, contract.Usage, name)
			return
		}
	}
	_, hasLine := form["line"]
	_, hasEntry := form["entry"]
	if hasLine == hasEntry {
		h.refuse(x, contract.Usage, "line")
		return
	}
	var words []string
	var basis, typed string
	if hasLine {
		typed = strings.TrimSpace(form.Get("line"))
		if typed == "" {
			h.sendHTML(x, http.StatusSeeOther, h.returnPath(x.r, "/"), nil)
			return
		}
		split, err := verb.SplitLine(typed)
		if err != nil {
			h.recordRefusedLine(x, typed, "", err)
			return
		}
		words = split
	} else {
		seq, err := strconv.Atoi(form.Get("entry"))
		earlier, held := h.log.find(seq)
		if err != nil || seq < 1 || strconv.Itoa(seq) != form.Get("entry") || !held || earlier.Verb == "" {
			h.refuse(x, contract.Usage, "entry")
			return
		}
		words = append([]string{earlier.Verb}, earlier.Args...)
		basis = earlier.Basis
		typed = strings.TrimPrefix(earlier.Line, "dinah ")
	}
	line, refusal := h.cfg.ParseLine(words)
	if refusal != nil {
		h.recordRefusedLine(x, typed, "", refusal)
		return
	}
	header := strings.TrimSpace(x.r.Header.Get("Dinah-Actor"))
	if header != "" && line.Actor != "" && strings.TrimSpace(line.Actor) != header {
		h.refuse(x, contract.Usage, "--actor")
		return
	}
	if !routed(line.Command) {
		h.recordRefusedLine(x, typed, line.Command, contract.Refuse(contract.NotServed, line.Command))
		return
	}
	x.verb = line.Command
	req := answer.Build(line.Command, line.Arguments)
	h.identify(x, req, strings.TrimSpace(line.Actor))
	if !actCommands()[line.Command] {
		h.typedRead(x, line, typed, req)
		return
	}
	req.Basis = basis
	result, ok := h.perform(x, line.Command, req)
	if !ok {
		return
	}
	h.log.record(h.entryFor("typed", typed, req, result))
	h.redirectAfter(x, line.Command, result)
}

// recordRefusedLine records a typed line that could not be performed, with
// what was typed and why, and returns the reader to the page.
func (h *head) recordRefusedLine(x *exchange, typed, command string, err error) {
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		refusal = contract.Refuse(contract.Usage, err.Error())
	}
	h.log.record(LogEntry{Source: "typed", Typed: typed, Verb: command, Outcome: contract.OutcomeRefused, Refusal: refusal.Name, Detail: refusal.Detail, Context: refusal.Extra})
	h.sendHTML(x, http.StatusSeeOther, h.returnPath(x.r, "/"), nil)
}

// perform opens the workbench and runs an act the head has built a request
// for, through execute, which every act route runs its library call through.
func (h *head) perform(x *exchange, command string, req *verb.Request) (answer.Sealed, bool) {
	if !h.open(x, req) {
		return answer.Sealed{}, false
	}
	return h.execute(x, command, req), true
}

// typedRead answers a typed read with a redirect to the read's own URL,
// recording it with the outcome read.
func (h *head) typedRead(x *exchange, line TypedLine, typed string, req *verb.Request) {
	location, served := h.readURL(x, line.Command, line.Arguments)
	if !served {
		h.recordRefusedLine(x, typed, line.Command, contract.Refuse(contract.NotServed, line.Command))
		return
	}
	if !localPath(location) {
		h.defectPage(x, "Location "+location)
		return
	}
	entry := LogEntry{Source: "typed", Typed: typed, Verb: line.Command, Outcome: "read", Target: location}
	if cmd, ok, _ := verb.DeriveCommand(req); ok {
		entry.Args = append([]string(nil), cmd.Args...)
		if req.Actor != "" && req.Actor != h.cfg.DefaultActor {
			entry.Args = append(entry.Args, "--actor", req.Actor)
		}
		entry.Line = "dinah " + verb.Command{Verb: cmd.Verb, Args: entry.Args}.Line()
	}
	h.log.record(entry)
	h.sendHTML(x, http.StatusSeeOther, location, nil)
}

// routed reports whether a command has a row in the route table, which is
// the set of commands the typed line runs. It is derived from the table and
// never written out, so it widens by itself as routes are added.
func routed(command string) bool {
	for _, name := range routedCommands() {
		if name == command {
			return true
		}
	}
	return false
}

// RoutedCommands lists every command some route runs, each once, which is
// the set the pages' typed line runs.
func RoutedCommands() []string {
	return routedCommands()
}

// RoutedActs lists the routed commands an unsafe method performs, in byte
// order, which is the set of routed commands that write. The terminal UI's
// correspondence check reads it to hold a command it lists as a read to a
// route that serves it on GET alone.
func RoutedActs() []string {
	acts := actCommands()
	names := make([]string, 0, len(acts))
	for name := range acts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// actCommands are the routed commands an unsafe method performs.
func actCommands() map[string]bool {
	acts := map[string]bool{}
	for _, r := range routes {
		for _, m := range r.methods {
			for _, a := range m.acts {
				acts[a.command] = true
			}
		}
	}
	return acts
}

// readURL composes the URL a typed read is answered at, the inverse of the
// route table's GET rows. The argument the path binds is escaped segment by
// segment through pathForRef, and every other argument becomes a query
// parameter under its own name, built with url.Values.Encode, so an argument
// holding a space or a tab gives an escaped URL. An argument the route holds
// back is carried into the URL and refused there by the route, as a URL typed
// by hand would be. It answers false for a read no route answers.
func (h *head) readURL(x *exchange, command string, arguments map[string]any) (string, bool) {
	query := url.Values{}
	bound := ""
	for _, param := range verb.Params(command) {
		value, given := arguments[param.Name]
		if !given {
			continue
		}
		text, isText := value.(string)
		if flag, isFlag := value.(bool); isFlag {
			if !flag {
				continue
			}
			text, isText = "true", true
		}
		if !isText {
			continue
		}
		switch {
		case (command == "show" || command == "instructions") && param.Name == "card",
			command == "list" && param.Name == "ref",
			command == "view" && param.Name == "view":
			bound = text
		default:
			query.Set(param.Name, text)
		}
	}
	path := ""
	switch command {
	case "status":
		path = "/"
	case "query":
		path = "/cards"
		if _, given := query["query"]; !given {
			query.Set("query", "")
		}
	case "next", "changes", "tree", "search", "whoami", "version":
		path = "/" + command
	case "view":
		path = "/views"
		if bound != "" {
			path = "/views/" + url.PathEscape(bound)
		}
	case "show", "instructions", "list":
		if bound == "" {
			return "", false
		}
		path = h.pathOfReference(x, bound)
		if command == "instructions" {
			path += "/instructions"
		}
	default:
		return "", false
	}
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path, true
}

// pathOfReference is the path a reference is served at, by the kind its
// leading segment resolves to: a roster word, the workbench, a workstream, a
// column, or anything else as a card, whose route answers 404 when it names
// none.
func (h *head) pathOfReference(x *exchange, ref string) string {
	for _, word := range rosterPaths {
		if ref == word {
			return pathForRef(pathRoster, ref)
		}
	}
	if ref == bench.WorkbenchRef {
		return pathForRef(pathWorkbench, ref)
	}
	if strings.HasPrefix(ref, workstreamPrefix) {
		return pathForRef(pathWorkstream, ref)
	}
	head, _, _ := strings.Cut(ref, "/")
	if library := h.contextLibrary(x); library != nil {
		if entity, _, err := library.Bench.ResolveReferenceIn(bench.LiveHalf, head); err == nil && entity != nil && entity.Kind == bench.KindColumn {
			return pathForRef(pathColumn, ref)
		}
	}
	return pathForRef(pathCard, ref)
}
