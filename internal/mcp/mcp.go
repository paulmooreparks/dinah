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
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
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
func Serve(library *verb.Library, in io.Reader, out io.Writer) error {
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
		answer := dispatch(library, &req)
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
func dispatch(library *verb.Library, req *request) *response {
	if len(req.ID) == 0 {
		return nil
	}
	answer := &response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		answer.Result = initializeResult(library)
	case "tools/list":
		answer.Result = map[string]any{"tools": toolList()}
	case "tools/call":
		result, err := call(library, req.Params)
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
func initializeResult(library *verb.Library) map[string]any {
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
		"instructions": workingAgreement(library),
	}
	return result
}

// workingAgreement is section 8 of the profile: the four rules that bind an
// owner rather than a tool, and the bench this process serves.
func workingAgreement(library *verb.Library) string {
	var b strings.Builder
	b.WriteString("You are working the workbench " + library.Bench.Title + ".\n\n")
	b.WriteString("The working agreement, which binds you rather than the tool:\n")
	b.WriteString("1. Claim a card before producing work on it.\n")
	b.WriteString("2. Do not hold a claim on a card you have stopped working.\n")
	b.WriteString("3. Treat the workbench as the authority for where a card stands and who holds it.\n")
	b.WriteString("4. Do not move a card out of an operator-owned state unless you are the operator.\n\n")
	b.WriteString("Every response carries an affordances member naming what you may do next. ")
	b.WriteString("A successful claim or move carries the instructions of the position in three ")
	b.WriteString("separate layers and the moves the flow allows. Tokens are canonical on this ")
	b.WriteString("surface and are never translated.\n\n")
	b.WriteString("The operator of this workbench is " + library.Bench.Operator + ". ")
	b.WriteString("Read the embedded guides through resources/list and resources/read.\n")
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

// call runs one tool and returns its canonical answer.
func call(library *verb.Library, params json.RawMessage) (map[string]any, error) {
	var args struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, err
	}
	tool, ok := toolsByName[args.Name]
	if !ok {
		return nil, contract.Refuse(contract.UnknownVerb, args.Name)
	}
	payload := tool.run(library, request2Args(tool.command, args.Arguments))
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(encoded)}},
	}
	return result, nil
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
