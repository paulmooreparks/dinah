package verb

import "dinah/internal/bench"

// composeChain carries the name of an exempt function and makes the read its
// exemption argues for, bench.GlobalInstructions, beside a read of a
// workbench file the exemption does not cover. dinah-619/comments/15 planted
// the second read in the real composeChain, and a guard keyed on the function
// name let it through. Keyed on the pair, the guard refuses it.
func (l *Library) composeChain() string {
	global := bench.GlobalInstructions(l.Home)
	standing, _ := bench.ReadText(l.Bench.Root + "/workbench.md")
	return global + standing
}
