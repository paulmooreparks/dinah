//go:build linux

package bench

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"

	"dinah/internal/durable"
)

// processIdentity is <machine_id>/<boot_id>/<pidns>/<starttime> for this
// process:
//
//   - machine_id is the first line of /etc/machine-id, which machine-id(5)
//     documents as the unique ID of the local system, empty when it cannot be
//     read;
//   - boot_id is /proc/sys/kernel/random/boot_id, which random(4) documents
//     as generated once per boot;
//   - pidns is the device and inode of /proc/self/ns/pid, and namespaces(7)
//     documents that two processes share a namespace exactly when those are
//     the same;
//   - starttime is field 22 of /proc/self/stat, which proc(5) documents as the
//     time the process started after boot, in clock ticks.
func processIdentity() string {
	fields := []string{
		firstLine("/etc/machine-id"),
		firstLine("/proc/sys/kernel/random/boot_id"),
		namespaceOf("/proc/self/ns/pid"),
		startTimeOf("/proc/self/stat"),
	}
	return strings.Join(fields, "/")
}

// linuxIdentity is a Linux Start identity taken apart.
type linuxIdentity struct {
	machine   string
	boot      string
	namespace string
	start     string
}

// parseLinuxIdentity splits the identity after the platform tag into its
// four fields, answering false when it does not have four.
func parseLinuxIdentity(start string) (linuxIdentity, bool) {
	fields := strings.Split(strings.TrimPrefix(start, "linux:"), "/")
	if len(fields) != 4 {
		return linuxIdentity{}, false
	}
	return linuxIdentity{machine: fields[0], boot: fields[1], namespace: fields[2], start: fields[3]}, true
}

// firstLine reads the first line of a small file, empty when it cannot.
func firstLine(path string) string {
	data, err := durable.ReadFile(path)
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(data), "\n")
	return strings.TrimSpace(line)
}

// namespaceOf renders the device and inode of a namespace link as
// <dev>.<ino>, empty when it cannot be read.
func namespaceOf(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return strconv.FormatUint(uint64(stat.Dev), 10) + "." + strconv.FormatUint(stat.Ino, 10)
}

// statFields reads a /proc/<pid>/stat file and answers its fields from field
// 3 onward. proc(5) documents field 2 as the executable name in parentheses,
// which may itself hold spaces and parentheses, so the split is made after the
// last closing parenthesis.
func statFields(path string) ([]string, error) {
	data, err := durable.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	closing := strings.LastIndex(text, ")")
	if closing < 0 {
		return nil, errors.New("no command name in " + path)
	}
	return strings.Fields(text[closing+1:]), nil
}

// startTimeOf answers field 22 of a stat file, empty when it cannot.
func startTimeOf(path string) string {
	fields, err := statFields(path)
	if err != nil || len(fields) < 20 {
		return ""
	}
	return fields[22-3]
}

// sameProcessTable is rule 5 of the verdict on Linux. A different host is
// another machine. A different boot_id is dead when both records carry the
// same non-empty machine_id, because the machine has booted since and every
// process of the earlier boot has ended, and unknown otherwise, because it may
// be another machine sharing the directory. The same boot_id with a different
// PID namespace is unknown, because the PID names nothing this process can
// look up.
func sameProcessTable(record LockRecord, host, self string) Verdict {
	if record.Host != host {
		return VerdictUnknown
	}
	theirs, ok := parseLinuxIdentity(record.Start)
	if !ok {
		return VerdictUnknown
	}
	mine, ok := parseLinuxIdentity(self)
	if !ok || theirs.boot == "" || mine.boot == "" {
		return VerdictUnknown
	}
	if theirs.boot != mine.boot {
		if theirs.machine != "" && theirs.machine == mine.machine {
			return VerdictDead
		}
		return VerdictUnknown
	}
	if theirs.namespace == "" || theirs.namespace != mine.namespace {
		return VerdictUnknown
	}
	return verdictContinue
}

// recordedProcessGone is rule 6 of the verdict on Linux: /proc/<pid>/stat
// absent is gone; a state of Z or X, which proc(5) documents as zombie and
// dead, is gone; a starttime that differs from the record is gone; anything
// else is not. A record whose starttime is empty, which startTimeOf leaves
// when the holder could not read its own stat file, names no process the
// table can be compared with, so only an absent or ended PID proves it gone.
func recordedProcessGone(record LockRecord) bool {
	theirs, ok := parseLinuxIdentity(record.Start)
	if !ok {
		return false
	}
	fields, err := statFields("/proc/" + strconv.Itoa(record.PID) + "/stat")
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	if err != nil || len(fields) < 20 {
		return false
	}
	state := fields[0]
	if state == "Z" || state == "X" {
		return true
	}
	if theirs.start == "" {
		return false
	}
	return fields[22-3] != theirs.start
}
