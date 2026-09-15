package lsp

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"time"

	"dinah/internal/bench"
)

// The two language identifiers this server looks at. A document of any other
// language is tracked so that a later change is not a surprise, and its model
// is empty.
const (
	languageMarkdown  = "markdown"
	languagePlaintext = "plaintext"
)

// didOpen takes a document into the set the server models.
func (s *Server) didOpen(read *message) error {
	var params didOpenParams
	if err := json.Unmarshal(read.Params, &params); err != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc := &document{
		uri:        params.TextDocument.URI,
		path:       uriPath(params.TextDocument.URI),
		languageID: params.TextDocument.LanguageID,
		text:       params.TextDocument.Text,
	}
	doc.slots, doc.model = s.modelOf(doc)
	s.docs[doc.uri] = doc
	return nil
}

// didChange replaces a document's text, this server declaring full
// synchronisation. Incremental synchronisation buys nothing here: the
// documents are anchors and markdown of a few kilobytes, and the model is
// recomputed from whole text on every change either way.
func (s *Server) didChange(read *message) error {
	var params didChangeParams
	if err := json.Unmarshal(read.Params, &params); err != nil {
		return nil
	}
	if len(params.ContentChanges) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, ok := s.docs[params.TextDocument.URI]
	if !ok {
		return nil
	}
	doc.text = params.ContentChanges[len(params.ContentChanges)-1].Text
	doc.slots, doc.model = s.modelOf(doc)
	return nil
}

// didClose drops a document the editor no longer has open, which is the whole
// of what bounds this server's memory.
func (s *Server) didClose(read *message) error {
	var params documentParams
	if err := json.Unmarshal(read.Params, &params); err != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, params.TextDocument.URI)
	return nil
}

// didChangeConfiguration reads the two settings the server owns, because the
// server decides what an annotation says.
//
// Raising the interval above the last walk's duration is a legitimate way out
// of the slow-walk state, so nothing is sent from here. The next completed
// walk sends whatever edge it crosses, which keeps every message on either
// edge sent by a completed walk and leaves no second route.
func (s *Server) didChangeConfiguration(read *message) error {
	var params didChangeConfigurationParams
	if err := json.Unmarshal(read.Params, &params); err != nil {
		return nil
	}
	s.applySettings(params.Settings)
	return nil
}

// applySettings reads the server's own section out of whatever shape the
// client sent, accepting both the section on its own and a settings object
// carrying it under dinah.lsp or under nested dinah and lsp members.
func (s *Server) applySettings(raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	for _, candidate := range settingsSections(raw) {
		var read settings
		if err := json.Unmarshal(candidate, &read); err != nil {
			continue
		}
		if read.AnnotateProse == nil && read.PollIntervalSeconds == nil {
			continue
		}
		s.mu.Lock()
		if read.AnnotateProse != nil {
			s.annotateProse = *read.AnnotateProse
		}
		if read.PollIntervalSeconds != nil && *read.PollIntervalSeconds >= minimumPollSeconds {
			s.interval = time.Duration(*read.PollIntervalSeconds) * time.Second
		}
		s.mu.Unlock()
		return
	}
}

// settingsSections lists the places the server's own settings may stand in
// what a client sent, most specific first.
func settingsSections(raw json.RawMessage) []json.RawMessage {
	sections := []json.RawMessage{raw}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw, &outer); err != nil {
		return sections
	}
	if flat, ok := outer["dinah.lsp"]; ok {
		sections = append([]json.RawMessage{flat}, sections...)
	}
	if dinah, ok := outer["dinah"]; ok {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(dinah, &inner); err == nil {
			if nested, ok := inner["lsp"]; ok {
				sections = append([]json.RawMessage{nested}, sections...)
			}
		}
	}
	return sections
}

// modelOf computes one document's annotation model. The caller holds the
// lock.
//
// Section 4.1's two questions are asked here and nowhere else: whether this
// document is looked at at all, which is the scope root, and whether its
// front matter is read as a schema, which is the narrower set of anchor
// filenames under the workbench's own root.
func (s *Server) modelOf(doc *document) ([]slot, []annotation) {
	if s.bench == nil || !s.lookedAt(doc) {
		return nil, nil
	}
	lines := bench.SplitLines(doc.text)
	name := filepath.Base(doc.path)
	var slots []slot
	bodyFrom := 0
	if s.schema(doc.path) {
		slots, bodyFrom = scanFrontMatter(name, lines)
	}
	slots = append(slots, scanProse(s.bench.Slug, lines, bodyFrom)...)
	return slots, s.annotate(slots)
}

// lookedAt reports whether a document is one this server models: a file, of a
// language it reads, under the scope root. A document in an unrelated folder
// is tracked and left alone, so it is never annotated against a workbench it
// has nothing to do with.
func (s *Server) lookedAt(doc *document) bool {
	if doc.path == "" || s.scopeRoot == "" {
		return false
	}
	if doc.languageID != languageMarkdown && doc.languageID != languagePlaintext {
		return false
	}
	return under(s.scopeRoot, doc.path)
}

// schema reports whether a document's front matter is read as a schema, which
// is the narrower set: an anchor filename, under the workbench's own root. An
// attachment payload is prose only, which is right.
func (s *Server) schema(path string) bool {
	return anchorNames[filepath.Base(path)] && under(s.bench.Root, path)
}

// under reports containment, through the library's own test so the editor and
// the command line agree on what lies inside what.
func under(root, path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	contained, err := bench.PathUnderRoot(root, abs)
	return err == nil && contained
}

// recompute rebuilds every open document's model and answers the documents
// whose model actually moved. The caller holds the lock.
//
// A write on a card nothing refers to therefore produces no editor traffic at
// all, which is what keeps a poll loop from redrawing the editor twice a
// second on a busy workbench.
func (s *Server) recompute() []string {
	var moved []string
	for uri, doc := range s.docs {
		slots, rebuilt := s.modelOf(doc)
		doc.slots = slots
		if reflect.DeepEqual(rebuilt, doc.model) {
			continue
		}
		doc.model = rebuilt
		moved = append(moved, uri)
	}
	return moved
}
