package httphead

import (
	"net/http"

	"dinah/internal/answer"
)

// affordanceRows generates the affordance table from the route table, one
// row per affordance name, the first route naming it winning.
func affordanceRows() []answer.AffordanceRow {
	seen := map[string]bool{}
	var rows []answer.AffordanceRow
	add := func(row answer.AffordanceRow) {
		if seen[row.Affordance] {
			return
		}
		seen[row.Affordance] = true
		rows = append(rows, row)
	}
	for _, r := range routes {
		for _, m := range r.methods {
			for _, a := range m.acts {
				if a.href == "" {
					continue
				}
				accepts := m.accepts
				if a.selectedBy != "" {
					accepts = []string{a.selectedBy}
				}
				members := map[string]string{}
				if m.name != http.MethodPost {
					members[formMethod] = m.name
				}
				if a.selectedBy != "" {
					members[formType] = a.selectedBy
				}
				add(answer.AffordanceRow{
					Affordance: surfaceName(a.command),
					Method:     m.name,
					Href:       a.href,
					Accepts:    append([]string{}, accepts...),
					Form:       &answer.FormRoute{Method: http.MethodPost, Href: a.href, Members: members},
				})
			}
		}
	}
	for _, r := range routes {
		if r.readHref == "" {
			continue
		}
		for _, command := range r.reads {
			add(answer.AffordanceRow{Affordance: surfaceName(command), Method: http.MethodGet, Href: r.readHref, Accepts: []string{}})
		}
	}
	return rows
}

// readAffordances answers GET /affordances.
func readAffordances(h *head, x *exchange) {
	encoded, err := answer.EncodeAffordanceTable(affordanceRows())
	if err != nil {
		http.Error(x.w, err.Error(), http.StatusInternalServerError)
		return
	}
	x.w.Header().Set("Content-Type", servedJSON(x.served))
	x.w.WriteHeader(http.StatusOK)
	x.w.Write(encoded)
}
