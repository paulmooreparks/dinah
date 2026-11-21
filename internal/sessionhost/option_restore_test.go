package sessionhost

import (
	"encoding/json"
	"errors"
	"reflect"
	"studio/internal/acp"
	"sync"
	"testing"
	"time"
)

func TestClaudeEquivalentModelRejectsAmbiguousOrUnrecordedAliases(t *testing.T) {
	option := acp.ConfigOption{ID: "model", Type: "select", Options: json.RawMessage(`[{"value":"one","name":"Fable 5.1"},{"value":"two","name":"Fable 5.1"}]`)}
	r := NewReopening("claude", nil, nil, func(s string) string { return s })
	if _, ok := r.equivalentModel(option, json.RawMessage(`"Fable 5.1"`)); ok {
		t.Fatal("ambiguous model name was selected")
	}
	option.Options = json.RawMessage(`[{"value":"claude-fable-5-1","name":"Fable 5.1"}]`)
	if _, ok := r.equivalentModel(option, json.RawMessage(`"fable"`)); ok {
		t.Fatal("an alias mapping was guessed without provider-supplied names")
	}
}

func TestReopeningCopiesCatalogAndSubstitutionBytes(t *testing.T) {
	subs := []OptionSubstitution{{Option: "model", From: json.RawMessage(` "old" `), To: json.RawMessage(` "new" `), FromName: "Old", ToName: "New"}}
	catalog := []acp.ConfigOption{{ID: "model", CurrentValue: json.RawMessage(`"old"`), Options: json.RawMessage(`[{"value":"old","name":"Old catalog"}]`)}}
	labels := 0
	r := NewReopening("codex", subs, catalog, func(provider string) string {
		labels++
		if provider != "codex" {
			t.Fatal(provider)
		}
		return "Codex"
	})
	subs[0].From[2] = 'x'
	subs[0].To[2] = 'x'
	subs[0].FromName = "changed"
	catalog[0].Options[0] = 'x'
	catalog[0].CurrentValue[1] = 'x'
	catalog[0].ID = "changed"
	if labels != 0 || r.SavedName("model", json.RawMessage(`"old"`)) != "Old catalog" {
		t.Fatal("reopening changed with its inputs")
	}
	option := acp.ConfigOption{ID: "model", Type: "select", Options: json.RawMessage(`[{"value":"new","name":"New"}]`)}
	sub, ok := r.substitute(option, json.RawMessage(`"old"`))
	if !ok || string(sub.From) != ` "old" ` || string(sub.To) != ` "new" ` || labels != 1 {
		t.Fatalf("substitution %#v %v %d", sub, ok, labels)
	}
	if r.catalog[0].ID != "model" || string(r.catalog[0].CurrentValue) != `"old"` {
		t.Fatal("catalog was not copied")
	}
}

func TestRestoreModelFirstUsesUpdatedCatalogAndConfirmsEachChoice(t *testing.T) {
	for _, confirm := range []bool{true, false} {
		var calls []string
		opts := []acp.ConfigOption{
			{ID: "effort", Type: "select", CurrentValue: json.RawMessage(`"low"`), Options: json.RawMessage(`[{"value":"low","name":"Low"}]`)},
			{ID: "model", Category: "model", Type: "select", CurrentValue: json.RawMessage(`"old"`), Options: json.RawMessage(`[{"value":"new","name":"New"}]`)},
		}
		o := NewOptions(OptionsHost{Call: func(_ Connection, _ string, in, out any) error {
			params := in.(acp.SetConfigOptionParams)
			calls = append(calls, params.ConfigID)
			if params.ConfigID == "model" {
				if confirm {
					opts[1].CurrentValue = json.RawMessage(`"new"`)
				}
				opts[0].Options = json.RawMessage(`[{"value":"high","name":"High"}]`)
			} else {
				opts[0].CurrentValue = json.RawMessage(`"high"`)
			}
			out.(*acp.ConfigOptionsResult).ConfigOptions = opts
			return nil
		}})
		got, applied, err := o.Restore(nil, "chat", opts, map[string]json.RawMessage{"effort": json.RawMessage(`"high"`), "model": json.RawMessage(`"new"`)}, nil)
		if confirm {
			if err != nil || len(got) != 2 || applied != nil || !reflect.DeepEqual(calls, []string{"model", "effort"}) {
				t.Fatalf("restore %v %v %v %v", got, applied, err, calls)
			}
		} else if err == nil || err.Error() != "the provider did not restore this chat's model setting" || got != nil || applied != nil || !reflect.DeepEqual(calls, []string{"model"}) {
			t.Fatalf("confirmation %v %v %v", got, err, calls)
		}
	}
}

func TestRestoreProviderFailureRetainsWrappedError(t *testing.T) {
	failure := errors.New("provider failure")
	o := NewOptions(OptionsHost{Call: func(Connection, string, any, any) error { return failure }})
	opts := []acp.ConfigOption{{ID: "model", Type: "select", CurrentValue: json.RawMessage(`"old"`), Options: json.RawMessage(`[{"value":"new"}]`)}}
	_, _, err := o.Restore(nil, "chat", opts, map[string]json.RawMessage{"model": json.RawMessage(`"new"`)}, nil)
	if !errors.Is(err, failure) || err.Error() != "restore this chat's model setting: provider failure" {
		t.Fatal(err)
	}
}

// One limit covers every request of a restore. The setting whose request is
// not answered and the ones after it are left at the agent's values with one
// note, no further request is sent, and the answer that comes late is read
// and goes nowhere.
func TestRestoreLeavesSettingsUnrestoredWhenTheAgentDoesNotAnswer(t *testing.T) {
	for _, reopened := range []bool{true, false} {
		gate := make(chan struct{})
		answered := make(chan struct{})
		var mu sync.Mutex
		var calls []string
		opts := []acp.ConfigOption{
			{ID: "effort", Name: "Effort", Type: "select", CurrentValue: json.RawMessage(`"low"`), Options: json.RawMessage(`[{"value":"low","name":"Low"},{"value":"high","name":"High"}]`)},
			{ID: "model", Category: "model", Type: "select", CurrentValue: json.RawMessage(`"old"`), Options: json.RawMessage(`[{"value":"old","name":"Old"},{"value":"new","name":"New"}]`)},
		}
		o := NewOptions(OptionsHost{Call: func(_ Connection, _ string, in, out any) error {
			mu.Lock()
			calls = append(calls, in.(acp.SetConfigOptionParams).ConfigID)
			mu.Unlock()
			<-gate
			out.(*acp.ConfigOptionsResult).ConfigOptions = []acp.ConfigOption{{ID: "model", CurrentValue: json.RawMessage(`"new"`)}}
			close(answered)
			return nil
		}})
		o.RestoreLimit = 30 * time.Millisecond
		var reopen *Reopening
		if reopened {
			reopen = NewReopening("codex", nil, nil, func(s string) string { return s })
		}
		got, applied, err := o.Restore(nil, "chat", opts, map[string]json.RawMessage{"effort": json.RawMessage(`"high"`), "model": json.RawMessage(`"new"`)}, reopen)
		if err != nil {
			t.Fatalf("the restore failed: %v", err)
		}
		if !reflect.DeepEqual(got, opts) {
			t.Fatalf("the options are %v, and the agent's were %v", got, opts)
		}
		if len(applied) != 2 || !applied[0].Unanswered() || !applied[1].Unanswered() || applied[0].Option != "model" || applied[1].Option != "effort" {
			t.Fatalf("the settings left unrestored are %+v", applied)
		}
		if string(applied[0].From) != `"new"` || string(applied[0].To) != `"old"` || string(applied[1].From) != `"high"` || string(applied[1].To) != `"low"` {
			t.Fatalf("the saved and the agent's values are %+v", applied)
		}
		want := "The agent did not answer the request to restore these settings of this chat: model (new), Effort (high) within 30ms, so they are left at the agent's values."
		if reopened {
			want += " The chat keeps its choice and asks again the next time it is opened."
		}
		if applied[0].Note() != want || applied[1].Note() != "" {
			t.Fatalf("the notes are %q and %q", applied[0].Note(), applied[1].Note())
		}
		mu.Lock()
		sent := append([]string(nil), calls...)
		mu.Unlock()
		if !reflect.DeepEqual(sent, []string{"model"}) {
			t.Fatalf("the agent was sent %v", sent)
		}
		close(gate)
		<-answered
		if string(got[1].CurrentValue) != `"old"` {
			t.Fatal("the late answer changed the options the restore returned")
		}
	}
}

// One setting left unrestored is named with the value the chat has instead.
func TestRestoreNotesOneUnansweredSettingWithItsValue(t *testing.T) {
	gate := make(chan struct{})
	defer close(gate)
	o := NewOptions(OptionsHost{Call: func(Connection, string, any, any) error { <-gate; return nil }})
	o.RestoreLimit = 30 * time.Millisecond
	opts := []acp.ConfigOption{{ID: "model", Name: "Model", Category: "model", Type: "select", CurrentValue: json.RawMessage(`"old"`), Options: json.RawMessage(`[{"value":"old","name":"Old"},{"value":"new","name":"New"}]`)}}
	_, applied, err := o.Restore(nil, "chat", opts, map[string]json.RawMessage{"model": json.RawMessage(`"new"`)}, nil)
	if err != nil || len(applied) != 1 {
		t.Fatalf("restore %+v %v", applied, err)
	}
	if want := "The agent did not answer the request to restore this chat's Model (new) setting within 30ms, so it is left at Old."; applied[0].Note() != want {
		t.Fatalf("the note is %q", applied[0].Note())
	}
}

// The limit for a restore stays within the thirty seconds first asked for.
func TestRestoreLimitIsThirtySecondsOrLessAndPositive(t *testing.T) {
	if RestoreSettingsLimit <= 0 || RestoreSettingsLimit > 30*time.Second {
		t.Fatalf("the restore limit is %s", RestoreSettingsLimit)
	}
}
