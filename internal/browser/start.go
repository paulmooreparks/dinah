//go:build !windows

package browser

import "os/exec"

// startCommand starts a program with its arguments, each passed to the
// program as one argument and through no shell, and reaps it once it exits
// so no zombie is left behind. Only this package's tests replace it.
var startCommand = func(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
