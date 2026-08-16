package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestPrintIdentification asserts that the identification line names dinah and
// says that no contract verbs are implemented yet.
func TestPrintIdentification(t *testing.T) {
	var buf bytes.Buffer
	printIdentification(&buf)

	got := buf.String()
	if !strings.Contains(got, "dinah") {
		t.Errorf("identification line does not mention dinah: %q", got)
	}
	if !strings.Contains(got, "No contract verbs are implemented yet") {
		t.Errorf("identification line does not note that no contract verbs are implemented: %q", got)
	}
}
