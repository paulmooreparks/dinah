package bench

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"dinah/internal/contract"
)

// fixtureCardDir is the one card newFixture writes.
func fixtureCardDir(root string) string {
	return filepath.Join(root, CardsDir, "c00000000001")
}

// plantRecord writes a lock record by hand, as a crash or a hand edit leaves
// one: a file nobody holds open and nobody holds an operating-system lock on.
func plantRecord(t *testing.T, path string, record LockRecord) {
	t.Helper()
	line, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	write(t, path, string(line)+"\n")
}

// endedChild starts an idle helper, ends it, and answers the process
// identifier and the start it ran under, which name a process that is proven
// gone.
func endedChild(t *testing.T) (int, string) {
	t.Helper()
	child := startLockChild(t, helperIdle, "ghost")
	start := child.expect(t, "idle")
	pid := child.pid()
	child.end(t)
	return pid, start
}

// otherPlatform answers a platform tag that is not this one.
func otherPlatform() string {
	if runtime.GOOS == "windows" {
		return "linux"
	}
	return "windows"
}

// TestTheVerdictOnALockFollowsItsHolder asserts that a lock a child process
// took through Acquire and still holds is judged live, and that once the
// child is ended through its own handle and waited on the same lock is judged
// dead, naming the child's record.
func TestTheVerdictOnALockFollowsItsHolder(t *testing.T) {
	root := newFixture(t)
	dir := fixtureCardDir(root)
	path := filepath.Join(dir, LockName)
	child := startLockChild(t, helperHold, "brin", dir)
	child.expect(t, "held")
	record, verdict := JudgeLock(path)
	if verdict != VerdictLive {
		t.Fatalf("a lock its running holder keeps judged %s, wanted live", verdict)
	}
	if record.Actor != "brin" || record.PID != child.pid() {
		t.Errorf("the live verdict named %+v, wanted brin at pid %d", record, child.pid())
	}
	child.end(t)
	dead := judgedDead(t, path)
	if dead.Actor != "brin" || dead.PID != child.pid() {
		t.Errorf("the dead verdict named %+v, wanted brin at pid %d", dead, child.pid())
	}
}

// TestNoPlantedRecordIsProvenDead asserts that each record the verdict cannot
// use, or that names a process table this one cannot read, is judged unknown
// with the operating-system lock free, and never dead.
func TestNoPlantedRecordIsProvenDead(t *testing.T) {
	root := newFixture(t)
	path := filepath.Join(fixtureCardDir(root), LockName)
	host, _ := selfIdentity()
	gonePID, goneStart := endedChild(t)
	running := startLockChild(t, helperIdle, "ghost")
	runningStart := running.expect(t, "idle")
	dead := LockRecord{Actor: "brin", PID: gonePID, TS: "2026-09-28T00:00:00Z", Host: host, Start: goneStart, OSLock: true}
	withoutOSLock := dead
	withoutOSLock.OSLock = false
	otherTag := dead
	otherTag.Start = otherPlatform() + ":1"
	otherHost := dead
	otherHost.Host = "elsewhere.invalid"
	alive := dead
	alive.PID = running.pid()
	alive.Start = runningStart
	cases := []struct {
		name string
		text string
	}{
		{"os_lock false", lockLine(t, withoutOSLock)},
		{"os_lock absent", strings.Replace(lockLine(t, withoutOSLock), `,"os_lock":false`, "", 1)},
		{"an empty record", ""},
		{"an unparseable record", "not a record\n"},
		{"another platform's start", lockLine(t, otherTag)},
		{"another host", lockLine(t, otherHost)},
		{"a running process that holds no operating-system lock", lockLine(t, alive)},
	}
	for _, linuxCase := range linuxOnlyUnknowns(t, dead) {
		cases = append(cases, struct {
			name string
			text string
		}{linuxCase.name, lockLine(t, linuxCase.record)})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			write(t, path, c.text)
			if _, verdict := JudgeLock(path); verdict != VerdictUnknown {
				t.Errorf("judged %s, wanted unknown", verdict)
			}
		})
	}
	write(t, path, lockLine(t, dead))
	if _, verdict := JudgeLock(path); verdict != VerdictDead {
		t.Errorf("the accepting case, a record whose process is gone, judged %s, wanted dead", verdict)
	}
}

// lockLine renders a record as a lock file's content.
func lockLine(t *testing.T, record LockRecord) string {
	t.Helper()
	line, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(line) + "\n"
}

// TestARecordNamingThisProcessFollowsTheRegistry asserts that a lock naming
// this process is judged live while another acquisition in this process owns
// its registry entry, and dead once none does.
func TestARecordNamingThisProcessFollowsTheRegistry(t *testing.T) {
	root := newFixture(t)
	dir := fixtureCardDir(root)
	path := filepath.Join(dir, LockName)
	held, err := Acquire(dir, "alka", "2026-09-28T00:00:00Z")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if _, verdict := JudgeLock(path); verdict != VerdictLive {
		t.Errorf("a lock this process holds judged %s, wanted live", verdict)
	}
	record := held.record
	held.abandon()
	if _, verdict := JudgeLock(path); verdict != VerdictDead {
		t.Errorf("this process's own orphan, %+v, judged %s, wanted dead", record, verdict)
	}
}

// TestEveryRecordCarriesItsIdentity asserts that a lock acquire writes
// carries this machine's host name, a start beginning with the platform tag
// and a colon, and os_lock true, and that a sibling carries the host and the
// start and no os_lock.
func TestEveryRecordCarriesItsIdentity(t *testing.T) {
	root := newFixture(t)
	dir := fixtureCardDir(root)
	lock, err := Acquire(dir, "alka", "2026-09-28T00:00:00Z")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	record, _ := ReadLockRecord(filepath.Join(dir, LockName))
	lock.Release()
	host, err := os.Hostname()
	if err != nil {
		t.Fatalf("host name: %v", err)
	}
	if record.Host != host || !strings.HasPrefix(record.Start, runtime.GOOS+":") || !record.OSLock {
		t.Errorf("an acquired lock records %+v, wanted host %s, a %s: start and os_lock", record, host, runtime.GOOS)
	}
	sibling, siblingRecord, err := AcquireSibling(dir, "alka", "2026-09-28T00:00:00Z", OpArchive, "")
	if err != nil {
		t.Fatalf("acquire the sibling: %v", err)
	}
	stored, _ := ReadLockRecord(SiblingPath(dir))
	sibling.Release()
	if stored != siblingRecord {
		t.Errorf("the sibling stored %+v and answered %+v", stored, siblingRecord)
	}
	if stored.Host != host || !strings.HasPrefix(stored.Start, runtime.GOOS+":") || stored.OSLock {
		t.Errorf("a sibling records %+v, wanted host, start and no os_lock", stored)
	}
}

// TestCheckReportsAndClearsDeadLocksInBothHalves asserts that check reports
// every entity lock whose holder is dead or unproven, in the live half and in
// the archive, that a live lock is not reported, and that the finish clears
// the dead ones, records each reclaim in the journal section 5.7 of the
// specification assigns it, and leaves the unknown locks and the siblings
// standing.
func TestCheckReportsAndClearsDeadLocksInBothHalves(t *testing.T) {
	root := newFixture(t)
	card := fixtureCardDir(root)
	writeWorkstream(t, root, "f00000000001", "title: Stream\nslug: stream\nstatus: active\nordinal: 1\n")
	workstream := filepath.Join(root, WorkstreamsDir, "f00000000001")
	column := filepath.Join(root, ColumnsDir, "b00000000001")
	archived := filepath.Join(root, ArchiveDir, CardsDir, "c00000000002")
	write(t, filepath.Join(archived, CardAnchor), cleanCard)
	write(t, filepath.Join(archived, JournalName), cleanJournal)
	child := startLockChild(t, helperHold, "brin", card, workstream, root, column, archived)
	child.expect(t, "held")
	child.end(t)
	for _, dir := range []string{card, workstream, root, column, archived} {
		judgedDead(t, filepath.Join(dir, LockName))
	}
	host, _ := selfIdentity()
	elsewhere := filepath.Join(root, CardsDir, "c00000000003", LockName)
	plantRecord(t, elsewhere, LockRecord{Actor: "cato", PID: 7, TS: "2026-09-28T00:00:00Z", Host: "elsewhere.invalid", Start: runtime.GOOS + ":1", OSLock: true})
	emptied := filepath.Join(root, CardsDir, "c00000000004", LockName)
	write(t, emptied, "")
	liveDir := filepath.Join(root, CardsDir, "c00000000005")
	write(t, filepath.Join(liveDir, CardAnchor), cleanCard)
	live, err := Acquire(liveDir, "alka", "2026-09-28T00:00:00Z")
	if err != nil {
		t.Fatalf("hold a live lock: %v", err)
	}
	defer live.Release()
	// A sibling whose entity stands at neither path is one the interrupted
	// act's own finish reports and leaves, so it stands for every sibling the
	// stale-lock repair must not touch.
	sibling := filepath.Join(root, CardsDir, "c0000000000f"+SiblingSuffix)
	archivedTarget := filepath.Join(root, ArchiveDir, CardsDir, "c0000000000f")
	plantRecord(t, sibling, LockRecord{Actor: "brin", PID: child.pid(), TS: "2026-09-28T00:00:00Z", Host: host, Op: OpArchive, To: archivedTarget})

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	stale := staleLocksIn(t, opened)
	dead, unknown := 0, 0
	for _, finding := range stale {
		switch {
		case strings.HasSuffix(finding.Detail, " dead"):
			dead++
			if finding.Detail != "brin dead" || finding.Next != staleLockDeadNext {
				t.Errorf("a dead lock reported %+v", finding)
			}
		case strings.HasSuffix(finding.Detail, "unknown"):
			unknown++
			if finding.Next != staleLockUnknownNext {
				t.Errorf("an unknown lock carries next %q", finding.Next)
			}
		}
		if finding.Path == filepath.Join(liveDir, LockName) {
			t.Errorf("a live lock was reported: %+v", finding)
		}
	}
	if len(stale) != 7 || dead != 5 || unknown != 2 {
		t.Fatalf("check reported %d stale locks, %d dead and %d unknown, wanted 7, 5 and 2: %+v", len(stale), dead, unknown, stale)
	}
	if detail := findingAt(stale, emptied).Detail; detail != "unknown" {
		t.Errorf("the empty record reported detail %q, wanted unknown with no holder", detail)
	}
	if detail := findingAt(stale, elsewhere).Detail; detail != "cato unknown" {
		t.Errorf("the lock naming another host reported detail %q, wanted cato unknown", detail)
	}

	_, cleared, err := opened.FinishInterrupted("alka", "2026-09-28T01:00:00Z")
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if len(cleared) != 5 {
		t.Errorf("the finish cleared %d locks, wanted 5: %+v", len(cleared), cleared)
	}
	for _, dir := range []string{card, workstream, root, column, archived} {
		if Exists(filepath.Join(dir, LockName)) {
			t.Errorf("the dead lock in %s still stands", dir)
		}
	}
	for _, standing := range []string{elsewhere, emptied, sibling} {
		if !Exists(standing) {
			t.Errorf("%s was removed, and only a person may clear it", standing)
		}
	}
	reclaims := map[string]int{
		filepath.Join(card, JournalName):       1,
		filepath.Join(workstream, JournalName): 1,
		filepath.Join(root, JournalName):       2,
		filepath.Join(archived, JournalName):   1,
	}
	for journal, want := range reclaims {
		lines := reclaimsIn(t, journal)
		if len(lines) != want {
			t.Errorf("%s carries %d lock_reclaimed lines, wanted %d", journal, len(lines), want)
		}
		for _, line := range lines {
			var carried LockRecord
			if err := json.Unmarshal([]byte(line.Note), &carried); err != nil || carried.Actor != "brin" || carried.PID != child.pid() {
				t.Errorf("a lock_reclaimed line carries %q, wanted the dead record", line.Note)
			}
		}
	}
	reopened, err := Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if again := staleLocksIn(t, reopened); len(again) != 2 {
		t.Errorf("a second check reported %d stale locks, wanted the two unknown: %+v", len(again), again)
	}
}

// staleLocksIn runs check and answers its stale-lock findings.
func staleLocksIn(t *testing.T, opened *Bench) []Finding {
	t.Helper()
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	var stale []Finding
	for _, finding := range findings {
		if finding.Key == FindingStaleLock {
			stale = append(stale, finding)
		}
	}
	return stale
}

// findingAt answers the finding reported against path.
func findingAt(findings []Finding, path string) Finding {
	for _, finding := range findings {
		if finding.Path == path {
			return finding
		}
	}
	return Finding{}
}

// reclaimsIn answers the lock_reclaimed lines of one journal.
func reclaimsIn(t *testing.T, journal string) []Event {
	t.Helper()
	events, _, err := ReadJournal(journal)
	if err != nil {
		t.Fatalf("read %s: %v", journal, err)
	}
	var found []Event
	for _, ev := range events {
		if ev.Event == contract.EventLockReclaimed {
			found = append(found, ev)
		}
	}
	return found
}

// TestTwoGoroutinesAtTheInterposeLeaveOneHolder asserts that when one
// goroutine is held at ReclaimInterpose on a dead lock, a second goroutine of
// the same process is refused dinah.locked rather than reclaiming too, and
// that the first then ends holding the lock with one lock_reclaimed line.
func TestTwoGoroutinesAtTheInterposeLeaveOneHolder(t *testing.T) {
	root := newFixture(t)
	dir := fixtureCardDir(root)
	child := startLockChild(t, helperHold, "brin", dir)
	child.expect(t, "held")
	child.end(t)
	judgedDead(t, filepath.Join(dir, LockName))

	arrived := make(chan struct{})
	carryOn := make(chan struct{})
	ReclaimInterpose = func() {
		close(arrived)
		<-carryOn
	}
	t.Cleanup(func() { ReclaimInterpose = nil })
	type outcome struct {
		lock *Lock
		err  error
	}
	first := make(chan outcome, 1)
	go func() {
		lock, err := Acquire(dir, "alka", "2026-09-28T00:00:00Z")
		first <- outcome{lock, err}
	}()
	<-arrived
	_, err := Acquire(dir, "cato", "2026-09-28T00:00:00Z")
	if !isLocked(err) {
		t.Errorf("a second goroutine reaching the same dead lock answered %v, wanted dinah.locked", err)
	}
	close(carryOn)
	won := <-first
	if won.err != nil {
		t.Fatalf("the goroutine held at the interpose answered %v", won.err)
	}
	record, _ := ReadLockRecord(filepath.Join(dir, LockName))
	won.lock.Release()
	if record.Actor != "alka" {
		t.Errorf("the lock names %q after the reclaim, wanted alka", record.Actor)
	}
	if lines := reclaimsIn(t, filepath.Join(dir, JournalName)); len(lines) != 1 {
		t.Errorf("the journal carries %d lock_reclaimed lines, wanted one", len(lines))
	}
}

// TestTwoProcessesAtTheInterposeLeaveOneHolder asserts that when one process
// is held at ReclaimInterpose on a dead lock, a second process reaching the
// same lock is refused, because the first holds the operating-system lock,
// and that the first then ends holding the lock with one lock_reclaimed line.
func TestTwoProcessesAtTheInterposeLeaveOneHolder(t *testing.T) {
	root := newFixture(t)
	dir := fixtureCardDir(root)
	dead := startLockChild(t, helperHold, "brin", dir)
	dead.expect(t, "held")
	dead.end(t)
	judgedDead(t, filepath.Join(dir, LockName))

	first := startLockChild(t, helperReclaim, "alka", dir)
	first.expect(t, "interposed")
	second := startLockChild(t, helperReclaim, "cato", dir)
	if answer := second.line(t); !strings.HasPrefix(answer, "refused") {
		t.Errorf("the second process answered %q, wanted a refusal", answer)
	}
	first.proceed(t)
	first.expect(t, "held")
	record, _ := ReadLockRecord(filepath.Join(dir, LockName))
	if record.Actor != "alka" || record.PID != first.pid() {
		t.Errorf("the lock names %+v, wanted the first process", record)
	}
	if lines := reclaimsIn(t, filepath.Join(dir, JournalName)); len(lines) != 1 {
		t.Errorf("the journal carries %d lock_reclaimed lines, wanted one", len(lines))
	}
}

// TestTwoGoroutinesRacingAFreeLockLeaveOneHolder asserts that two goroutines
// acquiring one free lock end with one holder and one refusal, and that a
// third acquisition in the same process is refused rather than granted by a
// reclaim.
func TestTwoGoroutinesRacingAFreeLockLeaveOneHolder(t *testing.T) {
	root := newFixture(t)
	dir := fixtureCardDir(root)
	var wg sync.WaitGroup
	start := make(chan struct{})
	locks := make([]*Lock, 2)
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			locks[i], errs[i] = Acquire(dir, "alka", "2026-09-28T00:00:00Z")
		}()
	}
	close(start)
	wg.Wait()
	won := 0
	var holder *Lock
	for i := range 2 {
		if errs[i] == nil {
			won++
			holder = locks[i]
			continue
		}
		if !isLocked(errs[i]) {
			t.Errorf("the losing acquisition answered %v, wanted dinah.locked", errs[i])
		}
	}
	if won != 1 {
		t.Fatalf("%d acquisitions won one free lock", won)
	}
	defer holder.Release()
	if _, err := Acquire(dir, "cato", "2026-09-28T00:00:00Z"); !isLocked(err) {
		t.Errorf("a third acquisition answered %v, wanted dinah.locked", err)
	}
	if lines := reclaimsIn(t, filepath.Join(dir, JournalName)); len(lines) != 0 {
		t.Errorf("a race over a free lock recorded %d reclaims", len(lines))
	}
}

// TestADeadProcessesSiblingIsNeverReclaimed asserts that a write meeting a
// sibling lock a dead process left is refused and leaves the sibling as it
// was, because a sibling is cleared by dinah check --finish alone.
func TestADeadProcessesSiblingIsNeverReclaimed(t *testing.T) {
	root := newFixture(t)
	dir := fixtureCardDir(root)
	host, _ := selfIdentity()
	pid, start := endedChild(t)
	siblingPath := SiblingPath(dir)
	plantRecord(t, siblingPath, LockRecord{Actor: "brin", PID: pid, TS: "2026-09-28T00:00:00Z", Host: host, Start: start, Op: OpArchive, OSLock: true})
	before, err := os.ReadFile(siblingPath)
	if err != nil {
		t.Fatalf("read the sibling: %v", err)
	}
	if _, err := Acquire(dir, "alka", "2026-09-28T00:00:00Z"); !isLocked(err) {
		t.Errorf("a write beside a dead process's sibling answered %v, wanted dinah.locked", err)
	}
	after, err := os.ReadFile(siblingPath)
	if err != nil || string(after) != string(before) {
		t.Errorf("the sibling changed: %q to %q (%v)", before, after, err)
	}
}

// isLocked reports whether err is the dinah.locked refusal.
func isLocked(err error) bool {
	var refusal *contract.Refusal
	return errors.As(err, &refusal) && refusal.Name == contract.Locked
}
