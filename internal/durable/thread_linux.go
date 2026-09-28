//go:build linux

package durable

import "golang.org/x/sys/unix"

// currentThread answers the identifier of the operating-system thread the
// calling goroutine runs on. The gettid(2) manual page says it "returns the
// caller's thread ID (TID)", which is unique within the system while the
// thread lives.
func currentThread() (uint64, bool) {
	return uint64(unix.Gettid()), true
}
