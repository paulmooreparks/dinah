package guard

import "dinah/internal/verb"

func answered() any {
	return verb.Response{Outcome: "ok"}
}
