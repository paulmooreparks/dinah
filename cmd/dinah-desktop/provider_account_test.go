package main

import (
	"encoding/json"
	"testing"

	"studio/internal/claudeagent"
)

// Claude's adapter reports the account it runs on; a subscription is told
// apart from an API key, and the page is told.
func TestClaudesAccountReportIsKept(t *testing.T) {
	a, _ := providerTestApp(t)
	var told []ProviderAccount
	a.sink = func(name string, data any) {
		if name == "agent:account" {
			told = append(told, data.(ProviderAccount))
		}
	}
	a.newAgentHandler().Notify(claudeagent.AuthStatusFromClaude, json.RawMessage(`{"authStatus":{"kind":"account","label":"Claude Max","account":{"plan":"max"}}}`))
	if got := a.ProviderAccounts(); len(got) != 1 || !got[0].Subscription || got[0].Label != "Claude Max" || got[0].Provider != "claude" {
		t.Fatalf("the account is %+v", got)
	}
	a.newAgentHandler().Notify(claudeagent.AuthStatusFromClaude, json.RawMessage(`{"authStatus":{"kind":"api_key","label":"Anthropic API key","detail":"ANTHROPIC_API_KEY"}}`))
	if got := a.ProviderAccounts(); got[0].Subscription || got[0].Kind != "api_key" {
		t.Fatalf("an API key was taken for a subscription: %+v", got)
	}
	a.newAgentHandler().Notify(claudeagent.AuthStatusFromClaude, json.RawMessage(`{"nothing":true}`))
	if len(told) != 2 {
		t.Fatalf("the page was told %d times", len(told))
	}
}
