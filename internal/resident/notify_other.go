//go:build !windows

package resident

// platformNotifier answers ErrUnsupported on every platform but Windows.
// Linux documents inotify and macOS documents FSEvents, but each has loss
// modes of its own to specify and test, and a later card can add either
// behind Notifier.
func platformNotifier(string, *Hooks) (Notifier, error) {
	return nil, ErrUnsupported
}
