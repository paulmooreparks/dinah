//go:build !windows

package bench

import "testing"

// reusedPIDsEverywhere does nothing here. Rule 6 outside Windows reads /proc
// or the null signal, and no table stands in for them; the arrangement that
// makes an ended helper's reused PID read as unknown is Windows's own.
func reusedPIDsEverywhere(*testing.T) {}
