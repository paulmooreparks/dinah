package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"studio/internal/layout"
)

type nativeSessionRecord struct {
	Provider   string `json:"provider"`
	Prompted   bool   `json:"prompted,omitempty"`
	Succeeded  bool   `json:"succeeded,omitempty"`
	LastFailed bool   `json:"lastFailed,omitempty"`
	Refused    bool   `json:"refused,omitempty"`
}

type missingNativeDetail struct {
	Kind        string `json:"kind"`
	Provider    string `json:"provider"`
	SessionID   string `json:"sessionId"`
	Recoverable bool   `json:"recoverable"`
}

type missingNativeError struct{ detail missingNativeDetail }

func (e *missingNativeError) Error() string {
	if e.detail.Kind == "session-unverified" {
		return "The first turn did not finish; continue from saved history to reopen this chat."
	}
	return "The provider could not reopen this session. Continue from saved history instead of retrying this native ID."
}
func (e *missingNativeError) ErrorDetail() any { return e.detail }

func (a *App) missingNative(id, provider string) error {
	return &missingNativeError{missingNativeDetail{Kind: "session-not-found", Provider: provider, SessionID: id, Recoverable: a.SavedConversation(id).Recoverable}}
}

func (a *App) unverifiedNative(id, provider string) error {
	return &missingNativeError{missingNativeDetail{Kind: "session-unverified", Provider: provider, SessionID: id, Recoverable: a.SavedConversation(id).Recoverable}}
}

func (a *App) nativeRecord(id string) nativeSessionRecord {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.NativeSessions[id]
}

func (a *App) nativeRecoveryKey(id string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if recipient := a.settings.BusSessions[id]; recipient != "" {
		return recipient
	}
	return id
}

func (a *App) beforeNativePrompt(s *Session) error {
	return a.updateSettings(func(settings *Settings) error {
		if settings.NativeSessions == nil {
			settings.NativeSessions = map[string]nativeSessionRecord{}
		}
		r := settings.NativeSessions[s.ID()]
		r.Provider, r.Prompted = s.Provider(), true
		// Until a response arrives, reopening must also handle a host crash.
		r.LastFailed = true
		settings.NativeSessions[s.ID()] = r
		return nil
	})
}

func (a *App) afterNativePrompt(s *Session, promptErr error) error {
	key := a.nativeRecoveryKey(s.ID())
	return a.updateSettings(func(settings *Settings) error {
		if settings.NativeSessions == nil {
			settings.NativeSessions = map[string]nativeSessionRecord{}
		}
		r := settings.NativeSessions[s.ID()]
		r.LastFailed = promptErr != nil
		if promptErr == nil {
			r.Succeeded = true
		}
		settings.NativeSessions[s.ID()] = r
		if promptErr == nil {
			delete(settings.NativeRecoveryAttempts, key)
		}
		return nil
	})
}

func (a *App) beforeNativeOpen(ws, id string) (SessionInfo, bool, error) {
	if s := a.session(id); s != nil && s.Info().Open {
		return SessionInfo{}, false, nil
	}
	provider, err := a.savedSessionProvider(id)
	if err != nil {
		return SessionInfo{}, true, err
	}
	r := a.nativeRecord(id)
	if r.Provider == provider && r.Prompted && r.LastFailed && !r.Succeeded {
		return SessionInfo{}, true, a.unverifiedNative(id, provider)
	}
	if r.Refused && r.Provider == provider {
		return SessionInfo{}, true, a.missingNative(id, provider)
	}
	tr, readErr := readTranscript(id)
	if readErr == nil && tr != nil && !r.Prompted && tr.PendingHandoff && tr.ParentSessionID != "" && len(tr.Messages) == 0 && tr.Handoff != "" {
		info, err := a.recoverNativeSession(ws, id, provider)
		return info, true, err
	}
	return SessionInfo{}, false, nil
}

func (a *App) nativeSessionMissing(ws, id string, refused error) (SessionInfo, error) {
	provider, err := a.savedSessionProvider(id)
	if err != nil {
		return SessionInfo{}, err
	}
	if err := a.updateSettings(func(settings *Settings) error {
		if settings.NativeSessions == nil {
			settings.NativeSessions = map[string]nativeSessionRecord{}
		}
		r := settings.NativeSessions[id]
		r.Provider, r.Refused = provider, true
		settings.NativeSessions[id] = r
		return nil
	}); err != nil {
		return SessionInfo{}, errors.Join(refused, err)
	}
	if a.nativeRecord(id).LastFailed || !a.SavedConversation(id).Recoverable {
		return SessionInfo{}, a.missingNative(id, provider)
	}
	return a.recoverNativeSession(ws, id, provider)
}

func (a *App) recoverNativeSession(ws, id, provider string) (SessionInfo, error) {
	key := a.nativeRecoveryKey(id)
	refusal := a.missingNative(id, provider)
	if !a.nativeRecord(id).Refused {
		refusal = a.unverifiedNative(id, provider)
	}
	if err := a.updateSettings(func(settings *Settings) error {
		if settings.NativeRecoveryAttempts == nil {
			settings.NativeRecoveryAttempts = map[string]string{}
		}
		if settings.NativeRecoveryAttempts[key] != "" {
			return refusal
		}
		settings.NativeRecoveryAttempts[key] = id
		return nil
	}); err != nil {
		return SessionInfo{}, err
	}
	model := a.nativeRecoveryModel(id, provider)
	preview, err := a.PreviewSessionRecoveryModel(id, provider, model)
	if err != nil {
		return SessionInfo{}, errors.Join(refusal, fmt.Errorf("saved-history recovery could not be prepared: %w", err))
	}
	if !layout.SamePath(preview.Facts.From.Workspace, ws) {
		return SessionInfo{}, errors.New("the saved conversation belongs to another workspace")
	}
	result, err := a.RecoverSessionModel(id, provider, preview.Revision, model)
	if err != nil {
		return SessionInfo{}, errors.Join(a.missingNative(id, provider), err)
	}
	s := a.session(result.Session.ID)
	if s == nil {
		return SessionInfo{}, errors.Join(refusal, errors.New("the recovered session disconnected before it could open"))
	}
	info := s.InfoVersioned()
	info.Notes = append(info.Notes, "The previous native session could not be resumed; this chat continues from its saved history.")
	return info, nil
}

func (a *App) nativeRecoveryModel(id, provider string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	catalog := a.settings.ProviderCatalogs[provider]
	if len(catalog) == 0 {
		catalog = a.settings.Catalog
	}
	for _, option := range catalog {
		if option.Category == "model" {
			var model string
			if json.Unmarshal(a.settings.SessionOptions[id][option.ID], &model) == nil {
				return model
			}
		}
	}
	return ""
}
