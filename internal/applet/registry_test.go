package applet

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"studio/internal/applet/contract"
)

const (
	checklistRef = "test.checklist/request@1"
	pictureRef   = "test.picture/draw@1"
)

// checklistBundle is a bundle using only the public contract: it asks the
// person to review a checklist, which the caller cannot complete.
func checklistBundle() Bundle {
	action := Action{Ref: checklistRef, Title: "Request Checklist Review", Replay: "human", Capabilities: []string{"attention.request"},
		InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["title","items"],"properties":{"title":{"type":"string","minLength":1,"maxLength":200},"body":{"type":"string","maxLength":20000},"items":{"type":"array","minItems":1,"maxItems":30,"items":{"type":"string","minLength":1,"maxLength":500}}}}`),
		OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["title","items","response"],"properties":{"title":{"type":"string"},"items":{"type":"array","items":{"type":"string"}},"response":{"type":"object","additionalProperties":false,"properties":{"notes":{"type":"string"}},"patternProperties":{"^item_[1-9][0-9]*$":{"type":"boolean"}}}}}`),
	}
	request := func(ctx context.Context, input string) (Result, error) {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		var in struct {
			Title string   `json:"title"`
			Body  string   `json:"body"`
			Items []string `json:"items"`
		}
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return Result{}, err
		}
		fields := make([]Field, 0, len(in.Items)+1)
		for i, label := range in.Items {
			fields = append(fields, Field{Name: fmt.Sprintf("item_%d", i+1), Label: label, Type: "boolean", Required: true})
		}
		fields = append(fields, Field{Name: "notes", Label: "Notes", Type: "string"})
		output, _ := json.Marshal(map[string]any{"title": in.Title, "items": in.Items})
		return Result{OutputJSON: string(output), Request: &Request{Title: in.Title, Body: in.Body, Fields: fields}}, nil
	}
	return Bundle{Manifest: Manifest{ID: "test.checklist", Version: "1.0.0", Title: "Review Checklist", APIVersion: 1, Actions: []Action{action}}, Handlers: map[string]Handler{checklistRef: request}, Sources: map[string][]byte{"handler.go": []byte("reviewed test handler")}}
}

// pictureBundle makes an SVG artifact of the text it is given.
func pictureBundle() Bundle {
	action := Action{Ref: pictureRef, Title: "Draw", Replay: "pure", Capabilities: []string{"artifact.create"},
		InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["text"],"properties":{"text":{"type":"string","minLength":1,"maxLength":100}}}`),
		OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["text","image"],"properties":{"text":{"type":"string"},"image":` + contract.ArtifactSchema + `}}`),
	}
	draw := func(ctx context.Context, input string) (Result, error) {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		var in struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return Result{}, err
		}
		output, _ := json.Marshal(map[string]string{"text": in.Text})
		svg := `<svg xmlns="http://www.w3.org/2000/svg"><text>` + in.Text + `</text></svg>`
		return Result{OutputJSON: string(output), Artifacts: []Artifact{{Key: "image", Name: "picture.svg", MIME: "image/svg+xml", Data: []byte(svg)}}}, nil
	}
	return Bundle{Manifest: Manifest{ID: "test.picture", Version: "1.0.0", Title: "Picture", APIVersion: 1, Actions: []Action{action}}, Handlers: map[string]Handler{pictureRef: draw}, Sources: map[string][]byte{"handler.go": []byte("reviewed test handler")}}
}

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	r, err := New(pictureBundle(), checklistBundle())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestArtifactAndHumanRequest(t *testing.T) {
	r := testRegistry(t)
	result, err := r.Execute(context.Background(), pictureRef, `{"text":"Hello"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 1 || !strings.HasPrefix(string(result.Artifacts[0].Data), "<svg ") {
		t.Fatal("missing SVG")
	}
	var out map[string]json.RawMessage
	if err = json.Unmarshal([]byte(result.OutputJSON), &out); err != nil {
		t.Fatal(err)
	}
	out["image"] = json.RawMessage(`{"id":"a","invocationId":"b","name":"picture.svg","mime":"image/svg+xml","size":1,"sha256":"` + strings.Repeat("a", 64) + `"}`)
	completed, _ := json.Marshal(out)
	action, _ := r.Describe(pictureRef)
	if err = Validate(action.OutputSchema, string(completed)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Execute(context.Background(), pictureRef, `{"text":123}`); err == nil {
		t.Fatal("accepted input against the schema")
	}
	result, err = r.Execute(context.Background(), checklistRef, `{"title":"Release Review","items":["Tests pass","Readme checked"],"body":"Review the candidate."}`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Request == nil || len(result.Request.Fields) != 3 || !result.Request.Fields[0].Required || result.Request.Fields[0].Type != "boolean" {
		t.Fatalf("bad request: %+v", result.Request)
	}
	if len(result.Artifacts) != 0 || strings.Contains(result.OutputJSON, "response") {
		t.Fatal("provider invented human response")
	}
	action, _ = r.Describe(checklistRef)
	if err = Validate(action.OutputSchema, `{"title":"Release Review","items":["Tests pass"],"response":{"item_1":false,"notes":"Needs work"}}`); err != nil {
		t.Fatal(err)
	}
}

func TestLosslessSchemaAndRegistry(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"count":{"const":9007199254740993}},"required":["count"]}`)
	if err := Validate(schema, `{"count":9007199254740993}`); err != nil {
		t.Fatal(err)
	}
	if err := Validate(schema, `{"count":9007199254740992}`); err == nil {
		t.Fatal("rounded distinct integer")
	}
	if err := Validate(json.RawMessage(`{"$ref":"file:///private/schema.json"}`), `{}`); err == nil {
		t.Fatal("allowed file loading")
	}
	if err := Validate(schema, `{"count":9007199254740993} {}`); err == nil {
		t.Fatal("accepted trailing JSON")
	}
	if _, err := New(checklistBundle(), checklistBundle()); err == nil {
		t.Fatal("accepted duplicate bundle")
	}
	b := checklistBundle()
	b.Manifest.APIVersion = 2
	if _, err := New(b); err == nil {
		t.Fatal("accepted incompatible API")
	}
	b = checklistBundle()
	b.Manifest.Actions[0].InputSchema = json.RawMessage(`{"type":"nonsense"}`)
	if _, err := New(b); err == nil {
		t.Fatal("accepted malformed schema")
	}
	for _, version := range []string{"garbage", "1", "01.2.3", "1.2.3-01"} {
		b = checklistBundle()
		b.Manifest.Version = version
		if _, err := New(b); err == nil {
			t.Errorf("accepted malformed package version %q", version)
		}
	}
	r := testRegistry(t)
	before, _ := r.Digest(checklistRef)
	catalog := r.Catalog()
	catalog[1].Actions[0].Ref = "changed"
	a, _ := r.Describe(checklistRef)
	if a.Ref != checklistRef {
		t.Fatal("catalog mutated registry")
	}
	b = checklistBundle()
	b.Sources["new.js"] = []byte("changed")
	other, err := New(b)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := other.Digest(checklistRef)
	if before == after {
		t.Fatal("implementation change did not change digest")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = r.Execute(ctx, pictureRef, `{"text":"abc"}`); err != context.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err = r.Execute(ctx, checklistRef, `{}`); err != context.DeadlineExceeded {
		t.Fatalf("deadline: %v", err)
	}
}

func TestConsentFollowsDeclaredEffectLevel(t *testing.T) {
	process := []string{"process.run:git"}
	for _, row := range []struct {
		effect string
		caps   []string
		asks   bool
	}{
		{"", process, true},
		{"", []string{"workspace.read"}, false},
		{"read", process, false},
		{"local", []string{"workspace.write", "process.run:git"}, false},
		{"history", process, true},
		{"network", []string{"workspace.read"}, true},
	} {
		if got := Asks(Action{Effect: row.effect, Capabilities: row.caps}); got != row.asks {
			t.Errorf("effect %q with %v: asks %v, want %v", row.effect, row.caps, got, row.asks)
		}
	}
}

func TestIndependentProviderAndCapabilityFailures(t *testing.T) {
	const ref = "independent.example/echo@1"
	build := func(handler Handler, capabilities []string) *Registry {
		t.Helper()
		b := Bundle{Manifest: Manifest{ID: "independent.example", Version: "1.0.0", APIVersion: 1, Actions: []Action{{Ref: ref, Replay: "pure", Capabilities: capabilities, InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`)}}}, Handlers: map[string]Handler{ref: handler}, Sources: map[string][]byte{"handler.go": []byte("reviewed test handler")}}
		r, err := New(b)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	r := build(func(ctx context.Context, input string) (Result, error) { return Result{OutputJSON: input}, nil }, nil)
	input := `{"integer":9007199254740993,"nested":{"count":9007199254740995}}`
	result, err := r.Execute(context.Background(), ref, input)
	if err != nil || result.OutputJSON != input {
		t.Fatalf("generic provider rounded JSON: %s %v", result.OutputJSON, err)
	}
	r = build(func(context.Context, string) (Result, error) {
		return Result{OutputJSON: `{}`, Artifacts: []Artifact{{Key: "image", Data: []byte("test")}}}, nil
	}, nil)
	if _, err = r.Execute(context.Background(), ref, `{}`); err == nil {
		t.Fatal("allowed undeclared artifact")
	}
	r = build(func(context.Context, string) (Result, error) {
		return Result{OutputJSON: `{}`, Request: &Request{Title: "Undeclared"}}, nil
	}, nil)
	if _, err = r.Execute(context.Background(), ref, `{}`); err == nil {
		t.Fatal("allowed undeclared human request")
	}
	r = build(func(context.Context, string) (Result, error) {
		return Result{OutputJSON: `{"image":"existing"}`, Artifacts: []Artifact{{Key: "image", Data: []byte("test")}}}, nil
	}, []string{"artifact.create"})
	if _, err = r.Execute(context.Background(), ref, `{}`); err == nil {
		t.Fatal("allowed artifact to overwrite output")
	}
}
