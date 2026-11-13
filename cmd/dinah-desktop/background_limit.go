package main

import (
	"fmt"

	"studio/internal/bushost"
)

func backgroundChatLimit(s Settings) int {
	if s.BackgroundChatLimit < 1 || s.BackgroundChatLimit > bushost.MaxBackgroundSlots {
		return bushost.BackgroundSlots
	}
	return s.BackgroundChatLimit
}

// BackgroundChatLimit is the effective application setting, including its default.
func (a *App) BackgroundChatLimit() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return backgroundChatLimit(a.settings)
}

// SetBackgroundChatLimit applies to new admissions on the next watcher round.
// Lowering the limit never closes a working chat.
func (a *App) SetBackgroundChatLimit(limit int) error {
	if limit < 1 || limit > bushost.MaxBackgroundSlots {
		return fmt.Errorf("choose a background chat limit from 1 to %d", bushost.MaxBackgroundSlots)
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.BackgroundChatLimit = limit
		return nil
	}); err != nil {
		return err
	}
	a.mu.Lock()
	a.runtime.SetBackgroundLimitLocked(backgroundChatLimit(a.settings))
	a.mu.Unlock()
	a.emitSettingsChanged()
	return nil
}
