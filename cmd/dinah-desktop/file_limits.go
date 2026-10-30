package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"studio/internal/providers"
)

// Each provider has a limit on the files the person attaches to one message,
// set on its card in Settings › Agents (Paul, 28 October 2026). The Agent
// Client Protocol has no field by which an agent reports such a limit, so
// the default comes from the provider's own documentation, and the card
// shows where it comes from. A limit of 0 is no limit: Dinah sends what the
// person attaches and shows the provider's error if it refuses a file. The
// files counted are those pasted, dropped or attached from the computer;
// a document chosen from a workspace goes as a link and is not counted.
// docs/plans/group-attachments.md records the decision and the sources.

// fileLimitSource is what a provider's documentation says: the default it
// leads to, the most the provider documents in one request (0 when it
// documents none), the sentence the card shows and the page it comes from.
type fileLimitSource struct {
	Default int
	Ceiling int
	Source  string
	URL     string
}

// fileLimitSourceOf is a provider's documented limit, and false for a provider
// with nothing to limit. The documentation is in the provider's row.
func fileLimitSourceOf(id string) (fileLimitSource, bool) {
	fl := providers.For(id).FileLimit
	if fl.Source == "" {
		return fileLimitSource{}, false
	}
	return fileLimitSource{Default: fl.Default, Ceiling: fl.Ceiling, Source: strings.ReplaceAll(fl.Source, "{product}", product.Name), URL: fl.URL}, true
}

// ProviderFileLimit is a provider's limit as its card in Settings shows it.
// Limit is the limit in force, 0 for none; Default is the documented
// default and Chosen whether the person set another. Ceiling is the most the
// provider documents, 0 when it documents none. Reason is the sentence a
// message box shows when the limit is reached.
type ProviderFileLimit struct {
	Provider  string `json:"provider"`
	Limit     int    `json:"limit"`
	Default   int    `json:"default"`
	Chosen    bool   `json:"chosen"`
	Ceiling   int    `json:"ceiling"`
	Source    string `json:"source"`
	SourceURL string `json:"sourceUrl,omitempty"`
	Reason    string `json:"reason"`
}

// fileLimitSetting is the setting's place in Settings, as the person finds it.
func fileLimitSetting(provider string) string {
	return "Settings › Agents › " + providerDisplayName(provider) + " › Files per Message"
}

func providerDisplayName(provider string) string {
	if name := providerName(provider); name != "" {
		return name
	}
	return provider
}

// fileLimitOf is the limit in force for a provider: the person's choice, or
// else the documented default.
func fileLimitOf(s Settings, provider string) int {
	provider = normalizedProvider(provider)
	if n, ok := s.FileLimits[provider]; ok {
		return n
	}
	src, _ := fileLimitSourceOf(provider)
	return src.Default
}

// fileLimitReason says why a box takes no more files for a provider.
func fileLimitReason(provider string, limit int) string {
	provider = normalizedProvider(provider)
	return providerDisplayName(provider) + " takes " + filesPerMessage(provider, limit)
}

// filesPerMessage is a limit in words, with the setting that changes it.
func filesPerMessage(provider string, limit int) string {
	return countOfFiles(limit) + " per message (" + fileLimitSetting(provider) + ")."
}

// countOfFiles is "1 file" or "n files".
func countOfFiles(n int) string {
	if n == 1 {
		return "1 file"
	}
	return strconv.Itoa(n) + " files"
}

// fileLimitViews are every offered provider's limits, for Settings and for
// the message boxes.
func fileLimitViews(s Settings) map[string]ProviderFileLimit {
	out := map[string]ProviderFileLimit{}
	for id := range providers.Registry {
		src, known := fileLimitSourceOf(id)
		if !known {
			continue
		}
		_, chosen := s.FileLimits[id]
		limit := fileLimitOf(s, id)
		v := ProviderFileLimit{Provider: id, Limit: limit, Default: src.Default, Chosen: chosen, Ceiling: src.Ceiling, Source: src.Source, SourceURL: src.URL}
		if limit > 0 {
			v.Reason = fileLimitReason(id, limit)
		}
		out[id] = v
	}
	return out
}

// checkStoredFileLimits drops what a settings file holds that is no limit
// a provider could have, so a hand-edited file cannot make one negative.
func checkStoredFileLimits(s *Settings) {
	for id, n := range s.FileLimits {
		src, known := fileLimitSourceOf(id)
		if !known || n < 0 || (src.Ceiling > 0 && n > src.Ceiling) {
			delete(s.FileLimits, id)
		}
	}
	if len(s.FileLimits) == 0 {
		s.FileLimits = nil
	}
}

// providerFileLimit is the limit in force for a provider now.
func (a *App) providerFileLimit(provider string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return fileLimitOf(a.settings, provider)
}

// SetProviderFileLimit sets a provider's limit on the files in one message;
// 0 is no limit. A limit above what the provider documents is refused, since
// the provider would refuse the message.
func (a *App) SetProviderFileLimit(provider string, limit int) error {
	src, known := fileLimitSourceOf(provider)
	if !known {
		return errors.New("unknown agent provider")
	}
	if limit < 0 {
		return errors.New("the number of files cannot be negative; leave it empty for no limit")
	}
	if src.Ceiling > 0 && limit > src.Ceiling {
		return fmt.Errorf("%s documents at most %d files in one message", providerDisplayName(provider), src.Ceiling)
	}
	if err := a.updateSettings(func(s *Settings) error {
		if s.FileLimits == nil {
			s.FileLimits = map[string]int{}
		}
		s.FileLimits[provider] = limit
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// ResetProviderFileLimit returns a provider's limit to its documented default.
func (a *App) ResetProviderFileLimit(provider string) error {
	if _, known := fileLimitSourceOf(provider); !known {
		return errors.New("unknown agent provider")
	}
	if err := a.updateSettings(func(s *Settings) error {
		delete(s.FileLimits, provider)
		if len(s.FileLimits) == 0 {
			s.FileLimits = nil
		}
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// tooManyFiles is the error for a message with more files than its chat's
// provider takes.
func tooManyFiles(provider string, limit, n int) error {
	return fmt.Errorf("the message has %s, and %s", countOfFiles(n), fileLimitReason(provider, limit))
}

// countFiles is how many of a message's attachments count toward the limit:
// the files pasted, dropped or attached from the computer.
func countFiles(atts []Attachment) int {
	n := 0
	for _, at := range atts {
		if at.Kind == "file" {
			n++
		}
	}
	return n
}

// chatProviderOf is the provider of a chat in the window, by its ID.
func (a *App) chatProviderOf(chat string) string {
	if s := a.session(chat); s != nil {
		return normalizedProvider(s.Provider())
	}
	return providerForSession(chat)
}

// GroupFileLimit is the limit on the files in a group's message: the
// smallest among its member chats that have one, with the member that set it
// and the provider that member uses. Reason is the sentence the group's box
// shows when the limit is reached.
type GroupFileLimit struct {
	Limit    int    `json:"limit"`
	Provider string `json:"provider"`
	Member   string `json:"member"`
	Reason   string `json:"reason"`
}

// groupFileLimit is the limit for a group's message, or nil when no member
// chat has one. A member outside the window, whose provider Dinah does not
// know, sets none.
func (a *App) groupFileLimit(members []BusGroupMember) *GroupFileLimit {
	a.mu.Lock()
	settings := a.settings
	a.mu.Unlock()
	var out *GroupFileLimit
	for _, m := range members {
		if m.State == "person" || m.Chat == "" {
			continue
		}
		provider := a.chatProviderOf(m.Chat)
		limit := fileLimitOf(settings, provider)
		if limit <= 0 || (out != nil && limit >= out.Limit) {
			continue
		}
		out = &GroupFileLimit{Limit: limit, Provider: provider, Member: m.Title,
			Reason: m.Title + " uses " + providerDisplayName(provider) + ", which takes " + filesPerMessage(provider, limit)}
	}
	return out
}
