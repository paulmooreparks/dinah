//go:build windows

package codexagent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
)

func TestBatchLauncher(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "with spaces")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "codex.cmd")
	if err := os.WriteFile(script, []byte("@echo off\r\necho %~1\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd, err := command(context.Background(), script, []string{"app-server"})
	if err != nil {
		t.Fatal(err)
	}
	// The transport hides the window after the launcher is built, which
	// must keep the launcher's command line.
	acp.HideWindow(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow || cmd.SysProcAttr.CmdLine == "" {
		t.Fatal("batch window was not hidden, or hiding it lost the command line")
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "app-server" {
		t.Fatalf("batch output %q", out)
	}
}

// Codex is told it runs in Dinah's window, so the person's own Dinah server
// in Codex's configuration stands down rather than joining the bus.
func TestStartMarksCodexHosted(t *testing.T) {
	t.Setenv(bus.HostedVar, "")
	dir := t.TempDir()
	script, out := filepath.Join(dir, "codex.cmd"), filepath.Join(dir, "hosted.txt")
	if err := os.WriteFile(script, []byte("@echo off\r\necho [%"+bus.HostedVar+"%]>\"%~1\"\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Start(script, []string{out}, dir, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	select {
	case <-c.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("the launcher did not finish")
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "[1]" {
		t.Fatalf("%s in Codex's environment is %q, not 1", bus.HostedVar, got)
	}
}

func TestBatchRejectsShellSyntax(t *testing.T) {
	for _, value := range []string{"a&b", "%PATH%", "!name!", "a|b", "a>b", "a<b", "a^b", "a\"b", "a\nb", "a(b)", "a?b"} {
		if _, err := batchLine("C:\\safe\\codex.cmd", []string{value}); err == nil {
			t.Errorf("accepted argument %q", value)
		}
		if _, err := batchLine("C:\\"+value+"\\codex.cmd", nil); err == nil {
			t.Errorf("accepted path %q", value)
		}
	}
}
