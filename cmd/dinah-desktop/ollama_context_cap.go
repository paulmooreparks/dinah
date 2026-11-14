package main

import (
	"errors"

	"studio/internal/ollamaagent"
)

func ollamaContextCap(s Settings) int {
	if s.OllamaContextCap < 1024 {
		return ollamaagent.DefaultContextCap
	}
	return s.OllamaContextCap
}

func (a *App) useOllamaContextCap(c *ollamaagent.Conn) {
	c.SetContextCap(func() int {
		a.mu.Lock()
		defer a.mu.Unlock()
		return ollamaContextCap(a.settings)
	})
}

// SetOllamaContextCap chooses the local context ceiling. Zero restores 64K.
func (a *App) SetOllamaContextCap(tokens int) error {
	if tokens != 0 && (tokens < 1024 || tokens > 2147483647) {
		return errors.New("the context limit must be a whole number from 1024 to 2147483647 tokens, or 0 for the default")
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.OllamaContextCap = tokens
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}
