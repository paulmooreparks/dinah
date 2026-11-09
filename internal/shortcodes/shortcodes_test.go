package shortcodes

import (
	"errors"
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
	if names != "status,toc,tracker," || got[0].Params == nil {
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
	if !Any() {
		t.Fatal("Any says no sets")
	}
}
