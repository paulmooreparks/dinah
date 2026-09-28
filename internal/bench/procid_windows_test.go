//go:build windows

package bench

import (
	"testing"

	"golang.org/x/sys/windows"
)

// fakeTable is a process table a test controls: whether the PID is in the
// snapshot, its creation time, which access it grants, and whether it has
// ended. It records every access it was asked for.
type fakeTable struct {
	snapshotFails bool
	inSnapshot    bool
	created       uint64
	grants        uint32
	hasEnded      bool
	asked         []uint32
}

// present answers the snapshot.
func (f *fakeTable) present(uint32) (bool, bool) {
	return f.inSnapshot, !f.snapshotFails
}

// open grants the access asked for only when every right in it is granted.
func (f *fakeTable) open(_ uint32, access uint32) (processHandle, error) {
	f.asked = append(f.asked, access)
	if access&^f.grants != 0 {
		return nil, windows.ERROR_ACCESS_DENIED
	}
	return fakeProcess{f}, nil
}

// fakeProcess is a handle fakeTable answered.
type fakeProcess struct{ table *fakeTable }

func (p fakeProcess) creation() (uint64, bool) { return p.table.created, true }
func (p fakeProcess) ended() (bool, error)     { return p.table.hasEnded, nil }
func (p fakeProcess) close()                   {}

// withProcesses makes rule 6 read table for the rest of the test.
func withProcesses(t *testing.T, table processTable) {
	t.Helper()
	saved := processes
	processes = table
	t.Cleanup(func() { processes = saved })
}

// TestRuleSixAsksOnlyForTheAccessItNeeds asserts rule 6 on Windows against a
// process table the test controls, so no answer depends on which processes
// are running: an absent PID is gone and a failed snapshot is not; a PID
// whose process grants only limited query and carries another creation time
// is gone, asked for limited query alone; the same creation time is not
// proven gone when SYNCHRONIZE is refused, and is gone or not by whether the
// process has ended when it is granted; and a process that cannot be opened
// at all is not gone.
func TestRuleSixAsksOnlyForTheAccessItNeeds(t *testing.T) {
	const limited = windows.PROCESS_QUERY_LIMITED_INFORMATION
	const recorded = 1000
	record := LockRecord{PID: 4242, Start: "windows:1000"}
	cases := []struct {
		name  string
		table fakeTable
		gone  bool
		asked []uint32
	}{
		{"an absent PID", fakeTable{inSnapshot: false}, true, nil},
		{"a failed snapshot", fakeTable{snapshotFails: true}, false, nil},
		{"a process that cannot be opened", fakeTable{inSnapshot: true, created: 2000}, false, []uint32{limited}},
		{"a later process granting limited query alone", fakeTable{inSnapshot: true, created: 2000, grants: limited}, true, []uint32{limited}},
		{"the recorded process refusing SYNCHRONIZE", fakeTable{inSnapshot: true, created: recorded, grants: limited}, false, []uint32{limited, limited | windows.SYNCHRONIZE}},
		{"the recorded process, ended", fakeTable{inSnapshot: true, created: recorded, grants: limited | windows.SYNCHRONIZE, hasEnded: true}, true, []uint32{limited, limited | windows.SYNCHRONIZE}},
		{"the recorded process, running", fakeTable{inSnapshot: true, created: recorded, grants: limited | windows.SYNCHRONIZE}, false, []uint32{limited, limited | windows.SYNCHRONIZE}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			table := c.table
			withProcesses(t, &table)
			if gone := recordedProcessGone(record); gone != c.gone {
				t.Errorf("rule 6 answered gone=%v, wanted %v", gone, c.gone)
			}
			if len(table.asked) != len(c.asked) {
				t.Fatalf("rule 6 asked for the access %#x, wanted %#x", table.asked, c.asked)
			}
			for i := range c.asked {
				if table.asked[i] != c.asked[i] {
					t.Errorf("open %d asked for %#x, wanted %#x", i+1, table.asked[i], c.asked[i])
				}
			}
		})
	}
}

// TestRuleSixOnAProcessTheTestHoldsAlive asserts rule 6 against the real
// process table for the one position that cannot change under the test: a
// helper the test has started and not ended. Its own record is not gone, and
// a record naming its PID with another creation time is gone.
func TestRuleSixOnAProcessTheTestHoldsAlive(t *testing.T) {
	child := startLockChild(t, helperIdle, "ghost")
	start := child.expect(t, "idle")
	record := LockRecord{PID: child.pid(), Start: start}
	if recordedProcessGone(record) {
		t.Errorf("a running helper's own record, %+v, was judged gone", record)
	}
	record.Start = "windows:1"
	if !recordedProcessGone(record) {
		t.Errorf("a record naming the running helper's PID with another creation time was not judged gone")
	}
}

// reusedPIDsEverywhere makes rule 6 read, for the rest of the test, a table in
// which every PID has been given to a process this one cannot open, which is
// the table a busy Windows machine can present once a helper the test ended
// has freed its PID. A verdict that depends on the live table answers
// unknown against it.
func reusedPIDsEverywhere(t *testing.T) {
	t.Helper()
	withProcesses(t, &fakeTable{inSnapshot: true})
}
