//go:build !windows

package ollamaagent

import (
	"os"
	"syscall"
)

// terminate asks the process to exit.
func terminate(p *os.Process) { _ = p.Signal(syscall.SIGTERM) }
