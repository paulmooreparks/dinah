package main

import (
	"bytes"
	"encoding/json"
	"errors"

	"studio/internal/acp"
	"studio/internal/agents"
	"studio/internal/providers"
	"studio/internal/sessionhost"
)

// The agent's config options (https://agentclientprotocol.com/protocol/session-config-options):
// the settings it offers for a session, such as its model and mode. The app
// shows what the agent offers, in its order, and never assumes a particular
// option or value exists. A choice made in a session is kept for that
// session; a default is given to every new session.

// setOptions records a session's options as the agent last sent them, and
// keeps them as the catalog that Settings shows when no session is open.
func (a *App) setOptions(s *Session, opts []acp.ConfigOption) error {
	return a.optionService().Update(s, opts, false)
}

func (a *App) saveSessionOptions(s *Session, opts []acp.ConfigOption, kept map[string]json.RawMessage) error {
	saveErr := a.updateSettings(func(updated *Settings) error {
		if updated.SessionProviders == nil {
			updated.SessionProviders = map[string]string{}
		}
		updated.SessionProviders[s.ID()] = normalizedProvider(s.Provider())
		if updated.SessionOptions == nil {
			updated.SessionOptions = map[string]map[string]json.RawMessage{}
		}
		values := copyPreferenceValues(a.settings.SessionOptions[s.ID()])
		for _, option := range opts {
			if from, ok := kept[option.ID]; ok {
				values[option.ID] = append(json.RawMessage(nil), from...)
			} else if len(bytes.TrimSpace(option.CurrentValue)) > 0 && !bytes.Equal(bytes.TrimSpace(option.CurrentValue), []byte("null")) {
				values[option.ID] = append(json.RawMessage(nil), option.CurrentValue...)
			}
		}
		updated.SessionOptions[s.ID()] = values
		if len(opts) > 0 {
			if !providers.For(normalizedProvider(s.Provider())).OwnPreferences {
				updated.Catalog = opts
			} else {
				if updated.ProviderCatalogs == nil {
					updated.ProviderCatalogs = map[string][]acp.ConfigOption{}
				}
				updated.ProviderCatalogs[s.Provider()] = opts
			}
		}
		// Reopening a chat usually changes nothing here, and the settings
		// file is large, so it is written only when something did.
		same := a.settings.SessionProviders[s.ID()] == updated.SessionProviders[s.ID()] && sameJSON(a.settings.SessionOptions[s.ID()], values) &&
			sameJSON(a.settings.Catalog, updated.Catalog) && sameJSON(a.settings.ProviderCatalogs, updated.ProviderCatalogs)
		if same {
			return errSettingsUnchanged
		}
		return nil
	})
	if errors.Is(saveErr, errSettingsUnchanged) {
		return nil
	}
	return saveErr
}

// sameJSON reports whether two values encode to the same JSON; maps encode
// with their keys in order, so equal maps compare equal.
func sameJSON(x, y any) bool {
	a, errA := json.Marshal(x)
	b, errB := json.Marshal(y)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}

// SessionOptions returns the options of an open session.
func (a *App) SessionOptions(id string) []acp.ConfigOption {
	s := a.session(id)
	if s == nil {
		return nil
	}
	return s.Options()
}

// SetSessionOption changes one of a session's options now, and remembers it
// for that session. The protocol lets this happen while the agent is working.
func (a *App) SetSessionOption(id, option string, value json.RawMessage) ([]acp.ConfigOption, error) {
	return a.optionService().Set(id, option, value)
}

// SetDefault sets the value every new session gets for an option, or, with
// a null value, lets the agent's own default stand.
func (a *App) SetDefault(option string, value json.RawMessage) (map[string]json.RawMessage, error) {
	return a.SetDefaultForProvider(a.defaultProvider(), option, value)
}

type ProviderModelPreferences struct {
	Provider string                     `json:"provider"`
	Catalog  []acp.ConfigOption         `json:"catalog"`
	Defaults map[string]json.RawMessage `json:"defaults"`
	Pinned   []string                   `json:"pinned"`
}

func supportedPreferenceProvider(provider string) bool { return product.Offers(provider) }

func copyPreferenceValues(values map[string]json.RawMessage) map[string]json.RawMessage {
	copy := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		copy[key] = append(json.RawMessage(nil), value...)
	}
	return copy
}

func (a *App) modelPreferencesLocked(provider string) ProviderModelPreferences {
	catalog, defaults, pinned := a.settings.Catalog, a.settings.Defaults, a.settings.Pinned
	if providers.For(normalizedProvider(provider)).OwnPreferences {
		catalog, defaults, pinned = a.settings.ProviderCatalogs[provider], a.settings.ProviderDefaults[provider], a.settings.ProviderPinned[provider]
	}
	options := make([]acp.ConfigOption, len(catalog))
	for i, option := range catalog {
		options[i] = option
		options[i].Options = append(json.RawMessage(nil), option.Options...)
		options[i].CurrentValue = append(json.RawMessage(nil), option.CurrentValue...)
	}
	return ProviderModelPreferences{Provider: provider, Catalog: options, Defaults: copyPreferenceValues(defaults), Pinned: append([]string{}, pinned...)}
}

// ProviderModelPreferences reads the selected provider's cached options without
// connecting to it or changing the application's default provider.
func (a *App) ProviderModelPreferences(provider string) (ProviderModelPreferences, error) {
	if !supportedPreferenceProvider(provider) {
		return ProviderModelPreferences{}, errors.New("that agent is not supported yet")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.modelPreferencesLocked(provider), nil
}

func (a *App) SetDefaultForProvider(provider, option string, value json.RawMessage) (map[string]json.RawMessage, error) {
	if !supportedPreferenceProvider(provider) {
		return nil, errors.New("that agent is not supported yet")
	}
	if option == "" {
		return nil, errors.New("choose an agent setting")
	}
	var prefs ProviderModelPreferences
	var values map[string]json.RawMessage
	err := a.updateSettings(func(s *Settings) error {
		prefs = a.modelPreferencesLocked(provider)
		reset := len(bytes.TrimSpace(value)) == 0 || bytes.Equal(bytes.TrimSpace(value), []byte("null"))
		if !reset {
			valid := false
			for _, offered := range prefs.Catalog {
				if offered.ID == option && agents.Allowed(offered, value) {
					valid = true
					break
				}
			}
			if !valid {
				return errors.New("the selected provider does not offer that setting value in its current catalog")
			}
		}
		values = copyPreferenceValues(prefs.Defaults)
		if reset {
			delete(values, option)
		} else {
			values[option] = append(json.RawMessage(nil), value...)
		}
		if !providers.For(normalizedProvider(provider)).OwnPreferences {
			s.Defaults = values
		} else {
			if s.ProviderDefaults == nil {
				s.ProviderDefaults = map[string]map[string]json.RawMessage{}
			}
			s.ProviderDefaults[provider] = values
		}
		return nil
	})
	if err != nil {
		return prefs.Defaults, err
	}
	return copyPreferenceValues(values), nil
}

// SetPinned makes a value a favourite, or stops it being one.
func (a *App) SetPinned(value string, pinned bool) ([]string, error) {
	return a.SetPinnedForProvider(a.defaultProvider(), value, pinned)
}

func (a *App) SetPinnedForProvider(provider, value string, pinned bool) ([]string, error) {
	if !supportedPreferenceProvider(provider) {
		return nil, errors.New("that agent is not supported yet")
	}
	var prefs ProviderModelPreferences
	var out []string
	err := a.updateSettings(func(s *Settings) error {
		prefs = a.modelPreferencesLocked(provider)
		if pinned {
			encoded, _ := json.Marshal(value)
			valid := false
			for _, option := range prefs.Catalog {
				if (option.Category == "model" || option.ID == "model") && option.Type == "select" && agents.Allowed(option, encoded) {
					valid = true
					break
				}
			}
			if !valid {
				return errors.New("the selected provider does not offer that model in its current catalog")
			}
		}
		out = []string{}
		for _, v := range prefs.Pinned {
			if v != value {
				out = append(out, v)
			}
		}
		if pinned {
			out = append(out, value)
		}
		if !providers.For(normalizedProvider(provider)).OwnPreferences {
			s.Pinned = out
		} else {
			if s.ProviderPinned == nil {
				s.ProviderPinned = map[string][]string{}
			}
			s.ProviderPinned[provider] = out
		}
		return nil
	})
	if err != nil {
		return prefs.Pinned, err
	}
	return append([]string{}, out...), nil
}

// SetShowThinking keeps whether the run shows the agent's thinking.
func (a *App) SetShowThinking(on bool) error {
	if err := a.updateSettings(func(s *Settings) error {
		s.ShowThinking = on
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

func (a *App) optionService() *sessionhost.Options {
	a.optionsOnce.Do(func() {
		a.optionsService = sessionhost.NewOptions(sessionhost.OptionsHost{
			Conversation: a.session, Connection: a.sessionConn, Call: a.call, Save: a.saveSessionOptions, Emit: a.emit,
		})
	})
	return a.optionsService
}
