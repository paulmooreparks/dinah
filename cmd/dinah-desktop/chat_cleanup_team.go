package main

import (
	"errors"
	"studio/internal/bus"
)

func (a *App) cleanupTeamRefusals(settings Settings) (map[string]string, error) {
	refusals := map[string]string{}
	if len(settings.BusSessions) == 0 {
		return refusals, nil
	}
	err := a.busHost().With(func(b *bus.Bus) error {
		seen := map[string]bool{}
		for _, recipient := range settings.BusSessions {
			if recipient == "" || seen[recipient] {
				continue
			}
			seen[recipient] = true
			if err := b.CheckRecipientRemoval(recipient); err != nil {
				var lead *bus.TeamLeadRefusal
				if !errors.As(err, &lead) {
					return err
				}
				refusals[recipient] = lead.Error()
			}
		}
		return nil
	})
	return refusals, err
}
