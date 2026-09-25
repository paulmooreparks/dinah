package guard

import (
	"dinah/internal/answer"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

func refused(r *verb.Request) any {
	return answer.Refusal(r, contract.Refuse(contract.Usage, "show"))
}
