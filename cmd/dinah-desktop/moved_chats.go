package main

import (
	"errors"
	"fmt"
	"time"

	"studio/internal/layout"
)

// A chat moved here from another computer by a workspace import
// (docs/design/workspace-export.md, "Continuing a Moved Chat") has its saved
// history and its stages, and no native session here: the session Claude or
// Codex kept stays on the computer it ran on, and neither documents a way to
// move it. Dinah therefore never asks the provider to load a moved chat.
// Opening it shows the saved history, which ends with the line the import
// added, and the person's next message continues it in a new session with
// the chat's provider and model, briefed from that history. That is the
// saved-history recovery a provider switch and a lost session already use
// (session_recovery.go), without the message recovery starts with, since the
// person's own message follows. The new session becomes the chat's next
// stage, so the chat keeps its logical ID and its place in every group and
// team.

// movedChat records where a moved chat came from: the identity of the Dinah
// that exported it, the export file's name, and when it was imported.
type movedChat struct {
	Exporter string    `json:"exporter,omitempty"`
	File     string    `json:"file,omitempty"`
	At       time.Time `json:"at"`
}

// movedChatDetail is what the page is told when it opens a moved chat that
// has not been continued here yet. ProviderMissing says that the chat's
// provider is not on this computer, so the chat asks for another.
type movedChatDetail struct {
	Kind            string `json:"kind"`
	Provider        string `json:"provider"`
	SessionID       string `json:"sessionId"`
	ProviderMissing bool   `json:"providerMissing,omitempty"`
}

// movedChatError refuses to open a moved chat, which is shown from its saved
// history instead. It is not a missingNativeError, so a message waiting for
// the chat on the bus does not continue it; the person's message does.
type movedChatError struct{ detail movedChatDetail }

func (e *movedChatError) Error() string {
	if e.detail.ProviderMissing {
		return providerName(e.detail.Provider) + " is not on this computer, so this chat moved from another computer cannot continue with it. Continue from Saved History carries it on with another program."
	}
	return "This chat moved here from another computer. Its next message continues it in a new session."
}

func (e *movedChatError) ErrorDetail() any { return e.detail }

// providerHere says whether a provider's program is on this computer. It is a
// variable so that a test can say so without looking for programs.
var providerHere = func(a *App, provider string) bool {
	return supportedPreferenceProvider(provider) && a.providerInfoOf(provider).Installed
}

// movedHere is the record of a moved chat that has not been continued here
// yet: its session is the one the import recorded, and no stage follows it.
func (a *App) movedHere(id string) (movedChat, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	moved, ok := a.settings.MovedChats[id]
	if !ok || a.settings.ChatBindings[id] != "" {
		return movedChat{}, false
	}
	if tr, pending := a.settings.ChatTransfers[id]; pending && tr.Target != "" {
		return movedChat{}, false
	}
	return moved, true
}

// openMoved refuses to open a moved chat that has not been continued here,
// without asking its provider anything.
func (a *App) openMoved(id string) error {
	if _, ok := a.movedHere(id); !ok {
		return nil
	}
	provider, err := a.savedSessionProvider(id)
	if err != nil {
		return err
	}
	return &movedChatError{movedChatDetail{Kind: "moved-chat", Provider: provider, SessionID: id, ProviderMissing: !providerHere(a, provider)}}
}

// ContinueMovedChat continues a chat moved here from another computer, as
// the person's next message is about to be sent to it: a new session with
// the chat's provider and model, which the handoff built from the saved
// history briefs with that message. It returns the new stage, as a recovery
// does, for the window to follow.
func (a *App) ContinueMovedChat(id string) (SessionContinuationResult, error) {
	if _, ok := a.movedHere(id); !ok {
		return SessionContinuationResult{}, errors.New("this chat did not move here from another computer, or it already continues here")
	}
	provider, err := a.savedSessionProvider(id)
	if err != nil {
		return SessionContinuationResult{}, err
	}
	if !providerHere(a, provider) {
		return SessionContinuationResult{}, fmt.Errorf("%s is not on this computer; continue this chat from its saved history with another program", providerName(provider))
	}
	tr, err := readTranscript(id)
	if err != nil {
		return SessionContinuationResult{}, err
	}
	key := a.nativeRecoveryKey(id)
	if err := a.updateSettings(func(settings *Settings) error {
		if settings.NativeRecoveryAttempts == nil {
			settings.NativeRecoveryAttempts = map[string]string{}
		}
		if settings.NativeRecoveryAttempts[key] != "" {
			return errors.New("this chat is already being continued")
		}
		settings.NativeRecoveryAttempts[key] = id
		return nil
	}); err != nil {
		return SessionContinuationResult{}, err
	}
	release := func() {
		_ = a.updateSettings(func(settings *Settings) error {
			if settings.NativeRecoveryAttempts[key] != id {
				return errSettingsUnchanged
			}
			delete(settings.NativeRecoveryAttempts, key)
			return nil
		})
	}
	model := a.nativeRecoveryModel(id, provider)
	preview, err := a.PreviewSessionRecoveryModel(id, provider, model)
	if err != nil {
		release()
		return SessionContinuationResult{}, err
	}
	if !layout.SamePath(preview.Facts.From.Workspace, tr.Workspace) {
		release()
		return SessionContinuationResult{}, errors.New("the saved conversation belongs to another workspace")
	}
	result, err := a.recoverSessionModelQuietly(id, provider, preview.Revision, model)
	if err != nil {
		release()
	}
	return result, err
}
