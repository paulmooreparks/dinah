package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/acp"
)

// substitutionApp is a test app with one saved Codex chat whose saved model
// is "fable", and an agent that offers the models in offered, as value and
// name pairs.
func substitutionApp(t *testing.T, offered string) (*App, *testProviderConnection, string, string) {
	t.Helper()
	a, c := providerTestApp(t)
	c.options = []acp.ConfigOption{{ID: "model", Name: "Model", Category: "model", Type: "select", CurrentValue: json.RawMessage(`"default"`), Options: json.RawMessage(offered)}}
	id, ws := "codex:fable-chat", t.TempDir()
	a.settings.SessionProviders = map[string]string{id: "codex"}
	a.settings.SessionOptions = map[string]map[string]json.RawMessage{id: {"model": json.RawMessage(`"fable"`)}}
	// The catalog from an earlier session still knows the saved model's name.
	a.settings.ProviderCatalogs = map[string][]acp.ConfigOption{"codex": {{ID: "model", Name: "Model", Category: "model", Type: "select",
		CurrentValue: json.RawMessage(`"fable"`), Options: json.RawMessage(`[{"value":"fable","name":"Fable"},{"value":"default","name":"Default"}]`)}}}
	if err := saveSettings(a.settings); err != nil {
		t.Fatal(err)
	}
	return a, c, id, ws
}

const withoutFable = `[{"value":"default","name":"Default"},{"value":"opus","name":"Opus"},{"value":"claude-fable-5-1[1m]","name":"Fable"}]`

func storeFableSubstitution(t *testing.T, a *App, to string) {
	t.Helper()
	a.settings.OptionSubstitutions = OptionSubstitutions{"codex": {{Option: "model", OptionName: "Model", From: json.RawMessage(`"fable"`), FromName: "Fable", To: json.RawMessage(to), ToName: "Fable"}}}
	if err := saveSettings(a.settings); err != nil {
		t.Fatal(err)
	}
}

func modelOf(t *testing.T, a *App, id string) string {
	t.Helper()
	for _, o := range a.SessionOptions(id) {
		if o.ID == "model" {
			return string(o.CurrentValue)
		}
	}
	t.Fatal("the chat has no model option")
	return ""
}

func TestRefusalCarriesTheOfferedValuesWithTheSuggestionFirst(t *testing.T) {
	a, _, id, ws := substitutionApp(t, withoutFable)
	_, err := a.OpenSessionAt(ws, id)
	var refusal *unofferedOptionError
	if !errors.As(err, &refusal) || !strings.Contains(err.Error(), "no longer available") {
		t.Fatalf("the refusal is not the unoffered option's: %v", err)
	}
	d := refusal.ErrorDetail().(UnofferedOption)
	if d.Kind != "unoffered-option" || d.Provider != "codex" || d.Option != "model" || d.OptionName != "Model" || string(d.Saved) != `"fable"` || d.SavedName != "Fable" {
		t.Fatalf("detail %+v", d)
	}
	var values []string
	for _, v := range d.Offered {
		values = append(values, string(v.Value))
	}
	if strings.Join(values, ",") != `"claude-fable-5-1[1m]","default","opus"` || !d.Offered[0].Suggested || d.Offered[1].Suggested || d.Offered[2].Suggested {
		t.Fatalf("offered %+v", d.Offered)
	}
	// The page receives the detail with the message (internal/bridge).
	b, _ := json.Marshal(refusal.ErrorDetail())
	if !strings.Contains(string(b), `"offered":[{"value":"claude-fable-5-1[1m]","name":"Fable","suggested":true}`) {
		t.Fatalf("encoded detail %s", b)
	}
	if a.session(id) != nil {
		t.Fatal("a refused chat was left open")
	}
}

func TestStoredSubstitutionIsAppliedWhenTheSavedValueIsMissing(t *testing.T) {
	a, _, id, ws := substitutionApp(t, withoutFable)
	storeFableSubstitution(t, a, `"claude-fable-5-1[1m]"`)
	info, err := a.OpenSessionAt(ws, id)
	if err != nil {
		t.Fatal(err)
	}
	if got := modelOf(t, a, id); got != `"claude-fable-5-1[1m]"` {
		t.Fatalf("the substitute was not applied: %s", got)
	}
	if got := string(a.settings.SessionOptions[id]["model"]); got != `"fable"` {
		t.Fatalf("the chat's saved value was replaced: %s", got)
	}
	want := "Opened with Fable (claude-fable-5-1[1m]) because Fable wasn't offered; you set this in Settings › Agents › Codex."
	if len(info.Notes) != 1 || info.Notes[0] != want {
		t.Fatalf("notes %q", info.Notes)
	}
	// The agent repeating the substitute leaves the saved value as it was.
	if err := a.setOptions(a.session(id), a.SessionOptions(id)); err != nil {
		t.Fatal(err)
	}
	if got := string(a.settings.SessionOptions[id]["model"]); got != `"fable"` {
		t.Fatalf("an update replaced the saved value: %s", got)
	}
	// The person choosing a model makes it the chat's own.
	if _, err := a.SetSessionOption(id, "model", json.RawMessage(`"opus"`)); err != nil {
		t.Fatal(err)
	}
	if got := string(a.settings.SessionOptions[id]["model"]); got != `"opus"` {
		t.Fatalf("the person's choice was not saved: %s", got)
	}
}

// A Fable chat saved with fast mode reopens with its substituted model even
// though that model offers no fast mode, and a setting offered without the
// saved value takes the agent's value; both are said in the chat, and the
// chat keeps its saved choices (Paul, 28 October 2026).
func TestSettingsTheModelDoesNotOfferDoNotStopAReopen(t *testing.T) {
	a, c, id, ws := substitutionApp(t, withoutFable)
	c.options = append(c.options, acp.ConfigOption{ID: "effort", Name: "Effort", Type: "select", CurrentValue: json.RawMessage(`"medium"`),
		Options: json.RawMessage(`[{"value":"low","name":"Low"},{"value":"medium","name":"Medium"}]`)})
	a.settings.SessionOptions[id]["fast"] = json.RawMessage(`true`)
	a.settings.SessionOptions[id]["effort"] = json.RawMessage(`"max"`)
	if err := saveSettings(a.settings); err != nil {
		t.Fatal(err)
	}
	storeFableSubstitution(t, a, `"claude-fable-5-1[1m]"`)
	info, err := a.OpenSessionAt(ws, id)
	if err != nil {
		t.Fatalf("a setting the model does not offer stopped the reopen: %v", err)
	}
	if got := modelOf(t, a, id); got != `"claude-fable-5-1[1m]"` {
		t.Fatalf("the substitute was not applied: %s", got)
	}
	notes := strings.Join(info.Notes, "\n")
	if !strings.Contains(notes, "Opened with Fable") || !strings.Contains(notes, "fast setting (true) isn't offered") || !strings.Contains(notes, "Effort max isn't offered") {
		t.Fatalf("notes %q", info.Notes)
	}
	saved := a.settings.SessionOptions[id]
	if string(saved["fast"]) != `true` || string(saved["effort"]) != `"max"` || string(saved["model"]) != `"fable"` {
		t.Fatalf("the chat's saved choices were not kept: %s %s %s", saved["fast"], saved["effort"], saved["model"])
	}
}

func TestSavedValueIsUsedWhenOffered(t *testing.T) {
	a, _, id, ws := substitutionApp(t, `[{"value":"default","name":"Default"},{"value":"fable","name":"Fable"},{"value":"claude-fable-5-1[1m]","name":"Fable"}]`)
	storeFableSubstitution(t, a, `"claude-fable-5-1[1m]"`)
	info, err := a.OpenSessionAt(ws, id)
	if err != nil {
		t.Fatal(err)
	}
	if got := modelOf(t, a, id); got != `"fable"` || len(info.Notes) != 0 {
		t.Fatalf("the substitute was used although the saved value is offered: %s %q", got, info.Notes)
	}
}

func TestRefusalWhenNeitherValueIsOffered(t *testing.T) {
	a, _, id, ws := substitutionApp(t, withoutFable)
	storeFableSubstitution(t, a, `"retired-model"`)
	_, err := a.OpenSessionAt(ws, id)
	var refusal *unofferedOptionError
	if !errors.As(err, &refusal) || len(refusal.ErrorDetail().(UnofferedOption).Offered) != 3 {
		t.Fatalf("expected the refusal with Open With: %v", err)
	}
	if got := string(NewApp().settings.SessionOptions[id]["model"]); got != `"fable"` {
		t.Fatalf("the refusal changed the saved value: %s", got)
	}
}

func TestNewChatsAreNeverSubstituted(t *testing.T) {
	a, _ := providerTestApp(t)
	a.settings.ProviderDefaults = map[string]map[string]json.RawMessage{"codex": {"model": json.RawMessage(`"fable"`)}}
	a.settings.OptionSubstitutions = OptionSubstitutions{"codex": {{Option: "model", From: json.RawMessage(`"fable"`), To: json.RawMessage(`"changed"`)}}}
	_, err := a.NewSessionAt(t.TempDir())
	var refusal *unofferedOptionError
	if err == nil || errors.As(err, &refusal) {
		t.Fatalf("a new chat's default was substituted or offered Open With: %v", err)
	}
}

func TestOpenWithReopensWithTheChosenValueAndCanStoreIt(t *testing.T) {
	a, _, id, ws := substitutionApp(t, withoutFable)
	info, err := a.OpenSessionWith(ws, id, "model", json.RawMessage(`"opus"`), false)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Open || modelOf(t, a, id) != `"opus"` || string(a.settings.SessionOptions[id]["model"]) != `"opus"` {
		t.Fatalf("Open With did not make the choice the chat's own: %+v %s", info, a.settings.SessionOptions[id]["model"])
	}
	if len(a.settings.OptionSubstitutions) != 0 || len(info.Notes) != 0 {
		t.Fatal("Open With without Always stored a substitution")
	}

	b, _, other, ws2 := substitutionApp(t, withoutFable)
	if _, err := b.OpenSessionWith(ws2, other, "model", json.RawMessage(`"claude-fable-5-1[1m]"`), true); err != nil {
		t.Fatal(err)
	}
	subs := NewApp().settings.OptionSubstitutions["codex"]
	if len(subs) != 1 || subs[0].Option != "model" || string(subs[0].From) != `"fable"` || string(subs[0].To) != `"claude-fable-5-1[1m]"` || subs[0].FromName != "Fable" || subs[0].ToName != "Fable" || subs[0].OptionName != "Model" {
		t.Fatalf("stored %+v", subs)
	}
	if err := b.RemoveOptionSubstitution("codex", "model", json.RawMessage(`"fable"`)); err != nil {
		t.Fatal(err)
	}
	if got := NewApp().settings.OptionSubstitutions; len(got) != 0 {
		t.Fatalf("Remove left %+v", got)
	}
	if err := b.RemoveOptionSubstitution("codex", "model", json.RawMessage(`"fable"`)); err == nil {
		t.Fatal("removing a substitution twice succeeded")
	}
}

func TestSubstitutionsRoundTripAndInvalidEntriesAreIgnored(t *testing.T) {
	providerTestApp(t)
	s, err := loadSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.OptionSubstitutions = OptionSubstitutions{"claude": {{Option: "model", OptionName: "Model", From: json.RawMessage(`"fable"`), FromName: "Fable", To: json.RawMessage(`"claude-fable-5-1[1m]"`), ToName: "Fable"}}}
	if err := saveSettings(s); err != nil {
		t.Fatal(err)
	}
	back, err := loadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !sameJSON(back.OptionSubstitutions, s.OptionSubstitutions) {
		t.Fatalf("round trip %+v", back.OptionSubstitutions)
	}
	hand := `{"approvals":"devin","workspace":"","showThinking":false,"optionSubstitutions":{
		"claude":[
			{"option":"model","from":"fable","to":"claude-fable-5-1[1m]"},
			{"option":"model","from":"fable","to":"opus"},
			{"option":"","from":"a","to":"b"},
			{"option":"model","from":{"x":1},"to":"b"},
			{"option":"model","from":"same","to":"same"},
			{"option":7,"from":"a","to":"b"},
			"not an entry",
			{"option":"thinking","from":true,"to":false}
		],
		"codex":"not a list",
		"":[{"option":"model","from":"a","to":"b"}]
	}}`
	if err := os.WriteFile(filepath.Join(configDir(), "settings.json"), []byte(hand), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadSettings()
	if err != nil {
		t.Fatalf("a bad substitution stopped the settings loading: %v", err)
	}
	subs := got.OptionSubstitutions
	if len(subs) != 1 || len(subs["claude"]) != 2 || string(subs["claude"][0].To) != `"claude-fable-5-1[1m]"` || subs["claude"][1].Option != "thinking" {
		t.Fatalf("kept %+v", subs)
	}
	if err := os.WriteFile(filepath.Join(configDir(), "settings.json"), []byte(`{"approvals":"devin","optionSubstitutions":["wrong"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := loadSettings(); err != nil || got.OptionSubstitutions != nil {
		t.Fatalf("a substitutions field of the wrong shape: %+v %v", got.OptionSubstitutions, err)
	}
}
