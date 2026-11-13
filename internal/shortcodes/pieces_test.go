package shortcodes

import (
	"strings"
	"testing"
)

func TestAContainersFrameAndBodyAreMarked(t *testing.T) {
	slot := `<slot name="content"></slot>`
	// The slot alone in its element: that element is the body.
	got := marked(`<div class="panel"><div class="panel-body">` + slot + `</div></div>`)
	if got != `<div class="panel" data-sc-frame=""><div class="panel-body" data-sc-body="">`+slot+`</div></div>` {
		t.Fatalf("a frame with a body element: %s", got)
	}
	// A title beside the slot is no part of the body, so the body gets an
	// element of its own, and a piece mark a set wrote is taken off.
	got = marked(`<div class="panel" data-sc-piece="first"><p class="title">Note</p>` + slot + `</div>`)
	if got != `<div class="panel" data-sc-frame=""><p class="title">Note</p><div data-sc-body="">`+slot+`</div></div>` {
		t.Fatalf("a frame with a title: %s", got)
	}
	// The frame itself is the body when the slot is all it has.
	if got = marked(`<blockquote>` + slot + `</blockquote>`); got != `<blockquote data-sc-frame="" data-sc-body="">`+slot+`</blockquote>` {
		t.Fatalf("a frame that is its body: %s", got)
	}
	// What is not one frame around one slot stays as it was.
	for _, same := range []string{slot, `<span class="change">` + slot + `</span>`, `<div>` + slot + `</div><div></div>`, `<div>` + slot + slot + `</div>`, `text <div>` + slot + `</div>`, `<div>no slot</div>`} {
		if got := marked(same); got != same {
			t.Fatalf("%s became %s", same, got)
		}
	}
	// The same HTML gives the same bytes.
	in := `<div class="panel"><p>Title</p>` + slot + `</div>`
	first, second := marked(in), marked(in)
	if first != second || !strings.Contains(first, "data-sc-body") {
		t.Fatal("marking is not a function of the HTML")
	}
}

type flowSet struct{}

func (flowSet) Catalog() []Descriptor { return nil }
func (flowSet) Render(r Request) (string, error) {
	switch r.Name {
	case "piecebox":
		return `<div class="box"><slot name="content"></slot></div>`, nil
	case "piecelist":
		return `<ul><li>One</li></ul>`, nil
	case "piecewhole":
		return `<div>Whole</div>`, nil
	}
	return "", ErrUnknown
}
func (flowSet) Flows(r Request) bool { return r.Name == "piecelist" }

func TestADrawingSaysWhetherItFlows(t *testing.T) {
	mu.Lock()
	before := sets
	sets = []Set{flowSet{}}
	mu.Unlock()
	t.Cleanup(func() { mu.Lock(); sets = before; mu.Unlock() })
	box, err := Draw(Request{Name: "piecebox"})
	if err != nil || box.Flow || !strings.Contains(box.HTML, "data-sc-frame") {
		t.Fatalf("a container: %+v, %v", box, err)
	}
	// A container read as a span is in a line, and is not marked.
	if in, _ := Draw(Request{Name: "piecebox", Kind: Span}); strings.Contains(in.HTML, "data-sc-") {
		t.Fatalf("a container in a line was marked: %s", in.HTML)
	}
	if list, err := Draw(Request{Name: "piecelist"}); err != nil || !list.Flow {
		t.Fatalf("a block that flows: %+v, %v", list, err)
	}
	if whole, err := Draw(Request{Name: "piecewhole"}); err != nil || whole.Flow {
		t.Fatalf("a block that stays whole: %+v, %v", whole, err)
	}
}
