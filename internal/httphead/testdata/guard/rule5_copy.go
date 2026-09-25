package guard

import "dinah/internal/verb"

func overwrite(response *verb.Response) {
	copy(response.Affordances, []string{"next"})
}
