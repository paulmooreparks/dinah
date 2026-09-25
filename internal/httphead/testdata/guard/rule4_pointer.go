package guard

import "dinah/internal/verb"

func held(response *verb.Response) string {
	return response.Outcome
}
