package shortcodes

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeSet struct{ names map[string]string }

func (f fakeSet) Catalog() []Descriptor {
	out := []Descriptor{}
	for name := range f.names {
		out = append(out, Descriptor{Name: name, Kind: Inline})
	}
	return out
}

func (f fakeSet) Render(r Request) (string, error) {
	html, ok := f.names[r.Name]
	if !ok {
		return "", ErrUnknown
	}
	return html + ":" + r.Args, nil
}

func TestRegistryMergesCatalogsAndRendersThroughTheFirstSetThatKnows(t *testing.T) {
	if Any() {
		t.Fatal("a fresh registry has sets")
	}
	Register(fakeSet{names: map[string]string{"status": "<span>status</span>", "toc": "<nav/>"}})
	Register(fakeSet{names: map[string]string{"status": "<b>other</b>", "tracker": "<a/>"}})
	got := Catalog()
	names := ""
	for _, d := range got {
		names += d.Name + ","
	}
	if names != "status,toc,tracker," || got[0].Params == nil || got[0].Title != "status" {
		t.Fatalf("catalog: %s %+v", names, got)
	}
	html, err := Render(Request{Name: "status", Args: "green Done"})
	if err != nil || html != "<span>status</span>:green Done" {
		t.Fatalf("render: %q %v", html, err)
	}
	if html, err := Render(Request{Name: "tracker", Args: "X-1"}); err != nil || html != "<a/>:X-1" {
		t.Fatalf("second set: %q %v", html, err)
	}
	if _, err := Render(Request{Name: "nobody"}); !errors.Is(err, ErrUnknown) {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := Render(Request{Name: "Bad Name"}); err == nil || errors.Is(err, ErrUnknown) {
		t.Fatalf("a bad name was accepted: %v", err)
	}
	if html, err := Render(Request{Name: "status", Kind: Span, Args: "x"}); err != nil || html != "<span>status</span>:x" {
		t.Fatalf("a kind the applet read: %q %v", html, err)
	}
	if _, err := Render(Request{Name: "status", Kind: "page"}); err == nil || errors.Is(err, ErrUnknown) {
		t.Fatalf("a kind that is none of the four was accepted: %v", err)
	}
	if !Any() {
		t.Fatal("Any says no sets")
	}
}

type formSet struct{ fakeSet }

func (formSet) Form(source string) (Form, bool) {
	if source != "{{label:red Late}}" {
		return Form{}, false
	}
	return Form{Kind: "label", Title: "Label", Fields: []Field{
		{Name: "colour", Label: "Colour", Value: "red", Values: []string{"red", "green"}},
		{Name: "text", Label: "Text", Value: "Late", Required: true},
	}}, true
}

func (formSet) Build(kind string, values map[string]string) (string, error) {
	if kind != "label" {
		return "", ErrUnknown
	}
	if values["text"] == "" {
		return "", &FieldError{Field: "text", Message: "A label needs its text."}
	}
	return "{{label:" + values["colour"] + " " + values["text"] + "}}", nil
}

func TestFormsComeFromTheSetThatHasThem(t *testing.T) {
	Register(fakeSet{names: map[string]string{"plain": "<i/>"}})
	Register(formSet{fakeSet{names: map[string]string{"label": "<b/>"}}})
	form, ok := FormOf("{{label:red Late}}")
	if !ok || form.Kind != "label" || len(form.Fields) != 2 || form.Fields[0].Values[1] != "green" {
		t.Fatalf("form: %v %+v", ok, form)
	}
	if _, ok := FormOf("{{plain}}"); ok {
		t.Fatal("a shortcode with no form has one")
	}
	source, err := Build("label", map[string]string{"colour": "green", "text": "Done"})
	if err != nil || source != "{{label:green Done}}" {
		t.Fatalf("build: %q %v", source, err)
	}
	var field *FieldError
	if _, err := Build("label", map[string]string{"colour": "green"}); !errors.As(err, &field) || field.Field != "text" {
		t.Fatalf("a missing value: %v", err)
	}
	if _, err := Build("plain", nil); !errors.Is(err, ErrUnknown) {
		t.Fatalf("a kind with no form: %v", err)
	}
	if _, err := Build("Bad Kind", nil); err == nil || errors.Is(err, ErrUnknown) {
		t.Fatalf("a bad kind was accepted: %v", err)
	}
}

type literalSet struct{}

func (literalSet) Catalog() []Descriptor {
	return []Descriptor{
		{Name: "markup", Kind: Container, Literal: true, ColorAttribute: "data-cell-colour", ColorClassPrefix: "cell-colour-"},
		{Name: "mark", Kind: Inline, Literal: true, ColorAttribute: "data-cell-colour", ColorClassPrefix: "cell-colour-"},
		{Name: "prose", Kind: Container, ColorAttribute: "data-cell-colour", ColorClassPrefix: "cell-colour-"},
		{Name: "styled", Kind: Container, Literal: true, ColorAttribute: "style", ColorClassPrefix: "-colour"},
		{Name: "handled", Kind: Container, Literal: true, ColorAttribute: "onclick", ColorClassPrefix: "cell colour"},
	}
}

func (literalSet) Render(Request) (string, error) { return "", ErrUnknown }

// A container may say its body is markup, and the catalog tells the applets
// so under the name their contract reads. The flag is dropped on a shortcode
// that has no body.
func TestACatalogSaysWhichContainersAreLiteral(t *testing.T) {
	Register(literalSet{})
	var container, inline *Descriptor
	for _, d := range Catalog() {
		switch d.Name {
		case "markup":
			container = &d
		case "mark":
			inline = &d
		}
	}
	if container == nil || inline == nil || !container.Literal || inline.Literal {
		t.Fatalf("container %+v, inline %+v", container, inline)
	}
	b, err := json.Marshal(container)
	if err != nil || !strings.Contains(string(b), `"literal":true`) {
		t.Fatalf("the container as the applets get it: %s %v", b, err)
	}
	if b, _ := json.Marshal(inline); strings.Contains(string(b), "literal") {
		t.Fatalf("the inline shortcode as the applets get it: %s", b)
	}
}

// A literal container may say which data attribute keeps a cell's colour.
// The catalog drops it on a shortcode with no literal body, and drops a name
// that is not a data attribute's.
func TestACatalogSaysWhichAttributeKeepsACellsColour(t *testing.T) {
	Register(literalSet{})
	got := map[string]string{}
	for _, d := range Catalog() {
		got[d.Name] = d.ColorAttribute
		if b, _ := json.Marshal(d); d.Name == "markup" && !strings.Contains(string(b), `"colorAttribute":"data-cell-colour"`) {
			t.Fatalf("the container as the applets get it: %s", b)
		}
	}
	if got["markup"] != "data-cell-colour" {
		t.Fatalf("the literal container's attribute: %q", got["markup"])
	}
	for _, name := range []string{"mark", "prose", "styled", "handled"} {
		if got[name] != "" {
			t.Errorf("%s kept the attribute %q", name, got[name])
		}
	}
}

// A literal container may say how the class names of a coloured cell begin.
// The catalog drops the prefix on a shortcode with no literal body, and
// drops one that is not the beginning of a class name.
func TestACatalogSaysHowACellsColourClassBegins(t *testing.T) {
	Register(literalSet{})
	got := map[string]string{}
	for _, d := range Catalog() {
		got[d.Name] = d.ColorClassPrefix
		if b, _ := json.Marshal(d); d.Name == "markup" && !strings.Contains(string(b), `"colorClassPrefix":"cell-colour-"`) {
			t.Fatalf("the container as the applets get it: %s", b)
		}
	}
	if got["markup"] != "cell-colour-" {
		t.Fatalf("the literal container's prefix: %q", got["markup"])
	}
	for _, name := range []string{"mark", "prose", "styled", "handled"} {
		if got[name] != "" {
			t.Fatalf("%s kept the prefix %q", name, got[name])
		}
	}
}
