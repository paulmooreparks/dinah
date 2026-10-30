package main

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

const codexUsagePage = "https://chatgpt.com/codex/settings/usage"
const openAIUsagePage = "https://platform.openai.com/usage"

// Optional numbers remain absent when the account service has not supplied them.
type ProviderUsageWindow struct {
	UsedPercent        *float64 `json:"usedPercent,omitempty"`
	WindowDurationMins *int64   `json:"windowDurationMins,omitempty"`
	ResetsAt           *int64   `json:"resetsAt,omitempty"`
}

type ProviderUsageBucket struct {
	ID        string               `json:"id"`
	Name      string               `json:"name,omitempty"`
	PlanType  string               `json:"planType,omitempty"`
	Primary   *ProviderUsageWindow `json:"primary,omitempty"`
	Secondary *ProviderUsageWindow `json:"secondary,omitempty"`
}

type ProviderUsageSnapshot struct {
	Provider    string `json:"provider"`
	Status      string `json:"status"`
	Detail      string `json:"detail"`
	URL         string `json:"url"`
	CheckedAt   int64  `json:"checkedAt"`
	AccountType string `json:"accountType,omitempty"`
	PlanType    string `json:"planType,omitempty"`
	// AccountLabel and LinkLabel are the words the page shows for the
	// account's type and for the link to the usage page, so that it spells
	// no provider's account in its own code.
	AccountLabel string                `json:"accountLabel,omitempty"`
	LinkLabel    string                `json:"linkLabel,omitempty"`
	Buckets      []ProviderUsageBucket `json:"buckets"`
}

type codexUsageAccount struct {
	Account *struct {
		Type     string `json:"type"`
		PlanType string `json:"planType"`
	} `json:"account"`
	RequiresOpenAIAuth *bool `json:"requiresOpenaiAuth"`
}

type codexUsageBucket struct {
	LimitID   string               `json:"limitId"`
	LimitName string               `json:"limitName"`
	PlanType  string               `json:"planType"`
	Primary   *ProviderUsageWindow `json:"primary"`
	Secondary *ProviderUsageWindow `json:"secondary"`
}

type codexUsageLimits struct {
	RateLimits          *codexUsageBucket            `json:"rateLimits"`
	RateLimitsByLimitID map[string]*codexUsageBucket `json:"rateLimitsByLimitId"`
}

// ProviderUsage reads account quotas without creating a session or prompting a model.
// These quotas are independent of the context fullness of an individual chat.
func (a *App) ProviderUsage(provider string) ProviderUsageSnapshot {
	provider = normalizedProvider(provider)
	return finishUsage(a.providerUsage(provider), providerName(provider))
}

// finishUsage adds the words the page shows for an account and for the link to
// the usage page. An API key's usage is billed separately and has its own page.
func finishUsage(s ProviderUsageSnapshot, name string) ProviderUsageSnapshot {
	switch s.AccountType {
	case "":
	case "apiKey":
		s.AccountLabel = "API Key (Separate Billing)"
	case "chatgpt":
		s.AccountLabel = "ChatGPT"
	default:
		s.AccountLabel = s.AccountType
	}
	if s.URL != "" {
		s.LinkLabel = "Open " + name + " Usage Page"
		if s.AccountType == "apiKey" {
			s.LinkLabel = "Open OpenAI API Usage"
		}
	}
	return s
}

func (a *App) providerUsage(provider string) ProviderUsageSnapshot {
	snapshot := ProviderUsageSnapshot{Provider: provider, Status: "unavailable", CheckedAt: time.Now().Unix(), Buckets: []ProviderUsageBucket{}}
	// How a provider's usage is shown is a fact of its row.
	row := providerRegistry[provider]
	switch row.Usage {
	case "link":
		a.mu.Lock()
		snapshot.URL = a.usagePage()
		a.mu.Unlock()
		if snapshot.URL == "" {
			snapshot.Detail = "Give the " + row.Name + " usage page in Settings to open it from here."
			return snapshot
		}
		snapshot.Status = "external"
		snapshot.Detail = "Open the " + row.Name + " usage page to see account usage."
		return snapshot
	case "none":
		snapshot.Detail = strings.ReplaceAll(row.UsageNote, "{product}", product.Name)
		return snapshot
	case "read":
	default:
		snapshot.Detail = "Account usage is not available for this provider."
		return snapshot
	}
	conn, _, err := a.lifecycle().Live(provider)
	if err != nil {
		snapshot.Detail = "Codex is unavailable. Check its connection in Settings > Agents, then refresh usage."
		return snapshot
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var account codexUsageAccount
	if err := conn.Call(ctx, "account/read", map[string]any{"refreshToken": false}, &account); err != nil {
		snapshot.Detail = "Codex account details could not be read. Check its sign-in in Settings > Agents, then refresh usage."
		return snapshot
	}
	if account.Account == nil {
		snapshot.Detail = "Codex did not report an account. Account usage is unavailable."
		if account.RequiresOpenAIAuth != nil && *account.RequiresOpenAIAuth {
			snapshot.URL = codexUsagePage
			snapshot.Detail = "Codex did not report a signed-in account. Sign in through Settings > Agents, then refresh usage."
		}
		return snapshot
	}
	snapshot.AccountType, snapshot.PlanType = account.Account.Type, account.Account.PlanType
	switch account.Account.Type {
	case "apiKey":
		snapshot.URL, snapshot.Status = openAIUsagePage, "external"
		snapshot.Detail = "Codex is using an API key. Open the OpenAI API usage page to see its billing usage."
		return snapshot
	case "chatgpt":
		snapshot.URL = codexUsagePage
	default:
		snapshot.Detail = "Account usage is not available for this Codex account type."
		return snapshot
	}
	var limits codexUsageLimits
	if err := conn.Call(ctx, "account/rateLimits/read", map[string]any{}, &limits); err != nil {
		snapshot.Detail = "Codex account limits could not be read. Refresh usage or open the Codex usage page."
		return snapshot
	}
	snapshot.Buckets = normalizeCodexUsage(limits)
	for _, bucket := range snapshot.Buckets {
		if bucket.Primary != nil || bucket.Secondary != nil {
			snapshot.Status = "available"
			break
		}
	}
	if snapshot.Status == "available" {
		snapshot.Detail = "These are the account quota windows reported by Codex."
	} else {
		snapshot.Detail = "Codex did not report any account quota windows. Open its usage page for account details."
	}
	return snapshot
}

func normalizeCodexUsage(limits codexUsageLimits) []ProviderUsageBucket {
	result := []ProviderUsageBucket{}
	appendBucket := func(id string, bucket *codexUsageBucket) {
		if bucket == nil {
			return
		}
		if bucket.LimitID != "" {
			id = bucket.LimitID
		}
		result = append(result, ProviderUsageBucket{ID: id, Name: bucket.LimitName, PlanType: bucket.PlanType,
			Primary: normalizeUsageWindow(bucket.Primary), Secondary: normalizeUsageWindow(bucket.Secondary)})
	}
	// The map is the authoritative multi-bucket representation when supplied.
	// Do not also display its backward-compatible single-bucket duplicate.
	if limits.RateLimitsByLimitID != nil {
		keys := make([]string, 0, len(limits.RateLimitsByLimitID))
		for id := range limits.RateLimitsByLimitID {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		for _, id := range keys {
			appendBucket(id, limits.RateLimitsByLimitID[id])
		}
	} else {
		appendBucket("", limits.RateLimits)
	}
	return result
}

func normalizeUsageWindow(window *ProviderUsageWindow) *ProviderUsageWindow {
	if window == nil {
		return nil
	}
	out := *window
	if out.UsedPercent != nil && *out.UsedPercent < 0 {
		out.UsedPercent = nil
	}
	if out.WindowDurationMins != nil && *out.WindowDurationMins <= 0 {
		out.WindowDurationMins = nil
	}
	if out.ResetsAt != nil && *out.ResetsAt <= 0 {
		out.ResetsAt = nil
	}
	if out.UsedPercent == nil && out.WindowDurationMins == nil && out.ResetsAt == nil {
		return nil
	}
	return &out
}

func (a *App) OpenProviderUsage(provider string) error {
	snapshot := a.ProviderUsage(provider)
	if snapshot.URL == "" {
		return errors.New(snapshot.Detail)
	}
	return a.openInBrowser(snapshot.URL)
}
