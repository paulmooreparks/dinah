package mcp

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// TestTheChangesToolCarriesTheColumnsMember asserts the machine head's half of
// dinah-515 criterion 21: the changes tool carries the new columns member when
// a column anchor moved by hand, and carries none when nothing did.
//
// A hand-edited column is the case the widened checkpoint exists for. It
// writes no journal line, so an answer reporting it is evidence that the third
// digest term moved rather than that any card did, and the cards array staying
// empty is the second half of that.
func TestTheChangesToolCarriesTheColumnsMember(t *testing.T) {
	library := newLibrary(t)

	minted := changesPayload(t, library, `{}`)
	token, ok := minted["cursor"].(string)
	if !ok || token == "" {
		t.Fatalf("the tool minted no cursor: %v", minted)
	}
	if columns, carried := minted["columns"]; carried {
		t.Errorf("a minting call carried the columns member: %v", columns)
	}

	encoded, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("marshal the cursor: %v", err)
	}
	quiet := changesPayload(t, library, `{"since":`+string(encoded)+`}`)
	if columns, carried := quiet["columns"]; carried {
		t.Errorf("a workbench nobody touched carried the columns member: %v", columns)
	}

	column := library.Bench.Columns[0]
	anchor := library.Bench.ColumnAnchorPath(column.ID)
	raw, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the column anchor: %v", err)
	}
	edited := strings.Replace(string(raw), "title: "+column.Title, "title: "+column.Title+"RENAMED", 1)
	if edited == string(raw) {
		t.Fatalf("the column anchor carries no title line to edit:\n%s", raw)
	}
	if err := os.WriteFile(anchor, []byte(edited), 0o644); err != nil {
		t.Fatalf("write the column anchor: %v", err)
	}
	opened, err := bench.Open(library.Bench.Root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	reopened := verb.New(opened, "")

	moved := changesPayload(t, reopened, `{"since":`+string(encoded)+`}`)
	if moved["changed"] != true {
		t.Fatalf("a hand-edited column anchor reported no change: %v", moved)
	}
	if cards, carried := moved["cards"]; carried {
		t.Errorf("a column edit resynced the cards array: %v", cards)
	}
	reported, carried := moved["columns"].([]any)
	if !carried || len(reported) == 0 {
		t.Fatalf("the answer carries no columns member: %v", moved)
	}
	named := false
	for _, entry := range reported {
		row, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if row["id"] == column.ID && row["title"] == column.Title+"RENAMED" {
			named = true
			if _, occupancy := row["count"]; occupancy {
				t.Error("the entry carries an occupancy count, and a truthful one costs a read of every card anchor")
			}
		}
	}
	if !named {
		t.Errorf("no entry names the column that was edited: %v", reported)
	}
}
