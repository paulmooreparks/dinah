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
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
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
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// Serve reads line-delimited JSON-RPC from a reader and answers on a writer
// until the reader closes. A line json.Unmarshal cannot turn into a request is
// answered with a JSON-RPC error of its own rather than dropped, so a caller
// that sent one is told, and scanning continues to the next line either way.
//
// A line carrying no bytes at all is the single exception and is skipped in
// silence, because a stream ending in a newline produces one and there is
// nothing on it to answer. A line carrying only spaces or a tab is not that
// case: it carries bytes the head cannot use, so it draws the same error any
// other unusable line draws.
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
		line := reader.Text()
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if err := encoder.Encode(malformedLineResponse(err)); err != nil {
				return err
			}
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

// malformedLineResponse builds the response for one line of stdin that
// json.Unmarshal could not turn into a request. A *json.SyntaxError means the
// line was not valid JSON at all, which is JSON-RPC 2.0 section 5.1's -32700
// "Parse error"; anything else json.Unmarshal returns here means the line was
// valid JSON but not shaped like a Request object, which is the same section's
// -32600 "Invalid Request". The identifier is the JSON literal null, per the
// specification's Response object rule for both of these cases, rather than
// the zero json.RawMessage the field would otherwise carry, which its own
// omitempty tag would drop from the encoded response entirely.
func malformedLineResponse(err error) *response {
	code := codeInvalidRequest
	if _, ok := err.(*json.SyntaxError); ok {
		code = codeParseError
	}
	return &response{
		JSONRPC: "2.0",
		ID:      json.RawMessage("null"),
		Error:   &rpcError{Code: code, Message: err.Error()},
	}
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
	b.WriteString("4. Do not move a card out of an operator-owned column unless you are the operator.\n\n")
	b.WriteString("Every response carries an affordances member naming what you may do next. ")
	b.WriteString("A successful claim or move carries the instructions of the position in three ")
	b.WriteString("separate layers and the moves the flow allows. Tokens are canonical on this ")
	b.WriteString("surface and are never translated.\n\n")
	switch {
	case defaultLib != nil && root != "":
		b.WriteString(catalog.T("mcp.reach", "root", root, "title", defaultLib.Bench.Title))
	case defaultLib != nil && root == "":
		b.WriteString(catalog.T("mcp.reach.unbounded", "title", defaultLib.Bench.Title))
	case defaultLib == nil && root != "":
		b.WriteString(catalog.T("mcp.reach.nodefault", "root", root))
	default:
		b.WriteString(catalog.T("mcp.reach.nodefault.unbounded"))
	}
	b.WriteString("\n\n")
	if defaultLib != nil {
		b.WriteString("The operator of this workbench is " + defaultLib.Bench.Operator + ". ")
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
//
// Both dispatch paths check the call's argument names against what the tool
// declares before they act on any of them, so an argument this surface never
// published is refused rather than read past.
func call(root string, defaultLib *verb.Library, libraries map[string]*verb.Library, params json.RawMessage) (map[string]any, error) {
	var args struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	if args.Name == "workbenches" {
		if err := checkArguments(toolsByName["workbenches"], args.Arguments); err != nil {
			return nil, err
		}
		return answerWorkbenches(root, defaultLib, args.Arguments)
	}
	tool, ok := toolsByName[args.Name]
	if !ok {
		return nil, contract.Refuse(contract.UnknownVerb, args.Name)
	}
	if err := checkArguments(tool, args.Arguments); err != nil {
		return nil, err
	}
	request := request2Args(tool.command, args.Arguments)
	// A root argument makes this a root-scoped read, which answers about every
	// workbench beneath a directory rather than about one. It is checked ahead
	// of resolveLibrary because no single workbench is being resolved: the walk
	// opens each one it finds, and the containment check that would have run on
	// a workbench argument runs on the root instead.
	if builder, scoped := rootScoped[args.Name]; scoped {
		if strings.TrimSpace(request.Root) != "" {
			return answerForest(root, args.Name, defaultLib, request, builder)
		}
		// A depth bound with no root to bound would be read and dropped, which
		// the terminal refuses. The two heads answer one question, so a caller
		// who names one here is told the same thing.
		if strings.TrimSpace(request.MaxDepth) != "" {
			return answerRefusal(request, contract.Refuse(contract.DepthWithoutRoot, request.MaxDepth)), nil
		}
	}
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

// rootScoped names every tool whose read grows a root argument, mapped to the
// builder that answers it for every workbench beneath that root.
//
// It is a table rather than a switch because the question each entry answers is
// the same question ("does this call carry a root") and the containment
// discipline around it is identical; only the builder differs. A tool absent
// from this table has no root-scoped form, and a root argument sent to one is
// not advertised by its schema and never reaches here.
var rootScoped = map[string]func(root, home string, req *verb.Request) (any, error){
	"tree": func(root, home string, req *verb.Request) (any, error) {
		level := req.Depth
		if level == "" {
			level = verb.LevelCards
		}
		return verb.TreeForest(root, home, req, verb.ParseChain(req.GroupBy), level, walkDepth(req))
	},
	"status": func(root, home string, req *verb.Request) (any, error) {
		return verb.StatusForest(root, home, req, walkDepth(req))
	},
	"list_cards": func(root, home string, req *verb.Request) (any, error) {
		return verb.ListForest(root, home, req, walkDepth(req))
	},
	"next_card": func(root, home string, req *verb.Request) (any, error) {
		return verb.NextForest(root, home, req, walkDepth(req))
	},
	"changes": func(root, home string, req *verb.Request) (any, error) {
		return verb.ChangesForest(root, home, req, walkDepth(req))
	},
	"search_cards": func(root, home string, req *verb.Request) (any, error) {
		return verb.SearchForest(root, home, req, walkDepth(req))
	},
}

// forestMember is the payload key each root-scoped answer is published under.
// One distinct key per verb, so a client dispatching on the response shape
// never has to guess which root-scoped answer the envelope carried.
var forestMember = map[string]string{
	"tree":         "forest",
	"status":       "root_status",
	"list_cards":   "root_listing",
	"next_card":    "root_offers",
	"changes":      "root_changes",
	"search_cards": "root_search",
}

// walkDepth reads the depth bound off a request, falling back to the surface's
// own default when the caller named none. A value that is not a count of rungs
// is refused rather than silently defaulted, which is what keeps a caller who
// mistyped it from being told they got what they asked for.
func walkDepth(req *verb.Request) int {
	depth, _ := parseWalkDepth(req.MaxDepth)
	return depth
}

// parseWalkDepth returns the depth bound a request names and the refusal a
// malformed one earns. The two are split from walkDepth so the dispatch can
// refuse before any walk begins while the builders take a plain int.
func parseWalkDepth(named string) (int, *contract.Refusal) {
	trimmed := strings.TrimSpace(named)
	if trimmed == "" {
		return bench.DefaultEnumerateDepth, nil
	}
	rungs, err := strconv.Atoi(trimmed)
	if err != nil || rungs < 0 {
		return bench.DefaultEnumerateDepth, contract.Refuse(contract.MalformedDepth, named)
	}
	return rungs, nil
}

// answerForest serves one root-scoped read.
//
// The root argument walks the server's own address space downward, so it gets
// the containment check resolveLibrary already applies to the workbench
// argument: the same bench.PathUnderRoot call, the same contract.OutsideRoot
// refusal naming the configured root, so an escape is refused by one name
// wherever a path argument can reach for one.
//
// home comes off the default library when there is one and is empty otherwise,
// which is resolveLibrary's own rule for a server started with no discovery.
func answerForest(root, tool string, defaultLib *verb.Library, request *verb.Request, build func(root, home string, req *verb.Request) (any, error)) (map[string]any, error) {
	abs, err := filepath.Abs(strings.TrimSpace(request.Root))
	if err != nil {
		return answerRefusal(request, contract.Refuse(contract.UnknownRoot, request.Root)), nil
	}
	contained, err := bench.PathUnderRoot(root, abs)
	if err != nil || !contained {
		refusal := contract.RefuseWith(contract.OutsideRoot, abs, map[string]string{"root": root})
		return answerRefusal(request, refusal), nil
	}
	if _, refusal := parseWalkDepth(request.MaxDepth); refusal != nil {
		return answerRefusal(request, refusal), nil
	}
	var home string
	if defaultLib != nil {
		home = defaultLib.Home
	}
	answer, err := build(abs, home, request)
	if err != nil {
		if refusal, ok := err.(*contract.Refusal); ok {
			return answerRefusal(request, refusal), nil
		}
		return nil, err
	}
	member, named := forestMember[tool]
	if !named {
		// A tool in the dispatch table and not in the member table would
		// publish its answer under the empty key, which is a shape no client
		// can dispatch on and which reads as a missing answer rather than as a
		// misnamed one. The two tables are paired by a test; this is what the
		// call does if that pairing is ever broken at run time.
		return nil, contract.Refuse(contract.UnknownVerb, tool)
	}
	payload := wrap(map[string]any{member: answer}, readAffordances)
	return textResult(payload)
}

// answerWorkbenches serves the workbenches tool.
//
// With no path argument it keeps its existing contract exactly, down to how
// its refusals travel: bench.Enumerate walks the server's own configured root
// and whatever comes back reaches the caller as it did before this argument
// existed. An unbounded server has no directory to search, and the refusal
// that says so is a transport error rather than a payload, which is dinah-307's
// contract and still holds wherever that server carries no default library.
//
// An unbounded server that did discover a default answers that one workbench
// rather than refusing (dinah-301). bench.Enumerate("") still cannot run a
// search, so what comes back is not a listing but the one identity the server
// already holds, and it is marked unbounded so a caller can tell the two
// apart. That branch sits inside the no-path arm alone, so a call carrying a
// path walks the path it names whether or not a default exists.
//
// With a non-empty path argument it switches to the same downward walk
// dinah workbenches <path> runs at a terminal: the path is resolved and checked
// against the root with bench.PathUnderRoot, refusing contract.OutsideRoot on
// escape, and bench.EnumerateDeep replaces bench.Enumerate. Those refusals are
// about an argument the caller sent, so they travel on the response the way
// every other argument-level refusal on this surface does, which is what
// resolveLibrary already does for a workbench argument that escapes the root.
func answerWorkbenches(root string, defaultLib *verb.Library, arguments map[string]any) (map[string]any, error) {
	named, _ := arguments["path"].(string)
	named = strings.TrimSpace(named)
	depth, _ := arguments["max-depth"].(string)
	if named == "" {
		// A depth bound with no path to bound would be read and dropped, and
		// the terminal refuses that rather than accepting an argument that
		// changes nothing. The two heads answer one question, so this one
		// refuses it too, and it is an argument-level refusal like the rest.
		if strings.TrimSpace(depth) != "" {
			return workbenchesRefusal(contract.Refuse(contract.DepthWithoutRoot, depth)), nil
		}
		if root == "" && defaultLib != nil {
			return workbenchesDefaultOnly(defaultLib)
		}
		listed, err := bench.Enumerate(root)
		if err != nil {
			return nil, err
		}
		return workbenchesPayload(listed)
	}
	listed, refusal := walkFromPath(root, named, depth)
	if refusal != nil {
		return workbenchesRefusal(refusal), nil
	}
	return workbenchesPayload(listed)
}

// walkFromPath runs the downward walk a path argument asks for, after the two
// checks that argument has to pass: it resolves to somewhere, and that
// somewhere lies under the root the server was started with.
func walkFromPath(root, named, depth string) ([]bench.Candidate, *contract.Refusal) {
	abs, err := filepath.Abs(named)
	if err != nil {
		return nil, contract.Refuse(contract.UnknownRoot, named)
	}
	contained, err := bench.PathUnderRoot(root, abs)
	if err != nil || !contained {
		return nil, contract.RefuseWith(contract.OutsideRoot, abs, map[string]string{"root": root})
	}
	rungs, refusal := parseWalkDepth(depth)
	if refusal != nil {
		return nil, refusal
	}
	walked, err := bench.EnumerateDeep(abs, rungs)
	if err != nil {
		if walkRefusal, ok := err.(*contract.Refusal); ok {
			return nil, walkRefusal
		}
		return nil, contract.Refuse(contract.UnknownRoot, abs)
	}
	return walked, nil
}

// workbenchesRefusal puts one argument-level refusal on the response, which is
// where this surface carries a refusal the caller can correct by sending
// different arguments.
func workbenchesRefusal(refusal *contract.Refusal) map[string]any {
	return answerRefusal(&verb.Request{Verb: "workbenches"}, refusal)
}

// workbenchesPayload renders a listing, sorted by path so the two modes and
// the two heads hand back one order.
func workbenchesPayload(listed []bench.Candidate) (map[string]any, error) {
	sort.SliceStable(listed, func(i, j int) bool {
		return listed[i].Path < listed[j].Path
	})
	payload := wrap(map[string]any{"workbenches": listed}, []string{"status", "columns", "list_cards", "next_card", "workbenches"})
	return textResult(payload)
}

// workbenchesDefaultOnly answers the workbenches tool for an unbounded server
// that discovered a default library. bench.Enumerate("") has no directory to
// search and cannot run, but the default is known without a search, and
// reporting nothing here would contradict every other tool call on the same
// session, which goes on answering against that same default (dinah-301). The
// row is built off the library already opened at startup rather than off a
// fresh anchor read, because it is the same anchor, and a second read of it
// tells the caller nothing bench.Open did not already confirm.
//
// The unbounded member marks this answer as something other than the outcome
// of a search. A caller that reads it knows the array may be incomplete, since
// a server with no root admits any workbench it can resolve and open
// (dinah-307) rather than only the one row here.
//
// The answer is composed here rather than through workbenchesPayload, which
// sorts a walk's result by path and answers a different question from this
// one.
func workbenchesDefaultOnly(defaultLib *verb.Library) (map[string]any, error) {
	row := bench.Candidate{
		Title: defaultLib.Bench.Title,
		Slug:  defaultLib.Bench.Slug,
		Path:  defaultLib.Bench.Root,
	}
	if bench.IsWorkbenchID(defaultLib.Bench.ID) {
		row.ID = defaultLib.Bench.ID
	}
	payload := wrap(map[string]any{
		"workbenches": []bench.Candidate{row},
		"unbounded":   true,
	}, []string{"status", "columns", "list_cards", "next_card", "workbenches"})
	return textResult(payload)
}

// textResult puts one payload into the shape every tool call answers with,
// which is a single text content block carrying the payload as indented JSON.
func textResult(payload map[string]any) (map[string]any, error) {
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
		if beneath, ok := bench.SoleBeneath(abs); ok {
			return nil, contract.RefuseWith(contract.NoWorkbench, abs, map[string]string{"found": beneath})
		}
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
// surface. The reads that write their own list already spell the tool names;
// the refusal path, and every read that asks the library instead, can carry
// the library's own spellings, which name the two reads as the commands ls
// and next. Translating here keeps the two vocabularies from disagreeing,
// which is the promise the mcp guide makes when it says that following the
// affordances cannot dead-end. Every spelling it does not remap passes
// through unchanged, so it is safe to run over an already-translated list.
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
		// A parameter the table binds to no request field is one no verb
		// reads, so it is dropped here rather than landing on whatever field
		// shares its name. workbenches names its depth the same way the
		// root-scoped reads do and answers it without a request at all, and
		// assigning it anyway made the declaration and the code disagree
		// about a value nothing goes on to read.
		if param.Field == "" {
			continue
		}
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

// unknownArgument is what call returns when a tools/call names an argument
// outside what the tool it named declares. It is a plain error, so dispatch's
// tools/call branch wraps it exactly as it already wraps an unrecognized tool
// name: as a JSON-RPC protocol error, never inside result.content. A refusal
// travels in the result because the contract defines it as an answer, and an
// argument this surface never published is not one of those, since no card,
// column or workbench was reached before the call was turned away.
type unknownArgument struct {
	// tool is the name the call gave.
	tool string
	// names are the argument names the tool does not declare, sorted.
	names []string
	// accepted is every argument name the tool does declare, sorted.
	accepted []string
}

// Error names each argument the tool did not recognize and then what it
// accepts in their place. The reader is an agent correcting its own call with
// nobody watching, and a message carrying only the news that something was
// invalid leaves it exactly as unable to compose the next call as the silence
// this check replaced.
func (e *unknownArgument) Error() string {
	word := "argument"
	if len(e.names) > 1 {
		word = "arguments"
	}
	quoted := make([]string, len(e.names))
	for i, name := range e.names {
		quoted[i] = strconv.Quote(name)
	}
	return fmt.Sprintf("tool %q does not accept %s %s; it accepts: %s",
		e.tool, word, strings.Join(quoted, ", "), strings.Join(e.accepted, ", "))
}

// checkArguments refuses a call carrying an argument name the tool does not
// declare, and reports every such name rather than the first one a map walk
// happens to yield. Both lists are sorted, so one call composes one message
// however Go orders the map on the run that composed it.
func checkArguments(t tool, arguments map[string]any) error {
	declared := declaredArgNames(t)
	var unknown []string
	for name := range arguments {
		if declared[name] {
			continue
		}
		unknown = append(unknown, name)
	}
	if len(unknown) == 0 {
		return nil
	}
	accepted := make([]string, 0, len(declared))
	for name := range declared {
		accepted = append(accepted, name)
	}
	sort.Strings(unknown)
	sort.Strings(accepted)
	return &unknownArgument{tool: t.name, names: unknown, accepted: accepted}
}

// assignValue puts one named string argument on the request.
func assignValue(req *verb.Request, name, value string) {
	switch name {
	case "card":
		req.Card = value
	case "ref":
		req.Ref = value
	case "column":
		req.Column = value
	case "since":
		req.Since = value
	case "query":
		req.Query = value
	case "group-by":
		req.GroupBy = value
	case "depth":
		req.Depth = value
	case "root":
		req.Root = value
	case "max-depth":
		req.MaxDepth = value
	case "action":
		req.Action = value
	case "field":
		req.Field = value
	case "workstream":
		req.Workstream = value
	case "slug":
		req.Slug = value
	case "value":
		req.Value = value
	case "name":
		req.Value = value
	case "severity":
		req.Severity = value
	case "priority":
		req.Priority = value
	case "title":
		req.Title = value
	case "text":
		req.Text = value
	case "phrase":
		req.SearchText = value
	case "reason":
		req.Reason = value
	case "kind":
		req.Kind = value
	case "capacity":
		req.Capacity = value
	case "before":
		req.Before = value
	case "file":
		req.File = value
	case "remint":
		req.Remint = value
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
	case "migrate-columns":
		req.MigrateColumns = value
	case "migrate-vocabulary":
		req.MigrateVocabulary = value
	case "migrate-container":
		req.MigrateContainer = value
	case "migrate-workstreams":
		req.MigrateWorkstreams = value
	case "witness":
		req.MigrateWitness = value
	case "no-claim":
		req.NoClaim = value
	case "archived":
		req.Archived = value
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
