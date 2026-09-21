package verb

import (
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// TestEveryPullCheckKeyIsCarriedByEveryCatalog is dinah-484 AC-11's second
// half. The card renumbers pull's own rows, and renumbering is a rename of
// every key from the insertion point down, applied by hand in eight files. A
// file left un-renumbered carries a key nothing reads and is missing one
// something does, and the help for that language then prints the key name
// where the sentence should be.
//
// The keys come off pullChecks itself rather than off a list written here, so
// a row added later joins this guard without anybody remembering to.
func TestEveryPullCheckKeyIsCarriedByEveryCatalog(t *testing.T) {
	checked := 0
	for _, row := range Checks(Pull) {
		if _, ok := msg.BaseEntry(row.Key); !ok {
			t.Errorf("%s is a row of pull's list and the base catalog carries no such key", row.Key)
			continue
		}
		for _, tag := range msg.Tags() {
			checked++
			if _, carried := msg.CatalogEntry(tag, row.Key); !carried {
				t.Errorf("%s/%s: the row is declared and this catalog carries no sentence for it", tag, row.Key)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no row was checked against any catalog, so this guard is asserting nothing")
	}
}

// TestEveryMoveCheckKeyIsCarriedByEveryCatalog is the same guard over the
// move's own list, which this card appends a row to. An appended row cannot
// renumber anything, so the failure it catches is narrower: a row declared in
// code and left out of the catalogs entirely.
func TestEveryMoveCheckKeyIsCarriedByEveryCatalog(t *testing.T) {
	checked := 0
	for _, row := range Checks(Move) {
		if _, ok := msg.BaseEntry(row.Key); !ok {
			t.Errorf("%s is a row of the move's list and the base catalog carries no such key", row.Key)
			continue
		}
		for _, tag := range msg.Tags() {
			checked++
			if _, carried := msg.CatalogEntry(tag, row.Key); !carried {
				t.Errorf("%s/%s: the row is declared and this catalog carries no sentence for it", tag, row.Key)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no row was checked against any catalog, so this guard is asserting nothing")
	}
}

// TestEveryReleaseCheckKeyIsCarriedByEveryCatalog is the same guard over
// release's own list, on TestEveryMoveCheckKeyIsCarriedByEveryCatalog's
// pattern. Release and unblock carried no such guard before dinah-540, which
// is how a row inserted ahead of check.release.2 (formerly check.release.1's
// only sibling) could go undetected in one catalog the way the no-owner row
// itself did.
func TestEveryReleaseCheckKeyIsCarriedByEveryCatalog(t *testing.T) {
	checked := 0
	for _, row := range Checks(Release) {
		if _, ok := msg.BaseEntry(row.Key); !ok {
			t.Errorf("%s is a row of release's list and the base catalog carries no such key", row.Key)
			continue
		}
		for _, tag := range msg.Tags() {
			checked++
			if _, carried := msg.CatalogEntry(tag, row.Key); !carried {
				t.Errorf("%s/%s: the row is declared and this catalog carries no sentence for it", tag, row.Key)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no row was checked against any catalog, so this guard is asserting nothing")
	}
}

// TestEveryUnblockCheckKeyIsCarriedByEveryCatalog is the same guard over
// unblock's own list, on the same pattern.
func TestEveryUnblockCheckKeyIsCarriedByEveryCatalog(t *testing.T) {
	checked := 0
	for _, row := range Checks(Unblock) {
		if _, ok := msg.BaseEntry(row.Key); !ok {
			t.Errorf("%s is a row of unblock's list and the base catalog carries no such key", row.Key)
			continue
		}
		for _, tag := range msg.Tags() {
			checked++
			if _, carried := msg.CatalogEntry(tag, row.Key); !carried {
				t.Errorf("%s/%s: the row is declared and this catalog carries no sentence for it", tag, row.Key)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no row was checked against any catalog, so this guard is asserting nothing")
	}
}

// TestNoOwnerGuardsEveryMutatingVerb is dinah-540 AC-5, the drift guard item
// 4 of that card's specification asks for: a sweep that drives every name
// historyWriters carries with the actor left empty, and asserts every one of
// them refuses no-owner.
//
// It sweeps historyWriters rather than cmd/dinah's own dispatch groups,
// because dinah-540's own review found that a sweep scoped to groupWork
// alone never drives set, reshape, column or workstream, all four of which
// are groupBench and all four of which already carry the check this test
// exists to hold in place.
//
// This is not, on its own, a check that historyWriters names every command
// whose entry writes a journal line: a name the registry has silently
// dropped drops out of this sweep's own count too, and the loop below can
// only fail loudly on driving fewer names than the registry currently
// claims, not on the registry claiming too few. What actually catches a
// write path reachable with no actor and not named here is
// bench.AppendEvent's own guard (internal/bench/journal.go), which every one
// of these 29 paths funnels through; TestAppendEventRefusesAnEmptyActor
// exercises that guard directly. This sweep's own value is narrower and
// different: it holds the *documented* check order (dinah help <verb>)
// against the behaviour, which a regression on AppendEvent's guard alone
// would not catch, because a per-verb check quietly deleted would still be
// caught by AppendEvent while dinah help <verb> silently stopped matching
// what the tool does.
func TestNoOwnerGuardsEveryMutatingVerb(t *testing.T) {
	h := newHarness(t)
	card := h.ready("dinah-540 coverage card")
	target := h.ready("dinah-540 link target")
	archived := h.ready("dinah-540 archive coverage card")
	h.attach(card, "note.txt", "an attachment body")
	attachmentRef := card + "/attachments/1"
	decisionRef := h.file(card, "decision", "a decision to exercise the five checklist verbs")
	ws := h.newWorkstream("dinah-540 coverage workstream")
	if response := h.library.Link(&Request{Verb: "link", Actor: "alka", Card: card, Kind: "relates_to", LinkTo: target}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("fixture: link: %s %s", response.Outcome, response.Refusal)
	}
	if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: archived}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("fixture: archive: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()

	fromResponse := func(r *Response) (string, string) { return r.Outcome, r.Refusal }
	fromError := func(err error) (string, string) {
		if err == nil {
			return contract.OutcomeOK, ""
		}
		if refusal, ok := err.(*contract.Refusal); ok {
			return contract.OutcomeRefused, refusal.Name
		}
		return "error", err.Error()
	}

	drivers := map[string]func() (string, string){
		Claim: func() (string, string) { return fromResponse(h.library.Do(&Request{Verb: Claim, Card: card})) },
		Move: func() (string, string) {
			return fromResponse(h.library.Do(&Request{Verb: Move, Card: card, Column: aftercareSlug}))
		},
		Release: func() (string, string) { return fromResponse(h.library.Do(&Request{Verb: Release, Card: card})) },
		Block: func() (string, string) {
			return fromResponse(h.library.Do(&Request{Verb: Block, Card: card, Reason: "an obstacle"}))
		},
		Unblock: func() (string, string) { return fromResponse(h.library.Do(&Request{Verb: Unblock, Card: card})) },
		Join: func() (string, string) {
			return fromResponse(h.library.Do(&Request{Verb: Join, Card: card, Workstream: ws.Slug}))
		},
		Leave: func() (string, string) {
			return fromResponse(h.library.Do(&Request{Verb: Leave, Card: card, Workstream: ws.Slug}))
		},
		Pull:  func() (string, string) { return fromResponse(h.library.Pull(&Request{Verb: Pull})) },
		Raise: func() (string, string) { return fromResponse(h.library.Raise(&Request{Verb: Raise, Card: card})) },
		"add": func() (string, string) {
			return fromResponse(h.library.Add(&Request{Verb: "add", Title: "a card filed under no actor"}))
		},
		"comment": func() (string, string) {
			return fromResponse(h.library.Comment(&Request{Verb: "comment", Card: card, Text: "a comment written under no actor"}))
		},
		"attach":  func() (string, string) { return fromResponse(h.library.Attach(&Request{Verb: "attach", Ref: card})) },
		"archive": func() (string, string) { return fromResponse(h.library.Archive(&Request{Verb: "archive", Ref: card})) },
		"restore": func() (string, string) {
			return fromResponse(h.library.Restore(&Request{Verb: "restore", Ref: archived}))
		},
		"delete": func() (string, string) {
			return fromResponse(h.library.Delete(&Request{Verb: "delete", Ref: attachmentRef}))
		},
		"rename": func() (string, string) {
			return fromResponse(h.library.Rename(&Request{Verb: "rename", Ref: attachmentRef, Value: "renamed.txt"}))
		},
		"workstream": func() (string, string) {
			return fromResponse(h.library.NewWorkstream(&Request{Verb: "workstream", Action: "new", Workstream: "dinah-540 nameless workstream"}))
		},
		"set": func() (string, string) {
			return fromResponse(h.library.SetField(&Request{Verb: "set", Ref: card, Field: bench.BodyField, Value: "a legal body"}))
		},
		"column": func() (string, string) {
			return fromResponse(h.library.NewColumn(&Request{Verb: "column", Action: "new", Column: "dinah-540 nameless station"}))
		},
		"file": func() (string, string) {
			return fromResponse(h.library.File(&Request{Verb: "file", Card: card, Kind: "decision", Text: "filed under no actor"}))
		},
		"resolve": func() (string, string) {
			return fromResponse(h.library.Resolve(&Request{Verb: "resolve", Ref: decisionRef, Text: "x"}))
		},
		"verify": func() (string, string) {
			return fromResponse(h.library.Verify(&Request{Verb: "verify", Ref: decisionRef, Text: "x"}))
		},
		"fail": func() (string, string) {
			return fromResponse(h.library.Fail(&Request{Verb: "fail", Ref: decisionRef, Text: "x"}))
		},
		"reopen": func() (string, string) {
			return fromResponse(h.library.Reopen(&Request{Verb: "reopen", Ref: decisionRef, Reason: "x"}))
		},
		"settle": func() (string, string) {
			return fromResponse(h.library.Settle(&Request{Verb: "settle", Ref: decisionRef, State: bench.ItemResolved, Text: "x"}))
		},
		"cite": func() (string, string) {
			return fromResponse(h.library.Cite(&Request{Verb: "cite", Ref: decisionRef, Scheme: "test", CiteTarget: "somewhere"}))
		},
		"link": func() (string, string) {
			return fromResponse(h.library.Link(&Request{Verb: "link", Card: card, Kind: "relates_to", LinkTo: target}))
		},
		"unlink": func() (string, string) {
			return fromResponse(h.library.Unlink(&Request{Verb: "unlink", Card: card, Kind: "relates_to", LinkTo: target}))
		},
		"reshape": func() (string, string) {
			_, err := h.library.Reshape(&Request{Verb: "reshape"})
			return fromError(err)
		},
		"check": func() (string, string) {
			_, err := h.library.Check(&Request{Verb: "check", MigrateSlugs: true})
			return fromError(err)
		},
	}

	if len(historyWriters) != 30 {
		t.Fatalf("historyWriters carries %d names, wanted 30; this test's own driver table needs updating alongside it", len(historyWriters))
	}
	driven := 0
	for name := range historyWriters {
		drive, declared := drivers[name]
		if !declared {
			t.Errorf("%s is a member of historyWriters and this sweep has no driver for it", name)
			continue
		}
		driven++
		outcome, refusal := drive()
		if outcome != contract.OutcomeRefused || refusal != contract.NoOwner {
			t.Errorf("%s: driven with no actor, wanted refused/no-owner, got %s/%s", name, outcome, refusal)
		}
	}
	if driven < 29 {
		t.Fatalf("the sweep drove %d of historyWriters' 29 names; a name silently dropped from the driver table would pass this test by driving less than it claims to", driven)
	}
}
