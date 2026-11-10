package sessionhost

import "sync"

// An agent lists every session it has for a folder, and some of them may not
// be chats of the person's: another program may have run them for a purpose
// of its own. A product that knows which ones those are registers a function
// that says so, and the window keeps them out of its lists of chats, as it
// keeps a hidden chat out. The sessions stay with the agent, untouched
// (docs/design/product-applets.md).

var (
	notChatsMu sync.Mutex
	notChats   []func(id string) bool
)

// RegisterNotChat adds a function that reports whether the session with the
// agent's id is something other than a chat of the person's. A product's
// packages call it from init. The window asks while it builds a list, so the
// function must return quickly.
func RegisterNotChat(f func(id string) bool) {
	notChatsMu.Lock()
	notChats = append(notChats, f)
	notChatsMu.Unlock()
}

// NotChat reports whether a registered function says the session is not a
// chat of the person's.
func NotChat(id string) bool {
	notChatsMu.Lock()
	fs := notChats
	notChatsMu.Unlock()
	for _, f := range fs {
		if f(id) {
			return true
		}
	}
	return false
}
