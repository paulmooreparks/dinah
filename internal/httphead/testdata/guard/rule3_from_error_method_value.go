package guard

import "dinah/internal/verb"

func failer(l *verb.Library) any {
	return l.FromError
}
