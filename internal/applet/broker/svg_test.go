package broker

import (
	"strings"
	"testing"

	"studio/internal/diagram"
)

// A drawing D2 makes, with its nested svg, masks, markers, classes and
// embedded fonts, is an artifact a person may keep (9 November 2026); the
// check used to admit rectangles and text only, which refused every diagram.
func TestADiagramsDrawingIsAnAcceptableArtifact(t *testing.T) {
	svg, err := diagram.Render("a -> b: hello\nb.shape: cylinder\nc: {shape: person}\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(svg), "<style") || !strings.Contains(string(svg), "url(#") {
		t.Fatalf("the sample lacks what the check must admit: %.200s", svg)
	}
	if err := ValidateSVG(string(svg)); err != nil {
		t.Fatalf("a D2 drawing was refused: %v", err)
	}
	// Mermaid's labels are XHTML inside foreignObject.
	mermaid := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 10 10"><g class="node"><foreignObject width="10" height="10"><div xmlns="http://www.w3.org/1999/xhtml" style="display: table-cell"><span class="nodeLabel">A</span></div></foreignObject></g><use xlink:href="#arrow"/></svg>`
	if err := ValidateSVG(mermaid); err != nil {
		t.Fatalf("a Mermaid drawing was refused: %v", err)
	}
}

// Scripts, handlers, outside references and declarations are refused
// wherever they are put: in elements, attributes, styles or the prologue.
func TestAnSVGArtifactMayCarryNothingThatActsOrLoads(t *testing.T) {
	wrap := func(inner string) string {
		return `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">` + inner + `</svg>`
	}
	refused := map[string]string{
		"a script":                    wrap(`<script>alert(1)</script>`),
		"a script in a label":         wrap(`<foreignObject><div xmlns="http://www.w3.org/1999/xhtml"><script>1</script></div></foreignObject>`),
		"an event handler":            wrap(`<rect onload="alert(1)"/>`),
		"an outside image":            wrap(`<image href="https://example.com/a.png"/>`),
		"an outside link (xlink)":     wrap(`<use xlink:href="https://example.com/a.svg#x"/>`),
		"a file":                      wrap(`<image href="file:///C:/secret.png"/>`),
		"a style import":              wrap(`<style>@import url("https://example.com/x.css");</style>`),
		"a style with an outside url": wrap(`<style>.a { background: url(https://example.com/a.png) }</style>`),
		"an inline style url":         wrap(`<rect style="fill: url(https://example.com/a.png)"/>`),
		"a doctype":                   `<!DOCTYPE svg [<!ENTITY x "y">]>` + wrap(``),
		"a stylesheet instruction":    `<?xml-stylesheet href="https://example.com/x.css"?>` + wrap(``),
		"an iframe":                   wrap(`<foreignObject><iframe xmlns="http://www.w3.org/1999/xhtml" src="https://example.com"/></foreignObject>`),
		"a second root":               wrap(``) + wrap(``),
		"another document":            `<html xmlns="http://www.w3.org/1999/xhtml"><body/></html>`,
		"an unknown namespace":        wrap(`<x:thing xmlns:x="urn:other"/>`),
	}
	for what, svg := range refused {
		if ValidateSVG(svg) == nil {
			t.Errorf("%s was accepted", what)
		}
	}
	accepted := map[string]string{
		"a fragment reference":     wrap(`<use xlink:href="#a"/><path fill="url(#grad)" d="M0 0"/>`),
		"a data picture":           wrap(`<image href="data:image/png;base64,iVBORw0KGgo="/>`),
		"an embedded font":         wrap(`<style>@font-face { font-family: f; src: url("data:application/font-woff;base64,d09G"); }</style>`),
		"the xml declaration":      `<?xml version="1.0" encoding="utf-8"?>` + wrap(`<text x="1" y="2">hi</text>`),
		"a nested svg with a mask": wrap(`<svg viewBox="0 0 1 1"><mask id="m"><rect/></mask><g mask="url(#m)"><circle r="1"/></g><marker id="k"><polygon points="0,0"/></marker></svg>`),
	}
	for what, svg := range accepted {
		if err := ValidateSVG(svg); err != nil {
			t.Errorf("%s was refused: %v", what, err)
		}
	}
}
