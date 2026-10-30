package main

import (
	"encoding/json"

	"studio/internal/pageview"
	"studio/internal/preview"
)

// The Editor's Page view edits the whole document in place (pageview). The
// page asks for the editor's document of the draft's text, sends the edited
// document back for its text as the person types, and asks for the preview
// of a block whose source was changed.

// previewOf draws Markdown as the draft's preview draws it.
func (a *App) previewOf(id string) (func(string) []preview.Block, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return nil, err
	}
	if _, err := o.Draft(id); err != nil {
		return nil, err
	}
	return func(markdown string) []preview.Block {
		blocks, _ := markdownPreview(markdown)
		return blocks
	}, nil
}

// PageDocumentOf reads a draft's text, as the page has it, for the editor.
func (a *App) PageDocumentOf(id, markdown string) (*pageview.Node, error) {
	draw, err := a.previewOf(id)
	if err != nil {
		return nil, err
	}
	return pageview.ParseDocument(markdown, pageview.Units(draw(markdown))), nil
}

// PageDocumentMarkdown writes the editor's document as the draft's text.
func (a *App) PageDocumentMarkdown(doc json.RawMessage) (string, error) {
	var n pageview.Node
	if err := json.Unmarshal(doc, &n); err != nil {
		return "", err
	}
	return pageview.SerializeDocument(&n)
}

// PageBlocksOf reads Markdown into new blocks for the editor to insert, such
// as a code block or an image. The blocks are new to the document, so they
// keep no place in it (pageview.Fresh).
func (a *App) PageBlocksOf(id, markdown string) ([]*pageview.Node, error) {
	draw, err := a.previewOf(id)
	if err != nil {
		return nil, err
	}
	doc := pageview.ParseDocument(markdown, pageview.Units(draw(markdown)))
	return pageview.Fresh(doc.Content), nil
}

// PageSource is what the page shows for a block edited as its source.
type PageSource struct {
	HTML     string            `json:"html"`
	Diagrams []preview.Diagram `json:"diagrams"`
}

// PageSourceOf draws a block's source as the preview draws it.
func (a *App) PageSourceOf(id, source string) (PageSource, error) {
	draw, err := a.previewOf(id)
	if err != nil {
		return PageSource{}, err
	}
	out := PageSource{Diagrams: []preview.Diagram{}}
	for _, u := range pageview.Units(draw(source)) {
		out.HTML += u.HTML
		out.Diagrams = append(out.Diagrams, u.Diagrams...)
	}
	return out, nil
}
