package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// fakeNotifier records what would have been shown; no test shows a real
// notification.
type fakeNotifier struct {
	focused, known bool
	shown          []string
	attention      int
	closed         bool
}

func (f *fakeNotifier) Notify(title, body string) error {
	f.shown = append(f.shown, title+" | "+body)
	return nil
}
func (f *fakeNotifier) Attention() error      { f.attention++; return nil }
func (f *fakeNotifier) Focused() (bool, bool) { return f.focused, f.known }
func (f *fakeNotifier) Close() error          { f.closed = true; return nil }

func TestHostNoticesOnlyWhenTheWindowIsNotInFront(t *testing.T) {
	f := &fakeNotifier{known: true, focused: true}
	h := &hostNotices{notifier: f}
	h.question(`"First"`, "", true)
	if len(f.shown) != 0 {
		t.Fatalf("notified while the window was in front: %v", f.shown)
	}
	// Where the host cannot tell, the page's report decides.
	f.known = false
	h.pageFocused = true
	h.question(`"Second"`, "", true)
	h.pageFocused = false
	h.question(`"Third"`, "", true)
	if len(f.shown) != 1 || !strings.Contains(f.shown[0], "Third") || f.attention != 1 {
		t.Fatalf("the page's focus should decide, got %v and %d flashes", f.shown, f.attention)
	}
}

func TestHostNoticesFollowTheSetting(t *testing.T) {
	f := &fakeNotifier{known: true}
	h := &hostNotices{notifier: f}
	h.question(`"Draft"`, "", false)
	if len(f.shown) != 0 {
		t.Fatalf("notified with the setting off: %v", f.shown)
	}
	h.question(`"Draft"`, "", true)
	if len(f.shown) != 1 || f.shown[0] != `"Draft" has a question for you | The agent asks for your permission.` {
		t.Fatalf("permission question: %v", f.shown)
	}
	// With no notifier, as in every other test, nothing happens.
	(&hostNotices{}).question("", "", true)
}

func TestNotifyWhenNeededSettingDefaultsOnAndPersists(t *testing.T) {
	a := NewApp()
	f := &fakeNotifier{known: true}
	a.notices.setNotifier(f)
	if !a.State().NotifyWhenNeeded {
		t.Fatal("notifications should be on by default")
	}
	if err := a.SetNotifyWhenNeeded(false); err != nil {
		t.Fatal(err)
	}
	if a.State().NotifyWhenNeeded {
		t.Fatal("the setting did not turn off")
	}
	saved, err := loadSettings()
	if err != nil || !saved.HostNotificationsOff {
		t.Fatalf("the setting was not saved: %v %+v", err, saved.HostNotificationsOff)
	}
	a.noticeQuestion("s1", json.RawMessage(`{"title":"Write notes.md"}`))
	if len(f.shown) != 0 {
		t.Fatalf("notified with the setting off: %v", f.shown)
	}
	if err := a.SetNotifyWhenNeeded(true); err != nil {
		t.Fatal(err)
	}
	a.noticeQuestion("s1", json.RawMessage(`{"title":"Write notes.md"}`))
	if len(f.shown) != 1 || !strings.Contains(f.shown[0], "Write notes.md") {
		t.Fatalf("permission question: %v", f.shown)
	}
}
