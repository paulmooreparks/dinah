package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupRedactsOwnedMigrationCopiesAndPreservesUnknownFields(t *testing.T) {
	providerTestApp(t)
	path := filepath.Join(configDir(), "settings.json"+beforeDecisions)
	fixture := `{"sessionTitles":{"deleted":"Erase this title","other":"Keep this title"},"sessionOptions":{"deleted":{"secret":"erase"}},"chatTransferHistory":[{"source":"deleted","target":"old"},{"source":"other","custom":"kept"}],"workspaceRecords":[{"id":"workspace","pins":[{"kind":"chat","session":"deleted"},{"kind":"file","path":"keep.txt"}],"extra":"keep"}],"future":{"unchanged":true}}`
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cleanupRedactBackups([]string{"deleted"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "deleted") || strings.Contains(string(data), "Erase this title") {
		t.Fatal("owned records survived", string(data))
	}
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if string(saved["future"]) != `{"unchanged":true}` || !strings.Contains(string(saved["sessionTitles"]), "Keep this title") || !strings.Contains(string(saved["workspaceRecords"]), "keep.txt") || !strings.Contains(string(saved["chatTransferHistory"]), "custom") {
		t.Fatal("unrelated rollback records changed", string(data))
	}
}
