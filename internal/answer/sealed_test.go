package answer

import (
	"bytes"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// TestASealedAnswerHandsOutACopy asserts that writing to the list
// Sealed.Affordances returns leaves the sealed answer unchanged, both on a
// response and on a read's own answer. The compiler keeps a head from
// reaching the list any other way, so this accessor is the one path to it.
func TestASealedAnswerHandsOutACopy(t *testing.T) {
	refused := RefusalSealed(&verb.Request{Verb: "show"}, contract.Refuse(contract.Usage, "show"))
	read := Sealed{payload: Wrap(map[string]any{"status": "ok"}, readAffordances)}
	for name, sealed := range map[string]Sealed{"a refusal": refused, "a read's own answer": read} {
		before, err := sealed.Encode()
		if err != nil {
			t.Fatalf("%s: encode: %v", name, err)
		}
		handed := sealed.Affordances()
		if len(handed) == 0 {
			t.Fatalf("%s publishes no affordances, so the copy has nothing to prove", name)
		}
		first := handed[0]
		handed[0] = "bogus"
		if again := sealed.Affordances(); again[0] != first {
			t.Errorf("%s: writing the handed list changed the answer's first affordance to %q", name, again[0])
		}
		after, err := sealed.Encode()
		if err != nil {
			t.Fatalf("%s: encode: %v", name, err)
		}
		if !bytes.Equal(before, after) {
			t.Errorf("%s: writing the handed list changed the encoded answer:\n%s\nwas:\n%s", name, after, before)
		}
	}
}
