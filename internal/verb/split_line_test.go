package verb

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"dinah/internal/contract"
)

// adversarialValues are the argument values TestSplitLineInvertsLine drives
// through Line and back: each one either makes quoteArgument quote, escapes a
// quote, ends in a run of backslashes, or names a character a shell would
// expand, which SplitLine must keep as it stands.
var adversarialValues = []string{
	" ",
	`"`,
	`a"b`,
	`trailing\`,
	`\\"`,
	`C:\temp\`,
	"$HOME",
	"%PATH%",
	"'single'",
	"`tick`",
	"{a,b}",
	"caf\u00e9",
}

// TestSplitLineInvertsLine holds SplitLine to quoteArgument's quoting: for
// every command DeriveCommand derives, a request whose every string field
// carries each adversarial value in turn renders a line that SplitLine takes
// back apart into the verb and the arguments DeriveCommand produced. A
// Required parameter is also driven with the empty string, which Line writes
// as a pair of quotes.
func TestSplitLineInvertsLine(t *testing.T) {
	names := append([]string(nil), Commands()...)
	sort.Strings(names)
	values := append(append([]string(nil), adversarialValues...), "")
	pairs, commands := 0, 0
	for _, command := range names {
		if _, exempt := derivationExemptions[command]; exempt || len(params[command]) == 0 {
			continue
		}
		commands++
		for _, value := range values {
			req := &Request{Verb: command}
			fields := reflect.ValueOf(req).Elem()
			for _, p := range params[command] {
				field := fields.FieldByName(p.Field)
				switch {
				case field.Type() == reflect.TypeOf(time.Duration(0)):
					field.Set(reflect.ValueOf(2 * time.Hour))
				case field.Kind() == reflect.Bool:
					field.SetBool(true)
				case field.Kind() == reflect.String:
					if value == "" && !p.Required {
						continue
					}
					field.SetString(value)
				case field.Kind() == reflect.Slice:
					if value != "" {
						field.Set(reflect.ValueOf([]string{value, value}))
					}
				}
			}
			cmd, ok, reason := DeriveCommand(req)
			if !ok {
				t.Fatalf("%s: DeriveCommand refused: %s", command, reason)
			}
			got, err := SplitLine(cmd.Line())
			if err != nil {
				t.Errorf("%s with %q: SplitLine(%q) refused: %v", command, value, cmd.Line(), err)
				continue
			}
			want := append([]string{cmd.Verb}, cmd.Args...)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s with %q: SplitLine(%q) = %q, want %q", command, value, cmd.Line(), got, want)
			}
			pairs++
		}
	}
	if commands == 0 || pairs != commands*len(values) {
		t.Fatalf("checked %d pairs over %d commands, wanted %d", pairs, commands, commands*len(values))
	}
	t.Logf("checked %d lines over %d commands and %d values", pairs, commands, len(values))
}

// TestSplitLineRefuses holds the two refusals beside the nearest line each
// accepts: an open quote beside the same quote closed, and a line break
// beside the same text on one line.
func TestSplitLineRefuses(t *testing.T) {
	for _, row := range []struct {
		refused, accepted string
		detail            string
		words             []string
	}{
		{refused: `comment pl-1 "open`, accepted: `comment pl-1 "open"`, detail: `"`, words: []string{"comment", "pl-1", "open"}},
		{refused: "comment pl-1 a\nb", accepted: "comment pl-1 a b", detail: `\n`, words: []string{"comment", "pl-1", "a", "b"}},
		{refused: "comment pl-1 a\rb", accepted: "comment pl-1 ab", detail: `\n`, words: []string{"comment", "pl-1", "ab"}},
	} {
		_, err := SplitLine(row.refused)
		refusal, ok := err.(*contract.Refusal)
		if !ok || refusal.Name != contract.Usage || refusal.Detail != row.detail {
			t.Errorf("SplitLine(%q) = %v, want %s with detail %q", row.refused, err, contract.Usage, row.detail)
		}
		words, err := SplitLine(row.accepted)
		if err != nil || !reflect.DeepEqual(words, row.words) {
			t.Errorf("SplitLine(%q) = %q, %v, want %q", row.accepted, words, err, row.words)
		}
	}
	for line, want := range map[string][]string{
		`--kind="external dep"`: {"--kind=external dep"},
		`a "" b`:                {"a", "", "b"},
		"  a\t\tb  ":            {"a", "b"},
		`a\b`:                   {`a\b`},
		`"a\\" b`:               {`a\`, "b"},
		`"\"x\""`:               {`"x"`},
		`it's $HOME %P% !^`:     {"it's", "$HOME", "%P%", "!^"},
		"":                      nil,
	} {
		got, err := SplitLine(line)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("SplitLine(%q) = %q, %v, want %q", line, got, err, want)
		}
	}
}
