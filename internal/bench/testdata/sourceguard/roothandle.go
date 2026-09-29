package bench

import "os"

// want: os.Root

// PlantFromRoot reads through an *os.Root opened elsewhere and handed in. The
// body names no package member, and a method call on a value is not judged,
// so the signature is where the read shows (dinah-619/comments/15).
func (b *Bench) PlantFromRoot(r *os.Root) ([]byte, error) {
	return r.ReadFile("workbench.md")
}
