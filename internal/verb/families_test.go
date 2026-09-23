package verb

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// composedWithoutARequest are the three event names no request-bearing verb
// writes, each with the reason it carries the owner's name and nothing else.
//
// Section 1.8 of dinah-496's contract puts a write with no request through
// bench.NamedActor, which composes from a name and declares nothing. So no
// implementation of that contract can make one of these three carry a harness,
// a provider, a model or a server, and a guard demanding it would be a guard
// nothing could satisfy.
var composedWithoutARequest = map[string]string{
	contract.EventExpired:          "the lapse sweep writes it from the lapsed holder's name, and the caller whose read triggered the sweep is not the owner the line is attributed to",
	contract.EventManualCorrection: "the witness writes it from the name of whoever touched the workbench, reconciling an edit made outside every verb",
	contract.EventRenumbered:       "the two number repairs write it, and the card whose number moved was claimed by nobody",
}

// TestEveryEventFamilyARequestWritesCarriesTheDeclaredMembers drives
// dinah-496's provenance criterion over the whole set rather than over a
// sample.
//
// The expected set is computed from internal/contract's own event names less
// the three above, so an event name added later joins this test with no edit
// here and fails it until somebody drives the family or says why it cannot be
// driven. That is the half round one of Agent Code Review found missing: the
// first draft drove nine families and asserted nine, which is a quarter of the
// population reported as if it were the population.
//
// Every act below is made by one caller declaring all four facts, and the
// assertion afterwards is one rule read over every line every journal in the
// store holds.
func TestEveryEventFamilyARequestWritesCarriesTheDeclaredMembers(t *testing.T) {
	h := tieredHarness(t)
	acting := func(verb string) *Request {
		return &Request{
			Verb: verb, Actor: "alka",
			Harness: "claude-code", Provider: "anthropic",
			Model: "claude-opus-5", Server: "ollama.com",
		}
	}
	run := func(name string, answer *Response) {
		t.Helper()
		if answer.Outcome != contract.OutcomeOK {
			t.Fatalf("%s: %s %s", name, answer.Outcome, answer.Refusal)
		}
		h.reopen()
	}

	// A card that walks the whole route, and a second one to link it to.
	add := acting("add")
	add.Title = "a card driven through every family"
	run("add", h.library.Add(add))
	ref := h.library.Bench.Slug + "-1"
	other := acting("add")
	other.Title = "a card to link to"
	run("add the second card", h.library.Add(other))
	partner := h.library.Bench.Slug + "-2"
	// A third card for the deletion, because deleting the partner would
	// destroy the journal holding its own archived and restored lines and the
	// walk below would then find neither.
	doomed := acting("add")
	doomed.Title = "a card to delete"
	run("add the third card", h.library.Add(doomed))
	condemned := h.library.Bench.Slug + "-3"

	move := acting(Move)
	move.Card = ref
	move.Column = aftercare
	run("move to the station", h.library.Do(move))

	// A workstream to join and leave, and a column to rewrite.
	stream := acting("workstream")
	stream.Action = "new"
	stream.Workstream = "A workstream"
	run("workstream new", h.library.NewWorkstream(stream))
	column := acting("column")
	column.Action = "new"
	column.Column = "A station"
	run("column new", h.library.NewColumn(column))

	steps := []struct {
		name string
		run  func() *Response
	}{
		{contract.EventClaimed, func() *Response {
			req := acting(Claim)
			req.Card = ref
			return h.library.Do(req)
		}},
		{contract.EventReleased, func() *Response {
			req := acting(Release)
			req.Card = ref
			return h.library.Do(req)
		}},
		{contract.EventBlocked, func() *Response {
			req := acting(Block)
			req.Card = ref
			req.Reason = "waiting on the vendor"
			return h.library.Do(req)
		}},
		{contract.EventUnblocked, func() *Response {
			req := acting(Unblock)
			req.Card = ref
			return h.library.Do(req)
		}},
		{contract.EventCommented, func() *Response {
			req := acting("comment")
			req.Card = ref
			req.Text = "a comment"
			return h.library.Comment(req)
		}},
		{contract.EventLinked, func() *Response {
			req := acting("link")
			req.Card = ref
			req.Kind = "relates_to"
			req.LinkTo = partner
			return h.library.Link(req)
		}},
		{contract.EventUnlinked, func() *Response {
			req := acting("unlink")
			req.Card = ref
			req.Kind = "relates_to"
			req.LinkTo = partner
			return h.library.Unlink(req)
		}},
		{contract.EventCardUpdated, func() *Response {
			req := acting("set")
			req.Ref = ref
			req.Field = bench.TierField
			req.Value = "workhorse"
			return h.library.SetField(req)
		}},
		{contract.EventWorkstreamJoined, func() *Response {
			req := acting(Join)
			req.Card = ref
			req.Workstream = "a-workstream"
			return h.library.Do(req)
		}},
		{contract.EventWorkstreamLeft, func() *Response {
			req := acting(Leave)
			req.Card = ref
			req.Workstream = "a-workstream"
			return h.library.Do(req)
		}},
		{contract.EventItemFiled, func() *Response {
			req := acting("file")
			req.Card = ref
			req.Kind = "decision"
			req.Text = "something to settle"
			return h.library.File(req)
		}},
		{contract.EventItemCited, func() *Response {
			req := acting("cite")
			req.Ref = ref + "/decisions/1"
			req.Scheme = "test"
			req.CiteTarget = "somewhere"
			return h.library.Cite(req)
		}},
		{contract.EventItemResolved, func() *Response {
			req := acting("resolve")
			req.Ref = ref + "/decisions/1"
			req.Text = "settled"
			return h.library.Resolve(req)
		}},
		{contract.EventItemReopened, func() *Response {
			req := acting("reopen")
			req.Ref = ref + "/decisions/1"
			req.Reason = "not settled after all"
			return h.library.Reopen(req)
		}},
		{contract.EventItemUpdated, func() *Response {
			req := acting("set")
			req.Ref = ref + "/decisions/1"
			req.Field = "owner"
			req.Value = "holder"
			return h.library.SetField(req)
		}},
		{contract.EventCommentUpdated, func() *Response {
			req := acting("set")
			req.Ref = ref + "/comments/1"
			req.Field = "body"
			req.Value = "a comment, rewritten"
			return h.library.SetField(req)
		}},
		{contract.EventDivergenceAccepted, func() *Response {
			req := acting("accept-divergence")
			req.Ref = ref + "/comments/1"
			return h.library.AcceptDivergence(req)
		}},
		{contract.EventTierOverridden, func() *Response {
			req := acting("set")
			req.Card = ref
			req.At = aftercareSlug
			req.Value = "frontier"
			return h.library.SetCardTierAt(req)
		}},
		{contract.EventWorkbenchUpdated, func() *Response {
			req := acting("set")
			req.Ref = "workbench"
			req.Field = "title"
			req.Value = "Fixture, retitled"
			return h.library.SetField(req)
		}},
		{contract.EventColumnUpdated, func() *Response {
			req := acting("set")
			req.Ref = aftercareSlug
			req.Field = "title"
			req.Value = "Aftercare, retitled"
			return h.library.SetField(req)
		}},
		{contract.EventWorkstreamUpdated, func() *Response {
			req := acting("set")
			req.Ref = "workstream/a-workstream"
			req.Field = "title"
			req.Value = "A workstream, retitled"
			return h.library.SetField(req)
		}},
	}
	for _, step := range steps {
		run(step.name, step.run())
	}

	// The attachment family, which needs a file on disk to copy from.
	payload := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(payload, []byte("bytes"), 0o644); err != nil {
		t.Fatalf("write the payload: %v", err)
	}
	attach := acting("attach")
	attach.Ref = ref
	attach.File = payload
	run(contract.EventAttached, h.library.Attach(attach))
	replace := acting("attach")
	replace.Ref = ref + "/attachments/1"
	replace.File = payload
	replace.Replace = true
	run(contract.EventAttachmentReplaced, h.library.Attach(replace))
	renameAttachment := acting("rename")
	renameAttachment.Ref = ref + "/attachments/1"
	renameAttachment.Value = "renamed.txt"
	run(contract.EventAttachmentRenamed, h.library.Rename(renameAttachment))
	describe := acting("set")
	describe.Ref = ref + "/attachments/1"
	describe.Field = "description"
	describe.Value = "what these bytes are"
	run(contract.EventAttachmentUpdated, h.library.SetField(describe))
	removeAttachment := acting("delete")
	removeAttachment.Ref = ref + "/attachments/1"
	removeAttachment.Confirm = true
	run(contract.EventAttachmentRemoved, h.library.Delete(removeAttachment))

	// The three lifecycle families, driven on the second card so the first
	// one stays where the rest of the run left it.
	archive := acting("archive")
	archive.Ref = partner
	run(contract.EventArchived, h.library.Archive(archive))
	restore := acting("restore")
	restore.Ref = partner
	restore.Archived = true
	run(contract.EventRestored, h.library.Restore(restore))
	remove := acting("delete")
	remove.Ref = condemned
	remove.Confirm = true
	run(contract.EventDeleted, h.library.Delete(remove))

	// The two terminal verbs an acceptance criterion closes with, which no
	// decision reaches: resolve closes a decision and an open question, and
	// verify and fail close a criterion.
	for at, close := range []struct {
		name string
		run  func(*Request) *Response
	}{
		{contract.EventItemVerified, h.library.Verify},
		{contract.EventItemFailed, h.library.Fail},
	} {
		file := acting("file")
		file.Card = ref
		file.Kind = "acceptance_criterion"
		file.Text = "something to check"
		run("file a criterion", h.library.File(file))
		closing := acting(close.name)
		closing.Ref = ref + "/criteria/" + strconv.Itoa(at+1)
		closing.Text = "checked"
		run(close.name, close.run(closing))
	}

	// The two states dinah-472 added, each landed by a verb of its own. Both
	// are the workbench operator's, and the caller above is the operator, so
	// each is filed and settled in the same shape the terminal verbs are.
	for at, close := range []struct {
		name string
		run  func(*Request) *Response
	}{
		{contract.EventItemWaived, h.library.Waive},
		{contract.EventItemWithdrawn, h.library.Withdraw},
	} {
		file := acting("file")
		file.Card = ref
		file.Kind = "acceptance_criterion"
		file.Text = "something the card stopped asking for"
		run("file a criterion", h.library.File(file))
		closing := acting(close.name)
		closing.Ref = ref + "/criteria/" + strconv.Itoa(at+3)
		closing.Text = "the operator decided"
		run(close.name, close.run(closing))
	}

	// The criterion-retirement grant, given and taken back. The revoke runs
	// against the grant the give left standing, because a revoke over a card
	// carrying none is refused and would write no line at all.
	grant := acting(GrantPermission)
	grant.Card = ref
	grant.Permission = CriterionRetirement
	run(contract.EventRetirementGranted, h.library.Do(grant))
	ungrant := acting(RevokePermission)
	ungrant.Card = ref
	ungrant.Permission = CriterionRetirement
	run(contract.EventRetirementRevoked, h.library.Do(ungrant))

	// The designation conversion's own account of the claims it passed, which
	// the forced form writes on the workbench's own journal. The store is
	// already at the current format, so the run converts nothing and the line
	// it writes is the one this family is about: a forced run says so whether
	// or not it passed a claim.
	converted := acting("check")
	converted.MigrateDesignations = true
	converted.ForceClaims = true
	if _, err := h.library.Check(converted); err != nil {
		t.Fatalf("%s: %v", contract.EventDesignationsMigrated, err)
	}
	h.reopen()

	// The override drop, which only a reshape writes: the station the card
	// carries an override for is left out of the new definition, so the
	// override goes with the column.
	report, err := h.library.Reshape(&Request{
		Verb: "reshape", Actor: "alka", From: h.source(reshapedWithoutAftercare),
		Map: []string{aftercare + "=" + review}, Confirm: true,
		Harness: "claude-code", Provider: "anthropic",
		Model: "claude-opus-5", Server: "ollama.com",
	})
	if err != nil {
		t.Fatalf("reshape: %v", err)
	}
	if report == nil {
		t.Fatal("the reshape answered no report")
	}
	h.reopen()

	// One rule, read over every line of every journal the store holds.
	families, lines := map[string]bool{}, 0
	err = filepath.Walk(h.root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Base(path) != bench.JournalName {
			return err
		}
		events, _, readErr := bench.ReadJournal(path)
		if readErr != nil {
			return readErr
		}
		for _, event := range events {
			lines++
			if _, nameless := composedWithoutARequest[event.Event]; nameless {
				continue
			}
			families[event.Event] = true
			if event.Actor.Harness != "claude-code" || event.Actor.Provider != "anthropic" ||
				event.Actor.Model != "claude-opus-5" || event.Actor.Server != "ollama.com" {
				t.Errorf("a %s line in %s carries %+v, wanted every declared member",
					event.Event, filepath.Base(filepath.Dir(path)), event.Actor)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the journals: %v", err)
	}
	if lines == 0 {
		t.Fatal("the walk read no journal line, so it is asserting nothing")
	}

	var missing []string
	for _, name := range contract.EventNames() {
		if _, nameless := composedWithoutARequest[name]; nameless {
			continue
		}
		if !families[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d event families a request-bearing verb writes were not driven, so nothing here asserts they carry the declared members: %s",
			len(missing), strings.Join(missing, ", "))
	}
	if len(families)+len(composedWithoutARequest) != len(contract.EventNames()) {
		t.Errorf("the run drove %d families beside the %d composed without a request, and this build declares %d event names",
			len(families), len(composedWithoutARequest), len(contract.EventNames()))
	}
}

// reshapedWithoutAftercare is the fixture's flow with the aftercare station
// left out, which is what makes a reshape drop the override the card carries
// for it.
const reshapedWithoutAftercare = `{
  "profile": "dinah-core/0.7",
  "title": "Fixture",
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake" },
    { "id": "a00000000002", "title": "Doing", "kind": "work" },
    { "id": "a00000000003", "title": "Review", "kind": "work" },
    { "id": "a00000000004", "title": "Finished", "kind": "done" },
    { "id": "a00000000006", "title": "Closed", "kind": "done" }
  ]
}`
