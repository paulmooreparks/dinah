//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"dinah/internal/verb"
)

// launchTUI runs dinah-tui as a child on the same console and answers its
// exit code. Windows has no execve, so the launcher waits: it hands the child
// this process's own console handles, which os/exec documents are passed to
// the child directly when they are *os.File values, and it ignores Ctrl+C and
// Ctrl+Break until the child exits, because Microsoft's "CTRL+C and
// CTRL+BREAK Signals" page says the signals "are passed to all console
// processes that are attached to the console" and the launcher must outlive
// the child to hand on its code. The child restores the console itself.
func (s *session) launchTUI(path string) int {
	child := exec.Command(path, s.args...)
	child.Env = tuiEnviron(os.Environ(), verb.BuildIdentity())
	if in, ok := s.in.(*os.File); ok {
		child.Stdin = in
	}
	if s.rawOut != nil {
		child.Stdout = s.rawOut
	}
	if s.rawErr != nil {
		child.Stderr = s.rawErr
	}
	ignored := make(chan os.Signal, 1)
	signal.Notify(ignored, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(ignored)
	if err := child.Start(); err != nil {
		return s.reportError(err)
	}
	err := child.Wait()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	if err != nil {
		return s.reportError(err)
	}
	return 0
}
