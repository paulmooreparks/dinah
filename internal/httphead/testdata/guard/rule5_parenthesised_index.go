package guard

import "dinah/internal/verb"

func rewrite(response *verb.Response) {
	(response.Affordances)[0] = "next"
}
