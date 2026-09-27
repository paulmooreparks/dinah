package resident

import "strings"

// NoVolumeRoot is WorkingDirectory's error when no candidate is confirmed as
// a volume's root directory. Its Error names each attempt and why it was
// refused. dinah serve answers it by staying where it started, and says so
// on its startup line, because in this case no documented mechanism keeps
// the folder it stands in free, and serving without moving falls back, for
// the working directory alone, to what a server that never moved would do
// (dinah-619/decisions/35).
type NoVolumeRoot struct {
	// Tried holds the workbench's drive attempt and then the Windows
	// directory's, in that order.
	Tried [2]Refused
}

// Refused is one attempt. Dir is the candidate, or "" when the attempt
// produced none (the open or the query of the workbench root failed, or its
// answer had no drive element), and Why is the error or the failed
// confirmation.
type Refused struct {
	Dir string
	Why error
}

func (e *NoVolumeRoot) Error() string {
	var parts []string
	for i, attempt := range e.Tried {
		label := "the workbench's drive"
		if i == 1 {
			label = "the Windows directory's drive"
		}
		dir := attempt.Dir
		if dir == "" {
			dir = "no candidate"
		}
		why := "no reason recorded"
		if attempt.Why != nil {
			why = attempt.Why.Error()
		}
		parts = append(parts, label+" ("+dir+"): "+why)
	}
	return "resident: no volume root could be confirmed to move to: " + strings.Join(parts, "; ")
}
