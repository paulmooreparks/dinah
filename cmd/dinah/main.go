// Command dinah is the dinah reference implementation's binary.
//
// This is a skeleton: it prints an identification line and exits. No
// contract verb, subcommand, or file format is implemented yet. That
// work belongs to a later stage of the project.
package main

import (
	"fmt"
	"io"
	"os"
)

const identification = "dinah: a reference implementation of a shared coordination contract for board-based work. Dinah Is Not A Harness. No contract verbs are implemented yet."

// printIdentification writes the identification line to w. Separated from
// main so the test can capture it without exec-ing the built binary.
func printIdentification(w io.Writer) {
	fmt.Fprintln(w, identification)
}

func main() {
	printIdentification(os.Stdout)
	os.Exit(0)
}
