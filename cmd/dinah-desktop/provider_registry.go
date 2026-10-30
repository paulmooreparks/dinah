package main

import (
	"studio/internal/providers"
	"studio/internal/sessionhost"
)

// The table of providers is internal/providers, which the session code reads as
// well. These names keep the window's code short.
type providerRow = providers.Row

var providerRegistry = providers.Registry

func providerName(id string) string { return providers.Name(id) }

func statusWordsFor(code string) string { return providers.WordsFor(code) }

// ProviderFacts is what the page reads of a provider besides whether it is
// installed (providerRegistry).
type ProviderFacts struct {
	ImageInput       string `json:"imageInput"`
	Removal          string `json:"removal"`
	Prices           bool   `json:"prices"`
	Usage            string `json:"usage"`
	OwnCommands      bool   `json:"ownCommands"`
	TakesSkills      bool   `json:"takesSkills"`
	Executable       string `json:"executable"`
	ChooseExecutable bool   `json:"chooseExecutable"`
	SignIn           bool   `json:"signIn"`
}

// providerFacts is a provider's facts as they are now. Removal is the one the
// provider reports once it has answered initialize, and the declared one before.
func (a *App) providerFacts(id string) ProviderFacts {
	row := providerRegistry[id]
	removal := row.Removal
	a.mu.Lock()
	conn, init := a.providerInitLocked(id)
	a.mu.Unlock()
	if conn != nil {
		removal = "hides"
		if init.AgentCapabilities.SessionCapabilities.Delete != nil {
			removal = "deletes"
		}
	}
	return ProviderFacts{ImageInput: row.ImageInput, Removal: removal, Prices: row.Prices, Usage: row.Usage, OwnCommands: row.OwnCommands,
		TakesSkills: row.TakesSkills, Executable: row.Executable, ChooseExecutable: row.ChooseExecutable, SignIn: row.SignIn}
}

// providerFactsNow is the facts of every provider the product offers, with
// which one is the default, for the page to read again when a reported fact
// replaces a declared one or the default changes.
func (a *App) providerFactsNow() []map[string]any {
	a.mu.Lock()
	def := defaultProviderOf(a.settings)
	a.mu.Unlock()
	out := make([]map[string]any, 0, len(product.Providers))
	for _, id := range product.Providers {
		out = append(out, map[string]any{"id": id, "default": id == def, "facts": a.providerFacts(id)})
	}
	return out
}

// emitProviderFacts tells the page the facts as they are now.
func (a *App) emitProviderFacts() { a.emit("providers:facts", a.providerFactsNow()) }

// emitProviderEvent is how the provider lifecycle emits: a provider that has
// become ready has answered initialize, so its reported facts are sent again.
func (a *App) emitProviderEvent(name string, data any) {
	a.emit(name, data)
	if name != sessionhost.EventState {
		return
	}
	if m, ok := data.(map[string]any); ok && m["state"] == "ready" {
		a.emitProviderFacts()
	}
}
