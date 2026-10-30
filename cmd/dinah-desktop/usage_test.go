package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type usageTestConnection struct {
	*testProviderConnection
	account, limits string
	failMethod      string
	deadline        time.Time
}

func (c *usageTestConnection) Call(ctx context.Context, method string, params, out any) error {
	c.calls = append(c.calls, providerCall{method, params})
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 8*time.Second {
		return errors.New("account calls must be bounded")
	}
	if c.deadline.IsZero() {
		c.deadline = deadline
	} else if deadline != c.deadline {
		return errors.New("account calls must share a timeout")
	}
	if method == c.failMethod {
		return errors.New("private server details must not reach the UI")
	}
	switch method {
	case "account/read":
		if params.(map[string]any)["refreshToken"] != false {
			return errors.New("usage must not request a token refresh")
		}
		return json.Unmarshal([]byte(c.account), out)
	case "account/rateLimits/read":
		return json.Unmarshal([]byte(c.limits), out)
	default:
		return errors.New("usage must not create a session or prompt a model")
	}
}

func usageTestApp(t *testing.T, account, limits string) (*App, *usageTestConnection) {
	t.Helper()
	a, _ := providerTestApp(t)
	c := &usageTestConnection{testProviderConnection: newTestProviderConnection(), account: account, limits: limits}
	setTestConnection(a, "codex", c)
	return a, c
}

func TestProviderUsageReportsAllBuckets(t *testing.T) {
	a, c := usageTestApp(t, `{"account":{"type":"chatgpt","planType":"pro","email":"private@example.com"}}`, `{
		"rateLimits":{"limitId":"codex","primary":{"usedPercent":99}},
		"rateLimitsByLimitId":{
			"codex_other":{"limitId":"codex_other","limitName":"Other Quota","primary":{"usedPercent":42,"windowDurationMins":60,"resetsAt":1730950800}},
			"codex":{"limitId":"codex","planType":"pro","primary":{"usedPercent":0,"windowDurationMins":300,"resetsAt":1730947200},"secondary":{"usedPercent":80,"windowDurationMins":10080,"resetsAt":1731547200}}
		}}`)
	got := a.ProviderUsage("codex")
	if got.Status != "available" || got.Provider != "codex" || got.AccountType != "chatgpt" || got.PlanType != "pro" || got.URL != codexUsagePage || got.CheckedAt == 0 {
		t.Fatalf("incorrect snapshot: %+v", got)
	}
	if len(got.Buckets) != 2 || got.Buckets[0].ID != "codex" || got.Buckets[1].Name != "Other Quota" {
		t.Fatalf("multi-bucket limits were lost or duplicated: %+v", got.Buckets)
	}
	first := got.Buckets[0]
	if first.Primary.UsedPercent == nil || *first.Primary.UsedPercent != 0 || *first.Primary.WindowDurationMins != 300 || *first.Secondary.UsedPercent != 80 || *first.Secondary.ResetsAt != 1731547200 {
		t.Fatalf("window data changed: %+v", first)
	}
	if len(c.calls) != 2 || c.calls[0].method != "account/read" || c.calls[1].method != "account/rateLimits/read" {
		t.Fatalf("unexpected calls: %+v", c.calls)
	}
	data, _ := json.Marshal(got)
	if strings.Contains(string(data), "private@example.com") {
		t.Fatal("account email leaked into the usage snapshot")
	}
}

func TestProviderUsageLegacyAndMissingFields(t *testing.T) {
	tests := []struct {
		name, input, status string
		buckets             int
	}{
		{"legacy", `{"rateLimits":{"limitId":"codex","primary":{"usedPercent":25}}}`, "available", 1},
		{"empty", `{}`, "unavailable", 0},
		{"null", `{"rateLimits":null,"rateLimitsByLimitId":null}`, "unavailable", 0},
		{"empty windows", `{"rateLimits":{"primary":{},"secondary":null}}`, "unavailable", 1},
		{"invalid numbers", `{"rateLimits":{"primary":{"usedPercent":-1,"windowDurationMins":0,"resetsAt":-5}}}`, "unavailable", 1},
		{"map authoritative", `{"rateLimits":{"primary":{"usedPercent":25}},"rateLimitsByLimitId":{}}`, "unavailable", 0},
		{"null bucket", `{"rateLimitsByLimitId":{"missing":null}}`, "unavailable", 0},
		{"malformed", `{"rateLimits":{"primary":{"usedPercent":"unknown"}}}`, "unavailable", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _ := usageTestApp(t, `{"account":{"type":"chatgpt"}}`, tt.input)
			got := a.ProviderUsage("codex")
			if got.Status != tt.status || len(got.Buckets) != tt.buckets || got.URL != codexUsagePage {
				t.Fatalf("incorrect missing-data handling: %+v", got)
			}
			if tt.name == "legacy" {
				window := got.Buckets[0].Primary
				if window.WindowDurationMins != nil || window.ResetsAt != nil {
					t.Fatal("missing fields were invented")
				}
			}
		})
	}
}

func TestProviderUsageAccountRouting(t *testing.T) {
	tests := []struct {
		name, account, status, url string
	}{
		{"api key", `{"account":{"type":"apiKey"}}`, "external", openAIUsagePage},
		{"signed out", `{"account":null,"requiresOpenaiAuth":true}`, "unavailable", codexUsagePage},
		{"custom provider", `{"account":null,"requiresOpenaiAuth":false}`, "unavailable", ""},
		{"missing account", `{}`, "unavailable", ""},
		{"bedrock", `{"account":{"type":"amazonBedrock"}}`, "unavailable", ""},
		{"unknown", `{"account":{}}`, "unavailable", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, c := usageTestApp(t, tt.account, `{}`)
			got := a.ProviderUsage("codex")
			if got.Status != tt.status || got.URL != tt.url || len(got.Buckets) != 0 || len(c.calls) != 1 {
				t.Fatalf("incorrect account routing: %+v; calls %+v", got, c.calls)
			}
		})
	}
}

func TestProviderUsageErrorsNeverFallBackToDevin(t *testing.T) {
	for _, method := range []string{"account/read", "account/rateLimits/read"} {
		t.Run(method, func(t *testing.T) {
			a, c := usageTestApp(t, `{"account":{"type":"chatgpt"}}`, `{}`)
			c.failMethod = method
			a.settings.UsagePage = "https://devin.example/usage"
			got := a.ProviderUsage("codex")
			if got.Status != "unavailable" || len(got.Buckets) != 0 || strings.Contains(got.URL, "devin") || strings.Contains(got.Detail, "private server") {
				t.Fatalf("unsafe error response: %+v", got)
			}
		})
	}
	a, c := usageTestApp(t, `{}`, `{}`)
	a.settings.UsagePage = "https://devin.example/custom"
	got := a.ProviderUsage("devin")
	if got.Status != "external" || got.URL != a.settings.UsagePage || len(c.calls) != 0 {
		t.Fatalf("Devin usage changed: %+v", got)
	}
	for _, provider := range []string{"claude", "ollama", "unknown"} {
		got := a.ProviderUsage(provider)
		if got.Status != "unavailable" || got.URL != "" || len(c.calls) != 0 {
			t.Fatalf("unsupported provider fell through: %+v", got)
		}
	}
	if err := a.OpenProviderUsage("unknown"); err == nil {
		t.Fatal("opening unsupported usage should explain its unavailability")
	}
}
