package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"studio/internal/sessionhost"
)

type waitingChatsStore struct {
	Version int                                `json:"version"`
	Items   map[string]sessionhost.WakeFailure `json:"items"`
}

func decodeWaitingChats(data []byte) (waitingChatsStore, error) {
	var store waitingChatsStore
	if len(data) > maxNotificationStore {
		return store, errors.New("waiting-chat recovery state exceeds 32 MiB")
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return store, err
	}
	if store.Version != 1 || store.Items == nil {
		return store, errors.New("unsupported waiting-chat recovery state")
	}
	for origin, item := range store.Items {
		if origin == "" || len(origin) > 512 || item.Route.Origin != origin || item.Route.Target == "" || !filepath.IsAbs(item.Route.Cwd) || !notificationID.MatchString(item.Attempt) || item.Created.IsZero() || len(item.Error) > 65536 {
			return store, errors.New("invalid waiting-chat recovery record")
		}
		if item.Observed != nil && (item.Observed.Origin != origin || item.Observed.Target == "" || !filepath.IsAbs(item.Observed.Cwd)) {
			return store, errors.New("invalid observed waiting-chat route")
		}
		switch item.Stage {
		case "opening", "failed", "routing-recovery":
		default:
			return store, errors.New("unsupported waiting-chat recovery stage")
		}
	}
	return store, nil
}

func readWaitingChats() (map[string]sessionhost.WakeFailure, error) {
	data, err := readStateFile(filepath.Join(configDir(), "waiting-chats.json"))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]sessionhost.WakeFailure{}, nil
	}
	if err != nil {
		return nil, err
	}
	store, err := decodeWaitingChats(data)
	return store.Items, err
}

func writeWaitingChats(items map[string]sessionhost.WakeFailure) error {
	data, err := json.Marshal(waitingChatsStore{1, items})
	if err != nil {
		return err
	}
	if _, err := decodeWaitingChats(data); err != nil {
		return err
	}
	return writeValidatedStateFile(filepath.Join(configDir(), "waiting-chats.json"), data, func(previous []byte) error { _, err := decodeWaitingChats(previous); return err })
}

func (a *App) waitingRoute(origin, cwd string) (sessionhost.WakeRoute, error) {
	if a.removedChat(origin) {
		return sessionhost.WakeRoute{}, errors.New("this chat was removed")
	}
	if a.transferHeld(origin, "") {
		return sessionhost.WakeRoute{}, errors.New("the chat has an unfinished continuation transfer; review saved-history recovery")
	}
	target, err := a.ResolveSessionContinuation(origin)
	resolutionError := ""
	if err != nil {
		resolutionError = err.Error()
		target = origin
	}
	if target == "" {
		target = origin
	}
	if tr, err := sessionhost.ReadTranscript(configDir(), target); err == nil && tr.Workspace != "" {
		cwd = tr.Workspace
	}
	if cwd == "" {
		return sessionhost.WakeRoute{}, errors.New("the waiting chat's workspace is unavailable; review its saved history")
	}
	provider, err := a.savedSessionProvider(target)
	if err != nil {
		return sessionhost.WakeRoute{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	recipient := a.settings.BusSessions[origin]
	if recipient == "" {
		tr := a.settings.ChatTransfers[origin]
		if destination, _, ok := committedDestination(&a.settings, origin, tr.Recipient); ok && destination == target {
			recipient = tr.Recipient
		}
	}
	return sessionhost.WakeRoute{Origin: origin, Target: target, Recipient: recipient, TargetRecipient: a.settings.BusSessions[target], Cwd: cwd, Provider: provider, Title: a.settings.SessionTitles[target], RoutingError: resolutionError}, nil
}

// OpenWaitingSession joins only the generation issued by the wake owner. It is
// used by both the page and fallback rather than opening the requested alias.
func (a *App) OpenWaitingSession(origin, attempt string) (SessionInfo, error) {
	info, err := a.busCoordinator().OpenDeliveryAttempt(origin, attempt)
	a.emit("notification:delivery", nil)
	return info, err
}

// RetryWaitingSession is an explicit person action. Routing conflicts remain
// blocked by the same ownership check even when retry was requested.
func (a *App) RetryWaitingSession(origin string) (SessionInfo, error) {
	info, err := a.busCoordinator().RetryDelivery(origin)
	a.emit("notification:delivery", nil)
	return info, err
}

func (a *App) ReloadWaitingChatState() error {
	err := a.busCoordinator().ReloadWaitingState()
	a.emit("notification:delivery", nil)
	return err
}

// These cards are derived from durable recovery state, so age/capacity pruning
// and bus read receipts cannot remove an unresolved waiting-delivery problem.
func (a *App) waitingChatNotifications() ([]Notification, error) {
	records, err := a.busCoordinator().WaitingFailures()
	if err != nil {
		return []Notification{{ID: "waiting_store_error", Kind: "delivery-store-error", Severity: "error", Title: "Waiting-chat recovery state could not be read", Body: "Automatic bus delivery is paused because its recovery state is unavailable. Ordinary notifications remain available. Repair or restore waiting-chats.json, then reload recovery state.", Details: []string{err.Error()}, Created: a.wakeClock().UTC()}}, nil
	}
	out := []Notification{}
	for origin, record := range records {
		if _, _, active := a.busCoordinator().CurrentDeliveryAttempt(origin); record.Stage == "opening" && active {
			continue
		}
		r := record.Route
		target := r
		if record.Observed != nil {
			target = *record.Observed
		}
		name := target.Title
		if name == "" {
			name = target.Target
		}
		if record.Stage == "routing-recovery" {
			record.Error += " Recover the original saved chat to preserve its waiting recipient. The separate continuation keeps its own recipient and history. Reviewing history alone does not deliver these messages."
		}
		out = append(out, Notification{ID: recoveryIncidentID(origin, r.Recipient), Kind: "delivery-failure", Severity: "error", Title: "Messages are waiting for " + name, Body: record.Error, SessionID: target.Target, SessionTitle: name, Workspace: target.Cwd, Provider: target.Provider, Created: record.Created, Lead: origin, RetryAllowed: record.Stage != "routing-recovery" && r.Recipient != "" && r.Recipient == r.TargetRecipient, Details: []string{fmt.Sprintf("Original chat: %s; recipient: %s", origin, r.Recipient), fmt.Sprintf("Original target: %s; recipient: %s", r.Target, r.TargetRecipient), fmt.Sprintf("Actual target: %s; recipient: %s", target.Target, target.TargetRecipient), "All pending messages are retained. Review the saved chat's recovery controls. Stage: " + record.Stage}})
	}
	return out, nil
}

func recoveryIncidentID(origin, recipient string) string {
	key := "recipient:" + recipient
	if recipient == "" {
		key = "source:" + origin
	}
	sum := sha256.Sum256([]byte(key))
	return "waiting_" + hex.EncodeToString(sum[:])
}
