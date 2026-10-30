//go:build !windows

// Package procjob ties the processes Dinah starts to Dinah's own life, so
// none outlives it: the agents and the shells of its Terminal. Outside
// Windows it does nothing yet.
package procjob

// Tie does nothing outside Windows.
func Tie(pid int) {}

func TieRequired(pid int) error { return nil }
