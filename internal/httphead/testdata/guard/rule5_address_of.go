package guard

import "dinah/internal/verb"

func rewrite(response *verb.Response) {
	list := &response.Affordances
	(*list)[0] = "next"
}
