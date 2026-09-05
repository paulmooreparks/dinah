// The workflow files whose safety sits in their text, read as text.
//
// Neither half of the manual checks these mirror can be automated: nobody can
// plant a file in a dependency tree from a unit test, and nobody can push a
// red branch from one. But each of those checks also has a half that is a
// plain file read, and a revert of either edit is silent otherwise. The gofmt
// scoping would come back as a red job for a reason unrelated to this
// repository's Go code, and the release gate would go back to publishing on a
// commit whose extension build failed.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

const repoRoot = join(__dirname, "..", "..", "..", "..", "..");
const ci = readFileSync(join(repoRoot, ".github", "workflows", "ci.yml"), "utf8");
const release = readFileSync(
	join(repoRoot, ".github", "workflows", "release.yml"),
	"utf8",
);
const promote = readFileSync(
	join(repoRoot, ".github", "workflows", "promote.yml"),
	"utf8",
);
const vscodeRelease = readFileSync(
	join(repoRoot, ".github", "workflows", "vscode-release.yml"),
	"utf8",
);
const gofmtAction = readFileSync(
	join(repoRoot, ".github", "actions", "gofmt-check", "action.yml"),
	"utf8",
);

test("the gofmt check is scoped to the Go trees", () => {
	// `gofmt -l .` walks into editors/vscode/node_modules/, so an npm
	// dependency that vendors an unformatted .go file as testdata turns the
	// job red for a reason that has nothing to do with this repository.
	assert.ok(
		gofmtAction.includes("gofmt -l cmd internal"),
		"the gofmt action no longer scopes gofmt to cmd and internal",
	);
	assert.ok(
		!/gofmt -l \.\s*\)/.test(gofmtAction) && !gofmtAction.includes("gofmt -l .)"),
		"the gofmt action still runs gofmt over the whole tree",
	);
	assert.ok(
		ci.includes("./.github/actions/gofmt-check"),
		"ci.yml no longer runs the gofmt check at all",
	);
});

test("a promoted tree is checked in its own right", () => {
	// The failure this guards is a promotion that publishes on somebody
	// else's green result. The assembled tree is a combination nobody has
	// built, so the checks run against it and against nothing else.
	assert.ok(
		promote.includes("./.github/actions/go-checks"),
		"promote.yml no longer runs the Go checks",
	);
	assert.ok(
		/path: tree/.test(promote),
		"promote.yml no longer points the checks at the tree it assembled",
	);
	const publish = promote.slice(promote.indexOf("\n  publish:"));
	assert.ok(
		/needs: \[assemble, verify\]/.test(publish),
		"promote.yml's publish job no longer waits for the checks on the assembled tree",
	);
});

test("a stable promotion reads its version line from the tree it promotes", () => {
	// Reading VERSION from the workflow's own checkout reads the trunk, which
	// moves on after a beta is cut. Promote a 0.1 beta once the trunk says 0.2
	// and the run mints v0.2.0 over the 0.1 tree and publishes it as stable.
	const assemble = promote.slice(
		promote.indexOf("Assemble the cut"),
		promote.indexOf("\n      - name: Bundle the assembled tree"),
	);
	const stable = assemble.slice(assemble.indexOf("\n          else"));
	assert.ok(
		stable.includes('git show "${BETA_TAG}:VERSION"'),
		"the stable branch no longer reads VERSION out of the promoted tag's own tree",
	);
	assert.ok(
		!/BASE=\$\(tr -d '\[:space:\]' < VERSION\)/.test(stable),
		"the stable branch reads VERSION from the workflow's checkout, which is the trunk rather than the tree being promoted",
	);
	assert.ok(
		/dinah-release patch --channel beta --base "\$BASE" --tag "\$BETA_TAG"/.test(
			stable,
		),
		"the stable branch no longer checks that beta_tag is a beta of the line it is publishing",
	);
});

test("a beta cut declares its dependencies one card at a time", () => {
	// The refusal this guards used to be satisfied by the single word "none",
	// which switched the dependency check off for a cut of any size.
	const links = promote.slice(promote.indexOf("      links:"));
	assert.ok(
		links.includes("dependent>none"),
		"promote.yml's links input no longer asks for a declaration per card",
	);
	assert.ok(
		!/or the single word none/.test(promote),
		"promote.yml still offers the whole-cut placeholder that any value satisfies",
	);
});

test("the jobs the release gate waits for are still named what it waits for", () => {
	// release.yml's wait-for-ci selects check runs by name. Moving a job's
	// body into a composite action leaves the name where it was, and that is
	// the point: a rename here would strand the gate on checks that never
	// arrive, and the gate would fail on a timeout rather than on anything
	// true about the commit.
	for (const job of ["test", "gofmt", "extension"]) {
		assert.ok(
			new RegExp(`^ {2}${job}:$`, "m").test(ci),
			`ci.yml no longer has a job named ${job}, which release.yml's gate waits for`,
		);
	}
});

test("the dev tag is computed by the code that has tests", () => {
	// The counter has to be read across both tag shapes, and the shell that
	// used to match tag strings could not be tested. internal/release can be,
	// so release.yml calls it.
	assert.ok(
		release.includes("dinah-release next-tag --channel dev"),
		"release.yml no longer computes its tag with the tested helper",
	);
	assert.ok(
		!release.includes('PREFIX="v${BASE}.0-dev."'),
		"release.yml still matches the old tag shape by hand, which reads only one of the two shapes the tag list carries",
	);
});

test("ci.yml carries an extension job on both platforms this code is sensitive to", () => {
	assert.ok(/^ {2}extension:$/m.test(ci), "ci.yml has no extension job");
	assert.ok(ci.includes("ubuntu-latest"));
	assert.ok(ci.includes("windows-latest"));
});

test("the release gate counts the extension check runs", () => {
	// The jq select decides which check runs the gate waits for. Without the
	// extension leg named here, a release publishes on a commit whose
	// extension build failed.
	assert.ok(
		release.includes('(.name | startswith("extension"))'),
		"release.yml's wait-for-ci does not select the extension check runs",
	);
	assert.ok(
		release.includes("-ge 6"),
		"release.yml's wait-for-ci threshold is not 6",
	);
});

test("no prose statement of the check-run count still says four", () => {
	// The number was written in four places at 8f3aead: the job-level comment
	// and three echoes inside the steps. A criterion that fixed only some of
	// them would leave a stale count in the release log, so this asserts on
	// the absence rather than on a count that would have to be right.
	const gate = release.slice(
		release.indexOf("\n  wait-for-ci:") - 700,
		release.indexOf("\n  build:"),
	);
	for (const stale of ["four check runs", "All four CI checks", "of 4 checks"]) {
		assert.ok(
			!gate.includes(stale),
			`release.yml's wait-for-ci still says "${stale}"`,
		);
	}
	assert.ok(gate.includes("six check runs"));
	assert.ok(gate.includes("of 6 checks"));
});

// The extension's own release workflow, read the same way.
//
// Nothing here can dispatch a workflow run, so these cover the half a file
// read covers, which is where every one of that workflow's decisions is
// written down. Each assertion below guards a decision whose reversal would
// otherwise be silent: a trigger firing on the file rather than on the version
// cuts a duplicate release on a lockfile bump, a release step attaching
// everything in the output directory publishes an internal manifest, and a
// publish step without its secret gate tries to publish on every run.

test("the extension release fires on main and only for the extension manifest", () => {
	assert.ok(
		/^on:\n {2}push:\n {4}branches: \[main\]\n {4}paths:\n {6}- "editors\/vscode\/package\.json"$/m.test(
			vscodeRelease,
		),
		"vscode-release.yml no longer triggers on pushes to main touching only editors/vscode/package.json",
	);
});

test("the release trigger compares the version field rather than the file", () => {
	// The paths filter says the file changed and says nothing about which
	// field. Without this comparison a push that renamed a command or edited
	// the npm scripts cuts a release the marketplace then refuses, because it
	// already carries that version.
	const step = vscodeRelease.slice(
		vscodeRelease.indexOf("- name: Read the version before and after this push"),
		vscodeRelease.indexOf("  wait-for-extension-ci:"),
	);
	assert.ok(
		step.includes("BEFORE_SHA: ${{ github.event.before }}") &&
			step.includes("AFTER_SHA: ${{ github.sha }}"),
		"the version check no longer reads the manifest at both ends of the push",
	);
	assert.ok(
		step.includes("node scripts/check-version-change.mjs /tmp/after.json"),
		"the version check no longer calls the tested comparison",
	);
	const waitJob = vscodeRelease.slice(
		vscodeRelease.indexOf("  wait-for-extension-ci:"),
		vscodeRelease.indexOf("  package-and-release:"),
	);
	assert.ok(
		waitJob.includes("if: needs.check-version.outputs.changed == 'true'"),
		"the first job downstream of the version check is no longer gated on the version having changed",
	);
});

// A --jq filter under --paginate may select and map, and may not reduce.
//
// `gh api --help` says that under --paginate "each page is a separate JSON
// array or object", so the filter runs once per page. A filter that only
// selects and maps is safe under that, because the pages concatenate into one
// stream carrying every match. A filter that reduces to a single answer emits
// one answer per page instead, which is invisible while the collection fits
// in a page. That is how this workflow shipped its first round: 92 releases,
// one page, one correct tag, and four tags the moment a smaller page size was
// asked for, written to $GITHUB_OUTPUT as an undelimited multi-line value.
//
// The reducing spellings below are the ones a reader would reach for first
// rather than the whole of jq's array vocabulary, so the per-step assertions
// carry the weight and this one catches the class on the way past.
const REDUCING_JQ = [
	"sort_by(",
	"group_by(",
	"unique",
	"max_by(",
	"min_by(",
	"| last",
	"| first",
	"| add",
	"| length",
];

const ghCommands = vscodeRelease
	.replace(/\\\n\s*/g, " ")
	.split("\n")
	.filter((line) => line.includes("gh api"));

test("a paginated gh read filters per page with a filter that does not reduce", () => {
	assert.ok(ghCommands.length > 0, "the workflow no longer calls gh api at all");
	for (const command of ghCommands) {
		if (!command.includes("--paginate") || !command.includes("--jq")) {
			continue;
		}
		// The filter is the single-quoted argument after --jq, and it carries
		// no single quote of its own, so the next one ends it. Bounding it
		// here keeps the shell pipeline that follows out of the check, which
		// is where the reduction is allowed to live.
		const opened = command.indexOf("'", command.indexOf("--jq"));
		const closed = command.indexOf("'", opened + 1);
		assert.ok(
			opened > 0 && closed > opened,
			`a --paginate read's --jq argument is not a single-quoted filter: ${command.trim()}`,
		);
		const filter = command.slice(opened + 1, closed);
		for (const spelling of REDUCING_JQ) {
			assert.ok(
				!filter.includes(spelling),
				`a --paginate read reduces inside --jq with "${spelling}", so it answers once per page: ${command.trim()}`,
			);
		}
	}
});

test("a first push to a ref is detected by the field GitHub documents", () => {
	// The all-zero SHA appears in no GitHub documentation of the push
	// payload, and the workbench refuses a branch point resting on an
	// external system's undocumented behaviour. github.event.created answers
	// the same question as a field GitHub commits to.
	assert.ok(
		vscodeRelease.includes("REF_CREATED: ${{ github.event.created }}") &&
			vscodeRelease.includes('[ "$REF_CREATED" != "true" ]'),
		"the workflow no longer decides a first push by the documented created field",
	);
	assert.ok(
		!/0{20,}/.test(vscodeRelease),
		"the workflow compares a SHA against an all-zero sentinel again",
	);
});

test("the newest dev release is chosen by sorting every page on created_at", () => {
	// The releases endpoint documents no ordering guarantee, so taking the
	// list's first element would rest on behaviour nobody promised. Sorting
	// is only half of it: the sort has to run over every page at once, which
	// the class guard above pins for every paginated read and which the two
	// assertions here pin for this one by name.
	const step = vscodeRelease.slice(
		vscodeRelease.indexOf("- name: Find the newest dev release"),
		vscodeRelease.indexOf("- name: Package the archive"),
	);
	assert.ok(
		step.includes('.created_at + " " + .tag_name'),
		"the dev-release lookup no longer emits created_at, so it cannot be sorting on it",
	);
	assert.ok(
		step.includes("| sort | tail -n 1 | cut -d \" \" -f 2"),
		"the dev-release lookup no longer sorts across the pages it fetched, so it answers once per page",
	);
	assert.ok(
		step.includes("returned more than one tag") &&
			step.includes(`printf '%s' "$TAG" | wc -l`),
		"the dev-release lookup no longer refuses a multi-line answer, so a per-page answer would reach $GITHUB_OUTPUT undelimited",
	);
	assert.ok(
		step.includes('test("^v[0-9]+\\\\.[0-9]+\\\\.[0-9]+-dev$")') &&
			step.includes('test("^v[0-9]+\\\\.[0-9]+\\\\.0-dev\\\\.[0-9]+$")'),
		"the dev-release lookup no longer reads both dev tag shapes, so it would find nothing on a line that predates the current shape",
	);
	assert.ok(
		step.includes("::error::no dev release found"),
		"a run finding no dev release no longer fails loudly",
	);
});

test("the packaging step carries the paired release the status bar reports", () => {
	// esbuild.mjs's pairedRelease() reads DINAH_PAIRED_RELEASE at compile
	// time and npm run package runs that compile. Set it anywhere but on this
	// step and the archive ships reporting its provenance as "source".
	const step = vscodeRelease.slice(
		vscodeRelease.indexOf("- name: Package the archive"),
		vscodeRelease.indexOf("- name: Confirm exactly one archive was produced"),
	);
	assert.ok(
		step.includes("DINAH_PAIRED_RELEASE: ${{ steps.dev-release.outputs.tag }}"),
		"the packaging step no longer carries DINAH_PAIRED_RELEASE",
	);
	assert.ok(
		step.includes("npm run package -- --published"),
		"the packaging step no longer packages the published version",
	);
	assert.ok(
		step.includes("npm run verify-package"),
		"the packaging step no longer verifies what it packaged",
	);
});

test("the run fails unless exactly one archive was produced", () => {
	const step = vscodeRelease.slice(
		vscodeRelease.indexOf("- name: Confirm exactly one archive was produced"),
		vscodeRelease.indexOf("- name: Tag this extension version"),
	);
	assert.ok(
		step.includes("[ ! -f vsix/dinah-universal.vsix ]") && step.includes('"$FOUND" != "1"'),
		"the archive check no longer pins both the count and the name",
	);
	assert.ok(
		step.includes("found $FOUND") && step.includes("ls -1 vsix/"),
		"the archive check no longer reports the count it found and the directory listing",
	);
});

test("the release is tagged out of the CLI's tag namespace and carries one file", () => {
	// Every dinah CLI tag is "v" followed immediately by a digit. The
	// extension's version runs on its own cadence and can reach a number the
	// CLI reaches too, so the two namespaces are kept disjoint by the prefix
	// rather than by the numbers not having collided yet.
	assert.ok(
		vscodeRelease.includes('echo "tag=vscode-v$VERSION" >> "$GITHUB_OUTPUT"'),
		"the extension release no longer tags itself out of the CLI's tag namespace",
	);
	const step = vscodeRelease.slice(
		vscodeRelease.indexOf("- name: Create the GitHub Release"),
		vscodeRelease.indexOf("- name: Marketplace publish"),
	);
	assert.ok(
		step.includes("files: editors/vscode/vsix/dinah-universal.vsix"),
		"the release no longer attaches the one archive by name",
	);
	assert.ok(
		!step.includes("*.vsix"),
		"the release attaches the output directory rather than the one archive",
	);
	assert.ok(
		!/^\s+.*manifest\.json/m.test(step),
		"the release attaches vsix/manifest.json, which has no reader outside this repository",
	);
});

test("the extension release waits for this commit's own extension checks", () => {
	assert.ok(
		vscodeRelease.includes('.name == "extension (ubuntu-latest)"') &&
			vscodeRelease.includes('.name == "extension (windows-latest)"'),
		"the wait step no longer selects the two check runs ci.yml's extension matrix produces",
	);
	assert.ok(
		vscodeRelease.includes('"$GOOD" -ge 2') && vscodeRelease.includes("of 2 checks done"),
		"the wait step's threshold and its progress line no longer agree on two checks",
	);
	assert.ok(
		vscodeRelease.includes("::error::an extension CI check on $SHA finished without success"),
		"a red extension check no longer fails the release run",
	);
	assert.ok(
		vscodeRelease.includes("needs: [check-version, wait-for-extension-ci]"),
		"the packaging job no longer waits for the extension checks",
	);
	assert.ok(
		vscodeRelease.includes('commits/$SHA/check-runs" --paginate') &&
			!vscodeRelease.includes("check-runs?per_page="),
		"the wait step reads a fixed page of check runs again, so a commit carrying more than one page could hide both extension legs",
	);
});

test("the marketplace publish is dormant until a token exists", () => {
	const step = vscodeRelease.slice(vscodeRelease.indexOf("- name: Marketplace publish"));
	assert.ok(
		step.includes("if: env.VSCE_PAT != ''"),
		"the publish step is no longer skipped when the marketplace token is absent",
	);
	const command = step
		.split("\n")
		.filter((line) => line.includes("vsce publish"));
	assert.equal(
		command.length,
		1,
		"the publish step no longer runs vsce publish exactly once",
	);
	assert.ok(
		command[0].includes("--skip-duplicate") && command[0].includes("--pre-release"),
		"the publish command no longer republishes idempotently as a pre-release",
	);
	assert.ok(
		command[0].includes("--packagePath vsix/dinah-universal.vsix"),
		"the publish command no longer names the archive it publishes",
	);
	// vsce spells this option two ways, -p and --pat, and a guard reading one
	// spelling is satisfied by the other. The VSCE_PAT check beside it catches
	// the reversal anybody would actually write; the character class catches
	// the one nobody would.
	assert.ok(
		!/ (?:-p|--pat)[ =]/.test(command[0]) && !command[0].includes("VSCE_PAT"),
		"the publish command puts the token in its arguments instead of leaving vsce to read VSCE_PAT",
	);
	assert.equal(
		vscodeRelease.split("vsce publish").length - 1,
		1,
		"the workflow publishes to the marketplace somewhere other than the one dormant step",
	);
});
