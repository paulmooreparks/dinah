package main

import (
	"errors"

	"studio/internal/shortcodes"
)

/* The shortcodes a build adds to the Markdown dialect reach the content
   applets through the host's shortcode services (content-host.js), which
   ask these methods (docs/design/shortcodes.md). Dinah's own build
   registers none, and the services then say so. */

// ShortcodeCatalog lists the shortcodes this build adds, for completion,
// the Insert menu and forms.
func (a *App) ShortcodeCatalog() []shortcodes.Descriptor { return shortcodes.Catalog() }

// RenderShortcode draws one occurrence of a shortcode this build adds, as
// HTML the applet sanitises and shows in place; a container's HTML holds
// one <slot name="content"></slot> for the body the applet draws itself,
// with its frame and body marked so that the applet can draw it across
// pages, and a block's drawing says whether it is flow content
// (internal/shortcodes/pieces.go).
func (a *App) RenderShortcode(r shortcodes.Request) (shortcodes.Drawing, error) {
	return shortcodes.Draw(r)
}

// ShortcodeForm reads one occurrence, as written, as the form it is changed
// through. It returns nil when the occurrence has no form and is changed
// as text.
func (a *App) ShortcodeForm(source string) *shortcodes.Form {
	form, ok := shortcodes.FormOf(source)
	if !ok {
		return nil
	}
	return &form
}

// ShortcodeBuilt is a shortcode written from a form's values, or the value
// that could not be written and the field it is in.
type ShortcodeBuilt struct {
	Source string                 `json:"source,omitempty"`
	Error  *shortcodes.FieldError `json:"error,omitempty"`
}

// BuildShortcode writes a shortcode from a form's values. The set that owns
// the kind keeps its argument grammar, and refuses what the shortcode
// cannot hold, saying which field.
func (a *App) BuildShortcode(kind string, values map[string]string) ShortcodeBuilt {
	source, err := shortcodes.Build(kind, values)
	if err == nil {
		return ShortcodeBuilt{Source: source}
	}
	var field *shortcodes.FieldError
	if errors.As(err, &field) {
		return ShortcodeBuilt{Error: field}
	}
	if errors.Is(err, shortcodes.ErrUnknown) {
		return ShortcodeBuilt{Error: &shortcodes.FieldError{Message: "This shortcode is not changed through a form."}}
	}
	return ShortcodeBuilt{Error: &shortcodes.FieldError{Message: err.Error()}}
}
