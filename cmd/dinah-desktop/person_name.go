package main

import (
	"errors"
	"os/user"
	"strings"
	"unicode"

	"studio/internal/bus"
)

// The person who runs Dinah has a handle on the bus, bus.Human, which is
// the address agents send to and reply to, and a name, which is what a
// person reads: on a delivered message's card and in a handoff. The name is
// the person's own choice in Settings › Chat, else the display name the
// operating system has for the account (os/user, whose Name "might be
// blank"), else the account's user name. Records keep the handle, and the
// name is put in when a heading is written, so a changed name shows on
// every message (Paul, 26 October 2026).

// personNameLimit is the longest name the setting takes.
const personNameLimit = 80

// SetPersonName sets the person's name, or clears it to use the account's
// name.
func (a *App) SetPersonName(name string) error {
	name = strings.TrimSpace(name)
	if len([]rune(name)) > personNameLimit {
		return errors.New("a name can be at most 80 characters")
	}
	if strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return errors.New("a name cannot hold control characters")
	}
	a.personMu.Lock()
	defer a.personMu.Unlock()
	if err := a.busHost().With(func(b *bus.Bus) error { return b.SetPersonName(name) }); err != nil {
		return err
	}
	a.personLoaded, a.personChosen = true, name
	a.dropLegacyPersonName()
	return nil
}

// chosenPersonName is the name the person chose, or empty. It is kept in
// the bus, where the command line and the agents' helpers read it too, and
// is read once and then held, since this window is the only one that sets
// it. The caller must not hold a.mu.
//
// A name from an earlier release is carried over from settings.json the
// first time, unless the bus already has one, even one the person cleared.
func (a *App) chosenPersonName() string {
	a.personMu.Lock()
	defer a.personMu.Unlock()
	if a.personLoaded {
		return a.personChosen
	}
	var name string
	err := a.busHost().With(func(b *bus.Bus) error {
		stored, ok, err := b.StoredPersonName()
		if err != nil {
			return err
		}
		if !ok {
			a.mu.Lock()
			legacy := strings.TrimSpace(a.settings.LegacyPersonName)
			a.mu.Unlock()
			if legacy != "" {
				if _, err := b.ImportPersonName(legacy); err != nil {
					return err
				}
				if stored, _, err = b.StoredPersonName(); err != nil {
					return err
				}
			}
		}
		name = stored
		return nil
	})
	if err != nil {
		// Unreadable now; the headings fall back to the account's name and
		// the next call tries again.
		logf("core", "error", "read the person's name: %v", err)
		return ""
	}
	a.personLoaded, a.personChosen = true, name
	a.dropLegacyPersonName()
	return name
}

// dropLegacyPersonName clears the old copy of the name in settings.json, so
// that the next save no longer writes it.
func (a *App) dropLegacyPersonName() {
	a.mu.Lock()
	has := a.settings.LegacyPersonName != ""
	a.mu.Unlock()
	if !has {
		return
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.LegacyPersonName = ""
		return nil
	}); err != nil {
		logf("core", "error", "clear the old copy of the person's name: %v", err)
	}
}

// personName is the person's name as a heading shows it.
func (a *App) personName() string {
	if chosen := a.chosenPersonName(); chosen != "" {
		return chosen
	}
	return accountName()
}

// agentPerson is what the agents are told to call the person: the name the
// person chose in Settings › Chat, or "the person" when they chose none
// (bus.PersonLabel). The account's name is not used here, since the person
// did not give it for this.
func (a *App) agentPerson() string {
	return bus.PersonLabel(a.chosenPersonName())
}

// accountName is the operating system's name for the account, or "" when
// it has none.
func accountName() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	if name := strings.TrimSpace(u.Name); name != "" {
		return name
	}
	// A Windows user name is DOMAIN\name.
	return u.Username[strings.LastIndex(u.Username, `\`)+1:]
}

// speakerName is how a heading writes a bus sender: the person's name for
// the person's handle, and any other sender as it is. Before an App sets
// it, the person is "you".
var speakerName = func(from string) string {
	if from == bus.Human {
		return "you"
	}
	return from
}

// useSpeakerNames makes headings write the App's person's name.
func (a *App) useSpeakerNames() {
	speakerName = func(from string) string {
		if from != bus.Human {
			return from
		}
		if name := a.personName(); name != "" {
			return name
		}
		return "you"
	}
}
