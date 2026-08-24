package bench

import (
	"strings"

	"dinah/internal/contract"
)

// checkStateKinds applies the position rules to the flow and reports a state
// carrying a kind this build does not implement.
//
// The rules are three. An intake state stands first, so at most one state is
// of that kind. A done state stands in the terminal region, which is the run
// of states at the end of the list every member of which is of kind done, and
// no state that is not done stands after a state that is. A buffer stands
// neither first nor in the terminal region.
//
// Every rule is reported rather than refused. A board whose kinds sit outside
// these positions opens and is read as it stands, because refusing it on read
// would leave it unopenable the moment somebody reorders a flow, and the acts
// that would create the condition afresh are refused on their own.
func (b *Bench) checkStateKinds() []Finding {
	var findings []Finding
	start := b.terminalRegionStart()
	for _, state := range b.States {
		path := b.StateAnchorPath(state.ID)
		if strings.Contains(state.Kind, ".") && state.Kind != contract.KindBuffer {
			findings = append(findings, Finding{Path: path, Key: FindingUnknownKind, Detail: state.Ref()})
			continue
		}
		if b.kindStandsWrong(state, start) {
			findings = append(findings, Finding{Path: path, Key: FindingKindOutOfPosition, Detail: state.Ref()})
		}
	}
	return findings
}

// kindStandsWrong reports whether one state's kind is disallowed at the
// position the state stands in, given where the terminal region starts.
func (b *Bench) kindStandsWrong(state *State, terminalStart int) bool {
	switch state.Kind {
	case contract.KindIntake:
		return state.Position != 0
	case contract.KindDone:
		return state.Position < terminalStart
	case contract.KindBuffer:
		return state.Position == 0 || state.Position >= terminalStart
	}
	return false
}

// terminalRegionStart returns the position the terminal region begins at,
// which is the length of the flow when the flow ends in a state that is not
// done. Walking back from the end is what makes the region a run rather than
// a set: a done state with anything but done states after it lies outside it.
func (b *Bench) terminalRegionStart() int {
	start := len(b.States)
	for i := len(b.States) - 1; i >= 0; i-- {
		if b.States[i].Kind != contract.KindDone {
			break
		}
		start = i
	}
	return start
}
