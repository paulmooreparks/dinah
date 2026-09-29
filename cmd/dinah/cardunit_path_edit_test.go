package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/testenv"
)

// TestPathAndEditTreatAMemberAsLinesOfItsJournal drives dinah-637/criteria/16
// on a store in the card-unit layout. dinah path refuses a comment and an item
// dinah.not-a-file and still answers a comment's attachment payload. dinah
// edit hands the editor a copy of a comment's body, writes the changed body
// through the ordinary body write, refuses the save when somebody else's write
// landed while the editor was open, and leaves no copy behind either way.
//
// Arming: dropping the expected digest editMember sets on the body write lets
// the second edit overwrite the write that landed mid-edit, which reddens the
// refusal assertion and the body read after it.
func TestPathAndEditTreatAMemberAsLinesOfItsJournal(t *testing.T) {
	bench.EnableCardUnitForTest(t)
	root := newBench(t)
	card := addCard(t, root, "a card whose comment is edited")
	if got := runCLI(t, root, "comment", card, "before"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", card, "acceptance_criterion", "the endpoint answers 404"); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	source := filepath.Join(t.TempDir(), "evidence.txt")
	if err := os.WriteFile(source, []byte("the evidence"), 0o644); err != nil {
		t.Fatalf("write the attachment's source: %v", err)
	}
	if got := runCLI(t, root, "attach", card+"/comments/1", source); got.code != 0 {
		t.Fatalf("attach: %d %s", got.code, got.errw)
	}

	refused := contract.ExitCode(contract.OutcomeRefused)
	for _, ref := range []string{card + "/comments/1", card + "/criteria/1"} {
		got := runCLI(t, root, "path", ref)
		if got.code != refused || !strings.HasPrefix(strings.TrimSpace(got.errw), contract.NotAFile) {
			t.Errorf("`dinah path %s` exited %d with %q, wanted %s", ref, got.code, got.errw, contract.NotAFile)
		}
	}
	payload := runCLI(t, root, "path", card+"/comments/1/attachments/1/payload")
	if payload.code != 0 {
		t.Fatalf("path of the payload: %d %s", payload.code, payload.errw)
	}
	if bytes, err := os.ReadFile(strings.TrimSpace(payload.out)); err != nil || string(bytes) != "the evidence" {
		t.Errorf("the payload path answers %q (%v), wanted the attached bytes", bytes, err)
	}

	logs := t.TempDir()
	t.Setenv("DINAH_EDITOR", os.Args[0])
	edit := func(name, line string) (invocation, string) {
		t.Helper()
		log := filepath.Join(logs, name+".log")
		t.Setenv(editorRecordVar, log)
		t.Setenv(testenv.EditorAppendVar, line)
		got := runCLI(t, root, "edit", card+"/comments/1")
		launched := editorLaunches(t, log)
		if len(launched) != 1 {
			t.Fatalf("%s: the edit launched the editor %d times, wanted once", name, len(launched))
		}
		if _, err := os.Stat(filepath.Dir(launched[0])); !os.IsNotExist(err) {
			t.Errorf("%s: the directory holding the copy is still there: %v", name, err)
		}
		return got, launched[0]
	}

	got, _ := edit("the author's edit", "and the author's addition")
	if got.code != 0 {
		t.Fatalf("the edit: %d %s", got.code, got.errw)
	}
	if body := shownCommentBody(t, root, card+"/comments/1"); body != "beforeand the author's addition\n" {
		t.Errorf("after the edit the comment reads %q", body)
	}

	// Somebody else's write lands while the editor is open: the stand-in
	// editor appends a body write to the card's journal before it returns.
	journal := strings.TrimSpace(runCLI(t, root, "path", card+"/journal").out)
	commentID := firstCommentIDOf(t, journal)
	elsewhere, err := json.Marshal(bench.Event{
		TS:    "2026-09-28T12:00:00Z",
		Event: contract.EventCommentUpdated,
		Actor: bench.NamedActor("bob"),
		Note:  commentID,
		Field: bench.BodyField,
		Text:  "what bob wrote meanwhile",
	})
	if err != nil {
		t.Fatalf("compose bob's line: %v", err)
	}
	t.Setenv(testenv.EditorSideFileVar, journal)
	t.Setenv(testenv.EditorSideLineVar, string(elsewhere))
	got, _ = edit("the raced edit", "and a second addition")
	if got.code != refused || !strings.HasPrefix(strings.TrimSpace(got.errw), contract.CommentBodyDiverged) {
		t.Errorf("the edit raced by bob's write exited %d with %q, wanted %s", got.code, got.errw, contract.CommentBodyDiverged)
	}
	// Human show ends what it prints in a newline, whatever the body ends in.
	if body := shownCommentBody(t, root, card+"/comments/1"); strings.TrimSuffix(body, "\n") != "what bob wrote meanwhile" {
		t.Errorf("after the refused edit the comment reads %q, wanted bob's write", body)
	}
}

// shownCommentBody answers the body dinah show prints for a comment.
func shownCommentBody(t *testing.T, root, ref string) string {
	t.Helper()
	got := runCLI(t, root, "show", ref)
	if got.code != 0 {
		t.Fatalf("show %s: %d %s", ref, got.code, got.errw)
	}
	_, body := bench.ParseAnchor(got.out)
	return body
}

// firstCommentIDOf answers the identifier of the first comment a journal records.
func firstCommentIDOf(t *testing.T, journal string) string {
	t.Helper()
	events, _, err := bench.ReadJournal(journal)
	if err != nil {
		t.Fatalf("read %s: %v", journal, err)
	}
	for _, ev := range events {
		if ev.Event == contract.EventCommented {
			return ev.Comment
		}
	}
	t.Fatalf("%s records no comment", journal)
	return ""
}
