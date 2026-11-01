package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"studio/internal/sessionhost"
	"time"
)

func transferSettingsVersion(s *Settings, source string) string {
	b, _ := json.Marshal([]any{s.ChatTransfers[source], s.ChatBindings[source], s.BusSessions[source]})
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func (a *App) transferVersion(source string) string {
	v, active := a.transferRunning.Load(source)
	a.mu.Lock()
	defer a.mu.Unlock()
	base := transferSettingsVersion(&a.settings, source)
	if active {
		base = v.(string)
	}
	data, _ := json.Marshal([]any{base, a.settings.Defaults, a.settings.ProviderDefaults})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func (a *App) finishTransfer(source string) {
	a.transferRunning.Delete(source)
	a.mu.Lock()
	tr := a.settings.ChatTransfers[source]
	a.mu.Unlock()
	if tr.Stage == "committed" && tr.Recipient != "" {
		if err := a.busCoordinator().ReleaseCommittedRecovery(source, tr.Recipient); err != nil {
			logf("chat", "error", "the binding committed, but its waiting hold remains until reconciliation: %v", err)
		}
	}
	a.emit("notification:delivery", nil)
}

// A transfer never represents delivery acknowledgment. Settings commit its
// binding and recipient together, after the target's handoff is durable.
type chatTransfer struct {
	Source    string    `json:"source"`
	Target    string    `json:"target,omitempty"`
	Provider  string    `json:"provider"`
	Recipient string    `json:"recipient,omitempty"`
	Revision  string    `json:"revision"`
	Stage     string    `json:"stage"`
	Model     string    `json:"model,omitempty"`
	Created   time.Time `json:"created"`
}

func (a *App) recoveryModel(source, target, model string) error {
	var option string
	for _, o := range a.SessionOptions(target) {
		if o.Category == "model" {
			option = o.ID
			break
		}
	}
	if option == "" {
		return errors.New("the allocated provider exposes no supported model selection")
	}
	if err := a.updateSettings(func(s *Settings) error {
		tr := s.ChatTransfers[source]
		if tr.Stage != "allocated" || tr.Target != target {
			return errors.New("the recovery allocation changed before model selection")
		}
		tr.Model = model
		s.ChatTransfers[source] = tr
		return nil
	}); err != nil {
		return err
	}
	value, _ := json.Marshal(model)
	options, err := a.SetSessionOption(target, option, value)
	if err != nil {
		return err
	}
	for _, o := range options {
		if o.ID == option && string(o.CurrentValue) == string(value) {
			return nil
		}
	}
	return errors.New("the provider did not confirm the selected recovery model")
}

func (a *App) recoverableFork(source, child string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	owner, other := a.settings.BusSessions[source], a.settings.BusSessions[child]
	return child != "" && owner != "" && other != "" && owner != other && a.settings.ChatBindings[source] == ""
}

func (a *App) chatBindings() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	bindings := maps.Clone(a.settings.ChatBindings)
	if bindings == nil {
		bindings = map[string]string{}
	}
	for source, tr := range a.settings.ChatTransfers {
		if tr.Stage != "committed" {
			bindings[source] = ""
		}
	}
	return bindings
}

func (a *App) transferChat(source, target, provider, recipient, revision, stage string, recovery bool) error {
	return a.updateSettings(func(s *Settings) error {
		if s.ChatTransfers == nil {
			s.ChatTransfers = map[string]chatTransfer{}
		}
		old, exists := s.ChatTransfers[source]
		if stage == "allocating" {
			a.transferRunning.Store(source, transferSettingsVersion(s, source))
			if s.ChatBindings[source] != "" {
				return errors.New("this chat already has a committed successor; recover that chat instead")
			}
			if exists && old.Stage != "committed" {
				if !recovery {
					return errors.New("an earlier continuation has an unconfirmed outcome; review saved-history recovery")
				}
				s.ChatTransferHistory = append(s.ChatTransferHistory, old)
			}
			if s.BusSessions[source] != recipient {
				return errors.New("the chat's message recipient changed before allocation")
			}
			s.ChatTransfers[source] = chatTransfer{Source: source, Provider: provider, Recipient: recipient, Revision: revision, Stage: stage, Created: time.Now().UTC()}
			return nil
		}
		if !exists || old.Revision != revision || old.Provider != provider || old.Recipient != recipient {
			return errors.New("the continuation transfer changed; review recovery again")
		}
		if old.Target != "" && old.Target != target {
			return errors.New("the allocated continuation target changed")
		}
		if stage == "allocated" && old.Stage != "allocating" || stage == "prepared" && old.Stage != "allocated" || stage == "committed" && old.Stage != "prepared" {
			return errors.New("the continuation transfer is out of order")
		}
		old.Target, old.Stage = target, stage
		s.ChatTransfers[source] = old
		if stage == "allocated" && !slices.Contains(s.Hidden, target) {
			s.Hidden = append(s.Hidden, target)
		}
		if stage == "committed" {
			if s.ChatBindings == nil {
				s.ChatBindings = map[string]string{}
			}
			s.ChatBindings[source] = target
			s.Hidden = slices.DeleteFunc(s.Hidden, func(id string) bool { return id == target })
		}
		return nil
	})
}

// A committed settings binding is the authority after a crash between commit
// and clearing the older waiting hold. Reconciliation never touches deliveries.
func (a *App) readReconciledWaitingChats() (map[string]sessionhost.WakeFailure, error) {
	records, err := readWaitingChats()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	changed := false
	for source, record := range records {
		target, parent, ok := committedDestination(&a.settings, source, record.Route.Recipient)
		if !ok {
			continue
		}
		saved, err := sessionhost.ReadTranscript(configDir(), target)
		if err != nil || saved.ParentSessionID != parent || saved.Deleted {
			continue
		}
		delete(records, source)
		changed = true
	}
	if changed {
		if err := writeWaitingChats(records); err != nil {
			return nil, err
		}
	}
	return records, nil
}

func committedDestination(s *Settings, source, recipient string) (target, parent string, ok bool) {
	if recipient == "" {
		return "", "", false
	}
	seen := map[string]bool{}
	target = source
	for len(seen) < 100 && !seen[target] {
		seen[target] = true
		next := s.ChatBindings[target]
		if next == "" {
			return target, parent, parent != "" && s.BusSessions[target] == recipient
		}
		tr := s.ChatTransfers[target]
		if tr.Stage != "committed" || tr.Target != next || tr.Recipient != recipient {
			return "", "", false
		}
		parent, target = target, next
	}
	return "", "", false
}

func (a *App) transferHeld(id, recipient string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, tr := range a.settings.ChatTransfers {
		if tr.Stage != "committed" && (recipient != "" && tr.Recipient == recipient || id == tr.Source || id == tr.Target) {
			return true
		}
	}
	return false
}

func (a *App) transferWarning(source string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	tr, ok := a.settings.ChatTransfers[source]
	if !ok || tr.Stage == "committed" {
		return ""
	}
	if tr.Target == "" {
		return "An earlier allocation has no confirmed native receipt. It may exist at the provider. No message was submitted by this transfer. A new recovery keeps that record and starts a separate native chat."
	}
	return fmt.Sprintf("An earlier transfer allocated %s but did not commit message ownership. No message was submitted by this transfer. A new recovery retains that candidate's record and starts a separate native chat.", tr.Target)
}

func (a *App) transferNotifications() []Notification {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []Notification
	for source, tr := range a.settings.ChatTransfers {
		if tr.Stage == "committed" {
			continue
		}
		out = append(out, Notification{ID: "transfer_" + source, Kind: "delivery-failure", Severity: "error", Title: "Chat recovery has no committed destination", Body: "Messages remain waiting with their original recipient. Review the original saved chat and choose a provider to start a new recovery. The earlier candidate is retained; it has no confirmed delivery.", Lead: source, SessionID: source, Provider: tr.Provider, Created: tr.Created, Details: []string{"Stage: " + tr.Stage, "Allocated target: " + tr.Target, "Recipient: " + tr.Recipient}})
	}
	return out
}
