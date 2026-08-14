package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintIdentification(t *testing.T) {
	var buf bytes.Buffer
	printIdentification(&buf)

	got := buf.String()
	if !strings.Contains(got, "yokoten") {
		t.Errorf("identification line does not mention yokoten: %q", got)
	}
	if !strings.Contains(got, "No protocol commands are implemented yet") {
		t.Errorf("identification line does not note that no protocol commands are implemented: %q", got)
	}
}
