package main

import (
	"os"
	"path/filepath"
	"testing"

	"studio/internal/credentials"
	"studio/internal/pty"
	"studio/internal/rotlog"
)

// TestMain keeps the tests away from the person's own settings, flows and
// history: every test reads and writes a throwaway settings folder instead.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "dinah-test-config-")
	if err != nil {
		panic(err)
	}
	configOverride = filepath.Join(dir, "dinah")
	// The Claude API key is kept in memory, never in the person's keychain.
	var claudeKey string
	claudeKeys.get = func() (string, error) {
		if claudeKey == "" {
			return "", credentials.ErrNotFound
		}
		return claudeKey, nil
	}
	claudeKeys.set = func(key string) error { claudeKey = key; return nil }
	claudeKeys.remove = func() error { claudeKey = ""; return nil }
	// Logs are written as they are made, so that nothing is written into a
	// test's folder after the test has ended.
	rotlog.Immediate = true
	// The Terminal's shells need a process with no standard handles of its
	// own, as the window is (main.go).
	if err := pty.ReleaseStandardHandles(); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
