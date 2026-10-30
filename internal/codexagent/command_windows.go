//go:build windows

package codexagent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Go's os/exec documentation requires CmdLine for cmd.exe and batch files,
// whose argument parsing differs from CommandLineToArgvW. The transport
// hides the window after the command is built (acp.Start).
func command(ctx context.Context, name string, args []string) (*exec.Cmd, error) {
	if !strings.EqualFold(filepath.Ext(name), ".cmd") && !strings.EqualFold(filepath.Ext(name), ".bat") {
		return exec.CommandContext(ctx, name, args...), nil
	}
	if !filepath.IsAbs(name) {
		return nil, errors.New("Codex batch launcher must have an absolute path")
	}
	line, err := batchLine(name, args)
	if err != nil {
		return nil, err
	}
	systemRoot := os.Getenv("SystemRoot")
	if !filepath.IsAbs(systemRoot) {
		return nil, errors.New("Windows system directory is unavailable")
	}
	shell := filepath.Join(systemRoot, "System32", "cmd.exe")
	if strings.ContainsAny(shell, "\"\r\n\x00") {
		return nil, errors.New("invalid Windows system directory")
	}
	cmd := exec.CommandContext(ctx, shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: "\"" + shell + "\" /d /s /v:off /c " + line}
	return cmd, nil
}

func batchLine(name string, args []string) (string, error) {
	values := append([]string{name}, args...)
	quoted := make([]string, len(values))
	for i, value := range values {
		// Provider startup arguments are literals. Reject expansion and shell
		// syntax instead of attempting to escape arbitrary command programs.
		if strings.ContainsAny(value, "%!?^&|<>\"\r\n\x00()") {
			return "", errors.New("Codex batch launcher path or argument contains unsupported shell characters")
		}
		quoted[i] = "\"" + value + "\""
	}
	return "\"" + strings.Join(quoted, " ") + "\"", nil
}
