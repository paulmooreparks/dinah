package answer

// AffordanceRow is one row of the affordance table a head publishes, mapping
// an affordance name to the request that performs it.
type AffordanceRow struct {
	// Affordance is the name an answer publishes.
	Affordance string `json:"affordance"`
	// Method is the method a client that is not a form sends.
	Method string `json:"method"`
	// Href is the URL template, in {card} and {path}.
	Href string `json:"href"`
	// Accepts are the request types the method takes on the href.
	Accepts []string `json:"accepts"`
	// Form is how an HTML form reaches the same act, and null for a read.
	Form *FormRoute `json:"form"`
}

// FormRoute is how an HTML form reaches an act: always a POST, to a URL,
// carrying the reserved members of the tunnel as fixed hidden fields.
type FormRoute struct {
	Method  string            `json:"method"`
	Href    string            `json:"href"`
	Members map[string]string `json:"members"`
}

// EncodeAffordanceTable encodes the affordance table a head publishes, as the
// one member affordances holding rows. It takes rows and nothing else, so a
// head that encodes through it can publish a table but never an answer; the
// HTTP head reaches Encode only through a Sealed answer and through this.
func EncodeAffordanceTable(rows []AffordanceRow) ([]byte, error) {
	return Encode(map[string]any{"affordances": rows})
}
