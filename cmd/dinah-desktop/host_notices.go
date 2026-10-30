package main

import (
	"encoding/json"
	"strings"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/hostnotify"
)

// Host notifications. An agent works for
// a long time while the person works elsewhere, so when it asks for
// permission and Dinah's window is not the active one, the app says
// so through the operating system as well as in its own Notifications.
// Nothing else reaches the host; toasts and reports stay in the app.

// hostNotices decides which changes reach the host's notifier.
type hostNotices struct {
	mu       sync.Mutex
	notifier hostnotify.Notifier // nil until the window starts, and in tests unless set
	// pageFocused is the page's own report of whether it has the focus,
	// used where the host cannot tell (hostnotify.Notifier.Focused).
	pageFocused bool
}

// question tells the person that an agent asks for permission.
func (h *hostNotices) question(chat, what string, enabled bool) {
	if chat == "" {
		chat = "A chat"
	}
	if what == "" {
		what = "The agent asks for your permission."
	}
	h.show(chat+" has a question for you", what, true, enabled)
}

func (h *hostNotices) show(title, body string, attention, enabled bool) {
	h.mu.Lock()
	n := h.notifier
	focused, known := false, false
	if n != nil {
		focused, known = n.Focused()
	}
	if !known {
		focused = h.pageFocused
	}
	h.mu.Unlock()
	if n == nil || !enabled || focused {
		return
	}
	if err := n.Notify(title, body); err != nil {
		logf("core", "warn", "host notification: %v", err)
	}
	if attention {
		_ = n.Attention()
	}
}

func (h *hostNotices) setNotifier(n hostnotify.Notifier) {
	h.mu.Lock()
	h.notifier = n
	h.mu.Unlock()
}

func (h *hostNotices) close() {
	h.mu.Lock()
	n := h.notifier
	h.notifier = nil
	h.mu.Unlock()
	if n != nil {
		_ = n.Close()
	}
}

// startHostNotices makes the host's notifier when the window starts.
func (a *App) startHostNotices() {
	if configOverride != "" {
		return
	}
	a.notices.setNotifier(hostnotify.New(product.Name, func() {
		if a.ctx != nil {
			wruntime.WindowUnminimise(a.ctx)
			wruntime.WindowShow(a.ctx)
		}
	}))
}

func (a *App) notifyWhenNeeded() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return !a.settings.HostNotificationsOff
}

// noticeQuestion is called when an agent asks for permission.
func (a *App) noticeQuestion(sessionID string, toolCall json.RawMessage) {
	chat := ""
	a.mu.Lock()
	chat = a.settings.SessionTitles[sessionID]
	s := a.runtime.ConversationLocked(sessionID)
	a.mu.Unlock()
	if chat == "" && s != nil {
		chat = s.Title()
	}
	if chat != "" {
		chat = `"` + chat + `"`
	}
	var call struct {
		Title string `json:"title"`
	}
	_ = json.Unmarshal(toolCall, &call)
	what := ""
	if t := strings.TrimSpace(call.Title); t != "" {
		what = "It asks for your permission: " + t
	}
	a.notices.question(chat, what, a.notifyWhenNeeded())
}

// ReportWindowFocus is the page's report of whether it has the focus, which
// decides where the host cannot tell itself (Linux).
func (a *App) ReportWindowFocus(focused bool) {
	a.notices.mu.Lock()
	a.notices.pageFocused = focused
	a.notices.mu.Unlock()
}

// SetNotifyWhenNeeded turns host notifications on or off.
func (a *App) SetNotifyWhenNeeded(on bool) error {
	if err := a.updateSettings(func(s *Settings) error {
		s.HostNotificationsOff = !on
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}
