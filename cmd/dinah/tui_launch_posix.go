//go:build !windows

package main

import (
	"os"
	"syscall"

	"dinah/internal/verb"
)

// launchTUI replaces this process with dinah-tui through syscall.Exec, which
// the syscall package documents as invoking execve(2). The process keeps its
// identifier, controlling terminal, standard streams, process group and
// signal dispositions, so Ctrl+Z, SIGWINCH, SIGINT and SIGTERM reach
// dinah-tui directly, and the shell sees dinah-tui's exit code as the
// command's. syscall.Exec returns only when it has failed, and the failure is
// reported as unreachable.
func (s *session) launchTUI(path string) int {
	argv := append([]string{path}, s.args...)
	err := syscall.Exec(path, argv, tuiEnviron(os.Environ(), verb.BuildIdentity()))
	return s.reportError(err)
}
