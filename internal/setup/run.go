package setup

import (
	"errors"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// CommandLine is the display form of a program and its arguments: each joined
// by single spaces, with an argument holding a space, a quote or nothing at
// all written in double quotes, its double quotes and backslashes escaped by a
// backslash. It is for a reader and is never executed.
func CommandLine(program string, args []string) string {
	words := make([]string, 0, len(args)+1)
	for _, word := range append([]string{program}, args...) {
		words = append(words, displayWord(word))
	}
	return strings.Join(words, " ")
}

// displayWord quotes one word of a command line where a reader needs it.
func displayWord(word string) string {
	if word != "" && !strings.ContainsAny(word, " \"'") {
		return word
	}
	escaped := strings.ReplaceAll(word, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

// runProgram executes a run step's program with no shell between setup and
// it, in the scope's base, with standard input on the null device and both
// output streams forwarded to the writer the command named. It answers the
// step-failed refusal when the program cannot be found or exits non-zero.
func (p *planner) runProgram(planned *plannedStep) *contract.Refusal {
	id := planned.step.ID
	path, err := exec.LookPath(planned.program)
	if err != nil {
		return contract.Refuse(contract.SetupStepFailed, id+": "+planned.program+" not found")
	}
	cmd := exec.Command(path, planned.args...)
	cmd.Dir = p.base
	out := p.opts.Programs
	if out == nil {
		out = io.Discard
	}
	cmd.Stdout = out
	cmd.Stderr = out
	err = cmd.Run()
	if err == nil {
		return nil
	}
	code := -1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	}
	return contract.Refuse(contract.SetupStepFailed, id+": "+planned.program+" exited "+strconv.Itoa(code))
}

// renderCommand renders a run step's program and arguments with a set of
// facts, leaving out an argument that is exactly one placeholder whose fact
// is empty.
func renderCommand(step Step, facts Facts) (string, []string) {
	program, _ := renderText(step.Program, facts, false)
	var args []string
	for _, arg := range step.Args {
		if name, whole := wholePlaceholder(arg); whole && facts.value(name) == "" {
			continue
		}
		rendered, _ := renderText(arg, facts, false)
		args = append(args, rendered)
	}
	return program, args
}
