package pages

import "encoding/json"

// The view structs below name only the JSON members the pages draw. Each
// renderer decodes the bytes answer.Encode produced for a read into them, so a
// page is another representation of the payload a JSON client reads and cannot
// draw anything the payload does not carry.

// statusPayload is the status read's answer.
type statusPayload struct {
	Status struct {
		Workbench string         `json:"workbench"`
		Actor     string         `json:"actor"`
		Columns   []statusColumn `json:"columns"`
	} `json:"status"`
}

// statusColumn is one column the status read lists.
type statusColumn struct {
	ID              string `json:"id"`
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	Kind            string `json:"kind"`
	OperatorOwned   bool   `json:"operator_owned"`
	AwaitingOutside bool   `json:"awaiting_outside"`
	TakesWorkUp     bool   `json:"takes_work_up"`
	PullDestination string `json:"pull_destination"`
	Capacity        int    `json:"capacity"`
	Count           int    `json:"count"`
}

// address is what a person types to name the column: its slug, or its
// identifier where it has none.
func (c statusColumn) address() string {
	if c.Slug != "" {
		return c.Slug
	}
	return c.ID
}

// treePayload is the tree read's answer.
type treePayload struct {
	Tree struct {
		GroupBy []string `json:"group_by"`
		Root    treeNode `json:"root"`
	} `json:"tree"`
}

// treeNode is one node of a projected tree.
type treeNode struct {
	Kind     string      `json:"kind"`
	ID       string      `json:"id"`
	Ref      string      `json:"ref"`
	Title    string      `json:"title"`
	Axis     string      `json:"axis"`
	Value    string      `json:"value"`
	Count    int         `json:"count"`
	Hidden   *treeHidden `json:"hidden"`
	Children []treeNode  `json:"children"`
}

// treeHidden is what a node does not show.
type treeHidden struct {
	Reason   []string `json:"reason"`
	Children int      `json:"children"`
	Subjects int      `json:"subjects"`
	Filtered int      `json:"filtered"`
}

// showPayload is the show read's answer for a card, a column or anything
// else it resolves.
type showPayload struct {
	Detail      *detailView     `json:"detail"`
	Record      json.RawMessage `json:"record"`
	Affordances []string        `json:"affordances"`
}

// detailView is a card as show draws it.
type detailView struct {
	Card        cardView         `json:"card"`
	Body        string           `json:"body"`
	Links       []linkView       `json:"links"`
	Attachments []attachmentView `json:"attachments"`
	Comments    []commentView    `json:"comments"`
	Checklist   []itemView       `json:"checklist"`
}

// cardView is a card's own members.
type cardView struct {
	ID              string            `json:"id"`
	Ref             string            `json:"ref"`
	Title           string            `json:"title"`
	Column          string            `json:"column"`
	ColumnTitle     string            `json:"column_title"`
	State           string            `json:"state"`
	Severity        string            `json:"severity"`
	Priority        string            `json:"priority"`
	Route           string            `json:"route"`
	PullDestination string            `json:"pull_destination"`
	Holder          string            `json:"holder"`
	ClaimSince      string            `json:"claim_since"`
	Expires         string            `json:"expires"`
	BlockReason     string            `json:"block_reason"`
	BlockKind       string            `json:"block_kind"`
	Workstreams     []string          `json:"workstreams"`
	Revision        string            `json:"revision"`
	Fields          map[string]string `json:"fields"`
}

// linkView is one link a card carries.
type linkView struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
	To   string `json:"to"`
}

// attachmentView is one attachment a card carries.
type attachmentView struct {
	Ref         string `json:"ref"`
	Filename    string `json:"filename"`
	Description string `json:"description"`
}

// commentView is one entry of a card's comment index.
type commentView struct {
	Ref     string `json:"ref"`
	Ordinal int    `json:"ordinal"`
	TS      string `json:"ts"`
	Author  string `json:"author"`
	Subject string `json:"subject"`
}

// itemView is one checklist item.
type itemView struct {
	Ref         string `json:"ref"`
	Kind        string `json:"kind"`
	State       string `json:"state"`
	ColumnTitle string `json:"column_title"`
	Owner       string `json:"owner"`
	Text        string `json:"text"`
}

// columnShowPayload is the show read's answer for a column, which carries the
// column under record.
type columnShowPayload struct {
	Record struct {
		ID              string `json:"id"`
		Slug            string `json:"slug"`
		Title           string `json:"title"`
		Kind            string `json:"kind"`
		OperatorOwned   bool   `json:"operator_owned"`
		TakesWorkUp     bool   `json:"takes_work_up"`
		AwaitingOutside bool   `json:"awaiting_outside"`
		Capacity        int    `json:"capacity"`
	} `json:"record"`
	Affordances []string `json:"affordances"`
}

// instructionsPayload is the instructions read's answer.
type instructionsPayload struct {
	Served struct {
		Instructions struct {
			Global   string `json:"global"`
			Standing string `json:"standing"`
			Column   string `json:"column"`
		} `json:"instructions"`
		LegalMoves []legalMove `json:"legal_moves"`
	} `json:"served"`
}

// legalMove is one destination a move may name.
type legalMove struct {
	Column    string `json:"column"`
	Ref       string `json:"ref"`
	Title     string `json:"title"`
	Direction string `json:"direction"`
}

// listPayload is a list read's answer, whichever shape it took.
type listPayload struct {
	Listing json.RawMessage `json:"listing"`
	Matches *matchesView    `json:"matches"`
}

// matchesView is what query answers: the cards that matched.
type matchesView struct {
	Cards []cardView `json:"cards"`
}

// searchPayload is the search read's answer.
type searchPayload struct {
	Results struct {
		Hits []searchHit `json:"hits"`
	} `json:"results"`
}

// searchHit is one entity search found.
type searchHit struct {
	Kind        string `json:"kind"`
	Ref         string `json:"ref"`
	Title       string `json:"title"`
	ColumnTitle string `json:"column_title"`
	MatchedIn   string `json:"matched_in"`
	Snippet     string `json:"snippet"`
}

// viewsPayload is the view read's answer with no view named.
type viewsPayload struct {
	Views []struct {
		Name  string `json:"name"`
		Title string `json:"title"`
	} `json:"views"`
}

// viewPayload is the view read's answer for one view.
type viewPayload struct {
	View struct {
		Name     string        `json:"name"`
		Title    string        `json:"title"`
		Layout   string        `json:"layout"`
		Sections []viewSection `json:"sections"`
	} `json:"view"`
}

// viewSection is one section of a drawn view.
type viewSection struct {
	Title          string            `json:"title"`
	Cards          []cardView        `json:"cards"`
	Refused        string            `json:"refused"`
	RefusedDetail  string            `json:"refused_detail"`
	RefusedContext map[string]string `json:"refused_context"`
}

// refusalPayload is the refusal members a response carries.
type refusalPayload struct {
	Outcome string            `json:"outcome"`
	Refusal string            `json:"refusal"`
	Detail  string            `json:"detail"`
	Context map[string]string `json:"context"`
	Card    *cardView         `json:"card"`
}

// affordanceRow is one row of the table GET /affordances publishes.
type affordanceRow struct {
	Affordance string `json:"affordance"`
	Method     string `json:"method"`
	Href       string `json:"href"`
	Form       *struct {
		Method  string            `json:"method"`
		Href    string            `json:"href"`
		Members map[string]string `json:"members"`
	} `json:"form"`
}

// affordanceTable is the document GET /affordances answers.
type affordanceTable struct {
	Affordances []affordanceRow `json:"affordances"`
}
