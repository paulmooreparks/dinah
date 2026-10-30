package broker

import (
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strings"
)

var svgColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// ValidateSVG accepts an SVG artifact only when it is a plain drawing of
// rectangles and text, the shapes the bundled applets draw, so that an
// artifact shown or saved can carry no script, link or external reference.
func ValidateSVG(svg string) error {
	if len(svg) == 0 || len(svg) > 4*1024*1024 {
		return errors.New("the SVG artifactis empty or exceeds 4 MB")
	}
	d := xml.NewDecoder(strings.NewReader(svg))
	depth, roots := 0, 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.New("the SVG artifactis invalid")
		}
		switch t := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if t.Name.Local != "svg" || t.Name.Space != "http://www.w3.org/2000/svg" || roots != 1 {
					return errors.New("the SVG artifact must be an SVG document")
				}
			}
			if t.Name.Space != "http://www.w3.org/2000/svg" || (t.Name.Local != "svg" && t.Name.Local != "rect" && t.Name.Local != "text") {
				return errors.New("the SVG artifactcontains unsupported elements")
			}
			for _, attr := range t.Attr {
				allowed := map[string]bool{"xmlns": true, "width": true, "height": true, "viewBox": true, "shape-rendering": true, "x": true, "y": true, "fill": true, "font-family": true, "font-size": true, "font-weight": true, "text-anchor": true}
				if !allowed[attr.Name.Local] || attr.Name.Space != "" {
					return errors.New("the SVG artifactcontains unsupported attributes")
				}
				if attr.Name.Local == "fill" && !svgColor.MatchString(attr.Value) {
					return errors.New("the SVG artifactcontains an unsupported color")
				}
			}
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(t)) != "" {
				return errors.New("the SVG artifacthas text outside its root")
			}
		case xml.Directive, xml.ProcInst:
			return errors.New("the SVG artifactcontains unsupported declarations")
		}
	}
	if roots != 1 || depth != 0 {
		return errors.New("the SVG artifactis incomplete")
	}
	return nil
}
