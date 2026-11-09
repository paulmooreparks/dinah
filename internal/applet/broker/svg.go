package broker

import (
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strings"
)

const (
	svgNS   = "http://www.w3.org/2000/svg"
	xlinkNS = "http://www.w3.org/1999/xlink"
	xhtmlNS = "http://www.w3.org/1999/xhtml"
	xmlNS   = "http://www.w3.org/XML/1998/namespace"
)

// The elements an SVG artifact may not hold, in any namespace: the ones that
// run code, load another document, or take input. Everything else in the
// SVG namespace, and plain markup in XHTML inside foreignObject (as Mermaid
// draws its labels), is a drawing.
var svgRefusedElements = map[string]bool{
	"script": true, "iframe": true, "frame": true, "object": true, "embed": true, "applet": true,
	"link": true, "meta": true, "base": true, "form": true, "input": true, "button": true,
	"select": true, "textarea": true, "audio": true, "video": true, "source": true, "track": true,
}

// A reference an SVG artifact may make: to a part of itself, or to data it
// carries that is a picture or a font. Anything else, such as a web address
// or a file, is external, and an artifact shown or saved may load nothing
// from outside itself.
var svgSafeReference = regexp.MustCompile(`(?i)^\s*(#|data:image/|data:font/|data:application/font|data:application/x-font|data:application/octet-stream;base64,)`)

// The CSS an SVG artifact's style may hold: no imports, no behaviours, and
// url() only to a part of the drawing or to data it carries.
var cssUnsafe = regexp.MustCompile(`(?i)@import|expression\s*\(|-moz-binding|behavior\s*:`)
var cssURL = regexp.MustCompile(`(?i)url\(\s*['"]?([^'")]*)`)

// ValidateSVG accepts an SVG artifact when it is a drawing and nothing more:
// one SVG document, with no script, no event handler, no reference to
// anything outside itself, and no declaration that could define entities.
// The artifact window shows it as an image, where none of those would act;
// the check is for the file a person saves and opens elsewhere, which is
// shown by the bundled applets' own drawings (D2, Mermaid, a canvas) passing.
func ValidateSVG(svg string) error {
	if len(svg) == 0 || len(svg) > 4*1024*1024 {
		return errors.New("the SVG artifact is empty or exceeds 4 MB")
	}
	d := xml.NewDecoder(strings.NewReader(svg))
	d.Strict = true
	depth, roots := 0, 0
	inStyle := 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.New("the SVG artifact is not well-formed XML")
		}
		switch t := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if t.Name.Local != "svg" || t.Name.Space != svgNS || roots != 1 {
					return errors.New("the SVG artifact must be one SVG document")
				}
			}
			if t.Name.Space != svgNS && t.Name.Space != xhtmlNS {
				return errors.New("the SVG artifact holds an element of an unsupported namespace")
			}
			if svgRefusedElements[strings.ToLower(t.Name.Local)] {
				return errors.New("the SVG artifact holds a " + t.Name.Local + " element, which a drawing may not")
			}
			if t.Name.Local == "style" {
				inStyle++
			}
			for _, attr := range t.Attr {
				if err := checkSVGAttribute(attr); err != nil {
					return err
				}
			}
			depth++
		case xml.EndElement:
			depth--
			if t.Name.Local == "style" && inStyle > 0 {
				inStyle--
			}
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(t)) != "" {
				return errors.New("the SVG artifact has text outside its root")
			}
			if inStyle > 0 {
				if err := checkCSS(string(t)); err != nil {
					return err
				}
			}
		case xml.Directive:
			return errors.New("the SVG artifact holds a declaration, which a drawing may not")
		case xml.ProcInst:
			if t.Target != "xml" {
				return errors.New("the SVG artifact holds a processing instruction, which a drawing may not")
			}
		}
	}
	if roots != 1 || depth != 0 {
		return errors.New("the SVG artifact is incomplete")
	}
	return nil
}

func checkSVGAttribute(attr xml.Attr) error {
	name := strings.ToLower(attr.Name.Local)
	switch attr.Name.Space {
	case "", "xmlns", xlinkNS, xmlNS:
	default:
		return errors.New("the SVG artifact holds an attribute of an unsupported namespace")
	}
	if strings.HasPrefix(name, "on") {
		return errors.New("the SVG artifact holds an event handler, which a drawing may not")
	}
	switch name {
	case "href", "src", "srcset", "poster", "action", "formaction", "ping", "manifest", "data", "codebase":
		if !svgSafeReference.MatchString(attr.Value) {
			return errors.New("the SVG artifact refers to something outside itself")
		}
	case "style":
		return checkCSS(attr.Value)
	}
	return nil
}

func checkCSS(css string) error {
	if cssUnsafe.MatchString(css) {
		return errors.New("the SVG artifact's styles hold an import or behaviour, which a drawing may not")
	}
	for _, m := range cssURL.FindAllStringSubmatch(css, -1) {
		if !svgSafeReference.MatchString(m[1]) {
			return errors.New("the SVG artifact's styles refer to something outside it")
		}
	}
	return nil
}
