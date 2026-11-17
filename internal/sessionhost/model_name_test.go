package sessionhost

import (
	"encoding/json"
	"testing"

	"studio/internal/acp"
)

// modelOptions is a model option whose list names two models, in groups as
// some agents send it, with the given value selected.
func modelOptions(value string) []acp.ConfigOption {
	return []acp.ConfigOption{{ID: "model", Category: "model", Type: "select", CurrentValue: json.RawMessage(`"` + value + `"`),
		Options: json.RawMessage(`[{"name":"Models","options":[{"value":"m-a","name":"Model A"},{"value":"m-b","name":"Model B"}]},{"value":"unnamed"}]`)}}
}

// A chat's information has the name of the model it uses now, and a change of
// model sends the information again, so the Chats list and the Org Chart show
// the new one. A change that leaves the model as it was sends only the options.
func TestChatInformationHasItsModelAndAChangeOfModelSendsItAgain(t *testing.T) {
	c := NewConversation(Identity{ID: "chat"}, InitialActivity{Open: true}, false, false)
	var events []string
	var last Info
	o := NewOptions(OptionsHost{
		Save: func(*Conversation, []acp.ConfigOption, map[string]json.RawMessage) error { return nil },
		Emit: func(name string, data any) {
			events = append(events, name)
			if info, ok := data.(Info); ok {
				last = info
			}
		},
	})
	if err := o.Update(c, modelOptions("m-a"), false); err != nil {
		t.Fatal(err)
	}
	if c.InfoVersioned().Model != "Model A" || len(events) != 2 || events[0] != EventOptions || events[1] != EventSession || last.Model != "Model A" {
		t.Fatalf("the first model was not sent with the chat's information: %v %+v", events, last)
	}
	events = nil
	if err := o.Update(c, modelOptions("m-a"), false); err != nil || len(events) != 1 || events[0] != EventOptions {
		t.Fatalf("options that leave the model as it was sent the information again: %v %v", events, err)
	}
	events = nil
	if err := o.Update(c, modelOptions("m-b"), false); err != nil || len(events) != 2 || last.Model != "Model B" {
		t.Fatalf("a change of model was not sent: %v %+v %v", events, last, err)
	}
	// A value the agent's list gives no name is left out rather than shown as its code.
	events = nil
	if err := o.Update(c, modelOptions("unnamed"), false); err != nil || len(events) != 2 || last.Model != "" {
		t.Fatalf("an unnamed model was shown: %v %+v %v", events, last, err)
	}
	data, _ := json.Marshal(c.InfoVersioned())
	var fields map[string]any
	if json.Unmarshal(data, &fields) != nil {
		t.Fatal(string(data))
	}
	if _, present := fields["model"]; present {
		t.Fatalf("the information has a model field with no model: %s", data)
	}
}
