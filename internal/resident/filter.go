package resident

import (
	"fmt"
	"strings"
)

// This file holds the portable half of the Windows watch on the root
// directory of the workbench's volume (section 4.6 of dinah-619's
// specification): classify, which judges the shape of the two final paths
// Windows gives for the workbench root, and place, which turns one record
// named relative to the volume root into a change relative to the workbench
// root. Both are pure, so they are tested on every platform.

// volumeGUIDPrefix is how a volume GUID path begins. "Naming a Volume" gives
// the documented shape, "\\?\Volume{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}\",
// and GetFinalPathNameByHandle with VOLUME_NAME_GUID says "the returned path
// will begin with a volume GUID path formatted like" it.
const volumeGUIDPrefix = `\\?\Volume{`

// volumeRootLength is the length of a volume GUID path with its trailing
// backslash: the prefix, the thirty-six characters of the GUID, and "}\".
const volumeRootLength = len(volumeGUIDPrefix) + 36 + 2

// volumeRootOf answers the volume GUID path a final path begins with, its
// trailing backslash included, and the rest of the path after it, or false
// when the path does not begin with one of the documented shape.
func volumeRootOf(path string) (string, string, bool) {
	if len(path) < volumeRootLength || !strings.HasPrefix(path, volumeGUIDPrefix) {
		return "", "", false
	}
	guid := path[len(volumeGUIDPrefix) : len(volumeGUIDPrefix)+36]
	if path[len(volumeGUIDPrefix)+36:volumeRootLength] != `}\` {
		return "", "", false
	}
	for i := 0; i < len(guid); i++ {
		c := guid[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return "", "", false
			}
		default:
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
				return "", "", false
			}
		}
	}
	return path[:volumeRootLength], path[volumeRootLength:], true
}

// dosDriveOf answers the drive element of a final path of the form
// "\\?\X:" followed by a backslash or nothing, and the rest after that
// backslash, or false for any other shape. GetFinalPathNameByHandle with
// VOLUME_NAME_DOS: "Return the path with the drive letter."
func dosDriveOf(path string) (string, string, bool) {
	const verbatim = `\\?\`
	if len(path) < len(verbatim)+2 || !strings.HasPrefix(path, verbatim) {
		return "", "", false
	}
	letter, colon := path[len(verbatim)], path[len(verbatim)+1]
	if !(letter >= 'A' && letter <= 'Z' || letter >= 'a' && letter <= 'z') || colon != ':' {
		return "", "", false
	}
	drive := path[len(verbatim) : len(verbatim)+2]
	rest := path[len(verbatim)+2:]
	switch {
	case rest == "":
		return drive, "", true
	case rest[0] == '\\':
		return drive, rest[1:], true
	}
	return "", "", false
}

// elementsOf splits the rest of a final path into its elements, none for an
// empty rest.
func elementsOf(rest string) []string {
	if rest == "" {
		return nil
	}
	return strings.Split(rest, `\`)
}

// classify judges the two final paths Windows gives for the workbench root,
// by the first row of this table that fits:
//
//   - guidFinal does not begin with a volume GUID path of the documented
//     shape: an error naming it.
//   - dosFinal does not begin with \\?\ and a drive element: an error naming
//     it.
//   - dosFinal's elements after the drive element equal guidFinal's elements
//     after the volume root, one for one, compared exactly: the volume root,
//     backslash included, and the long form of each element of the path from
//     it to the workbench root.
//   - dosFinal has more elements than guidFinal and ends with all of
//     guidFinal's: WhyMountedInFolder. The volume holding the workbench is
//     reached through a folder of the volume whose drive letter dosFinal
//     names, so the short names of the elements before the match belong to
//     no path on the watched volume.
//   - anything else: an error naming both.
//
// The comparison is exact because both strings come from one handle and one
// normalisation, so a difference, even of case, means the pair cannot be
// trusted.
func classify(guidFinal, dosFinal string) (string, []string, Why, error) {
	volumeRoot, guidRest, ok := volumeRootOf(guidFinal)
	if !ok {
		return "", nil, "", fmt.Errorf("resident: the workbench's final path %q does not begin with a volume GUID path", guidFinal)
	}
	_, dosRest, ok := dosDriveOf(dosFinal)
	if !ok {
		return "", nil, "", fmt.Errorf("resident: the workbench's final path %q does not begin with a drive", dosFinal)
	}
	long, dos := elementsOf(guidRest), elementsOf(dosRest)
	if equalElements(dos, long) {
		return volumeRoot, long, "", nil
	}
	if len(dos) > len(long) && equalElements(dos[len(dos)-len(long):], long) {
		return "", nil, WhyMountedInFolder, nil
	}
	return "", nil, "", fmt.Errorf("resident: the workbench's final paths %q and %q do not name one path on one volume", guidFinal, dosFinal)
}

// equalElements reports whether two element lists are equal, exactly.
func equalElements(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// element is one folder of the path from the volume root to the workbench
// root, in its long form and its short form (equal where it has no short
// form).
type element struct{ long, alias string }

// placed says what one record means to the workbench.
type placed int

const (
	// elsewhere: outside the workbench and above nothing of it; dropped.
	elsewhere placed = iota
	// inside: below the workbench root; a Change relative to the root.
	inside
	// moved: the workbench root or a folder above it was created, removed
	// or renamed.
	moved
)

// place judges one record against the prefix, the elements of the path from
// the volume root to the workbench root. name is the record's FileName,
// "relative to the directory handle", which is the volume root, and separated
// by backslashes.
//
// An element of the record matches an element of the prefix when it equals
// either of its names under strings.EqualFold. The comparison folds case and
// accepts either name because the two ways it can be wrong cost different
// amounts: a false match hands the applier a path it reconciles against the
// disk as it stands, which finds the workbench's own files as they are, or
// costs one rebuild, and a false miss would lose a change. So it errs toward
// matching, and assumes nothing about how the volume folds case or which name
// a record carries ("If there is both a short and long name for the file, the
// function will return one of these names, but it is unspecified which one").
func place(prefix []element, name string, action Action) (Change, placed) {
	r := strings.Split(name, `\`)
	k := len(prefix)
	n := len(r)
	if n > k {
		n = k
	}
	for i := 0; i < n; i++ {
		if !strings.EqualFold(r[i], prefix[i].long) && !strings.EqualFold(r[i], prefix[i].alias) {
			return Change{}, elsewhere
		}
	}
	if len(r) > k {
		return Change{Path: strings.Join(r[k:], `\`), Action: action}, inside
	}
	if action == Modified {
		// A Modified on a folder is a time stamp or attribute change, and
		// every change to the root's entries is reported under its own name,
		// which the row above takes.
		return Change{}, elsewhere
	}
	return Change{}, moved
}
