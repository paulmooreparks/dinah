package main

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"dinah/internal/answer"
	"dinah/internal/bench"
	"dinah/internal/verb"
)

// TestDeriveCommandRoundTrips asserts that the command line verb.DeriveCommand
// composes from a request parses back into the same request. It is the check
// that keeps the derivation honest: a line that renders every argument and
// reparses into a different call teaches its reader a command that does not do
// what they just did, which is worse for a command log than having no log.
//
// The parse half is the terminal's own. parseArgs is the function os.Args
// reaches, and (*session).request is the shared helper every run function
// builds its request through, so the flag spellings, the repeatable flag's
// occurrence order and the positional split are all decided by the code that
// decides them in production. Only the per-parameter assignment is written
// here, because each command's own run function does that in its own way and
// none of them hands its request back to a caller.
func TestDeriveCommandRoundTrips(t *testing.T) {
	exempted := verb.DerivationExemptions()
	names := append([]string(nil), verb.Commands()...)
	sort.Strings(names)
	rounded := 0
	cfg := bench.LoadConfig(t.TempDir())

	for _, command := range names {
		if _, skip := exempted[command]; skip {
			continue
		}
		// A command the terminal does not dispatch has no line a reader could
		// type, so the typed-line parser refuses it as the terminal does.
		if _, notDispatched := commandExemptions[command]; notDispatched {
			continue
		}
		declared := verb.Params(command)
		if len(declared) == 0 {
			continue
		}
		sent := sentinelRequest(t, command, declared)
		cmd, ok, reason := verb.DeriveCommand(sent)
		if !ok {
			t.Errorf("%s: refused derivation: %s", command, reason)
			continue
		}
		back, refusal := reparse(cfg, command, cmd.Args)
		if refusal != nil {
			t.Errorf("%s: the derived line %q was refused on the way back in: %v", command, cmd.Line(), refusal)
			continue
		}
		for _, p := range declared {
			want := reflect.ValueOf(sent).Elem().FieldByName(p.Field)
			got := reflect.ValueOf(back).Elem().FieldByName(p.Field)
			if !reflect.DeepEqual(want.Interface(), got.Interface()) {
				t.Errorf("%s: the line %q reparsed %s as %#v, want %#v",
					command, cmd.Line(), p.Field, got.Interface(), want.Interface())
			}
		}
		rounded++
	}
	if rounded == 0 {
		t.Fatal("no command was round-tripped, so this check read nothing")
	}
}

// sentinelRequest builds a request carrying a distinct recognizable value on
// every field the command's parameters name. The value is chosen by the
// field's Go type, and a string carries the parameter's own name so that a
// reparse landing two positionals in each other's slots is caught; one shared
// string would compare equal either way.
func sentinelRequest(t *testing.T, command string, declared []verb.Param) *verb.Request {
	t.Helper()
	req := &verb.Request{Verb: command}
	fields := reflect.ValueOf(req).Elem()
	for _, p := range declared {
		field := fields.FieldByName(p.Field)
		if !field.IsValid() {
			t.Fatalf("%s: the parameter %q names the field %q, which verb.Request does not have", command, p.Name, p.Field)
		}
		switch {
		case field.Type() == reflect.TypeOf(time.Duration(0)):
			field.Set(reflect.ValueOf(8 * time.Hour))
		case field.Kind() == reflect.Bool:
			field.SetBool(true)
		case field.Kind() == reflect.String:
			field.SetString("sentinel-" + p.Name)
		case field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.String:
			field.Set(reflect.ValueOf([]string{
				"sentinel-" + p.Name + "-a=sentinel-" + p.Name + "-b",
				"sentinel-" + p.Name + "-c=sentinel-" + p.Name + "-d",
			}))
		default:
			t.Fatalf("%s: the parameter %q names a field of type %s, which this check has no sentinel for", command, p.Name, field.Type())
		}
	}
	return req
}

// reparse drives the derived words back through the pages' typed-line parser,
// which runs the terminal's own parse steps, and builds the request with
// answer.Build, which is what the pages' command log performs a typed line
// through. So the parse a log line is tested against is the parse a line
// typed into the log runs.
func reparse(cfg *bench.Config, command string, args []string) (*verb.Request, error) {
	typed, refusal := parseTypedLine(cfg, append([]string{command}, args...))
	if refusal != nil {
		return nil, refusal
	}
	return answer.Build(typed.Command, typed.Arguments), nil
}
