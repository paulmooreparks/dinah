package sessionhost

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"studio/internal/acp"
	"studio/internal/agents"
	"time"
)

//go:embed option_restore.go
var OptionRestoreSource []byte

// RestoreSettingsLimit is how long the agent is given for all the requests
// that restore a chat's settings after its session was loaded or made
// (session/set_config_option, one for each setting that differs from the
// agent's value). Before 22 November 2026 these requests had no limit, and an
// agent that answered the load and then never answered one of them left the
// chat at Loading.
//
// The limit is for the whole restore and not for each request, because a chat
// can have several settings to restore, and their limits one after another
// would not fit the time an open has. That time is bushost.ConnectPatience
// (two minutes), which the wake scheduler waits for a connect it asked for,
// less the five seconds the page is given to open the chat first. The load
// may take ReopenLoadLimit, 90 seconds, so 90 + 20 + 5 = 115 seconds is the
// longest these steps take together, five seconds inside the patience.
// TestTheReopenLimitFitsTheWakePatience keeps that sum. A healthy request of
// this kind is answered in well under a second, since the agent changes a
// value it holds in memory.
//
// At the limit the open does not fail. The setting whose request was not
// answered, and each setting after it, is left at the agent's value, no
// further request is sent, and the chat says so once (unansweredSettings).
// The request that was not answered stays open on the connection, as a load
// given up at its limit does (boundedLoad): the protocol documents no way to
// withdraw it, an answer is matched to it by its id, and a late answer is
// read and discarded.
const RestoreSettingsLimit = 20 * time.Second

// errRestoreUnanswered is what setWithin returns at the limit.
var errRestoreUnanswered = errors.New("the agent did not answer the request to restore a setting in time")

func (o *Options) restoreLimit() time.Duration {
	if o.RestoreLimit > 0 {
		return o.RestoreLimit
	}
	return RestoreSettingsLimit
}

// setWithin is SetOption that waits for the agent's answer until the deadline
// at most. Past the deadline it returns errRestoreUnanswered and leaves the
// request open on the connection. The answer, if one ever comes, is read by
// the goroutine that made the request and goes no further: it changes neither
// the chat's options nor what the page shows.
func (o *Options) setWithin(conn Connection, session string, option acp.ConfigOption, value json.RawMessage, deadline time.Time) ([]acp.ConfigOption, error) {
	left := time.Until(deadline)
	if left <= 0 {
		return nil, errRestoreUnanswered
	}
	type answer struct {
		opts []acp.ConfigOption
		err  error
	}
	done := make(chan answer, 1)
	go func() {
		opts, err := o.SetOption(conn, session, option, value)
		done <- answer{opts, err}
	}()
	timer := time.NewTimer(left)
	defer timer.Stop()
	select {
	case a := <-done:
		return a.opts, a.err
	case <-timer.C:
		return nil, errRestoreUnanswered
	}
}

func (o *Options) Restore(conn Connection, session string, opts []acp.ConfigOption, chosen map[string]json.RawMessage, reopen *Reopening) ([]acp.ConfigOption, []AppliedSubstitution, error) {
	var applied []AppliedSubstitution
	// unanswered are the settings left at the agent's value because the
	// agent did not answer in time, in the order they were to be restored.
	var unanswered []AppliedSubstitution
	deadline := time.Now().Add(o.restoreLimit())
	expired := false
	ids := make([]string, 0, len(chosen))
	modelIDs := map[string]bool{}
	for _, option := range opts {
		if option.Category == "model" || option.ID == "model" {
			modelIDs[option.ID] = true
		}
	}
	for id := range chosen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if modelIDs[ids[i]] != modelIDs[ids[j]] {
			return modelIDs[ids[i]]
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		var offered *acp.ConfigOption
		for i := range opts {
			if opts[i].ID == id {
				offered = &opts[i]
				break
			}
		}
		value := chosen[id]
		if modelIDs[id] && offered != nil && reopen != nil && !agents.Allowed(*offered, value) {
			if equivalent, ok := reopen.equivalentModel(*offered, value); ok {
				value = equivalent
			}
		}
		// Only the model must be the chat's own: it is what makes the
		// conversation the same one. A setting that depends on the model,
		// such as fast mode or effort, may be missing for the model the chat
		// reopened with, so it is left at the agent's value and said in the
		// chat; the chat's saved choice is kept, and used again when the agent
		// offers it. Paul asked on 28 October 2026 for Fable chats to reopen
		// reliably, after a chat refused over fast mode once its model was
		// substituted.
		if !modelIDs[id] && (offered == nil || !agents.Allowed(*offered, value)) {
			applied = append(applied, unofferedSetting(id, offered, value))
			continue
		}
		if offered != nil && !agents.Allowed(*offered, value) && reopen != nil {
			sub, ok := reopen.substitute(*offered, value)
			if !ok {
				return nil, nil, reopen.unoffered(*offered, value)
			}
			applied = append(applied, sub)
			value = sub.To
		}
		if offered == nil || !agents.Allowed(*offered, value) {
			return nil, nil, fmt.Errorf("this chat's saved %s setting is no longer available from its provider", id)
		}
		if bytes.Equal(bytes.TrimSpace(value), bytes.TrimSpace(offered.CurrentValue)) {
			continue
		}
		// Once one request has gone unanswered, no other is sent: the limit
		// is spent, and the agent has shown that it is not answering.
		var next []acp.ConfigOption
		err := errRestoreUnanswered
		if !expired {
			next, err = o.setWithin(conn, session, *offered, value, deadline)
		}
		if errors.Is(err, errRestoreUnanswered) {
			expired = true
			// A stored substitution that was to be used for this setting was
			// not made either, so its note is taken back.
			kept := applied[:0]
			for _, sub := range applied {
				if sub.Option != id {
					kept = append(kept, sub)
				}
			}
			applied = kept
			unanswered = append(unanswered, unansweredSetting(*offered, chosen[id]))
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("restore this chat's %s setting: %w", id, err)
		}
		confirmed := false
		for _, option := range next {
			if option.ID == id && bytes.Equal(bytes.TrimSpace(value), bytes.TrimSpace(option.CurrentValue)) {
				confirmed = true
			}
		}
		if !confirmed {
			return nil, nil, fmt.Errorf("the provider did not restore this chat's %s setting", id)
		}
		opts = next
	}
	if len(unanswered) > 0 {
		unanswered[0].note = unansweredNote(unanswered, o.restoreLimit(), reopen != nil)
		applied = append(applied, unanswered...)
	}
	return opts, applied, nil
}

// unansweredSetting is a saved setting the agent was asked to restore and did
// not answer for in time. The chat opens with the agent's value. A reopened
// chat keeps its saved choice, as it does for a setting that is not offered
// (unofferedSetting), so the next open asks for it again.
func unansweredSetting(offered acp.ConfigOption, saved json.RawMessage) AppliedSubstitution {
	sub := AppliedSubstitution{unanswered: true, OptionSubstitution: OptionSubstitution{Option: offered.ID, OptionName: offered.ID,
		From: append(json.RawMessage(nil), saved...), To: append(json.RawMessage(nil), offered.CurrentValue...)}}
	if offered.Name != "" {
		sub.OptionName = offered.Name
	}
	current := ValueText(offered.CurrentValue)
	for _, choice := range agents.OptionChoices(offered) {
		if choice.Value == current {
			sub.ToName = choice.Name
		}
	}
	return sub
}

// unansweredNote is the one line a chat shows for all the settings that were
// left at the agent's value because the agent did not answer in time.
func unansweredNote(subs []AppliedSubstitution, limit time.Duration, reopened bool) string {
	within := limit.String()
	if limit >= time.Second {
		within = fmt.Sprintf("%d seconds", int(limit/time.Second))
	}
	names := make([]string, 0, len(subs))
	for _, sub := range subs {
		names = append(names, sub.OptionName+" ("+ValueText(sub.From)+")")
	}
	what, left := "this chat's "+names[0]+" setting", "it is left at "+valueWords(subs[0].ToName, subs[0].To)
	if len(subs) > 1 {
		what, left = "these settings of this chat: "+strings.Join(names, ", "), "they are left at the agent's values"
	}
	note := "The agent did not answer the request to restore " + what + " within " + within + ", so " + left + "."
	if reopened {
		note += " The chat keeps its choice and asks again the next time it is opened."
	}
	return note
}
