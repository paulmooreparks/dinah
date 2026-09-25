package httphead

import (
	"net/http"
	"net/url"
	"sort"

	"dinah/internal/answer"
	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// runRead answers a route whose command takes nothing from the path.
func runRead(command string) func(h *head, x *exchange) {
	return func(h *head, x *exchange) {
		req, ok := h.buildRead(x, command, nil)
		if !ok || !h.open(x, req) {
			return
		}
		h.answer(x, command, req, 0)
	}
}

// runShowAt answers show of the reference the path names.
func runShowAt(h *head, x *exchange) {
	kind, ref, _ := refForPath(x.r.URL.Path)
	h.readBound(x, "show", map[string]string{"card": ref}, kind, ref)
}

// runListAt answers list of the roster the path names.
func runListAt(h *head, x *exchange) {
	_, ref, _ := refForPath(x.r.URL.Path)
	h.readBound(x, "list", map[string]string{"ref": ref}, pathRoster, ref)
}

// readCards answers the cards collection: query when the query parameter is
// present, whatever its value, and list over the cards roster otherwise.
func readCards(h *head, x *exchange) {
	if _, present := x.r.URL.Query()["query"]; present {
		runRead("query")(h, x)
		return
	}
	h.readBound(x, "list", map[string]string{"ref": verb.RosterCards}, pathRoster, verb.RosterCards)
}

// readInstructions answers the instructions of the card or column the path
// names.
func readInstructions(h *head, x *exchange) {
	kind, head := x.route.binds, pathHead(x)
	h.readBound(x, "instructions", map[string]string{"card": head}, kind, head)
}

// readViews answers the listing of views, which takes none of view's
// parameters. All three are bound to nothing here, so a query string naming
// any of them answers 400.
func readViews(h *head, x *exchange) {
	h.readBound(x, "view", map[string]string{"view": "", "card": "", "explain": ""}, pathNone, "")
}

// readView answers one named view.
func readView(h *head, x *exchange) {
	h.readBound(x, "view", map[string]string{"view": x.r.PathValue("view")}, pathNone, "")
}

// readBelow answers a reference below a card or a column: list when the
// resolver says the reference names a collection, and show otherwise.
func readBelow(h *head, x *exchange) {
	kind, ref, _ := refForPath(x.r.URL.Path)
	marker := x.r.URL.Query()["archived"]
	probe := &verb.Request{Verb: "show", Archived: len(marker) == 1 && marker[0] != "false"}
	if !h.open(x, probe) || !h.bindsKind(x, probe, kind, pathHead(x)) {
		return
	}
	half := bench.LiveHalf
	if probe.Archived {
		half = bench.ArchivedHalf
	}
	command := "show"
	if kind == pathCard {
		if file, own := bench.CardOwnFile(x.r.PathValue("rest")); own && file == bench.CardFileJournal {
			command = "list"
		}
	}
	if command == "show" {
		if _, collection, err := x.library.Bench.ResolveReferenceIn(half, ref); err == nil && collection != nil {
			command = "list"
		}
	}
	bound := map[string]string{"card": ref}
	if command == "list" {
		bound = map[string]string{"ref": ref}
	}
	req, ok := h.buildRead(x, command, bound)
	if !ok {
		return
	}
	h.answer(x, command, req, 0)
}

// readClaim answers a GET of a card's claim with a redirect to the card,
// because a claim's state is carried on the card.
func readClaim(h *head, x *exchange) {
	probe := &verb.Request{Verb: verb.Claim}
	if !h.open(x, probe) {
		return
	}
	entity, ok := h.resolveKind(x, probe, pathCard, pathHead(x))
	if !ok {
		return
	}
	ref := pathHead(x)
	if entity != nil {
		ref = entity.Ref
	}
	header := x.w.Header()
	header.Set("Location", pathForRef(pathCard, ref))
	header.Set("Content-Type", servedJSON(x.served))
	x.w.WriteHeader(http.StatusSeeOther)
}

// readWindow answers a card's window route, which dinah-338 gives an HTML
// renderer. Until then it resolves the card and answers 501.
func readWindow(h *head, x *exchange) {
	probe := &verb.Request{Verb: "show"}
	if !h.open(x, probe) || !h.bindsKind(x, probe, pathCard, pathHead(x)) {
		return
	}
	h.refuseFor(x, probe, contract.NotImplemented, x.r.URL.Path)
}

// readBound answers a read whose command takes one parameter from the path,
// after checking that the path's first variable names the kind its prefix
// binds.
func (h *head) readBound(x *exchange, command string, bound map[string]string, kind pathKind, head string) {
	req, ok := h.buildRead(x, command, bound)
	if !ok || !h.open(x, req) {
		return
	}
	if (kind == pathCard || kind == pathColumn) && !h.bindsKind(x, req, kind, head) {
		return
	}
	h.answer(x, command, req, 0)
}

// buildRead builds the library request for a read from the query string and
// the parameters the path binds. A query parameter the command does not
// publish on this route, one given twice, or a marker whose value is not a
// boolean, answers 400.
func (h *head) buildRead(x *exchange, command string, bound map[string]string) (*verb.Request, bool) {
	x.verb = command
	query, err := url.ParseQuery(x.r.URL.RawQuery)
	if err != nil {
		h.refuse(x, contract.Usage, x.r.URL.RawQuery)
		return nil, false
	}
	members := published(command, keysOf(bound)...)
	arguments := map[string]any{}
	for _, name := range sortedNames(query) {
		param, ok := members[name]
		if !ok || len(query[name]) != 1 {
			h.refuse(x, contract.Usage, name)
			return nil, false
		}
		value := query[name][0]
		if !param.Marker {
			arguments[name] = value
			continue
		}
		switch value {
		case "", "true":
			arguments[name] = true
		case "false":
			arguments[name] = false
		default:
			h.refuse(x, contract.Usage, name)
			return nil, false
		}
	}
	if missing := missingRequired(command, arguments, bound); missing != "" {
		h.refuse(x, contract.Usage, missing)
		return nil, false
	}
	for name, value := range bound {
		arguments[name] = value
	}
	req := answer.Build(command, arguments)
	h.identify(x, req, "")
	return req, true
}

// bindsKind checks that a path's first variable names the kind its prefix
// binds, answering 404 when it does not.
func (h *head) bindsKind(x *exchange, req *verb.Request, kind pathKind, head string) bool {
	_, ok := h.resolveKind(x, req, kind, head)
	return ok
}

// resolveKind resolves a path's first variable and checks it names the kind
// the path's prefix binds, so a column and a card sharing a spelling cannot
// answer each other's URL. It answers 404 when the variable names nothing or
// names the other kind. Any other resolution error, such as a reference the
// archive does not hold, is left to the library, which raises it again in
// its own order; the entity is then nil and the caller goes on with the
// path's own spelling.
func (h *head) resolveKind(x *exchange, req *verb.Request, kind pathKind, head string) (*bench.EntityRef, bool) {
	name, want := contract.UnknownCard, bench.KindCard
	if kind == pathColumn {
		name, want = contract.UnknownColumn, bench.KindColumn
	}
	half := bench.LiveHalf
	if req.Archived {
		half = bench.ArchivedHalf
	}
	entity, _, err := x.library.Bench.ResolveReferenceIn(half, head)
	if err != nil {
		if refusal, ok := err.(*contract.Refusal); ok && namesNothing(refusal.Name) {
			h.refuseFor(x, req, name, head)
			return nil, false
		}
		if _, ok := err.(*contract.Refusal); ok {
			return nil, true
		}
		h.failed(x, req, err)
		return nil, false
	}
	if entity == nil || entity.Kind != want {
		h.refuseFor(x, req, name, head)
		return nil, false
	}
	return entity, true
}

// namesNothing reports whether a refusal says a reference named nothing,
// which the path's prefix then answers in its own kind's name.
func namesNothing(name string) bool {
	switch name {
	case contract.UnknownCard, contract.UnknownColumn, contract.UnknownPath, contract.UnknownWorkstream:
		return true
	}
	return false
}

// answer runs a built request and writes the answer.
func (h *head) answer(x *exchange, command string, req *verb.Request, success int) {
	h.write(x, h.execute(x, command, req), success)
}

// pathHead is the path's first variable, a card or a column reference.
func pathHead(x *exchange) string {
	if x.route.binds == pathColumn {
		return x.r.PathValue("column")
	}
	return x.r.PathValue("card")
}

// missingRequired names the first required parameter of a command that
// neither the request nor the path supplied, and is empty when none is
// missing.
func missingRequired(command string, arguments map[string]any, bound map[string]string) string {
	for _, param := range verb.Params(command) {
		if !param.Required || param.Field == "" {
			continue
		}
		if _, given := arguments[param.Name]; given {
			continue
		}
		if _, given := bound[param.Name]; given {
			continue
		}
		return param.Name
	}
	return ""
}

// keysOf lists a map's keys.
func keysOf(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

// sortedNames lists the names of a set of values in order, so a request
// carrying several faults is answered for the same one on every run.
func sortedNames(values url.Values) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
