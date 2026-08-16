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
	if !strings.Contains(got, "dinah") {
		t.Errorf("identification line does not mention dinah: %q", got)
	}
	if !strings.Contains(got, "No contract verbs are implemented yet") {
		t.Errorf("identification line does not note that no contract verbs are implemented: %q", got)
	}
}
