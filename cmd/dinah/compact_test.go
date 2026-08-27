package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// compactFixtureDefinition is the workbench the tests below read. Its four
// states cover what the compact projection has to survive: an intake holding a
// ready card to offer, a work state to claim in, a state that waits on
// somebody outside the workbench, and a done state that offers nothing and
// holds nothing. It declares both level axes, so a card can carry a severity
// and a priority and a write can be refused over a level name.
const compactFixtureDefinition = `{
  "profile": "dinah-core/1.0",
  "title": "Compact fixture",
  "levels": { "severity": ["trivial", "minor", "major"], "priority": ["later", "soon", "now"] },
  "states": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake" },
    { "id": "c00000000002", "title": "Doing", "kind": "work" },
    { "id": "c00000000003", "title": "Approval", "kind": "work", "awaiting_outside": true },
    { "id": "c00000000004", "title": "Done", "kind": "done" }
  ]
}`

// awkwardTitle carries the three bytes the compact grammar has to escape,
// which is what makes the round trip below prove anything: a literal pipe
// would otherwise end a field early, a literal backslash would turn the byte
// after it into an escape, and a literal newline would end the record.
const awkwardTitle = "a|b\\c\nd"

// newCompactBench builds the fixture workbench and populates it: one card
// carrying the awkward title, and one card belonging to two workstreams with a
// severity and a priority set. Approval and Done are left holding nothing.
func newCompactBench(t *testing.T) string {
	t.Helper()
	root := newBenchFromDefinition(t, compactFixtureDefinition)
	if got := runCLI(t, root, "--json", "add", awkwardTitle); got.code != 0 {
		t.Fatalf("add the awkward card: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "--json", "add", "Second card"); got.code != 0 {
		t.Fatalf("add the second card: %d %s", got.code, got.errw)
	}
	for _, title := range []string{"Alpha", "Beta"} {
		if got := runCLI(t, root, "--json", "workstream", "new", title); got.code != 0 {
			t.Fatalf("workstream new %s: %d %s", title, got.code, got.errw)
		}
		if got := runCLI(t, root, "--json", "join", "fx-2", strings.ToLower(title)); got.code != 0 {
			t.Fatalf("join fx-2 to %s: %d %s", title, got.code, got.errw)
		}
	}
	for _, pair := range [][2]string{{"severity", "major"}, {"priority", "now"}} {
		if got := runCLI(t, root, "--json", "card", "set", "fx-2", pair[0], pair[1]); got.code != 0 {
			t.Fatalf("card set fx-2 %s: %d %s", pair[0], got.code, got.errw)
		}
	}
	if got := runCLI(t, root, "--json", "move", "fx-2", "doing"); got.code != 0 {
		t.Fatalf("move fx-2 to doing: %d %s", got.code, got.errw)
	}
	return root
}

// compactRecord is one decoded record: the kind that leads it, and its fields
// with the grammar's escapes undone.
type compactRecord struct {
	kind   string
	fields []string
}

// field returns the field at a position, and the empty string past the end, so
// a reader of a record written before a trailing field existed reads the same
// absence an empty field carries.
func (r compactRecord) field(at int) string {
	if at < 0 || at >= len(r.fields) {
		return ""
	}
	return r.fields[at]
}

// decodeCompactPayload takes a whole compact payload apart into its records.
// It is the test suite's own reader of the grammar rather than a shipped API:
// nothing in this card promises a Go package for consuming the compact form,
// only the wire grammar it reads here.
//
// Malformed input is a hard error rather than a guess, so a payload carrying a
// backslash before a byte the grammar does not escape fails here instead of
// decoding to something plausible.
//
// The opening record is checked for its shape here and for its value in
// TestTheCompactFormOpensOnItsVersionRecord. This reader is the wrong place to
// pin the value: it runs under every compact test, so pinning it here would
// turn a version change into a package-wide failure that names no cause, and
// comparing the field against compactVersion instead would compare the
// constant with itself and assert nothing at all.
func decodeCompactPayload(payload string) ([]compactRecord, error) {
	if !strings.HasSuffix(payload, "\n") {
		return nil, errors.New("the payload does not end on a newline")
	}
	lines := strings.Split(strings.TrimSuffix(payload, "\n"), "\n")
	records := make([]compactRecord, 0, len(lines))
	for _, line := range lines {
		fields, err := decodeCompactLine(line)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", line, err)
		}
		records = append(records, compactRecord{kind: fields[0], fields: fields[1:]})
	}
	opening := records[0]
	if opening.kind != "fmt" || opening.field(0) != "compact" || opening.field(1) == "" {
		return nil, fmt.Errorf("the payload opens on %v rather than the version record", opening)
	}
	return records[1:], nil
}

// decodeCompactLine splits one record into its fields, left to right, tracking
// whether the byte before was an unescaped backslash: a separator reached
// outside that state ends a field, and one reached inside it is data.
//
// Reading the line byte by byte is safe for text in any script, because no
// byte of a multi-byte UTF-8 sequence can equal the separator or the escape.
func decodeCompactLine(line string) ([]string, error) {
	var fields []string
	var current strings.Builder
	escaped := false
	for i := 0; i < len(line); i++ {
		b := line[i]
		if escaped {
			switch b {
			case '\\':
				current.WriteByte('\\')
			case '|':
				current.WriteByte('|')
			case 'n':
				current.WriteByte('\n')
			case 'r':
				current.WriteByte('\r')
			default:
				return nil, fmt.Errorf("a backslash stands before %q, which the grammar does not escape", string(b))
			}
			escaped = false
			continue
		}
		switch b {
		case '\\':
			escaped = true
		case '|':
			fields = append(fields, current.String())
			current.Reset()
		default:
			current.WriteByte(b)
		}
	}
	if escaped {
		return nil, errors.New("the record ends on a bare backslash")
	}
	return append(fields, current.String()), nil
}

// decodeCompactCard rebuilds a card from its record.
func decodeCompactCard(record compactRecord) verb.CardView {
	card := verb.CardView{
		ID:          record.field(0),
		Ref:         record.field(1),
		Title:       record.field(2),
		State:       record.field(3),
		StateTitle:  record.field(4),
		Substate:    record.field(5),
		Severity:    record.field(6),
		Priority:    record.field(7),
		Holder:      record.field(8),
		ClaimSince:  record.field(9),
		Expires:     record.field(10),
		BlockReason: record.field(11),
		BlockKind:   record.field(12),
		Revision:    record.field(13),
	}
	if len(record.fields) > 14 {
		card.Workstreams = append([]string{}, record.fields[14:]...)
	}
	return card
}

// decodeCompactFlag reads a boolean field: the one byte a set flag carries, and
// an empty field for a flag that is not set.
func decodeCompactFlag(value string) bool { return value == "1" }

// decodeCompactResponse rebuilds a verb's response from a compact payload.
func decodeCompactResponse(payload string) (*verb.Response, error) {
	records, err := decodeCompactPayload(payload)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 || records[0].kind != "rsp" {
		return nil, errors.New("the payload carries no rsp record")
	}
	head := records[0]
	response := &verb.Response{
		Outcome:       head.field(0),
		Verb:          head.field(1),
		Refusal:       head.field(2),
		Detail:        head.field(3),
		Basis:         head.field(4),
		Warning:       head.field(5),
		WarningDetail: head.field(6),
		Message:       head.field(7),
	}
	closed := false
	for _, record := range records[1:] {
		switch record.kind {
		case "card":
			card := decodeCompactCard(record)
			response.Card = &card
		case "wstream":
			cards, err := strconv.Atoi(record.field(5))
			if err != nil {
				return nil, fmt.Errorf("the workstream's card count: %w", err)
			}
			response.Workstream = &verb.WorkstreamView{
				ID:     record.field(0),
				Ref:    record.field(1),
				Slug:   record.field(2),
				Title:  record.field(3),
				Status: record.field(4),
				Cards:  cards,
			}
		case "instr":
			response.Instructions = &verb.Instructions{
				Global:   record.field(0),
				Standing: record.field(1),
				State:    record.field(2),
			}
		case "move":
			response.LegalMoves = append(response.LegalMoves, verb.LegalMove{
				State:     record.field(0),
				Ref:       record.field(1),
				Title:     record.field(2),
				Direction: record.field(3),
				Reject:    decodeCompactFlag(record.field(4)),
			})
		case "ctx":
			if response.Context == nil {
				response.Context = map[string]string{}
			}
			response.Context[record.field(0)] = record.field(1)
		case "msgval":
			if response.MessageValues == nil {
				response.MessageValues = map[string]string{}
			}
			response.MessageValues[record.field(0)] = record.field(1)
		case "aff":
			response.Affordances = append([]string{}, record.fields...)
			closed = true
		default:
			return nil, fmt.Errorf("a response carries no %s record", record.kind)
		}
	}
	if !closed {
		return nil, errors.New("the payload carries no aff record to close the block")
	}
	return response, nil
}

// decodeCompactRefusalReport rebuilds a pre-verb refusal from a compact
// payload, reading the same rsp record a verb's response writes.
func decodeCompactRefusalReport(payload string) (refusalReport, error) {
	records, err := decodeCompactPayload(payload)
	if err != nil {
		return refusalReport{}, err
	}
	if len(records) == 0 || records[0].kind != "rsp" {
		return refusalReport{}, errors.New("the payload carries no rsp record")
	}
	head := records[0]
	report := refusalReport{
		Outcome: head.field(0),
		Refusal: head.field(2),
		Detail:  head.field(3),
	}
	for _, record := range records[1:] {
		switch record.kind {
		case "ctx":
			if report.Context == nil {
				report.Context = map[string]string{}
			}
			report.Context[record.field(0)] = record.field(1)
		case "wb":
			report.Workbenches = append(report.Workbenches, bench.Candidate{
				Title: record.field(0),
				Slug:  record.field(1),
				Path:  record.field(2),
			})
		case "aff":
		default:
			return refusalReport{}, fmt.Errorf("a pre-verb refusal carries no %s record", record.kind)
		}
	}
	return report, nil
}

// decodeCompactListing rebuilds an ls answer from a compact payload.
func decodeCompactListing(payload string) (*verb.Listing, error) {
	records, err := decodeCompactPayload(payload)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 || records[0].kind != "lst" {
		return nil, errors.New("the payload carries no lst record")
	}
	listing := &verb.Listing{State: records[0].field(0)}
	for _, record := range records[1:] {
		if record.kind != "card" {
			return nil, fmt.Errorf("a listing carries no %s record", record.kind)
		}
		listing.Cards = append(listing.Cards, decodeCompactCard(record))
	}
	return listing, nil
}

// decodeCompactOffers rebuilds a next answer from a compact payload. A card
// record belongs to the offer above it, and an offer with no card record below
// it offers none.
func decodeCompactOffers(payload string) ([]verb.Offer, error) {
	records, err := decodeCompactPayload(payload)
	if err != nil {
		return nil, err
	}
	var offers []verb.Offer
	for _, record := range records {
		switch record.kind {
		case "off":
			offers = append(offers, verb.Offer{
				State:           record.field(0),
				Title:           record.field(1),
				AwaitingOutside: decodeCompactFlag(record.field(2)),
				NoTaker:         decodeCompactFlag(record.field(3)),
				TakenByPull:     decodeCompactFlag(record.field(4)),
			})
		case "card":
			if len(offers) == 0 {
				return nil, errors.New("a card record stands before any offer")
			}
			card := decodeCompactCard(record)
			offers[len(offers)-1].Card = &card
		default:
			return nil, fmt.Errorf("a next answer carries no %s record", record.kind)
		}
	}
	return offers, nil
}

// settled returns a value with every empty collection replaced by a nil one,
// so a comparison reads the two encodings' one absence as one absence. JSON's
// omitempty drops an empty map and an empty slice, and compact writes the same
// absence as a record it does not append, so neither form can say which of the
// two a value held and neither is asserted to.
func settled(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var settled any
	if err := json.Unmarshal(data, &settled); err != nil {
		return value
	}
	return stripEmpty(settled)
}

// stripEmpty removes every empty array, empty object and empty string from a
// decoded JSON tree, which is what leaves one absence to compare against one
// absence.
func stripEmpty(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		kept := map[string]any{}
		for key, held := range typed {
			settled := stripEmpty(held)
			if settled == nil {
				continue
			}
			kept[key] = settled
		}
		if len(kept) == 0 {
			return nil
		}
		return kept
	case []any:
		var kept []any
		for _, held := range typed {
			kept = append(kept, stripEmpty(held))
		}
		if len(kept) == 0 {
			return nil
		}
		return kept
	case string:
		if typed == "" {
			return nil
		}
		return typed
	case bool:
		if !typed {
			return nil
		}
		return typed
	}
	return value
}

// sameFields reports whether two values carry the same field values, and says
// what differs when they do not.
func sameFields(t *testing.T, what string, compact, canonical any) {
	t.Helper()
	left, right := settled(compact), settled(canonical)
	if reflect.DeepEqual(left, right) {
		return
	}
	t.Errorf("%s: the compact form decodes to different fields\ncompact:   %#v\ncanonical: %#v", what, left, right)
}

// machineForms runs one invocation in both machine forms and returns the two
// payloads. Every caller below runs a read or a refusal, both of which change
// nothing, so the two runs answer about one workbench in one state.
func machineForms(t *testing.T, root string, argv ...string) (compact, canonical invocation) {
	t.Helper()
	compact = runCLI(t, root, append([]string{"--format", "compact"}, argv...)...)
	canonical = runCLI(t, root, append([]string{"--json"}, argv...)...)
	if second := runCLI(t, root, append([]string{"--json"}, argv...)...); second.out != canonical.out {
		t.Fatalf("%v answers differently when it is run twice, so the two forms below are not answering about one workbench", argv)
	}
	return compact, canonical
}

// TestFormatResolvesFromTheFlagTheMarkerAndTheEnvironment asserts dinah-31
// AC-1: the nine ways a caller can name an output format, and what each of them
// resolves to or refuses.
func TestFormatResolvesFromTheFlagTheMarkerAndTheEnvironment(t *testing.T) {
	cases := []struct {
		name        string
		jsonFlag    bool
		formatFlag  string
		environment string
		want        outputFormat
		refusal     string
		detail      string
	}{
		{name: "the marker alone", jsonFlag: true, want: formatJSON},
		{name: "the flag alone", formatFlag: "json", want: formatJSON},
		{name: "the environment alone", environment: "json", want: formatJSON},
		{name: "the marker and the flag agreeing", jsonFlag: true, formatFlag: "json", want: formatJSON},
		{name: "nothing at either rung", want: formatHuman},
		{name: "the flag naming compact", formatFlag: "compact", want: formatCompact},
		{name: "the environment naming compact", environment: "compact", want: formatCompact},
		{
			name:       "the marker and the flag disagreeing",
			jsonFlag:   true,
			formatFlag: "compact",
			refusal:    contract.Usage,
			detail:     "--json conflicts with --format compact",
		},
		{
			name:       "the flag naming nothing the tool writes",
			formatFlag: "cmopact",
			refusal:    contract.UnknownFormat,
			detail:     "cmopact",
		},
		{
			name:        "the environment naming nothing the tool writes",
			environment: "yaml",
			refusal:     contract.UnknownFormat,
			detail:      "yaml",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveFormat(c.jsonFlag, c.formatFlag, c.environment)
			if c.refusal == "" {
				if err != nil {
					t.Fatalf("wanted the format %q, got the refusal %v", c.want, err)
				}
				if got != c.want {
					t.Errorf("wanted the format %q, got %q", c.want, got)
				}
				return
			}
			refusal, ok := err.(*contract.Refusal)
			if !ok {
				t.Fatalf("wanted the refusal %s, got %v", c.refusal, err)
			}
			if refusal.Name != c.refusal {
				t.Errorf("wanted the refusal %s, got %s", c.refusal, refusal.Name)
			}
			if refusal.Detail != c.detail {
				t.Errorf("wanted the detail %q, got %q", c.detail, refusal.Detail)
			}
			if got != formatHuman {
				t.Errorf("a refused format resolved to %q rather than the rendering", got)
			}
		})
	}
}

// TestAnUnknownFormatIsRefusedBeforeTheWorkbenchOpens asserts the other half
// of dinah-31 AC-1: the refusal reaches a caller through the head, and it is
// raised ahead of discovery, so a caller who mistyped a format name is told
// that rather than being told about the workbench they are standing outside
// of.
func TestAnUnknownFormatIsRefusedBeforeTheWorkbenchOpens(t *testing.T) {
	outside := t.TempDir()
	t.Setenv("DINAH_HOME", t.TempDir())
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_WORKBENCH", "")

	t.Setenv("DINAH_FORMAT", "")
	got := runCLI(t, outside, "--format", "cmopact", "status")
	if code := contract.ExitCode(contract.OutcomeRefused); got.code != code {
		t.Errorf("wanted the refused code %d, got %d\n%s", code, got.code, got.errw)
	}
	if name, _, _ := strings.Cut(got.errw, " "); name != contract.UnknownFormat {
		t.Errorf("wanted %s leading stderr, got %q", contract.UnknownFormat, got.errw)
	}
	if !strings.Contains(got.errw, "cmopact") {
		t.Errorf("the refusal does not show the caller their own spelling: %q", got.errw)
	}

	// The same value in the environment earns the same refusal, because a
	// script that exported it deserves what a script that typed it gets.
	t.Setenv("DINAH_FORMAT", "cmopact")
	fromEnvironment := runCLI(t, outside, "status")
	if name, _, _ := strings.Cut(fromEnvironment.errw, " "); name != contract.UnknownFormat {
		t.Errorf("wanted %s from the environment, got %q", contract.UnknownFormat, fromEnvironment.errw)
	}

	// The conflict between the marker and the flag is a misuse of the command
	// line rather than an unrecognised value, so it keeps dinah.usage.
	t.Setenv("DINAH_FORMAT", "")
	conflict := runCLI(t, outside, "--json", "--format", "compact", "status")
	if name, _, _ := strings.Cut(conflict.errw, " "); name != contract.Usage {
		t.Errorf("wanted %s for the conflicting pair, got %q", contract.Usage, conflict.errw)
	}
}

// TestTheCompactGrammarSurvivesTheBytesItEscapes asserts that the three bytes
// the grammar reserves survive a round trip through it, and that a payload the
// grammar does not admit is refused by the reader rather than guessed at.
func TestTheCompactGrammarSurvivesTheBytesItEscapes(t *testing.T) {
	for _, value := range []string{awkwardTitle, `\`, `|`, "\r\n", `\|`, "", "ordinary"} {
		line := compactEscape(value)
		if strings.ContainsAny(line, "\n\r") {
			t.Errorf("the escaped form of %q still carries a line ending: %q", value, line)
		}
		fields, err := decodeCompactLine("k|" + line)
		if err != nil {
			t.Fatalf("decode the escaped form of %q: %v", value, err)
		}
		if len(fields) != 2 || fields[1] != value {
			t.Errorf("%q round-tripped to %#v", value, fields)
		}
	}
	for _, malformed := range []string{`k|a\qb`, `k|a\`} {
		if _, err := decodeCompactLine(malformed); err == nil {
			t.Errorf("%q decoded rather than being refused", malformed)
		}
	}
}

// TestTheCompactListingCarriesEveryFieldTheCanonicalListingCarries asserts
// dinah-31 AC-3: a listing's cards decode to the same field values in the same
// order under both machine forms, for the whole workbench and for one state,
// over a fixture carrying a title with a pipe, a backslash and a newline in
// it, a card in two workstreams with both levels set, and a state holding no
// ready card.
func TestTheCompactListingCarriesEveryFieldTheCanonicalListingCarries(t *testing.T) {
	root := newCompactBench(t)
	listings := [][]string{
		{"ls"},
		{"ls", "intake"},
		{"ls", "doing"},
		{"ls", "approval"},
		{"ls", "done"},
	}
	awkward, workstreams, empty := 0, 0, 0
	for _, argv := range listings {
		t.Run(strings.Join(argv, " "), func(t *testing.T) {
			compact, canonical := machineForms(t, root, argv...)
			decoded, err := decodeCompactListing(compact.out)
			if err != nil {
				t.Fatalf("decode the compact listing: %v\n%s", err, compact.out)
			}
			var wanted verb.Listing
			if err := json.Unmarshal([]byte(canonical.out), &wanted); err != nil {
				t.Fatalf("read the canonical listing: %v\n%s", err, canonical.out)
			}
			sameFields(t, strings.Join(argv, " "), decoded, &wanted)
			if len(wanted.Cards) == 0 {
				empty++
			}
			for _, card := range wanted.Cards {
				if card.Title == awkwardTitle {
					awkward++
				}
				if len(card.Workstreams) == 2 && card.Severity != "" && card.Priority != "" {
					workstreams++
				}
			}
		})
	}
	if awkward == 0 || workstreams == 0 || empty == 0 {
		t.Fatalf("the fixture did not present all three cases: awkward %d, two-workstream %d, empty %d", awkward, workstreams, empty)
	}
}

// TestTheCompactOffersCarryEveryFieldTheCanonicalOffersCarry asserts dinah-31
// AC-4: each state's offer decodes to the same field values in the same order
// under both machine forms, including whether the offer carries a card, over a
// fixture holding a state with a card to offer, a state with nothing ready and
// a state that waits on somebody outside.
func TestTheCompactOffersCarryEveryFieldTheCanonicalOffersCarry(t *testing.T) {
	root := newCompactBench(t)
	compact, canonical := machineForms(t, root, "next")
	decoded, err := decodeCompactOffers(compact.out)
	if err != nil {
		t.Fatalf("decode the compact offers: %v\n%s", err, compact.out)
	}
	var wanted []verb.Offer
	if err := json.Unmarshal([]byte(canonical.out), &wanted); err != nil {
		t.Fatalf("read the canonical offers: %v\n%s", err, canonical.out)
	}
	sameFields(t, "next", decoded, wanted)

	offered, bare, outside := 0, 0, 0
	for _, offer := range wanted {
		switch {
		case offer.Card != nil:
			offered++
		case offer.AwaitingOutside:
			outside++
		default:
			bare++
		}
	}
	if offered == 0 || bare == 0 || outside == 0 {
		t.Fatalf("the fixture did not present all three offers: with a card %d, with none %d, awaiting outside %d", offered, bare, outside)
	}
}

// TestEveryRefusedActDecodesTheSameUnderBothMachineForms asserts the refused
// half of dinah-31 AC-5: claim, move, release, block and unblock each refused
// once, and a write refused over a level name, which is the refusal on this
// path that carries Context. A refusal changes nothing, so both forms answer
// about one workbench in one state and the comparison is between two runs of
// the head rather than between one run and a re-encoding of it.
func TestEveryRefusedActDecodesTheSameUnderBothMachineForms(t *testing.T) {
	root := newCompactBench(t)
	if got := runCLI(t, root, "--json", "claim", "fx-2"); got.code != 0 {
		t.Fatalf("claim fx-2 as the holder: %d %s", got.code, got.errw)
	}
	t.Setenv("DINAH_ACTOR", "bo")

	cases := [][]string{
		{"claim", "fx-2"},
		{"move", "fx-2", "done"},
		{"release", "fx-2"},
		{"block", "fx-2", "an obstacle"},
		{"unblock", "fx-2"},
		{"card", "set", "fx-1", "severity", "bogus"},
	}
	carriedContext := 0
	for _, argv := range cases {
		t.Run(strings.Join(argv, " "), func(t *testing.T) {
			compact, canonical := machineForms(t, root, argv...)
			if compact.code != canonical.code {
				t.Errorf("the two forms exit %d and %d", compact.code, canonical.code)
			}
			decoded, err := decodeCompactResponse(compact.out)
			if err != nil {
				t.Fatalf("decode the compact response: %v\n%s", err, compact.out)
			}
			var wanted verb.Response
			if err := json.Unmarshal([]byte(canonical.out), &wanted); err != nil {
				t.Fatalf("read the canonical response: %v\n%s", err, canonical.out)
			}
			if wanted.Outcome != contract.OutcomeRefused {
				t.Fatalf("wanted a refused outcome, got %q", wanted.Outcome)
			}
			sameFields(t, strings.Join(argv, " "), decoded, &wanted)
			if len(wanted.Context) > 0 {
				carriedContext++
			}
		})
	}
	if carriedContext == 0 {
		t.Fatal("no refusal carried Context, so the ctx record went unexercised")
	}
}

// TestEveryAcceptedActDecodesTheSameUnderBothMachineForms asserts the OK half
// of dinah-31 AC-5: claim, move, release, block and unblock each carried to an
// OK outcome, and a pull that finds nothing, which is the answer on this path
// carrying a Message and its named values.
//
// An accepted act changes the workbench, so a second run of it answers about a
// card that has moved and a revision that has changed. The head is therefore
// run once, in the canonical form, and its own answer is encoded compactly and
// read back, which asserts what the criterion asks: one response value carries
// the same fields through either encoding. The refused half above runs the head
// twice and covers the wiring from emit through to compactEncode.
func TestEveryAcceptedActDecodesTheSameUnderBothMachineForms(t *testing.T) {
	root := newCompactBench(t)
	// The order is what each act's own preconditions admit: an unblock leaves
	// the card ready rather than held, so the claim before a release is taken
	// again, and the pull is aimed at the state whose upstream is empty, which
	// is the answer on this path that carries a Message and its named values.
	cases := [][]string{
		{"claim", "fx-2"},
		{"block", "fx-2", "an obstacle"},
		{"unblock", "fx-2"},
		{"claim", "fx-2"},
		{"release", "fx-2"},
		{"move", "fx-2", "intake"},
		{"pull", "done"},
		{"workstream", "new", "Gamma"},
		{"workstream", "set", "gamma", "status", "paused"},
	}
	instructions, moves, messages, workstreams := 0, 0, 0, 0
	for _, argv := range cases {
		t.Run(strings.Join(argv, " "), func(t *testing.T) {
			got := runCLI(t, root, append([]string{"--json"}, argv...)...)
			if got.code != 0 {
				t.Fatalf("%v: %d %s", argv, got.code, got.errw)
			}
			var wanted verb.Response
			if err := json.Unmarshal([]byte(got.out), &wanted); err != nil {
				t.Fatalf("read the canonical response: %v\n%s", err, got.out)
			}
			encoded, ok := compactEncode(&wanted)
			if !ok {
				t.Fatal("compactEncode declines a verb's response")
			}
			decoded, err := decodeCompactResponse(encoded)
			if err != nil {
				t.Fatalf("decode the compact response: %v\n%s", err, encoded)
			}
			sameFields(t, strings.Join(argv, " "), decoded, &wanted)
			if wanted.Instructions != nil {
				instructions++
			}
			if len(wanted.LegalMoves) > 0 {
				moves++
			}
			if wanted.Message != "" && len(wanted.MessageValues) > 0 {
				messages++
			}
			if wanted.Workstream != nil {
				workstreams++
			}
		})
	}
	if instructions == 0 || moves == 0 || messages == 0 || workstreams == 0 {
		t.Fatalf("the acts did not present all four members: instructions %d, legal moves %d, message values %d, workstream %d", instructions, moves, messages, workstreams)
	}
}

// TestTheCompactFormReachesBothResponseCallSites asserts the wiring the
// accepted-act test above cannot: emit and emitWorkstream are the only two
// call sites that produce a verb's response, and an act that changes the
// workbench can be run only once, so each is run here in the compact form
// alone and read back for the subject it wrote.
func TestTheCompactFormReachesBothResponseCallSites(t *testing.T) {
	root := newCompactBench(t)

	claimed := runCLI(t, root, "--format", "compact", "claim", "fx-2")
	if claimed.code != 0 {
		t.Fatalf("claim fx-2: %d %s", claimed.code, claimed.errw)
	}
	fromEmit, err := decodeCompactResponse(claimed.out)
	if err != nil {
		t.Fatalf("decode the claim: %v\n%s", err, claimed.out)
	}
	if fromEmit.Card == nil || fromEmit.Card.Ref != "fx-2" || fromEmit.Verb != "claim" {
		t.Errorf("emit's compact response does not carry the card it acted on: %#v", fromEmit)
	}

	made := runCLI(t, root, "--format", "compact", "workstream", "new", "Gamma")
	if made.code != 0 {
		t.Fatalf("workstream new Gamma: %d %s", made.code, made.errw)
	}
	fromWorkstream, err := decodeCompactResponse(made.out)
	if err != nil {
		t.Fatalf("decode the workstream act: %v\n%s", err, made.out)
	}
	if fromWorkstream.Workstream == nil || fromWorkstream.Workstream.Title != "Gamma" {
		t.Errorf("emitWorkstream's compact response does not carry the workstream it wrote: %#v", fromWorkstream)
	}
}

// TestAPreVerbRefusalDecodesTheSameUnderBothMachineForms asserts dinah-31
// AC-6: a refusal raised before any verb runs carries the same Refusal, Detail
// and Context under both machine forms, and the ambiguous-workbench refusal
// carries the same candidate rows as well.
func TestAPreVerbRefusalDecodesTheSameUnderBothMachineForms(t *testing.T) {
	root := newCompactBench(t)
	compact, canonical := machineForms(t, root, "bogus-command")
	decoded, err := decodeCompactRefusalReport(compact.out)
	if err != nil {
		t.Fatalf("decode the compact refusal: %v\n%s", err, compact.out)
	}
	var wanted refusalReport
	if err := json.Unmarshal([]byte(canonical.out), &wanted); err != nil {
		t.Fatalf("read the canonical refusal: %v\n%s", err, canonical.out)
	}
	if wanted.Outcome != contract.OutcomeRefused || wanted.Refusal != contract.UnknownVerb {
		t.Fatalf("wanted a %s refusal, got %q %q", contract.UnknownVerb, wanted.Outcome, wanted.Refusal)
	}
	sameFields(t, "a command the tool does not offer", decoded, wanted)

	// dinah.ambiguous-workbench is the one pre-verb refusal whose machine form
	// carries more than its named values, so it is read separately. A second
	// init beside the first is what leaves the search a choice it cannot make.
	if got := runCLI(t, root, "--json", "init", "--slug", "second", "--operator", "alka"); got.code != 0 {
		t.Fatalf("the second init: %d %s", got.code, got.errw)
	}
	ambiguousCompact, ambiguousCanonical := machineForms(t, root, "status")
	ambiguousDecoded, err := decodeCompactRefusalReport(ambiguousCompact.out)
	if err != nil {
		t.Fatalf("decode the compact ambiguous refusal: %v\n%s", err, ambiguousCompact.out)
	}
	var ambiguousWanted refusalReport
	if err := json.Unmarshal([]byte(ambiguousCanonical.out), &ambiguousWanted); err != nil {
		t.Fatalf("read the canonical ambiguous refusal: %v\n%s", err, ambiguousCanonical.out)
	}
	if ambiguousWanted.Refusal != contract.AmbiguousWorkbench {
		t.Fatalf("wanted %s, got %q", contract.AmbiguousWorkbench, ambiguousWanted.Refusal)
	}
	if len(ambiguousWanted.Workbenches) < 2 {
		t.Fatalf("the ambiguous refusal names %d workbenches, so the wb record went unexercised", len(ambiguousWanted.Workbenches))
	}
	sameFields(t, "the ambiguous workbench", ambiguousDecoded, ambiguousWanted)
}

// TestAShapeWithNoCompactRenderingEmitsTheCanonicalJSON asserts dinah-31 AC-7:
// a command whose answer is not one of the three compact-capable shapes writes
// canonical JSON under --format compact, byte for byte what --json writes, so
// a driver loop reading compact never has to handle a form the tool declined
// to produce.
func TestAShapeWithNoCompactRenderingEmitsTheCanonicalJSON(t *testing.T) {
	root := newCompactBench(t)
	for _, argv := range [][]string{
		{"states"},
		{"show", "fx-2"},
		{"status"},
		{"config"},
		{"check"},
		{"query", "state:doing"},
		{"log", "fx-2"},
		{"workbenches"},
		{"version"},
		{"workstream"},
	} {
		t.Run(strings.Join(argv, " "), func(t *testing.T) {
			compact, canonical := machineForms(t, root, argv...)
			if compact.out != canonical.out {
				t.Errorf("the compact run differs from the canonical one\ncompact:   %q\ncanonical: %q", compact.out, canonical.out)
			}
			var decoded any
			if err := json.Unmarshal([]byte(compact.out), &decoded); err != nil {
				t.Errorf("the compact run did not write JSON: %v\n%s", err, compact.out)
			}
		})
	}
}

// wantVersionLine is the line every compact payload opens on, written out as
// the text a consumer matches against rather than composed from
// compactVersion. Composing it would compare the constant with itself, and an
// assertion that moves with the value it checks holds nothing still: the
// grammar could renumber itself, break every consumer reading the marker, and
// leave the suite green.
//
// So this literal is the pin. Changing compactVersion reddens the test below
// by name and does not compile away, which makes whoever renumbers the grammar
// say so here deliberately.
const wantVersionLine = "fmt|compact|1"

// TestTheCompactFormOpensOnItsVersionRecord asserts the framing decision on
// this card: every compact payload opens with fmt|compact|1, before any other
// record, so a caller can check the version before it assumes the field order.
func TestTheCompactFormOpensOnItsVersionRecord(t *testing.T) {
	root := newCompactBench(t)
	for _, argv := range [][]string{{"ls"}, {"next"}, {"claim", "fx-2"}} {
		got := runCLI(t, root, append([]string{"--format", "compact"}, argv...)...)
		opening, _, _ := strings.Cut(got.out, "\n")
		if opening != wantVersionLine {
			t.Errorf("%v opens on %q rather than %q, and a consumer checking the marker before it trusts the field order reads the wrong version", argv, opening, wantVersionLine)
		}
		if !strings.HasSuffix(got.out, "\n") || strings.HasSuffix(got.out, "\n\n") {
			t.Errorf("%v does not end on exactly one newline: %q", argv, got.out)
		}
	}
}
