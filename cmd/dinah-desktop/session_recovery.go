package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
)

func recoveryChoiceRevision(revision, model string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(revision+"\x00"+model)))
}

func (a *App) PreviewSessionRecoveryModel(id, provider, model string) (SessionContinuationPreview, error) {
	if len(model) > 512 {
		return SessionContinuationPreview{}, errors.New("the model identifier is too long")
	}
	p, err := a.PreviewSessionRecovery(id, provider)
	if err == nil {
		p.Revision = recoveryChoiceRevision(p.Revision, model)
		if model != "" {
			p.Facts.To.Model = model
		}
	}
	return p, err
}

func (a *App) RecoverSessionModel(id, provider, revision, model string) (SessionContinuationResult, error) {
	if len(model) > 512 {
		return SessionContinuationResult{}, errors.New("the model identifier is too long")
	}
	p, err := a.PreviewSessionRecovery(id, provider)
	if err != nil {
		return SessionContinuationResult{}, err
	}
	if revision != recoveryChoiceRevision(p.Revision, model) {
		return SessionContinuationResult{}, errors.New("the recovery destination or history changed; review it again")
	}
	result, err := a.continuations().RecoverModel(id, provider, p.Revision, model)
	return a.finishRecovered(id, result, err, true)
}

// recoverSessionModelQuietly is RecoverSessionModel without the message that
// starts the recovered chat, for a chat whose next message is the person's
// own, such as a chat moved here from another computer (moved_chats.go).
func (a *App) recoverSessionModelQuietly(id, provider, revision, model string) (SessionContinuationResult, error) {
	p, err := a.PreviewSessionRecovery(id, provider)
	if err != nil {
		return SessionContinuationResult{}, err
	}
	if revision != recoveryChoiceRevision(p.Revision, model) {
		return SessionContinuationResult{}, errors.New("the recovery destination or history changed; review it again")
	}
	result, err := a.continuations().RecoverModel(id, provider, p.Revision, model)
	return a.finishRecovered(id, result, err, false)
}

// PreviewSessionRecovery reads only local records, even when the original
// provider cannot load its thread. The normal handoff budget still applies.
func (a *App) PreviewSessionRecovery(id, provider string) (SessionContinuationPreview, error) {
	p, err := a.continuations().PreviewRecovery(id, provider)
	if warning := a.transferWarning(id); warning != "" {
		p.Warnings = append(p.Warnings, warning)
	}
	return p, err
}

func (a *App) RecoverSession(id, provider, revision string) (SessionContinuationResult, error) {
	result, err := a.continuations().Recover(id, provider, revision)
	return a.finishRecovered(id, result, err, true)
}

// finishRecovered ends a recovery. With prompt, the recovered chat is started
// at once; without it, the handoff waits for the person's next message, which
// the caller is about to send.
func (a *App) finishRecovered(id string, result SessionContinuationResult, err error, prompt bool) (SessionContinuationResult, error) {
	if err == nil {
		// Recovery starts the saved conversation immediately. Waiting for the
		// person's next message would leave another untouched allocation.
		if prompt {
			if sendErr := a.Send(result.Session.ID, "Continue the saved conversation and resume the user's pending task. Preserve its decisions and verify unfinished work before proceeding.", nil, nil); sendErr != nil {
				return result, sendErr
			}
		}
		if s := a.session(result.Session.ID); s != nil {
			result.Session = s.InfoVersioned()
		}
		a.mu.Lock()
		recipient := a.settings.BusSessions[result.Session.ID]
		a.mu.Unlock()
		if releaseErr := a.busCoordinator().ReleaseCommittedRecovery(id, recipient); releaseErr != nil {
			return result, releaseErr
		}
		a.emit("notification:delivery", nil)
	}
	return result, err
}
