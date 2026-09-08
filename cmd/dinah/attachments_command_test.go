package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// TestTheAttachmentsCommandAnswersForEveryKindThatCarriesOne asserts the new
// read at the terminal: it answers for the workbench, a column, a card and a
// comment, its JSON carries a path that opens the attachment's own bytes, and
// its human form draws the sentence and the table (dinah-334 AC-10).
//
// The path is asserted by opening it and reading what came back, not by
// looking for the key. The field is optional in the wire format, so a check
// for its presence would pass against a build publishing a path that points
// at nothing, which is the failure worth catching.
func TestTheAttachmentsCommandAnswersForEveryKindThatCarriesOne(t *testing.T) {
	root := newBench(t)
	ref := addCard(t, root, "a card with things below it")
	if got := runCLI(t, root, "comment", ref, "a thought"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("the bytes"), 0o644); err != nil {
		t.Fatalf("write the source: %v", err)
	}
	for _, target := range []string{"workbench", "intake", ref, ref + "/comments/1"} {
		if got := runCLI(t, root, "attach", target, source); got.code != 0 {
			t.Fatalf("attach to %s: %d %s", target, got.code, got.errw)
		}
	}

	cases := []struct {
		name string
		argv []string
		kind string
		ref  string
	}{
		{name: "the workbench, named by nothing", argv: nil, kind: "workbench", ref: "workbench"},
		{name: "the workbench, named", argv: []string{"workbench"}, kind: "workbench", ref: "workbench"},
		{name: "a column", argv: []string{"intake"}, kind: "column", ref: "intake"},
		{name: "a card", argv: []string{ref}, kind: "card", ref: ref},
		{name: "a comment", argv: []string{ref + "/comments/1"}, kind: "comment", ref: ref + "/comments/1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := runCLI(t, root, append([]string{"--json", "attachments"}, c.argv...)...)
			if got.code != 0 {
				t.Fatalf("attachments %v: %d %s", c.argv, got.code, got.errw)
			}
			var listing verb.AttachmentListing
			if err := json.Unmarshal([]byte(got.out), &listing); err != nil {
				t.Fatalf("decode: %v\n%s", err, got.out)
			}
			if listing.Kind != c.kind {
				t.Errorf("kind: wanted %s, got %s", c.kind, listing.Kind)
			}
			if listing.Ref != c.ref {
				t.Errorf("ref: wanted %s, got %s", c.ref, listing.Ref)
			}
			if len(listing.Attachments) != 1 {
				t.Fatalf("wanted one attachment, got %d", len(listing.Attachments))
			}
			view := listing.Attachments[0]
			if want := c.ref + "/attachments/1"; view.Ref != want {
				t.Errorf("the attachment is addressed %q, wanted %q", view.Ref, want)
			}
			body, err := os.ReadFile(view.Path)
			if err != nil {
				t.Fatalf("the published path does not open: %v", err)
			}
			if string(body) != "the bytes" {
				t.Errorf("the published path opens the wrong file, it holds %q", string(body))
			}

			// The human form draws the entity it was asked about and the
			// attachment's own filename, which is the pair a reader needs in
			// order to know whose attachment they are looking at.
			human := runCLI(t, root, append([]string{"attachments"}, c.argv...)...)
			if human.code != 0 {
				t.Fatalf("attachments %v: %d %s", c.argv, human.code, human.errw)
			}
			if !strings.Contains(human.out, c.ref) {
				t.Errorf("the printed answer does not name %s:\n%s", c.ref, human.out)
			}
			if !strings.Contains(human.out, "notes.txt") {
				t.Errorf("the printed answer does not carry the attachment's filename:\n%s", human.out)
			}
		})
	}
}

// TestTheAttachmentsCommandSaysSoWhenThereAreNone asserts that an entity
// carrying no attachment is answered rather than refused: exit 0, an empty
// list in the machine form, and a sentence naming the entity in the human one
// (dinah-334 AC-6).
func TestTheAttachmentsCommandSaysSoWhenThereAreNone(t *testing.T) {
	root := newBench(t)
	ref := addCard(t, root, "a card with nothing attached")

	got := runCLI(t, root, "--json", "attachments", ref)
	if got.code != 0 {
		t.Fatalf("attachments %s: %d %s", ref, got.code, got.errw)
	}
	// The empty list has to survive the wire as a list rather than as null,
	// so a client can iterate what it was given without a nil check the
	// contract never asked it to write.
	if !strings.Contains(got.out, "\"attachments\": []") {
		t.Errorf("the machine answer does not carry an empty list:\n%s", got.out)
	}
	var listing verb.AttachmentListing
	if err := json.Unmarshal([]byte(got.out), &listing); err != nil {
		t.Fatalf("decode: %v\n%s", err, got.out)
	}
	if listing.Attachments == nil {
		t.Error("the decoded list is nil, and an entity carrying none reports an empty list")
	}

	human := runCLI(t, root, "attachments", ref)
	if human.code != 0 {
		t.Fatalf("attachments %s: %d %s", ref, human.code, human.errw)
	}
	if !strings.Contains(human.out, ref) {
		t.Errorf("the printed answer does not name the entity it was asked about:\n%s", human.out)
	}
	if strings.Contains(human.out, "Position") {
		t.Errorf("the printed answer drew a table for an entity carrying nothing:\n%s", human.out)
	}
}

// TestAListingCarriesTheAttachmentCountRatherThanTheList asserts the split
// Decision 2 draws: a card's row in a queue listing says how many attachments
// the card has and carries none of them, and the card's own detail carries the
// list (dinah-334 AC-4).
func TestAListingCarriesTheAttachmentCountRatherThanTheList(t *testing.T) {
	root := newBench(t)
	ref := addCard(t, root, "a card carrying two files")
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("the bytes"), 0o644); err != nil {
		t.Fatalf("write the source: %v", err)
	}
	for i := 0; i < 2; i++ {
		if got := runCLI(t, root, "attach", ref, source); got.code != 0 {
			t.Fatalf("attach: %d %s", got.code, got.errw)
		}
	}

	got := runCLI(t, root, "--json", "ls")
	if got.code != 0 {
		t.Fatalf("ls: %d %s", got.code, got.errw)
	}
	var listing verb.Listing
	if err := json.Unmarshal([]byte(got.out), &listing); err != nil {
		t.Fatalf("decode: %v\n%s", err, got.out)
	}
	if len(listing.Cards) != 1 {
		t.Fatalf("wanted one card, got %d", len(listing.Cards))
	}
	if listing.Cards[0].AttachmentCount != 2 {
		t.Errorf("the listed card reports %d attachments, wanted 2", listing.Cards[0].AttachmentCount)
	}
	if strings.Contains(got.out, "notes.txt") {
		t.Errorf("the listing carried an attachment's own fields, which is what the count exists to avoid:\n%s", got.out)
	}
}

// TestTheNotAttachableRefusalPrintsTheAdviceForItsKind asserts that the
// alternation resolves at the terminal: an item, an attachment and a
// workstream each draw dinah.not-attachable with the base sentence and with
// the one next step written for that kind, and with neither of the other two.
//
// The expectations are rendered through the catalog rather than spelled in
// English here, so a later wording edit moves the test with the copy while the
// test still pins which key each case reaches.
//
// Arming: swapping the When on the item fragment to attachment leaves both of
// the first two cases printing an advice, and only the "and neither other"
// assertions go red.
func TestTheNotAttachableRefusalPrintsTheAdviceForItsKind(t *testing.T) {
	root := newBench(t)
	ref := addCard(t, root, "a card with things below it")
	mustRunCLI(t, root, "file", ref, "open_question", "Does attach refuse?")
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("the bytes"), 0o644); err != nil {
		t.Fatalf("write the source: %v", err)
	}
	mustRunCLI(t, root, "attach", ref, source)
	mustRunCLI(t, root, "workstream", "new", "Probe stream")

	base := msg.For(msg.Base)
	for _, c := range []struct {
		name     string
		argument string
		resolved string
		kind     string
	}{
		{name: "a checklist item", argument: ref + "/oq/1", resolved: ref + "/checklist/1", kind: "item"},
		{name: "an attachment", argument: ref + "/attachments/1", resolved: ref + "/attachments/1", kind: "attachment"},
		{name: "a workstream", argument: "workstream/probe-stream", resolved: "probe-stream", kind: "workstream"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := runCLI(t, root, "attach", c.argument, source)
			if got.code == 0 {
				t.Fatalf("attach to %s succeeded, and its kind mounts no attachments", c.argument)
			}
			if !strings.HasPrefix(got.errw, contract.NotAttachable) {
				t.Errorf("stderr should open with %s, got:\n%s", contract.NotAttachable, got.errw)
			}
			sentence := base.T("refusal.dinah.not-attachable", "detail", c.resolved, "kind", c.kind)
			if !strings.Contains(got.errw, sentence) {
				t.Errorf("stderr should carry %q, got:\n%s", sentence, got.errw)
			}
			advice := map[string]string{
				"item":       base.T("refusal.dinah.not-attachable.next-item", "item", c.resolved),
				"attachment": base.T("refusal.dinah.not-attachable.next-attachment", "attachment", c.resolved),
				"workstream": base.T("refusal.dinah.not-attachable.next"),
			}
			for kind, splice := range advice {
				carried := strings.Contains(got.errw, splice)
				if kind == c.kind && !carried {
					t.Errorf("stderr should carry the %s advice %q, got:\n%s", kind, splice, got.errw)
				}
				if kind != c.kind && carried {
					t.Errorf("stderr carries the %s advice %q, and this is the %s case:\n%s", kind, splice, c.kind, got.errw)
				}
			}
		})
	}
}

// TestTheAttachHelpPageNamesTheKindPrecondition asserts that attach's
// precondition list reads as the code evaluates it: the reference, the owner,
// the kind, then the file.
//
// The order is written out as a literal as well as compared against
// verb.Checks, because a guard that recomputed its expectation from the table
// under test would stay green through a reordering of that table.
//
// Arming: swapping rows 3 and 4 in beyondChecks reddens the literal comparison
// and leaves the generated one green.
func TestTheAttachHelpPageNamesTheKindPrecondition(t *testing.T) {
	root := newBench(t)
	t.Setenv("COLUMNS", "80")
	wanted := []string{contract.UnknownPath, contract.NoOwner, contract.NotAttachable, contract.UnknownPath}

	declared := verb.Checks("attach")
	if len(declared) != len(wanted) {
		t.Fatalf("attach declares %d preconditions, wanted %d: %+v", len(declared), len(wanted), declared)
	}
	for i, refusal := range wanted {
		if declared[i].Refusal != refusal {
			t.Errorf("precondition %d is %s, wanted %s", i+1, declared[i].Refusal, refusal)
		}
	}

	page := runCLI(t, root, "help", "attach")
	if page.code != 0 {
		t.Fatalf("help attach: %d %s", page.code, page.errw)
	}
	var rows []string
	for _, line := range strings.Split(page.out, "\n") {
		fields := strings.Fields(line)
		// A numbered row of the refusal table opens with its ordinal and
		// closes with the refusal name, and no other line of the page does.
		if len(fields) < 2 || fields[0] != strconv.Itoa(len(rows)+1) {
			continue
		}
		rows = append(rows, fields[len(fields)-1])
	}
	if len(rows) != len(wanted) {
		t.Fatalf("the page draws %d numbered rows, wanted %d:\n%s", len(rows), len(wanted), page.out)
	}
	for i, refusal := range wanted {
		if rows[i] != refusal {
			t.Errorf("row %d names %s, wanted %s", i+1, rows[i], refusal)
		}
	}
	for _, line := range strings.Split(page.out, "\n") {
		if displayWidth(line) > 80 {
			t.Errorf("help attach draws a line %d columns wide:\n%q", displayWidth(line), line)
		}
	}
}
