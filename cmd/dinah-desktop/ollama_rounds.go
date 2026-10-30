package main

import (
	"errors"

	"studio/internal/ollamaagent"
)

// How many rounds of tool calls one Ollama turn may take. A round is a model
// answer followed by the tools it asked for. The person chooses a number under
// Settings, Agents, Ollama, beside File Access, or none, which is what every
// install starts with. A turn that reaches a number the person chose ends with a
// message saying so and the protocol's own stop reason, max_turn_requests, so
// the page can tell it from a finished turn and offer to continue.

// OllamaRoundLimit is the limit as the page is told it. Limit is the number of
// rounds, and 0 means no limit, which is what every install starts with.
// Chosen is true when the person has set a number, and false for the default.
type OllamaRoundLimit struct {
	Limit  int  `json:"limit"`
	Chosen bool `json:"chosen"`
}

// ollamaRounds is the limit now, or 0 for none.
func ollamaRounds(s Settings) int { return max(s.OllamaRounds, 0) }

func ollamaRoundLimit(s Settings) OllamaRoundLimit {
	n := ollamaRounds(s)
	return OllamaRoundLimit{Limit: n, Chosen: n > 0}
}

// useOllamaRounds gives a connection to Ollama the function that says how many
// rounds a turn may take, which it asks before each round, so a change in
// Settings applies to a turn that is running.
func (a *App) useOllamaRounds(c *ollamaagent.Conn) {
	c.SetRoundLimit(func() int {
		a.mu.Lock()
		defer a.mu.Unlock()
		return ollamaRounds(a.settings)
	})
}

// SetOllamaRoundLimit chooses how many rounds of tool calls a turn may take, or
// 0 to clear the limit to none.
func (a *App) SetOllamaRoundLimit(rounds int) error {
	if rounds < 0 {
		return errors.New("a limit is a number of rounds, 1 or more, or 0 for none")
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.OllamaRounds = rounds
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}
