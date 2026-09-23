package bench

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestTheActorUnmarshallerTakesBothShapesAndConsultsNoFormat drives
// dinah-496's tolerant-branch criterion and CORE-ACTING-1. bench.Actor accepts
// a JSON string as the name alone and an object as itself, on a journal
// carrying both shapes in one file, and it consults no workbench format to
// decide which.
//
// The journal is read through ReadJournal with no Bench in hand at all, which
// is the whole point of the branch being ungated: a store part-way through the
// migration carries both shapes in one file, and the line answers the question
// the format would have been asked.
func TestTheActorUnmarshallerTakesBothShapesAndConsultsNoFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), JournalName)
	write(t, path, strings.Join([]string{
		`{"ts":"2026-09-15T09:00:00Z","event":"created","actor":"ana"}`,
		`{"ts":"2026-09-15T09:01:00Z","event":"claimed","actor":{"name":"claude","harness":"claude-code"}}`,
		`{"ts":"2026-09-15T09:02:00Z","event":"released","actor":"ana"}`,
		"",
	}, "\n"))
	events, torn, err := ReadJournal(path)
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}
	if torn {
		t.Fatal("the reader called the journal torn")
	}
	strings, objects := 0, 0
	for _, event := range events {
		if event.Actor.Harness == "" {
			strings++
			continue
		}
		objects++
	}
	if strings != 2 || objects != 1 {
		t.Errorf("the reader found %d string actors and %d object actors, wanted 2 and 1", strings, objects)
	}
	for at, want := range []string{"ana", "claude", "ana"} {
		if events[at].Actor.Name != want {
			t.Errorf("line %d names %q, wanted %q", at+1, events[at].Actor.Name, want)
		}
	}
}

// TestAStringActorReadsBackWithEveryOtherMemberUnchanged drives dinah-496's
// pre-migration read criterion and CORE-ACTING-3, which forbids refusing an act
// over an absent description.
//
// The journal is one a build before this card wrote, on a workbench declaring
// storage format 4 with no migration having run. Every line presents as an
// object carrying the name alone, every other member is unchanged, and the
// count of lines read is asserted so a reader that read nothing cannot pass.
func TestAStringActorReadsBackWithEveryOtherMemberUnchanged(t *testing.T) {
	root := containedPath(t.TempDir())
	write(t, filepath.Join(root, WorkbenchAnchor), strings.Replace(tieredDefinition, "format: 7", "format: 4", 1))
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), columnDefinition)
	write(t, filepath.Join(root, CardNumbersName), "1 c00000000001\n")
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), cleanCard)
	path := filepath.Join(root, CardsDir, "c00000000001", JournalName)
	write(t, path, strings.Join([]string{
		`{"ts":"2026-08-17T09:00:00Z","event":"created","actor":"alka","title":"A card","to":"b00000000001","to_title":"Only"}`,
		`{"ts":"2026-08-17T09:01:00Z","event":"claimed","actor":"alka","expires":"2026-08-17T10:01:00Z"}`,
		`{"ts":"2026-08-17T09:02:00Z","event":"blocked","actor":"brin","reason":"waiting on the vendor","kind":"external"}`,
		"",
	}, "\n"))
	opened, err := openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("open a workbench declaring format 4: %v", err)
	}
	if opened.Format != 4 {
		t.Fatalf("the workbench opened at format %d, wanted 4", opened.Format)
	}
	events, _, err := ReadJournal(path)
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("the reader read %d lines, wanted 3", len(events))
	}
	for at, want := range []string{"alka", "alka", "brin"} {
		got := events[at].Actor
		if got.Name != want {
			t.Errorf("line %d names %q, wanted %q", at+1, got.Name, want)
		}
		if got.Harness != "" || got.Provider != "" || got.Model != "" || got.Server != "" {
			t.Errorf("line %d invented a description nobody declared: %+v", at+1, got)
		}
	}
	if events[1].Expires != "2026-08-17T10:01:00Z" {
		t.Errorf("the claim's expiry did not survive the read: %+v", events[1])
	}
	if events[2].Reason != "waiting on the vendor" || events[2].Kind != "external" {
		t.Errorf("the block's reason and kind did not survive the read: %+v", events[2])
	}
}

// TestAWrittenActorCarriesExactlyWhatTheCallerDeclared drives dinah-496's
// journal-shape criterion together with CORE-ACTING-2. A line written by a
// caller declaring a harness, provider, model and server carries all four under
// the OpenTelemetry spellings; one declaring the first three and no server
// carries no server member; and one declaring none of the four carries the name
// alone. One reader reads all three.
func TestAWrittenActorCarriesExactlyWhatTheCallerDeclared(t *testing.T) {
	path := filepath.Join(t.TempDir(), JournalName)
	written := []Actor{
		{Name: "claude", Harness: "claude-code", Provider: "anthropic", Model: "claude-opus-5", Server: "ollama.com"},
		{Name: "claude", Harness: "claude-code", Provider: "anthropic", Model: "claude-opus-5"},
		NamedActor("ana"),
	}
	for _, actor := range written {
		if err := AppendEvent(path, Event{TS: "2026-09-15T09:00:00Z", Event: "claimed", Actor: actor}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	text, err := ReadText(path)
	if err != nil {
		t.Fatalf("read the journal text: %v", err)
	}
	lines := SplitLines(strings.TrimRight(text, "\n"))
	if len(lines) != 3 {
		t.Fatalf("the journal holds %d lines, wanted 3", len(lines))
	}
	for at, want := range [][]string{
		{`"name":"claude"`, `"harness":"claude-code"`, `"gen_ai.provider.name":"anthropic"`, `"gen_ai.request.model":"claude-opus-5"`, `"server.address":"ollama.com"`},
		{`"name":"claude"`, `"harness":"claude-code"`, `"gen_ai.provider.name":"anthropic"`, `"gen_ai.request.model":"claude-opus-5"`},
		{`"actor":{"name":"ana"}`},
	} {
		for _, member := range want {
			if !strings.Contains(lines[at], member) {
				t.Errorf("line %d does not carry %s: %s", at+1, member, lines[at])
			}
		}
	}
	if strings.Contains(lines[1], "server.address") {
		t.Errorf("a caller declaring no server wrote a server member: %s", lines[1])
	}
	for _, absent := range []string{"harness", "gen_ai.provider.name", "gen_ai.request.model", "server.address"} {
		if strings.Contains(lines[2], absent) {
			t.Errorf("a caller declaring nothing wrote %s: %s", absent, lines[2])
		}
	}
	events, _, err := ReadJournal(path)
	if err != nil {
		t.Fatalf("read the journal back: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("the reader read %d lines, wanted 3", len(events))
	}
	for at, want := range written {
		if events[at].Actor != want {
			t.Errorf("line %d read back as %+v, wanted %+v", at+1, events[at].Actor, want)
		}
	}
}

// TestAnUnderstoodMemberOfADescriptionSurvives drives CORE-ACTING-4. A member
// of the actor object this build does not declare is preserved by the read of a
// line, which is what a tool meeting a description richer than its own owes.
//
// The read is the assertion, not a write: this build re-encodes nothing on
// open, and cmd/dinah-migrate-actors is the one place that rewrites a line at
// all, where the whole line outside the actor value travels literally.
func TestAnUnderstoodMemberOfADescriptionSurvives(t *testing.T) {
	line := `{"ts":"2026-09-15T09:00:00Z","event":"claimed","actor":{"name":"claude","gen_ai.agent.name":"reviewer"}}`
	var event Event
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatalf("read a line carrying a member this build does not declare: %v", err)
	}
	if event.Actor.Name != "claude" {
		t.Errorf("the name read as %q", event.Actor.Name)
	}
}
