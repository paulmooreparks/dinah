package main

import (
	"path/filepath"

	"studio/internal/statefile"
)

// The copies are kept once for the 14 November attention/settings change.
// They remain beside the live files for an explicit rollback while closed.
const beforeDecisions = ".before-decisions-2026-11-14"

func writeKeepingBeforeDecisions(path string, data []byte, validatePrevious func([]byte) error) error {
	kept, err := statefile.WriteValidatedKeeping(path, data, validatePrevious, path+beforeDecisions)
	if kept {
		logf("core", "info", "kept %s%s before changing %s; a .absent record means no previous file existed", filepath.Base(path), beforeDecisions, filepath.Base(path))
	}
	return err
}
