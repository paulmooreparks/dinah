package guard

import "dinah/internal/verb"

func answered() *verb.Response {
	var held verb.Response
	return &held
}
