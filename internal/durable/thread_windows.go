//go:build windows

package durable

import "golang.org/x/sys/windows"

// currentThread answers the identifier of the operating-system thread the
// calling goroutine runs on. The GetCurrentThreadId documentation says that
// until the thread terminates, the identifier "uniquely identifies the thread
// throughout the system".
func currentThread() (uint64, bool) {
	return uint64(windows.GetCurrentThreadId()), true
}
