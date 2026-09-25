package guard

import (
	"dinah/internal/contract"
	"dinah/internal/verb"
)

func refuse(req *verb.Request) *verb.Response {
	return verb.ComposeRefusal(req, contract.Refuse(contract.Usage, "x"))
}
