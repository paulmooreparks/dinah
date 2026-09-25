package httphead

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"

	"dinah/internal/answer"
	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// The four form members that stand for what an HTML form cannot send. The
// set is closed at four, and they are honoured on form bodies alone.
const (
	formMethod = "_method"
	formType   = "_type"
	formBasis  = "_basis"
	formActor  = "_actor"
)

// comparesBasis names the acts whose command compares a basis, which are the
// ones If-Match and _basis reach.
var comparesBasis = map[string]bool{
	verb.Move: true, verb.Block: true, verb.Unblock: true,
	verb.Claim: true, verb.Release: true, verb.Pull: true,
}

// body is what an act's request carried.
type body struct {
	// members are the members the act's command reads, a string or a
	// boolean each.
	members map[string]any
	// form marks a form body, whose values are all strings.
	form bool
	// reserved are the underscore members a form carried.
	reserved map[string]string
}

// act answers an unsafe request: it reads the body, applies the form tunnel,
// chooses the act, reads the identity and the basis, checks the members,
// and runs the command.
func (h *head) act(x *exchange) {
	if x.r.URL.RawQuery != "" {
		h.refuse(x, contract.Usage, firstQueryName(x.r.URL.RawQuery))
		return
	}
	sent, ok := h.readBody(x)
	if !ok {
		return
	}
	chosen, ok := h.chooseAct(x, sent)
	if !ok {
		return
	}
	x.verb = chosen.command
	actor, ok := h.actorOf(x, sent)
	if !ok {
		return
	}
	basis, ok := h.basisOf(x, sent, chosen.command)
	if !ok {
		return
	}
	bound := map[string]string{}
	if x.route.binds == pathCard || x.route.binds == pathColumn {
		bound["card"] = pathHead(x)
	}
	arguments, ok := h.membersOf(x, sent, chosen.command, bound)
	if !ok {
		return
	}
	req := answer.Build(chosen.command, arguments)
	h.identify(x, req, actor)
	req.Basis = basis
	if !h.open(x, req) {
		return
	}
	if len(bound) > 0 {
		entity, ok := h.resolveKind(x, req, x.route.binds, pathHead(x))
		if !ok {
			return
		}
		req.Card = pathHead(x)
		if entity != nil {
			req.Card = entity.Ref
		}
	}
	if strings.HasSuffix(x.route.pattern, "{rest...}") {
		target, ok := h.commentTarget(x, req)
		if !ok {
			return
		}
		req.Card = target
	}
	payload, response := h.execute(x, chosen.command, req)
	success := http.StatusOK
	if chosen.created && !(chosen.command == verb.Pull && req.NoClaim) {
		success = http.StatusCreated
	}
	if response != nil && response.Outcome == contract.OutcomeOK {
		if location := h.locationOf(x, chosen.command, req, response); location != "" {
			x.w.Header().Set("Location", location)
		} else {
			success = http.StatusOK
		}
	}
	h.writePayload(x, payload, response, success)
}

// readBody reads an act's body: a form, a JSON object, or nothing.
func (h *head) readBody(x *exchange) (*body, bool) {
	sent := &body{members: map[string]any{}, reserved: map[string]string{}}
	mediaType := ""
	if raw := x.r.Header.Get("Content-Type"); raw != "" {
		mediaType, _ = parseMediaType(raw)
	}
	switch {
	case mediaType == typeForm:
		sent.form = true
		if err := x.r.ParseForm(); err != nil {
			h.bodyFailed(x, err)
			return nil, false
		}
		// Every name is checked for repetition before any is read, so a
		// member given twice is refused rather than resolved to its first
		// value, which is what Form.Get would do.
		for _, name := range sortedNames(x.r.PostForm) {
			if len(x.r.PostForm[name]) != 1 {
				h.refuse(x, contract.Usage, name)
				return nil, false
			}
		}
		for name, values := range x.r.PostForm {
			switch name {
			case formMethod, formType, formBasis, formActor:
				sent.reserved[name] = values[0]
			default:
				sent.members[name] = values[0]
			}
		}
	case mediaType != "":
		data, err := io.ReadAll(x.r.Body)
		if err != nil {
			h.bodyFailed(x, err)
			return nil, false
		}
		if len(bytes.TrimSpace(data)) == 0 {
			return sent, true
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err := decoder.Decode(&sent.members); err != nil || sent.members == nil {
			h.refuse(x, contract.Usage, "body")
			return nil, false
		}
		if _, err := decoder.Token(); err != io.EOF {
			h.refuse(x, contract.Usage, "body")
			return nil, false
		}
		// Decoding into a map keeps a repeated member's last value, so the
		// repetition is looked for separately and refused as the form path
		// refuses it.
		if name, repeated := repeatedMember(data); repeated {
			h.refuse(x, contract.Usage, name)
			return nil, false
		}
	}
	return sent, true
}

// repeatedMember names the first member a JSON object carries more than
// once. It reads only the object's own members, so a name repeated inside a
// nested value is not a repetition here, and it reports false for text that
// is not an object, which the caller has already refused.
func repeatedMember(data []byte) (string, bool) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if token, err := decoder.Token(); err != nil || token != json.Token(json.Delim('{')) {
		return "", false
	}
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return "", false
		}
		name, isName := key.(string)
		if !isName {
			return "", false
		}
		if seen[name] {
			return name, true
		}
		seen[name] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return "", false
		}
	}
	return "", false
}

// bodyFailed answers a body that could not be read: 413 at the size limit,
// and 400 for anything else.
func (h *head) bodyFailed(x *exchange, err error) {
	if tooLarge(err) {
		h.refuse(x, contract.BodyTooLarge, "")
		return
	}
	h.refuse(x, contract.Usage, "body")
}

// chooseAct applies the form tunnel and picks the act the request performs.
func (h *head) chooseAct(x *exchange, sent *body) (*act, bool) {
	if sent.form {
		switch tunnelled, named := sent.reserved[formMethod]; {
		case !named:
		case tunnelled == http.MethodPatch || tunnelled == http.MethodDelete:
			x.method = tunnelled
		default:
			h.refuse(x, contract.Usage, formMethod)
			return nil, false
		}
		if _, named := sent.reserved[formType]; named && x.method != http.MethodPatch {
			h.refuse(x, contract.Usage, formType)
			return nil, false
		}
	}
	m := x.route.method(x.method)
	if m == nil || (m.tunnel && x.method == x.r.Method) || len(m.acts) == 0 {
		h.notAllowed(x)
		return nil, false
	}
	if len(m.acts) == 1 {
		return &m.acts[0], true
	}
	selector, _ := parseMediaType(x.r.Header.Get("Content-Type"))
	if sent.form {
		named := false
		selector, named = sent.reserved[formType]
		if !named {
			h.refuse(x, contract.Usage, formType)
			return nil, false
		}
	}
	for i := range m.acts {
		if m.acts[i].selectedBy == selector {
			return &m.acts[i], true
		}
	}
	h.refuse(x, contract.Usage, formType)
	return nil, false
}

// actorOf reads who the request acts as: Dinah-Actor, then a form's _actor.
// The two naming different actors answer 400, because the request has not
// said which it means. An empty result leaves the process default to
// identify.
func (h *head) actorOf(x *exchange, sent *body) (string, bool) {
	header := strings.TrimSpace(x.r.Header.Get("Dinah-Actor"))
	member, named := sent.reserved[formActor]
	member = strings.TrimSpace(member)
	if named && header != "" && member != header {
		h.refuse(x, contract.Usage, formActor)
		return "", false
	}
	if header != "" {
		return header, true
	}
	return member, true
}

// basisOf reads the basis an act is evaluated against, from If-Match and a
// form's _basis, under the rules of sections 7.3 and 10 of the
// specification. A basis of * is no basis, chosen explicitly.
func (h *head) basisOf(x *exchange, sent *body, command string) (string, bool) {
	header, hasHeader, valid := ifMatch(x.r)
	if hasHeader && !valid {
		h.refuse(x, contract.Usage, "If-Match")
		return "", false
	}
	member, hasMember := sent.reserved[formBasis]
	if !comparesBasis[command] {
		if hasHeader {
			h.refuse(x, contract.Usage, "If-Match")
			return "", false
		}
		if hasMember {
			h.refuse(x, contract.Usage, formBasis)
			return "", false
		}
		return "", true
	}
	if hasMember && member == "" {
		h.refuse(x, contract.Usage, formBasis)
		return "", false
	}
	if hasHeader && hasMember && header != member {
		h.refuse(x, contract.Usage, formBasis)
		return "", false
	}
	if x.method == http.MethodPatch && !hasHeader && !hasMember {
		h.refuse(x, contract.BasisRequired, "If-Match")
		return "", false
	}
	basis := header
	if !hasHeader {
		basis = member
	}
	if basis == "*" {
		return "", true
	}
	return basis, true
}

// ifMatch reads an If-Match header: whether one was sent, and whether it
// holds exactly one strong entity-tag or *. The tag is returned unquoted.
func ifMatch(r *http.Request) (string, bool, bool) {
	values := r.Header.Values("If-Match")
	if len(values) == 0 {
		return "", false, false
	}
	if len(values) > 1 {
		return "", true, false
	}
	value := strings.TrimSpace(values[0])
	if value == "*" {
		return "*", true, true
	}
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", true, false
	}
	inner := value[1 : len(value)-1]
	if inner == "" || strings.ContainsAny(inner, "\"") {
		return "", true, false
	}
	for _, c := range inner {
		if c < 0x21 || c == 0x7f {
			return "", true, false
		}
	}
	return inner, true, true
}

// membersOf checks an act's members against what its command publishes on
// this route, converts each to the type the parameter takes, and answers 400
// for a member the route does not publish, a value of the wrong type, or a
// required member left out.
func (h *head) membersOf(x *exchange, sent *body, command string, bound map[string]string) (map[string]any, bool) {
	members := published(command, keysOf(bound)...)
	names := make([]string, 0, len(sent.members))
	for name := range sent.members {
		names = append(names, name)
	}
	sort.Strings(names)
	arguments := map[string]any{}
	for _, name := range names {
		param, ok := members[name]
		if !ok {
			h.refuse(x, contract.Usage, name)
			return nil, false
		}
		value, ok := memberValue(param, sent.members[name], sent.form)
		if !ok {
			h.refuse(x, contract.Usage, name)
			return nil, false
		}
		arguments[name] = value
	}
	if missing := missingRequired(command, arguments, bound); missing != "" {
		h.refuse(x, contract.Usage, missing)
		return nil, false
	}
	return arguments, true
}

// memberValue converts one member to the type its parameter takes: a
// string for a valued parameter and a boolean for a marker. In a form a
// marker is true for true and on, which a checkbox with no value attribute
// sends, and false for false.
func memberValue(param verb.Param, value any, form bool) (any, bool) {
	if !form {
		if param.Marker {
			flag, ok := value.(bool)
			return flag, ok
		}
		text, ok := value.(string)
		return text, ok
	}
	text, _ := value.(string)
	if !param.Marker {
		return text, true
	}
	switch text {
	case "true", "on":
		return true, true
	case "false":
		return false, true
	}
	return nil, false
}

// commentTarget checks that a POST below a card or a column names a
// comments collection, and returns the reference of what the comment is
// on. Any other reference takes no POST.
func (h *head) commentTarget(x *exchange, req *verb.Request) (string, bool) {
	_, ref, _ := refForPath(x.r.URL.Path)
	_, collection, err := x.library.Bench.ResolveReferenceIn(bench.LiveHalf, ref)
	if err != nil {
		h.failed(x, req, err)
		return "", false
	}
	if collection == nil || collection.Mount.Kind != bench.KindComment {
		x.w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
		h.refuseFor(x, req, contract.MethodNotAllowed, x.r.Method)
		return "", false
	}
	cut := strings.LastIndex(ref, "/")
	return ref[:cut], true
}

// locationOf composes the Location an act that created something answers
// with, from the canonical reference the response carries.
func (h *head) locationOf(x *exchange, command string, req *verb.Request, response *verb.Response) string {
	switch command {
	case "add":
		if response.Card != nil {
			return pathForRef(pathCard, response.Card.Ref)
		}
	case verb.Claim:
		if response.Card != nil {
			return pathForRef(pathCard, response.Card.Ref) + "/claim"
		}
	case verb.Pull:
		if response.Card != nil && !req.NoClaim {
			return pathForRef(pathCard, response.Card.Ref) + "/claim"
		}
	case "comment":
		if response.Detail != "" {
			return pathForRef(x.route.binds, response.Detail)
		}
	}
	return ""
}

// identify sets who a request acts as and what performs it: the actor from
// the request or the process default, and each of the four declared facts
// from its header, falling back member by member to the process's own. An
// empty or whitespace header takes the fallback.
func (h *head) identify(x *exchange, req *verb.Request, actor string) {
	if actor == "" {
		actor = strings.TrimSpace(x.r.Header.Get("Dinah-Actor"))
	}
	if actor == "" {
		actor = h.cfg.DefaultActor
	}
	req.Actor = actor
	req.Harness = declaredOr(x.r, "Dinah-Harness", h.cfg.Agent.Harness)
	req.Provider = declaredOr(x.r, "Dinah-Provider", h.cfg.Agent.Provider)
	req.Model = declaredOr(x.r, "Dinah-Model", h.cfg.Agent.Model)
	req.Server = declaredOr(x.r, "Dinah-Server", h.cfg.Agent.Server)
}

// declaredOr reads one declared fact off a request header, falling back to
// the process's own when the header is absent, empty or whitespace.
func declaredOr(r *http.Request, name, fallback string) string {
	if value := strings.TrimSpace(r.Header.Get(name)); value != "" {
		return value
	}
	return fallback
}

// firstQueryName names the first parameter of a raw query string, for the
// refusal of a query string sent to an act.
func firstQueryName(raw string) string {
	first, _, _ := strings.Cut(raw, "&")
	name, _, _ := strings.Cut(first, "=")
	return name
}
