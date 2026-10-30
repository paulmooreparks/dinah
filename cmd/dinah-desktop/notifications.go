package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// Notification is a report kept under Notifications: what a toast said, with a kind for where it came from
// ("turn-failure" for a failed chat turn, "document" for one draft, "run"
// for a flow run that finished, failed or stopped, "app" for anything else)
// and a severity (error, warning, success or info).
// Questions, such as provider approval requests, are not kept, because
// their sources keep them and their response channels do not survive a
// restart.
type Notification struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Severity     string   `json:"severity"`
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	Details      []string `json:"details"`
	SessionID    string   `json:"sessionId"`
	SessionTitle string   `json:"sessionTitle"`
	Workspace    string   `json:"workspace"`
	Provider     string   `json:"provider"`
	// DraftID is the draft a report is about, which the card reopens.
	DraftID string `json:"draftId,omitempty"`
	// Lead is the chat ID, in the window, of the lead a failed relay was rerouted to, so the card can link to it.
	Lead string `json:"lead,omitempty"`
	// RunID is the run a "run" report is about, in Workspace, which a failed
	// run's card goes on with.
	RunID string `json:"runId,omitempty"`
	// Group, GroupTitle and Message say which group chat and which message a
	// "mention" report is about, in which a chat asked for the person
	// (mention_notices.go); SessionTitle is the chat that did.
	Group      string    `json:"group,omitempty"`
	GroupTitle string    `json:"groupTitle,omitempty"`
	Message    int64     `json:"message,omitempty"`
	Created    time.Time `json:"created"`
	Dismissed  bool      `json:"dismissed"`
}

var notificationDraftID = regexp.MustCompile(`^[0-9a-f]{32}$`)

type notificationStore struct {
	Version int            `json:"version"`
	Items   []Notification `json:"items"`
}

var notificationStoreMu sync.Mutex
var notificationID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

const maxNotificationStore = 32 << 20

func validateNotification(n Notification) error {
	if !notificationID.MatchString(n.ID) || n.Title == "" {
		return errors.New("invalid durable notification")
	}
	switch n.Severity {
	case "error", "warning", "success", "info":
	default:
		return errors.New("invalid durable notification")
	}
	switch n.Kind {
	case "turn-failure":
		if n.Severity != "error" || n.SessionID == "" || !filepath.IsAbs(n.Workspace) {
			return errors.New("invalid durable notification")
		}
	case "document":
		if !notificationDraftID.MatchString(n.DraftID) || n.SessionID != "" || n.Workspace != "" && !filepath.IsAbs(n.Workspace) {
			return errors.New("invalid durable notification")
		}
	case "app":
		if n.DraftID != "" || n.SessionID != "" || n.RunID != "" || n.Workspace != "" && !filepath.IsAbs(n.Workspace) {
			return errors.New("invalid durable notification")
		}
	case "mention":
		if n.Message <= 0 || n.ID != fmt.Sprintf("mention-%d", n.Message) || n.Workspace != "" || n.DraftID != "" || n.RunID != "" {
			return errors.New("invalid durable notification")
		}
	case "relay":
		if n.Message <= 0 || n.ID != fmt.Sprintf("relay-%d", n.Message) || n.Workspace != "" || n.DraftID != "" || n.RunID != "" {
			return errors.New("invalid durable notification")
		}
	case "run":
		if !notificationID.MatchString(n.RunID) || !filepath.IsAbs(n.Workspace) || n.SessionID != "" || n.DraftID != "" && !notificationDraftID.MatchString(n.DraftID) {
			return errors.New("invalid durable notification")
		}
	default:
		return errors.New("invalid durable notification")
	}
	if len(n.Title) > 4096 || len(n.Body) > 65536 || len(n.SessionTitle) > 4096 || len(n.Details) > 512 {
		return errors.New("the notification exceeds its size limit")
	}
	if len(n.SessionID) > 512 || len(n.Provider) > 64 || len(n.Workspace) > 32768 {
		return errors.New("notification source metadata exceeds its size limit")
	}
	for _, detail := range n.Details {
		if len(detail) > 8192 {
			return errors.New("a notification detail exceeds its size limit")
		}
	}
	return nil
}
func decodeNotifications(data []byte) (notificationStore, error) {
	out := notificationStore{}
	if len(data) > maxNotificationStore {
		return out, errors.New("the notification store exceeds 32 MiB")
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, fmt.Errorf("read notifications: %w", err)
	}
	if out.Version != 1 || out.Items == nil {
		return out, errors.New("unsupported notification store; saved notifications have not been replaced")
	}
	seen := map[string]bool{}
	for _, item := range out.Items {
		if item.Created.IsZero() {
			return out, errors.New("saved notification has no creation time")
		}
		if err := validateNotification(item); err != nil {
			return out, err
		}
		if seen[item.ID] {
			return out, errors.New("duplicate saved notification identity")
		}
		seen[item.ID] = true
	}
	return out, nil
}
func readNotifications() (notificationStore, error) {
	data, err := readStateFile(filepath.Join(configDir(), "notifications.json"))
	if errors.Is(err, os.ErrNotExist) {
		return notificationStore{Version: 1, Items: []Notification{}}, nil
	}
	if err != nil {
		return notificationStore{}, err
	}
	return decodeNotifications(data)
}
func writeNotifications(store notificationStore) error {
	data, err := json.Marshal(store)
	if err != nil {
		return err
	}
	if len(data) > maxNotificationStore {
		return errors.New("the notification store exceeds 32 MiB")
	}
	return writeValidatedStateFile(filepath.Join(configDir(), "notifications.json"), data, func(previous []byte) error { _, err := decodeNotifications(previous); return err })
}

// A report is kept for notificationRetention, and the store keeps the
// newest maxNotifications; a test lowers the number, since filling 500 takes
// 500 writes of the store.
const notificationRetention = 7 * 24 * time.Hour

var maxNotifications = 500

// notificationNow is the clock retention is measured by; tests set it.
var notificationNow = time.Now

// pruned leaves out the reports past their retention, and the oldest beyond
// the most the store keeps.
func pruned(items []Notification) []Notification {
	cutoff := notificationNow().Add(-notificationRetention)
	out := []Notification{}
	for _, item := range items {
		if item.Created.After(cutoff) {
			out = append(out, item)
		}
	}
	if len(out) > maxNotifications {
		out = out[len(out)-maxNotifications:]
	}
	return out
}

func (a *App) Notifications() ([]Notification, error) {
	notificationStoreMu.Lock()
	defer notificationStoreMu.Unlock()
	store, err := readNotifications()
	if err != nil {
		return nil, err
	}
	out := []Notification{}
	for _, item := range pruned(store.Items) {
		if !item.Dismissed {
			out = append(out, item)
		}
	}
	return out, nil
}

// ClearNotifications dismisses every report. Questions are not reports and
// are not touched.
func (a *App) ClearNotifications() error {
	notificationStoreMu.Lock()
	defer notificationStoreMu.Unlock()
	store, err := readNotifications()
	if err != nil {
		return err
	}
	store.Items = pruned(store.Items)
	for i := range store.Items {
		store.Items[i].Dismissed = true
	}
	return writeNotifications(store)
}
func (a *App) SaveNotification(item Notification) (Notification, error) {
	saved, _, err := a.saveNotification(item)
	return saved, err
}

// saveNotification keeps a report, and says whether it is new: a report with
// the identity of one already kept is not kept again, whether or not it was
// dismissed.
func (a *App) saveNotification(item Notification) (Notification, bool, error) {
	if err := validateNotification(item); err != nil {
		return Notification{}, false, err
	}
	notificationStoreMu.Lock()
	defer notificationStoreMu.Unlock()
	store, err := readNotifications()
	if err != nil {
		return Notification{}, false, err
	}
	for _, existing := range store.Items {
		if existing.ID == item.ID {
			return existing, false, nil
		}
	}
	item.Dismissed = false
	item.Created = notificationNow().UTC()
	store.Items = pruned(append(store.Items, item))
	if err := writeNotifications(store); err != nil {
		return Notification{}, false, err
	}
	return item, true, nil
}
func (a *App) DismissNotification(id string) error {
	if !notificationID.MatchString(id) {
		return errors.New("invalid notification identity")
	}
	notificationStoreMu.Lock()
	defer notificationStoreMu.Unlock()
	store, err := readNotifications()
	if err != nil {
		return err
	}
	for i := range store.Items {
		if store.Items[i].ID == id {
			if store.Items[i].Dismissed {
				return nil
			}
			store.Items[i].Dismissed = true
			return writeNotifications(store)
		}
	}
	return errors.New("the notification is not stored")
}
