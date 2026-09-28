package lsp

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dinah/internal/durable"
)

// TestAWaitReachesTheClientsLogAsAWarning asserts that the server surfaces a
// wait notice as window/logMessage with type 2, which the protocol names
// Warning, carrying the notice text with the path written relative to the
// workbench.
func TestAWaitReachesTheClientsLogAsAWarning(t *testing.T) {
	root := t.TempDir()
	out := &bytes.Buffer{}
	server := New(Options{Workbench: root}, strings.NewReader(""), out)
	wait := durable.Wait{
		Op:      "append",
		Path:    filepath.Join(root, "cards", "0123456789ab", "journal.ndjson"),
		Last:    errors.New("the file is in use"),
		Elapsed: 10 * time.Second,
		Notice:  2,
	}
	server.Waiting(wait)
	framed := out.String()
	_, body, found := strings.Cut(framed, "\r\n\r\n")
	if !found {
		t.Fatalf("the server wrote no frame: %q", framed)
	}
	var sent struct {
		Method string `json:"method"`
		Params struct {
			Type    int    `json:"type"`
			Message string `json:"message"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(body), &sent); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	if sent.Method != methodLogMessage || sent.Params.Type != 2 {
		t.Errorf("the notice went out as %s type %d, wanted %s type 2", sent.Method, sent.Params.Type, methodLogMessage)
	}
	if !strings.Contains(sent.Params.Message, "cards/0123456789ab/journal.ndjson") || !strings.Contains(sent.Params.Message, "10 seconds") {
		t.Errorf("the notice reads %q", sent.Params.Message)
	}
}
