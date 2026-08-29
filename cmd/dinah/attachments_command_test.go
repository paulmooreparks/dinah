package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
