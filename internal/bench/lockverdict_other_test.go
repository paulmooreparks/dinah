//go:build !linux

package bench

import "testing"

// plantedCase is one planted record and the name its subtest reports under.
type plantedCase struct {
	name   string
	record LockRecord
}

// linuxOnlyUnknowns answers nothing outside Linux, whose start alone carries a
// boot and a PID namespace.
func linuxOnlyUnknowns(*testing.T, LockRecord) []plantedCase {
	return nil
}
