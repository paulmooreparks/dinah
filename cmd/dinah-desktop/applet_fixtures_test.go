package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"studio/internal/applet"
	"studio/internal/applet/contract"
)

// The test applets stand in for applets the app does not ship: a human
// checklist review, and a picture that is made as an artifact.
const (
	testChecklistRef = "test.checklist/request@1"
	testPictureID    = "test.picture"
	testPictureRef   = "test.picture/draw@1"
)

func testChecklistBundle() applet.Bundle {
	action := applet.Action{Ref: testChecklistRef, Title: "Request Checklist Review", Description: "Ask the person to review a checklist.", Replay: "human", Capabilities: []string{"attention.request"},
		InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["title","items"],"properties":{"title":{"type":"string","minLength":1,"maxLength":200},"body":{"type":"string","maxLength":20000},"items":{"type":"array","minItems":1,"maxItems":30,"items":{"type":"string","minLength":1,"maxLength":500}}}}`),
		OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["title","items","response"],"properties":{"title":{"type":"string"},"items":{"type":"array","items":{"type":"string"}},"response":{"type":"object","additionalProperties":false,"properties":{"notes":{"type":"string"}},"patternProperties":{"^item_[1-9][0-9]*$":{"type":"boolean"}}}}}`),
	}
	request := func(ctx context.Context, input string) (applet.Result, error) {
		var in struct {
			Title string   `json:"title"`
			Body  string   `json:"body"`
			Items []string `json:"items"`
		}
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return applet.Result{}, err
		}
		fields := make([]applet.Field, 0, len(in.Items)+1)
		for i, label := range in.Items {
			fields = append(fields, applet.Field{Name: fmt.Sprintf("item_%d", i+1), Label: label, Type: "boolean", Required: true})
		}
		fields = append(fields, applet.Field{Name: "notes", Label: "Notes", Type: "string"})
		output, _ := json.Marshal(map[string]any{"title": in.Title, "items": in.Items})
		return applet.Result{OutputJSON: string(output), Request: &applet.Request{Title: in.Title, Body: in.Body, Fields: fields}}, ctx.Err()
	}
	return applet.Bundle{Manifest: applet.Manifest{ID: "test.checklist", Version: "1.0.0", Title: "Review Checklist", Description: "A test bundle for human checklist review.", APIVersion: 1, Actions: []applet.Action{action}}, Handlers: map[string]applet.Handler{testChecklistRef: request}, Sources: map[string][]byte{"handler.go": []byte("reviewed test handler")}}
}

func testPictureBundle() applet.Bundle {
	action := applet.Action{Ref: testPictureRef, Title: "Draw a Picture", Description: "Draw the text given as an SVG picture.", Replay: "pure", Capabilities: []string{"artifact.create"},
		InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["text"],"properties":{"text":{"type":"string","minLength":1,"maxLength":100}}}`),
		OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["text","image"],"properties":{"text":{"type":"string"},"image":` + contract.ArtifactSchema + `}}`),
	}
	draw := func(ctx context.Context, input string) (applet.Result, error) {
		var in struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return applet.Result{}, err
		}
		output, _ := json.Marshal(map[string]string{"text": in.Text})
		svg := `<svg xmlns="http://www.w3.org/2000/svg"><text>` + in.Text + `</text></svg>`
		return applet.Result{OutputJSON: string(output), Artifacts: []applet.Artifact{{Key: "image", Name: "picture.svg", MIME: "image/svg+xml", Data: []byte(svg)}}}, ctx.Err()
	}
	return applet.Bundle{Manifest: applet.Manifest{ID: testPictureID, Version: "1.0.0", Title: "Picture", Description: "A test bundle that makes an artifact.", APIVersion: 1, Actions: []applet.Action{action}}, Handlers: map[string]applet.Handler{testPictureRef: draw}, Sources: map[string][]byte{"handler.go": []byte("reviewed test handler")}}
}

const testEchoRef = "test.echo/run@1"
const testEchoSchema = `{"type":"object","additionalProperties":false,"required":["value"],"properties":{"value":{"type":"number"}}}`

// testEchoRegistry is a registry of one pure action that the handler given
// carries out.
func testEchoRegistry(t *testing.T, handler applet.Handler) *applet.Registry {
	t.Helper()
	r, err := applet.New(applet.Bundle{Manifest: applet.Manifest{ID: "test.echo", Version: "1.0.0", Title: "Echo", APIVersion: 1, Actions: []applet.Action{{Ref: testEchoRef, Title: "Echo", InputSchema: json.RawMessage(testEchoSchema), OutputSchema: json.RawMessage(testEchoSchema), Replay: "pure"}}}, Sources: map[string][]byte{"implementation": []byte("1.0.0")}, Handlers: map[string]applet.Handler{testEchoRef: handler}})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// withTestApplets gives an app's broker the test applets beside the
// built-in ones, and returns the app.
func withTestApplets(t *testing.T, a *App) *App {
	t.Helper()
	b, err := a.appletHostService().Broker()
	if err != nil {
		t.Fatal(err)
	}
	bundles, err := a.builtinAppletBundles()
	if err != nil {
		t.Fatal(err)
	}
	r, err := applet.New(append(bundles, testChecklistBundle(), testPictureBundle())...)
	if err != nil {
		t.Fatal(err)
	}
	b.Registry = r
	return a
}
