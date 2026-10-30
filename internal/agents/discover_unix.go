//go:build !windows && !darwin

package agents

// installedPath adds nothing on Linux, where an app started from the
// desktop has the session's search path.
func installedPath() string { return "" }
