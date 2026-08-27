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
