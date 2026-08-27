package main

import (
	"io"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// outputFormat is which projection of a verb's answer an invocation wants.
// The set is closed, because the head renders one of exactly three things and
// a value outside the set is refused rather than guessed at.
type outputFormat string

// The three projections. The human rendering is the zero value, so a session
// nobody sets a format on renders for a person.
const (
	formatHuman   outputFormat = ""
	formatJSON    outputFormat = "json"
	formatCompact outputFormat = "compact"
)

// resolveFormat decides which projection an invocation gets, from the --json
// marker, the --format value and DINAH_FORMAT, and returns the refusal where
// the three do not name a projection between them.
//
// The ladder is bench.Resolve's, the same primitive the workbench and the
// actor resolve through, so flag beats environment here for the reason it
// beats it everywhere else and no second ladder has to be learned.
//
// --json survives as its own marker flag rather than being rewritten to
// --format json, so a script that has always written it parses, helps and
// exits exactly as before. Written together with --format json it is
// redundant and allowed; written together with any other --format value it is
// two flags naming different projections, which is a misuse of the command
// line rather than an unrecognised value, so it takes contract.Usage where an
// unrecognised value takes contract.UnknownFormat.
func resolveFormat(jsonFlag bool, formatFlag, environment string) (outputFormat, error) {
	if jsonFlag && formatFlag != "" && formatFlag != string(formatJSON) {
		return formatHuman, contract.Refuse(contract.Usage, "--json conflicts with --format "+formatFlag)
	}
	flagValue := formatFlag
	if jsonFlag {
		flagValue = string(formatJSON)
	}
	resolved, _ := bench.Resolve(
		bench.Layer{Source: bench.SourceFlag, Value: flagValue},
		bench.Layer{Source: bench.SourceEnvironment, Value: environment},
	)
	switch outputFormat(resolved) {
	case formatHuman, formatJSON, formatCompact:
		return outputFormat(resolved), nil
	}
	return formatHuman, contract.Refuse(contract.UnknownFormat, resolved)
}

// emitMachine writes a value in whichever machine form the invocation asked
// for: the compact projection where one is defined for the value's own type,
// and the canonical JSON everywhere else.
//
// The fallback is per type rather than a blanket switch, so a later card
// defining a compact rendering for one more shape adds a case to
// compactEncode and changes nothing here.
func (s *session) emitMachine(value any) int {
	if s.format == formatCompact {
		if data, ok := compactEncode(value); ok {
			io.WriteString(s.out, data)
			return exitCodeFor(value)
		}
	}
	if code := s.emitCanonical(value); code != 0 {
		return code
	}
	return exitCodeFor(value)
}

// exitCodeFor is the exit code a machine answer carries: the outcome's own
// code for a verb's response, and zero for every read, whose answer reports no
// outcome to take a code from. It is the code each call site computed for
// itself before emitMachine took the branch over.
func exitCodeFor(value any) int {
	if response, ok := value.(*verb.Response); ok {
		return contract.ExitCode(response.Outcome)
	}
	return 0
}
