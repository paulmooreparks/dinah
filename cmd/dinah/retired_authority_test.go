package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// retiredAuthorityLedger is the fixture holding the sentences dinah-495
// retired. The file carries the reasoning and the limit as well as the
// entries, so a reader who meets a failure meets the argument with it.
var retiredAuthorityLedger = filepath.Join("testdata", "retired-authority-sentences.txt")

// retiredAuthorityDocuments is the corpus the ledger is searched over: the
// nine embedded guides, the quick start, the format design document, the
// published profile, the README, and the eight message catalogues.
//
// The catalogues are in the corpus because leaving them out is what let this
// card ship the defect a second time. dinah-495 retired the owner flag's help
// text, wrote a replacement in the same shape the card was retiring
// everywhere else, and neither guard it shipped could see the catalogues at
// all, so the one line the card called its narrower instance sat in the blind
// spot of both. A catalogue is read as bytes like every other file here,
// which works because a JSON string escapes a quotation mark and a backslash
// and nothing an entry below carries.
//
// The set is written out rather than derived from guide.Topics() or from
// msg.Tags(), because the count assertion below is the point: a corpus that
// silently grew or shrank would move the number this check holds itself to,
// and the whole reason the ledger exists is that an earlier count of these
// sentences was low three times running.
var retiredAuthorityDocuments = []string{
	filepath.Join("internal", "guide", "guides", "first-session.md"),
	filepath.Join("internal", "guide", "guides", "getting-started.md"),
	filepath.Join("internal", "guide", "guides", "mcp.md"),
	filepath.Join("internal", "guide", "guides", "on-behalf.md"),
	filepath.Join("internal", "guide", "guides", "principles.md"),
	filepath.Join("internal", "guide", "guides", "query.md"),
	filepath.Join("internal", "guide", "guides", "references.md"),
	filepath.Join("internal", "guide", "guides", "verbs.md"),
	filepath.Join("internal", "guide", "guides", "workbench-layout.md"),
	filepath.Join("docs", "quick-start.md"),
	filepath.Join("docs", "design", "format.md"),
	filepath.Join("docs", "spec", "core-profile.md"),
	"README.md",
	filepath.Join("internal", "msg", "locales", "af.json"),
	filepath.Join("internal", "msg", "locales", "cs.json"),
	filepath.Join("internal", "msg", "locales", "de.json"),
	filepath.Join("internal", "msg", "locales", "en.json"),
	filepath.Join("internal", "msg", "locales", "es.json"),
	filepath.Join("internal", "msg", "locales", "fil.json"),
	filepath.Join("internal", "msg", "locales", "hi.json"),
	filepath.Join("internal", "msg", "locales", "id.json"),
}

// retiredAuthorityEntries is how many sentences the ledger must hold, and
// retiredAuthorityFiles how many files the search must read. Both are
// equalities rather than floors. A floor passes a fixture somebody half
// deleted and a corpus somebody trimmed, and each failure would read exactly
// like a clean sweep.
const (
	retiredAuthorityEntries = 25
	retiredAuthorityFiles   = 21
)

// readRetiredAuthorityLedger reads the ledger's entries, dropping comments and
// blank lines. Each entry comes back flattened to single spaces, which is the
// shape the documents are searched in.
func readRetiredAuthorityLedger(t *testing.T) []string {
	t.Helper()
	handle, err := os.Open(retiredAuthorityLedger)
	if err != nil {
		t.Fatalf("open %s: %v", retiredAuthorityLedger, err)
	}
	defer handle.Close()
	var entries []string
	scanner := bufio.NewScanner(handle)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, flattenWords(line))
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read %s: %v", retiredAuthorityLedger, err)
	}
	return entries
}

// TestNoRetiredAuthoritySentenceStandsInTheDocumentation asserts that none of
// the sentences dinah-495 retired has come back to the documentation.
//
// Each retired sentence said the operator alone can do something, or that an
// act is his. Dinah does refuse those acts to anybody else, and the refusal is
// worth having, but it compares a name against a recorded name: the `--actor`
// flag outranks the environment and the config, and a caller holding the
// workbench's files edits an item's state with no verb at all. So the retired
// form overstates what the tool guarantees, and a reader who builds on it
// builds on a guard that one flag walks past.
//
// The search flattens each file before looking, so a sentence that comes back
// across a line break, or rewrapped to a different width, is still found. The
// two count assertions are what stop this reading nothing: a ledger that lost
// its entries and a corpus that lost its files would both otherwise report a
// clean sweep.
//
// One thing the corpus does not buy, now that the catalogues are in it. The
// ledger holds English bytes, so a retired sentence coming back as a
// translation is not found. That is not hypothetical: the German rendering of
// the entry this card retired twice is what made the second defect plain to a
// reader, while the English it was translated from read as though it had been
// repaired. TestATranslationTracksItsEnglishSource does force a translation to
// be rewritten whenever its English changes, which is what keeps the two from
// drifting apart, and nothing here reads the rewrite.
func TestNoRetiredAuthoritySentenceStandsInTheDocumentation(t *testing.T) {
	entries := readRetiredAuthorityLedger(t)
	if len(entries) != retiredAuthorityEntries {
		t.Errorf("%s holds %d entries and dinah-495 retired %d sentences, so the ledger and the card disagree",
			retiredAuthorityLedger, len(entries), retiredAuthorityEntries)
	}
	read := 0
	for _, relative := range retiredAuthorityDocuments {
		raw, err := os.ReadFile(filepath.Join(repositoryRoot, relative))
		if err != nil {
			t.Errorf("%s: %v", relative, err)
			continue
		}
		flattened := flattenWords(string(raw))
		if flattened == "" {
			t.Errorf("%s is empty, so this check read nothing of it", relative)
			continue
		}
		read++
		for at, entry := range entries {
			if !strings.Contains(flattened, entry) {
				continue
			}
			t.Errorf("%s carries retired authority sentence %d, %q; write what Dinah refuses rather than what only the operator can do",
				relative, at+1, entry)
		}
	}
	if read != retiredAuthorityFiles {
		t.Errorf("the sweep read %d files and dinah-495 swept %d, so it covers less than it claims", read, retiredAuthorityFiles)
	}
}
