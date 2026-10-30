package diagram

import (
	"strings"
	"testing"
)

// TestRender_EmbedsItsFonts checks that the SVG brings the fonts its labels
// were measured in, as @font-face rules with data URLs. Dinah's
// drawing keeps only fonts of that form (frontend/diagrams.js), so a d2
// upgrade that changes it would draw labels in another font.
func TestRender_EmbedsItsFonts(t *testing.T) {
	svg, err := Render("a: Alpha\nb: Beta\na -> b: label {style.italic: true}\n")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(svg)
	if n := strings.Count(s, "@font-face"); n < 2 {
		t.Fatalf("want an @font-face for the bold and the italic labels, got %d", n)
	}
	if n := strings.Count(s, `src: url("data:application/font-woff;base64,`); n != strings.Count(s, "@font-face") {
		t.Errorf("every @font-face should have a data URL source; %d of %d do", n, strings.Count(s, "@font-face"))
	}
}

// TestRender_ProducesSizedSVG drives the real d2 compiler (no mock): valid d2
// source compiles to an SVG whose root element carries intrinsic width/height
// injected from the viewBox (the ensureRootSize fix). This is the only test
// that exercises the actual oss.terrastruct.com/d2 dependency; the handler-layer
// tests use a mock renderer.
func TestRender_ProducesSizedSVG(t *testing.T) {
	svg, err := Render("a -> b\n")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(svg)
	i := strings.Index(s, "<svg ")
	if i < 0 {
		t.Fatalf("no <svg root element in output")
	}
	end := strings.Index(s[i:], ">")
	rootTag := s[i : i+end+1]
	if !strings.Contains(rootTag, "width=") || !strings.Contains(rootTag, "height=") {
		t.Errorf("root <svg> must carry intrinsic width+height, got %q", rootTag)
	}
	if !strings.Contains(rootTag, "viewBox=") {
		t.Errorf("expected viewBox on root <svg>, got %q", rootTag)
	}
}

// TestRender_InvalidSourceErrors confirms a parse/compile failure surfaces as an
// error rather than a panic or empty output, so the handler layer can turn it
// into a *DiagramRenderError.
func TestRender_InvalidSourceErrors(t *testing.T) {
	_, err := Render("a -> ->\n{{{ not d2")
	if err == nil {
		t.Fatalf("expected an error for invalid d2 source, got nil")
	}
}

// TestEnsureRootSize_PreservesExistingSize confirms the injector is a no-op when
// width/height are already present (it must not double-inject).
func TestEnsureRootSize_PreservesExistingSize(t *testing.T) {
	in := []byte(`<svg width="5" height="6" viewBox="0 0 5 6"><g/></svg>`)
	out := ensureRootSize(in)
	if string(out) != string(in) {
		t.Errorf("ensureRootSize must not alter an already-sized root; got %q", out)
	}
}

// TestEnsureRootSize_InjectsFromViewBox confirms width/height are taken from the
// viewBox's third and fourth fields when absent.
func TestEnsureRootSize_InjectsFromViewBox(t *testing.T) {
	in := []byte(`<svg viewBox="0 0 120 80"><g/></svg>`)
	out := string(ensureRootSize(in))
	if !strings.Contains(out, `width="120"`) || !strings.Contains(out, `height="80"`) {
		t.Errorf("expected width=120 height=80 injected from viewBox, got %q", out)
	}
}
