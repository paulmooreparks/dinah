package main

import (
	"errors"
	"maps"
	"slices"
)

// The view a document opens in when it has none of its own yet, a choice in
// Settings › Appearance, kept by the document's media type so that more
// types can have one. Only Markdown has more than one view today: Visual,
// Text, Split and Preview, which the editor calls page, text, split and
// preview. Markdown opens in Visual unless the person chooses otherwise.

var documentViewChoices = map[string][]string{
	"text/markdown": {"page", "text", "split", "preview"},
}

var documentViewDefaults = map[string]string{"text/markdown": "page"}

// documentViews is each type's view, the person's choice over the default.
func documentViews(s Settings) map[string]string {
	out := maps.Clone(documentViewDefaults)
	for kind, view := range s.DocumentViews {
		if validDocumentView(kind, view) {
			out[kind] = view
		}
	}
	return out
}

func validDocumentView(kind, view string) bool {
	return slices.Contains(documentViewChoices[kind], view)
}

// SetDocumentView chooses the view documents of a media type open in.
func (a *App) SetDocumentView(kind, view string) error {
	if !validDocumentView(kind, view) {
		return errors.New("that view is not one this kind of document has")
	}
	if err := a.updateSettings(func(s *Settings) error {
		if s.DocumentViews == nil {
			s.DocumentViews = map[string]string{}
		}
		if view == documentViewDefaults[kind] {
			delete(s.DocumentViews, kind)
		} else {
			s.DocumentViews[kind] = view
		}
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}
