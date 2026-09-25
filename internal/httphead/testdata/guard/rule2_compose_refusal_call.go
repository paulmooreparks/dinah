package guard

import (
	"dinah/internal/contract"
	"dinah/internal/verb"
)

func refuse(req *verb.Request) any {
	return verb.ComposeRefusal(req, contract.Refuse(contract.Usage, "x"))
}
