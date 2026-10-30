//go:build live

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"studio/internal/acp"
)

func packagedAppletHelper(t *testing.T) string {
	t.Helper()
	helper := filepath.Join(t.TempDir(), "dinah-helper.exe")
	build := exec.Command("go", "build", "-tags", "desktop,production", "-ldflags", "-H windowsgui", "-o", helper, ".")
	acp.HideWindow(build)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, output)
	}
	previous := appletHelperExecutable
	appletHelperExecutable = func() (string, error) { return helper, nil }
	t.Cleanup(func() { appletHelperExecutable = previous })
	return helper
}

// The helper starts without an active host or any credentials in its config.
// Authorization is checked separately for each tool call.
func TestAppletMCPPackagedHandshake(t *testing.T) {
	helper := packagedAppletHelper(t)
	a := withTestApplets(t, NewApp())
	channel, _ := testAppletBridge(t, a, t.TempDir(), "handshake-chat")
	capability := appletContextPrefix + a.appletBridge.instance + "." + channel.token
	cmd := exec.Command(helper, "--applet-mcp", "--state-dir", configDir())
	cmd.Env = os.Environ()
	acp.HideWindow(cmd)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cli, err := mcp.NewClient(&mcp.Implementation{Name: "packaged-handshake-test", Version: "1"}, nil).Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	tools, err := cli.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 5 {
		t.Fatalf("got %d tools", len(tools.Tools))
	}
	call := &mcp.CallToolParams{Name: "applet_discover"}
	result, err := cli.CallTool(ctx, call)
	if err != nil || !result.IsError {
		t.Fatal("discovery without chat authorization was accepted")
	}
	call.Arguments = map[string]any{"context": capability}
	result, err = cli.CallTool(ctx, call)
	if err != nil || result.IsError {
		t.Fatalf("authorized discovery failed: %v", err)
	}
	t.Log("Packaged helper lists tools and requires chat authorization for action discovery")
}

func TestAppletMCPArtifactInvocation(t *testing.T) {
	packagedAppletHelper(t)
	a, done := liveApp(t)
	defer done()
	defer a.closeAppletBridge()
	withTestApplets(t, a)
	r := record(a)
	session, err := a.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Send(session.ID, "@Picture Draw a picture of the text Hello. Use the applet tools, then retrieve its artifact. Do not use shell, file or network tools. Report the invocation ID when complete.", nil, []Attachment{{Kind: "applet", Provider: testPictureID}}); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, a, session.ID)
	invocations, err := a.AppletInvocationsAt(session.Cwd)
	if err != nil {
		t.Fatal(err)
	}
	var completed *AppletInvocation
	for i := range invocations {
		v := &invocations[i]
		if v.OwnerKind == "chat" && v.OwnerID == session.ID && v.Action == testPictureRef && v.Status == "done" && len(v.Artifacts) == 1 {
			completed = v
			break
		}
	}
	if completed == nil {
		r.mu.Lock()
		response := r.said.String()
		r.mu.Unlock()
		t.Fatalf("no completed picture invocation; agent response: %s", response)
	}
	t.Log("The model drew a picture through the automatically registered applet")
	oldContext, err := a.appletPromptContext(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	a.Stop()
	a.closeAppletBridge()
	trace, err := os.ReadFile(acpTracePath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(trace), "dataBase64") {
		t.Fatal("model did not retrieve the generated artifact bytes")
	}
	if appletContextPattern.Match(trace) {
		t.Fatal("authorization appeared in trace")
	}
	// Construct a fresh host so recovery cannot depend on the first App's
	// in-memory broker, session table or settings object.
	a = withTestApplets(t, NewApp())
	defer a.closeAppletBridge()
	defer a.Stop()
	r = record(a)
	if _, err := a.OpenSessionAt(session.Cwd, session.ID); err != nil {
		t.Fatal(err)
	}
	newContext, err := a.appletPromptContext(session.ID)
	if err != nil || newContext == oldContext {
		t.Fatal("reopening did not refresh authorization")
	}
	// Replay may include old tool output, so record the trace length before
	// checking the new turn's artifact retrieval.
	before, err := os.Stat(acpTracePath())
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Send(session.ID, "Use the applet tools to retrieve the existing picture artifact from invocation "+completed.ID+". Do not draw another picture. Report the existing invocation ID and artifact name after retrieving it.", nil, nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, a, session.ID)
	after, err := a.AppletInvocationsAt(session.Cwd)
	if err != nil || len(after) != len(invocations) {
		t.Fatal("reopening duplicated the operation")
	}
	r.mu.Lock()
	response := r.said.String()
	r.mu.Unlock()
	if !strings.Contains(response, completed.ID) {
		t.Fatalf("model did not report the invocation: %s", response)
	}
	a.Stop()
	trace, err = os.ReadFile(acpTracePath())
	if err != nil || int64(len(trace)) < before.Size() || !strings.Contains(string(trace[before.Size():]), "dataBase64") {
		t.Fatal("reopened chat did not retrieve the earlier artifact bytes")
	}
	if appletContextPattern.Match(trace) {
		t.Fatal("replayed authorization appeared in trace")
	}
	t.Log("Created a fresh application host, reopened the chat and retrieved its earlier artifact without duplication")
}
