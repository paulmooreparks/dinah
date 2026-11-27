package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/applet"
)

func TestTeamStartReportsUnavailableModelAndEffectiveDefault(t *testing.T) {
	a, _, lead, _ := duplicationApp(t, "codex:member")
	out, err := call(t, a.teamStart2Handler, asChat(a, lead), map[string]any{"title": "Worker", "brief": "Review the invented fixture.", "model": "invented-unavailable", "options": map[string]any{"effort": "high"}})
	if err != nil {
		t.Fatal(err)
	}
	member := a.session(out["sessionId"].(string))
	refused := out["notApplied"].([]any)
	if len(refused) != 1 {
		t.Fatalf("refused model was silently replaced: notApplied=%v", refused)
	}
	reason := refused[0].(map[string]any)
	if reason["id"] != "model" || !strings.Contains(reason["reason"].(string), "invented-unavailable") {
		t.Fatalf("model refusal lacks request and reason: %v", reason)
	}
	if out["model"] != "initial" || optionOf(member, "model") != "initial" {
		t.Fatalf("effective default not reported: result=%v, actual=%s", out, optionOf(member, "model"))
	}
	if optionOf(member, "effort") != "high" {
		t.Fatal("unrelated accepted setting was lost")
	}
}

type teamModelConnection struct {
	*testProviderConnection
	refuse      bool
	keepDefault bool
}

func (c *teamModelConnection) Call(ctx context.Context, method string, params, result any) error {
	if method == "session/set_config_option" && params.(acp.SetConfigOptionParams).ConfigID == "model" {
		if c.refuse {
			return errors.New("invented model refusal")
		}
		if c.keepDefault {
			return acp.Result(acp.ConfigOptionsResult{ConfigOptions: c.options}, result)
		}
	}
	return c.testProviderConnection.Call(ctx, method, params, result)
}

func TestTeamStartModelReportsAcrossBothActionVersions(t *testing.T) {
	for _, version := range []string{teamStartAction, teamStart2Action} {
		for _, mode := range []string{"accepted", "inherited-unavailable", "refused", "kept-default", "no-model-setting"} {
			t.Run(version+"/"+mode, func(t *testing.T) {
				a, c, lead, _ := duplicationApp(t, "codex:member")
				requested := "changed"
				if mode == "inherited-unavailable" {
					c.options[0].Options = json.RawMessage(`[{"value":"initial","name":"Initial"}]`)
				}
				if mode == "no-model-setting" {
					c.options = c.options[1:]
				}
				if mode == "refused" || mode == "kept-default" {
					init := a.runtime.ProviderLocked("codex").Initialization()
					a.runtime.PutProviderLocked("codex", newTestProvider(&teamModelConnection{testProviderConnection: c, refuse: mode == "refused", keepDefault: mode == "kept-default"}, init))
				}
				input := map[string]any{"title": "Worker", "brief": "Review an invented fixture."}
				if mode != "inherited-unavailable" {
					input["model"] = requested
				}
				h := a.teamStartHandler
				if version == teamStart2Action {
					h = a.teamStart2Handler
				}
				out, err := call(t, h, asChat(a, lead), input)
				if err != nil {
					t.Fatal(err)
				}
				member := a.session(out["sessionId"].(string))
				actual := optionOf(member, "model")
				if out["model"] != actual {
					t.Fatalf("reported model %v differs from %s", out["model"], actual)
				}
				refused := out["notApplied"].([]any)
				if mode == "accepted" {
					if actual != requested || len(refused) != 0 {
						t.Fatalf("accepted model: %v", out)
					}
				} else {
					if len(refused) != 1 {
						t.Fatalf("missing model refusal: %v", out)
					}
					r := refused[0].(map[string]any)
					if r["id"] != "model" || !strings.Contains(r["reason"].(string), requested) {
						t.Fatalf("incomplete refusal: %v", r)
					}
					if mode == "refused" && !strings.Contains(r["reason"].(string), "invented model refusal") {
						t.Fatalf("provider reason lost: %v", r)
					}
				}
				bundle, err := a.teamAppletBundle()
				if err != nil {
					t.Fatal(err)
				}
				for _, action := range bundle.Manifest.Actions {
					if action.Ref == version {
						raw, _ := json.Marshal(out)
						if err := applet.Validate(action.OutputSchema, string(raw)); err != nil {
							t.Fatalf("output violates action schema: %v", err)
						}
					}
				}
			})
		}
	}
}
