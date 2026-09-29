package verb

import "dinah/internal/bench"

// want: names dinah/internal/bench.ReadText

// plantPair makes the read its plant-local exemption pair argues for,
// bench.GlobalInstructions, beside a read of a workbench file the pair does
// not cover. dinah-619/comments/15 planted the second read in the real
// composeChain, and a guard keyed on the function let it through. Keyed on
// the pair, the guard refuses the second read alone.
func (l *Library) plantPair() string {
	global := bench.GlobalInstructions(l.Home)
	standing, _ := bench.ReadText(l.Bench.Root + "/workbench.md")
	return global + standing
}
