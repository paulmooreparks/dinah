package guard

import "dinah/internal/verb"

func failer(l *verb.Library) func(*verb.Request, error) *verb.Response {
	return l.FromError
}
