package bench

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// tieredDefinition is the workbench anchor most of the cases below open: three
// declared rungs, and a table listing one model at each with the middle rung
// carrying a second entry that declares a server.
//
// It drives CORE-CAP-1 and CORE-CAP-2, which are the ordered set of
// capabilities a workbench declares and the owner descriptions that satisfy
// each one.
const tieredDefinition = `---
format: 5
profile: dinah-core/0.17
title: Fixture
slug: fx
operator: alka
levels:
  tier: [minimal, workhorse, frontier]
tiers:
  frontier:
    meaning: novel design judgement
    models:
      - {provider: anthropic, model: claude-opus-5}
  workhorse:
    meaning: scoped implementation against a contract
    models:
      - {provider: anthropic, model: claude-sonnet-5}
      - {provider: ollama, model: "qwen3:235b", server: ollama.com}
  minimal:
    meaning: mechanical edits
    models:
      - {provider: ollama, model: qwen3:8b}
columns:
  - b00000000001
---
Standing text.
`

// openTiered plants one anchor and opens it, which every case below starts
// from.
func openTiered(t *testing.T, anchor string) *Bench {
	t.Helper()
	root := containedPath(t.TempDir())
	write(t, filepath.Join(root, WorkbenchAnchor), anchor)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), columnDefinition)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return opened
}

// TestTheTiersReaderTakesOneFlowMappingPerLine drives dinah-496's reader
// criterion together with CORE-CAP-1 and CORE-CAP-2, which are the ordered set
// of capabilities a workbench declares and the owner descriptions that satisfy
// each one. A model entry is one flow mapping on one line,
// provider and model are required and server is optional, a value carrying a
// colon reads whether it is quoted or not, and an entry naming a member outside
// the three or carrying a comma inside a quoted value declares nothing.
func TestTheTiersReaderTakesOneFlowMappingPerLine(t *testing.T) {
	opened := openTiered(t, tieredDefinition)
	entries := opened.Tiers()
	if len(entries) != 3 {
		t.Fatalf("the reader declared %d entries, wanted 3: %+v", len(entries), entries)
	}
	if entries[0].Tier != "frontier" || entries[0].Meaning != "novel design judgement" {
		t.Errorf("the first entry is %+v, wanted frontier with its meaning", entries[0])
	}
	// The unquoted and the quoted colon are the same value after the read,
	// which is what splitting a part on its first colon buys.
	workhorse := entries[1]
	if len(workhorse.Models) != 2 {
		t.Fatalf("workhorse lists %d models, wanted 2: %+v", len(workhorse.Models), workhorse.Models)
	}
	if workhorse.Models[1] != (TierModel{Provider: "ollama", Model: "qwen3:235b", Server: "ollama.com"}) {
		t.Errorf("the quoted colon read as %+v", workhorse.Models[1])
	}
	if got := opened.Tiers()[2].Models[0]; got != (TierModel{Provider: "ollama", Model: "qwen3:8b"}) {
		t.Errorf("the unquoted colon read as %+v", got)
	}

	// Four malformed shapes, each leaving its own entry undeclared.
	damaged := strings.Replace(tieredDefinition,
		"      - {provider: ollama, model: qwen3:8b}",
		strings.Join([]string{
			"      - {provider: ollama, model: qwen3:8b, region: eu}",
			"      - {provider: ollama, model: \"a,b\"}",
			"      - {provider: ollama}",
			"      - {model: qwen3:8b}",
		}, "\n"), 1)
	broken := openTiered(t, damaged)
	if entries := broken.Tiers(); len(entries) != 2 {
		t.Errorf("the reader declared %d entries, wanted the two well-formed ones: %+v", len(entries), entries)
	}
	malformed := broken.MalformedTierEntries()
	if len(malformed) != 5 {
		t.Errorf("the reader refused %d entries, wanted the four model lines and the tier they left with no models: %+v", len(malformed), malformed)
	}
}

// TestServerMatchingRunsInBothDirections drives dinah-496's server criterion
// and CORE-CAP-2's own matching. An entry carrying a server is matched by a
// caller declaring that address and by nobody else; an entry carrying none is
// matched by a caller declaring any address and by a caller declaring none; and
// where the table carries both entries for one provider and model, the caller
// declaring the address resolves to the tier of the entry carrying it whichever
// of the two the anchor declares first.
func TestServerMatchingRunsInBothDirections(t *testing.T) {
	opened := openTiered(t, tieredDefinition)
	cases := []struct {
		provider, model, server string
		tier                    string
		resolves                bool
	}{
		{"ollama", "qwen3:235b", "ollama.com", "workhorse", true},
		{"ollama", "qwen3:235b", "", "", false},
		{"ollama", "qwen3:235b", "localhost", "", false},
		{"ollama", "qwen3:8b", "", "minimal", true},
		{"ollama", "qwen3:8b", "anywhere.example", "minimal", true},
		{"anthropic", "claude-opus-5", "", "frontier", true},
		{"anthropic", "", "", "", false},
		{"", "claude-opus-5", "", "", false},
	}
	for _, want := range cases {
		got, resolved := opened.TierOf(want.provider, want.model, want.server)
		if got != want.tier || resolved != want.resolves {
			t.Errorf("%s/%s@%q resolved to %q %v, wanted %q %v",
				want.provider, want.model, want.server, got, resolved, want.tier, want.resolves)
		}
	}

	// The specific entry beats the general one whichever order the anchor
	// carries them in, which is what the two passes are for.
	for _, order := range []string{"specific first", "general first"} {
		entries := []string{
			"      - {provider: ollama, model: shared, server: ollama.com}",
			"      - {provider: ollama, model: shared}",
		}
		if order == "general first" {
			entries[0], entries[1] = entries[1], entries[0]
		}
		anchor := strings.Replace(tieredDefinition,
			"      - {provider: ollama, model: qwen3:8b}",
			"      - {provider: ollama, model: qwen3:8b}\n"+entries[1], 1)
		anchor = strings.Replace(anchor,
			"      - {provider: anthropic, model: claude-sonnet-5}",
			"      - {provider: anthropic, model: claude-sonnet-5}\n"+entries[0], 1)
		both := openTiered(t, anchor)
		if got, _ := both.TierOf("ollama", "shared", "ollama.com"); got == "" {
			t.Errorf("%s: a caller declaring the address resolved to nothing", order)
		}
	}
}

// TestSatisfyingModelsWalkUpwardFromTheRequirement asserts that the entries a
// refusal and an offer name are every entry at or above the requirement, in
// levels.tier order from the lowest satisfying rung upward, so the cheapest
// model that would do the work is first.
func TestSatisfyingModelsWalkUpwardFromTheRequirement(t *testing.T) {
	opened := openTiered(t, tieredDefinition)
	got := RenderTierModels(opened.SatisfyingModels("workhorse"))
	want := "anthropic/claude-sonnet-5, ollama/qwen3:235b@ollama.com, anthropic/claude-opus-5"
	if got != want {
		t.Errorf("the satisfying entries render as %q, wanted %q", got, want)
	}
	if got := opened.SatisfyingModels("nonesuch"); got != nil {
		t.Errorf("a rung the workbench does not declare is satisfied by %+v, wanted nothing", got)
	}
}

// TestCheckReportsTheFiveTierFindings drives dinah-496's check criteria. Each
// finding is reported on a fixture carrying exactly that defect and on no clean
// fixture, and the sweep says how many entities it examined.
func TestCheckReportsTheFiveTierFindings(t *testing.T) {
	examined := 0
	clean := openTiered(t, tieredDefinition)
	cleanFindings, err := clean.Check()
	if err != nil {
		t.Fatalf("check the clean fixture: %v", err)
	}
	examined++
	for _, finding := range cleanFindings {
		switch finding.Key {
		case FindingTierWithoutModels, FindingTiersWithoutLevels,
			FindingRequirementsWithoutTable, FindingModelListedTwice, FindingTiersEntryMalformed:
			t.Errorf("the clean fixture reports %s: %+v", finding.Key, finding)
		}
	}

	cases := []struct {
		name   string
		anchor string
		key    string
	}{
		{
			name:   "a rung nothing satisfies",
			anchor: strings.Replace(tieredDefinition, "  tier: [minimal, workhorse, frontier]", "  tier: [minimal, workhorse, frontier, apex]", 1),
			key:    FindingTierWithoutModels,
		},
		{
			name:   "a table with no axis to name",
			anchor: strings.Replace(tieredDefinition, "levels:\n  tier: [minimal, workhorse, frontier]\n", "", 1),
			key:    FindingTiersWithoutLevels,
		},
		{
			name: "one entry under two tiers",
			anchor: strings.Replace(tieredDefinition,
				"      - {provider: ollama, model: qwen3:8b}",
				"      - {provider: ollama, model: qwen3:8b}\n      - {provider: anthropic, model: claude-opus-5}", 1),
			key: FindingModelListedTwice,
		},
		{
			name: "an entry that does not read",
			anchor: strings.Replace(tieredDefinition,
				"      - {provider: ollama, model: qwen3:8b}",
				"      - {provider: ollama, model: qwen3:8b, region: eu}", 1),
			key: FindingTiersEntryMalformed,
		},
	}
	for _, want := range cases {
		opened := openTiered(t, want.anchor)
		findings, err := opened.Check()
		if err != nil {
			t.Fatalf("%s: check: %v", want.name, err)
		}
		examined++
		if !reports(findings, want.key) {
			t.Errorf("%s: check reports no %s: %+v", want.name, want.key, findings)
		}
	}
	if examined != 5 {
		t.Fatalf("the sweep examined %d fixtures, wanted the clean one and the four defects", examined)
	}
}

// TestCheckKeysTheDuplicateOnTheWholeTriple asserts that two entries differing
// only by server are not a duplicate, and that two carrying an identical triple
// under different tiers are, with both tiers named.
func TestCheckKeysTheDuplicateOnTheWholeTriple(t *testing.T) {
	entries := 0
	differing := strings.Replace(tieredDefinition,
		"      - {provider: ollama, model: qwen3:8b}",
		"      - {provider: ollama, model: qwen3:8b}\n      - {provider: ollama, model: \"qwen3:235b\"}", 1)
	opened := openTiered(t, differing)
	for _, entry := range opened.Tiers() {
		entries += len(entry.Models)
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if reports(findings, FindingModelListedTwice) {
		t.Errorf("two entries differing only by server are reported as a duplicate: %+v", findings)
	}

	identical := strings.Replace(tieredDefinition,
		"      - {provider: ollama, model: qwen3:8b}",
		"      - {provider: ollama, model: qwen3:8b}\n      - {provider: ollama, model: \"qwen3:235b\", server: ollama.com}", 1)
	repeated := openTiered(t, identical)
	for _, entry := range repeated.Tiers() {
		entries += len(entry.Models)
	}
	duplicates, err := repeated.Check()
	if err != nil {
		t.Fatalf("check the repeated fixture: %v", err)
	}
	named := ""
	for _, finding := range duplicates {
		if finding.Key == FindingModelListedTwice {
			named = finding.Detail
		}
	}
	if named == "" {
		t.Fatalf("two entries carrying one triple are not reported: %+v", duplicates)
	}
	for _, tier := range []string{"workhorse", "minimal"} {
		if !strings.Contains(named, tier) {
			t.Errorf("the finding does not name %s, so the duplicate cannot be removed from the wrong one: %q", tier, named)
		}
	}
	if entries != 10 {
		t.Fatalf("the sweep examined %d model entries, wanted 10", entries)
	}
}

// TestCheckCountsTheCardsWhoseRequirementsRefuseNobody asserts the finding a
// workbench carrying tier requirements and no table meets, and that the detail
// carries how many cards carry one.
func TestCheckCountsTheCardsWhoseRequirementsRefuseNobody(t *testing.T) {
	anchor := strings.Replace(tieredDefinition, "tiers:\n", "no_tiers:\n", 1)
	root := containedPath(t.TempDir())
	write(t, filepath.Join(root, WorkbenchAnchor), anchor)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), columnDefinition)
	write(t, filepath.Join(root, CardNumbersName), "1 c00000000001\n2 c00000000002\n3 c00000000003\n")
	for at, tier := range []string{"tier: frontier", "tier_at:\n  - column: b00000000001\n    tier: workhorse", ""} {
		id := "c0000000000" + strconv.Itoa(at+1)
		card := "---\ntitle: A card\ncolumn: b00000000001\nstate: ready\n"
		if tier != "" {
			card += tier + "\n"
		}
		card += "---\nFraming.\n"
		write(t, filepath.Join(root, CardsDir, id, CardAnchor), card)
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if opened.DeclaresTierTable() {
		t.Fatal("the fixture declares a tiers block, so it is not the workbench this case is written for")
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	detail := ""
	for _, finding := range findings {
		if finding.Key == FindingRequirementsWithoutTable {
			detail = finding.Detail
		}
	}
	if detail != "2" {
		t.Errorf("the finding names %q cards carrying a requirement, wanted 2: %+v", detail, findings)
	}
}

// TestTheTierTableSurvivesTheInterchange drives CORE-JSON-13. A workbench
// carrying a tiers block exports with a tiers member keyed in levels.tier
// declaration order, and importing that export and exporting again produces
// byte-identical output.
func TestTheTierTableSurvivesTheInterchange(t *testing.T) {
	opened := openTiered(t, tieredDefinition)
	first, err := opened.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(first, &object); err != nil {
		t.Fatalf("the export does not parse: %v", err)
	}
	raw, carried := object[TiersKey]
	if !carried {
		t.Fatal("the export carries no tiers member")
	}
	members, read := jsonMembers(raw)
	if !read {
		t.Fatalf("the tiers member is not an object: %s", raw)
	}
	order := make([]string, 0, len(members))
	for _, member := range members {
		order = append(order, member.name)
	}
	if strings.Join(order, ",") != "minimal,workhorse,frontier" {
		t.Errorf("the export keys the table %v, wanted levels.tier declaration order", order)
	}

	definition, err := ReadDefinition(first)
	if err != nil {
		t.Fatalf("read the export back: %v", err)
	}
	clone := containedPath(t.TempDir())
	if err := Instantiate(clone, "fx", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	reopened, err := Open(clone)
	if err != nil {
		t.Fatalf("open the clone: %v", err)
	}
	second, err := reopened.Export()
	if err != nil {
		t.Fatalf("export the clone: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("a second export differs from the first:\n%s\n---\n%s", first, second)
	}
}

// TestEveryFormatThisBuildReadsStillOpens asserts that no read path this card
// adds refuses a workbench on account of its declared storage format. One
// workbench at each of formats 1 through 4 opens, and the count of openings is
// asserted so a loop that ran none cannot report success.
func TestEveryFormatThisBuildReadsStillOpens(t *testing.T) {
	opened := 0
	for format := 1; format <= 4; format++ {
		anchor := strings.Replace(tieredDefinition, "format: 5", "format: "+strconv.Itoa(format), 1)
		root := containedPath(t.TempDir())
		write(t, filepath.Join(root, WorkbenchAnchor), anchor)
		write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), columnDefinition)
		bench, err := Open(root)
		if err != nil {
			t.Errorf("a workbench declaring format %d is refused: %v", format, err)
			continue
		}
		if bench.Format != format {
			t.Errorf("a workbench declaring format %d opened as %d", format, bench.Format)
		}
		opened++
	}
	if opened != 4 {
		t.Fatalf("%d workbenches opened, wanted one at each of the four formats", opened)
	}
}

// reports says whether a finding of one key is among a set.
func reports(findings []Finding, key string) bool {
	for _, finding := range findings {
		if finding.Key == key {
			return true
		}
	}
	return false
}

// TestAMeaningWrappedOntoASecondLineIsReported is the repair for what Agent
// Code Review found at round one of dinah-496.
//
// A meaning written across two lines matched no member pattern on its
// continuation, so the sentence was truncated to its first line, nothing was
// reported, and an export carried the truncation back into the file. The
// contract's own section 10 drafts all three meanings across two lines each,
// which is the block the operator is to paste at Acceptance, so the silence was
// pointed straight at the one workbench that was going to meet it.
//
// A line the reader cannot place is now reported, and the entry it stands in
// declares nothing, because a half-read entry written back is worse than an
// absent one.
func TestAMeaningWrappedOntoASecondLineIsReported(t *testing.T) {
	wrapped := strings.Replace(tieredDefinition,
		"    meaning: mechanical edits\n",
		"    meaning: mechanical edits whose correctness is visible in the\n      diff\n", 1)
	opened := openTiered(t, wrapped)

	for _, entry := range opened.Tiers() {
		if entry.Tier == "minimal" {
			t.Errorf("the rung whose meaning wrapped still declares %+v, so a truncated sentence can be written back", entry)
		}
	}
	reported := ""
	for _, entry := range opened.MalformedTierEntries() {
		if strings.Contains(entry.Line, "diff") {
			reported = entry.Detail()
		}
	}
	if reported == "" {
		t.Fatalf("the continuation line is not reported: %+v", opened.MalformedTierEntries())
	}
	if !strings.Contains(reported, "minimal") {
		t.Errorf("the report does not name the tier the line stands under: %q", reported)
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !reports(findings, FindingTiersEntryMalformed) {
		t.Errorf("check reports no %s: %+v", FindingTiersEntryMalformed, findings)
	}

	// An annotation comment inside the block is a person writing to a person
	// and is not a line the reader failed to place.
	annotated := strings.Replace(tieredDefinition,
		"    meaning: mechanical edits\n",
		"    # the cheapest rung, and the one most work lands at\n    meaning: mechanical edits\n", 1)
	clean := openTiered(t, annotated)
	if got := clean.MalformedTierEntries(); len(got) != 0 {
		t.Errorf("an annotation comment is reported as an unreadable line: %+v", got)
	}
	if len(clean.Tiers()) != 3 {
		t.Errorf("a block carrying a comment declares %d entries, wanted 3", len(clean.Tiers()))
	}
}
