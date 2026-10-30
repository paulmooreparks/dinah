// Package diagram renders D2 diagram source to SVG in-process, with no browser
// and no CGO (the D2 library does layout via an embedded JS engine). It is the
// engine behind ```d2 fenced blocks in a Markdown body.
//
// Pure domain logic: no MCP or HTTP-server imports.
package diagram

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"

	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	d2log "oss.terrastruct.com/d2/lib/log"
	"oss.terrastruct.com/d2/lib/textmeasure"
)

// quietLogger discards D2's internal slog output; without a logger in context
// D2 emits a stack-trace warning on every render.
var quietLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// Render compiles D2 source and returns a standalone SVG document whose root
// element has an intrinsic width and height (taken from the viewBox), so an
// image element shows it at its natural size rather than collapsing it.
//
// No CGO, no browser, no network: the D2 library uses an embedded JS engine
// for layout (oss.terrastruct.com/d2 v0.7.1).
func Render(src string) ([]byte, error) {
	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return nil, fmt.Errorf("diagram: text ruler: %w", err)
	}
	layout := "dagre"
	themeID := int64(0)
	pad := int64(100)

	compileOpts := &d2lib.CompileOptions{
		Ruler:  ruler,
		Layout: &layout,
		LayoutResolver: func(string) (d2graph.LayoutGraph, error) {
			return d2dagrelayout.DefaultLayout, nil
		},
	}
	renderOpts := &d2svg.RenderOpts{ThemeID: &themeID, Pad: &pad}

	ctx := d2log.With(context.Background(), quietLogger)
	d, _, err := d2lib.Compile(ctx, src, compileOpts, renderOpts)
	if err != nil {
		return nil, fmt.Errorf("diagram: compile: %w", err)
	}
	svg, err := d2svg.Render(d, renderOpts)
	if err != nil {
		return nil, fmt.Errorf("diagram: render: %w", err)
	}
	return ensureRootSize(svg), nil
}

var viewBoxRe = regexp.MustCompile(`viewBox="([^"]+)"`)

// ensureRootSize adds width/height to the root <svg> from its viewBox when they
// are absent (D2 emits only a viewBox on the outer element). A root SVG without
// an intrinsic size does not render reliably when it is shown as an image file.
func ensureRootSize(svg []byte) []byte {
	s := string(svg)
	i := strings.Index(s, "<svg ")
	if i < 0 {
		return svg
	}
	end := strings.Index(s[i:], ">")
	if end < 0 {
		return svg
	}
	tag := s[i : i+end+1]
	if strings.Contains(tag, "width=") || strings.Contains(tag, "height=") {
		return svg
	}
	m := viewBoxRe.FindStringSubmatch(tag)
	if m == nil {
		return svg
	}
	f := strings.Fields(m[1]) // minx miny width height
	if len(f) != 4 {
		return svg
	}
	newTag := fmt.Sprintf(`<svg width="%s" height="%s" %s`, f[2], f[3], tag[len("<svg "):])
	return []byte(s[:i] + newTag + s[i+end+1:])
}
