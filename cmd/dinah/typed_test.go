package main

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"dinah/internal/answer"
	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/httphead"
	"dinah/internal/verb"
)

// TestTypedLineRoundTripsEveryRoutedCommand holds the typed line to the log
// line: for every command the HTTP head routes, a request carrying a sentinel
// in every field renders as a line, the line splits back into its words, the
// words parse through the terminal's own steps, and answer.Build gives back a
// request whose every bound field holds what was sent. version is the one
// routed command DeriveCommand refuses, because its one parameter binds no
// field, so it is walked through the parse alone and must build a version
// request.
func TestTypedLineRoundTripsEveryRoutedCommand(t *testing.T) {
	cfg := bench.LoadConfig(t.TempDir())
	routed := httphead.RoutedCommands()
	sort.Strings(routed)
	walked := 0
	for _, command := range routed {
		walked++
		if _, exempt := verb.DerivationExemptions()[command]; exempt {
			typed, refusal := parseTypedLine(cfg, []string{"dinah", command})
			if command != "version" || refusal != nil || answer.Build(typed.Command, typed.Arguments).Verb != command {
				t.Errorf("%s: a routed command DeriveCommand refuses, and the bare line came back %+v %v", command, typed, refusal)
			}
			continue
		}
		declared := verb.Params(command)
		sent := sentinelRequest(t, command, declared)
		cmd, ok, reason := verb.DeriveCommand(sent)
		if !ok {
			t.Errorf("%s: DeriveCommand refused: %s", command, reason)
			continue
		}
		words, err := verb.SplitLine("dinah " + cmd.Line())
		if err != nil {
			t.Errorf("%s: SplitLine refused %q: %v", command, cmd.Line(), err)
			continue
		}
		typed, refusal := parseTypedLine(cfg, words)
		if refusal != nil {
			t.Errorf("%s: the line %q was refused: %v", command, cmd.Line(), refusal)
			continue
		}
		back := answer.Build(typed.Command, typed.Arguments)
		for _, p := range declared {
			if p.Field == "" {
				continue
			}
			want := reflect.ValueOf(sent).Elem().FieldByName(p.Field).Interface()
			got := reflect.ValueOf(back).Elem().FieldByName(p.Field).Interface()
			if !reflect.DeepEqual(want, got) {
				t.Errorf("%s: the line %q came back with %s %#v, want %#v", command, cmd.Line(), p.Field, got, want)
			}
		}
	}
	if walked != 20 {
		t.Errorf("walked %d routed commands, wanted the twenty dinah-152 routes", walked)
	}
	t.Logf("walked %d routed commands", walked)
}

// TestTypedLineRefuses holds each refusal of the typed line beside the
// nearest line it accepts.
func TestTypedLineRefuses(t *testing.T) {
	home := t.TempDir()
	cfg := bench.LoadConfig(home)
	parse := func(line string) (TypedCommand, *contract.Refusal) {
		words, err := verb.SplitLine(line)
		if err != nil {
			return TypedCommand{}, err.(*contract.Refusal)
		}
		return parseTypedLine(cfg, words)
	}
	for _, row := range []struct {
		refused, refusal, detail, accepted string
	}{
		{"frobnicate fx-1", contract.UnknownVerb, "frobnicate", "claim fx-1"},
		{"claim fx-1 --workbench x", contract.Usage, "--workbench", "claim fx-1 --actor claude"},
		{"claim fx-1 --lang de", contract.Usage, "--lang", "claim fx-1"},
		{"claim fx-1 --json", contract.Usage, "--json", "claim fx-1"},
		{"claim fx-1 --quiet", contract.Usage, "--quiet", "claim fx-1"},
		{"claim fx-1 --format json", contract.Usage, "--format", "claim fx-1"},
		{"claim fx-1 --help", contract.Usage, "--help", "claim fx-1"},
		{"claim fx-1 -h", contract.Usage, "--help", "claim fx-1"},
		{"claim fx-1 --version", contract.Usage, "--version", "claim fx-1"},
		{"comment fx-1 -", contract.Usage, "-", "comment fx-1 dash"},
		{"comment fx-1 two words", contract.MultipleWords, "", `comment fx-1 "two words"`},
		{"claim fx-1 --column doing", contract.Usage, "--column", "move fx-1 --column doing"},
		{"claim fx-1 stray", contract.Usage, "stray", "dinah claim fx-1"},
	} {
		if _, refusal := parse(row.refused); refusal == nil || refusal.Name != row.refusal || (row.detail != "" && refusal.Detail != row.detail) {
			t.Errorf("%q: wanted %s %q, got %v", row.refused, row.refusal, row.detail, refusal)
		}
		if typed, refusal := parse(row.accepted); refusal != nil || typed.Command == "" {
			t.Errorf("%q: wanted it accepted, got %v", row.accepted, refusal)
		}
	}
	if typed, _ := parse("claim fx-1 --actor claude"); typed.Actor != "claude" || typed.Arguments["card"] != "fx-1" {
		t.Errorf("--actor: %+v", typed)
	}
	if typed, _ := parse(`comment fx-1 "two words"`); typed.Arguments["text"] != "two words" {
		t.Errorf("a quoted free text: %+v", typed)
	}
	aliased := bench.LoadConfig(home)
	if err := aliased.SetAlias(bench.AliasPrefix+"mv", "move $1 $2", true); err != nil {
		t.Fatalf("set the alias: %v", err)
	}
	aliased = bench.LoadConfig(home)
	typed, refusal := parseTypedLine(aliased, []string{"mv", "fx-1", "doing"})
	if refusal != nil || typed.Command != "move" || typed.Arguments["card"] != "fx-1" || typed.Arguments["column"] != "doing" {
		t.Errorf("an alias: wanted move fx-1 doing, got %+v %v", typed, refusal)
	}
	if !strings.Contains(verb.Usage("move"), "column") {
		t.Fatal("move declares no column")
	}
}
