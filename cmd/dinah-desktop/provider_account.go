package main

// The account a provider's agent runs on, as far as the agent says.
// Claude's adapter reports it (claudeagent.AuthStatusMethod): "account" for a
// Claude subscription, "api_key" for an API key, "external" or "gateway"
// for a cloud backend, "none" when signed out, and nothing when it cannot
// tell. A chat's cost is then shown for what it is: a turn paid by a
// subscription costs the person nothing more, and the adapter's figure is
// what it would have cost at API prices.
//
//	agent:account  ProviderAccount  a provider's account changed

// ProviderAccount is the account a provider's agent reports.
type ProviderAccount struct {
	Provider string `json:"provider"`
	// Kind is the report's kind: account, api_key, external, gateway or
	// none. Empty means nothing has been reported.
	Kind string `json:"kind"`
	// Label names it, such as "Claude Max" or "Anthropic API key".
	Label string `json:"label"`
	// Subscription is set when a subscription pays for the turns.
	Subscription bool `json:"subscription"`
	// StateGen and StateVer order the snapshots of the accounts the page is
	// given (internal/sessionhost/state_events.go).
	StateGen string `json:"stateGen,omitempty"`
	StateVer uint64 `json:"stateVer,omitempty"`
}

// reportAccount keeps the account a provider's agent reported and tells the page,
// numbered with the other state the page is given.
func (a *App) reportAccount(acct ProviderAccount) {
	a.mu.Lock()
	if a.accounts == nil {
		a.accounts = map[string]ProviderAccount{}
	}
	a.accounts[acct.Provider] = acct
	acct.StateGen, acct.StateVer = a.appClock().Next()
	a.mu.Unlock()
	a.emit("agent:account", acct)
}

// ProviderAccounts returns every account a provider has reported.
func (a *App) ProviderAccounts() []ProviderAccount {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]ProviderAccount, 0, len(a.accounts))
	for _, acct := range a.accounts {
		acct.StateGen, acct.StateVer = a.appClock().Next()
		out = append(out, acct)
	}
	return out
}
