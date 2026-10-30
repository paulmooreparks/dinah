//go:build !windows && !linux

package hostnotify

// On macOS a notification goes through UNUserNotificationCenter, which only
// serves a signed application bundle and needs cgo or Objective-C, so the
// notifier does nothing there, as on every platform without its own file.
// https://developer.apple.com/documentation/usernotifications/unusernotificationcenter
func newNotifier(appName string, activate func()) Notifier { return none{} }
