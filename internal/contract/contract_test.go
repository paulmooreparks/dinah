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
