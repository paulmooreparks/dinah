package claudeagent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"studio/internal/acp"
)

func TestAdvertisedSteeringUsesHostOwnedIdleFallback(t *testing.T) {
	for _, scenario := range []string{"injected", "promptRequired", "startedNewTurn", "malformed", "rpc-error", "absent", "false", "invalid-capability", "image"} {
		t.Run(scenario, func(t *testing.T) {
			client, server := net.Pipe()
			c := &Conn{Conn: acp.Connect(client, client, &recorder{}, acp.Options{})}
			t.Cleanup(func() { c.Close(); server.Close(); <-c.Done() })
			calls := make(chan string, 4)
			go func() {
				sc := bufio.NewScanner(server)
				out := json.NewEncoder(server)
				for sc.Scan() {
					var m struct {
						ID     json.RawMessage `json:"id"`
						Method string          `json:"method"`
						Params struct {
							Session string             `json:"sessionId"`
							Prompt  []acp.ContentBlock `json:"prompt"`
							Meta    struct {
								Steering struct {
									Idle string `json:"idleBehavior"`
								} `json:"steering"`
							} `json:"_meta"`
						} `json:"params"`
					}
					if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
						t.Error(err)
						return
					}
					calls <- m.Method
					result := any(map[string]any{})
					switch m.Method {
					case "initialize":
						if scenario != "absent" {
							var supported any = true
							if scenario == "false" {
								supported = false
							}
							if scenario == "invalid-capability" {
								supported = "true"
							}
							result = map[string]any{"_meta": map[string]any{"steering": map[string]any{"supported": supported}}}
						}
					case "_session/steering":
						if m.Params.Session != "fixture" || m.Params.Meta.Steering.Idle != "promptRequired" || len(m.Params.Prompt) != 1 || m.Params.Prompt[0].Text != "Stop at the next safe point." {
							t.Errorf("steer params: %+v", m.Params)
						}
						if scenario == "rpc-error" {
							out.Encode(map[string]any{"id": m.ID, "error": map[string]any{"code": -32603, "message": "fixture failure"}})
							continue
						}
						result = map[string]string{"outcome": scenario, "reason": "noRunningTurn"}
					default:
						t.Errorf("unexpected method %s", m.Method)
					}
					out.Encode(map[string]any{"id": m.ID, "result": result})
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, nil); err != nil {
				t.Fatal(err)
			}
			<-calls
			offered := scenario != "absent" && scenario != "false" && scenario != "invalid-capability"
			if c.CanSteer("claude:fixture") != offered || c.CanSteerContent("claude:fixture", []string{"image"}) {
				t.Fatal("capability mismatch")
			}
			kind := "text"
			if scenario == "image" {
				kind = "image"
			}
			confirmed := 0
			err := c.Steer(ctx, acp.PromptParams{SessionID: "claude:fixture", Prompt: []acp.ContentBlock{{Type: kind, Text: "Stop at the next safe point."}}}, func() { confirmed++ })
			if (err == nil) != (scenario == "injected") || (confirmed == 1) != (scenario == "injected") {
				t.Fatalf("error=%v confirmed=%d", err, confirmed)
			}
			if scenario == "promptRequired" && !errors.Is(err, acp.ErrNoActiveTurn) {
				t.Fatal(err)
			}
			if scenario == "startedNewTurn" || scenario == "malformed" {
				if !errors.Is(err, acp.ErrSteeringUnconfirmed) {
					t.Fatal(err)
				}
			}
			if !offered || scenario == "image" {
				select {
				case method := <-calls:
					t.Fatalf("unsupported steer sent %s", method)
				default:
				}
			} else {
				if method := <-calls; method != "_session/steering" {
					t.Fatal(method)
				}
			}
		})
	}
}
