package guard

import (
	reply "dinah/internal/answer"
	"dinah/internal/verb"
)

func answered(l *verb.Library, r *verb.Request) any {
	payload, _ := reply.Run("show", l, r)
	return payload
}
