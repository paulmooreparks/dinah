package bench

import "os/exec"

// PlantDelegated reads a file by running another program. os/exec is not
// judged and the read happens in another process, so this passes. It is kept
// as the reproduction of a gap the guard states rather than closes.
func (b *Bench) PlantDelegated(p string) ([]byte, error) {
	return exec.Command("cmd", "/c", "type", p).Output()
}
