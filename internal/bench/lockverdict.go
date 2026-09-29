package bench

import (
	"errors"
	"io/fs"
	"os"
	"runtime"
	"strings"

	"dinah/internal/contract"
	"dinah/internal/durable"
)

// Verdict is what JudgeLock concludes about the holder of a lock file.
type Verdict int

// The three verdicts. Only VerdictDead ever permits a reclaim.
const (
	// VerdictLive means a holder is present: another handle holds the
	// operating-system lock, or another acquisition in this process owns
	// the lock's registry entry.
	VerdictLive Verdict = iota
	// VerdictDead means the lock's holder is proven to have ended.
	VerdictDead
	// VerdictUnknown means nothing proves the holder alive or dead: the
	// record does not parse, was written without the operating-system lock,
	// names another platform, host, boot or PID namespace, or the process
	// table and the operating-system lock disagree.
	VerdictUnknown
	// verdictContinue is rule 5's answer when the record names this
	// process's own process table, so rule 6 decides. It never leaves this
	// package.
	verdictContinue
)

// String names a verdict the way dinah check prints it.
func (v Verdict) String() string {
	switch v {
	case VerdictLive:
		return "live"
	case VerdictDead:
		return "dead"
	}
	return "unknown"
}

// ReclaimInterpose, nil in production, is called between a VerdictDead and
// the reclaim's first step, so a test can hold two reclaimers there together.
var ReclaimInterpose func()

// judgeOpened, nil in production, is called once the judge has opened the lock
// file and before it tries the operating-system lock, so a test can release
// the lock in that window.
var judgeOpened func()

// processGone is rule 6, the platform's own recordedProcessGone. A test that
// has ended a process of its own replaces it, so that its verdicts do not
// depend on whether the operating system has since given that PID to another
// process.
var processGone = recordedProcessGone

// judgement is what the verdict found: the record the file carries, the
// verdict, whether the file was gone before it could be opened, and, for a
// VerdictDead, the handle the judge holds the operating-system lock on.
type judgement struct {
	record  LockRecord
	verdict Verdict
	gone    bool
	file    *os.File
	line    string
}

// judgeOwner is the owner a judge's own registry entry names.
type judgeOwner struct{}

// JudgeLock answers who holds the lock file at path and whether that holder is
// live, dead, or cannot be proven either. It first inserts a registry entry of
// its own for the path, so no acquisition in this process can take the lock
// while it judges, and removes it before it returns. A file that is gone
// answers VerdictLive with an empty record.
func JudgeLock(path string) (LockRecord, Verdict) {
	return judgeLock(Disk{}, path)
}

// JudgeLock is the free JudgeLock reading a record it cannot hold the
// operating-system lock on through this bench's source.
func (b *Bench) JudgeLock(path string) (LockRecord, Verdict) {
	return judgeLock(b.source(), path)
}

// judgeLock is JudgeLock's body, reading through src.
func judgeLock(src Source, path string) (LockRecord, Verdict) {
	hold, owner := durable.Register(path, judgeOwner{})
	if hold == nil {
		return holderRecord(src, owner, path), VerdictLive
	}
	defer hold.Unregister()
	judged := judge(src, path)
	if judged.file != nil {
		durable.CloseLockFile(judged.file)
	}
	if judged.gone {
		return LockRecord{}, VerdictLive
	}
	return judged.record, judged.verdict
}

// holderRecord answers the record of the acquisition in this process that owns
// a lock's registry entry, or what the file records when a judge owns it.
func holderRecord(src Source, owner any, path string) LockRecord {
	if held, ok := owner.(*Lock); ok && held.record.Actor != "" {
		return held.record
	}
	record, _ := readLockRecord(src, path)
	return record
}

// judge applies the verdict's rules 2 to 6 to the lock file at path, for a
// caller that already holds the path's registry entry, which is rule 1. It
// keeps the handle and its operating-system lock only for a VerdictDead.
//
// A lock released while the judge works is gone, not unknown: a file whose
// name is no longer there once the open has failed, and a handle whose file
// the name no longer reaches once the operating-system lock is taken, both
// mean the holder let it go, and the caller tries again.
//
// The judge opens the lock file itself, because the operating-system lock it
// asks about lives on a handle, which no Source can answer for; a record it
// could not take that lock on it reads through src.
func judge(src Source, path string) judgement {
	f, err := durable.OpenLockFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return judgement{gone: true}
	}
	if err != nil {
		if _, statErr := os.Lstat(path); errors.Is(statErr, fs.ErrNotExist) {
			return judgement{gone: true}
		}
		record, _ := readLockRecord(src, path)
		return judgement{record: record, verdict: VerdictUnknown}
	}
	if judgeOpened != nil {
		judgeOpened()
	}
	taken, err := durable.TryOSLock(f)
	if err != nil || !taken {
		f.Close()
		record, _ := readLockRecord(src, path)
		verdict := VerdictLive
		if err != nil {
			verdict = VerdictUnknown
		}
		return judgement{record: record, verdict: verdict}
	}
	if named, err := durable.StillNamed(f, path); err == nil && !named {
		durable.CloseLockFile(f)
		return judgement{gone: true}
	}
	data, err := durable.ReadHeld(f)
	if err != nil {
		durable.CloseLockFile(f)
		return judgement{verdict: VerdictUnknown}
	}
	record, parsed := parseLockRecord(string(data))
	line := strings.TrimSpace(firstRecordLine(string(data)))
	verdict := verdictOn(record, parsed)
	if verdict != VerdictDead {
		durable.CloseLockFile(f)
		return judgement{record: record, verdict: verdict, line: line}
	}
	return judgement{record: record, verdict: VerdictDead, file: f, line: line}
}

// firstRecordLine answers the first line of a lock file's content.
func firstRecordLine(text string) string {
	lines := SplitLines(text)
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

// verdictOn applies rules 3 to 6 to a record whose file the judge holds the
// operating-system lock on.
func verdictOn(record LockRecord, parsed bool) Verdict {
	host, start := selfIdentity()
	if parsed && record.Start == start && record.PID == os.Getpid() {
		return VerdictDead
	}
	if !parsed || !record.OSLock || record.PID <= 0 {
		return VerdictUnknown
	}
	tag, _, _ := strings.Cut(record.Start, ":")
	if !strings.Contains(record.Start, ":") || tag != runtime.GOOS {
		return VerdictUnknown
	}
	table := sameProcessTable(record, host, start)
	if table != verdictContinue {
		return table
	}
	if processGone(record) {
		return VerdictDead
	}
	return VerdictUnknown
}

// reclaim takes over a dead lock in place, on the handle the judge holds the
// operating-system lock on, and records the reclaim in journal.
//
// It checks that the handle still names the file at the lock's path, rewrites
// the record with its own and flushes it, and checks again. A failure at
// either check closes the handle and refuses, and the bytes went to a file
// nobody reaches by name. The rewrite is not atomic: a reclaimer that dies
// between the truncation and the flush leaves an empty or partial record,
// which the verdict judges unknown and only a person clears.
func (l *Lock) reclaim(src Source, judged judgement, dir string, actor Actor, now, journal string, journalLock *Lock, tolerated *LockRecord) (*Lock, error) {
	f := judged.file
	if ReclaimInterpose != nil {
		ReclaimInterpose()
	}
	refuse := func() (*Lock, error) {
		durable.CloseLockFile(f)
		l.hold.Unregister()
		return nil, contract.Refuse(contract.Locked, lockHolder(src, l.path))
	}
	if named, err := durable.StillNamed(f, l.path); err != nil || !named {
		return refuse()
	}
	record := newRecord(actor.Name, now)
	record.OSLock = true
	if err := writeHeldRecord(f, record); err != nil {
		return refuse()
	}
	if named, err := durable.StillNamed(f, l.path); err != nil || !named {
		return refuse()
	}
	l.file = f
	l.record = record
	if err := l.refuseOnSibling(src, dir, tolerated); err != nil {
		return nil, err
	}
	ev := Event{
		TS:     now,
		Event:  contract.EventLockReclaimed,
		Actor:  NamedActor(actor.Name),
		Note:   judged.line,
		Column: columnOf(dir),
	}
	held := journalLock
	if held == nil {
		held = l
	}
	if err := AppendEvent(held, journal, ev); err != nil {
		l.Release()
		return nil, err
	}
	return l, nil
}
