package main

import (
	"errors"
	"slices"

	"studio/internal/providers"
)

// The first-start question.
//
// Judgements were switched on from the first start and did nothing a person
// could see, because no model was chosen: the status bar said "Off", and a
// calibration run chose a model in the background without a word. Paul asked
// on 20 November 2026 for judgements to set themselves up, and decided that
// they ask first. So the question is asked once, in the Judgements flyout,
// which opens by itself that once; until it is answered the status bar says
// Set Up, nothing is chosen and no call is made
// (docs/mockups/judgements-first-start, docs/plans/four-requests-2026-11-20.md).

// judgementStartCalls is the most calls a day the starting budget allows:
// about four times what a busy day used in the records of November 2026.
const judgementStartCalls = 200

// JudgementSetupOffer is the question as the flyout asks it.
type JudgementSetupOffer struct {
	// Provider would answer, and ProviderName is its name. Provider is empty
	// when no provider that can judge is offered, and there is no question.
	Provider     string `json:"provider"`
	ProviderName string `json:"providerName"`
	// Billing is "plan" for a provider used on a sign-in, "key" for one
	// billed for each token on the person's API key, and "local" for one that
	// runs on this computer.
	Billing string `json:"billing"`
	// CallsPerDay is the most calls a day the starting budget allows. Limit
	// is that budget for the week in Unit, or nil where a call's cost is not
	// known or is nothing.
	CallsPerDay int    `json:"callsPerDay"`
	Limit       *int   `json:"limit"`
	Unit        string `json:"unit"`
	// Shown says the flyout has opened by itself once already.
	Shown bool `json:"shown"`
}

// judgementSetupOffer is the question for the settings as they are: the
// default provider answers if it can judge, and otherwise the first that can.
func (a *App) judgementSetupOffer(s JudgementSettings) *JudgementSetupOffer {
	offer := &JudgementSetupOffer{CallsPerDay: judgementStartCalls, Shown: s.SetupShown}
	ids := judgeProviders()
	if len(ids) == 0 {
		return offer
	}
	offer.Provider = ids[0]
	if def := a.defaultProvider(); slices.Contains(ids, def) {
		offer.Provider = def
	}
	row := providers.Registry[offer.Provider]
	offer.ProviderName, offer.Unit = row.Name, row.BudgetUnit
	switch {
	case row.JudgeLocal:
		offer.Billing = "local"
	case providerBilledByKey(a, offer.Provider):
		offer.Billing = "key"
	default:
		offer.Billing = "plan"
	}
	if row.JudgeCallCost > 0 && !row.JudgeLocal {
		week := judgementStartCalls * 7 * row.JudgeCallCost
		offer.Limit = &week
	}
	return offer
}

// providerBilledByKey says the provider is used with the person's own API
// key, which is billed for each token.
func providerBilledByKey(a *App, provider string) bool {
	m, ok := providerModules[provider]
	return ok && m.BilledByKey != nil && m.BilledByKey(a)
}

// AcceptJudgementSetup is the person's Turn On: the provider the question
// named judges, with the starting budget, and the choice of its model begins
// in the background.
func (a *App) AcceptJudgementSetup() error {
	snap, err := a.JudgementSettings()
	if err != nil {
		return err
	}
	s := snap.Settings
	offer := a.judgementSetupOffer(s)
	if offer.Provider == "" {
		return errors.New("no agent that can judge is offered")
	}
	s.Setup, s.Enabled, s.Provider = "on", true, offer.Provider
	if offer.Limit != nil {
		for i := range s.Budgets {
			if s.Budgets[i].Provider == offer.Provider && s.Budgets[i].Limit == nil {
				limit := *offer.Limit
				s.Budgets[i].Limit = &limit
			}
		}
	}
	saved, err := a.SetJudgementSettings(s, snap.Revision)
	if err != nil {
		return err
	}
	if saved.Conflict {
		return errors.New("the judgement settings changed meanwhile; try again")
	}
	go a.maybeCalibrate()
	return nil
}

// DeferJudgementSetup is the person's Not Now: judgements stay as they are,
// and the question stays in the flyout.
func (a *App) DeferJudgementSetup() error { return a.noteJudgementSetup("later", true) }

// NoteJudgementSetupShown records that the flyout opened by itself, which it
// does once.
func (a *App) NoteJudgementSetupShown() error { return a.noteJudgementSetup("", true) }

func (a *App) noteJudgementSetup(answer string, shown bool) error {
	snap, err := a.JudgementSettings()
	if err != nil {
		return err
	}
	s := snap.Settings
	if s.Setup == "on" {
		return nil
	}
	if answer != "" {
		s.Setup = answer
	}
	s.SetupShown = s.SetupShown || shown
	_, err = a.SetJudgementSettings(s, snap.Revision)
	return err
}
