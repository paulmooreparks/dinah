package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"studio/internal/bus"
	"studio/internal/judge"
	"studio/internal/providers"
)

// A team's budget (docs/plans/teams.md): an optional allowance for each provider, in that provider's own unit, shared by the
// team's members on it and by the teams under them. What a turn used is what the provider reported for its requests, which the
// adapters that report it send as turn_cost updates; a turn with nothing reported is recorded as unknown and is never counted as
// zero, so a figure that is not complete says so (Unknown, Complete). Running past an allowance warns the person and the strip;
// it holds nothing.

// turnCosts are the tokens each chat's current turn has reported.
var turnCosts = struct {
	sync.Mutex
	byChat map[string]int64
}{byChat: map[string]int64{}}

// noteTurnCost adds what a request of a chat used to its turn.
func (a *App) noteTurnCost(chat string, tokens int64) {
	turnCosts.Lock()
	defer turnCosts.Unlock()
	turnCosts.byChat[chat] += tokens
}

// recordTurnUsage records what a turn used, for a chat that is a team member; a turn that reported nothing is recorded as
// unknown. It runs as every turn ends.
func (a *App) recordTurnUsage(s *Session) {
	turnCosts.Lock()
	tokens := turnCosts.byChat[s.ID()]
	delete(turnCosts.byChat, s.ID())
	turnCosts.Unlock()
	provider := normalizedProvider(s.Provider())
	unit := providers.Registry[provider].BudgetUnit
	if unit == "" || !a.teamsOpen() {
		return
	}
	busID := a.knownBusID(s.ID())
	if busID == "" {
		return
	}
	_ = a.withTeamBus(func(b *bus.Bus) error {
		lead, _, err := b.LeadOf(context.Background(), busID)
		if err != nil || lead == "" {
			return nil
		}
		if err := b.RecordTurnUsage(context.Background(), busID, provider, unit, int(tokens), tokens > 0); err != nil {
			logf("bus", "warning", "the usage of a team member's turn could not be recorded: %v", err)
		}
		return nil
	})
	a.teamsChanged()
}

// TeamBudgetRow is a team's allowance on one provider as the strip shows it. Used is what the turns that reported their usage
// came to in the period, and Unknown how many turns reported none, so Used is the whole use only when Complete. Remaining is
// absent unless the use is complete, so an incomplete figure never makes the allowance look reliable. Over says the counted
// use has passed the allowance, which is true whatever the turns not counted came to.
type TeamBudgetRow struct {
	Provider  string `json:"provider"`
	Name      string `json:"name"`
	Unit      string `json:"unit"`
	Allowance int    `json:"allowance"`
	Used      int    `json:"used"`
	Unknown   int    `json:"unknown"`
	Complete  bool   `json:"complete"`
	Remaining *int   `json:"remaining,omitempty"`
	Over      bool   `json:"over"`
	Today     int    `json:"today"`
	TodayGaps int    `json:"todayUnknown"`
	// HasAllowance says an allowance is set, which an allowance of 0 is, so that no allowance and an explicit zero differ.
	HasAllowance bool `json:"hasAllowance"`
}

// TeamBudget is the budget of a team: the period it runs over, which is the Judgements budget's weekly period, and a row for
// each provider with an allowance, then each provider with use and no allowance.
type TeamBudget struct {
	PeriodStart time.Time       `json:"periodStart"`
	PeriodEnd   time.Time       `json:"periodEnd"`
	Rows        []TeamBudgetRow `json:"rows"`
	// Providers are every provider whose usage is counted, with its native unit, so an allowance can be set on one that has
	// no use yet.
	Providers []TeamBudgetProvider `json:"providers"`
	// Unmeasured lists the providers that report no per-request usage, whose turns are recorded as unknown.
	Unmeasured []string `json:"unmeasured"`
}

// TeamBudgetProvider is a provider a team can have an allowance on, and its own unit.
type TeamBudgetProvider struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Unit     string `json:"unit"`
}

func (a *App) teamBudget(b *bus.Bus, ctx context.Context, lead string) (TeamBudget, error) {
	settings, _, err := a.judgementSettings()
	period := judge.DefaultPeriod
	if err == nil {
		period = judge.Period{Weekday: settings.Period.Weekday, Hour: settings.Period.Hour}
	}
	now := time.Now()
	start, end := judge.PeriodAt(now, period)
	out := TeamBudget{PeriodStart: start, PeriodEnd: end, Rows: []TeamBudgetRow{}, Providers: teamBudgetProviders(), Unmeasured: []string{}}
	for _, p := range out.Providers {
		if !providerReportsRequestUsage(p.Provider) {
			out.Unmeasured = append(out.Unmeasured, p.Provider)
		}
	}
	allowances, err := b.TeamAllowances(ctx, lead)
	if err != nil {
		return out, err
	}
	used, err := b.TeamUsage(ctx, lead, start, end)
	if err != nil {
		return out, err
	}
	y, m, d := now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	day, err := b.TeamUsage(ctx, lead, midnight, midnight.AddDate(0, 0, 1))
	if err != nil {
		return out, err
	}
	rows := map[string]*TeamBudgetRow{}
	row := func(provider, unit string) *TeamBudgetRow {
		r := rows[provider]
		if r == nil {
			r = &TeamBudgetRow{Provider: provider, Name: providerName(provider), Unit: unit, Complete: true}
			rows[provider] = r
		}
		return r
	}
	for _, al := range allowances {
		r := row(al.Provider, al.Unit)
		r.Allowance, r.HasAllowance = al.Allowance, true
	}
	for _, u := range used {
		r := row(u.Provider, u.Unit)
		r.Used, r.Unknown = u.Used, u.Unknown
	}
	for _, u := range day {
		r := row(u.Provider, u.Unit)
		r.Today, r.TodayGaps = u.Used, u.Unknown
	}
	has := map[string]bool{}
	for _, al := range allowances {
		has[al.Provider] = true
	}
	var names []string
	for provider := range rows {
		names = append(names, provider)
	}
	sort.Strings(names)
	for _, provider := range names {
		r := rows[provider]
		r.Complete = r.Unknown == 0
		if has[provider] {
			r.Over = r.Used > r.Allowance
			if r.Complete {
				left := max(r.Allowance-r.Used, 0)
				r.Remaining = &left
			}
		}
		out.Rows = append(out.Rows, *r)
	}
	return out, nil
}

// TeamAllowanceInput is an allowance the person sets: a provider and a count in its own unit.
type TeamAllowanceInput struct {
	Provider  string `json:"provider"`
	Allowance int    `json:"allowance"`
}

// SetTeamBudget sets a team's allowances, by provider, each in that provider's unit, replacing the set; an empty list removes
// the budget. It is the person's: no agent tool sets one.
func (a *App) SetTeamBudget(leadChat string, allowances []TeamAllowanceInput) (TeamBudget, error) {
	leadBus := a.knownBusID(leadChat)
	if leadBus == "" {
		return TeamBudget{}, errors.New("the lead must be a chat that " + product.Name + " knows on the bus")
	}
	var set []bus.TeamAllowance
	seen := map[string]bool{}
	for _, in := range allowances {
		unit := providers.Registry[in.Provider].BudgetUnit
		if unit == "" {
			return TeamBudget{}, fmt.Errorf("%q is not a provider whose usage "+product.Name+" counts", in.Provider)
		}
		if in.Allowance < 0 {
			return TeamBudget{}, errors.New("an allowance is a count of 0 or more")
		}
		if seen[in.Provider] {
			return TeamBudget{}, fmt.Errorf("%s has two allowances", providerName(in.Provider))
		}
		seen[in.Provider] = true
		set = append(set, bus.TeamAllowance{Provider: in.Provider, Unit: unit, Allowance: in.Allowance})
	}
	var out TeamBudget
	err := a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		if err := b.SetTeamAllowances(ctx, leadBus, set); err != nil {
			return err
		}
		var err error
		out, err = a.teamBudget(b, ctx, leadBus)
		return err
	})
	if err == nil {
		a.teamsChanged()
	}
	return out, err
}

// warnedOver are the allowances the person has been told are passed, once for each team, provider and period.
var warnedOver = struct {
	sync.Mutex
	seen map[string]bool
}{seen: map[string]bool{}}

// warnOverAllowances tells the person, once for each period, of a team that has used an allowance up.
func (a *App) warnOverAllowances(view TeamView, leadTitle string) {
	for _, r := range view.Budget.Rows {
		if !r.Over {
			continue
		}
		key := fmt.Sprintf("%s|%s|%d", view.Lead.Bus, r.Provider, view.Budget.PeriodStart.Unix())
		warnedOver.Lock()
		done := warnedOver.seen[key]
		warnedOver.seen[key] = true
		warnedOver.Unlock()
		if done {
			continue
		}
		sum := sha256.Sum256([]byte(key))
		item := Notification{ID: "team-allowance-" + hex.EncodeToString(sum[:8]), Kind: "app", Severity: "warning",
			Title: leadTitle + "'s team has used its " + r.Name + " allowance",
			Body: fmt.Sprintf("The team has used %d of %d %s on %s this period, counting only the turns that reported their usage (%d did not). Its turns are not held; the allowance can be raised in the team's strip.",
				r.Used, r.Allowance, r.Unit, r.Name, r.Unknown)}
		if saved, created, err := a.saveNotification(item); err == nil && created {
			a.emit("notifications:added", saved)
			a.notices.show(saved.Title, saved.Body, false, a.notifyWhenNeeded())
		}
	}
}

// TeamUsageRow is what a member's own turns used on one provider in the period: Used is the counted use and Unknown the turns
// that reported none, so Used is the whole use only when Complete.
type TeamUsageRow struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Unit     string `json:"unit"`
	Used     int    `json:"used"`
	Unknown  int    `json:"unknown"`
	Complete bool   `json:"complete"`
	// Today and TodayUnknown are the same since local midnight, which the strip shows beside the period's.
	Today        int `json:"today"`
	TodayUnknown int `json:"todayUnknown"`
}

// memberUsage is a member's own use in the budget's period.
func (a *App) memberUsage(b *bus.Bus, ctx context.Context, session string) []TeamUsageRow {
	settings, _, err := a.judgementSettings()
	period := judge.DefaultPeriod
	if err == nil {
		period = judge.Period{Weekday: settings.Period.Weekday, Hour: settings.Period.Hour}
	}
	start, end := judge.PeriodAt(time.Now(), period)
	used, err := b.MemberUsage(ctx, session, start, end)
	out := []TeamUsageRow{}
	if err != nil {
		return out
	}
	now := time.Now()
	y, m, d := now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	today, _ := b.MemberUsage(ctx, session, midnight, midnight.AddDate(0, 0, 1))
	byProvider := map[string]bus.Usage{}
	for _, u := range today {
		byProvider[u.Provider] = u
	}
	for _, u := range used {
		out = append(out, TeamUsageRow{Provider: u.Provider, Name: providerName(u.Provider), Unit: u.Unit, Used: u.Used, Unknown: u.Unknown, Complete: u.Unknown == 0,
			Today: byProvider[u.Provider].Used, TodayUnknown: byProvider[u.Provider].Unknown})
	}
	return out
}

// teamBudgetProviders are the providers whose usage is counted, in order, with each one's own unit.
func teamBudgetProviders() []TeamBudgetProvider {
	var out []TeamBudgetProvider
	for id, p := range providers.Registry {
		if p.BudgetUnit != "" {
			out = append(out, TeamBudgetProvider{Provider: id, Name: providerName(id), Unit: p.BudgetUnit})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out
}

// providerReportsRequestUsage says the provider's adapter reports what each request used, which a team's budget counts. Claude's
// adapter reports how full the context is and, only as an unstable field of the protocol, a turn's usage, so its turns are unknown.
func providerReportsRequestUsage(provider string) bool {
	return providers.Registry[provider].RequestUsage
}

// startLine is the one line a team's group keeps when a lead starts a member: its title, provider and model and the first line of its
// brief, so a start is never silent.
func startLine(title, provider, model, brief string) string {
	what := providerName(provider)
	if model != "" {
		what += ", " + model
	}
	return fmt.Sprintf("%s (%s): %s", title, what, firstLine(brief))
}
