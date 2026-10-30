package codexagent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"studio/internal/acp"
)

func TestMCPToolApprovalUsesQuestionIDAndLabel(t *testing.T) {
	for _, tc := range []struct{ choice, want string }{{"choice_0", "Accept"}, {"choice_1", "Decline"}, {"choice_2", "Cancel"}, {"forged", ""}} {
		h := &testHandler{choice: tc.choice}
		c := &Conn{handler: h}
		params := acp.Raw(map[string]any{"threadId": "thread", "itemId": "mcp-call", "questions": []any{map[string]any{"id": "tool_approval", "header": "Applet Permission", "question": "Allow managed applet discovery?", "options": []any{map[string]string{"label": "Accept", "description": "Run once"}, map[string]string{"label": "Decline"}, map[string]string{"label": "Cancel"}}}}})
		got, err := c.request(context.Background(), "item/tool/requestUserInput", params)
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Answers map[string]struct {
				Answers []string `json:"answers"`
			} `json:"answers"`
		}
		if err := acp.Decode(got, &result); err != nil {
			t.Fatal(err)
		}
		a := result.Answers["tool_approval"].Answers
		if tc.want == "" {
			if len(a) != 0 {
				t.Fatalf("forged choice accepted: %v", a)
			}
		} else if len(a) != 1 || a[0] != tc.want {
			t.Fatalf("got %v, want %s", a, tc.want)
		}
		if h.permission.SessionID != "codex:thread" || h.permission.Options[0].Name != "Accept" {
			t.Fatalf("host permission %#v", h.permission)
		}
	}
}

func TestToolApprovalWireResponse(t *testing.T) {
	for _, choice := range []string{"choice_0", "allow_once"} {
		t.Run(choice, func(t *testing.T) {
			h := &testHandler{choice: choice}
			response := make(chan json.RawMessage, 1)
			c := fake(t, h, func(m message, send func(message)) {
				switch m.Method {
				case "turn/start":
					send(answer(m, map[string]any{"turn": turn{ID: "turn", Status: "inProgress"}}))
					send(message{ID: json.RawMessage(`"approval-request"`), Method: "item/tool/requestUserInput", Params: json.RawMessage(`{"threadId":"thread","itemId":"tool","questions":[{"id":"approve-applet","header":"Approval","question":"Allow dinah-applets applet_list?","options":[{"label":"Accept","description":"Run once"},{"label":"Decline","description":"Do not run"}]}]}`)})
				case "":
					if string(m.ID) != `"approval-request"` {
						t.Errorf("wrong approval ID %s", m.ID)
					}
					response <- m.Result
					send(message{Method: "turn/completed", Params: acp.Raw(map[string]any{"threadId": "thread", "turn": turn{ID: "turn", Status: "completed"}})})
				}
			})
			c.sessions["codex:thread"] = &session{streamed: map[string]bool{}}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: "codex:thread"}, nil); err != nil {
				t.Fatal(err)
			}
			var got struct {
				Answers map[string]struct {
					Answers []string `json:"answers"`
				} `json:"answers"`
			}
			if err := json.Unmarshal(<-response, &got); err != nil {
				t.Fatal(err)
			}
			answers := got.Answers["approve-applet"].Answers
			if choice == "choice_0" {
				if len(answers) != 1 || answers[0] != "Accept" {
					t.Fatalf("valid selection %v", answers)
				}
			} else if len(answers) != 0 {
				t.Fatalf("invalid ACP kind accepted as option ID: %v", answers)
			}
		})
	}
}

func TestFreeformCodexQuestionIsExplicitlyUnsupported(t *testing.T) {
	c := &Conn{}
	_, err := c.request(context.Background(), "item/tool/requestUserInput", json.RawMessage(`{"threadId":"t","questions":[{"id":"q","question":"Text?"}]}`))
	if err == nil {
		t.Fatal("freeform question silently answered")
	}
}
