package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"studio/internal/appletcontext"
	"studio/internal/communication"
)

const appletBridgeLimit = 1 << 20
const appletBridgeResponseLimit = 24 << 20
const appletContextPrefix = "dinah-applet-v1:"
const appletContextHeading = "[Dinah Applet Context]\n"

var appletHelperExecutable = os.Executable

type appletChannel struct {
	token, workspace, session string
	conn                      agentConnection
	bound                     bool
	connected                 bool
	expires                   time.Time
}
type appletBridgeServer struct {
	mu                   sync.Mutex
	channels             map[string]*appletChannel
	server               *http.Server
	endpoint, host       string
	instance, descriptor string
	slots                chan struct{}
}
type appletBridgeRequest struct {
	Operation      string `json:"operation"`
	Provider       string `json:"provider,omitempty"`
	Ref            string `json:"ref,omitempty"`
	InputJSON      string `json:"inputJSON,omitempty"`
	Digest         string `json:"digest,omitempty"`
	ID             string `json:"id,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
	RequestID      string `json:"requestId,omitempty"`
}

func (a *App) newAppletChannel(conn agentConnection, workspace string) (*appletChannel, []any, error) {
	a.bridgeMu.Lock()
	defer a.bridgeMu.Unlock()
	b := a.appletBridge
	if b == nil {
		if folderClosed() {
			return nil, nil, errFolderClosed
		}
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			return nil, nil, fmt.Errorf("start applet bridge: %w", err)
		}
		b = &appletBridgeServer{channels: map[string]*appletChannel{}, host: listener.Addr().String(), slots: make(chan struct{}, 16)}
		b.endpoint = "http://" + b.host + "/rpc"
		b.instance = rand.Text()
		b.descriptor = filepath.Join(configDir(), "applet-hosts", b.instance+".json")
		if err := os.MkdirAll(filepath.Dir(b.descriptor), 0o700); err != nil {
			listener.Close()
			return nil, nil, err
		}
		data, _ := json.Marshal(appletHostDescriptor{Endpoint: b.endpoint})
		if err := os.WriteFile(b.descriptor, data, 0o600); err != nil {
			listener.Close()
			return nil, nil, err
		}
		b.server = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.serveAppletBridge(b, w, r) }), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
		a.appletBridge = b
		go b.server.Serve(listener)
	}
	channel := &appletChannel{token: rand.Text(), workspace: workspace, conn: conn, expires: time.Now().Add(24 * time.Hour)}
	if conn != nil {
		conn.RegisterTraceSecret(appletContextPrefix + b.instance + "." + channel.token)
		conn.RegisterTraceSecret(channel.token)
	}
	b.mu.Lock()
	b.channels[channel.token] = channel
	b.mu.Unlock()
	// Devin reads the managed persistent registration before opening sessions.
	return channel, []any{}, nil
}

type appletHostDescriptor struct {
	Endpoint string `json:"endpoint"`
}

var appletContextPart = regexp.MustCompile(`^[A-Z2-7]{26,128}$`)
var appletContextPattern = regexp.MustCompile(`dinah-applet-v1:[A-Z2-7]{26,128}\.[A-Z2-7]{26,128}`)

func redactAppletContexts(text string) string { return appletcontext.Redact(text) }

// The model supplies an opaque capability. Routing is read only from a
// host-created descriptor in the helper's installation-owned state directory.
func resolveAppletContext(stateDir, capability string) (*appletBridgeClient, error) {
	value, hasPrefix := strings.CutPrefix(capability, appletContextPrefix)
	instance, token, ok := strings.Cut(value, ".")
	if !hasPrefix || !ok || !appletContextPart.MatchString(instance) || !appletContextPart.MatchString(token) {
		return nil, errors.New("use the current applet context supplied by " + product.Name + " with this turn")
	}
	path := filepath.Join(stateDir, "applet-hosts", instance+".json")
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("this applet context has expired; use the current context supplied by " + product.Name)
	}
	defer f.Close()
	var descriptor appletHostDescriptor
	dec := json.NewDecoder(io.LimitReader(f, 4096))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&descriptor); err != nil {
		return nil, errors.New("invalid " + product.Name + " applet host descriptor")
	}
	var trailing any
	if dec.Decode(&trailing) != io.EOF {
		return nil, errors.New("invalid " + product.Name + " applet host descriptor")
	}
	return newAppletBridgeClient(descriptor.Endpoint, token)
}

// appletDirectory is one line listing every applet in the running registry,
// built-in and installed extensions alike, by provider ID and title. It is
// read afresh for each turn so it cannot go stale, and it lists no actions
// or schemas, which stay with discovery. It is empty when the registry
// cannot be read, since the rest of the context still works without it.
func (a *App) appletDirectory() string {
	registry, err := a.appletRegistry()
	if err != nil {
		return ""
	}
	var entries []string
	for _, provider := range registry.Catalog() {
		if len(provider.Actions) == 0 {
			continue
		}
		entries = append(entries, provider.ID+" ("+provider.Title+")")
	}
	if len(entries) == 0 {
		return ""
	}
	return "The applets available are " + strings.Join(entries, ", ") + ". " +
		"They offer actions to agents; before choosing a workaround, read an applet's actions with applet_discover and its provider argument.\n"
}

func (a *App) appletPromptContext(session string) (string, error) {
	directory := a.appletDirectory()
	person := a.agentPerson()
	a.bridgeMu.Lock()
	defer a.bridgeMu.Unlock()
	b := a.appletBridge
	if b != nil {
		b.mu.Lock()
		defer b.mu.Unlock()
		for _, c := range b.channels {
			if c.bound && c.session == session {
				c.expires = time.Now().Add(24 * time.Hour)
				return appletContextHeading + product.Name + " supplies the internal dinah-applets MCP server automatically. " +
					"Use its applet_discover, applet_invoke, applet_status and applet_artifact tools for bundled applets. " +
					"For every applet tool call, pass this turn's context argument exactly: " + appletContextPrefix + b.instance + "." + c.token +
					"\nThis is a private authorization for this chat. Do not repeat it in replies, files or other tools. " +
					"It replaces any older applet context in the conversation. Discover the action schema and digest before invoking. " +
					"dinah.chat/ask@2 sends its question to the other chat over the bus, and its completion arrives automatically as a later prompt; invoke it once, and do not poll or sleep. For other actions, poll the returned invocation ID instead of starting the same action again. " +
					"Human requests must be answered by the person in Notifications. " +
					"If the service fails, report an internal integration error; never ask the person to configure or reconnect it.\n" + a.chatIdentityLine(session) + directory + "\n" + communication.Window(person), nil
			}
		}
	}
	return "", errors.New("the chat's managed applet connection is unavailable")
}

func (a *App) bindAppletChannel(c *appletChannel, session string) error {
	a.bridgeMu.Lock()
	b := a.appletBridge
	a.bridgeMu.Unlock()
	if b == nil {
		return errors.New("the applet bridge stopped")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.channels[c.token] != c || session == "" {
		return errors.New("the applet channel expired")
	}
	for token, other := range b.channels {
		if other != c && other.session == session {
			delete(b.channels, token)
		}
	}
	c.session, c.bound = session, true
	return nil
}

func (a *App) revokeAppletChannel(c *appletChannel) {
	if c == nil {
		return
	}
	a.bridgeMu.Lock()
	b := a.appletBridge
	a.bridgeMu.Unlock()
	if b != nil {
		b.mu.Lock()
		delete(b.channels, c.token)
		b.mu.Unlock()
	}
}
func (a *App) revokeAppletChannels(conn agentConnection) {
	a.bridgeMu.Lock()
	b := a.appletBridge
	a.bridgeMu.Unlock()
	if b != nil {
		b.mu.Lock()
		for token, c := range b.channels {
			if c.conn == conn {
				delete(b.channels, token)
			}
		}
		b.mu.Unlock()
	}
}
func (a *App) revokeAppletSession(session string) {
	a.bridgeMu.Lock()
	b := a.appletBridge
	a.bridgeMu.Unlock()
	if b != nil {
		b.mu.Lock()
		for token, c := range b.channels {
			if c.session == session {
				delete(b.channels, token)
			}
		}
		b.mu.Unlock()
	}
}
func (a *App) closeAppletBridge() {
	a.bridgeMu.Lock()
	b := a.appletBridge
	a.appletBridge = nil
	a.bridgeMu.Unlock()
	if b != nil {
		b.mu.Lock()
		b.channels = map[string]*appletChannel{}
		b.mu.Unlock()
		_ = b.server.Close()
		_ = os.Remove(b.descriptor)
	}
}

func (a *App) serveAppletBridge(b *appletBridgeServer, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost || r.URL.Path != "/rpc" || r.URL.RawQuery != "" || r.Host != b.host || r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") != "" {
		http.Error(w, "invalid applet bridge request", http.StatusForbidden)
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		http.Error(w, "JSON required", http.StatusUnsupportedMediaType)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		http.Error(w, "applet channel expired", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	b.mu.Lock()
	channel := b.channels[token]
	var owner appletChannel
	if channel != nil {
		owner = *channel
	}
	b.mu.Unlock()
	if channel == nil || time.Now().After(owner.expires) {
		http.Error(w, "applet channel expired", http.StatusUnauthorized)
		return
	}
	select {
	case b.slots <- struct{}{}:
		defer func() { <-b.slots }()
	default:
		http.Error(w, "applet bridge busy", http.StatusTooManyRequests)
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, appletBridgeLimit))
	dec.DisallowUnknownFields()
	var q appletBridgeRequest
	if err := dec.Decode(&q); err != nil {
		http.Error(w, "invalid applet request", http.StatusBadRequest)
		return
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		http.Error(w, "one applet request is required", http.StatusBadRequest)
		return
	}
	if !owner.bound && q.Operation != "discover" {
		http.Error(w, "the chat is not ready", http.StatusConflict)
		return
	}
	// Hold the channel lock through accepting a nonblocking broker operation.
	// Revocation cannot race a request into a different or expired owner.
	b.mu.Lock()
	if b.channels[token] != channel {
		b.mu.Unlock()
		http.Error(w, "applet channel expired", http.StatusUnauthorized)
		return
	}
	result, err := a.dispatchAppletBridge(owner, q)
	if err == nil && q.Operation == "discover" {
		channel.connected = true
	}
	b.mu.Unlock()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (a *App) dispatchAppletBridge(owner appletChannel, q appletBridgeRequest) (any, error) {
	switch q.Operation {
	case "discover":
		registry, err := a.appletRegistry()
		if err != nil {
			return nil, err
		}
		// An action or a provider narrows the answer to what was asked for;
		// one that matches nothing returns no actions, never the catalogue.
		actions := []map[string]any{}
		for _, provider := range registry.Catalog() {
			if q.Provider != "" && provider.ID != q.Provider {
				continue
			}
			for _, action := range provider.Actions {
				if q.Ref != "" && action.Ref != q.Ref {
					continue
				}
				digest, err := registry.Digest(action.Ref)
				if err != nil {
					return nil, err
				}
				actions = append(actions, map[string]any{"provider": provider.ID, "action": action, "digest": digest})
			}
		}
		return map[string]any{"actions": actions}, nil
	case "invoke":
		if len(q.RequestID) < 1 || len(q.RequestID) > 128 {
			return nil, errors.New("requestId must identify this operation with 1 to 128 characters; reuse it when retrying")
		}
		if q.TimeoutSeconds < 1 || q.TimeoutSeconds > 86400 {
			return nil, errors.New("timeoutSeconds must be between 1 and 86400")
		}
		if q.Digest == "" {
			return nil, errors.New("discover the action and supply its digest before invoking it")
		}
		identity, _ := json.Marshal([]string{"chat", owner.session, q.RequestID})
		sum := sha256.Sum256(identity)
		id := hex.EncodeToString(sum[:16])
		deadline := time.Now().Add(time.Duration(q.TimeoutSeconds) * time.Second)
		if previous, err := readAppletInvocation(owner.workspace, id); err == nil {
			// A retry preserves the first accepted deadline and validates the
			// owner, action, digest and exact input in startAppletInvocationID.
			deadline = previous.Deadline
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		v, err := a.startAppletInvocationID(owner.workspace, "chat", owner.session, id, q.Ref, q.InputJSON, q.Digest, deadline)
		if err != nil {
			return nil, err
		}
		return appletDeliveryReceipt(v), nil
	case "status":
		if err := checkInvocationID(q.ID); err != nil {
			return nil, err
		}
		v, err := a.appletInvocation(owner.workspace, "chat", owner.session, q.ID)
		if err != nil {
			return nil, err
		}
		return appletDeliveryReceipt(v), nil
	case "cancel":
		if err := a.cancelAppletInvocation(owner.workspace, "chat", owner.session, q.ID); err != nil {
			return nil, err
		}
		return a.appletInvocation(owner.workspace, "chat", owner.session, q.ID)
	case "artifact":
		return a.appletArtifact(owner.workspace, "chat", owner.session, q.ID)
	default:
		return nil, errors.New("unknown applet operation")
	}
}

// invocationIDForm is what an invocation's id looks like: the id an invoke returns, in
// letters and digits, which is not the requestId the caller chose.
var invocationIDForm = regexp.MustCompile("^[a-zA-Z0-9_-]{1,128}$")

// checkInvocationID refuses a status request whose id cannot be an invocation's, and
// says what to use, since the requestId and the id are easily mixed up.
func checkInvocationID(id string) error {
	if !invocationIDForm.MatchString(id) {
		return errors.New("that is not an invocation id: use the id field the invoke returned (letters and digits only), not the requestId")
	}
	return nil
}

// appletDeliveryReceipt preserves the invocation fields used by existing
// clients and adds an explicit next step for asynchronous chat requests.
func appletDeliveryReceipt(v AppletInvocation) any {
	if v.Action != chatAskAction {
		return v
	}
	return struct {
		AppletInvocation
		Delivery   string `json:"delivery"`
		NextAction string `json:"nextAction"`
	}{v, "automatic", "This is a receipt. " + product.Name + " delivers completion automatically when this chat is idle or after its active turn. Continue independent work or end your turn. Do not poll, sleep or repeat the request."}
}

type appletBridgeClient struct {
	endpoint, token string
	http            *http.Client
}

func newAppletBridgeClient(endpoint, token string) (*appletBridgeClient, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "/rpc" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || token == "" {
		return nil, errors.New("invalid applet bridge configuration")
	}
	return &appletBridgeClient{endpoint: endpoint, token: token, http: &http.Client{Timeout: 25 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("applet bridge redirects are disabled") }}}, nil
}
func (c *appletBridgeClient) call(ctx context.Context, q appletBridgeRequest) (json.RawMessage, error) {
	data, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}
	if len(data) > appletBridgeLimit {
		return nil, errors.New("applet request exceeds 1 MB")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New("the " + product.Name + " applet bridge is unavailable")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, appletBridgeResponseLimit+1))
	if err != nil {
		return nil, err
	}
	if len(body) > appletBridgeResponseLimit {
		return nil, errors.New("applet response exceeds the supported size")
	}
	if res.StatusCode != http.StatusOK {
		var failure struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &failure) == nil && failure.Error != "" {
			return nil, errors.New(failure.Error)
		}
		return nil, fmt.Errorf("applet bridge refused the request (%d)", res.StatusCode)
	}
	if !json.Valid(body) {
		return nil, errors.New("invalid applet bridge response")
	}
	return body, nil
}

func appletMCPServer(resolve func(string) (*appletBridgeClient, error)) *mcp.Server {
	return appletMCPServerUsing(resolve, &appletMCPAdmission{})
}

func appletMCPServerUsing(resolve func(string) (*appletBridgeClient, error), admission *appletMCPAdmission) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "dinah-applets", Version: appVersion}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
		Instructions: product.Name + " manages this service. Every tool requires the current context supplied with the chat turn. Never print or save that private context. Discover bundled applet actions and their input schemas first. Invoke with inputJSON containing an exact JSON object and the discovered digest. All operations belong to the authorized chat's workspace. Poll status for completion, except for dinah.chat/ask@2: it sends its question to the other chat over the bus, so invoke it once and the answer, an error or a cancellation arrives automatically as a later prompt; do not poll or sleep. Waiting requests require the person in Notifications; no tool can answer them. Retrieve artifacts only by the returned artifact ID. Skills use these same tools and receive no additional authority.",
	})
	s.AddReceivingMiddleware(admission.middleware)
	yes := true
	tools := []*mcp.Tool{
		{Name: "applet_artifact", Description: "Read an artifact owned by this chat, including MIME type, digest and base64 bytes.", InputSchema: json.RawMessage(`{"properties":{"context":{"description":"The private applet context supplied by ` + product.Name + ` with the current chat turn. Never invent, print or save this value.","type":"string"},"id":{"type":"string"}},"required":["id","context"],"type":"object"}`), Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &yes, OpenWorldHint: &yes}},
		{Name: "applet_cancel", Description: "Cancel an invocation owned by this chat.", InputSchema: json.RawMessage(`{"properties":{"context":{"description":"The private applet context supplied by ` + product.Name + ` with the current chat turn. Never invent, print or save this value.","type":"string"},"id":{"type":"string"}},"required":["id","context"],"type":"object"}`), Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &yes, OpenWorldHint: &yes}},
		{Name: "applet_discover", Description: "List applet actions, schemas, capabilities and implementation digests available to this chat. Pass action for one action or provider for one applet's actions; with neither, the whole catalogue is returned, which is large.", InputSchema: json.RawMessage(`{"properties":{"action":{"description":"An action reference such as dinah.chat/find@1. Only that action is returned, or none when it does not exist.","type":"string"},"context":{"description":"The private applet context supplied by ` + product.Name + ` with the current chat turn. Never invent, print or save this value.","type":"string"},"provider":{"description":"An applet's provider ID such as dinah.chat. Only that applet's actions are returned.","type":"string"}},"required":["context"],"type":"object"}`), Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &yes, OpenWorldHint: &yes}},
		{Name: "applet_invoke", Description: "Start a bundled applet action in this chat's workspace. Returns a durable invocation; use status to obtain its result or see a human request.", InputSchema: json.RawMessage(`{"properties":{"action":{"type":"string"},"context":{"description":"The private applet context supplied by ` + product.Name + ` with the current chat turn. Never invent, print or save this value.","type":"string"},"digest":{"type":"string"},"inputJSON":{"description":"An exact JSON object serialized as a string, preserving large numbers.","type":"string"},"requestId":{"description":"Choose a unique identifier for this operation (1 to 128 characters). Reuse it with the same action, digest and input if a call fails or its response is lost. A repeated request returns the existing invocation and retains its original deadline.","type":"string"},"timeoutSeconds":{"description":"Integer from 1 to 86400, default 300.","type":"number"}},"required":["requestId","action","inputJSON","digest","context"],"type":"object"}`), Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &yes, OpenWorldHint: &yes}},
		{Name: "applet_status", Description: "Read an invocation owned by this chat. A waiting request must be answered by the person in Notifications.", InputSchema: json.RawMessage(`{"properties":{"context":{"description":"The private applet context supplied by ` + product.Name + ` with the current chat turn. Never invent, print or save this value.","type":"string"},"id":{"type":"string"}},"required":["id","context"],"type":"object"}`), Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &yes, OpenWorldHint: &yes}},
	}
	for _, tool := range tools {
		op := strings.TrimPrefix(tool.Name, "applet_")
		name, rules := tool.Name, appletToolRules(tool)
		s.AddTool(tool, func(ctx context.Context, r *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Arguments that are not a JSON object read as no arguments, as
			// the prior SDK's getters did, so the call fails on authorization
			// rather than as a protocol error.
			var args map[string]any
			if json.Unmarshal(r.Params.Arguments, &args) != nil {
				args = nil
			}
			text := func(key string) string { v, _ := args[key].(string); return v }
			c, err := resolve(text("context"))
			if err != nil {
				return appletMCPText(err.Error(), true), nil
			}
			// The server does not check a call against a tool's schema on its own, so a
			// call that lacks a required field, or carries one the tool does not have, is
			// refused here with the field's name rather than read as an empty value.
			if err := rules.check(name, args); err != nil {
				return appletMCPText(err.Error(), true), nil
			}
			seconds, number := appletMCPTimeout(args["timeoutSeconds"])
			q := appletBridgeRequest{Operation: op, Provider: text("provider"), Ref: text("action"), InputJSON: text("inputJSON"), Digest: text("digest"), ID: text("id"), RequestID: text("requestId"), TimeoutSeconds: seconds}
			if op == "invoke" && (number < 1 || number > 86400 || number != float64(seconds)) {
				return appletMCPText("timeoutSeconds must be an integer between 1 and 86400", true), nil
			}
			data, err := c.call(ctx, q)
			if err != nil {
				return appletMCPText(err.Error(), true), nil
			}
			return appletMCPText(string(data), false), nil
		})
	}
	return s
}

// appletToolRule is what a tool's schema says of its arguments: the names it has, which of
// them are required, and the JSON type each has.
type appletToolRule struct {
	names    []string
	required []string
	types    map[string]string
}

// appletToolRules reads a tool's input schema.
func appletToolRules(tool *mcp.Tool) appletToolRule {
	var schema struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	raw, _ := json.Marshal(tool.InputSchema)
	_ = json.Unmarshal(raw, &schema)
	r := appletToolRule{required: schema.Required, types: map[string]string{}}
	for name, p := range schema.Properties {
		r.names = append(r.names, name)
		r.types[name] = p.Type
	}
	sort.Strings(r.names)
	return r
}

// check refuses arguments that lack a required field, carry a field the tool does not have,
// or give a field a value of another type, naming each.
func (r appletToolRule) check(tool string, args map[string]any) error {
	var problems []string
	for _, name := range r.required {
		// The context may come with the connection and not the call, which the resolver
		// has already accepted or refused.
		if name == "context" {
			continue
		}
		if _, ok := args[name]; !ok {
			problems = append(problems, name+" is required")
		}
	}
	var unknown []string
	for name := range args {
		if _, ok := r.types[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	for _, name := range unknown {
		problems = append(problems, name+" is not a parameter of "+tool)
	}
	for _, name := range r.names {
		v, ok := args[name]
		if !ok {
			continue
		}
		switch r.types[name] {
		case "string":
			if _, isString := v.(string); !isString {
				problems = append(problems, name+" must be a string")
			}
		case "number":
			if _, isNumber := v.(float64); !isNumber {
				problems = append(problems, name+" must be a number")
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "; ") + " (the parameters of " + tool + " are " + strings.Join(r.names, ", ") + ")")
}

func appletMCPText(text string, failed bool) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, IsError: failed}
}

// Keep the prior SDK getters' defaults and string-number conversions.
func appletMCPTimeout(value any) (int, float64) {
	seconds, number := 300, float64(300)
	switch v := value.(type) {
	case float64:
		seconds, number = int(v), v
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			seconds = n
		}
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			number = n
		}
	}
	return seconds, number
}

func runAppletMCP(stateDir string) error {
	if !filepath.IsAbs(stateDir) {
		return errors.New("the applet state directory must be absolute")
	}
	return appletMCPServer(func(capability string) (*appletBridgeClient, error) {
		return resolveAppletContext(stateDir, capability)
	}).Run(context.Background(), &mcp.StdioTransport{MaxLineLength: -1})
}
