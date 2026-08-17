package verb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
)

// TestAddFilesACard asserts the creation rules: the first state, substate
// ready, a journal opened with the created event, a title-less request
// refused malformed (CORE-CARD-3), a state the bench does not declare refused
// unknown-state (CORE-CARD-4), and the asymmetric capacity treatment of the
// first state against a named one.
func TestAddFilesACard(t *testing.T) {
	h := newHarness(t)
	ref := h.add("first card")
	card := h.card(ref)
	if card.State != intake {
		t.Errorf("wanted the first state %s, got %s", intake, card.State)
	}
	if card.Substate != contract.SubstateReady {
		t.Errorf("wanted substate ready, got %s", card.Substate)
	}
	events := h.events(ref)
	if len(events) != 1 || events[0].Event != contract.EventCreated {
		t.Fatalf("wanted a journal opened with the created event, got %+v", events)
	}
	if events[0].Title != "first card" {
		t.Errorf("created event: wanted the title, got %q", events[0].Title)
	}

	if response := h.library.Add(&Request{Verb: "add", Actor: "alka"}); response.Refusal != contract.Malformed {
		t.Errorf("a title-less request: wanted malformed, got %s %s", response.Outcome, response.Refusal)
	}
	if response := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "x", State: "nowhere"}); response.Refusal != contract.UnknownState {
		t.Errorf("an unknown state: wanted unknown-state, got %s %s", response.Outcome, response.Refusal)
	}

	// Doing is limited to one card. A filing into it is refused once it is
	// full, while a filing into the first state is admitted whatever.
	full := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "occupant", State: doing})
	if full.Outcome != contract.OutcomeOK {
		t.Fatalf("first filing into Doing: %s %s", full.Outcome, full.Refusal)
	}
	h.reopen()
	if response := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "second", State: doing}); response.Refusal != contract.AtCapacity {
		t.Errorf("a filing into a full state: wanted at-capacity, got %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	for i := 0; i < 3; i++ {
		if response := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "intake filing"}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("a filing into the first state should never be refused, got %s", response.Refusal)
		}
		h.reopen()
	}
}

// TestIdentifiersAreUniqueAcrossBothHalves asserts CORE-CARD-1: an identifier
// in use in the archived half is not reused, which is the uniqueness scope the
// format's archive section fixes.
func TestIdentifiersAreUniqueAcrossBothHalves(t *testing.T) {
	h := newHarness(t)
	ref := h.add("archivable")
	id := h.card(ref).ID
	h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
	h.reopen()
	if !h.library.Bench.HasIdentifier(id) {
		t.Error("an archived identifier should still be in use")
	}
	if h.library.Bench.HasIdentifier("ffffffffffff") {
		t.Error("an identifier nothing carries should be free")
	}
}

// TestCommentAndAttach asserts the comment and attachment entities: the
// anchors they write, the ordering by timestamp, and the journal events each
// lifecycle act records.
func TestCommentAndAttach(t *testing.T) {
	h := newHarness(t)
	ref := h.add("annotated")

	h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: ref, Text: "the second thought"})
	h.reopen()
	h.advance(-1)
	h.library.Comment(&Request{Verb: "comment", Actor: "bob", Card: ref, Text: "the first thought"})
	h.reopen()

	comments, err := bench.Comments(h.card(ref).Dir)
	if err != nil {
		t.Fatalf("comments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("wanted two comments, got %d", len(comments))
	}
	if !strings.Contains(comments[0].Body, "first thought") {
		t.Errorf("comments should order by timestamp, got %q first", comments[0].Body)
	}
	if comments[0].Author != "bob" {
		t.Errorf("author: wanted bob, got %q", comments[0].Author)
	}

	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("the bytes"), 0o644); err != nil {
		t.Fatalf("source: %v", err)
	}
	attached := h.library.Attach(&Request{Verb: "attach", Actor: "alka", Ref: ref, File: source, Description: "notes"})
	if attached.Outcome != contract.OutcomeOK {
		t.Fatalf("attach: %s %s", attached.Outcome, attached.Refusal)
	}
	h.reopen()
	payload := filepath.Join(h.card(ref).Dir, bench.AttachmentsDir, attached.Detail, bench.PayloadDir, "notes.txt")
	if !bench.Exists(payload) {
		t.Errorf("wanted the payload under its original name at %s", payload)
	}

	replacement := filepath.Join(t.TempDir(), "revised.txt")
	if err := os.WriteFile(replacement, []byte("other bytes"), 0o644); err != nil {
		t.Fatalf("replacement: %v", err)
	}
	target := ref + "/attachments/" + attached.Detail
	if response := h.library.Attach(&Request{Verb: "attach", Actor: "alka", Ref: target, File: replacement, Replace: true}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("replace: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: target, Confirm: true}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("delete attachment: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()

	wanted := map[string]bool{
		contract.EventAttached:           false,
		contract.EventAttachmentReplaced: false,
		contract.EventAttachmentRemoved:  false,
	}
	for _, ev := range h.events(ref) {
		if _, ok := wanted[ev.Event]; !ok {
			continue
		}
		wanted[ev.Event] = true
		if ev.Attachment == "" || ev.Filename == "" {
			t.Errorf("%s: wanted the attachment id and its filename as of the event, got %q %q", ev.Event, ev.Attachment, ev.Filename)
		}
	}
	for name, seen := range wanted {
		if !seen {
			t.Errorf("journal: wanted a %s event, got none", name)
		}
	}
}

// TestArchiveAndDelete asserts that archiving takes a card out of the live
// set while its identifier still resolves, that a delete needs its
// confirmation, that a state cards occupy is refused, and that deleting a
// card another card's link names succeeds and leaves fsck the dangler.
func TestArchiveAndDelete(t *testing.T) {
	h := newHarness(t)
	occupant := h.add("occupant")
	linked := h.add("linked")

	// A card carrying a link to the card that is about to be deleted.
	card := h.card(occupant)
	card.FM.SetRaw("links", []string{"links:", "  - kind: relates", "    to: " + h.card(linked).ID})
	if err := card.Save(); err != nil {
		t.Fatalf("write link: %v", err)
	}
	h.reopen()

	if response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: linked}); response.Refusal != contract.Unconfirmed {
		t.Fatalf("an unconfirmed delete: wanted %s, got %s %s", contract.Unconfirmed, response.Outcome, response.Refusal)
	}
	if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: intake}); response.Refusal != contract.Occupied {
		t.Fatalf("archiving an occupied state: wanted %s, got %s %s", contract.Occupied, response.Outcome, response.Refusal)
	}
	if response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: intake, Confirm: true}); response.Refusal != contract.Occupied {
		t.Fatalf("deleting an occupied state: wanted %s, got %s %s", contract.Occupied, response.Outcome, response.Refusal)
	}

	// The archived card leaves the listings, the offers and the count.
	archivable := h.add("archivable")
	h.mustDo(&Request{Verb: Move, Card: archivable, Actor: "alka", State: doing})
	if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: archivable}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("archive: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	listing, err := h.library.List(&Request{Verb: "ls", State: doing})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listing.Cards) != 0 {
		t.Errorf("an archived card should leave the listing, got %d cards", len(listing.Cards))
	}
	// Doing is limited to one card, so an admitted move proves the archived
	// card left the count of CORE-MOVE-5 as well.
	if response := h.do(&Request{Verb: Move, Card: occupant, Actor: "alka", State: doing}); response.Outcome != contract.OutcomeOK {
		t.Errorf("an archived card should leave the capacity count, got %s", response.Refusal)
	}

	// Deleting a card another card's link names succeeds, and fsck reports
	// the dangling reference afterwards.
	deleted := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: linked, Confirm: true})
	if deleted.Outcome != contract.OutcomeOK {
		t.Fatalf("delete: %s %s", deleted.Outcome, deleted.Refusal)
	}
	h.reopen()
	findings, err := h.library.Fsck(&Request{Verb: "fsck", Actor: "alka"})
	if err != nil {
		t.Fatalf("fsck: %v", err)
	}
	found := false
	for _, finding := range findings {
		if finding.Key == bench.FindingDanglingLink {
			found = true
		}
	}
	if !found {
		t.Errorf("wanted a dangling-link finding after the delete, got %+v", findings)
	}
}

// TestInterchangeRoundTrip asserts CORE-JSON-1, CORE-JSON-2, CORE-JSON-3,
// CORE-JSON-4, CORE-JSON-5, CORE-JSON-7 and CORE-CARD-9:
// an object carrying an unknown member survives export, import and export,
// and so does an unrecognised field on a card.
func TestInterchangeRoundTrip(t *testing.T) {
	h := newHarness(t)
	h.library.Bench.FM.Set("acme.department", `"catering"`)
	if err := h.library.Bench.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	h.reopen()

	first, err := h.library.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	object := map[string]any{}
	if err := json.Unmarshal(first, &object); err != nil {
		t.Fatalf("the interchange form should parse as one object: %v", err)
	}
	for _, member := range []string{"profile", "title", "states"} {
		if _, ok := object[member]; !ok {
			t.Errorf("CORE-JSON-3: the written object carries no %s", member)
		}
	}
	if object["acme.department"] != "catering" {
		t.Errorf("CORE-JSON-7: the unrecognised member did not survive the export, got %v", object["acme.department"])
	}

	definition, err := bench.ReadDefinition(first)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(definition.States) != len(h.library.Bench.States) {
		t.Fatalf("wanted %d states, got %d", len(h.library.Bench.States), len(definition.States))
	}
	for i, element := range definition.States {
		var id string
		if err := json.Unmarshal(element["id"], &id); err != nil {
			t.Fatalf("state id: %v", err)
		}
		if id != h.library.Bench.States[i].ID {
			t.Errorf("CORE-JSON-4: position %d wanted %s, got %s", i, h.library.Bench.States[i].ID, id)
		}
	}

	second := filepath.Join(t.TempDir(), "again")
	if err := bench.Instantiate(second, "again", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	reopened, err := bench.Open(second)
	if err != nil {
		t.Fatalf("open the instantiated bench: %v", err)
	}
	third, err := reopened.Export()
	if err != nil {
		t.Fatalf("export again: %v", err)
	}
	if string(third) != string(first) {
		t.Errorf("a read and a write back changed the interchange form:\nfirst:\n%s\nthird:\n%s", first, third)
	}

	// A field the profile does not define survives on a card too.
	ref := h.add("extended")
	card := h.card(ref)
	card.FM.Set("acme.table", "seven")
	if err := card.Save(); err != nil {
		t.Fatalf("card save: %v", err)
	}
	h.reopen()
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	if got := h.card(ref).FM.Value("acme.table"); got != "seven" {
		t.Errorf("CORE-CARD-9: the unrecognised field did not survive a write, got %q", got)
	}
}

// TestExtractReproducesTheDefinition asserts that extract copies the bench's
// definition out with its identifiers kept and leaves the work behind, and
// that a bench instantiated from the result has the same definition.
//
// The comparison is over the parsed definition rather than over bytes,
// because canonical frontmatter key order is not settled yet. The byte-level
// form of the property waits on the card that settles it.
func TestExtractReproducesTheDefinition(t *testing.T) {
	h := newHarness(t)
	ref := h.add("work that stays behind")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})

	target := filepath.Join(t.TempDir(), "template")
	if err := h.library.Extract(target); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if bench.Exists(filepath.Join(target, bench.CardsDir)) {
		t.Error("extract carried the cards out with the definition")
	}
	if bench.Exists(filepath.Join(target, bench.JournalName)) {
		t.Error("extract carried a journal out with the definition")
	}

	extracted, err := bench.Open(target)
	if err != nil {
		t.Fatalf("open the extracted definition: %v", err)
	}
	if len(extracted.States) != len(h.library.Bench.States) {
		t.Fatalf("wanted %d states, got %d", len(h.library.Bench.States), len(extracted.States))
	}
	for i, state := range extracted.States {
		original := h.library.Bench.States[i]
		if state.ID != original.ID || state.Title != original.Title || state.Kind != original.Kind {
			t.Errorf("state %d: wanted %s/%s/%s, got %s/%s/%s", i,
				original.ID, original.Title, original.Kind, state.ID, state.Title, state.Kind)
		}
		if state.Capacity != original.Capacity || state.OperatorOwned != original.OperatorOwned {
			t.Errorf("state %d: capacity or operator flag differs", i)
		}
	}
}

// TestActorLadderAndConfig asserts the actor ladder, the config surface and
// the preservation of a key the tool does not know.
func TestActorLadderAndConfig(t *testing.T) {
	h := newHarness(t)
	cfg := bench.LoadConfig(h.home)
	if err := cfg.Set("actor", "from-config"); err != nil {
		t.Fatalf("set actor: %v", err)
	}
	if err := cfg.Set("lang", "hi"); err != nil {
		t.Fatalf("set lang: %v", err)
	}
	if err := cfg.Set("colour", "green"); err == nil {
		t.Error("an unknown key should be refused")
	} else if refusal, ok := err.(*contract.Refusal); !ok || refusal.Name != contract.UnknownKey {
		t.Errorf("wanted %s, got %v", contract.UnknownKey, err)
	}

	// A key the tool does not recognise, already in the file, survives.
	path := filepath.Join(h.home, bench.UserBaseName, bench.ConfigName)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(text, "---\n", "---\nacme.theme: dark\n", 1)), 0o644); err != nil {
		t.Fatalf("hand edit: %v", err)
	}
	reloaded := bench.LoadConfig(h.home)
	if err := reloaded.Set("lang", "en"); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	after := bench.LoadConfig(h.home)
	if after.Get("acme.theme") != "dark" {
		t.Errorf("an unrecognised setting did not survive a write, got %q", after.Get("acme.theme"))
	}
	if after.Get("actor") != "from-config" {
		t.Errorf("actor: wanted from-config, got %q", after.Get("actor"))
	}

	// The flag wins over every layer below it.
	resolved, err := bench.ResolveActor("from-flag", after)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved != "from-flag" {
		t.Errorf("wanted the flag to win, got %q", resolved)
	}
	// With no layer carrying one, the ladder refuses rather than inventing.
	empty := bench.LoadConfig(filepath.Join(t.TempDir(), "nothing"))
	t.Setenv("DINAH_ACTOR", "")
	if _, err := bench.ResolveActor("", empty); err == nil {
		t.Error("an unresolvable actor should be refused")
	}
}

// TestVersionCarriesTheConformanceClaim asserts CORE-VER-1 and CORE-VER-2:
// the claim carries a major and a minor number and no maturity channel, and
// the coverage report names every shipped catalog.
func TestVersionCarriesTheConformanceClaim(t *testing.T) {
	release := Version(true)
	if release.Profile != "dinah-core/1.0" {
		t.Errorf("conformance claim: wanted dinah-core/1.0, got %s", release.Profile)
	}
	for _, channel := range []string{"dev", "beta", "stable"} {
		if strings.Contains(release.Profile, channel) {
			t.Errorf("CORE-VER-2: the claim names the channel %s", channel)
		}
	}
	if release.Tool == release.Profile {
		t.Error("the tool's release number and the conformance claim must not be conflated")
	}
	if release.Format != bench.StorageFormat {
		t.Errorf("storage format: wanted %d, got %d", bench.StorageFormat, release.Format)
	}
	wanted := map[string]bool{"en": true, "hi": true, "de": true, "cs": true, "id": true, "es": true, "fil": true, "af": true}
	for _, coverage := range release.Catalogs {
		delete(wanted, coverage.Tag)
		if coverage.Present != coverage.Total {
			t.Errorf("%s: wanted every key present, got %d of %d", coverage.Tag, coverage.Present, coverage.Total)
		}
		complete := coverage.Tag == "en" || coverage.Tag == "hi"
		if complete && coverage.Translated != coverage.Total {
			t.Errorf("%s ships complete, got %d of %d translated", coverage.Tag, coverage.Translated, coverage.Total)
		}
		if !complete && coverage.Translated != 0 {
			t.Errorf("%s ships as a skeleton, got %d translated", coverage.Tag, coverage.Translated)
		}
	}
	if len(wanted) > 0 {
		t.Errorf("catalogs missing from the report: %v", wanted)
	}
}

// TestUnsupportedVersionsAreRefusedLoudly asserts CORE-BENCH-4 and the
// storage format's own refusal, each naming the version it wanted, together
// with CORE-OUT-5, which supplies malformed where no more particular refusal
// name applies.
func TestUnsupportedVersionsAreRefusedLoudly(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		value   string
		refusal string
	}{
		{name: "a profile major this binary does not implement", key: "profile", value: "dinah-core/9.0", refusal: contract.UnsupportedVer},
		{name: "a storage format newer than the binary knows", key: "format", value: "99", refusal: contract.UnsupportedVer},
		{name: "no profile declared at all", key: "profile", value: "", refusal: contract.Malformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			if c.value == "" {
				h.library.Bench.FM.Delete(c.key)
			} else {
				h.library.Bench.FM.Set(c.key, c.value)
			}
			if err := bench.WriteText(filepath.Join(h.root, bench.WorkbenchAnchor), h.library.Bench.FM.Render(h.library.Bench.Standing)); err != nil {
				t.Fatalf("write: %v", err)
			}
			_, err := bench.Open(h.root)
			refusal, ok := err.(*contract.Refusal)
			if !ok {
				t.Fatalf("wanted a refusal, got %v", err)
			}
			if refusal.Name != c.refusal {
				t.Errorf("wanted %s, got %s", c.refusal, refusal.Name)
			}
			if c.refusal == contract.UnsupportedVer && !strings.Contains(refusal.Detail, strings.TrimPrefix(c.value, "dinah-core/")) {
				t.Errorf("the refusal should name the version wanted, got %q", refusal.Detail)
			}
		})
	}
}

// TestWhoamiAnswersTheOperatorQuestion asserts CORE-OWNER-2 through the read
// surface, and that a bench designating no operator refuses.
func TestWhoamiAnswersTheOperatorQuestion(t *testing.T) {
	h := newHarness(t)
	identity, err := h.library.Whoami(&Request{Verb: "whoami", Actor: "alka"})
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if !identity.IsOperator {
		t.Error("alka is the operator of the fixture bench")
	}
	other, err := h.library.Whoami(&Request{Verb: "whoami", Actor: "bob"})
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if other.IsOperator {
		t.Error("bob is not the operator of the fixture bench")
	}
	if _, err := h.library.Whoami(&Request{Verb: "whoami"}); err == nil {
		t.Error("with no actor resolvable, whoami should refuse rather than invent one")
	}
	status, err := h.library.Status(&Request{Verb: "status", Actor: "alka"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.IsOperator || status.Operator != "alka" {
		t.Error("status carries the same two facts as whoami")
	}
}

// TestGuidesAreServedAndNeverSeeded asserts that the embedded guides list and
// print, that an unknown topic is refused, and that no guide text is written
// into a bench by init or by any verb.
func TestGuidesAreServedAndNeverSeeded(t *testing.T) {
	topics := guide.Topics()
	if len(topics) == 0 {
		t.Fatal("wanted at least one embedded guide")
	}
	first, err := guide.Text(topics[0])
	if err != nil {
		t.Fatalf("guide: %v", err)
	}
	if first == "" {
		t.Error("an embedded guide should carry text")
	}
	if _, err := guide.Text("no-such-topic"); err == nil {
		t.Error("an unknown topic should be refused")
	} else if refusal, ok := err.(*contract.Refusal); !ok || refusal.Name != contract.UnknownGuide {
		t.Errorf("wanted %s, got %v", contract.UnknownGuide, err)
	}

	h := newHarness(t)
	ref := h.add("ordinary work")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: doing})
	needle := strings.SplitN(strings.TrimPrefix(first, "# "), "\n", 2)[0]
	err = filepath.WalkDir(h.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		text, readErr := bench.ReadText(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(text, needle) {
			t.Errorf("guide text was seeded into the bench at %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// TestEveryRefusalNameIsLegal asserts CORE-OUT-3 and DOC-LAYER-1: every
// refusal name this binary can emit is one the profile declares or one
// carrying the dinah layer's prefix, and the two sets do not overlap.
func TestEveryRefusalNameIsLegal(t *testing.T) {
	declared := map[string]bool{}
	for _, name := range contract.Declared {
		declared[name] = true
		if strings.Contains(name, ".") {
			t.Errorf("DOC-LAYER-1: the profile's own name %s carries a full stop", name)
		}
	}
	if len(contract.Declared) != 16 {
		t.Errorf("wanted the profile's sixteen declared names, got %d", len(contract.Declared))
	}
	for _, name := range contract.Introduced {
		if declared[name] {
			t.Errorf("%s is both declared by the profile and introduced by this layer", name)
		}
		if !strings.HasPrefix(name, contract.LayerPrefix) {
			t.Errorf("%s is not one of the profile's sixteen and carries no layer prefix", name)
		}
	}
	// Every name reachable from the code is one of the two sets, which is
	// what the check enumerating them is for.
	reachable := append(append([]string{}, contract.Declared...), contract.Introduced...)
	for _, name := range reachable {
		if !contract.NameIsLegal(name) {
			t.Errorf("CORE-OUT-3 admits neither form of %s", name)
		}
	}
	for _, checks := range [][]Check{
		Checks(Claim), Checks(Move), Checks(Release), Checks(Block), Checks(Unblock),
		Checks("add"), Checks("comment"), Checks("attach"), Checks("archive"),
		Checks("delete"), Checks("guide"), Checks("config"), Checks("init"),
		Checks("extract"), Checks("edit"), Checks("path"), Checks("show"),
		Checks("log"), Checks("ls"), Checks("next"), Checks("instructions"),
		Checks("whoami"),
	} {
		for _, check := range checks {
			if !contract.NameIsLegal(check.Refusal) {
				t.Errorf("a declared check reports the illegal refusal name %s", check.Refusal)
			}
		}
	}
}

// TestOutcomesAndExitCodes asserts CORE-OUT-1 and the exit code each outcome
// carries.
func TestOutcomesAndExitCodes(t *testing.T) {
	wanted := map[string]int{
		contract.OutcomeOK:          0,
		contract.OutcomeRefused:     2,
		contract.OutcomeStale:       3,
		contract.OutcomeUnreachable: 4,
	}
	for outcome, code := range wanted {
		if got := contract.ExitCode(outcome); got != code {
			t.Errorf("%s: wanted exit code %d, got %d", outcome, code, got)
		}
	}
	h := newHarness(t)
	ref := h.add("outcomes")
	for _, req := range []*Request{
		{Verb: Claim, Card: ref, Actor: "alka"},
		{Verb: Claim, Card: ref, Actor: "bob"},
		{Verb: Claim, Card: "fx-99", Actor: "alka"},
	} {
		response := h.do(req)
		if _, ok := wanted[response.Outcome]; !ok {
			t.Errorf("CORE-OUT-1: %q is not one of the four outcome tokens", response.Outcome)
		}
		if response.Outcome == contract.OutcomeRefused && response.Refusal == "" {
			t.Error("CORE-OUT-2: a refusal carries exactly one refusal name")
		}
		if response.Outcome == contract.OutcomeOK && response.Refusal != "" {
			t.Errorf("an ok outcome carries the refusal name %s", response.Refusal)
		}
	}
}

// TestStalePrefixWarnsRatherThanRefuses asserts that within a bench a card
// reference resolves on its number, and a prefix naming no current slug is
// accepted with a warning.
func TestStalePrefixWarnsRatherThanRefuses(t *testing.T) {
	h := newHarness(t)
	ref := h.add("renamed bench")
	number := strings.TrimPrefix(ref, "fx-")
	response := h.do(&Request{Verb: Claim, Card: "yokoten-" + number, Actor: "alka"})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("wanted the number to resolve the card, got %s %s", response.Outcome, response.Refusal)
	}
	if response.Warning == "" || response.WarningDetail != "yokoten" {
		t.Errorf("wanted a warning naming the stale prefix, got %q %q", response.Warning, response.WarningDetail)
	}
}
