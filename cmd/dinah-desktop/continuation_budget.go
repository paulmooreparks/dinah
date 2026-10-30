package main

import (
	"encoding/json"
	"strconv"

	"studio/internal/acp"
	"studio/internal/providers"
	"studio/internal/sessionhost"
)

// A provider change hands the earlier conversation to the new provider as one
// message. That message has to fit the target's context, and each provider's
// is different: Codex's app-server refuses a request over 1,048,576
// characters outright, and an Ollama model reads only the context length it
// was started with. Handing over the whole history therefore failed for a
// long chat, so the handoff is now built to a budget: the newest messages go
// in full, the older ones as one line each, and the count of what was
// summarised is in the handoff and in the preview the person approves. The
// full history stays in Dinah, so nothing is lost.

const (
	// budgetOther is for a provider whose window Dinah has no figure for.
	budgetOther = 200_000
	// budgetLeast is the smallest budget a handoff is built to, so that a
	// tiny context still gets a usable summary rather than nothing.
	budgetLeast = sessionhost.MinimumHandoffBudget
)

// continuationBudget is how many characters a handoff to provider may take,
// from the provider's own limit and, when the session is open, its context
// option in tokens.
func continuationBudget(provider string, options []acp.ConfigOption) int {
	row := providers.For(normalizedProvider(provider))
	if row.HandoffFromContext {
		for _, o := range options {
			if o.ID != "context" {
				continue
			}
			// The option is a JSON string, as the agent reports it, and a
			// number in some versions, so both are read.
			var tokens int
			if json.Unmarshal(o.CurrentValue, &tokens) != nil {
				var s string
				if json.Unmarshal(o.CurrentValue, &s) == nil {
					tokens, _ = strconv.Atoi(s)
				}
			}
			if tokens > 0 {
				return ollamaContextBudget(tokens)
			}
		}
	}
	if row.HandoffChars > 0 {
		return row.HandoffChars
	}
	return budgetOther
}

// ollamaContextBudget is half of a model's window in characters, at three
// characters a token, so that the conversation after the handoff still has
// room to work.
func ollamaContextBudget(tokens int) int {
	chars := tokens * 3 / 2
	return max(chars, budgetLeast)
}

// summaryBudget is how much of the one-line-per-message summary is kept
// beside a chat. It is generous, because its purpose is a second switch made
// when the earlier chats and their providers are both unavailable.
const summaryBudget = 300_000

// judgedHandoffFormatter is the handoff formatter with a judge's summary of the older
// messages, which the rule's digest replaces only when the judge has one.
func (a *App) judgedHandoffFormatter() sessionhost.HandoffFormatter {
	f := handoffFormatter()
	f.Digest = a.judgeHandoffDigest
	return f
}

func handoffFormatter() sessionhost.HandoffFormatter {
	return sessionhost.HandoffFormatter{ProductName: product.Name, Speaker: conversationSpeaker}
}
func continuationSummary(history []ConversationMessage, budget int) string {
	return handoffFormatter().Summary(history, budget)
}
func compactHandoff(history []ConversationMessage, budget int) (string, int) {
	return handoffFormatter().Compact(history, budget)
}
func digestLine(m ConversationMessage) string { return handoffFormatter().DigestLine(m) }
func clipText(text string, size int) string   { return sessionhost.ClipText(text, size) }
func safeCut(text string, at int) int         { return sessionhost.SafeCut(text, at) }
