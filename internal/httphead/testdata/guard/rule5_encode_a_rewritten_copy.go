package guard

import (
	"encoding/json"

	"dinah/internal/answer"
)

// rewritten is the round-three review's reproduction: it takes the copy
// Sealed.Affordances returns, swaps its first two names, and encodes a map of
// its own in place of the sealed answer.
func rewritten(answered answer.Sealed) ([]byte, error) {
	encoded, err := answered.Encode()
	if list := answered.Affordances(); err == nil && len(list) > 1 {
		var members map[string]any
		if json.Unmarshal(encoded, &members) == nil {
			list[0], list[1] = list[1], list[0]
			members["affordances"] = list
			encoded, err = answer.Encode(members)
		}
	}
	return encoded, err
}
