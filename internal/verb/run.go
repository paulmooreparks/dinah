package verb

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// Run is the name of the command that runs one agent session on a card. The
// terminal head launches the process; this file holds the parts of the
// lifecycle that read and write the workbench, so the refusals and the
// bookkeeping are the library's like every other command's.
const Run = "run"

// The four results an agent may report at the end of its final text.
const (
	RunForward = "forward"
	RunBack    = "back"
	RunBlock   = "block"
	RunStay    = "stay"
)

// RunPlan is what a run decides before it launches anything: the card and its
// column, the recipe the column names, whether this run resumes a stored
// session, and whether the session it ends with is kept on the card.
type RunPlan struct {
	// Card is the card as it stood when the plan was made.
	Card *bench.Card
	// Ref is the card's reference, which every library call the run makes
	// names it by.
	Ref string
	// Column is the column the card stands in, whose recipe runs.
	Column *bench.Column
	// Recipe is the recipe the column names.
	Recipe *bench.RunRecipe
	// Worker is the column's declared worker, continue or fresh.
	Worker string
	// PushBack is true where the card's last move was a rejection into this
	// column, which resumes the stored session whatever the column declares,
	// because the findings are addressed to the agent that did the work.
	PushBack bool
	// Resume is true where the run continues Session rather than starting a
	// new session.
	Resume bool
	// Session is the stored session the run resumes, empty on a fresh run.
	Session string
	// Keep is true where the session this run ends with is stored on the
	// card. A fresh run at a fresh column is not kept, so a reviewer's
	// session never displaces the session of the agent the card goes back to.
	Keep bool
	// Cumulative is the last cumulative figure stored for Session, nil where
	// none is stored or the run is fresh.
	Cumulative *float64
	// Round is which pass of this column the run is, counted from the spend
	// lines the card already carries for the column.
	Round int
}

// PlanRun resolves a run request into a plan, or into the refusal it earns.
// The rows run in the order checks.go lists for run, behind the harness row
// every writing command runs first: the card exists, the
// request names an owner, the card is not blocked and not held by somebody
// else, its column names a recipe, the recipe and the column's worker are
// well formed, and the workbench declares the card fields the run writes.
func (l *Library) PlanRun(req *Request) (*RunPlan, *Response) {
	if refused := l.malformedHarness(req, nil); refused != nil {
		return nil, refused
	}
	// The reference goes to the general resolver, on the terms canSpend sends
	// its own there, so a reference naming a collection is refused as the
	// collection it is and a blank one is refused ahead of the resolver.
	if strings.TrimSpace(req.Card) == "" {
		return nil, l.refuse(req, nil, contract.UnknownCard, "")
	}
	resolved, err := l.Bench.ResolveEntity(req.Card)
	if err != nil {
		return nil, l.FromError(req, err)
	}
	if resolved.Kind != bench.KindCard || resolved.Card == nil {
		return nil, l.refuse(req, resolved.Card, contract.UnknownCard, req.Card)
	}
	card := resolved.Card
	if req.Actor == "" {
		return nil, l.refuse(req, card, contract.NoOwner, "")
	}
	if card.State == contract.StateBlocked {
		return nil, l.refuse(req, card, contract.Blocked, card.BlockReason)
	}
	if card.Holder != "" && card.Holder != req.Actor {
		return nil, l.refuse(req, card, contract.Held, card.Holder)
	}
	column := l.Bench.Column(card.Column)
	name := bench.ColumnRunRecipe(column)
	if column == nil || name == "" {
		detail := card.Column
		if column != nil {
			detail = column.Ref()
		}
		return nil, l.refuse(req, card, contract.NoRunRecipe, detail)
	}
	recipes, defects := l.Bench.RunRecipes()
	for _, defect := range defects {
		if defect.Name == name {
			return nil, l.refuse(req, card, contract.Malformed, bench.RunKey+"."+name+"."+defect.Member)
		}
	}
	recipe := recipes[name]
	if recipe == nil {
		return nil, l.refuse(req, card, contract.NoRunRecipe, column.Ref())
	}
	worker, ok := bench.ColumnWorker(column)
	if !ok {
		return nil, l.refuse(req, card, contract.Malformed, bench.WorkerKey)
	}
	if refused := l.runFieldsDeclared(req, card, recipe); refused != nil {
		return nil, refused
	}
	plan := &RunPlan{
		Card:   card,
		Ref:    card.Ref(l.Bench.Slug),
		Column: column,
		Recipe: recipe,
		Worker: worker,
	}
	events, _, err := l.Bench.ReadJournal(card.JournalPath())
	if err != nil {
		return nil, l.FromError(req, err)
	}
	plan.Round = 1
	for _, ev := range events {
		switch {
		case ev.Event == contract.EventSpend && ev.Column == column.ID:
			plan.Round++
		case ev.Event == contract.EventMoved:
			plan.PushBack = ev.Reject && ev.To == column.ID
		}
	}
	entity, err := l.Bench.ResolveEntity(plan.Ref)
	if err != nil {
		return nil, l.FromError(req, err)
	}
	fm, _, err := l.entityAnchor(entity)
	if err != nil {
		return nil, l.FromError(req, err)
	}
	stored := func(key string) string { return bench.FieldValue(fm, key) }
	session := stored(bench.RunSessionField)
	continues := worker == bench.WorkerContinue || plan.PushBack
	plan.Keep = continues
	if continues && session != "" && len(recipe.Resume) > 0 && stored(bench.RunRecipeField) == recipe.Name {
		plan.Resume = true
		plan.Session = session
		if spend := recipe.Receipt.Spend; spend != nil && spend.Cumulative {
			if figure, err := strconv.ParseFloat(stored(bench.RunCumulativeField(spend.Unit)), 64); err == nil {
				plan.Cumulative = &figure
			}
		}
	}
	return plan, nil
}

// runFieldsDeclared refuses a run on a workbench that does not declare, on
// cards, the fields the run writes: run.session and run.recipe as strings,
// and run.cumulative.<unit> as a number where the recipe's spend is
// cumulative. The run refuses before it claims anything rather than finding
// out after the agent has worked, because a session it cannot store is a
// session the next run cannot resume.
func (l *Library) runFieldsDeclared(req *Request, card *bench.Card, recipe *bench.RunRecipe) *Response {
	wanted := []struct{ key, typ string }{
		{bench.RunSessionField, bench.FieldTypeString},
		{bench.RunRecipeField, bench.FieldTypeString},
	}
	if spend := recipe.Receipt.Spend; spend != nil && spend.Cumulative {
		wanted = append(wanted, struct{ key, typ string }{bench.RunCumulativeField(spend.Unit), bench.FieldTypeNumber})
	}
	for _, want := range wanted {
		declared := l.Bench.DeclaredFieldOf(want.key)
		if declared == nil || !declared.Declares(bench.KindCard) {
			return l.FromError(req, undeclaredField(l.Bench, bench.KindCard, want.key, declared))
		}
		if declared.Type != want.typ || len(declared.Values) > 0 {
			return l.refuse(req, card, contract.Malformed, want.key)
		}
	}
	return nil
}

// RunReceiptRead is what a run read off the harness's receipt.
type RunReceiptRead struct {
	// Session is the harness's session identifier, empty where the recipe
	// names no session member or the receipt carried none.
	Session string
	// Text is the agent's final text.
	Text string
	// Error is true where the recipe names an error member and the receipt
	// set it.
	Error bool
	// Figure is the spend figure as the receipt reported it, nil where the
	// recipe declares no spend or the receipt carried no number there.
	Figure *float64
}

// ReadRunReceipt reads a receipt off a command's standard output: the whole
// output where it is one JSON object, and otherwise the last line that is
// one, which is where a harness printing a stream of events puts its result.
// It answers false where no object carries the recipe's text member as a
// string, which is the receipt a run calls unreadable.
func ReadRunReceipt(stdout []byte, receipt bench.RunReceipt) (*RunReceiptRead, bool) {
	object, ok := lastJSONObject(stdout)
	if !ok {
		return nil, false
	}
	text, ok := receiptMember(object, receipt.Text).(string)
	if !ok {
		return nil, false
	}
	read := &RunReceiptRead{Text: text}
	if receipt.Session != "" {
		read.Session, _ = receiptMember(object, receipt.Session).(string)
	}
	if receipt.Error != "" {
		read.Error, _ = receiptMember(object, receipt.Error).(bool)
	}
	if receipt.Spend != nil {
		if figure, ok := receiptMember(object, receipt.Spend.Field).(float64); ok && figure >= 0 && !math.IsInf(figure, 0) {
			read.Figure = &figure
		}
	}
	return read, true
}

// lastJSONObject answers the output as one JSON object, or the last line of
// it that is one.
func lastJSONObject(stdout []byte) (map[string]any, bool) {
	var object map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(stdout), &object); err == nil && object != nil {
		return object, true
	}
	lines := bytes.Split(stdout, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		object = nil
		if err := json.Unmarshal(line, &object); err == nil && object != nil {
			return object, true
		}
	}
	return nil, false
}

// receiptMember walks a dotted path into a decoded object, answering nil
// where any step is absent or is not an object.
func receiptMember(object map[string]any, path string) any {
	var at any = object
	for _, step := range strings.Split(path, ".") {
		mapping, ok := at.(map[string]any)
		if !ok {
			return nil
		}
		at = mapping[step]
	}
	return at
}

// RunOutcome is the result an agent reported, and the text before it.
type RunOutcome struct {
	// Outcome is one of the four results, stay where the agent reported none.
	Outcome string `json:"outcome"`
	// Reason is the agent's own reason, empty where it gave none.
	Reason string `json:"reason,omitempty"`
	// Handoff is the text before the result block, or the whole text where
	// there is no block.
	Handoff string `json:"-"`
	// Reported is false where the text carried no readable result block.
	Reported bool `json:"-"`
}

// runFence is the opening of the block an agent ends its text with.
const runFence = "```json"

// ParseRunOutcome reads the result block off the end of an agent's final
// text: a fenced json block, last in the text, carrying an outcome and a
// reason. A text with no block, or with one that does not read as one of the
// four outcomes, is a stay, and its whole text is the handoff.
func ParseRunOutcome(text string) RunOutcome {
	stay := RunOutcome{Outcome: RunStay, Handoff: strings.TrimSpace(text)}
	trimmed := strings.TrimRight(text, " \t\r\n")
	if !strings.HasSuffix(trimmed, "```") {
		return stay
	}
	body := trimmed[:len(trimmed)-3]
	open := strings.LastIndex(body, runFence)
	if open < 0 {
		return stay
	}
	payload := body[open+len(runFence):]
	if !strings.HasPrefix(strings.TrimLeft(payload, " \t"), "\n") && !strings.HasPrefix(strings.TrimLeft(payload, " \t"), "\r\n") {
		return stay
	}
	var reported RunOutcome
	if err := json.Unmarshal([]byte(strings.TrimSpace(payload)), &reported); err != nil {
		return stay
	}
	switch reported.Outcome {
	case RunForward, RunBack, RunBlock, RunStay:
	default:
		return stay
	}
	reported.Reason = strings.TrimSpace(reported.Reason)
	reported.Handoff = strings.TrimSpace(body[:open])
	reported.Reported = true
	return reported
}

// RunSpent is the spend line a run recorded.
type RunSpent struct {
	Unit       string   `json:"unit"`
	Figure     *float64 `json:"figure,omitempty"`
	Unreported bool     `json:"unreported,omitempty"`
}

// RecordRun writes what a run learned onto the card: the session and recipe
// where the plan keeps them, the cumulative figure the next resumed run
// subtracts, and the spend line. A receipt that could not be read records the
// line as unreported and stores nothing, so the next run starts fresh rather
// than resuming a session nobody could name.
//
// The answers are the spend line written and the first refusal any of the
// writes met; a refusal stops the writes that follow it.
func (l *Library) RecordRun(req *Request, plan *RunPlan, read *RunReceiptRead) (*RunSpent, *Response) {
	spend := plan.Recipe.Receipt.Spend
	if read != nil && plan.Keep {
		// A receipt naming no session leaves nothing the next run can resume,
		// so the stored session and its baseline are cleared rather than left
		// to be subtracted from a session they do not belong to. A session
		// whose receipt carried no figure clears the baseline for the same
		// reason. An empty value is a clear.
		session, recipe, baseline := "", "", ""
		if read.Session != "" {
			session, recipe = read.Session, plan.Recipe.Name
			if read.Figure != nil {
				baseline = formatFigure(*read.Figure)
			}
		}
		writes := []struct{ key, value string }{
			{bench.RunSessionField, session},
			{bench.RunRecipeField, recipe},
		}
		if spend != nil && spend.Cumulative {
			writes = append(writes, struct{ key, value string }{bench.RunCumulativeField(spend.Unit), baseline})
		}
		for _, write := range writes {
			set := *req
			set.Verb = "set"
			set.Ref, set.Field, set.Value = plan.Ref, write.key, write.value
			if response := l.SetField(&set); response.Outcome != contract.OutcomeOK {
				return nil, response
			}
		}
	}
	if spend == nil {
		return nil, nil
	}
	spent := &RunSpent{Unit: spend.Unit}
	line := *req
	line.Verb = "spend"
	line.Card, line.Unit = plan.Ref, spend.Unit
	line.Round = strconv.Itoa(plan.Round)
	line.Note = Run + " " + plan.Recipe.Name
	line.Column = ""
	if read == nil || read.Figure == nil {
		spent.Unreported = true
		line.Unreported = true
	} else {
		figure := *read.Figure
		if spend.Cumulative && plan.Resume && plan.Cumulative != nil && figure >= *plan.Cumulative {
			figure -= *plan.Cumulative
		}
		figure = math.Round(figure*1e9) / 1e9
		spent.Figure = &figure
		line.Total = formatFigure(figure)
	}
	if response := l.Spend(&line); response.Outcome != contract.OutcomeOK {
		return spent, response
	}
	return spent, nil
}

// formatFigure writes a figure in the shortest form that reads back as the
// same number.
func formatFigure(figure float64) string {
	return strconv.FormatFloat(figure, 'f', -1, 64)
}
