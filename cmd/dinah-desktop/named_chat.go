package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// CreateNamedChat makes a chat in a workspace on a provider, with the title the person gave it and, when asked,
// a model, and sends it no prompt. The page's New Chat step in the Add Chats dialog uses it so that one call
// either makes a finished chat or makes none: a chat that cannot be given its title or its model is deleted
// again, or hidden when its provider cannot delete, so a retry never leaves a duplicate or a half-configured chat behind.
func (a *App) CreateNamedChat(workspace, provider, model, title string) (SessionInfo, error) {
	title, model = strings.TrimSpace(title), strings.TrimSpace(model)
	if title == "" {
		return SessionInfo{}, errors.New("give the chat a title")
	}
	info, err := a.NewSessionWithProviderAt(workspace, provider)
	if err != nil {
		return SessionInfo{}, err
	}
	fail := func(err error) (SessionInfo, error) {
		// A provider that cannot delete a chat has it taken off the list instead (HideSession).
		if e := a.DeleteSession(info.ID); e != nil {
			logf("bus", "warning", "the chat %s that could not be set up was not deleted, so it is hidden: %v", info.ID, e)
			_ = a.HideSession(info.ID)
		}
		return SessionInfo{}, err
	}
	if _, err := a.RenameSession(info.ID, title); err != nil {
		return fail(fmt.Errorf("the chat could not be given its title: %w", err))
	}
	if model != "" {
		if err := a.chooseModel(info.ID, model); err != nil {
			return fail(err)
		}
	}
	if s := a.session(info.ID); s != nil {
		info.Title = title
	}
	return info, nil
}

// chooseModel gives a chat one of the models its provider offers, and says so when the provider does not offer it.
func (a *App) chooseModel(chat, model string) error {
	s := a.session(chat)
	if s == nil {
		return errors.New("the new chat is not open")
	}
	for _, o := range s.Options() {
		if o.Category != "model" {
			continue
		}
		for _, v := range judgeOptionValues(o) {
			if v.Value != model {
				continue
			}
			value, _ := json.Marshal(model)
			if _, err := a.SetSessionOption(chat, o.ID, value); err != nil {
				return fmt.Errorf("the chat could not be given the model %s: %w", model, err)
			}
			return nil
		}
	}
	return fmt.Errorf("this agent does not offer the model %s", model)
}
