package appletcontext

import (
	"strings"
	"testing"
)

const live = "dinah-applet-v1:ABCDEFGHIJKLMNOPQRSTUVWXYZ.ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func TestRedactRemovesEveryContext(t *testing.T) {
	text := "pass " + live + " and again " + live + " ok"
	got := Redact(text)
	if strings.Contains(got, "ABCDEFGHIJ") || strings.Count(got, Redacted) != 2 || !Contains(text) || Contains(got) {
		t.Fatalf("redacted: %q", got)
	}
	if string(RedactBytes([]byte(`{"a":"`+live+`"}`))) != `{"a":"`+Redacted+`"}` {
		t.Fatal("a context in a JSON record was not replaced in place")
	}
	if Redact("dinah-applet-v1:short") != "dinah-applet-v1:short" {
		t.Fatal("text that is not a context was changed")
	}
}
