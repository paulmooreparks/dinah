package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/claudeagent"
	"studio/internal/ollamaagent"
	"studio/internal/profile"
	"studio/internal/sessionhost"
)

// Every provider the product offers has a row, whose name and facts are
// values the page knows how to read.
func TestEveryOfferedProviderHasACompleteRow(t *testing.T) {
	for _, id := range product.Providers {
		row, ok := providerRegistry[id]
		if !ok || row.Name == "" {
			t.Errorf("%s has no row or no name", id)
			continue
		}
		if !slices.Contains([]string{"initialize", "adapter", "none"}, row.ImageInput) {
			t.Errorf("%s: image input %q", id, row.ImageInput)
		}
		if !slices.Contains([]string{"deletes", "hides"}, row.Removal) {
			t.Errorf("%s: removal %q", id, row.Removal)
		}
		if !slices.Contains([]string{"read", "link", "none"}, row.Usage) {
			t.Errorf("%s: usage %q", id, row.Usage)
		}
	}
}

// AgentProviders lists the providers in the product's order, with exactly one
// default, the words for each status code, and the facts of its row.
func TestAgentProvidersGivesOrderDefaultWordsAndFacts(t *testing.T) {
	a, _ := providerTestApp(t)
	list := a.AgentProviders()
	var ids []string
	defaults := 0
	for _, p := range list {
		ids = append(ids, p.ID)
		if p.Default {
			defaults++
		}
		if p.StatusWords != statusWordsFor(p.Status) {
			t.Errorf("%s: status %q has the words %q", p.ID, p.Status, p.StatusWords)
		}
		if p.Facts.ImageInput != providerRegistry[p.ID].ImageInput || p.Name != providerRegistry[p.ID].Name {
			t.Errorf("%s: its entry does not follow its row: %+v", p.ID, p)
		}
	}
	if !slices.Equal(ids, product.Providers) {
		t.Fatalf("the order is %v, not %v", ids, product.Providers)
	}
	if defaults != 1 {
		t.Fatalf("%d providers are the default", defaults)
	}
	// The default follows the person's choice.
	if err := a.SetDefaultProvider("claude"); err != nil {
		t.Fatal(err)
	}
	for _, p := range a.AgentProviders() {
		if p.Default != (p.ID == "claude") {
			t.Errorf("%s: default %v after choosing Claude", p.ID, p.Default)
		}
	}
}

// Removal is the declared value until the provider has answered initialize, and
// then what it reports.
func TestRemovalIsReportedOnceTheProviderHasAnswered(t *testing.T) {
	a, _ := providerTestApp(t)
	// Codex is declared as hiding, and answers without delete.
	if got := a.providerFacts("codex").Removal; got != "hides" {
		t.Fatalf("codex answered without delete and the facts say %q", got)
	}
	initial := a.runtime.ProviderLocked("codex").Initialization()
	initial.AgentCapabilities.SessionCapabilities.Delete = &struct{}{}
	setTestInitialization(a, "codex", initial)
	if got := a.providerFacts("codex").Removal; got != "deletes" {
		t.Fatalf("codex now reports delete and the facts say %q", got)
	}
	// Claude has not answered, so its declared value applies.
	if got := a.providerFacts("claude").Removal; got != "deletes" {
		t.Fatalf("claude has not answered and the facts say %q", got)
	}
}

// Ollama not answering has a code of its own, and one table turns every code
// into words.
func TestOllamaNotAnsweringIsNotRunning(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "127.0.0.1:1")
	var p AgentProviderInfo
	p.ID = "ollama"
	ollamaInfo(&p)
	if p.Status != "not-running" || statusWordsFor(p.Status) != "Not Running" {
		t.Fatalf("status %q, words %q", p.Status, statusWordsFor(p.Status))
	}
	for code, want := range map[string]string{"detected": "", "not-installed": "Not Installed", "adapter-missing": "Not Set Up", "runtime-missing": "Not Set Up", "unsupported": "Not Offered"} {
		if got := statusWordsFor(code); got != want {
			t.Errorf("%s: %q, not %q", code, got, want)
		}
	}
}

// What the adapter that Dinah ships says in its answer to initialize agrees
// with its row. Ollama's adapter can be started in a test; Codex's is a
// program's own and is not checked here.
func TestTheOllamaAdapterAgreesWithItsRow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			io.WriteString(w, `{"version":"0.34.1"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c, err := ollamaagent.Start(srv.URL, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var init acp.InitializeResult
	if err := c.Call(context.Background(), "initialize", acp.InitializeParams{ProtocolVersion: 1}, &init); err != nil {
		t.Fatal(err)
	}
	row := providerRegistry["ollama"]
	if deletes := init.AgentCapabilities.SessionCapabilities.Delete != nil; deletes != (row.Removal == "deletes") {
		t.Errorf("the adapter deletes %v and its row says %q", deletes, row.Removal)
	}
	// It states images for the agent as a whole, but the answer is per model, so
	// the row sends the question to the adapter.
	if !init.AgentCapabilities.PromptCapabilities.Image || row.ImageInput != "adapter" {
		t.Errorf("image: %+v, row %q", init.AgentCapabilities.PromptCapabilities, row.ImageInput)
	}
}

// The page is told again when a provider becomes ready, since the removal it
// reports may replace the declared one, and when the default changes.
func TestProviderFactsAreSentAgainWhenReadyAndWhenTheDefaultChanges(t *testing.T) {
	a, _ := providerTestApp(t)
	var sent [][]map[string]any
	a.sink = func(name string, data any) {
		if name == "providers:facts" {
			sent = append(sent, data.([]map[string]any))
		}
	}
	a.emitProviderEvent(sessionhost.EventState, map[string]any{"provider": "codex", "state": "starting"})
	if len(sent) != 0 {
		t.Fatal("the facts were sent for a provider that is not ready")
	}
	a.emitProviderEvent(sessionhost.EventState, map[string]any{"provider": "codex", "state": "ready"})
	if len(sent) != 1 || len(sent[0]) != len(product.Providers) {
		t.Fatalf("a provider becoming ready sent %d lists", len(sent))
	}
	if err := a.SetDefaultProvider("claude"); err != nil {
		t.Fatal(err)
	}
	last := sent[len(sent)-1]
	for _, entry := range last {
		if entry["default"] != (entry["id"] == "claude") {
			t.Errorf("after choosing Claude: %+v", entry)
		}
	}
}

// How a provider's usage is shown, and whether it has prices, follow its row and
// not its name.
func TestUsageAndFeaturesFollowTheRow(t *testing.T) {
	a, _ := providerTestApp(t)
	if d := a.ProviderUsage("ollama").Detail; !strings.Contains(d, "no account usage") {
		t.Errorf("ollama: %q", d)
	}
	if d := a.ProviderUsage("claude").Detail; !strings.Contains(d, product.Name) || strings.Contains(d, "{product}") {
		t.Errorf("claude: %q", d)
	}
	if d := a.ProviderUsage("someone-else").Detail; d != "Account usage is not available for this provider." {
		t.Errorf("a provider with no row: %q", d)
	}
	if d := a.ProviderUsage("devin"); d.Status != "unavailable" || !strings.Contains(d.Detail, "Devin usage page") {
		t.Errorf("a provider whose usage is a link: %+v", d)
	}
	if got := a.ModelPrices("codex"); len(got) != 0 {
		t.Errorf("prices for a provider that has none: %v", got)
	}
}

// A chat's provider comes from the prefix in its row, and a chat with none, or
// with no provider, is the legacy provider's.
func TestAChatsProviderComesFromItsSessionPrefix(t *testing.T) {
	seen := map[string]string{}
	for id, row := range providerRegistry {
		if row.SessionPrefix == "" {
			if id != profile.LegacyProvider {
				t.Errorf("%s has no session prefix, and only the legacy provider may have none", id)
			}
			continue
		}
		if other, ok := seen[row.SessionPrefix]; ok {
			t.Errorf("%s and %s share the prefix %q", id, other, row.SessionPrefix)
		}
		seen[row.SessionPrefix] = id
		if got := providerForSession(row.SessionPrefix + "abc"); got != id {
			t.Errorf("%sabc is %q, not %q", row.SessionPrefix, got, id)
		}
	}
	for _, id := range []string{"abc", "01234567-89ab", ""} {
		if got := providerForSession(id); got != profile.LegacyProvider {
			t.Errorf("%q is %q, not the legacy provider", id, got)
		}
	}
	if normalizedProvider("") != profile.LegacyProvider || normalizedProvider("claude") != "claude" {
		t.Error("a chat with no provider is not the legacy provider's")
	}
}

// Claude's prefix in the table is the one the adapter gives its sessions.
func TestClaudesPrefixIsTheAdaptersOwn(t *testing.T) {
	if providerRegistry["claude"].SessionPrefix != claudeagent.Prefix {
		t.Fatalf("the table says %q and the adapter %q", providerRegistry["claude"].SessionPrefix, claudeagent.Prefix)
	}
}

// Each provider's own work is reached through its module: a provider with no
// module is not supported, and one that signs in outside Dinah says so.
func TestProviderModulesAreReachedByTheirIDs(t *testing.T) {
	for _, id := range product.Providers {
		if _, ok := providerModules[id]; !ok {
			t.Errorf("%s has no module", id)
		}
	}
	a, _ := providerTestApp(t)
	if err := a.AgentSignIn("nobody"); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("a provider with no module: %v", err)
	}
	if err := prepareProviderApplets("nobody", t.TempDir()); err == nil {
		t.Error("applets were prepared for a provider with no module")
	}
	for id, word := range map[string]string{"ollama": "no sign-in", "claude": "never signs you in"} {
		if err := a.AgentSignIn(id); err == nil || !strings.Contains(err.Error(), word) {
			t.Errorf("%s: %v", id, err)
		}
	}
	// A provider that is installed but cannot be asked has its own session
	// parameters only when its module gives them.
	base := acp.LoadSessionParams{SessionID: "x", Cwd: t.TempDir()}
	for _, id := range []string{"codex", "ollama"} {
		if _, plain := a.providerSessionParams(id, "x", base.Cwd, nil).(acp.LoadSessionParams); plain {
			t.Errorf("%s has its own session parameters", id)
		}
	}
	if _, plain := a.providerSessionParams("claude", "x", base.Cwd, nil).(acp.LoadSessionParams); !plain {
		t.Error("a provider with no module parameters has more than the protocol's")
	}
	if _, fresh := a.providerSessionParams("claude", "", base.Cwd, nil).(acp.NewSessionParams); !fresh {
		t.Error("a new session has no new-session parameters")
	}
	// A refused request of a provider with no module of its own for it says to
	// sign in outside, by name.
	if _, ok := providerModules["claude"]; !ok || providerModules["claude"].AuthRequired != nil {
		t.Error("claude is expected to sign in outside Dinah")
	}
}

// A chosen executable that does not work is reported by the provider's own
// module, in the status code the words are made from.
func TestAnInvalidChosenExecutableIsReportedByTheProvidersModule(t *testing.T) {
	a, _ := providerTestApp(t)
	missing := filepath.Join(t.TempDir(), "no-such-program")
	if err := a.updateSettings(func(s *Settings) error {
		s.ProviderExecutables = map[string]string{"codex": missing, "devin": missing}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, p := range a.AgentProviders() {
		if p.ID != "codex" && p.ID != "devin" {
			continue
		}
		seen++
		if p.Status != "invalid-path" || p.Available || p.StatusWords != "Not Installed" || !strings.Contains(p.Detail, "Choose another executable") {
			t.Errorf("%s: %+v", p.ID, p)
		}
	}
	if seen != 2 {
		t.Fatalf("saw %d of the two providers", seen)
	}
}

// The usage snapshot carries the words for the account and for the link, so
// the page spells no provider's account.
func TestUsageSnapshotsCarryTheirOwnWords(t *testing.T) {
	for _, tc := range []struct {
		in                      ProviderUsageSnapshot
		name, account, linkText string
	}{
		{ProviderUsageSnapshot{AccountType: "apiKey", URL: "https://example.test/a"}, "Codex", "API Key (Separate Billing)", "Open OpenAI API Usage"},
		{ProviderUsageSnapshot{AccountType: "chatgpt", URL: "https://example.test/b"}, "Codex", "ChatGPT", "Open Codex Usage Page"},
		{ProviderUsageSnapshot{AccountType: "enterprise"}, "Codex", "enterprise", ""},
		{ProviderUsageSnapshot{URL: "https://example.test/c"}, "Devin", "", "Open Devin Usage Page"},
		{ProviderUsageSnapshot{}, "Claude", "", ""},
	} {
		got := finishUsage(tc.in, tc.name)
		if got.AccountLabel != tc.account || got.LinkLabel != tc.linkText {
			t.Errorf("%+v: account %q and link %q, not %q and %q", tc.in, got.AccountLabel, got.LinkLabel, tc.account, tc.linkText)
		}
	}
}
