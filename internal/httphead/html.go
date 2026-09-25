package httphead

import (
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"dinah/internal/answer"
	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/pages"
	"dinah/internal/verb"
)

// The headers every text/html answer carries beside no-store, nosniff and
// Vary. frame-ancestors 'self' and X-Frame-Options refuse to let a page on
// another site frame these pages, which is the only defence against a click
// a framing page lures the reader into: that click is a same-origin form post
// and passes every row of admission. Referrer-Policy is same-origin and never
// no-referrer, because under no-referrer a browser sends Origin: null on a
// form post, which admission refuses.
const (
	htmlType              = "text/html; charset=utf-8"
	htmlReferrerPolicy    = "same-origin"
	htmlContentSecurity   = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; form-action 'self'; frame-ancestors 'self'; base-uri 'none'; object-src 'none'"
	htmlFrameOptions      = "SAMEORIGIN"
	defaultCardPageFields = "card,body,links,attachments,comments,checklist"
)

// pageOffers is what every route with a page offers: the two JSON types
// first, so a request with no Accept or with */* still gets the vendor type,
// and then text/html, which a browser's navigation Accept ranks above */*.
var pageOffers = []string{typeVendor, typeJSON, typeHTML}

// sendHTML is the one writer of a text/html answer, so every page, window,
// error page and redirect the pages answer carries the same headers and no
// renderer sets or omits one itself. A location makes the answer a 303 with
// no body and no Content-Type.
func (h *head) sendHTML(x *exchange, status int, location string, body []byte) {
	header := x.w.Header()
	header.Set("Referrer-Policy", htmlReferrerPolicy)
	header.Set("Content-Security-Policy", htmlContentSecurity)
	header.Set("X-Frame-Options", htmlFrameOptions)
	header.Del("ETag")
	if location != "" {
		header.Del("Content-Type")
		header.Set("Location", location)
		x.w.WriteHeader(http.StatusSeeOther)
		return
	}
	header.Set("Content-Type", htmlType)
	x.w.WriteHeader(status)
	x.w.Write(body)
}

// wantsHTML reports whether an answer goes out as a page. A request past
// negotiation wants what negotiation chose; a request refused before a route
// was chosen negotiates against the fixed list of pageOffers, so a browser
// is answered with a page and every other client exactly as before.
func (h *head) wantsHTML(x *exchange) bool {
	if x.served != "" {
		return x.served == typeHTML
	}
	chosen, ok := negotiate(x.r.Header.Get("Accept"), pageOffers)
	return ok && chosen == typeHTML
}

// pageState is what an HTML request carries beside its route's own read:
// the page parameters taken off its query, and the reads every page draws
// from, taken before the route's own.
type pageState struct {
	// windows are the page parameters, which the window state reads.
	windows url.Values
	// rest are the query's other parameters in the order they arrived.
	rest []pages.Param
	// read marks the three reads below as taken.
	read bool
	// cursor, status and tree are the changes cursor and the status and
	// default tree payloads.
	cursor       string
	status, tree []byte
	// library is the workbench the page's own reads run against, opened
	// once, and nil when it will not open.
	library *verb.Library
}

// takePageParameters removes the page parameters from an HTML GET's query
// before the route's read parses it, and keeps them for the window state. A
// page parameter given twice answers 400 naming it, which is the rule for any
// parameter. It reports whether it refused.
func (h *head) takePageParameters(x *exchange) bool {
	state := &pageState{windows: url.Values{}}
	x.page = state
	var kept []string
	for _, pair := range strings.Split(x.r.URL.RawQuery, "&") {
		if pair == "" {
			continue
		}
		rawName, rawValue, _ := strings.Cut(pair, "=")
		name, errName := url.QueryUnescape(rawName)
		value, errValue := url.QueryUnescape(rawValue)
		if errName != nil || errValue != nil {
			kept = append(kept, pair)
			continue
		}
		if !pages.IsPageParameter(name) {
			kept = append(kept, pair)
			state.rest = append(state.rest, pages.Param{Name: name, Value: value})
			continue
		}
		if _, given := state.windows[name]; given {
			h.refuse(x, contract.Usage, name)
			return true
		}
		state.windows.Set(name, value)
	}
	x.r.URL.RawQuery = strings.Join(kept, "&")
	return false
}

// readFor runs one read on the context's behalf, with the request's own
// identity, and returns the payload bytes, or nil when the read did not
// answer ok. It does not go through execute, so the test seams that watch a
// request's own library call do not see the page's reads.
func (h *head) readFor(x *exchange, library *verb.Library, command string, arguments map[string]any) []byte {
	req := answer.Build(command, arguments)
	h.identify(x, req, "")
	answered := answer.RunSealed(command, library, req)
	if outcome := answered.Outcome(); outcome != "" && outcome != contract.OutcomeOK {
		return nil
	}
	encoded, err := answered.Encode()
	if err != nil {
		return nil
	}
	return encoded
}

// contextLibrary opens the workbench for the page's own reads once per
// request, and answers nil when it will not open, which is the error page
// drawn without one.
func (h *head) contextLibrary(x *exchange) *verb.Library {
	if x.page == nil {
		x.page = &pageState{windows: url.Values{}}
	}
	if x.page.library == nil {
		opened, err := bench.Open(h.cfg.Root)
		if err != nil {
			return nil
		}
		x.page.library = verb.New(opened, h.cfg.Home)
	}
	return x.page.library
}

// readPageState takes the changes cursor, then the status, then the default
// tree, before any other read of the request, so a change landing while the
// page is drawn is reported by the next poll: at worst one redundant redraw,
// never a missed one.
func (h *head) readPageState(x *exchange) {
	if x.page == nil {
		x.page = &pageState{windows: url.Values{}}
	}
	if x.page.read {
		return
	}
	x.page.read = true
	library := h.contextLibrary(x)
	if library == nil {
		return
	}
	if changes := h.readFor(x, library, "changes", map[string]any{}); changes != nil {
		var set struct {
			Cursor string `json:"cursor"`
		}
		json.Unmarshal(changes, &set)
		x.page.cursor = set.Cursor
	}
	x.page.status = h.readFor(x, library, "status", map[string]any{})
	x.page.tree = h.readFor(x, library, "tree", map[string]any{})
}

// renderer is the process's resolved language.
func (h *head) renderer() *msg.Renderer {
	return msg.For(h.cfg.Lang)
}

// pageContext composes what every renderer is handed: the page state, the
// windows the URL names with the keys naming no live card dropped and each
// open card read, the command log, and the affordance table.
func (h *head) pageContext(x *exchange) *pages.Context {
	h.readPageState(x)
	c := &pages.Context{
		R:            h.renderer(),
		Lang:         h.renderer().Tag,
		Path:         x.r.URL.Path,
		Query:        x.r.URL.Query(),
		Rest:         x.page.rest,
		Status:       x.page.status,
		Tree:         x.page.tree,
		Cursor:       x.page.cursor,
		DefaultActor: h.cfg.DefaultActor,
		Log:          h.log.pageEntries(h.renderer(), logCapacity),
	}
	if c.Lang == "" {
		c.Lang = msg.Base
	}
	if table, err := answer.EncodeAffordanceTable(affordanceRows()); err == nil {
		c.Affordances = table
	}
	if c.Status == nil {
		return c
	}
	library := h.contextLibrary(x)
	windows := pages.ParseWindows(x.page.windows)
	if library != nil {
		windows = windows.Without(func(key string) bool {
			entity, _, err := library.Bench.ResolveReferenceIn(bench.LiveHalf, key)
			return err != nil || entity == nil || entity.Kind != bench.KindCard
		})
		for _, key := range windows.Open {
			c.WindowCards = append(c.WindowCards, h.windowCard(x, library, key))
		}
	}
	c.Windows = windows
	return c
}

// windowContext composes what the window route's one article is drawn with,
// which is less than a page's context holds: no changes cursor, no tree, no
// window state, and the newest log entry alone, since that is the only entry
// a sheet reads. The window's own two reads are windowCard's. The status is
// read as well, because the sheet names the pull destination by its column's
// title and the status is the one answer carrying every column's title; a
// window a page draws takes the same title from the status the page read.
func (h *head) windowContext(x *exchange) *pages.Context {
	c := &pages.Context{
		R:            h.renderer(),
		Lang:         h.renderer().Tag,
		Path:         x.r.URL.Path,
		Query:        x.r.URL.Query(),
		DefaultActor: h.cfg.DefaultActor,
		Status:       h.readFor(x, x.library, "status", map[string]any{}),
		Log:          h.log.pageEntries(h.renderer(), 1),
	}
	if c.Lang == "" {
		c.Lang = msg.Base
	}
	if table, err := answer.EncodeAffordanceTable(affordanceRows()); err == nil {
		c.Affordances = table
	}
	return c
}

// windowCard reads what one window draws.
func (h *head) windowCard(x *exchange, library *verb.Library, key string) pages.WindowCard {
	return pages.WindowCard{
		Key:          key,
		Show:         h.readFor(x, library, "show", map[string]any{"card": key, "fields": defaultCardPageFields}),
		Instructions: h.readFor(x, library, "instructions", map[string]any{"card": key}),
	}
}

// writePage answers a composed answer as a page: the route's own renderer
// when the read answered, and the error page otherwise.
func (h *head) writePage(x *exchange, answered answer.Sealed, status int) {
	encoded, err := answered.Encode()
	if err != nil {
		h.defect(x, err)
		return
	}
	if outcome := answered.Outcome(); outcome != "" && outcome != contract.OutcomeOK {
		h.errorPage(x, status, encoded)
		return
	}
	if x.route == nil {
		h.errorPage(x, status, encoded)
		return
	}
	c := h.pageContext(x)
	var body []byte
	switch x.route.pattern {
	case "/{$}":
		body, err = pages.Board(c)
	case "/cards":
		body, err = pages.CardList(c, encoded)
	case "/cards/{card}":
		body, err = pages.Card(c, encoded, h.readFor(x, h.contextLibrary(x), "instructions", map[string]any{"card": pathHead(x)}))
	case "/columns/{column}":
		column := pathHead(x)
		if library := h.contextLibrary(x); library != nil {
			if entity, _, err := library.Bench.ResolveReferenceIn(bench.LiveHalf, column); err == nil && entity != nil {
				column = entity.ID
			}
		}
		body, err = pages.Column(c, column, h.readFor(x, h.contextLibrary(x), "instructions", map[string]any{"card": pathHead(x)}))
	case "/tree":
		body, err = pages.TreePage(c, encoded)
	case "/search":
		body, err = pages.Search(c, encoded)
	case "/views":
		body, err = pages.Views(c, encoded)
	case "/views/{view}":
		body, err = pages.View(c, encoded, func(name, detail string, context map[string]string) pages.Refusal {
			return h.refusalOf(x.verb, "", name, detail, context, status)
		})
	default:
		_, ref, _ := refForPath(x.r.URL.Path)
		body, err = pages.Generic(c, x.verb, ref, encoded)
	}
	if err != nil {
		h.defect(x, err)
		return
	}
	h.sendHTML(x, status, "", body)
}

// refusalOf renders a refusal's sentence, next step included, for a page,
// through the composer the terminal uses. The card is the reference of the
// card the answer carried, and empty where it carried none.
func (h *head) refusalOf(command, card, name, detail string, context map[string]string, status int) pages.Refusal {
	refused := contract.RefuseWith(name, detail, context)
	composed := answer.RefusalSentence(h.renderer(), command, card, refused)
	sentence := composed.Sentence + composed.Tail(true)
	return pages.Refusal{Status: status, Name: name, Detail: detail, Sentence: sentence}
}

// errorPage answers a refusal as a page with the status the outcome maps to.
func (h *head) errorPage(x *exchange, status int, encoded []byte) {
	var refused struct {
		Refusal string            `json:"refusal"`
		Detail  string            `json:"detail"`
		Context map[string]string `json:"context"`
		Card    *struct {
			Ref string `json:"ref"`
		} `json:"card"`
	}
	json.Unmarshal(encoded, &refused)
	card := ""
	if refused.Card != nil {
		card = refused.Card.Ref
	}
	drawn := h.refusalOf(x.verb, card, refused.Refusal, refused.Detail, refused.Context, status)
	body, err := pages.ErrorPage(h.pageContext(x), drawn)
	if err != nil {
		h.defect(x, err)
		return
	}
	h.sendHTML(x, status, "", body)
}

// defect answers 500 for a fault of the head's own, as a page when the
// request wants one. A page that cannot be drawn is answered with its
// message, still through the one HTML writer.
func (h *head) defect(x *exchange, err error) {
	if !h.wantsHTML(x) {
		http.Error(x.w, err.Error(), http.StatusInternalServerError)
		return
	}
	message := html.EscapeString(err.Error())
	h.sendHTML(x, http.StatusInternalServerError, "", []byte(message))
}

// writeWindow answers the window route: one card's window markup alone.
func (h *head) writeWindow(x *exchange) {
	c := h.windowContext(x)
	body, err := pages.Window(c, h.windowCard(x, x.library, pathHead(x)))
	if err != nil {
		h.defect(x, err)
		return
	}
	h.sendHTML(x, http.StatusOK, "", body)
}

// readAsset answers the assets route from the fixed table, in each file's own
// type and whatever the Accept header says, and 404 in the vendor JSON type
// for anything the table does not list.
func readAsset(h *head, x *exchange) {
	data, contentType, ok := pages.Asset(x.r.PathValue("rest"))
	if !ok || x.r.URL.RawQuery != "" {
		x.served = typeVendor
		h.refuse(x, contract.UnknownResource, x.r.URL.Path)
		return
	}
	x.w.Header().Set("Content-Type", contentType)
	x.w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	x.w.WriteHeader(http.StatusOK)
	x.w.Write(data)
}

// writeGeneric answers a payload the head composed itself as the generic
// page.
func (h *head) writeGeneric(x *exchange, encoded []byte) {
	body, err := pages.Generic(h.pageContext(x), x.verb, "", encoded)
	if err != nil {
		h.defect(x, err)
		return
	}
	h.sendHTML(x, http.StatusOK, "", body)
}
