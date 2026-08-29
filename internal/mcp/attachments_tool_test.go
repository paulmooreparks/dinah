package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// TestTheAttachmentsToolPublishesTheSameListingTheTerminalPrints asserts that
// the second head serves the new read and serves the library's own listing
// through it, path included (dinah-334 AC-10).
//
// The path is asserted by opening the file it names. A check that the key is
// there would pass against a head that published an empty or wrong path, and
// opening an attachment is the whole reason the field exists.
func TestTheAttachmentsToolPublishesTheSameListingTheTerminalPrints(t *testing.T) {
	library := newLibrary(t)
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("the bytes"), 0o644); err != nil {
		t.Fatalf("write the source: %v", err)
	}
	for _, target := range []string{"workbench", "fx-1"} {
		attached := library.Attach(&verb.Request{Verb: "attach", Actor: "alka", Ref: target, File: source})
		if attached.Outcome != contract.OutcomeOK {
			t.Fatalf("attach to %s: %s %s", target, attached.Outcome, attached.Refusal)
		}
	}

	cases := []struct {
		name      string
		arguments string
		kind      string
		ref       string
	}{
		{name: "the workbench, named by nothing", arguments: `{"actor":"alka"}`, kind: "workbench", ref: "workbench"},
		{name: "a card", arguments: `{"actor":"alka","ref":"fx-1"}`, kind: "card", ref: "fx-1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"attachments","arguments":`+c.arguments+`}}`)
			decoded := payload(t, answer)
			if _, carried := decoded["affordances"]; !carried {
				t.Errorf("the response carries no affordances member: %v", decoded)
			}
			encoded, err := json.Marshal(decoded["attachments"])
			if err != nil {
				t.Fatalf("marshal the listing: %v", err)
			}
			var listing verb.AttachmentListing
			if err := json.Unmarshal(encoded, &listing); err != nil {
				t.Fatalf("decode the listing: %v\n%s", err, string(encoded))
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
		})
	}
}
