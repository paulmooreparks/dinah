package guard

import "dinah/internal/verb"

func failed(l *verb.Library, req *verb.Request, err error) any {
	return l.FromError(req, err)
}
