// Package hostnotify tells the person something through the host operating
// system rather than the app's own window: a notification from the system,
// and a request for attention on the app's main window. Dinah uses it
// when a flow waits for the person while its window is in the background.
//
// Every implementation uses only documented interfaces of its platform. The
// Windows one uses Shell_NotifyIcon and FlashWindowEx, the Linux one the
// freedesktop Desktop Notifications specification over D-Bus, and on every
// other platform the notifier does nothing. The package uses no cgo, so a
// program built with CGO_ENABLED=0 may import it.
package hostnotify

// Notifier shows notifications through the host operating system.
type Notifier interface {
	// Notify shows a notification with a title and a body. When the host
	// cannot show one, it returns nil and does nothing.
	Notify(title, body string) error
	// Attention asks the host to draw the person's eye to the app's main
	// window without taking the focus from the window in front.
	Attention() error
	// Focused reports whether one of the program's windows is the active
	// window, and whether the host can tell at all; when known is false the
	// caller decides by other means.
	Focused() (focused, known bool)
	// Close removes whatever the notifier put on the screen and releases
	// what it holds.
	Close() error
}

// New returns the host's notifier for a program called appName, which is
// also the title of its main window. activate, which may be nil, is called
// when the person clicks a notification or the program's notification icon.
// Nothing is shown and no connection is made until the first Notify.
func New(appName string, activate func()) Notifier {
	return newNotifier(appName, activate)
}

// none is the notifier of a host that offers nothing usable.
type none struct{}

func (none) Notify(title, body string) error { return nil }
func (none) Attention() error                { return nil }
func (none) Focused() (bool, bool)           { return false, false }
func (none) Close() error                    { return nil }
