package bench

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
	// The embedded zone database is what time.LoadLocation falls back to
	// where the host carries none, which a Windows host does not, so every
	// IANA name a dinah.schedule block declares resolves on every host. It is
	// imported here, beside the one call that needs it, rather than in a
	// command's main, so every binary and every test reading the block
	// resolves the same names.
	_ "time/tzdata"

	"dinah/internal/contract"
)

// ScheduleKey is the top-level frontmatter key the schedule settings are
// declared under. It is read from the workbench's own workbench.md and from
// nowhere else, and no verb writes it.
const ScheduleKey = contract.ScheduleKey

// ScheduleFields lists the three date fields in the order a card carries them
// and a reader meets them.
var ScheduleFields = []string{StartAfterField, StartByField, DueField}

// The two members of the dinah.schedule block.
const (
	scheduleZoneMember = "time_zone"
	scheduleSoonMember = "soon_days"
)

// The defaults a member takes where the block leaves it out or carries it
// unreadably.
const (
	// DefaultScheduleZone is the zone today is read in where none is
	// declared.
	DefaultScheduleZone = "UTC"
	// DefaultSoonDays is how many days ahead soon looks where no window is
	// declared.
	DefaultSoonDays = 7
	// maxSoonDays is the widest soon window a block may declare.
	maxSoonDays = 365
)

// The three defects a dinah.schedule block can carry, as machine tokens. None
// of them refuses anything: the member at fault takes its default and dinah
// check names it.
const (
	ScheduleNotAMapping     = "not-a-mapping"
	ScheduleMalformedMember = "malformed-member"
	ScheduleUnknownMember   = "unknown-member"
)

// scheduleLocalZone is the one zone name time.LoadLocation resolves that a
// block may not declare, because it names whatever zone the machine asking is
// set to.
const scheduleLocalZone = "Local"

// soonLiteral is the one spelling a soon window takes: a whole number written
// in decimal with no sign, no fraction and no leading zero.
var soonLiteral = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

// ScheduleSettings is what the dinah.schedule layer declares, with every
// member the block leaves out, or carries unreadably, at its default.
type ScheduleSettings struct {
	// Zone is the location today is read in.
	Zone *time.Location
	// ZoneName is the zone as declared, and "UTC" where defaulted.
	ZoneName string
	// ZoneDeclared says the block declared a usable time_zone, which is what
	// the notice about reading today in UTC asks.
	ZoneDeclared bool
	// SoonDays is how many days ahead due_soon and start_soon look.
	SoonDays int
}

// ScheduleDefect names one member the reader could not use, or the whole
// block where it is not a mapping. Member is empty for the whole block.
type ScheduleDefect struct {
	// Defect is ScheduleNotAMapping, ScheduleMalformedMember or
	// ScheduleUnknownMember.
	Defect string
	// Member is the member at fault, empty where the block itself is.
	Member string
	// Read is the text the reader met for the member: the string a JSON
	// string spells, and the JSON itself for anything else.
	Read string
}

// DefaultSchedule is the settings a workbench declaring no dinah.schedule
// block carries.
func DefaultSchedule() ScheduleSettings {
	return ScheduleSettings{Zone: time.UTC, ZoneName: DefaultScheduleZone, SoonDays: DefaultSoonDays}
}

// ReadSchedule reads the dinah.schedule key of whatever frontmatter it is
// handed, through blockValue and firstMembers as ReadUrgency reads its block.
//
// A key the frontmatter does not carry, and a key with nothing readable
// beneath it, both declare nothing, and both members take their defaults. A
// value that is not a mapping is ScheduleNotAMapping. A member that cannot be
// used is ScheduleMalformedMember and reads as its default, and a member the
// block does not know is ScheduleUnknownMember and is otherwise ignored.
//
// A defect falls back rather than refusing, which parts this reader from
// ReadUrgency. The settings reach next, pull, show and every listing, so a
// refusal would stop all work on a workbench over a typo, where a fallback
// misplaces today by at most a day and dinah check reports it.
func ReadSchedule(fm *Frontmatter) (ScheduleSettings, []ScheduleDefect) {
	settings := DefaultSchedule()
	if fm == nil || !fm.Has(ScheduleKey) {
		return settings, nil
	}
	raw := blockValue(fm, ScheduleKey)
	if sameJSON(raw, mustMarshal("")) {
		return settings, nil
	}
	members, mapping := firstMembers(raw)
	if !mapping {
		return settings, []ScheduleDefect{{Defect: ScheduleNotAMapping}}
	}
	var defects []ScheduleDefect
	for _, member := range members {
		switch member.name {
		case scheduleZoneMember:
			name, zone, ok := scheduleZone(member.value)
			if !ok {
				defects = append(defects, ScheduleDefect{Defect: ScheduleMalformedMember, Member: member.name, Read: scheduleRead(member.value)})
				continue
			}
			settings.Zone, settings.ZoneName, settings.ZoneDeclared = zone, name, true
		case scheduleSoonMember:
			days, ok := soonDays(member.value)
			if !ok {
				defects = append(defects, ScheduleDefect{Defect: ScheduleMalformedMember, Member: member.name, Read: scheduleRead(member.value)})
				continue
			}
			settings.SoonDays = days
		default:
			defects = append(defects, ScheduleDefect{Defect: ScheduleUnknownMember, Member: member.name, Read: scheduleRead(member.value)})
		}
	}
	return settings, defects
}

// scheduleZone reads a time_zone member: a JSON string, not empty, not Local,
// that time.LoadLocation resolves. Local is refused because it would make
// today depend on the machine asking rather than on the workbench.
func scheduleZone(raw json.RawMessage) (string, *time.Location, bool) {
	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return "", nil, false
	}
	if name == "" || name == scheduleLocalZone {
		return "", nil, false
	}
	zone, err := time.LoadLocation(name)
	if err != nil {
		return "", nil, false
	}
	return name, zone, true
}

// soonDays reads a soon_days member: a JSON number written as a whole decimal
// from 0 to 365. A quoted number is a string and is refused, and so is a
// fraction or an exponent, whatever value it spells.
func soonDays(raw json.RawMessage) (int, bool) {
	text := strings.TrimSpace(string(raw))
	if !soonLiteral.MatchString(text) {
		return 0, false
	}
	days, err := strconv.Atoi(text)
	if err != nil || days > maxSoonDays {
		return 0, false
	}
	return days, true
}

// scheduleRead is the text a defect reports for a member: the string itself
// where the member is a JSON string, so a zone name reads as it was written,
// and the JSON otherwise.
func scheduleRead(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return strings.TrimSpace(string(raw))
}

// Schedule answers the workbench's schedule settings and every defect its
// block carries. The settings are always usable, because a defect has
// already fallen back to the member's default.
func (b *Bench) Schedule() (ScheduleSettings, []ScheduleDefect) {
	return b.schedule, append([]ScheduleDefect(nil), b.scheduleDefects...)
}

// Today is the calendar date now falls on in the workbench's declared zone.
// It is the one place today is computed, so every condition, every relative
// query value and every withholding reads the same day.
func (b *Bench) Today(now time.Time) Date {
	zone := b.schedule.Zone
	if zone == nil {
		zone = time.UTC
	}
	local := now.In(zone)
	return DateOf(local.Year(), local.Month(), local.Day())
}

// Date is a calendar date with no time of day and no zone: the whole of one
// day in whatever calendar the workbench reads it in. The zero value is no
// date at all.
type Date struct {
	// day is midnight UTC at the start of the date, which makes whole-day
	// arithmetic exact.
	day time.Time
}

// DateOf is the date of a year, a month and a day.
func DateOf(year int, month time.Month, day int) Date {
	return Date{day: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// ParseDate reads a date written YYYY-MM-DD under FieldDateLayout, which
// refuses a date the calendar does not carry and a field written short, so
// 2026-02-30 and 2026-10-1 are refused and 2026-02-28 is admitted.
func ParseDate(text string) (Date, bool) {
	parsed, err := time.Parse(FieldDateLayout, text)
	if err != nil {
		return Date{}, false
	}
	return Date{day: parsed}, true
}

// IsZero reports whether the date is no date at all.
func (d Date) IsZero() bool {
	return d.day.IsZero()
}

// Before reports whether d is an earlier day than other.
func (d Date) Before(other Date) bool {
	return d.day.Before(other.day)
}

// After reports whether d is a later day than other.
func (d Date) After(other Date) bool {
	return d.day.After(other.day)
}

// AddDays is the date n calendar days after d, or before it where n is
// negative.
func (d Date) AddDays(n int) Date {
	return Date{day: d.day.AddDate(0, 0, n)}
}

// DaysUntil is how many calendar days lie from d to other, negative where
// other is the earlier day.
func (d Date) DaysUntil(other Date) int {
	return int(other.day.Sub(d.day).Hours() / 24)
}

// String writes the date as YYYY-MM-DD, and the empty string for no date.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.day.Format(FieldDateLayout)
}

// ScheduleDate reads one of a card's scheduling dates, reporting false where
// the card carries none or carries one that does not parse. A value that does
// not parse is read as absent by every condition and by selection, and dinah
// check reports it.
func (c *Card) ScheduleDate(field string) (Date, bool) {
	stored := c.scheduleValue(field)
	if stored == "" {
		return Date{}, false
	}
	return ParseDate(stored)
}

// scheduleValue is what the card stores under one of the three date fields,
// and the empty string for any other name.
func (c *Card) scheduleValue(field string) string {
	switch field {
	case StartAfterField:
		return c.StartAfter
	case StartByField:
		return c.StartBy
	case DueField:
		return c.Due
	}
	return ""
}

// CarriesScheduleDate reports whether the card stores anything under any of
// the three date fields, parseable or not.
func (c *Card) CarriesScheduleDate() bool {
	return c.StartAfter != "" || c.StartBy != "" || c.Due != ""
}

// ScheduleOrderViolation is one pair of a card's dates standing in the wrong
// order: the earlier field carries a later date than the later field.
type ScheduleOrderViolation struct {
	First, FirstDate   string
	Second, SecondDate string
}

// ScheduleOrderViolations reports every pair of the card's parseable dates
// out of order, in the order start_after against start_by, start_after
// against due, start_by against due. Equal dates are in order, which is what
// lets a card carry start_after and start_by on one day.
func (c *Card) ScheduleOrderViolations() []ScheduleOrderViolation {
	pairs := [][2]string{
		{StartAfterField, StartByField},
		{StartAfterField, DueField},
		{StartByField, DueField},
	}
	var violations []ScheduleOrderViolation
	for _, pair := range pairs {
		first, ok := c.ScheduleDate(pair[0])
		if !ok {
			continue
		}
		second, ok := c.ScheduleDate(pair[1])
		if !ok {
			continue
		}
		if first.After(second) {
			violations = append(violations, ScheduleOrderViolation{
				First: pair[0], FirstDate: first.String(),
				Second: pair[1], SecondDate: second.String(),
			})
		}
	}
	return violations
}

// scheduleAnchors are the keys a newly written date is placed after, most
// preferred first: the level and route keys Card.Save places under state, in
// the order that leaves the last of them written lowest.
var scheduleAnchors = []string{RouteField, TierField, PriorityField, SeverityField, "state"}

// SetScheduleDate writes one of a card's three dates into its header, or
// removes it where the value is empty. A key the header already carries stays
// where it is. A new key is placed after whichever of the other dates that
// precede it the header carries, and failing those after the route, the
// levels or the state, so the three read in their own order after the card's
// classifications and before its workstreams whoever writes them.
func SetScheduleDate(fm *Frontmatter, field, value string) {
	if value == "" {
		fm.Delete(field)
		return
	}
	if fm.Has(field) {
		fm.Set(field, value)
		return
	}
	var anchors []string
	for _, earlier := range ScheduleFields {
		if earlier == field {
			break
		}
		anchors = append([]string{earlier}, anchors...)
	}
	anchors = append(anchors, scheduleAnchors...)
	for _, anchor := range anchors {
		if fm.Has(anchor) {
			fm.SetAfter(field, value, anchor)
			return
		}
	}
	fm.Set(field, value)
}

// ScheduleMigration is the account dinah check --migrate-schedule answers
// with.
type ScheduleMigration struct {
	// From is the format the workbench declared before the run.
	From int `json:"from"`
	// Stamped is true where the anchor was written with ScheduleFormat.
	Stamped bool `json:"stamped"`
	// Preview is true where the run carried no confirmation, so it wrote
	// nothing whatever the format was.
	Preview bool `json:"preview,omitempty"`
}

// MigrateSchedule stamps ScheduleFormat on a workbench declaring a lower
// format, and does nothing to one already at or above it, so the stamp is a
// floor and never a downgrade. Without apply it classifies and writes nothing.
// The stamp is one write to the workbench anchor, through the writer the other
// migrations stamp their format with, and no card is read or written, because
// a date written onto a card below the format reads the same above it.
//
// A workbench below DesignationFormat is refused with the refusal every read
// of such a store raises, which names `dinah check --migrate-designations`.
// Stamping it would leave it declaring a format that says its designations
// were converted when no card was read to find out.
func (b *Bench) MigrateSchedule(apply bool) (*ScheduleMigration, error) {
	if b.Format < DesignationFormat {
		return nil, contract.Refuse(contract.StoreAwaitingMigration, b.Root)
	}
	report := &ScheduleMigration{From: b.Format, Preview: !apply}
	if !apply || b.Format >= ScheduleFormat {
		return report, nil
	}
	if err := b.stampFormat(ScheduleFormat); err != nil {
		return nil, err
	}
	report.Stamped = true
	return report, nil
}
