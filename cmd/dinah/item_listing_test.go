package main

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

type listedItemExpectation struct {
	ref     string
	kind    string
	state   string
	text    string
	ordinal int
}

// TestItemCollectionListingsKeepNarrowMembershipAndCanonicalReferences
// covers the eight full and unresolved checklist collection shapes in both
// terminal forms. Every printed reference is handed back to show and path so
// the test proves it still identifies the listed item.
func TestItemCollectionListingsKeepNarrowMembershipAndCanonicalReferences(t *testing.T) {
	root := itemListingBench(t)
	all := []listedItemExpectation{
		{
			ref: "fx-1/questions/1", kind: "open_question", state: bench.ItemResolved,
			text: "settled question", ordinal: 1,
		},
		{
			ref: "fx-1/criteria/1", kind: "acceptance_criterion", state: bench.ItemVerified,
			text: "verified criterion", ordinal: 2,
		},
		{
			ref: "fx-1/decisions/1", kind: "decision", state: bench.ItemResolved,
			text: "settled decision", ordinal: 3,
		},
		{
			ref: "fx-1/decisions/2", kind: "decision", state: bench.ItemPending,
			text: "pending decision", ordinal: 4,
		},
		{
			ref: "fx-1/questions/2", kind: "open_question", state: bench.ItemPending,
			text: "pending question", ordinal: 5,
		},
		{
			ref: "fx-1/criteria/2", kind: "acceptance_criterion", state: bench.ItemPending,
			text: "pending criterion", ordinal: 6,
		},
	}
	byRef := make(map[string]listedItemExpectation, len(all))
	for _, item := range all {
		byRef[item.ref] = item
	}
	cases := []struct {
		ref        string
		unresolved bool
		want       []string
	}{
		{ref: "fx-1/checklist", want: []string{all[0].ref, all[1].ref, all[2].ref, all[3].ref, all[4].ref, all[5].ref}},
		{ref: "fx-1/questions", want: []string{all[0].ref, all[4].ref}},
		{ref: "fx-1/criteria", want: []string{all[1].ref, all[5].ref}},
		{ref: "fx-1/decisions", want: []string{all[2].ref, all[3].ref}},
		{ref: "fx-1/checklist", unresolved: true, want: []string{all[3].ref, all[4].ref, all[5].ref}},
		{ref: "fx-1/questions", unresolved: true, want: []string{all[4].ref}},
		{ref: "fx-1/criteria", unresolved: true, want: []string{all[5].ref}},
		{ref: "fx-1/decisions", unresolved: true, want: []string{all[3].ref}},
	}
	if len(cases) != 8 {
		t.Fatalf("the collection-shape sweep carries %d cases, wanted eight", len(cases))
	}
	for _, tc := range cases {
		name := tc.ref
		if tc.unresolved {
			name += " unresolved"
		}
		t.Run(name, func(t *testing.T) {
			argv := []string{"list", tc.ref}
			if tc.unresolved {
				argv = append(argv, "--unresolved")
			}
			human := mustRun(t, root, argv...).out
			humanRefs, humanKinds := itemRows(human)
			if !slices.Equal(humanRefs, tc.want) {
				t.Errorf("the human listing carried refs %v, wanted %v\n%s", humanRefs, tc.want, human)
			}
			wantKinds := make([]string, 0, len(tc.want))
			for _, ref := range tc.want {
				wantKinds = append(wantKinds, byRef[ref].kind)
			}
			if !slices.Equal(humanKinds, wantKinds) {
				t.Errorf("the human listing carried kinds %v, wanted %v\n%s", humanKinds, wantKinds, human)
			}

			machineArgs := append(slices.Clone(argv), "--json")
			payload := mustRun(t, root, machineArgs...).out
			var listing verb.ItemListing
			if err := json.Unmarshal([]byte(payload), &listing); err != nil {
				t.Fatalf("the JSON listing will not parse: %v\n%s", err, payload)
			}
			if listing.Ref != tc.ref || listing.Kind != bench.KindItem {
				t.Errorf("the JSON envelope is ref %q kind %q, wanted ref %q kind %q", listing.Ref, listing.Kind, tc.ref, bench.KindItem)
			}
			if len(listing.Members) != len(tc.want) {
				t.Fatalf("the JSON listing carried %d items, wanted %d: %s", len(listing.Members), len(tc.want), payload)
			}
			for i, entry := range listing.Members {
				want := byRef[tc.want[i]]
				if entry.Ref != want.ref || entry.Kind != want.kind || entry.State != want.state || entry.Text != want.text || entry.Ordinal != want.ordinal {
					t.Errorf("member %d was %#v, wanted ref %q kind %q state %q text %q ordinal %d", i+1, entry, want.ref, want.kind, want.state, want.text, want.ordinal)
				}
				assertListedItemRecovers(t, root, entry, want)
			}
		})
	}
}

func itemListingBench(t *testing.T) string {
	t.Helper()
	root := newBench(t)
	mustRun(t, root, "add", "a card carrying every item kind twice")
	mustRun(t, root, "file", "fx-1", "open_question", "settled question")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "verified criterion")
	mustRun(t, root, "file", "fx-1", "decision", "settled decision")
	mustRun(t, root, "file", "fx-1", "decision", "pending decision")
	mustRun(t, root, "file", "fx-1", "open_question", "pending question")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "pending criterion")
	mustRun(t, root, "resolve", "fx-1/questions/1", "answered")
	mustRun(t, root, "verify", "fx-1/criteria/1", "proved")
	mustRun(t, root, "resolve", "fx-1/decisions/1", "decided")
	return root
}

func itemRows(output string) ([]string, []string) {
	var refs []string
	var kinds []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasPrefix(fields[0], "fx-1/") {
			continue
		}
		if fields[1] != "open_question" && fields[1] != "acceptance_criterion" && fields[1] != "decision" {
			continue
		}
		refs = append(refs, fields[0])
		kinds = append(kinds, fields[1])
	}
	return refs, kinds
}

func assertListedItemRecovers(t *testing.T, root string, entry verb.ItemIndexEntry, want listedItemExpectation) {
	t.Helper()
	payload := mustRun(t, root, "show", entry.Ref, "--json").out
	var detail verb.ItemDetail
	if err := json.Unmarshal([]byte(payload), &detail); err != nil {
		t.Fatalf("show %s will not parse: %v\n%s", entry.Ref, err, payload)
	}
	fm, body := bench.ParseAnchor(detail.Text)
	if detail.Ref != entry.Ref || fm.Value("kind") != want.kind || fm.Value("state") != want.state || strings.TrimRight(body, "\n") != want.text {
		t.Errorf("show %s recovered ref %q kind %q state %q text %q", entry.Ref, detail.Ref, fm.Value("kind"), fm.Value("state"), strings.TrimRight(body, "\n"))
	}
	anchor := strings.TrimSpace(mustRun(t, root, "path", entry.Ref).out)
	if id := filepath.Base(filepath.Dir(anchor)); id != entry.ID {
		t.Errorf("path %s recovered id %q, wanted %q", entry.Ref, id, entry.ID)
	}
}
