package main

import (
	"bytes"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// renderedStorage draws one storage migration account in the base language.
func renderedStorage(run *bench.StorageMigration) string {
	var out bytes.Buffer
	s := &session{out: &out, errw: &out, r: msg.For(msg.Base), width: 120}
	s.renderStorageMigration(run)
	return out.String()
}

// TestTheStorageMigrationAccountSaysWhereItStopped holds the human account of
// dinah check --migrate-storage to what the report carries, on the three
// endings a reader meets: a rehearsal, which says nothing was written; a run
// stopped at a held card, a refused rename and a proof difference after
// removal began, which names each and offers the choice of accepting the
// lines or restoring the backup; and a finished run, which says to stop the
// processes that had the store open. The report fields themselves are
// asserted by internal/bench's migration tests, which reach the stops through
// that package's failure hooks; this test is about what a person reads.
func TestTheStorageMigrationAccountSaysWhereItStopped(t *testing.T) {
	rehearsed := renderedStorage(&bench.StorageMigration{Outcome: contract.ReadOK, Rehearsal: true, From: 11, To: 12})
	if !strings.Contains(rehearsed, "Nothing was written.") {
		t.Errorf("the rehearsal's account does not say nothing was written:\n%s", rehearsed)
	}

	stopped := renderedStorage(&bench.StorageMigration{
		Outcome:        contract.ReadFindings,
		From:           11,
		To:             12,
		Phase:          bench.StoragePhaseProof,
		Held:           &bench.StorageHeldCard{Card: "fx-1", Holder: "cato"},
		Refused:        &bench.StorageRefusal{Path: "cards/aa/comments/bb/attachments", Error: "the file is in use"},
		Differences:    []bench.ManifestDifference{{Key: "comment aa bb", Before: "comment aa bb before", After: "comment aa bb after"}},
		RemovalStarted: true,
		Backup:         &bench.StorageBackup{Path: "D:/backups/fx", Digest: "sha256:00"},
		Manifest:       bench.StorageManifestSummary{Before: "sha256:11", After: "sha256:22", Lines: 3},
	})
	for _, want := range []string{"fx-1", "cato", "the file is in use", "comment aa bb", "comment aa bb before", "comment aa bb after", "D:/backups/fx"} {
		if !strings.Contains(stopped, want) {
			t.Errorf("the stopped run's account lacks %q:\n%s", want, stopped)
		}
	}

	finished := renderedStorage(&bench.StorageMigration{Outcome: contract.ReadOK, From: 11, To: 12})
	if strings.Contains(finished, "Nothing was written.") || finished == "" {
		t.Errorf("the finished run's account reads as a rehearsal or says nothing:\n%s", finished)
	}
}
