package main

import (
	"encoding/json"
	"errors"
	"strings"

	"studio/internal/agents"
	"studio/internal/providers"
	"studio/internal/sessionhost"
)

// A chat reopens with the values of its options it was saved with, and
// Dinah refuses to reopen it when its agent no longer offers one of them,
// since it will not move an existing conversation to another model by
// itself (restoreOptions). Claude's adapter sometimes leaves a chat's saved
// model out of the list it offers (docs/design/chat-recovery.md). Paul asked
// on 28 October 2026 for a better way out than starting again, and decided
// on this one: the refusal offers Open With…, listing the values the agent
// does offer, and the person chooses one. Ticking "Always Use This When …
// Isn't Offered" stores the choice as a substitution for the provider, so
// that later reopenings use it without asking while the saved value is not
// offered, and use the saved value again once it is. Nothing maps one value
// to another by itself: only the person's choice in the panel and the
// substitutions they stored apply, and Settings › Agents lists them, each
// with Remove.

type OptionSubstitution = sessionhost.OptionSubstitution
type OptionSubstitutions = sessionhost.OptionSubstitutions
type OfferedValue = sessionhost.OfferedValue
type UnofferedOption = sessionhost.UnofferedOption
type unofferedOptionError = sessionhost.UnofferedOptionError
type reopening = sessionhost.Reopening
type appliedSubstitution = sessionhost.AppliedSubstitution

func optionValue(v json.RawMessage) (any, bool) { return sessionhost.OptionValue(v) }
func sameValue(x, y json.RawMessage) bool       { return sessionhost.SameOptionValue(x, y) }
func valueText(v json.RawMessage) string        { return sessionhost.ValueText(v) }
func findSubstitution(list []OptionSubstitution, option string, from json.RawMessage) int {
	return sessionhost.FindSubstitution(list, option, from)
}

// checkStoredSubstitutions drops the stored substitutions that make no
// sense: one with no option, with a value no option can have, from a value
// to itself, or a second for the same option and saved value.
func checkStoredSubstitutions(s *Settings) {
	for provider, list := range s.OptionSubstitutions {
		var kept []OptionSubstitution
		for _, sub := range list {
			_, fromOK := optionValue(sub.From)
			_, toOK := optionValue(sub.To)
			if provider == "" || strings.TrimSpace(sub.Option) == "" || !fromOK || !toOK || sameValue(sub.From, sub.To) || findSubstitution(kept, sub.Option, sub.From) >= 0 {
				continue
			}
			kept = append(kept, sub)
		}
		if len(kept) == 0 {
			delete(s.OptionSubstitutions, provider)
		} else {
			s.OptionSubstitutions[provider] = kept
		}
	}
	if len(s.OptionSubstitutions) == 0 {
		s.OptionSubstitutions = nil
	}
}

// reopeningFor is what reopening a chat of a provider needs from the
// settings.
func (a *App) reopeningFor(provider string) *reopening {
	provider = normalizedProvider(provider)
	a.mu.Lock()
	defer a.mu.Unlock()
	catalog := a.settings.Catalog
	if providers.For(provider).OwnPreferences {
		catalog = a.settings.ProviderCatalogs[provider]
	}
	r := sessionhost.NewReopening(provider, a.settings.OptionSubstitutions[provider], catalog, providerDisplayName)
	return r
}

// optionSubstitutionViews are the substitutions stored for each provider,
// for Settings.
func optionSubstitutionViews(s Settings) map[string][]OptionSubstitution {
	out := map[string][]OptionSubstitution{}
	for provider, list := range s.OptionSubstitutions {
		out[provider] = append([]OptionSubstitution(nil), list...)
	}
	return out
}

// OpenSessionWith reopens a chat whose saved value of an option is not
// offered, with the value the person chose from those offered instead
// (Open With…). The chat keeps the chosen value as its own from then on.
// With always, once the chat is open the choice is stored as a substitution
// for the chat's provider, so later reopenings of any of its chats make it
// without asking while the saved value is not offered. A substitution that
// cannot be saved does not undo the opening; the chat's note says so.
func (a *App) OpenSessionWith(ws, id, option string, value json.RawMessage, always bool) (SessionInfo, error) {
	if strings.TrimSpace(option) == "" {
		return SessionInfo{}, errors.New("choose the setting to open the chat with")
	}
	if _, ok := optionValue(value); !ok {
		return SessionInfo{}, errors.New("choose a value the agent offers")
	}
	provider, err := a.savedSessionProvider(id)
	if err != nil {
		return SessionInfo{}, err
	}
	provider = normalizedProvider(provider)
	a.mu.Lock()
	saved := append(json.RawMessage(nil), a.settings.SessionOptions[id][option]...)
	a.mu.Unlock()
	if _, ok := optionValue(saved); !ok {
		return SessionInfo{}, errors.New("this chat has no saved value of that setting to replace")
	}
	// The saved value's name is read before the opening, which replaces the
	// provider's catalog with the options the agent offers now.
	sub := OptionSubstitution{Option: option, From: saved, To: append(json.RawMessage(nil), value...), FromName: a.reopeningFor(provider).SavedName(option, saved)}
	info, err := a.sessionsLifecycle().Open(ws, id, map[string]json.RawMessage{option: value})
	if err != nil || !always || sameValue(saved, value) {
		return info, err
	}
	if s := a.session(id); s != nil {
		for _, o := range s.Options() {
			if o.ID != option {
				continue
			}
			sub.OptionName = o.Name
			for _, choice := range agents.OptionChoices(o) {
				if choice.Value == valueText(value) {
					sub.ToName = choice.Name
				}
			}
		}
	}
	if err := a.storeSubstitution(provider, sub); err != nil {
		note := "The chat opened, but the substitution could not be saved: " + err.Error()
		a.chatLogs().Note(id, note)
		info.Notes = append(info.Notes, note)
	}
	return info, nil
}

// storeSubstitution stores the person's substitution of one value for a
// saved one, replacing any they stored before for the same saved value.
func (a *App) storeSubstitution(provider string, sub OptionSubstitution) error {
	option, from := sub.Option, sub.From
	if err := a.updateSettings(func(s *Settings) error {
		if s.OptionSubstitutions == nil {
			s.OptionSubstitutions = OptionSubstitutions{}
		}
		list := s.OptionSubstitutions[provider]
		if i := findSubstitution(list, option, from); i >= 0 {
			list = append(list[:i:i], list[i+1:]...)
		}
		s.OptionSubstitutions[provider] = append(list, sub)
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// RemoveOptionSubstitution forgets a stored substitution, so that a chat
// whose saved value is not offered is refused again and offers Open With….
func (a *App) RemoveOptionSubstitution(provider, option string, from json.RawMessage) error {
	if err := a.updateSettings(func(s *Settings) error {
		list := s.OptionSubstitutions[provider]
		i := findSubstitution(list, option, from)
		if i < 0 {
			return errors.New("that substitution is no longer stored")
		}
		list = append(list[:i:i], list[i+1:]...)
		if len(list) == 0 {
			delete(s.OptionSubstitutions, provider)
		} else {
			s.OptionSubstitutions[provider] = list
		}
		if len(s.OptionSubstitutions) == 0 {
			s.OptionSubstitutions = nil
		}
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}
