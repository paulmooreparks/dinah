package guard

import (
	"dinah/internal/answer"
	"dinah/internal/verb"
)

func answered(l *verb.Library, r *verb.Request) any {
	payload, _ := answer.Run("show", l, r)
	return payload
}
