package guard

import "dinah/internal/verb"

func failed(l *verb.Library, req *verb.Request, err error) *verb.Response {
	return l.FromError(req, err)
}
