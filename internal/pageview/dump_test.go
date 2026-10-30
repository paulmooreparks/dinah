package pageview

import (
	"encoding/json"
	"os"
	"testing"
)

// TestWriteDumpedDocument writes the editor's document in DINAH_PAGE_EDITED
// (JSON, as the page sends it) as text to DINAH_PAGE_OUT, for checking an edit
// made in a browser against the real writer. Like TestDumpDocument it runs
// only on request.
func TestWriteDumpedDocument(t *testing.T) {
	in, out := os.Getenv("DINAH_PAGE_EDITED"), os.Getenv("DINAH_PAGE_OUT")
	if in == "" || out == "" {
		t.Skip("set DINAH_PAGE_EDITED and DINAH_PAGE_OUT")
	}
	data, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	var doc Node
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	text, err := SerializeDocument(&doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDumpDocument writes the editor's document and the preview of the
// document named by DINAH_PAGE_DOC to DINAH_PAGE_OUT, for looking at Page view
// beside the preview in a browser. The output holds the document's content,
// so it is written only on request and never committed.
func TestDumpDocument(t *testing.T) {
	in, out := os.Getenv("DINAH_PAGE_DOC"), os.Getenv("DINAH_PAGE_OUT")
	if in == "" || out == "" {
		t.Skip("set DINAH_PAGE_DOC and DINAH_PAGE_OUT")
	}
	data, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	blocks := previewOf(text)
	inserts := map[string]any{}
	for _, md := range []string{"```\ncode\n```"} {
		inserts[md] = Fresh(parseDoc(md).Content)
	}
	raw, err := json.Marshal(map[string]any{"text": text, "doc": parseDoc(text), "blocks": blocks, "inserts": inserts})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
