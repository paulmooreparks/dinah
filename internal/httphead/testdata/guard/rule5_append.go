package guard

import "dinah/internal/verb"

func extend(response *verb.Response) []string {
	return append(response.Affordances, "next")
}
