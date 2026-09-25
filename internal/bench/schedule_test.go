package bench

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"dinah/internal/contract"
)

// scheduleFixture plants the registry fixture with a dinah.schedule block, given
// whole as a person types it into workbench.md, above its columns list, and
// stamps it at the format given. An empty block plants none.
func scheduleFixture(t *testing.T, format int, block string) string {
	t.Helper()
	return conditionedFixture(t, format, block)
}

// readScheduleOf reads the settings and defects of a workbench whose workbench.md
// carries the block given.
func readScheduleOf(t *testing.T, block string) (ScheduleSettings, []ScheduleDefect) {
	t.Helper()
	return openConditioned(t, scheduleFixture(t, StorageFormat, block)).Schedule()
}

// defectOn reports the first defect naming a member, and false where none does.
func defectOn(defects []ScheduleDefect, member string) (ScheduleDefect, bool) {
	for _, defect := range defects {
		if defect.Member == member {
			return defect, true
		}
	}
	return ScheduleDefect{}, false
}

// TestReadScheduleFallsBackMemberByMember is dinah-605/criteria/2. Every row
// names the block as written, the zone and window the reader should answer, and
// the defect it should report, so a reader that refused where it should fall
// back, or admitted where it should refuse, fails the row naming the spelling.
func TestReadScheduleFallsBackMemberByMember(t *testing.T) {
	cases := []struct {
		name   string
		block  string
		zone   string
		soon   int
		member string // the member the one defect names, "-" for the whole block, "" for none
		defect string
	}{
		{name: "no block", block: "", zone: "UTC", soon: 7},
		{name: "a block with nothing beneath it", block: "dinah.schedule:\n", zone: "UTC", soon: 7},
		{name: "Asia/Singapore", block: "dinah.schedule:\n  time_zone: Asia/Singapore\n", zone: "Asia/Singapore", soon: 7},
		{name: "UTC declared", block: "dinah.schedule:\n  time_zone: UTC\n", zone: "UTC", soon: 7},
		{name: "Local", block: "dinah.schedule:\n  time_zone: Local\n", zone: "UTC", soon: 7, member: "time_zone", defect: ScheduleMalformedMember},
		{name: "Mars/Olympus", block: "dinah.schedule:\n  time_zone: Mars/Olympus\n", zone: "UTC", soon: 7, member: "time_zone", defect: ScheduleMalformedMember},
		{name: "an empty zone", block: "dinah.schedule:\n  time_zone: \"\"\n", zone: "UTC", soon: 7, member: "time_zone", defect: ScheduleMalformedMember},
		{name: "soon 0", block: "dinah.schedule:\n  soon_days: 0\n", zone: "UTC", soon: 0},
		{name: "soon 365", block: "dinah.schedule:\n  soon_days: 365\n", zone: "UTC", soon: 365},
		{name: "soon -1", block: "dinah.schedule:\n  soon_days: -1\n", zone: "UTC", soon: 7, member: "soon_days", defect: ScheduleMalformedMember},
		{name: "soon 366", block: "dinah.schedule:\n  soon_days: 366\n", zone: "UTC", soon: 7, member: "soon_days", defect: ScheduleMalformedMember},
		{name: "soon 1.5", block: "dinah.schedule:\n  soon_days: 1.5\n", zone: "UTC", soon: 7, member: "soon_days", defect: ScheduleMalformedMember},
		{name: "soon quoted", block: "dinah.schedule:\n  soon_days: \"7\"\n", zone: "UTC", soon: 7, member: "soon_days", defect: ScheduleMalformedMember},
		{name: "a scalar block", block: "dinah.schedule: soon\n", zone: "UTC", soon: 7, member: "-", defect: ScheduleNotAMapping},
		{name: "an unknown member", block: "dinah.schedule:\n  time-zone: Asia/Singapore\n", zone: "UTC", soon: 7, member: "time-zone", defect: ScheduleUnknownMember},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			settings, defects := readScheduleOf(t, c.block)
			if settings.ZoneName != c.zone || settings.Zone == nil || settings.Zone.String() != c.zone {
				t.Errorf("the zone read as %q (%v), want %q", settings.ZoneName, settings.Zone, c.zone)
			}
			if settings.SoonDays != c.soon {
				t.Errorf("the window read as %d, want %d", settings.SoonDays, c.soon)
			}
			if c.member == "" {
				if len(defects) != 0 {
					t.Errorf("wanted no defect, got %+v", defects)
				}
				return
			}
			if len(defects) != 1 {
				t.Fatalf("wanted exactly one defect, got %+v", defects)
			}
			member := c.member
			if member == "-" {
				member = ""
			}
			if defects[0].Member != member || defects[0].Defect != c.defect {
				t.Errorf("wanted %s on %q, got %+v", c.defect, member, defects[0])
			}
		})
	}
}

// TestADeclaredZoneIsReportedAsDeclared holds ZoneDeclared to the one case that
// sets it, because the zone notice reads it: a default UTC and a declared UTC
// read the same day, and only one of them was chosen by somebody.
func TestADeclaredZoneIsReportedAsDeclared(t *testing.T) {
	if settings, _ := readScheduleOf(t, ""); settings.ZoneDeclared {
		t.Error("a workbench declaring no block reports its zone as declared")
	}
	if settings, _ := readScheduleOf(t, "dinah.schedule:\n  time_zone: Mars/Olympus\n"); settings.ZoneDeclared {
		t.Error("an unusable zone reports as declared")
	}
	if settings, _ := readScheduleOf(t, "dinah.schedule:\n  time_zone: UTC\n"); !settings.ZoneDeclared {
		t.Error("a declared UTC reports as not declared")
	}
}

// TestTheScheduleKeyIsMintedAndReadFromTheWorkbenchAlone is the rest of
// dinah-605/criteria/2: the key is one Dinah mints, so a guide may quote it,
// and ReadSchedule reads only the frontmatter it is handed, which Open hands
// the workbench's own anchor. A user's config.md carrying the block changes
// nothing, which is asserted end to end in internal/verb.
func TestTheScheduleKeyIsMintedAndReadFromTheWorkbenchAlone(t *testing.T) {
	minted := false
	for _, key := range contract.MintedKeys {
		if key == ScheduleKey {
			minted = true
		}
	}
	if !minted {
		t.Errorf("contract.MintedKeys %v does not carry %s", contract.MintedKeys, ScheduleKey)
	}
	if ScheduleKey != "dinah.schedule" {
		t.Errorf("the key is spelled %q", ScheduleKey)
	}
}

// TestTodayIsReadInTheDeclaredZone is the bench half of dinah-605/criteria/3:
// one instant falls on two calendar days, and the workbench's zone decides
// which.
func TestTodayIsReadInTheDeclaredZone(t *testing.T) {
	instant := time.Date(2026, 10, 2, 17, 0, 0, 0, time.UTC)
	singapore := openConditioned(t, scheduleFixture(t, StorageFormat, "dinah.schedule:\n  time_zone: Asia/Singapore\n"))
	if got := singapore.Today(instant).String(); got != "2026-10-03" {
		t.Errorf("Singapore reads today as %s, want 2026-10-03", got)
	}
	plain := openConditioned(t, scheduleFixture(t, StorageFormat, ""))
	if got := plain.Today(instant).String(); got != "2026-10-02" {
		t.Errorf("a workbench naming no zone reads today as %s, want 2026-10-02", got)
	}
}

// TestParseDateAdmitsTheCalendarAndNothingElse pins the guard every date
// write and every query value runs.
func TestParseDateAdmitsTheCalendarAndNothingElse(t *testing.T) {
	for _, admitted := range []string{"2026-02-28", "2024-02-29", "2026-10-06"} {
		if _, ok := ParseDate(admitted); !ok {
			t.Errorf("%s was refused", admitted)
		}
	}
	for _, refused := range []string{"2026-02-30", "2026-10-1", "today", "", "2026-10-06T00:00:00Z", "2025-02-29", " 2026-10-06"} {
		if _, ok := ParseDate(refused); ok {
			t.Errorf("%q was admitted", refused)
		}
	}
	day, _ := ParseDate("2026-10-03")
	if got := day.AddDays(7).String(); got != "2026-10-10" {
		t.Errorf("seven days on is %s", got)
	}
	if got := day.AddDays(-3).String(); got != "2026-09-30" {
		t.Errorf("three days back is %s", got)
	}
	later, _ := ParseDate("2026-10-10")
	if got := day.DaysUntil(later); got != 7 {
		t.Errorf("the days until read %d", got)
	}
}

// TestADateLandsAfterTheRouteAndBeforeTheWorkstreams pins where a written date
// sits in the header, whichever order the three were written in: after the
// route and the levels, in its own order, and ahead of the workstreams.
func TestADateLandsAfterTheRouteAndBeforeTheWorkstreams(t *testing.T) {
	fm, _ := ParseAnchor("---\ntitle: T\ncolumn: c\nstate: ready\nseverity: major\nroute: short\nworkstreams:\n  - w1\n---\n")
	SetScheduleDate(fm, DueField, "2026-10-10")
	SetScheduleDate(fm, StartAfterField, "2026-10-06")
	SetScheduleDate(fm, StartByField, "2026-10-08")
	want := []string{"title", "column", "state", "severity", "route", StartAfterField, StartByField, DueField, "workstreams"}
	if got := strings.Join(fm.Keys(), " "); got != strings.Join(want, " ") {
		t.Errorf("the keys read %s, want %s", got, strings.Join(want, " "))
	}
	if got := fm.Value(DueField); got != "2026-10-10" {
		t.Errorf("due reads %q", got)
	}
	rendered := fm.Render("")
	if !strings.Contains(rendered, "\ndue: 2026-10-10\n") {
		t.Errorf("the date is not written bare:\n%s", rendered)
	}
	SetScheduleDate(fm, StartByField, "")
	if fm.Has(StartByField) {
		t.Error("clearing a date left its key behind")
	}
}

// TestAQuotedDateIsHonoured pins the one hand-written spelling beside the bare
// one: under format 9's quoted-scalar rule a quoted value reads as text, and
// that text parses as the same date.
func TestAQuotedDateIsHonoured(t *testing.T) {
	root := newFixture(t)
	edit(t, root, "state: ready", "state: ready\ndue: \"2026-10-10\"")
	card := fixtureCard(t, openConditioned(t, root))
	if date, ok := card.ScheduleDate(DueField); !ok || date.String() != "2026-10-10" {
		t.Errorf("a quoted due date read as %v %v", date, ok)
	}
}

// TestCheckReportsEveryScheduleDefect is the bench half of
// dinah-605/criteria/15: each report fires on the fixture that plants it and on
// no other, and a clean dated workbench at the current format reports none.
func TestCheckReportsEveryScheduleDefect(t *testing.T) {
	t.Run("a clean dated card", func(t *testing.T) {
		root := scheduleFixture(t, StorageFormat, "dinah.schedule:\n  time_zone: Asia/Singapore\n")
		edit(t, root, "state: ready", "state: ready\nstart_after: 2026-10-06\nstart_by: 2026-10-06\ndue: 2026-10-10")
		for _, finding := range findingsOf(t, openConditioned(t, root)) {
			if strings.HasPrefix(finding.Key, "check.schedule") {
				t.Errorf("a clean dated card raised %s %s", finding.Key, finding.Detail)
			}
		}
	})
	t.Run("every pair out of order", func(t *testing.T) {
		root := newFixture(t)
		edit(t, root, "state: ready", "state: ready\nstart_after: 2026-10-12\nstart_by: 2026-10-11\ndue: 2026-10-10")
		findings := findingsOf(t, openConditioned(t, root))
		if n := countOf(findings, FindingScheduleOrder); n != 3 {
			t.Errorf("three pairs out of order raised %d findings", n)
		}
		for _, detail := range []string{
			"start_after 2026-10-12 after start_by 2026-10-11",
			"start_after 2026-10-12 after due 2026-10-10",
			"start_by 2026-10-11 after due 2026-10-10",
		} {
			if !findingWith(findings, FindingScheduleOrder, detail) {
				t.Errorf("no finding reads %q", detail)
			}
		}
	})
	t.Run("a date that does not parse", func(t *testing.T) {
		root := newFixture(t)
		edit(t, root, "state: ready", "state: ready\ndue: 2026-10-1")
		if !findingWith(findingsOf(t, openConditioned(t, root)), FindingScheduleDateMalformed, "due 2026-10-1") {
			t.Error("a malformed due date was not reported")
		}
	})
	t.Run("an unusable block", func(t *testing.T) {
		for block, detail := range map[string]string{
			"dinah.schedule: soon\n":                       "dinah.schedule not a mapping",
			"dinah.schedule:\n  time_zone: Mars/Olympus\n": "dinah.schedule time_zone Mars/Olympus",
			"dinah.schedule:\n  soon_days: 400\n":          "dinah.schedule soon_days 400",
			"dinah.schedule:\n  soon_days: \"7\"\n":        `dinah.schedule soon_days "7"`,
		} {
			findings := findingsOf(t, openConditioned(t, scheduleFixture(t, StorageFormat, block)))
			if !findingWith(findings, FindingScheduleMalformed, detail) {
				t.Errorf("%q raised no finding reading %q: %+v", block, detail, findings)
			}
		}
	})
	t.Run("an unknown member", func(t *testing.T) {
		findings := findingsOf(t, openConditioned(t, scheduleFixture(t, StorageFormat, "dinah.schedule:\n  timezone: UTC\n")))
		if !findingWith(findings, FindingScheduleMemberUnknown, "timezone") {
			t.Errorf("the unknown member was not reported: %+v", findings)
		}
		for _, finding := range findings {
			if finding.Key == FindingScheduleMemberUnknown && finding.Severity != SeverityCleanup {
				t.Errorf("the unknown member is reported at %q", finding.Severity)
			}
		}
	})
}

// TestTheBelowFormatFindingWaitsForADatedCard is the bench half of
// dinah-605/criteria/10: a workbench below format 10 is reported once a live
// card carries a date and not before, and a workbench at 10 never is.
func TestTheBelowFormatFindingWaitsForADatedCard(t *testing.T) {
	root := scheduleFixture(t, 9, "")
	if n := countOf(findingsOf(t, openConditioned(t, root)), FindingScheduleBelowFormat); n != 0 {
		t.Errorf("a format-9 workbench with no dated card raised %d", n)
	}
	edit(t, root, "state: ready", "state: ready\nstart_after: 2026-10-06")
	if !findingWith(findingsOf(t, openConditioned(t, root)), FindingScheduleBelowFormat, "9") {
		t.Error("a format-9 workbench with a dated card raised no below-format finding naming 9")
	}
	current := scheduleFixture(t, StorageFormat, "")
	edit(t, current, "state: ready", "state: ready\nstart_after: 2026-10-06")
	if n := countOf(findingsOf(t, openConditioned(t, current)), FindingScheduleBelowFormat); n != 0 {
		t.Errorf("a workbench at the current format raised %d", n)
	}
}

// TestTheZoneNoticeNeedsADatedCardAndNoZone is the notice half of
// dinah-605/criteria/15: it fires where a live card carries a date and no
// usable zone is declared, and on neither of the two workbenches that differ
// from that in one respect.
func TestTheZoneNoticeNeedsADatedCardAndNoZone(t *testing.T) {
	noticed := func(root string) bool {
		for _, notice := range openConditioned(t, root).Notices() {
			if notice.Key == NoticeScheduleZoneUndeclared {
				return notice.Path == filepath.Join(root, WorkbenchAnchor) && notice.Severity == SeverityCleanup
			}
		}
		return false
	}
	undated := scheduleFixture(t, StorageFormat, "")
	if noticed(undated) {
		t.Error("a workbench with no dated card was noticed")
	}
	dated := scheduleFixture(t, StorageFormat, "")
	edit(t, dated, "state: ready", "state: ready\ndue: 2026-10-10")
	if !noticed(dated) {
		t.Error("a dated workbench declaring no zone was not noticed, or not at its workbench.md")
	}
	zoned := scheduleFixture(t, StorageFormat, "dinah.schedule:\n  time_zone: UTC\n")
	edit(t, zoned, "state: ready", "state: ready\ndue: 2026-10-10")
	if noticed(zoned) {
		t.Error("a dated workbench declaring its zone was noticed")
	}
}

// TestTheScheduleStampIsAFloor is the migration half of dinah-605/criteria/10:
// without the confirmation nothing is written, with it the one format line is,
// a store already at or past the number is left alone, and a store below
// format 7 is refused naming the designation migration.
func TestTheScheduleStampIsAFloor(t *testing.T) {
	root := scheduleFixture(t, 9, "")
	edit(t, root, "state: ready", "state: ready\ndue: 2026-10-10")
	anchor := filepath.Join(root, WorkbenchAnchor)
	before, _ := ReadText(anchor)
	preview, err := openConditioned(t, root).MigrateSchedule(false)
	if err != nil || preview.Stamped || !preview.Preview || preview.From != 9 {
		t.Fatalf("the preview answered %+v %v", preview, err)
	}
	if after, _ := ReadText(anchor); after != before {
		t.Error("the preview wrote the anchor")
	}
	stamped, err := openConditioned(t, root).MigrateSchedule(true)
	if err != nil || !stamped.Stamped {
		t.Fatalf("the stamp answered %+v %v", stamped, err)
	}
	after, _ := ReadText(anchor)
	if after != strings.Replace(before, "format: 9", "format: "+strconv.Itoa(ScheduleFormat), 1) {
		t.Errorf("the stamp wrote more than the format line:\n%s", after)
	}
	if n := countOf(findingsOf(t, openConditioned(t, root)), FindingScheduleBelowFormat); n != 0 {
		t.Errorf("the finding survived the stamp %d times", n)
	}
	again, err := openConditioned(t, root).MigrateSchedule(true)
	if err != nil || again.Stamped {
		t.Errorf("a second stamp answered %+v %v", again, err)
	}

	old := scheduleFixture(t, 6, "")
	opened, err := openFixtureAtAnyFormat(t, old)
	if err != nil {
		t.Fatalf("open the format-6 store: %v", err)
	}
	_, err = opened.MigrateSchedule(true)
	var refusal *contract.Refusal
	if !errors.As(err, &refusal) || refusal.Name != contract.StoreAwaitingMigration {
		t.Errorf("a format-6 store answered %v, want %s", err, contract.StoreAwaitingMigration)
	}
	if text, _ := ReadText(filepath.Join(old, WorkbenchAnchor)); !strings.Contains(text, "format: 6\n") {
		t.Error("the refused stamp wrote the format-6 anchor")
	}
}
