package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// TestAliasConfigurationLifecycle covers dynamic alias settings, ordered
// listing, exact retrieval, removal, and preservation of unrelated rows.
func TestAliasConfigurationLifecycle(t *testing.T) {
	home, dir := settingsHome(t)
	if got := runCLI(t, dir, "config", "set", "alias.lc", "list cards"); got.code != 0 {
		t.Fatalf("set alias: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, dir, "config", "get", "alias.lc"); got.code != 0 || got.out != "list cards\n" {
		t.Fatalf("get alias: code %d, out %q, err %s", got.code, got.out, got.errw)
	}
	path := filepath.Join(bench.UserBase(home), bench.ConfigName)
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	withUnknown := strings.Replace(string(stored), "alias.lc: list cards", "alias.lc: list cards\ncolour: green", 1)
	if err := os.WriteFile(path, []byte(withUnknown), 0o644); err != nil {
		t.Fatalf("add preserved key: %v", err)
	}
	var listed []verb.SettingView
	machine := runCLI(t, dir, "--json", "config")
	if err := json.Unmarshal([]byte(machine.out), &listed); err != nil {
		t.Fatalf("decode listing: %v\n%s", err, machine.out)
	}
	if len(listed) != len(bench.ConfigKeys)+2 {
		t.Fatalf("listing has %d rows, wanted %d", len(listed), len(bench.ConfigKeys)+2)
	}
	alias := listed[len(bench.ConfigKeys)]
	if alias.Key != "alias.lc" || alias.Value != "list cards" || alias.Source != bench.SourceConfig {
		t.Errorf("alias row = %+v", alias)
	}
	if got := runCLI(t, dir, "config", "set", "alias.lc"); got.code != 0 {
		t.Fatalf("remove alias: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, dir, "config", "set", "alias.empty", ""); got.code != contract.ExitCode(contract.OutcomeRefused) || !strings.Contains(got.errw, bench.AliasEmptyTemplate) {
		t.Fatalf("explicit empty template: code %d, err %s", got.code, got.errw)
	}
	if got := runCLI(t, dir, "config", "set", "alias.Bad", "config"); got.code != contract.ExitCode(contract.OutcomeRefused) || !strings.Contains(got.errw, bench.AliasInvalidName) {
		t.Fatalf("invalid alias name: code %d, err %s", got.code, got.errw)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config after removal: %v", err)
	}
	if strings.Contains(string(after), "alias.lc:") || !strings.Contains(string(after), "colour: green") {
		t.Errorf("removal changed the wrong rows:\n%s", after)
	}
}

// TestAliasExpansionUsesTheOrdinaryArgumentGrammar covers plain and embedded
// substitution, extra arguments, global flags, one-pass expansion, and the
// missing-argument refusal.
func TestAliasExpansionUsesTheOrdinaryArgumentGrammar(t *testing.T) {
	_, dir := settingsHome(t)
	for _, row := range [][]string{
		{"alias.cg", "config get"},
		{"alias.ag", "config get alias.$1"},
		{"alias.jg", "config --json get $1"},
		{"alias.first", "second"},
		{"alias.second", "config"},
	} {
		got := runCLI(t, dir, "config", "set", row[0], row[1])
		if got.code != 0 {
			t.Fatalf("set %s: %d %s", row[0], got.code, got.errw)
		}
	}
	plain := runCLI(t, dir, "cg", "alias.ag")
	if plain.code != 0 || plain.out != "config get alias.$1\n" {
		t.Fatalf("plain expansion: code %d, out %q, err %s", plain.code, plain.out, plain.errw)
	}
	embedded := runCLI(t, dir, "ag", "ag")
	if embedded.code != 0 || embedded.out != "config get alias.$1\n" {
		t.Fatalf("embedded expansion: code %d, out %q, err %s", embedded.code, embedded.out, embedded.errw)
	}
	machine := runCLI(t, dir, "jg", "alias.ag")
	if machine.code != 0 || machine.out != "config get alias.$1\n" {
		t.Fatalf("template flag expansion: code %d, out %q, err %s", machine.code, machine.out, machine.errw)
	}
	withOriginalFlag := runCLI(t, dir, "--json", "ag", "ag")
	if withOriginalFlag.code != 0 || withOriginalFlag.out != "config get alias.$1\n" {
		t.Fatalf("original flag expansion: code %d, out %q, err %s", withOriginalFlag.code, withOriginalFlag.out, withOriginalFlag.errw)
	}
	missing := runCLI(t, dir, "ag")
	if missing.code != contract.ExitCode(contract.OutcomeRefused) || !strings.Contains(missing.errw, contract.AliasMissing) {
		t.Fatalf("missing argument: code %d, err %s", missing.code, missing.errw)
	}
	once := runCLI(t, dir, "first")
	if once.code != contract.ExitCode(contract.OutcomeRefused) || !strings.Contains(once.errw, contract.UnknownVerb+" ") || !strings.Contains(once.errw, "second") {
		t.Fatalf("recursive alias should be an unknown command: code %d, err %s", once.code, once.errw)
	}
	extra := runCLI(t, dir, "ag", "ag", "extra")
	if extra.code != contract.ExitCode(contract.OutcomeRefused) || !strings.Contains(extra.errw, "extra") {
		t.Fatalf("extra argument should reach config's ordinary arity check: code %d, err %s", extra.code, extra.errw)
	}
	boundary := runCLI(t, dir, "--", "ag", "ag", "--json")
	if boundary.code != contract.ExitCode(contract.OutcomeRefused) || !strings.Contains(boundary.errw, "--json") {
		t.Fatalf("the option boundary should keep --json positional: code %d, err %s", boundary.code, boundary.errw)
	}
}

// TestAliasValidationPairsAcceptingAndRefusingCases covers every stable defect
// class beside a nearby accepted value, including the public boundary names.
func TestAliasValidationPairsAcceptingAndRefusingCases(t *testing.T) {
	accepted := []struct {
		name     string
		template string
	}{
		{name: "a", template: "config"},
		{name: "a-b9", template: "config\tget alias.a"},
		{name: "literal", template: "config get $x"},
		{name: "nine", template: "show $1/$2/$3/$4/$5/$6/$7/$8/$9"},
	}
	for _, test := range accepted {
		if _, _, defect := bench.ValidateAlias(test.name, test.template); defect != "" {
			t.Errorf("ValidateAlias(%q, %q) refused with %s", test.name, test.template, defect)
		}
	}
	refused := []struct {
		name, template, defect string
	}{
		{name: "", template: "config", defect: bench.AliasInvalidName},
		{name: "Bad", template: "config", defect: bench.AliasInvalidName},
		{name: "-bad", template: "config", defect: bench.AliasInvalidName},
		{name: "bad-", template: "config", defect: bench.AliasInvalidName},
		{name: "bad--name", template: "config", defect: bench.AliasInvalidName},
		{name: "bad.name", template: "config", defect: bench.AliasInvalidName},
		{name: "é", template: "config", defect: bench.AliasInvalidName},
		{name: "empty", template: " \t ", defect: bench.AliasEmptyTemplate},
		{name: "shell", template: "!echo nope", defect: bench.AliasShellTemplate},
		{name: "space", template: "config\u00a0get", defect: bench.AliasUnicodeWhitespace},
		{name: "zero", template: "show $0", defect: bench.AliasPlaceholderZero},
		{name: "gap", template: "show $1/$3", defect: bench.AliasPlaceholderGap},
		{name: "many", template: "show $10", defect: bench.AliasPlaceholderOutside},
	}
	for _, test := range refused {
		if _, _, defect := bench.ValidateAlias(test.name, test.template); defect != test.defect {
			t.Errorf("ValidateAlias(%q, %q) defect = %q, wanted %q", test.name, test.template, defect, test.defect)
		}
	}
}

// TestAliasShadowingKeepsTheCommandAndStoredRow covers write-time refusal and
// load-time shadow classification without losing recovery through config.
func TestAliasShadowingKeepsTheCommandAndStoredRow(t *testing.T) {
	home, dir := settingsHome(t)
	before := configBytes(t, home)
	refused := runCLI(t, dir, "config", "set", "alias.help", "config")
	if refused.code != contract.ExitCode(contract.OutcomeRefused) || !strings.Contains(refused.errw, contract.AliasShadow) {
		t.Fatalf("shadow write: code %d, err %s", refused.code, refused.errw)
	}
	if after := configBytes(t, home); string(after) != string(before) {
		t.Fatalf("refused shadow write changed config:\nbefore: %s\nafter: %s", before, after)
	}
	path := filepath.Join(bench.UserBase(home), bench.ConfigName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(path, []byte("---\nalias.help: config\nalias.show: show $0\n---\n"), 0o644); err != nil {
		t.Fatalf("write stale alias: %v", err)
	}
	if got := runCLI(t, dir, "help", "config"); got.code != 0 || !strings.HasPrefix(got.out, "config ") {
		t.Fatalf("real command did not win: code %d, out %q, err %s", got.code, got.out, got.errw)
	}
	rows := settingRows(t, runCLI(t, dir, "--json", "config"))
	if rows["alias.help"].Source != bench.SourceShadowed {
		t.Errorf("stale alias source = %q, wanted %q", rows["alias.help"].Source, bench.SourceShadowed)
	}
	if rows["alias.show"].Source != bench.SourceInvalid {
		t.Errorf("malformed shadow source = %q, wanted %q", rows["alias.show"].Source, bench.SourceInvalid)
	}
	if got := runCLI(t, dir, "config", "set", "alias.help"); got.code != 0 {
		t.Fatalf("remove stale alias: %d %s", got.code, got.errw)
	}
}

// TestMalformedStoredAliasesStayVisibleAndRecoverable covers quarantine,
// exact invocation refusal, and repair without rewriting neighboring rows.
func TestMalformedStoredAliasesStayVisibleAndRecoverable(t *testing.T) {
	home, dir := settingsHome(t)
	path := filepath.Join(bench.UserBase(home), bench.ConfigName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	stored := "---\nalias.Bad: config\nalias.broken: show $0\ncolour: green\n---\nbody stays\n"
	if err := os.WriteFile(path, []byte(stored), 0o644); err != nil {
		t.Fatalf("write malformed aliases: %v", err)
	}
	rows := settingRows(t, runCLI(t, dir, "--json", "config"))
	for _, key := range []string{"alias.Bad", "alias.broken"} {
		if rows[key].Source != bench.SourceInvalid {
			t.Errorf("%s source = %q, wanted invalid", key, rows[key].Source)
		}
	}
	human := runCLI(t, dir, "config")
	if human.code != 0 || !strings.Contains(human.out, "alias.Bad") || !strings.Contains(human.out, "alias.broken") || !strings.Contains(human.out, "invalid") {
		t.Fatalf("human listing dropped malformed aliases: code %d, out %s, err %s", human.code, human.out, human.errw)
	}
	if got := runCLI(t, dir, "config", "get", "alias.Bad"); got.code != 0 || got.out != "config\n" {
		t.Fatalf("get malformed alias: code %d, out %q, err %s", got.code, got.out, got.errw)
	}
	invoked := runCLI(t, dir, "broken")
	if invoked.code != contract.ExitCode(contract.OutcomeRefused) || !strings.Contains(invoked.errw, bench.AliasPlaceholderZero) {
		t.Fatalf("invoke malformed alias: code %d, err %s", invoked.code, invoked.errw)
	}
	if got := runCLI(t, dir, "config", "set", "alias.Bad"); got.code != 0 {
		t.Fatalf("remove invalid-name alias: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, dir, "config", "set", "alias.broken", "config"); got.code != 0 {
		t.Fatalf("repair invalid-template alias: %d %s", got.code, got.errw)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read repaired config: %v", err)
	}
	text := string(after)
	if strings.Contains(text, "alias.Bad:") || !strings.Contains(text, "alias.broken: config") || !strings.Contains(text, "colour: green") || !strings.Contains(text, "body stays") {
		t.Errorf("recovery changed unrelated content:\n%s", text)
	}
}

// TestAliasedMutationKeepsCanonicalMachineAndJournalShapes proves that alias
// spelling does not enter a successful machine response or a stored event.
func TestAliasedMutationKeepsCanonicalMachineAndJournalShapes(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "config", "set", "alias.zzq", "add $1"); got.code != 0 {
		t.Fatalf("set mutation alias: %d %s", got.code, got.errw)
	}
	got := runCLI(t, root, "--json", "zzq", "made")
	if got.code != 0 {
		t.Fatalf("aliased add: %d %s", got.code, got.errw)
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(got.out), &response); err != nil {
		t.Fatalf("decode aliased response: %v\n%s", err, got.out)
	}
	if _, present := response["alias"]; present {
		t.Errorf("machine response carries alias: %s", got.out)
	}
	if _, present := response["invocation"]; present {
		t.Errorf("machine response carries invocation: %s", got.out)
	}
	if strings.Contains(got.out, "zzq") {
		t.Errorf("machine response leaks alias spelling: %s", got.out)
	}
	workbench := soleBenchDir(t, root)
	var journals strings.Builder
	err := filepath.WalkDir(workbench, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Name() != bench.JournalName {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		journals.Write(data)
		return nil
	})
	if err != nil {
		t.Fatalf("read journals: %v", err)
	}
	stored := journals.String()
	if !strings.Contains(stored, `"event":"created"`) {
		t.Fatalf("no created event was read from the journal:\n%s", stored)
	}
	for _, leaked := range []string{"zzq", `"alias"`, `"invocation"`} {
		if strings.Contains(stored, leaked) {
			t.Errorf("journal leaks %q:\n%s", leaked, stored)
		}
	}
}
