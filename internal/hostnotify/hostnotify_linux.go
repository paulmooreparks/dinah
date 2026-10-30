//go:build linux

package hostnotify

import (
	"html"
	"slices"
	"sync"

	"github.com/godbus/dbus/v5"
)

// On Linux a notification is sent to the notification server of the
// person's desktop through the Desktop Notifications Specification, on the
// session bus:
// https://specifications.freedesktop.org/notification-spec/latest/
//
// Notify is org.freedesktop.Notifications.Notify. Its "default" action is
// the one the server invokes when the notification itself is clicked, and
// the server reports it with the ActionInvoked signal. A server without the
// "actions" capability may ignore actions; the notification is shown
// regardless. When there is no session bus or no notification server, the
// notifier does nothing.
//
// The specification offers no request for attention on a window, and the
// window belongs to GTK, which this cgo-free package cannot reach, so
// Attention does nothing and Focused does not know.

const (
	notificationsName = "org.freedesktop.Notifications"
	notificationsPath = "/org/freedesktop/Notifications"
)

type dbusNotifier struct {
	appName  string
	activate func()

	mu     sync.Mutex
	tried  bool
	conn   *dbus.Conn
	markup bool
	ids    map[uint32]bool
}

func newNotifier(appName string, activate func()) Notifier {
	return &dbusNotifier{appName: appName, activate: activate, ids: map[uint32]bool{}}
}

// connect opens the session bus once. It returns nil when there is none.
func (n *dbusNotifier) connect() *dbus.Conn {
	if n.tried {
		return n.conn
	}
	n.tried = true
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil
	}
	var caps []string
	if conn.Object(notificationsName, notificationsPath).Call(notificationsName+".GetCapabilities", 0).Store(&caps) != nil {
		conn.Close()
		return nil
	}
	n.markup = slices.Contains(caps, "body-markup")
	if conn.AddMatchSignal(dbus.WithMatchObjectPath(notificationsPath), dbus.WithMatchInterface(notificationsName), dbus.WithMatchMember("ActionInvoked")) == nil {
		signals := make(chan *dbus.Signal, 8)
		conn.Signal(signals)
		go n.listen(signals)
	}
	n.conn = conn
	return conn
}

// listen calls activate when one of this notifier's notifications is
// clicked.
func (n *dbusNotifier) listen(signals <-chan *dbus.Signal) {
	for s := range signals {
		if s.Name != notificationsName+".ActionInvoked" || len(s.Body) != 2 {
			continue
		}
		id, _ := s.Body[0].(uint32)
		key, _ := s.Body[1].(string)
		n.mu.Lock()
		ours := n.ids[id]
		delete(n.ids, id)
		n.mu.Unlock()
		if ours && key == "default" && n.activate != nil {
			n.activate()
		}
	}
}

func (n *dbusNotifier) Notify(title, body string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	conn := n.connect()
	if conn == nil {
		return nil
	}
	if n.markup {
		// A server with body markup reads the body as markup, in which "&"
		// and "<" start entities and tags.
		body = html.EscapeString(body)
	}
	hints := map[string]dbus.Variant{"urgency": dbus.MakeVariant(byte(1))}
	var id uint32
	err := conn.Object(notificationsName, notificationsPath).Call(notificationsName+".Notify", 0,
		n.appName, uint32(0), "", title, body, []string{"default", "Open " + n.appName}, hints, int32(-1)).Store(&id)
	if err != nil {
		return err
	}
	n.ids[id] = true
	return nil
}

func (n *dbusNotifier) Attention() error      { return nil }
func (n *dbusNotifier) Focused() (bool, bool) { return false, false }

func (n *dbusNotifier) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.conn == nil {
		return nil
	}
	err := n.conn.Close()
	n.conn = nil
	return err
}
