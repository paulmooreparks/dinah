package guard

import (
	"dinah/internal/answer"
	"dinah/internal/verb"
)

func failed(l *verb.Library, r *verb.Request, err error) any {
	return answer.FromError(l, r, err)
}
