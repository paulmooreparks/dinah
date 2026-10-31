// Package providers declares what differs between agent providers, in one
// table with one row for each. Shared code asks a row for a fact and does not
// compare with a provider's name: a provider's name, its session prefix and its
// facts are all here, so adding a provider is filling in one row, and the
// enforcement test (internal/profile/provider_names_test.go) lists the sites
// that still compare with a name.
package providers

import "strings"

// Row is everything the product declares about one agent provider.
type Row struct {
	Name string
	// ImageInput is where the answer to whether a chat's model reads pictures
	// comes from: "initialize" (promptCapabilities.image), "adapter" (the
	// adapter's own request for the model) or "none" (nothing documented says,
	// so the answer is unknown).
	ImageInput string
	// Removal is what removing a chat does at the provider: "deletes" or
	// "hides" (the chat stays with the provider). Once the provider has
	// answered initialize, sessionCapabilities.delete says it, and wins.
	Removal string
	// Prices says the provider's models have prices to show.
	Prices bool
	// Usage is how its account usage is shown: "read" (Dinah reads the
	// quotas), "link" (the person gives the page that shows them) or "none".
	Usage string
	// UsageNote is what the usage page says when there is nothing to show;
	// {product} stands for the product's name.
	UsageNote string
	// OwnCommands says the provider has commands of its own, which skills of
	// scope "provider" are offered for.
	OwnCommands bool
	// TakesSkills says skills can be shared to the provider.
	TakesSkills bool
	// Executable is the program Dinah starts for it as the person reads its
	// name, empty when there is none, and ChooseExecutable whether the person
	// may choose which one.
	Executable       string
	ChooseExecutable bool
	// SignIn says the provider has a sign-in step.
	SignIn bool
	// SessionPrefix begins the ID of every chat the provider makes. A chat whose
	// ID has none is the legacy provider's (profile.LegacyProvider).
	SessionPrefix string
	// ManagedServers says Dinah's applet servers are given to the provider in
	// the parameters of a chat's session. A provider without it reads them from
	// a registration file Dinah writes.
	ManagedServers bool
	// OwnPreferences says the provider's model catalog, option defaults and
	// pinned models are kept for it. The legacy provider's are the original
	// settings of the program, which every provider used before there were
	// others, and are kept where they were.
	OwnPreferences bool
	// HookSteering says a chat can be steered through the provider's hooks once
	// the hooks have tied the agent's session to it.
	HookSteering bool
	// SteerAccepted says a steering message is reported accepted when the agent
	// takes it, and not delivered, because the agent does not say when it is
	// read.
	SteerAccepted bool
	// JournalFolder is the folder of the journal's own, inside the journal
	// folder, in which the provider's chats keep their message times, with
	// their IDs encoded in the file names. It is empty for a provider whose
	// journals are in the journal folder itself.
	JournalFolder string
	// HookNamespace begins the session IDs of a provider that runs Dinah's
	// command steering hook, so its agent's session IDs do not meet another
	// provider's on the bus. It is empty for a provider that does not.
	HookNamespace string
	// MergedToolCalls says the provider streams a tool call as updates that its
	// history replay has merged, so its calls are recorded as the replay has
	// them and not one row for each event.
	MergedToolCalls bool
	// SnapshotToolCalls identifies adapters whose terminal tool updates replace
	// streamed output with the complete result, as their replay does.
	SnapshotToolCalls bool
	// FileLimit is what the provider's documentation says about the files in
	// one message. Its Source is empty when the provider has nothing to limit.
	FileLimit FileLimit
	// HandoffChars is how many characters a handoff to the provider may take,
	// from its own limit, and 0 for a provider whose window is not known.
	// HandoffFromContext says the budget follows the chat's context option
	// once the chat is open, with HandoffChars as the figure until then.
	HandoffChars       int
	HandoffFromContext bool
	// BudgetUnit is the unit the provider counts the cost of a call in, which a
	// budget for its judgement calls is set in: tokens for most, ACUs for Devin
	// (docs/plans/judgement-agents.md). Every row declares one.
	BudgetUnit string
	// RequestUsage says the provider's adapter reports what each request used, as a turn_cost update, which a team's budget counts;
	// a provider without it has its turns recorded as unknown, and none is estimated (docs/plans/teams.md).
	RequestUsage bool
}

// FileLimit is a provider's documented limit on the files in one message: the
// default it leads to, the most it documents (0 when it documents none), the
// sentence the card shows, in which {product} stands for the product's name,
// and the page it comes from.
type FileLimit struct {
	Default int
	Ceiling int
	Source  string
	URL     string
}

// Registry has a row for every provider the product may offer. The profile
// chooses which of them it offers, and in what order. Claude's prefix is the
// one in claudeagent.Prefix, which a test checks.
var Registry = map[string]Row{
	"devin": {Name: "Devin", BudgetUnit: "acu", ImageInput: "none", Removal: "deletes", Prices: true, Usage: "link", OwnCommands: true, TakesSkills: true,
		Executable: "Devin", ChooseExecutable: true, SignIn: true, HookSteering: true, HookNamespace: "devin:",
		FileLimit: FileLimit{Source: "Devin's documentation gives no limit on the files in one message, so there is none unless you set one. {product} sends what you attach and shows Devin's error if it refuses a file."}},
	"codex": {Name: "Codex", BudgetUnit: "tokens", RequestUsage: true, ImageInput: "initialize", Removal: "hides", Usage: "read", TakesSkills: true, Executable: "Codex", ChooseExecutable: true,
		SignIn: true, SessionPrefix: "codex:", ManagedServers: true, OwnPreferences: true, SteerAccepted: true, JournalFolder: "codex", SnapshotToolCalls: true,
		// 600,000 is under the app-server's 1,048,576-character request cap, leaving room for the turn that follows and for the thread to grow.
		HandoffChars: 600_000,
		FileLimit: FileLimit{Default: 20, Ceiling: 1500,
			Source: "OpenAI's images and vision guide allows up to 1,500 images and 512 MB in one request. The default of 20 is Paul's own choice, lower than that ceiling.",
			URL:    "https://developers.openai.com/api/docs/guides/images-vision"}},
	"claude": {Name: "Claude", BudgetUnit: "tokens", ImageInput: "initialize", Removal: "deletes", Usage: "none", Executable: "Node.js", ChooseExecutable: true,
		SessionPrefix: "claude:", ManagedServers: true, OwnPreferences: true, HookSteering: true, MergedToolCalls: true,
		// 400,000 is about 100,000 tokens, half of a 200,000-token window.
		HandoffChars: 400_000,
		FileLimit: FileLimit{Default: 20, Ceiling: 600,
			Source: "Anthropic's vision guide allows up to 600 images in one API request, and 100 for models with a 200k-token context window. When a request has more than 20 images, any image over 2000 px on a side is refused, and images sent in earlier turns count toward the 20. {product} reduces any picture over 2000 px on its longer side before Claude sees it.",
			URL:    "https://platform.claude.com/docs/en/build-with-claude/vision"},
		UsageNote: "Claude's plan limits cannot be read by {product}; see your usage in Claude Code or at claude.ai, or in the Claude Console for an API key."},
	"ollama": {Name: "Ollama", BudgetUnit: "tokens", RequestUsage: true, ImageInput: "adapter", Removal: "deletes", Usage: "none", SessionPrefix: "ollama:", ManagedServers: true, OwnPreferences: true,
		// 90,000 is for a chat whose context length is not known yet.
		HandoffChars: 90_000, HandoffFromContext: true,
		FileLimit: FileLimit{Source: "Ollama's API documentation gives no limit on the images in one request, so there is none unless you set one. {product} sends what you attach and shows Ollama's error if it refuses a file."},
		UsageNote: "Ollama reports no account usage. A model on this computer has no limit but its memory; a cloud model's usage is on your ollama.com account."},
}

// CommandHookNamespace is the namespace of the session IDs of the provider that
// runs Dinah's command steering hook, and "" when none does.
func CommandHookNamespace() string {
	for _, row := range Registry {
		if row.HookNamespace != "" {
			return row.HookNamespace
		}
	}
	return ""
}

// OwnCommandsProvider is the ID of the provider whose row says it has commands
// of its own, which the commands a session offers are the source of, and "" when
// none does.
func OwnCommandsProvider() string {
	for id, row := range Registry {
		if row.OwnCommands {
			return id
		}
	}
	return ""
}

// For is a provider's row, and the empty row for a provider that has none.
func For(id string) Row { return Registry[id] }

// Name is the name people read for a provider, empty when it has no row.
func Name(id string) string { return Registry[id].Name }

// StatusWords turn a provider's status code into the words that say what is
// wrong, the same for every provider: a provider only decides which code it
// reports. An empty string says nothing is wrong.
var StatusWords = map[string]string{
	"detected":        "",
	"not-installed":   "Not Installed",
	"invalid-path":    "Not Installed",
	"not-running":     "Not Running",
	"adapter-missing": "Not Set Up",
	"runtime-missing": "Not Set Up",
	"unsupported":     "Not Offered",
}

// WordsFor is the words for a status code, and for a code the table does not
// have, none.
func WordsFor(code string) string { return StatusWords[strings.TrimSpace(code)] }
