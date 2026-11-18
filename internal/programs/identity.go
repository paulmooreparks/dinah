package programs

import (
	"encoding/json"
	"errors"
	"strings"
)

// IdentityCommand is the command that makes a program of a product print
// its Identity as one line of JSON and end. The window asks the programs
// beside it what they are this way before it moves one to another name
// (cmd/dinah-desktop/program_names.go). A file name and a version line
// cannot answer that question when a product gives one program's earlier
// name to another program, so every program a product installs answers it.
// A program built before this command existed fails to answer, which says
// that it is an earlier build.
const IdentityCommand = "identity"

// Identity is what a program says it is: the product it belongs to (the
// profile's ID), its own current name without an extension, and the release
// it belongs to.
type Identity struct {
	Product string `json:"product"`
	Program string `json:"program"`
	Version string `json:"version"`
}

// Line is the identity as the program prints it.
func (i Identity) Line() string {
	b, _ := json.Marshal(i)
	return string(b)
}

// ParseIdentity reads what a program printed for IdentityCommand.
func ParseIdentity(out string) (Identity, error) {
	var i Identity
	dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(out)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&i); err != nil {
		return Identity{}, errors.New("the program did not say what it is")
	}
	if i.Product == "" || i.Program == "" || i.Version == "" {
		return Identity{}, errors.New("the program's identity is incomplete")
	}
	return i, nil
}
