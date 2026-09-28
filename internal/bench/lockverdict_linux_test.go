//go:build linux

package bench

import (
	"path/filepath"
	"strings"
	"testing"
)

// plantedCase is one planted record and the name its subtest reports under.
type plantedCase struct {
	name   string
	record LockRecord
}

// withIdentity answers a record whose Linux start carries the given fields in
// place of the dead record's own, where a field given empty keeps the dead
// record's value and the machine field is written as given.
func withIdentity(t *testing.T, dead LockRecord, machine, boot, namespace string) LockRecord {
	t.Helper()
	fields, ok := parseLinuxIdentity(dead.Start)
	if !ok {
		t.Fatalf("this process's start %q does not parse", dead.Start)
	}
	if boot != "" {
		fields.boot = boot
	}
	if namespace != "" {
		fields.namespace = namespace
	}
	fields.machine = machine
	record := dead
	record.Start = "linux:" + strings.Join([]string{fields.machine, fields.boot, fields.namespace, fields.start}, "/")
	return record
}

// linuxOnlyUnknowns answers the Linux records the verdict cannot prove dead:
// another boot of a machine whose identity differs or is missing, and a PID
// namespace other than this process's.
func linuxOnlyUnknowns(t *testing.T, dead LockRecord) []plantedCase {
	t.Helper()
	fields, ok := parseLinuxIdentity(dead.Start)
	if !ok {
		t.Fatalf("the dead record's start %q does not parse", dead.Start)
	}
	return []plantedCase{
		{"another boot of another machine", withIdentity(t, dead, "0123456789abcdef0123456789abcdef", "another-boot", "")},
		{"another boot with no machine identity", withIdentity(t, dead, "", "another-boot", "")},
		{"another PID namespace", withIdentity(t, dead, fields.machine, "", "1.2")},
	}
}

// TestAnEarlierBootOfThisMachineIsDead asserts that a Linux record from an
// earlier boot of this machine, which carries the same non-empty machine_id
// and another boot_id, is judged dead, because every process of an earlier
// boot has ended.
func TestAnEarlierBootOfThisMachineIsDead(t *testing.T) {
	_, start := selfIdentity()
	mine, ok := parseLinuxIdentity(start)
	if !ok || mine.machine == "" {
		t.Skip("this machine carries no /etc/machine-id, so an earlier boot cannot be proven its own")
	}
	root := newFixture(t)
	path := filepath.Join(fixtureCardDir(root), LockName)
	host, _ := selfIdentity()
	dead := LockRecord{Actor: "brin", PID: 1, TS: "2026-09-28T00:00:00Z", Host: host, Start: start, OSLock: true}
	earlier := withIdentity(t, dead, mine.machine, "an-earlier-boot", "")
	plantRecord(t, path, earlier)
	if _, verdict := JudgeLock(path); verdict != VerdictDead {
		t.Errorf("a record from an earlier boot of this machine judged %s, wanted dead", verdict)
	}
}
