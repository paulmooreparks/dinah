package guard

import "dinah/internal/verb"

func answered() *verb.Response {
	return &verb.Response{Outcome: "ok"}
}
