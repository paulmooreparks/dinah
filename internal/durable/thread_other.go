//go:build !windows && !linux

package durable

// currentThread answers that the thread is not known. macOS and the BSDs
// offer no thread identifier through golang.org/x/sys/unix, so every
// goroutine there shares one act. Nothing is retried outside Windows, so on
// these platforms the act only records that it wrote, and no operation ever
// waits or gives up because of it.
func currentThread() (uint64, bool) {
	return 0, false
}
