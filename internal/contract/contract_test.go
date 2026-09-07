package contract

import "testing"

// TestTheBufferKindCarriesTheLayerPrefix is dinah-273 AC-1. The token lands on
// disk in every board declaring a buffer, so its spelling is asserted rather
// than left to whoever reads the constant, and the second half says where the
// prefix comes from: CORE-STATE-11 admits a kind of a layer's minting and no
// other, so a bare fourth word would be refused as malformed.
func TestTheBufferKindCarriesTheLayerPrefix(t *testing.T) {
	if KindBuffer != "dinah.buffer" {
		t.Errorf("KindBuffer is %q, and the token a board author types is dinah.buffer", KindBuffer)
	}
	if KindBuffer != LayerPrefix+"buffer" {
		t.Errorf("KindBuffer is %q and does not carry LayerPrefix, which CORE-STATE-11 requires of a minted kind", KindBuffer)
	}
}

// TestMintedKindsCarriesTheBufferAndNothingUndotted asserts that every kind
// Dinah introduces carries the layer prefix, since a bare one is a value every
// conforming tool refuses.
func TestMintedKindsCarriesTheBufferAndNothingUndotted(t *testing.T) {
	found := false
	for _, kind := range MintedKinds {
		if kind == KindBuffer {
			found = true
		}
		if !NameIsLegal(kind) {
			t.Errorf("MintedKinds carries %q, which CORE-OUT-3's shape does not admit", kind)
		}
	}
	if !found {
		t.Errorf("MintedKinds is %v and does not carry the buffer this build implements", MintedKinds)
	}
}

// TestAReadsExitCodeIsItsOwnTableAndNeverTheRefusedOne is dinah-346 AC-1. The
// two tables are asserted together rather than separately, because the whole
// point of the second one is that it does not collide with the first: a read
// that found something exits 5 and a refusal exits 2, so a script reading the
// exit status of dinah check can tell a bad --workbench from a workbench
// carrying defects. A token neither ReadOK nor ReadFindings exits 1, on the
// terms ExitCode reserves 1 for an outcome the profile does not declare.
//
// It is also what drives CORE-OUT-7, which the core profile published at 0.9.
// That statement holds a tool answering an outcome as a number to giving
// `refused` a number no other outcome it reports uses, and the assertion at
// the foot of this test is that comparison over this build's own two tables.
// dinah-346 minted these tokens under a decision that a convention of one
// tool's invocation surface sat outside the profile, and dinah-358 reversed
// that decision, so the convention now moves the profile revision and this
// test is the conformance evidence for it.
func TestAReadsExitCodeIsItsOwnTableAndNeverTheRefusedOne(t *testing.T) {
	if ReadOK != "ok" {
		t.Errorf("ReadOK is %q, and the token a client reads is ok", ReadOK)
	}
	if ReadFindings != "findings" {
		t.Errorf("ReadFindings is %q, and the token a client reads is findings", ReadFindings)
	}
	cases := []struct {
		outcome string
		want    int
	}{
		{outcome: ReadOK, want: 0},
		{outcome: ReadFindings, want: 5},
		{outcome: OutcomeRefused, want: 1},
		{outcome: "anything-else", want: 1},
		{outcome: "", want: 1},
	}
	for _, c := range cases {
		if got := ExitCodeForRead(c.outcome); got != c.want {
			t.Errorf("ExitCodeForRead(%q) is %d, wanted %d", c.outcome, got, c.want)
		}
	}
	if ExitCodeForRead(ReadFindings) == ExitCode(OutcomeRefused) {
		t.Errorf("a read that found something and a refusal both exit %d, which is the overload dinah-346 removed", ExitCodeForRead(ReadFindings))
	}
}

// TestTierNotHigherIsMintedOnceAndCostsTheProfileNothing asserts dinah-409
// AC-9: the raise's one new refusal is Dinah's own, it is entered in the
// minted list exactly once, and neither the profile's own refusal set nor the
// event set moves for it.
//
// The two counts are what the criterion pins, and both are read off the
// declarations rather than carried forward from prose. The event count matters
// because raise composes two events that already exist rather than minting a
// third, and a third would cost a coordinated compat-fixture change nobody
// asked for.
func TestTierNotHigherIsMintedOnceAndCostsTheProfileNothing(t *testing.T) {
	if TierNotHigher != LayerPrefix+"tier-not-higher" {
		t.Errorf("TierNotHigher is %q, and a minted refusal carries the layer prefix", TierNotHigher)
	}
	minted := 0
	for _, name := range Introduced {
		if name == TierNotHigher {
			minted++
		}
	}
	if minted != 1 {
		t.Errorf("Introduced carries TierNotHigher %d times, wanted once", minted)
	}
	for _, name := range Declared {
		if name == TierNotHigher {
			t.Errorf("the profile's own set carries %s, which Dinah minted", name)
		}
	}
	if len(Declared) != 17 {
		t.Errorf("the profile declares %d refusal names, and raise was to leave that seventeen unchanged", len(Declared))
	}
	// The count is pinned rather than derived, so minting an event is a
	// deliberate act that fails here first. It stood at twenty-one while
	// raise composed two existing events rather than minting a third, and it
	// moved to twenty-seven when the checklist verbs minted their six, each
	// of which paid the coordinated compat-fixture change this guard exists
	// to make somebody notice.
	if len(Events) != 27 {
		t.Errorf("the event set carries %d names, and this build declares twenty-seven", len(Events))
	}
}
