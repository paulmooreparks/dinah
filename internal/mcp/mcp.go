// Package mcp is the second head: one bench served over MCP on stdio.
//
// It is a projection and nothing else. Every tool call becomes the same
// library request the cli head makes, and the canonical answer travels back
// unrendered, so an agent on this surface and a person at a terminal are
// answered by one implementation. Language settings reach the cli head's
// rendering and reach nothing here, because CORE-TEXT-3 keeps a machine
// surface on canonical tokens whatever language is asked for.
package mcp

import (
	"bufio"
	"encoding/json"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// Version is the MCP protocol revision this head speaks.
const Version = "2024-11-05"

// request is one JSON-RPC 2.0 request read from stdin.
type request struct {
	// JSONRPC is the protocol version, always 2.0.
	JSONRPC string `json:"jsonrpc"`
	// ID is the caller's correlation value, absent on a notification.
	ID json.RawMessage `json:"id,omitempty"`
	// Method is the method being called.
	Method string `json:"method"`
	// Params are the method's arguments.
	Params json.RawMessage `json:"params,omitempty"`
}

// response is one JSON-RPC 2.0 response written to stdout.
type response struct {
	// JSONRPC is the protocol version, always 2.0.
	JSONRPC string `json:"jsonrpc"`
	// ID echoes the request's correlation value.
	ID json.RawMessage `json:"id,omitempty"`
	// Result is the answer to a call that was understood.
	Result any `json:"result,omitempty"`
	// Error is the answer to one that was not.
	Error *rpcError `json:"error,omitempty"`
}

// rpcError is a transport-level failure. A refusal is not one of these: a
// refusal is a legitimate answer the contract defines, and it travels in the
// result as the canonical response it is.
type rpcError struct {
	// Code is the JSON-RPC error code.
	Code int `json:"code"`
	// Message says what went wrong with the call itself.
	Message string `json:"message"`
}

// The JSON-RPC error codes this head reports.
const (
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// Serve reads line-delimited JSON-RPC from a reader and answers on a writer
// until the reader closes.
//
// The transport is the standard library's: encoding/json over bufio, which
// covers the whole of what this head needs, so the module keeps its record of
// no external dependencies.
//
// root is the directory the server is bound to, defaultLib is the workbench
// resolved at startup and may be nil when discovery did not resolve one, and
// libraries is the map every per-call dispatch reads against. Serve owns the
// map and the entries it opens, the way it owns the single library today.
func Serve(root string, defaultLib *verb.Library, libraries map[string]*verb.Library, in io.Reader, out io.Writer) error {
	reader := bufio.NewScanner(in)
	reader.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	encoder := json.NewEncoder(out)
	for reader.Scan() {
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}
		answer := dispatch(root, defaultLib, libraries, &req)
		if answer == nil {
			continue
		}
		if err := encoder.Encode(answer); err != nil {
			return err
		}
	}
	return reader.Err()
}

// dispatch answers one request, or returns nil for a notification, which
// carries no identifier and wants no answer.
func dispatch(root string, defaultLib *verb.Library, libraries map[string]*verb.Library, req *request) *response {
	if len(req.ID) == 0 {
		return nil
	}
	answer := &response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		answer.Result = initializeResult(root, defaultLib)
	case "tools/list":
		answer.Result = map[string]any{"tools": toolList()}
	case "tools/call":
		result, err := call(root, defaultLib, libraries, req.Params)
		if err != nil {
			answer.Error = &rpcError{Code: codeInvalidParams, Message: err.Error()}
			return answer
		}
		answer.Result = result
	case "resources/list":
		answer.Result = map[string]any{"resources": resourceList()}
	case "resources/read":
		result, err := readResource(req.Params)
		if err != nil {
			answer.Error = &rpcError{Code: codeInvalidParams, Message: err.Error()}
			return answer
		}
		answer.Result = result
	case "ping":
		answer.Result = map[string]any{}
	default:
		answer.Error = &rpcError{Code: codeMethodNotFound, Message: req.Method}
	}
	return answer
}

// initializeResult carries the working agreement and the orientation an agent
// needs before its first tool call, served live from the binary and never
// seeded into the bench.
func initializeResult(root string, defaultLib *verb.Library) map[string]any {
	result := map[string]any{
		"protocolVersion": Version,
		"capabilities": map[string]any{
			"tools":     map[string]any{},
			"resources": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "dinah",
			"version": verb.ToolRelease,
		},
		"instructions": workingAgreement(root, defaultLib),
	}
	return result
}

// workingAgreement is section 8 of the profile: the four rules that bind an
// owner rather than a tool, and the bench this process serves. The default
// library may be nil, when discovery did not resolve one; the rules still bind
// the agent and the bench the call names is still the one the agent reads
// about.
func workingAgreement(root string, defaultLib *verb.Library) string {
	var b strings.Builder
	catalog := msg.For(msg.Base)
	if defaultLib != nil {
		b.WriteString("You are working the workbench " + defaultLib.Bench.Title + ".\n\n")
	}
	b.WriteString("The working agreement, which binds you rather than the tool:\n")
	b.WriteString("1. Claim a card before producing work on it.\n")
	b.WriteString("2. Do not hold a claim on a card you have stopped working.\n")
	b.WriteString("3. Treat the workbench as the authority for where a card stands and who holds it.\n")
	b.WriteString("4. Do not move a card out of an operator-owned state unless you are the operator.\n\n")
	b.WriteString("Every response carries an affordances member naming what you may do next. ")
	b.WriteString("A successful claim or move carries the instructions of the position in three ")
	b.WriteString("separate layers and the moves the flow allows. Tokens are canonical on this ")
	b.WriteString("surface and are never translated.\n\n")
	if defaultLib != nil {
		b.WriteString(catalog.T("mcp.reach", "root", root, "title", defaultLib.Bench.Title))
		b.WriteString("\n\n")
		b.WriteString("The operator of this workbench is " + defaultLib.Bench.Operator + ". ")
	} else {
		b.WriteString(catalog.T("mcp.reach.nodefault", "root", root))
		b.WriteString("\n\n")
	}
	b.WriteString("Read the embedded guides through resources/list and resources/read.\n")
	b.WriteString("Read the guide mcp through resources/read for the loop this surface expects.\n")
	return b.String()
}

// resourceList offers the embedded guides. They are resources here rather
// than a tool, because a guide is something an agent reads rather than an act
// it takes.
func resourceList() []map[string]any {
	var resources []map[string]any
	for _, topic := range guide.Topics() {
		resource := map[string]any{
			"uri":      "dinah://guide/" + topic,
			"name":     guide.Title(topic),
			"mimeType": "text/markdown",
		}
		resources = append(resources, resource)
	}
	return resources
}

// readResource serves one guide by URI, byte-identical to what the cli head
// prints for the same topic.
func readResource(params json.RawMessage) (map[string]any, error) {
	var args struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	topic := strings.TrimPrefix(args.URI, "dinah://guide/")
	text, err := guide.Text(topic)
	if err != nil {
		return nil, err
	}
	content := map[string]any{
		"uri":      args.URI,
		"mimeType": "text/markdown",
		"text":     text,
	}
	return map[string]any{"contents": []map[string]any{content}}, nil
}

// call runs one tool and returns its canonical answer. The workbench
// argument is read off the tool call here, because this is the one site that
// has to know which library answers; the workbench value never reaches the
// library, since `workbench` tells the head rather than the library which
// implementation of the verb runs.
//
// The workbenches tool acts on the root rather than on a library, so it is
// answered ahead of the table lookup that leads to `run`, and the refusal it
// carries when no workbench is reachable is the same one the override branch
// of bench.DiscoverSource raises. Every other tool gets the per-call
// resolution the spec describes: a missing or empty argument falls through to
// the default, the value is resolved to an absolute path, the containment
// check refuses anything outside the root, a missing workbench.md refuses,
// and `bench.Open` runs last so its own refusals travel unchanged.
func call(root string, defaultLib *verb.Library, libraries map[string]*verb.Library, params json.RawMessage) (map[string]any, error) {
	var args struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	if args.Name == "workbenches" {
		return answerWorkbenches(root)
	}
	tool, ok := toolsByName[args.Name]
	if !ok {
		return nil, contract.Refuse(contract.UnknownVerb, args.Name)
	}
	request := request2Args(tool.command, args.Arguments)
	candidate, _ := args.Arguments["workbench"].(string)
	library, refusal := resolveLibrary(root, defaultLib, libraries, request, candidate)
	if refusal != nil {
		return answerRefusal(request, refusal), nil
	}
	payload := tool.run(library, request)
	if response, ok := payload.(*verb.Response); ok {
		response.Affordances = surfaceAffordances(response.Affordances)
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(encoded)}},
	}
	return result, nil
}

// answerWorkbenches serves the workbenches tool: bench.Enumerate walks the
// root, the rows are sorted by path, and the answer carries the same shape
// every tool's payload carries. Enumerate refuses with NoWorkbenchFound when
// its cache lookup fails, which is what an MCP caller would otherwise see as
// a transport error; translating here keeps the refusal on the response side.
func answerWorkbenches(root string) (map[string]any, error) {
	listed, err := bench.Enumerate(root)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(listed, func(i, j int) bool {
		return listed[i].Path < listed[j].Path
	})
	payload := wrap(map[string]any{"workbenches": listed}, []string{"status", "states", "list_cards", "next_card", "workbenches"})
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(encoded)}},
	}, nil
}

// resolveLibrary carries out the per-call workbench resolution. It returns
// either a library and a nil refusal, or a nil library and the refusal that
// comes back. The six steps the spec numbers are the order of the ifs below,
// since each step names the refusal that came back when no later step ran.
func resolveLibrary(root string, defaultLib *verb.Library, libraries map[string]*verb.Library, request *verb.Request, candidate string) (*verb.Library, *contract.Refusal) {
	if candidate == "" {
		if defaultLib == nil {
			return nil, contract.Refuse(contract.NoWorkbenchFound, "")
		}
		return defaultLib, nil
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return nil, contract.Refuse(contract.NoWorkbench, candidate)
	}
	contained, err := bench.PathUnderRoot(root, abs)
	if err != nil {
		return nil, contract.RefuseWith(contract.OutsideRoot, abs, map[string]string{"root": root})
	}
	if !contained {
		return nil, contract.RefuseWith(contract.OutsideRoot, abs, map[string]string{"root": root})
	}
	if !bench.Exists(filepath.Join(abs, bench.WorkbenchAnchor)) {
		return nil, contract.Refuse(contract.NoWorkbench, abs)
	}
	if libraries != nil {
		if lib, ok := libraries[abs]; ok {
			return lib, nil
		}
	}
	opened, err := bench.Open(abs)
	if err != nil {
		if refusal, ok := err.(*contract.Refusal); ok {
			return nil, refusal
		}
		return nil, contract.Refuse(contract.NoWorkbench, abs)
	}
	// A nil defaultLib is the no-discovery case the spec numbers as step 1 of
	// resolveLibrary. The Home the verb library carries is empty then, which
	// is the same shape an agent running the cli head against the per-call
	// target sees: the library carries the bench it answered; it does not
	// carry a separate home that has to be inherited.
	var home string
	if defaultLib != nil {
		home = defaultLib.Home
	}
	library := verb.New(opened, home)
	if libraries != nil {
		libraries[abs] = library
	}
	return library, nil
}

// commandTool maps a library command name to the tool an agent calls on this
// surface. Most affordances keep the same spelling in both vocabularies; the
// two reads the surface names in full form are the exceptions, and a refusal
// that carried the library's short forms would point an agent at a tool this
// surface does not serve.
var commandTool = map[string]string{
	"ls":   "list_cards",
	"next": "next_card",
}

// surfaceAffordances translates a response's affordances from the library's
// command spellings to the tool names an agent can actually call on this
// surface. The read path already serves the tool names; the refusal path
// inherits the library's no-card default set, which still spells the two
// reads as commands. Translating here keeps the two vocabularies from
// disagreeing, which is the promise the mcp guide makes when it says that
// following the affordances cannot dead-end. Reads never return a
// *verb.Response, so applying the translation to every such payload touches
// refusals alone and is safe to run twice.
func surfaceAffordances(affordances []string) []string {
	translated := make([]string, 0, len(affordances))
	for _, affordance := range affordances {
		if name, ok := commandTool[affordance]; ok {
			translated = append(translated, name)
		} else {
			translated = append(translated, affordance)
		}
	}
	return translated
}

// answerRefusal wraps a refused response in the content envelope every
// payload travels through, so the MCP client reads it as a tool answer rather
// than as a transport error.
func answerRefusal(request *verb.Request, refusal *contract.Refusal) map[string]any {
	response := verb.ComposeRefusal(request, refusal)
	response.Affordances = surfaceAffordances(response.Affordances)
	encoded, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
		}
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(encoded)}},
	}
}

// request2Args builds a library request from the tool call's arguments, using
// the same parameter list the cli head composes its syntax from.
func request2Args(command string, arguments map[string]any) *verb.Request {
	req := &verb.Request{Verb: command}
	for _, param := range verb.Params(command) {
		value, ok := arguments[param.Name]
		if !ok {
			continue
		}
		if param.Marker {
			flag, _ := value.(bool)
			assignMarker(req, param.Name, flag)
			continue
		}
		text, _ := value.(string)
		assignValue(req, param.Name, text)
	}
	if actor, ok := arguments["actor"].(string); ok {
		req.Actor = actor
	}
	if basis, ok := arguments["basis"].(string); ok {
		req.Basis = basis
	}
	return req
}

// assignValue puts one named string argument on the request.
func assignValue(req *verb.Request, name, value string) {
	switch name {
	case "card":
		req.Card = value
	case "ref":
		req.Ref = value
	case "state":
		req.State = value
	case "query":
		req.Query = value
	case "group-by":
		req.GroupBy = value
	case "depth":
		req.Depth = value
	case "action":
		req.Action = value
	case "field":
		req.Field = value
	case "workstream":
		req.Workstream = value
	case "value":
		req.Value = value
	case "title":
		req.Title = value
	case "text":
		req.Text = value
	case "reason":
		req.Reason = value
	case "kind":
		req.Kind = value
	case "file":
		req.File = value
	case "expires":
		if parsed, err := verb.ParseDuration(value); err == nil {
			req.Expires = parsed
		}
	}
}

// assignMarker puts one named flag on the request.
func assignMarker(req *verb.Request, name string, value bool) {
	switch name {
	case "override":
		req.Override = value
	case "replace":
		req.Replace = value
	case "yes":
		req.Confirm = value
	case "ready":
		req.ReadyOnly = value
	case "finish":
		req.Finish = value
	case "migrate-ordinals":
		req.MigrateOrdinals = value
	case "migrate-slugs":
		req.MigrateSlugs = value
	case "migrate-states":
		req.MigrateStates = value
	case "migrate-workstreams":
		req.MigrateWorkstreams = value
	case "no-claim":
		req.NoClaim = value
	case "catalogs":
		// The version tool always reports catalog coverage, so the marker
		// carries nothing here.
	}
}

// journalView is a card's history as this head returns it, which is the
// journal's own records with nothing resolved against the bench as it stands.
type journalView struct {
	// Events are the recorded acts in the order they were recorded.
	Events []bench.Event `json:"events"`
	// Affordances name what the caller may do next.
	Affordances []string `json:"affordances"`
}
