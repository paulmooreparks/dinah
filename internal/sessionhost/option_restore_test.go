package sessionhost

import (
	"encoding/json"
	"errors"
	"reflect"
	"studio/internal/acp"
	"testing"
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
