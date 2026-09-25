package guard

import "dinah/internal/verb"

func drop(response *verb.Response) {
	if response != nil && len(response.Affordances) > 1 {
		copy(response.Affordances[:1], response.Affordances[1:2])
	}
}
