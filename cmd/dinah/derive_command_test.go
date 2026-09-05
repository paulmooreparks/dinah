package main

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

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

	for _, command := range names {
		if _, skip := exempted[command]; skip {
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
		back, refusal := reparse(command, cmd.Args)
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

// reparse drives the derived arguments back through the terminal's own parser
// and shared request helper, then fills the remaining fields the way the run
// functions do: a marker from the flag's presence, a valued flag from what it
// carried, a repeatable flag from every occurrence in order, and a positional
// from its slot in the words after the command name.
func reparse(command string, args []string) (*verb.Request, error) {
	valued := map[string]bool{}
	for _, flag := range valuedFlags {
		valued[flag] = true
	}
	parsed, refusal := parseArgs(append([]string{command}, args...), valued)
	if refusal != nil {
		return nil, refusal
	}
	s := &session{}
	req := s.request(command, parsed)
	fields := reflect.ValueOf(req).Elem()
	words := parsed.rest()
	slot := 0
	for _, p := range verb.Params(command) {
		field := fields.FieldByName(p.Field)
		switch {
		case p.Marker:
			field.SetBool(parsed.has(p.Name))
		case p.Flag && field.Kind() == reflect.Slice:
			field.Set(reflect.ValueOf(parsed.values(p.Name)))
		case p.Flag && field.Type() == reflect.TypeOf(time.Duration(0)):
			lease, err := verb.ParseDuration(parsed.value(p.Name))
			if err != nil {
				return nil, err
			}
			field.Set(reflect.ValueOf(lease))
		case p.Flag:
			field.SetString(parsed.value(p.Name))
		case p.Rest:
			field.SetString(strings.Join(words[min(slot, len(words)):], " "))
			slot = len(words)
		default:
			if slot < len(words) {
				field.SetString(words[slot])
				slot++
			}
		}
	}
	return req, nil
}
