package codexagent

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"studio/internal/acp"
)

// This uses only new fixture threads and sends no model turn. It tests the
// production adapter's new/close/load sequence across an app-server restart.
func TestInstalledEmptyThreadPersistence(t *testing.T) {
	if os.Getenv("DINAH_CODEX_PERSISTENCE_SMOKE") != "1" {
		t.Skip("set DINAH_CODEX_PERSISTENCE_SMOKE=1 for the isolated installed check")
	}
	path, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", t.TempDir())
	for _, key := range []string{"OPENAI_API_KEY", "CODEX_API_KEY", "AZURE_OPENAI_API_KEY", "OPENAI_ACCESS_TOKEN", "ACCESS_TOKEN", "OPENAI_BASE_URL"} {
		t.Setenv(key, "")
	}
	cwd := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := func() *Conn {
		c, err := Start(path, []string{"app-server", "--listen", "stdio://"}, cwd, &testHandler{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(c.Close)
		if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, nil); err != nil {
			t.Fatal(err)
		}
		return c
	}
	c := start()
	var created acp.NewSessionResult
	if err := c.Call(ctx, "session/new", acp.NewSessionParams{Cwd: cwd}, &created); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "session/close", acp.CloseSessionParams{SessionID: created.SessionID}, nil); err != nil {
		t.Fatal(err)
	}
	c.Close()
	<-c.Done()
	restarted := start()
	if err := restarted.Call(ctx, "session/load", acp.LoadSessionParams{SessionID: created.SessionID, Cwd: cwd}, nil); err != nil {
		t.Fatal(err)
	}
	t.Logf("same fixture identity resumed without a first turn: %s", created.SessionID)
}
