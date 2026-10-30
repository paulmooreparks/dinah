package main

import "errors"

// The key that sends a chat's prompt, a choice in Settings › Chat. Enter
// sends by default, with Shift+Enter for a new line, as most chat tools do;
// a person who writes long prompts may choose Ctrl+Enter, with Enter for a
// new line. Ctrl+Enter sends under either choice.

const (
	sendWithEnter     = "enter"
	sendWithCtrlEnter = "ctrl-enter"
)

// promptSendKey is the key that sends, as State gives it.
func promptSendKey(s Settings) string {
	if s.PromptSendKey == sendWithCtrlEnter {
		return sendWithCtrlEnter
	}
	return sendWithEnter
}

// SetPromptSendKey chooses the key that sends a chat's prompt: "enter" or
// "ctrl-enter".
func (a *App) SetPromptSendKey(key string) error {
	if key != sendWithEnter && key != sendWithCtrlEnter {
		return errors.New("choose Enter or Ctrl+Enter to send")
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.PromptSendKey = ""
		if key == sendWithCtrlEnter {
			s.PromptSendKey = sendWithCtrlEnter
		}
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}
