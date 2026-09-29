package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// TestRedactAtTheTerminal drives what dinah-637/criteria/37 and /38 ask of the
// terminal. Without --yes dinah redact names the member, its lines and its
// journal and changes nothing; with it, the human answer lists a comment's
// two attachments under Attachments left by reference and filename, and
// --json carries them as attachments_left; a comment carrying none prints no
// such section and answers an empty list. Afterwards show prints the
// catalogue line naming who redacted the comment in place of its body, and a
// search for the redacted token matches nothing.
//
// Arming: dropping the attachments section from renderRedaction reddens the
// human answer's assertion, and dropping the redaction line from runShow
// reddens show's.
func TestRedactAtTheTerminal(t *testing.T) {
	bench.EnableCardUnitForTest(t)
	root := newBench(t)
	const token = "tok38-terminal-2f7a"
	for _, argv := range [][]string{
		{"add", "A card whose comments are redacted"},
		{"comment", "fx-1", "A comment naming " + token + "."},
		{"comment", "fx-1", "A comment carrying nothing attached."},
	} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Fatalf("%v: %d %s", argv, got.code, got.errw)
		}
	}
	for _, name := range []string{"first.txt", "second.txt"} {
		source := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(source, []byte("bytes of "+name), 0o644); err != nil {
			t.Fatalf("write %s: %v", source, err)
		}
		if got := runCLI(t, root, "attach", "fx-1/comments/1", source); got.code != 0 {
			t.Fatalf("attach %s: %d %s", name, got.code, got.errw)
		}
	}

	journal := filepath.Join(soleBenchDir(t, root), bench.CardsDir)
	before := redactedTreeBytes(t, journal)
	dry := runCLI(t, root, "redact", "fx-1/comments/1")
	if dry.code != 0 || !strings.Contains(dry.out, "Would redact fx-1/comments/1") || !strings.Contains(dry.out, "Lines in all: 1") {
		t.Errorf("redact without --yes answered %d:\n%s%s", dry.code, dry.out, dry.errw)
	}
	if redactedTreeBytes(t, journal) != before {
		t.Error("redact without --yes changed the store")
	}

	// A file an earlier redaction left when it stopped before its rename is
	// removed by the next one, which says so.
	cardDir := filepath.Join(soleBenchDir(t, root), bench.CardsDir)
	cards, err := bench.ListIDs(cardDir)
	if err != nil || len(cards) != 1 {
		t.Fatalf("wanted one card under %s: %v %v", cardDir, cards, err)
	}
	leftover := filepath.Join(cardDir, cards[0], bench.JournalName+bench.RedactLeftoverSuffix)
	if err := os.WriteFile(leftover, []byte("a stale composition\n"), 0o644); err != nil {
		t.Fatalf("plant %s: %v", leftover, err)
	}
	done := runCLI(t, root, "redact", "fx-1/comments/1", "--yes")
	if done.code != 0 {
		t.Fatalf("redact --yes: %d %s", done.code, done.errw)
	}
	// The removed file is matched by its name rather than its whole path,
	// because a temporary directory reaches the binary through whatever
	// spelling of it the platform resolves, which on macOS is /private/var.
	for _, want := range []string{
		"Removed ",
		filepath.Base(leftover) + ", which a stopped dinah redact left behind.",
		"Redacted fx-1/comments/1",
		"Attachments left:",
		"fx-1/comments/1/attachments/1 (first.txt)",
		"fx-1/comments/1/attachments/2 (second.txt)",
		"These attachments are still readable; remove them with dinah delete if they must go.",
	} {
		if !strings.Contains(done.out, want) {
			t.Errorf("the human answer lacks %q:\n%s", want, done.out)
		}
	}
	bare := runCLI(t, root, "--json", "redact", "fx-1/comments/2", "--yes")
	var answer struct {
		AttachmentsLeft []map[string]string `json:"attachments_left"`
		OwnLines        int                 `json:"own_lines"`
		LegacyLines     int                 `json:"legacy_lines"`
		Lines           int                 `json:"lines"`
	}
	if err := json.Unmarshal([]byte(bare.out), &answer); err != nil {
		t.Fatalf("decode %s: %v", bare.out, err)
	}
	if answer.AttachmentsLeft == nil || len(answer.AttachmentsLeft) != 0 || !strings.Contains(bare.out, `"attachments_left": []`) {
		t.Errorf("a comment carrying no attachment answered %s, wanted an empty attachments_left", bare.out)
	}
	if answer.OwnLines+answer.LegacyLines != answer.Lines || answer.Lines != 1 {
		t.Errorf("the answer counts %d own and %d legacy lines and %d in all", answer.OwnLines, answer.LegacyLines, answer.Lines)
	}

	shown := runCLI(t, root, "show", "fx-1/comments/1")
	if shown.code != 0 || !strings.Contains(shown.out, "Redacted by alka at ") || strings.Contains(shown.out, token) {
		t.Errorf("show of the redacted comment printed:\n%s%s", shown.out, shown.errw)
	}
	found := runCLI(t, root, "search", token)
	if strings.Contains(found.out, "fx-1") {
		t.Errorf("a search for the redacted token matched:\n%s", found.out)
	}

	// The card's own listing of its comments, and an item's own show, print
	// the redaction in place of the text as well.
	listed := runCLI(t, root, "show", "fx-1", "--all")
	if listed.code != 0 || !strings.Contains(listed.out, "Redacted by alka at ") {
		t.Errorf("show of the card does not print the redaction among its comments:\n%s%s", listed.out, listed.errw)
	}
	for _, argv := range [][]string{
		{"file", "fx-1", "decision", "A decision whose text goes."},
		{"redact", "fx-1/decisions/1", "--yes"},
	} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Fatalf("%v: %d %s", argv, got.code, got.errw)
		}
	}
	item := runCLI(t, root, "show", "fx-1/decisions/1")
	if item.code != 0 || !strings.Contains(item.out, "Redacted by alka at ") || strings.Contains(item.out, "whose text goes") {
		t.Errorf("show of the redacted item printed:\n%s%s", item.out, item.errw)
	}
}

// TestRedactReachesAMemberDeletedBeforeTheMigration answers
// dinah-637/questions/24 on the migrated fixture. The fixture deleted an item
// before its migration, and that deletion's line names the item in note alone
// and carries its text as title, which is the one line anywhere in the store
// still holding the text. Redacting the item by its identifier rewrites that
// line, after which a byte search of every file of the store finds the text
// nowhere, the answer counts the line, and dinah check reports no defect: the
// redacted line names a member a deletion before the migration removed, which
// the replay knows. A comment deleted before the migration kept its text only
// in the file its deletion removed, so no line carries it, and redacting it is
// refused dinah.nothing-to-redact and changes no file.
//
// Arming: matching a deleted line to an item by its item member alone, as
// ownTexts did, leaves the title in place, so the redaction is refused
// nothing-to-redact and the byte search finds the text; recording no deletion
// before the migration marker in the replay leaves the redacted line naming a
// member the replay never established, which check reports as
// check.member-unknown.
func TestRedactReachesAMemberDeletedBeforeTheMigration(t *testing.T) {
	store, root := migratedFixture(t)
	const deletedItem, deletedItemText = "9a485947b98f", "a decision deleted afterwards"
	const deletedComment = "b4856c895544"
	if hits := storeTextHits(t, store, deletedItemText); hits != 1 {
		t.Fatalf("the migrated store holds the deleted item's text %d times, wanted the one deletion line", hits)
	}
	// The fixture carries findings of its own, a comment edited by hand among
	// them, so the check is compared with what it reported before.
	findingsBefore := checkFindingKeys(t, root, store)
	if !strings.Contains(findingsBefore, "check.comment-body-diverged") {
		t.Fatalf("check on the migrated fixture reported %s, and the fixture's hand-edited comment is missing from it, so the comparison below would prove nothing", findingsBefore)
	}

	done := runCLI(t, root, "--workbench", store, "--json", "redact", "fx-1/checklist/"+deletedItem, "--yes")
	if done.code != 0 {
		t.Fatalf("redact the deleted item: %d\n%s%s", done.code, done.out, done.errw)
	}
	var answer struct {
		OwnLines int `json:"own_lines"`
		Lines    int `json:"lines"`
	}
	if err := json.Unmarshal([]byte(done.out), &answer); err != nil {
		t.Fatalf("decode %s: %v", done.out, err)
	}
	if answer.OwnLines != 1 || answer.Lines != 1 {
		t.Errorf("the redaction counts %d own lines and %d in all, wanted the one deletion line", answer.OwnLines, answer.Lines)
	}
	if hits := storeTextHits(t, store, deletedItemText); hits != 0 {
		t.Errorf("after the redaction the store still holds the deleted item's text %d times", hits)
	}
	if after := checkFindingKeys(t, root, store); after != findingsBefore {
		t.Errorf("check reported %s before the redaction and %s after it", findingsBefore, after)
	}

	before := redactedTreeBytes(t, store)
	refused := runCLI(t, root, "--workbench", store, "redact", "fx-1/comments/"+deletedComment, "--yes")
	if refused.code != 2 || !strings.Contains(refused.errw, "dinah.nothing-to-redact") {
		t.Errorf("redacting a comment deleted before the migration answered %d:\n%s%s", refused.code, refused.out, refused.errw)
	}
	if redactedTreeBytes(t, store) != before {
		t.Error("the refused redaction changed the store")
	}
}

// checkFindingKeys answers the finding keys dinah check reports on a store,
// sorted and joined, with each finding's detail beside its key.
func checkFindingKeys(t *testing.T, root, store string) string {
	t.Helper()
	got := runCLI(t, root, "--workbench", store, "--json", "check")
	var answer struct {
		Findings []struct {
			Key    string `json:"Key"`
			Detail string `json:"Detail"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(got.out), &answer); err != nil {
		t.Fatalf("decode the check answer %s: %v", got.out, err)
	}
	var keys []string
	for _, finding := range answer.Findings {
		keys = append(keys, finding.Key+" "+finding.Detail)
	}
	sort.Strings(keys)
	return "[" + strings.Join(keys, "; ") + "]"
}

// storeTextHits counts the occurrences of a text in every file under a store,
// byte for byte, which is the search a redaction has to defeat.
func storeTextHits(t *testing.T, store, text string) int {
	t.Helper()
	hits := 0
	err := filepath.WalkDir(store, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hits += strings.Count(string(data), text)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", store, err)
	}
	return hits
}

// redactedTreeBytes reads every file under a directory into one string, in walk
// order, so two readings compare equal when nothing changed.
func redactedTreeBytes(t *testing.T, dir string) string {
	t.Helper()
	var all strings.Builder
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		all.WriteString(path + "\n" + string(data) + "\n")
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return all.String()
}
