package verb

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// sourceFile writes bytes to a file outside the workbench, the way a person
// hands attach a path, and answers the path.
func sourceFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// definitionAct is one of the seven writes an attachment's holder authority
// governs, run against a harness by whichever owner the case names.
type definitionAct struct {
	// name is what a failure calls the act.
	name string
	// run performs the act as actor and answers its response.
	run func(h *harness, actor string) *Response
}

// definitionActs are the seven acts on an attachment hanging on holder, which
// is a column's reference or the workbench's. Each one names the attachment by
// the position the setup in TestOnlyTheOperatorWritesADefinitionAttachment
// leaves it at: the first live attachment, and for the restore the one
// archived attachment, which is the first of the archive mirror's own
// collection.
func definitionActs(holder string) []definitionAct {
	first := holder + "/attachments/1"
	return []definitionAct{
		{name: "attach", run: func(h *harness, actor string) *Response {
			file := sourceFile(h.t, "new.md", "new bytes\n")
			return h.library.Attach(&Request{Verb: "attach", Actor: actor, Ref: holder, File: file})
		}},
		{name: "attach --replace", run: func(h *harness, actor string) *Response {
			file := sourceFile(h.t, "first.md", "replaced bytes\n")
			return h.library.Attach(&Request{Verb: "attach", Actor: actor, Ref: first, File: file, Replace: true})
		}},
		{name: "rename", run: func(h *harness, actor string) *Response {
			return h.library.Rename(&Request{Verb: "rename", Actor: actor, Ref: first, Value: "renamed.md"})
		}},
		{name: "set description", run: func(h *harness, actor string) *Response {
			return h.library.SetField(&Request{Verb: "set", Actor: actor, Ref: first, Field: bench.DescriptionField, Value: "changed"})
		}},
		{name: "restore", run: func(h *harness, actor string) *Response {
			return h.library.Restore(&Request{Verb: "restore", Actor: actor, Ref: holder + "/attachments/1"})
		}},
		{name: "archive", run: func(h *harness, actor string) *Response {
			return h.library.Archive(&Request{Verb: "archive", Actor: actor, Ref: first})
		}},
		{name: "delete", run: func(h *harness, actor string) *Response {
			return h.library.Delete(&Request{Verb: "delete", Actor: actor, Ref: first, Confirm: true})
		}},
	}
}

// TestOnlyTheOperatorWritesADefinitionAttachment asserts dinah-545/criteria/20
// on both holders whose own fields are the operator's, a column and the
// workbench. For each of the seven acts a non-operator is refused not-operator
// and neither the workbench's tree nor its journal changes by a byte, and the
// operator's same act then succeeds, which is the accepting case a refusal
// that refused everything would fail. A card's attachment stays open to any
// owner, both to attach and to rename.
func TestOnlyTheOperatorWritesADefinitionAttachment(t *testing.T) {
	holders := []string{aftercareSlug, bench.WorkbenchRef}
	for _, holder := range holders {
		t.Run(holder, func(t *testing.T) {
			h := newHarness(t)
			h.attach(holder, "first.md", "first bytes\n")
			h.attach(holder, "second.md", "second bytes\n")
			if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: holder + "/attachments/2"}); response.Outcome != contract.OutcomeOK {
				t.Fatalf("archive the second attachment: %s %s", response.Outcome, response.Refusal)
			}
			h.reopen()
			acts := definitionActs(holder)
			if len(acts) != 7 {
				t.Fatalf("the criterion names seven acts and this sweep drives %d", len(acts))
			}
			for _, act := range acts {
				before := h.digest()
				refused := act.run(h, "bob")
				h.reopen()
				if refused.Refusal != contract.NotOperator {
					t.Errorf("%s by a non-operator: wanted %s, got %s %s", act.name, contract.NotOperator, refused.Outcome, refused.Refusal)
				}
				if after := h.digest(); after != before {
					t.Errorf("%s by a non-operator wrote to the workbench", act.name)
				}
				accepted := act.run(h, "alka")
				h.reopen()
				if accepted.Outcome != contract.OutcomeOK {
					t.Errorf("%s by the operator: wanted ok, got %s %s", act.name, accepted.Outcome, accepted.Refusal)
				}
			}
		})
	}

	t.Run("a card's attachment", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("a card")
		file := sourceFile(t, "notes.md", "notes\n")
		if response := h.library.Attach(&Request{Verb: "attach", Actor: "bob", Ref: ref, File: file}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("a non-operator's attach to a card: wanted ok, got %s %s", response.Outcome, response.Refusal)
		}
		h.reopen()
		if response := h.library.Rename(&Request{Verb: "rename", Actor: "bob", Ref: ref + "/attachments/1", Value: "renamed.md"}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("a non-operator's rename of a card attachment: wanted ok, got %s %s", response.Outcome, response.Refusal)
		}
	})
}

// TestOnlyTheOperatorArchivesRestoresOrDeletesAColumn asserts
// dinah-545/criteria/30. A non-operator's archive, restore and delete of a
// column are each refused not-operator, move no directory and append no
// journal line, and the operator's same three acts succeed. A non-operator
// still archives and deletes a card.
func TestOnlyTheOperatorArchivesRestoresOrDeletesAColumn(t *testing.T) {
	h := newHarness(t)
	type columnAct struct {
		name string
		run  func(actor string) *Response
	}
	acts := []columnAct{
		{name: "archive", run: func(actor string) *Response {
			return h.library.Archive(&Request{Verb: "archive", Actor: actor, Ref: aftercareSlug})
		}},
		{name: "restore", run: func(actor string) *Response {
			return h.library.Restore(&Request{Verb: "restore", Actor: actor, Ref: aftercareSlug})
		}},
		{name: "delete", run: func(actor string) *Response {
			return h.library.Delete(&Request{Verb: "delete", Actor: actor, Ref: aftercareSlug, Confirm: true})
		}},
	}
	for _, act := range acts {
		before := h.digest()
		refused := act.run("bob")
		h.reopen()
		if refused.Refusal != contract.NotOperator {
			t.Errorf("%s of a column by a non-operator: wanted %s, got %s %s", act.name, contract.NotOperator, refused.Outcome, refused.Refusal)
		}
		if after := h.digest(); after != before {
			t.Errorf("%s of a column by a non-operator wrote to the workbench", act.name)
		}
		accepted := act.run("alka")
		h.reopen()
		if accepted.Outcome != contract.OutcomeOK {
			t.Errorf("%s of a column by the operator: wanted ok, got %s %s", act.name, accepted.Outcome, accepted.Refusal)
		}
	}

	archived := h.add("a card to archive")
	if response := h.library.Archive(&Request{Verb: "archive", Actor: "bob", Ref: archived}); response.Outcome != contract.OutcomeOK {
		t.Errorf("a non-operator's archive of a card: wanted ok, got %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	deleted := h.add("a card to delete")
	if response := h.library.Delete(&Request{Verb: "delete", Actor: "bob", Ref: deleted, Confirm: true}); response.Outcome != contract.OutcomeOK {
		t.Errorf("a non-operator's delete of a card: wanted ok, got %s %s", response.Outcome, response.Refusal)
	}
}

// TestAColumnAttachmentLineCarriesTheColumnLocator asserts
// dinah-545/criteria/10. Every line the seven acts write about an attachment
// hanging on a column carries the column's identifier and its title as of the
// write, a retitle between two acts included. A line about an attachment on
// the workbench itself carries neither, and neither does a card attachment's
// line on its card's journal.
func TestAColumnAttachmentLineCarriesTheColumnLocator(t *testing.T) {
	h := newHarness(t)
	h.attach(aftercareSlug, "first.md", "first bytes\n")
	h.attach(aftercareSlug, "second.md", "second bytes\n")
	h.attach(bench.WorkbenchRef, "first.md", "first bytes\n")
	h.attach(bench.WorkbenchRef, "second.md", "second bytes\n")

	// The second attachment is archived and restored and the first is put
	// through every other act, so each of the seven events is written once
	// per holder.
	sequence := func(holder string) {
		second := holder + "/attachments/2"
		for _, response := range []*Response{
			h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: second}),
			h.library.Restore(&Request{Verb: "restore", Actor: "alka", Ref: holder + "/attachments/1"}),
		} {
			if response.Outcome != contract.OutcomeOK {
				t.Fatalf("archive and restore on %s: %s %s", holder, response.Outcome, response.Refusal)
			}
		}
		h.reopen()
		for _, act := range definitionActs(holder)[1:4] {
			if response := act.run(h, "alka"); response.Outcome != contract.OutcomeOK {
				t.Fatalf("%s on %s: %s %s", act.name, holder, response.Outcome, response.Refusal)
			}
			h.reopen()
		}
		if response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: holder + "/attachments/1", Confirm: true}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("delete on %s: %s %s", holder, response.Outcome, response.Refusal)
		}
		h.reopen()
	}
	sequence(bench.WorkbenchRef)
	// The column is retitled before its own sequence, so every one of its
	// lines after the two attaches names the new title and the two attached
	// lines name the old one.
	if response := h.library.SetField(&Request{Verb: "set", Actor: "alka", Ref: aftercareSlug, Field: bench.TitleField, Value: "Aftercare, renamed"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("retitle the column: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	sequence(aftercareSlug)

	attachmentEvents := map[string]bool{
		contract.EventAttached:           true,
		contract.EventAttachmentReplaced: true,
		contract.EventAttachmentRenamed:  true,
		contract.EventAttachmentRemoved:  true,
		contract.EventAttachmentUpdated:  true,
		contract.EventArchived:           true,
		contract.EventRestored:           true,
	}
	seen := map[string]int{}
	onWorkbench, onColumn := 0, 0
	for _, ev := range h.benchEvents() {
		if !attachmentEvents[ev.Event] {
			continue
		}
		seen[ev.Event]++
		if ev.Column == "" {
			onWorkbench++
			if ev.ColumnTitle != "" {
				t.Errorf("a workbench attachment's %s line carries column_title %q", ev.Event, ev.ColumnTitle)
			}
			continue
		}
		onColumn++
		if ev.Column != aftercare {
			t.Errorf("a %s line names the column %q, wanted %q", ev.Event, ev.Column, aftercare)
		}
		wantTitle := "Aftercare, renamed"
		if ev.Event == contract.EventAttached {
			wantTitle = "Aftercare"
		}
		if ev.ColumnTitle != wantTitle {
			t.Errorf("a %s line names the title %q, wanted %q", ev.Event, ev.ColumnTitle, wantTitle)
		}
	}
	if len(seen) != len(attachmentEvents) {
		t.Errorf("wanted all seven events written, got %v", seen)
	}
	// Each holder wrote two attached lines and one of each of the other six.
	if onWorkbench != 8 || onColumn != 8 {
		t.Errorf("wanted eight lines per holder, got %d on the workbench and %d on the column", onWorkbench, onColumn)
	}

	ref := h.add("a card")
	h.attach(ref, "notes.md", "notes\n")
	for _, ev := range h.events(ref) {
		if ev.Event == contract.EventAttached && (ev.Column != "" || ev.ColumnTitle != "") {
			t.Errorf("a card attachment's line carries a column locator: %+v", ev)
		}
	}
}

// addsWithAttachments keeps every fixture column and adds Z, whose element
// carries two attachments, one of them holding a carriage return, a line feed
// and a NUL byte, and replaces aftercare's element with one carrying an
// attachment of its own, which a kept column must ignore.
const addsWithAttachments = `{
  "profile": "dinah-core/0.7",
  "title": "Fixture",
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake",
      "instructions": "Intake instructions.\n" },
    { "id": "new", "title": "Zed", "kind": "work",
      "attachments": [
        { "filename": "one.md", "description": "the first", "payload": "b25lCg==" },
        { "filename": "two.bin", "payload": "YQ0KAGI=" }
      ] },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "capacity": 1,
      "instructions": "Doing instructions.\n" },
    { "id": "a00000000003", "title": "Review", "kind": "work", "operator_owned": true,
      "instructions": "Review instructions.\n" },
    { "id": "a00000000005", "title": "Aftercare", "kind": "work",
      "instructions": "Aftercare instructions.\n",
      "attachments": [ { "filename": "ignored.md", "payload": "aWdub3JlZAo=" } ] },
    { "id": "a00000000004", "title": "Finished", "kind": "done",
      "instructions": "Finished instructions.\n" },
    { "id": "a00000000006", "title": "Closed", "kind": "done",
      "instructions": "Closed instructions.\n" }
  ]
}`

// addedPayloads are the bytes addsWithAttachments carries for Z, decoded.
var addedPayloads = map[string]string{
	"one.md":  "one\n",
	"two.bin": "a\r\n\x00b",
}

// wantZedListing fails unless the column at dir carries exactly the two
// attachments addsWithAttachments declares for Z, in order and byte for byte.
func wantZedListing(t *testing.T, what, dir string) {
	t.Helper()
	attachments, err := bench.Attachments(dir)
	if err != nil {
		t.Fatalf("%s: list Z's attachments: %v", what, err)
	}
	if len(attachments) != 2 {
		t.Fatalf("%s: wanted Z to carry two attachments, got %d", what, len(attachments))
	}
	for position, want := range []string{"one.md", "two.bin"} {
		got := attachments[position]
		if got.Filename != want {
			t.Errorf("%s: attachment %d is %q, wanted %q", what, position+1, got.Filename, want)
		}
		data, err := os.ReadFile(got.Path)
		if err != nil {
			t.Fatalf("%s: read %s: %v", what, got.Path, err)
		}
		if string(data) != addedPayloads[want] {
			t.Errorf("%s: %s holds %q, wanted %q", what, want, data, addedPayloads[want])
		}
	}
	if attachments[0].Description != "the first" {
		t.Errorf("%s: the first attachment's description is %q", what, attachments[0].Description)
	}
}

// TestAReshapeAddingAColumnWritesItsAttachmentsOnce asserts
// dinah-545/criteria/19 and the kept half of dinah-545/criteria/9. A reshape
// adding Z leaves Z listing both attachments with byte-identical payloads and
// two attached lines carrying Z's locator; the same reshape run again, and a
// run whose first attempt stopped after Z's first attachment, each leave Z
// with exactly two. A kept column's attachments are untouched even though the
// new definition's element for it carries a different attachments member.
func TestAReshapeAddingAColumnWritesItsAttachmentsOnce(t *testing.T) {
	h := newHarness(t)
	h.attach(aftercareSlug, "kept.md", "kept bytes\n")
	keptBefore := treeDigest(t, filepath.Join(h.library.Bench.ColumnDir(aftercare), bench.AttachmentsDir))
	source := h.source(addsWithAttachments)
	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read the definition: %v", err)
	}
	zed := bench.DeriveColumnID(bench.SourceDigest(body), 1)

	if _, err := h.reshape(source, true); err != nil {
		t.Fatalf("the reshape: %v", err)
	}
	zedDir := h.library.Bench.ColumnDir(zed)
	wantZedListing(t, "the first run", zedDir)
	// The harness attached kept.md to aftercare through the verb, which
	// writes a line of its own under aftercare's locator, so the reshape's
	// lines are the attached lines naming any other column.
	attached := 0
	for _, ev := range h.benchEvents() {
		if ev.Event != contract.EventAttached || ev.Column == aftercare {
			continue
		}
		attached++
		if ev.Column != zed || ev.ColumnTitle != "Zed" {
			t.Errorf("an attached line carries the locator %q %q, wanted %q Zed", ev.Column, ev.ColumnTitle, zed)
		}
	}
	if attached != 2 {
		t.Errorf("wanted two attached lines from the reshape, got %d", attached)
	}
	if after := treeDigest(t, filepath.Join(h.library.Bench.ColumnDir(aftercare), bench.AttachmentsDir)); !maps.Equal(after, keptBefore) {
		t.Error("the reshape changed the kept column's attachments")
	}

	if _, err := h.reshape(source, true); err != nil {
		t.Fatalf("the second run: %v", err)
	}
	wantZedListing(t, "the second run", zedDir)

	// The interrupted run is planted as the state it leaves: Z's anchor and
	// its first attachment written, and Z absent from the sequence.
	interrupted := newHarness(t)
	interruptedSource := interrupted.source(addsWithAttachments)
	definition, err := bench.ReadDefinition([]byte(addsWithAttachments))
	if err != nil {
		t.Fatalf("read the definition: %v", err)
	}
	if err := bench.WriteColumnFromElement(interrupted.root, zed, "zed", definition.Columns[1]); err != nil {
		t.Fatalf("plant Z's anchor: %v", err)
	}
	plantedDir := interrupted.library.Bench.ColumnDir(zed)
	if _, err := bench.AddAttachmentBytes(plantedDir, "one.md", []byte("one\n"), "the first", "alka"); err != nil {
		t.Fatalf("plant Z's first attachment: %v", err)
	}
	interrupted.reopen()
	if countIn(interrupted.sequence(), zed) != 0 {
		t.Fatal("the planted state already carries Z in its sequence, so it is not the interrupted state")
	}
	if _, err := interrupted.reshape(interruptedSource, true); err != nil {
		t.Fatalf("the retry: %v", err)
	}
	wantZedListing(t, "the retry", plantedDir)
}

// TestARetiredColumnKeepsItsAttachmentsInTheArchive asserts the retired half
// of dinah-545/criteria/9: a reshape retiring a column carrying an attachment
// leaves the attachment's payload, byte for byte, under the column's archive
// directory.
func TestARetiredColumnKeepsItsAttachmentsInTheArchive(t *testing.T) {
	h := newHarness(t)
	h.attach(aftercareSlug, "kept.bin", "a\r\n\x00b")
	attachments, err := bench.Attachments(h.library.Bench.ColumnDir(aftercare))
	if err != nil || len(attachments) != 1 {
		t.Fatalf("the fixture's attachment: %v %d", err, len(attachments))
	}
	id := attachments[0].ID

	if _, err := h.reshape(h.source(dropsAftercare), true); err != nil {
		t.Fatalf("the reshape: %v", err)
	}
	archived := filepath.Join(h.root, bench.ArchiveDir, bench.ColumnsDir, aftercare, bench.AttachmentsDir, id, bench.PayloadDir, "kept.bin")
	data, err := os.ReadFile(archived)
	if err != nil {
		t.Fatalf("the archived payload: %v", err)
	}
	if string(data) != "a\r\n\x00b" {
		t.Errorf("the archived payload holds %q", data)
	}
}

// TestTheListingIsServedAfterTheColumnAndOnlyWhereThereIsOne asserts the
// library half of dinah-545/criteria/2 and dinah-545/criteria/5: a move into a
// column carrying an attachment serves that one entry, and a move into a
// column carrying none, or carrying only an archived one, serves no listing
// and never names the listing withheld, even to a connection holding every
// key the first serve recorded.
func TestTheListingIsServedAfterTheColumnAndOnlyWhereThereIsOne(t *testing.T) {
	h := newHarness(t)
	h.attach(aftercareSlug, "notes.md", "notes\n")
	h.attach("doing", "gone.md", "gone\n")
	if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: "doing/attachments/1"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("archive: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	ref := h.add("mover")

	into := h.mustDo(&Request{Verb: Move, Actor: "alka", Card: ref, Column: aftercare})
	listing := into.Instructions.ColumnAttachments
	if len(listing) != 1 || listing[0].Ref != aftercareSlug+"/attachments/1" || listing[0].Filename != "notes.md" {
		t.Fatalf("wanted the one attachment listed, got %+v", listing)
	}
	held := map[string]bool{}
	for _, key := range into.ChainServed {
		held[key] = true
	}
	// The held set grows the way a connection's does, so the second empty
	// column is served to a connection already holding every key the first
	// one recorded, which is the position where an empty listing recorded as
	// a layer of its own would come back named withheld.
	for _, column := range []string{doing, intake} {
		moved := h.library.Do(&Request{Verb: Move, Actor: "alka", Card: ref, Column: column, HeldChain: held})
		h.reopen()
		for _, key := range moved.ChainServed {
			held[key] = true
		}
		if moved.Outcome != contract.OutcomeOK {
			t.Fatalf("move to %s: %s %s", column, moved.Outcome, moved.Refusal)
		}
		if moved.Instructions.ColumnAttachments != nil {
			t.Errorf("a column carrying no live attachment served %+v", moved.Instructions.ColumnAttachments)
		}
		if strings.Contains(strings.Join(moved.Instructions.Withheld, ","), LayerColumnAttachments) {
			t.Errorf("a column carrying no live attachment named the listing withheld: %v", moved.Instructions.Withheld)
		}
	}
}
