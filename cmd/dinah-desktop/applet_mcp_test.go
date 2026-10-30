package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/applet"
)

func testAppletBridge(t *testing.T, a *App, workspace, session string) (*appletChannel, *appletBridgeClient) {
	t.Helper()
	c, _, err := a.newAppletChannel(nil, workspace)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.closeAppletBridge)
	if session != "" {
		if err := a.bindAppletChannel(c, session); err != nil {
			t.Fatal(err)
		}
	}
	client, err := newAppletBridgeClient(a.appletBridge.endpoint, c.token)
	if err != nil {
		t.Fatal(err)
	}
	return c, client
}

func TestAppletBridgeOwnershipAndLifecycle(t *testing.T) {
	a := withTestApplets(t, NewApp())
	workspace := t.TempDir()
	c, client := testAppletBridge(t, a, workspace, "")
	if _, err := client.call(context.Background(), appletBridgeRequest{Operation: "discover"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.call(context.Background(), appletBridgeRequest{Operation: "status", ID: "other"}); err == nil {
		t.Fatal("unbound channel invoked")
	}
	if err := a.bindAppletChannel(c, "chat-one"); err != nil {
		t.Fatal(err)
	}
	a.settings.Workspace = t.TempDir()
	registry, err := a.appletRegistry()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := registry.Digest(testPictureRef)
	if err != nil {
		t.Fatal(err)
	}
	body, err := client.call(context.Background(), appletBridgeRequest{Operation: "invoke", RequestID: "generate-one", Ref: testPictureRef, InputJSON: `{"text":"00012345678901234567890"}`, Digest: digest, TimeoutSeconds: 30})
	if err != nil {
		t.Fatal(err)
	}
	var invocation AppletInvocation
	if err := json.Unmarshal(body, &invocation); err != nil {
		t.Fatal(err)
	}
	if invocation.Workspace != workspace || invocation.OwnerID != "chat-one" {
		t.Fatalf("wrong owner: %+v", invocation)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		body, err = client.call(context.Background(), appletBridgeRequest{Operation: "status", ID: invocation.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &invocation); err != nil {
			t.Fatal(err)
		}
		if invocation.Status != "running" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if invocation.Status != "done" || len(invocation.Artifacts) != 1 {
		t.Fatalf("the picture did not complete: %+v", invocation)
	}
	_, other := testAppletBridge(t, a, workspace, "chat-two")
	for _, q := range []appletBridgeRequest{{Operation: "status", ID: invocation.ID}, {Operation: "cancel", ID: invocation.ID}, {Operation: "artifact", ID: invocation.Artifacts[0].ID}} {
		if _, err := other.call(context.Background(), q); err == nil {
			t.Fatalf("foreign owner accepted: %s", q.Operation)
		}
	}
	body, err = client.call(context.Background(), appletBridgeRequest{Operation: "artifact", ID: invocation.Artifacts[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	var artifact AppletArtifact
	if err := json.Unmarshal(body, &artifact); err != nil || artifact.DataBase64 == "" {
		t.Fatal("artifact bytes missing")
	}
	a.revokeAppletSession("chat-one")
	if _, err := client.call(context.Background(), appletBridgeRequest{Operation: "discover"}); err == nil {
		t.Fatal("revoked channel remained active")
	}
	if _, err := other.call(context.Background(), appletBridgeRequest{Operation: "discover"}); err != nil {
		t.Fatal("revocation affected other chat")
	}
}

func TestAppletBridgeRequestBoundary(t *testing.T) {
	a := withTestApplets(t, NewApp())
	c, _ := testAppletBridge(t, a, t.TempDir(), "chat")
	b := a.appletBridge
	tests := []struct {
		name, method, host, origin, authorization, body string
		status                                          int
	}{
		{"valid", "POST", b.host, "", "Bearer " + c.token, `{"operation":"discover"}`, 200},
		{"no bearer scheme", "POST", b.host, "", c.token, `{"operation":"discover"}`, 401},
		{"foreign host", "POST", "evil.test", "", "Bearer " + c.token, `{"operation":"discover"}`, 403},
		{"browser origin", "POST", b.host, "https://evil.test", "Bearer " + c.token, `{"operation":"discover"}`, 403},
		{"wrong credential", "POST", b.host, "", "Bearer wrong", `{"operation":"discover"}`, 401},
		{"GET", "GET", b.host, "", "Bearer " + c.token, `{"operation":"discover"}`, 403},
		{"owner injection", "POST", b.host, "", "Bearer " + c.token, `{"operation":"discover","workspace":"C:/Other"}`, 400},
		{"extra object", "POST", b.host, "", "Bearer " + c.token, `{"operation":"discover"}{}`, 400},
		{"oversized", "POST", b.host, "", "Bearer " + c.token, `{"operation":"` + strings.Repeat("x", appletBridgeLimit) + `"}`, 400},
		{"human response forbidden", "POST", b.host, "", "Bearer " + c.token, `{"operation":"respond"}`, 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, b.endpoint, strings.NewReader(tt.body))
			r.Host = tt.host
			r.Header.Set("Origin", tt.origin)
			r.Header.Set("Authorization", tt.authorization)
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			a.serveAppletBridge(b, w, r)
			if w.Code != tt.status {
				t.Fatalf("got %d want %d: %s", w.Code, tt.status, w.Body.String())
			}
		})
	}
}

func TestAppletBridgeHumanRequestCanOnlyBeCancelled(t *testing.T) {
	a := withTestApplets(t, NewApp())
	_, client := testAppletBridge(t, a, t.TempDir(), "chat-review")
	registry, err := a.appletRegistry()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := registry.Digest(testChecklistRef)
	if err != nil {
		t.Fatal(err)
	}
	body, err := client.call(context.Background(), appletBridgeRequest{Operation: "invoke", RequestID: "review-one", Ref: testChecklistRef, Digest: digest, InputJSON: `{"title":"Synthetic review","items":["Check the label"]}`, TimeoutSeconds: 30})
	if err != nil {
		t.Fatal(err)
	}
	var v AppletInvocation
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		body, err = client.call(context.Background(), appletBridgeRequest{Operation: "status", ID: v.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &v); err != nil {
			t.Fatal(err)
		}
		if v.Status != "running" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if v.Status != "waiting" || v.Request == nil {
		t.Fatalf("missing human request: %+v", v)
	}
	if _, err := client.call(context.Background(), appletBridgeRequest{Operation: "respond", ID: v.ID}); err == nil {
		t.Fatal("agent answered human request")
	}
	body, err = client.call(context.Background(), appletBridgeRequest{Operation: "cancel", ID: v.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &v); err != nil || v.Status != "cancelled" {
		t.Fatalf("request not cancelled: %s", body)
	}
}

func TestAppletMCPExactInputAndMethods(t *testing.T) {
	exact := `{"count":9007199254740993,"nested":[1e1000,"000123"]}`
	requests := make(chan appletBridgeRequest, 8)
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Error("missing credential")
		}
		var q appletBridgeRequest
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			t.Error(err)
		}
		requests <- q
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"done"}`)
	}))
	defer fixture.Close()
	client, err := newAppletBridgeClient(fixture.URL+"/rpc", "test")
	if err != nil {
		t.Fatal(err)
	}
	s := newAppletWire(t, func(string) (*appletBridgeClient, error) { return client, nil })
	s.initialize()
	msg, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "applet_invoke", "arguments": map[string]any{"requestId": "exact-input", "action": "example/action@1", "inputJSON": exact, "digest": "test", "timeoutSeconds": 30}}})
	result := s.handle(msg)
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), `"isError":true`) {
		t.Fatalf("MCP failed: %s", encoded)
	}
	select {
	case q := <-requests:
		if q.InputJSON != exact {
			t.Fatalf("input changed: %s", q.InputJSON)
		}
	case <-time.After(time.Second):
		t.Fatal("no broker call")
	}
	listed, _ := json.Marshal(s.handle(json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)))
	for _, name := range []string{"applet_discover", "applet_invoke", "applet_status", "applet_cancel", "applet_artifact"} {
		if !strings.Contains(string(listed), name) {
			t.Fatalf("missing %s", name)
		}
	}
	var catalog struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Annotations struct {
					ReadOnly bool `json:"readOnlyHint"`
				} `json:"annotations"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(listed, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, tool := range catalog.Result.Tools {
		readOnly := tool.Name == "applet_discover" || tool.Name == "applet_status" || tool.Name == "applet_artifact"
		if tool.Annotations.ReadOnly != readOnly {
			t.Errorf("%s read-only annotation does not match its effects", tool.Name)
		}
	}
	if strings.Contains(string(listed), "applet_respond") {
		t.Fatal("agent can answer human request")
	}
}

func TestAppletBridgeRevocationAndConfiguration(t *testing.T) {
	a := withTestApplets(t, NewApp())
	c, client := testAppletBridge(t, a, t.TempDir(), "one")
	replacement, _, err := a.newAppletChannel(nil, c.workspace)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.bindAppletChannel(replacement, "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.call(context.Background(), appletBridgeRequest{Operation: "discover"}); err == nil {
		t.Fatal("old credential survived session reload")
	}
	a.revokeAppletChannels(nil)
	if err := a.bindAppletChannel(replacement, "one"); err == nil {
		t.Fatal("connection-revoked channel rebound")
	}
	conn := &acp.Conn{}
	c, servers, err := a.newAppletChannel(conn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 0 {
		t.Fatal("session setup should use the persistent managed registration")
	}
	a.revokeAppletChannel(c)
	for _, endpoint := range []string{"http://example.com/rpc", "https://127.0.0.1:1/rpc", "http://127.0.0.1:1/rpc?token=bad", "http://user@127.0.0.1:1/rpc"} {
		if _, err := newAppletBridgeClient(endpoint, "test"); err == nil {
			t.Fatalf("unsafe endpoint %s", endpoint)
		}
	}
}

func TestAppletManagedContextIsolationAndExpiry(t *testing.T) {
	a := withTestApplets(t, NewApp())
	first, _ := testAppletBridge(t, a, t.TempDir(), "first")
	second, _ := testAppletBridge(t, a, t.TempDir(), "second")
	contextFor := func(c *appletChannel) string { return appletContextPrefix + a.appletBridge.instance + "." + c.token }
	for _, invalid := range []string{"", "../host.token", appletContextPrefix + "../../host." + first.token, "http://127.0.0.1:80/rpc", contextFor(first) + "/extra"} {
		if _, err := resolveAppletContext(configDir(), invalid); err == nil {
			t.Fatal("invalid routing context accepted")
		}
	}
	client, err := resolveAppletContext(configDir(), contextFor(first))
	if err != nil {
		t.Fatal(err)
	}
	a.appletBridge.mu.Lock()
	first.expires = time.Now().Add(-time.Second)
	a.appletBridge.mu.Unlock()
	if _, err := client.call(context.Background(), appletBridgeRequest{Operation: "discover"}); err == nil {
		t.Fatal("expired context accepted")
	}
	if _, err := a.appletPromptContext("first"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.call(context.Background(), appletBridgeRequest{Operation: "discover"}); err != nil {
		t.Fatal("new turn did not renew authorization")
	}
	a.revokeAppletSession("first")
	if _, err := client.call(context.Background(), appletBridgeRequest{Operation: "discover"}); err == nil {
		t.Fatal("revoked context accepted")
	}
	other, err := resolveAppletContext(configDir(), contextFor(second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.call(context.Background(), appletBridgeRequest{Operation: "discover"}); err != nil {
		t.Fatal("revoking one chat affected another")
	}
	oldContext := contextFor(second)
	a.closeAppletBridge()
	if _, err := resolveAppletContext(configDir(), oldContext); err == nil {
		t.Fatal("closed host descriptor survived")
	}
}

func TestAppletMCPRetryKeepsInvocation(t *testing.T) {
	a := withTestApplets(t, NewApp())
	workspace := t.TempDir()
	_, client := testAppletBridge(t, a, workspace, "retry-chat")
	registry, err := a.appletRegistry()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := registry.Digest(testPictureRef)
	if err != nil {
		t.Fatal(err)
	}
	q := appletBridgeRequest{Operation: "invoke", RequestID: "one-operation", Ref: testPictureRef, InputJSON: `{"text":"Retry"}`, Digest: digest, TimeoutSeconds: 30}
	call := func(client *appletBridgeClient) AppletInvocation {
		t.Helper()
		body, err := client.call(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		var v AppletInvocation
		if err := json.Unmarshal(body, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	first := call(client)
	q.TimeoutSeconds = 60
	second := call(client)
	if first.ID != second.ID || !first.Deadline.Equal(second.Deadline) {
		t.Fatal("retry created a new operation or extended its deadline")
	}
	q.InputJSON = `{"text":"000123"}`
	if _, err := client.call(context.Background(), q); err == nil {
		t.Fatal("request ID reused with different inputs")
	}
	q.InputJSON = `{"text":"Retry"}`
	_, other := testAppletBridge(t, a, workspace, "other-chat")
	third := call(other)
	if third.ID == first.ID {
		t.Fatal("request ID was shared across chats")
	}
	awaitApplet(t, a, workspace, "chat", "retry-chat", first.ID, "done")
	awaitApplet(t, a, workspace, "chat", "other-chat", third.ID, "done")
}

func TestAppletContextReplayIsPrivate(t *testing.T) {
	a := withTestApplets(t, NewApp())
	channel, _ := testAppletBridge(t, a, t.TempDir(), "chat")
	capability := appletContextPrefix + a.appletBridge.instance + "." + channel.token
	text, err := a.appletPromptContext("chat")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, capability) {
		t.Fatal("prompt has no authorization")
	}
	var updates []string
	a.sink = func(name string, data any) {
		if name == "agent:update" {
			encoded, _ := json.Marshal(data)
			updates = append(updates, string(encoded))
		}
	}
	for _, message := range []string{text, "Make a changelog\n\n" + text} {
		params, _ := json.Marshal(map[string]any{"sessionId": "chat", "update": map[string]any{"sessionUpdate": "user_message_chunk", "content": map[string]string{"type": "text", "text": message}}})
		a.newAgentHandler().Notify("session/update", params)
	}
	if len(updates) != 1 || !strings.Contains(updates[0], "Make a changelog") || strings.Contains(updates[0], "Applet Context") || strings.Contains(updates[0], channel.token) {
		t.Fatal("replayed host context entered the user transcript")
	}
}

// Discovery narrows to the action or the provider asked for, and a name that
// matches nothing returns no actions rather than the whole catalogue.
func TestAppletDiscoverFilters(t *testing.T) {
	a := withTestApplets(t, NewApp())
	refs := func(q appletBridgeRequest) []string {
		t.Helper()
		q.Operation = "discover"
		v, err := a.dispatchAppletBridge(appletChannel{}, q)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, entry := range v.(map[string]any)["actions"].([]map[string]any) {
			out = append(out, entry["action"].(applet.Action).Ref)
		}
		return out
	}
	all := refs(appletBridgeRequest{})
	if len(all) < 10 {
		t.Fatalf("the unfiltered catalogue has %d actions", len(all))
	}
	if got := refs(appletBridgeRequest{Ref: chatAskAction}); len(got) != 1 || got[0] != chatAskAction {
		t.Fatalf("one action: %v", got)
	}
	chat := refs(appletBridgeRequest{Provider: "dinah.chat"})
	if len(chat) == 0 || len(chat) >= len(all) {
		t.Fatalf("one provider: %v", chat)
	}
	for _, ref := range chat {
		if !strings.HasPrefix(ref, "dinah.chat/") {
			t.Fatalf("provider filter let %s through", ref)
		}
	}
	if got := refs(appletBridgeRequest{Ref: "dinah.chat/nothing@9"}); len(got) != 0 {
		t.Fatalf("unknown action: %v", got)
	}
	if got := refs(appletBridgeRequest{Provider: "dinah.nothing"}); len(got) != 0 {
		t.Fatalf("unknown provider: %v", got)
	}
}

// Every turn's applet context lists the applets by provider ID and title,
// read from the running registry, without their actions.
func TestAppletContextListsTheApplets(t *testing.T) {
	a := withTestApplets(t, NewApp())
	testAppletBridge(t, a, t.TempDir(), "chat")
	text, err := a.appletPromptContext("chat")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := a.appletRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for _, provider := range registry.Catalog() {
		if len(provider.Actions) > 0 && !strings.Contains(text, provider.ID+" ("+provider.Title+")") {
			t.Fatalf("the context leaves out %s:\n%s", provider.ID, text)
		}
	}
	_, line, _ := strings.Cut(text, "The applets available are ")
	line, _, _ = strings.Cut(line, "\n")
	if line == "" || strings.Contains(line, "@") {
		t.Fatalf("the directory is missing or lists actions: %q", line)
	}
}

// A chat invokes a read-only action that finds a chat, then reads its result with the id
// the invoke returned, as an agent does; and an id that cannot be an invocation's, such
// as the requestId, is refused with what to use instead.
func TestAChatCanInvokeFindAndReadItsResult(t *testing.T) {
	a := withTestApplets(t, NewApp())
	workspace := t.TempDir()
	_, client := testAppletBridge(t, a, workspace, "claude:caller")
	registry, err := a.appletRegistry()
	if err != nil {
		t.Fatal(err)
	}
	const ref = "dinah.chat/find@1"
	digest, err := registry.Digest(ref)
	if err != nil {
		t.Fatal(err)
	}
	body, err := client.call(context.Background(), appletBridgeRequest{Operation: "invoke", RequestID: "find a chat", Ref: ref, InputJSON: `{"name":"Nothing Like It"}`, Digest: digest, TimeoutSeconds: 30})
	if err != nil {
		t.Fatal(err)
	}
	var inv AppletInvocation
	if err := json.Unmarshal(body, &inv); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		body, err = client.call(context.Background(), appletBridgeRequest{Operation: "status", ID: inv.ID})
		if err != nil {
			t.Fatalf("polling the invocation of find: %v", err)
		}
		if err := json.Unmarshal(body, &inv); err != nil {
			t.Fatal(err)
		}
		if inv.Status != "running" {
			break
		}
	}
	if inv.Status != "done" || !strings.Contains(inv.OutputJSON, `"match":"not-found"`) {
		t.Fatalf("the find did not finish with its result: %+v", inv)
	}
	_, err = client.call(context.Background(), appletBridgeRequest{Operation: "status", ID: "find a chat"})
	if err == nil || !strings.Contains(err.Error(), "use the id field the invoke returned") || !strings.Contains(err.Error(), "not the requestId") {
		t.Fatalf("a requestId given as an id: %v", err)
	}
}

// A digest discovered before an update no longer fits, and the refusal says to discover the
// action again and use the new digest.
func TestAStaleDigestSaysToDiscoverTheActionAgain(t *testing.T) {
	a := withTestApplets(t, NewApp())
	_, client := testAppletBridge(t, a, t.TempDir(), "claude:caller")
	_, err := client.call(context.Background(), appletBridgeRequest{Operation: "invoke", RequestID: "stale", Ref: "dinah.chat/find@1", InputJSON: `{"name":"x"}`, Digest: strings.Repeat("0", 64), TimeoutSeconds: 30})
	if err == nil || !strings.Contains(err.Error(), "discover it again with applet_discover") || !strings.Contains(err.Error(), "new digest") {
		t.Fatalf("a stale digest: %v", err)
	}
}
