package httphead

import (
	"net/http"

	"dinah/internal/answer"
)

// affordanceRow maps one affordance name to the request that performs it.
type affordanceRow struct {
	// Affordance is the name an answer publishes.
	Affordance string `json:"affordance"`
	// Method is the method a client that is not a form sends.
	Method string `json:"method"`
	// Href is the URL template, in {card} and {path}.
	Href string `json:"href"`
	// Accepts are the request types the method takes on the href.
	Accepts []string `json:"accepts"`
	// Form is how an HTML form reaches the same act, and null for a read.
	Form *formRoute `json:"form"`
}

// formRoute is how an HTML form reaches an act: always a POST, to a URL,
// carrying the reserved members of the tunnel as fixed hidden fields.
type formRoute struct {
	Method  string            `json:"method"`
	Href    string            `json:"href"`
	Members map[string]string `json:"members"`
}

// affordanceRows generates the affordance table from the route table, one
// row per affordance name, the first route naming it winning.
func affordanceRows() []affordanceRow {
	seen := map[string]bool{}
	var rows []affordanceRow
	add := func(row affordanceRow) {
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
				add(affordanceRow{
					Affordance: surfaceName(a.command),
					Method:     m.name,
					Href:       a.href,
					Accepts:    append([]string{}, accepts...),
					Form:       &formRoute{Method: http.MethodPost, Href: a.href, Members: members},
				})
			}
		}
	}
	for _, r := range routes {
		if r.readHref == "" {
			continue
		}
		for _, command := range r.reads {
			add(affordanceRow{Affordance: surfaceName(command), Method: http.MethodGet, Href: r.readHref, Accepts: []string{}})
		}
	}
	return rows
}

// readAffordances answers GET /affordances.
func readAffordances(h *head, x *exchange) {
	encoded, err := answer.Encode(map[string]any{"affordances": affordanceRows()})
	if err != nil {
		http.Error(x.w, err.Error(), http.StatusInternalServerError)
		return
	}
	x.w.Header().Set("Content-Type", servedJSON(x.served))
	x.w.WriteHeader(http.StatusOK)
	x.w.Write(encoded)
}
