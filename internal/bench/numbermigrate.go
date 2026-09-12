package bench

import (
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"dinah/internal/contract"
)

// numberMigrant is one card the migration has read, carrying everything the
// run needs from it and nothing the guard worries about. The free reader
// answers a card whose Number is zero, and a record with no *Card field is
// how that value stays inside this file rather than reaching a caller.
type numberMigrant struct {
	// dir is the card's directory, which is where the journal write and the
	// strip below it happen.
	dir string
	// id is the card's 12-hex identifier.
	id string
	// number is the number the anchor's number key carries, zero when the key
	// is absent or holds a value the registry's grammar refuses.
	number int
	// keyed reports whether the anchor carries a number key at all, which is
	// what puts the card in the strip set.
	keyed bool
	// created is the timestamp of the card's own created event, zero when the
	// journal carries none that reads.
	created time.Time
}

// readMigrant reads one card for the migration through the free reader. A
// workbench waiting for this migration has no registry to stamp a number
// from, so the stamping readers would answer nothing the run can use, and the
// one reference this file's guard budget carries is spent here. The card the
// reader answers never leaves the function: its directory and its frontmatter
// are read into locals, and the record is built from those alone.
func readMigrant(root, id string) (numberMigrant, error) {
	card, err := LoadCard(root, id)
	if err != nil {
		return numberMigrant{}, err
	}
	dir := card.Dir
	fm := card.FM
	migrant := numberMigrant{dir: dir, id: id, keyed: fm.Has("number")}
	if migrant.keyed {
		if numbered, err := strconv.Atoi(fm.Value("number")); err == nil && numbered > 0 {
			migrant.number = numbered
		}
	}
	migrant.created = createdStamp(dir)
	return migrant, nil
}

// createdStamp answers the timestamp of the card's own created event, which is
// the first half of the tie-break the migration orders colliding cards by. A
// card with no journal, or a journal whose created event carries no timestamp
// that parses, answers the zero time, and the comparator sorts a zero after
// every card that has one rather than letting it sort first the way a bare
// time comparison would.
func createdStamp(dir string) time.Time {
	events, _, err := ReadJournal(filepath.Join(dir, JournalName))
	if err != nil {
		return time.Time{}
	}
	for _, ev := range events {
		if ev.Event != contract.EventCreated {
			continue
		}
		return ParseStamp(ev.TS)
	}
	return time.Time{}
}

// migrantBefore is the tie-break the format fixes, and it is a total order
// over any group of cards: the created timestamp earliest first, then the
// identifier ascending, and a card with no readable created event after every
// card that has one. Two clones migrating one workbench independently walk the
// same order and mint the same registry, which is what makes the renumbering
// deterministic rather than merely defined.
func migrantBefore(a, b numberMigrant) bool {
	aStamped, bStamped := !a.created.IsZero(), !b.created.IsZero()
	if aStamped && bStamped {
		if !a.created.Equal(b.created) {
			return a.created.Before(b.created)
		}
		return a.id < b.id
	}
	if aStamped != bStamped {
		return aStamped
	}
	return a.id < b.id
}

// renumberedCard is one card the run moved to a number it did not arrive
// holding, with the number it did arrive holding, which is what the event and
// the finding composed from it say.
type renumberedCard struct {
	id   string
	dir  string
	from int
	to   int
}

// MigrateNumbers builds the card-number registry from the numbers cards still
// carry in their anchors, strips the number key from every anchor, and stamps
// format 3 on the workbench anchor. It answers the count of lines it wrote,
// the identifiers of the cards it renumbered, a finding per renumbered card,
// and the error that ended it when one did. A non-nil error travels beside
// whatever the run had already written rather than in place of it.
//
// The workbench lock is held for the whole run, on the discipline add and
// delete already keep, because the registry is written beneath it and a
// filing that allocated a number mid-migration would mint a line the
// composition below has already decided the fate of. The registry is re-read
// from the file rather than from the bench's own copy, because a workbench
// below format 3 synthesizes that copy from the very anchors this run is
// about to strip.
//
// Every card in both halves is read, live first and archived second, and a
// card whose anchor will not load ends the run rather than being stepped over,
// because a number nobody read is a number the registry may hand out again and
// this repair is the one place where that gap is unrecoverable. A card's
// claimed number is the number of the first well-formed line naming it, since
// a workbench mid-repair can disagree with its own anchors and the line is
// the record allocation actually made, and otherwise the number its anchor
// carries. Cards claiming one number are ordered by the tie-break, the first
// keeps the number, and every other card in the group is renumbered above the
// high-water mark, which is the greatest number any line or any anchor
// carries. Cards claiming no number take the next numbers in walk order.
//
// The composed file's well-formed lines ascend by number, malformed lines keep
// their bytes at the end in the order they were stored, and a composition
// equal to the file as it stands writes nothing and renumbers nobody.
//
// The writes are ordered so a crash leaves a state a re-run of this same
// repair finishes: the registry first, because the anchors' keys are the only
// other copy of every number; then a renumbered event on each renumbered
// card's journal; then the strip of each keyed anchor under the card's own
// lock; and the format stamp last, because a workbench still declaring
// format 2 re-runs the migration and one declaring 3 has nothing left to do.
// The per-card locks are all taken before the first write, so a card a claim
// holds costs the run its refusal and writes nothing.
func (b *Bench) MigrateNumbers(actor, now string) (int, []string, []Finding, error) {
	lock, err := Acquire(b.Root, actor, now)
	if err != nil {
		return 0, nil, nil, err
	}
	defer lock.Release()
	registry := LoadNumberRegistry(filepath.Join(b.Root, CardNumbersName))
	var migrants []numberMigrant
	for _, root := range []string{b.CardsRoot(), b.ArchivedCardsRoot()} {
		ids, err := ListIDs(root)
		if err != nil {
			return 0, nil, nil, err
		}
		for _, id := range ids {
			migrant, err := readMigrant(root, id)
			if err != nil {
				return 0, nil, nil, err
			}
			migrants = append(migrants, migrant)
		}
	}
	// The high-water mark covers every line and every anchor, so a renumbered
	// card can never take a number anything still holds.
	high := registry.Highest
	known := make(map[string]int, len(migrants))
	for at, migrant := range migrants {
		known[migrant.id] = at
		if migrant.number > high {
			high = migrant.number
		}
	}
	// slot names, for every card a well-formed line claims, the index of the
	// first line claiming it in file order. Where the anchor and the line
	// disagree the line wins, because it is the record allocation made.
	slot := make(map[string]int)
	for at, line := range registry.Lines {
		if line.Number == 0 || line.ID == "-" {
			continue
		}
		if _, is := known[line.ID]; !is {
			continue
		}
		if _, taken := slot[line.ID]; taken {
			continue
		}
		slot[line.ID] = at
	}
	claimed := make([]int, len(migrants))
	for at, migrant := range migrants {
		claimed[at] = migrant.number
		if lineAt, is := slot[migrant.id]; is {
			claimed[at] = registry.Lines[lineAt].Number
		}
	}
	// Colliding claims are grouped and processed in ascending number, and the
	// loser of each group is renumbered in tie-break order.
	groups := make(map[int][]int)
	var claimedNumbers []int
	for at := range migrants {
		if claimed[at] == 0 {
			continue
		}
		if _, grouped := groups[claimed[at]]; !grouped {
			claimedNumbers = append(claimedNumbers, claimed[at])
		}
		groups[claimed[at]] = append(groups[claimed[at]], at)
	}
	sort.Ints(claimedNumbers)
	final := make([]int, len(migrants))
	copy(final, claimed)
	next := high
	var renumbered []renumberedCard
	for _, number := range claimedNumbers {
		group := groups[number]
		if len(group) == 1 {
			continue
		}
		sort.SliceStable(group, func(i, j int) bool {
			return migrantBefore(migrants[group[i]], migrants[group[j]])
		})
		for _, at := range group[1:] {
			next++
			final[at] = next
			renumbered = append(renumbered, renumberedCard{
				id:   migrants[at].id,
				dir:  migrants[at].dir,
				from: number,
				to:   next,
			})
		}
	}
	// Cards claiming no number take the next numbers in walk order, minting
	// no event and no finding, because a card nobody numbered was never called
	// anything and has no reference to break.
	for at := range migrants {
		if claimed[at] != 0 {
			continue
		}
		next++
		final[at] = next
	}
	// The composition keeps every well-formed line at the shape the run
	// decided, adds a line for every card no line claimed, sorts the
	// well-formed lines ascending by number, and carries the malformed lines
	// verbatim at the end.
	var composed []NumberLine
	var malformed []string
	slotted := make(map[int]bool)
	for _, lineAt := range slot {
		slotted[lineAt] = true
	}
	for at, line := range registry.Lines {
		if line.Number == 0 {
			malformed = append(malformed, line.Raw)
			continue
		}
		if line.ID != "-" && slotted[at] {
			number := final[known[line.ID]]
			composed = append(composed, NumberLine{
				Number: number,
				ID:     line.ID,
				Raw:    strconv.Itoa(number) + " " + line.ID,
			})
			continue
		}
		composed = append(composed, line)
	}
	for at, migrant := range migrants {
		if _, is := slot[migrant.id]; is {
			continue
		}
		composed = append(composed, NumberLine{
			Number: final[at],
			ID:     migrant.id,
			Raw:    strconv.Itoa(final[at]) + " " + migrant.id,
		})
	}
	sort.SliceStable(composed, func(i, j int) bool {
		return composed[i].Number < composed[j].Number
	})
	for _, raw := range malformed {
		composed = append(composed, NumberLine{Raw: raw})
	}
	// A composition equal to the file as it stands writes nothing, which is
	// the second run's answer: the registry already covers every card and no
	// number moved.
	unchanged := len(composed) == len(registry.Lines)
	for at := 0; unchanged && at < len(composed); at++ {
		unchanged = composed[at].Raw == registry.Lines[at].Raw
	}
	// Every card whose anchor will be stripped and every card renumbered is
	// locked before the first write, so a refusal costs the workbench nothing.
	var locks []*Lock
	defer func() {
		for _, held := range locks {
			held.Release()
		}
	}()
	locked := make(map[string]bool)
	take := func(dir string) error {
		if locked[dir] {
			return nil
		}
		held, err := Acquire(dir, actor, now)
		if err != nil {
			return err
		}
		locked[dir] = true
		locks = append(locks, held)
		return nil
	}
	for _, migrant := range migrants {
		if !migrant.keyed {
			continue
		}
		if err := take(migrant.dir); err != nil {
			return 0, nil, nil, err
		}
	}
	for _, moved := range renumbered {
		if err := take(moved.dir); err != nil {
			return 0, nil, nil, err
		}
	}
	written := 0
	if !unchanged {
		lines := make([]string, len(composed))
		for at, line := range composed {
			lines[at] = line.Raw
		}
		if err := WriteNumberLines(filepath.Join(b.Root, CardNumbersName), lines); err != nil {
			return 0, nil, nil, err
		}
		written = len(lines)
	}
	var ids []string
	var findings []Finding
	for _, moved := range renumbered {
		ev := Event{
			TS:    now,
			Event: contract.EventRenumbered,
			Actor: actor,
			From:  strconv.Itoa(moved.from),
			To:    strconv.Itoa(moved.to),
		}
		if err := AppendEvent(filepath.Join(moved.dir, JournalName), ev); err != nil {
			return written, ids, findings, err
		}
		ids = append(ids, moved.id)
		findings = append(findings, Finding{
			Path:   filepath.Join(moved.dir, CardAnchor),
			Key:    FindingCardNumberRenumbered,
			Detail: moved.id,
		})
	}
	for _, migrant := range migrants {
		if !migrant.keyed {
			continue
		}
		anchor := filepath.Join(migrant.dir, CardAnchor)
		text, err := ReadText(anchor)
		if err != nil {
			return written, ids, findings, err
		}
		fm, body := ParseAnchor(text)
		// The anchor is read again under the lock rather than carried from
		// the walk, because the walk stood outside it and the key could have
		// left since. A card that no longer carries the key is already
		// stripped and costs nothing.
		if !fm.Has("number") {
			continue
		}
		fm.Delete("number")
		if err := WriteText(anchor, fm.Render(body)); err != nil {
			return written, ids, findings, err
		}
	}
	if b.FM.Value("format") != strconv.Itoa(RegistryFormat) {
		b.FM.Set("format", strconv.Itoa(RegistryFormat))
		if err := b.Save(); err != nil {
			return written, ids, findings, err
		}
	}
	b.Format = RegistryFormat
	b.ReloadNumbers()
	return written, ids, findings, nil
}

// RenumberCards repairs check.card-number-duplicate and nothing else. For
// each number more than one well-formed line claims, the line appearing first
// in the file keeps it, and every later line claiming the same number is
// rewritten where it stands with the next number above the high-water mark,
// taking the later lines in file order. Line order is the only record of who
// claimed first and that record is the evidence the repair depends on, so no
// line moves and the numbers in the file need not ascend afterwards.
//
// A later claimant whose identifier names no card directory is stranded and
// has no journal, so its line is renumbered and nothing is said: the event and
// the finding are the card's, and a line naming nothing names no card. Every
// other card named by a renumbered line gets a renumbered event on its
// journal and a finding in the run's report, and the per-card locks for all of
// them are taken before the first write, on the migration's terms.
//
// The workbench lock is held for the whole run, because the registry file is
// rewritten beneath it and a filing between the read and the write would mint
// a line the composition has already decided the fate of. The registry is
// re-read from the file for the same reason the migration re-reads it, and it
// is reloaded once the write has landed so the bench and the file agree.
func (b *Bench) RenumberCards(actor, now string) ([]string, []Finding, error) {
	lock, err := Acquire(b.Root, actor, now)
	if err != nil {
		return nil, nil, err
	}
	defer lock.Release()
	registry := LoadNumberRegistry(filepath.Join(b.Root, CardNumbersName))
	// A later claimant is any well-formed line past the first claiming its
	// number, met in file order, and each takes the next number above the
	// high-water mark.
	next := registry.Highest
	type movedLine struct {
		at   int
		id   string
		dir  string
		card bool
		from int
		to   int
	}
	var moved []movedLine
	claimants := make(map[int]int)
	for at, line := range registry.Lines {
		if line.Number == 0 || line.ID == "-" {
			continue
		}
		claimants[line.Number]++
		if claimants[line.Number] < 2 {
			continue
		}
		next++
		entry := movedLine{at: at, id: line.ID, from: line.Number, to: next}
		for _, root := range []string{b.CardsRoot(), b.ArchivedCardsRoot()} {
			if Exists(filepath.Join(root, line.ID)) {
				entry.dir = filepath.Join(root, line.ID)
				entry.card = true
				break
			}
		}
		moved = append(moved, entry)
	}
	if len(moved) == 0 {
		return nil, nil, nil
	}
	var locks []*Lock
	defer func() {
		for _, held := range locks {
			held.Release()
		}
	}()
	for _, entry := range moved {
		if !entry.card {
			continue
		}
		held, err := Acquire(entry.dir, actor, now)
		if err != nil {
			return nil, nil, err
		}
		locks = append(locks, held)
	}
	lines := make([]string, len(registry.Lines))
	for at, line := range registry.Lines {
		lines[at] = line.Raw
	}
	for _, entry := range moved {
		lines[entry.at] = strconv.Itoa(entry.to) + " " + entry.id
	}
	if err := WriteNumberLines(filepath.Join(b.Root, CardNumbersName), lines); err != nil {
		return nil, nil, err
	}
	var ids []string
	var findings []Finding
	for _, entry := range moved {
		if !entry.card {
			continue
		}
		ev := Event{
			TS:    now,
			Event: contract.EventRenumbered,
			Actor: actor,
			From:  strconv.Itoa(entry.from),
			To:    strconv.Itoa(entry.to),
		}
		if err := AppendEvent(filepath.Join(entry.dir, JournalName), ev); err != nil {
			return ids, findings, err
		}
		ids = append(ids, entry.id)
		findings = append(findings, Finding{
			Path:   filepath.Join(entry.dir, CardAnchor),
			Key:    FindingCardNumberRenumbered,
			Detail: entry.id,
		})
	}
	b.ReloadNumbers()
	return ids, findings, nil
}
