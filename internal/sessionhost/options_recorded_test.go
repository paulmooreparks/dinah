package sessionhost

import (
	"encoding/json"
	"errors"
	"testing"

	"studio/internal/acp"
)

func recordedOptionsFixture(t *testing.T) (*Options, **Conversation, *int) {
	t.Helper()
	chat := NewConversation(Identity{ID: "chat"}, InitialActivity{Open: true}, false, false)
	chat.UpdateOptions([]acp.ConfigOption{{ID: "model", Category: "model", Type: "select", CurrentValue: json.RawMessage(`"initial"`), Options: json.RawMessage(`[{"value":"initial","name":"Initial"},{"value":"task","name":"Task"},{"value":"person","name":"Person"}]`)}})
	calls := 0
	options := NewOptions(OptionsHost{
		Conversation: func(string) *Conversation { return chat },
		Connection:   func(*Conversation) Connection { return &lifecycleConnection{} },
		Call: func(_ Connection, method string, in, out any) error {
			if method != "session/set_config_option" {
				t.Fatal(method)
			}
			calls++
			rows := chat.Options()
			rows[0].CurrentValue = append(json.RawMessage(nil), in.(acp.SetConfigOptionParams).Value.(json.RawMessage)...)
			out.(*acp.ConfigOptionsResult).ConfigOptions = rows
			return nil
		},
		Save: func(*Conversation, []acp.ConfigOption, map[string]json.RawMessage) error { return nil },
		Emit: func(string, any) {},
	})
	return options, &chat, &calls
}

func TestRecordedModelChoiceCapturesBeforeDispatchAndRestoresConditionally(t *testing.T) {
	options, chat, calls := recordedOptionsFixture(t)
	var checkpoints []OptionChange
	_, receipt, err := options.SetRecorded("chat", "model", json.RawMessage(`"task"`), func(change OptionChange) error {
		if len(checkpoints) == 0 {
			if *calls != 0 || change.Applied || string(change.From) != `"initial"` {
				t.Fatal("native change preceded original-value receipt")
			}
			if (*chat).State().Ready || !(*chat).State().Working {
				t.Fatal("model change lacked exclusive admission")
			}
			if ready, _, _ := (*chat).AdmitSend(NewPrompt("Person's concurrent turn", nil)); ready {
				t.Fatal("turn overlapped model change")
			}
			if (*chat).BeginContinuation() || (*chat).BeginSwitch() {
				t.Fatal("provider switch overlapped model change")
			}
		}
		checkpoints = append(checkpoints, change)
		return nil
	})
	if err != nil || !receipt.Applied || len(checkpoints) != 2 || *calls != 1 || !(*chat).State().Ready || (*chat).State().Working {
		t.Fatalf("change: %+v %v", receipt, err)
	}
	if _, _, err := options.RestoreRecorded(receipt, nil); err != nil || *calls != 2 || string((*chat).Options()[0].CurrentValue) != `"initial"` {
		t.Fatalf("restore: %v", err)
	}
	if _, _, err := options.RestoreRecorded(receipt, nil); !errors.Is(err, ErrOptionsChanged) || *calls != 2 {
		t.Fatal("restoration receipt replayed")
	}
}

func TestRecordedModelChoiceNeverOverwritesNewerPersonChoiceOrRuntime(t *testing.T) {
	for _, mode := range []string{"different", "same", "away and back", "reconstructed"} {
		t.Run(mode, func(t *testing.T) {
			options, chat, calls := recordedOptionsFixture(t)
			_, receipt, err := options.SetRecorded("chat", "model", json.RawMessage(`"task"`), func(OptionChange) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "different":
				_, err = options.Set("chat", "model", json.RawMessage(`"person"`))
			case "same":
				_, err = options.Set("chat", "model", json.RawMessage(`"task"`))
			case "away and back":
				_, err = options.Set("chat", "model", json.RawMessage(`"person"`))
				if err == nil {
					_, err = options.Set("chat", "model", json.RawMessage(`"task"`))
				}
			case "reconstructed":
				rows := (*chat).Options()
				*chat = NewConversation(Identity{ID: "chat"}, InitialActivity{Open: true}, false, false)
				(*chat).UpdateOptions(rows)
			}
			if err != nil {
				t.Fatal(err)
			}
			before := *calls
			if _, _, err := options.RestoreRecorded(receipt, nil); !errors.Is(err, ErrOptionsChanged) || *calls != before {
				t.Fatalf("overwrote newer choice: %v, calls %d -> %d", err, before, *calls)
			}
		})
	}
}

func TestRecordedModelChoiceRefusesBeforeNativeWorkWhenReceiptCannotPersist(t *testing.T) {
	options, chat, calls := recordedOptionsFixture(t)
	_, receipt, err := options.SetRecorded("chat", "model", json.RawMessage(`"task"`), func(OptionChange) error { return errors.New("invented receipt failure") })
	if err == nil || receipt.Applied || *calls != 0 || !(*chat).State().Ready || (*chat).State().Working {
		t.Fatalf("failed receipt admitted native work or left admission held: %+v %v", receipt, err)
	}
}
