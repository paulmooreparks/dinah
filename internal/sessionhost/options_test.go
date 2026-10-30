package sessionhost

import (
	"encoding/json"
	"errors"
	"reflect"
	"studio/internal/acp"
	"testing"
)

func TestOptionsFinishSavesAndEmitsBeforeClearingRestore(t *testing.T) {
	for _, fail := range []bool{false, true} {
		c := NewConversation(Identity{ID: "chat"}, InitialActivity{Open: true}, false, true)
		failure := errors.New("disk failed")
		var order []string
		opts := []acp.ConfigOption{{ID: "model", CurrentValue: json.RawMessage(`"model-a"`)}}
		o := NewOptions(OptionsHost{
			Save: func(s *Conversation, got []acp.ConfigOption, _ map[string]json.RawMessage) error {
				order = append(order, "save")
				loading, _ := s.OptionForChange("model")
				if !loading || !reflect.DeepEqual(s.Options(), opts) || !reflect.DeepEqual(got, opts) {
					t.Fatal("save state order")
				}
				if fail {
					return failure
				}
				return nil
			},
			Emit: func(name string, data any) {
				order = append(order, "emit")
				loading, _ := c.OptionForChange("model")
				if !loading || name != "agent:options" {
					t.Fatal("event state order")
				}
				event := data.(map[string]any)
				if fail && event["error"] != "The chat's provider settings could not be saved: disk failed" {
					t.Fatal(event)
				}
			},
		})
		err := o.Finish(c, opts)
		loading, _ := c.OptionForChange("model")
		if loading != fail || (fail && err != failure) || (!fail && err != nil) || !reflect.DeepEqual(order, []string{"save", "emit"}) {
			t.Fatalf("finish %v %v %v", loading, err, order)
		}
	}
}

func TestOptionsUpdateDuringRestoreSkipsPersistence(t *testing.T) {
	c := NewConversation(Identity{ID: "chat"}, InitialActivity{}, false, true)
	events := 0
	o := NewOptions(OptionsHost{Save: func(*Conversation, []acp.ConfigOption, map[string]json.RawMessage) error {
		t.Fatal("saved intermediate restore")
		return nil
	}, Emit: func(string, any) { events++ }})
	if err := o.Update(c, nil, false); err != nil || events != 1 {
		t.Fatalf("update %v %d", err, events)
	}
}

func TestOptionsSetUsesBooleanWireTypeAndPersistsAfterProvider(t *testing.T) {
	c := NewConversation(Identity{ID: "chat"}, InitialActivity{Open: true}, false, false)
	option := acp.ConfigOption{ID: "flag", Type: "boolean", CurrentValue: json.RawMessage(`false`)}
	c.UpdateOptions([]acp.ConfigOption{option})
	conn := &lifecycleConnection{}
	var order []string
	o := NewOptions(OptionsHost{
		Conversation: func(string) *Conversation { return c }, Connection: func(*Conversation) Connection { return conn },
		Call: func(got Connection, method string, in, out any) error {
			order = append(order, "call")
			params := in.(acp.SetConfigOptionParams)
			if got != conn || method != "session/set_config_option" || params.SessionID != "chat" || params.ConfigID != "flag" || params.Type != "boolean" || string(params.Value.(json.RawMessage)) != "true" {
				t.Fatalf("call %v", params)
			}
			option.CurrentValue = json.RawMessage(`true`)
			out.(*acp.ConfigOptionsResult).ConfigOptions = []acp.ConfigOption{option}
			return nil
		},
		Save: func(*Conversation, []acp.ConfigOption, map[string]json.RawMessage) error {
			order = append(order, "save")
			return nil
		},
		Emit: func(string, any) { order = append(order, "emit") },
	})
	got, err := o.Set("chat", "flag", json.RawMessage(`true`))
	if err != nil || len(got) != 1 || !reflect.DeepEqual(order, []string{"call", "save", "emit"}) {
		t.Fatalf("set %v %v %v", got, err, order)
	}
}
