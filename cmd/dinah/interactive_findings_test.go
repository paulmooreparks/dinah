//go:build tui

package main

import (
	"strings"
	"testing"
	"unicode"
)

// TestAMalformedFilterNamesTheQueryCommand holds the code review's finding
// on the filter prompt (dinah-603/comments/17): a filter the library refuses
// is composed under the query command that reads it, so the message area
// names dinah query and its arguments, not dinah tui's.
func TestAMalformedFilterNamesTheQueryCommand(t *testing.T) {
	run := runTUIThrough(t, tuiBench(t), tuiSeam(t, strings.NewReader("/bad query ((("+keyEnter+keyCtrlC), 100, 30))
	if run.model == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	message := strings.Join(run.model.message, "\n")
	if !strings.Contains(message, "dinah query") || strings.Contains(message, "dinah tui") {
		t.Errorf("the refusal of a malformed filter reads %q, which should name dinah query and not dinah tui", message)
	}
	t.Logf("the message: %s", message)
}

// TestTheFinalPanicCarriesNoControlCharacter holds the code review's finding
// on the crash report (dinah-603/comments/17): the value the head panics
// again with after writing its report is the original's text with every
// control character replaced, so the runtime's own panic line cannot write
// an escape sequence to the terminal.
func TestTheFinalPanicCarriesNoControlCharacter(t *testing.T) {
	planted := "planted update \x1b]0;evil\x07 and \x1b[31mred\r\n"
	value := crashPanicValue(planted)
	for _, r := range value.Error() {
		if unicode.IsControl(r) {
			t.Errorf("the final panic's value %q carries the control character %U", value.Error(), r)
		}
	}
	if !strings.Contains(value.Error(), "planted update") || !strings.Contains(value.Error(), "evil") {
		t.Errorf("the final panic's value %q lost the original's text", value.Error())
	}
}
