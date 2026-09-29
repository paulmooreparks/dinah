//go:build !windows

package resident

// platformNotifier answers &Unsupported{WhyPlatform} on every platform but
// Windows. Linux documents inotify and macOS documents FSEvents, but inotify
// watches one directory per registration and FSEvents needs a cgo binding,
// and each has loss modes of its own to specify and test, so a later card can
// add either behind Notifier.
func platformNotifier(*Hooks) (Notifier, error) {
	return nil, &Unsupported{Why: WhyPlatform}
}
