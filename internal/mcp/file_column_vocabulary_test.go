package mcp

import (
	"encoding/json"
	"testing"
)

// TestFileItemPublishesItsColumnVocabulary drives dinah-506/criteria/19.
// file_item was the one column-taking tool whose column parameter declared no
// vocabulary, so a client resolving column choices from the schema, which is
// what the palette wizard dinah-420 built does, could offer them for every
// other tool and not for this one.
//
// The object compared is the marshalled schema the head emits rather than the
// parameter table it was built from. The table is where the declaration was
// missing, and a test reading the table back would pass on a schemaFor that
// dropped the key on the way out.
func TestFileItemPublishesItsColumnVocabulary(t *testing.T) {
	served, known := toolsByName["file_item"]
	if !known {
		t.Fatal("this head serves no file_item tool, so this test read nothing")
	}
	encoded, err := json.Marshal(schemaFor(served))
	if err != nil {
		t.Fatalf("marshal the schema: %v", err)
	}
	var schema struct {
		Properties map[string]struct {
			Source string `json:"x-dinah-vocabulary-source"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(encoded, &schema); err != nil {
		t.Fatalf("read the schema back: %v", err)
	}
	column, published := schema.Properties["column"]
	if !published {
		t.Fatal("the file_item schema publishes no column property")
	}
	if column.Source != "columns" {
		t.Errorf("file_item.column publishes the vocabulary source %q, wanted %q", column.Source, "columns")
	}
}
