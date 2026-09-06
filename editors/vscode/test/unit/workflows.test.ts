// The workflow files this repository's release safety sits in, read as text.
//
// Neither half of the manual checks these mirror can be automated: nobody can
// plant a file in a dependency tree from a unit test, and nobody can dispatch
// a real release run from one. But each of those checks also has a half that
// is a plain file read, and a revert of either edit is silent otherwise. The
// gofmt scoping would come back as a red job for a reason unrelated to this
// repository's Go code, and the release gate would go back to publishing
// without an explicit check that CI reported success.

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

test("release.yml waits on ci.yml as a job, not by polling check runs", () => {
	assert.ok(
		/\n {2}ci:\n {4}uses: \.\/\.github\/workflows\/ci\.yml\n/.test(release),
		"release.yml no longer calls ci.yml as a reusable workflow",
	);
	assert.ok(
		/needs: \[compute-version, ci\]/.test(release),
		"release.yml's build job no longer waits on the ci job",
	);
	assert.ok(
		!release.includes("check-runs") && !release.includes("check_runs"),
		"release.yml still polls the check-runs API",
	);
});

test("release.yml's build and release jobs fail closed on an explicit result check", () => {
	// D-4: the gate must not rest on GitHub's own default handling of an
	// unconditioned job at the workflow_call boundary. Each job that consumes
	// CI's result reads needs.<job>.result directly and requires the literal
	// string "success", so failure, cancelled and skipped are all refused by
	// the same comparison. See dinah-363 D-4.
	const build = release.slice(
		release.indexOf("\n  build:"),
		release.indexOf("\n  release:"),
	);
	assert.ok(
		/if: needs\.compute-version\.result == 'success' && needs\.ci\.result == 'success'/.test(
			build,
		),
		"release.yml's build job no longer gates on an explicit needs.ci.result == 'success' check",
	);
	const releaseJob = release.slice(
		release.indexOf("\n  release:"),
		release.indexOf("\n  cleanup-tag:"),
	);
	assert.ok(
		/if: needs\.compute-version\.result == 'success' && needs\.build\.result == 'success'/.test(
			releaseJob,
		),
		"release.yml's release job no longer gates on an explicit needs.build.result == 'success' check",
	);
});

test("ci.yml stays callable and every job bounds its own runtime", () => {
	assert.ok(
		/workflow_call:/.test(ci),
		"ci.yml no longer accepts workflow_call, so release.yml cannot depend on it as a job",
	);
	for (const job of ["test", "gofmt", "extension"]) {
		const body = ci.slice(ci.indexOf(`\n  ${job}:`));
		const nextJob = body.slice(2).search(/\n {2}\S/);
		const scoped = nextJob === -1 ? body : body.slice(0, nextJob + 2);
		assert.ok(
			/timeout-minutes:\s*\d+/.test(scoped),
			`ci.yml's ${job} job has no explicit timeout-minutes`,
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

test("the extension release can also be started by hand, with nothing to fill in", () => {
	// A version-change gate cannot cut the first release at the version the
	// manifest already carries, which is how 1.0.0 became unreleasable. The
	// dispatch trigger takes no inputs because the manual path always releases
	// whatever version is currently committed.
	assert.ok(
		/^ {2}workflow_dispatch: \{\}$/m.test(vscodeRelease),
		"vscode-release.yml can no longer be dispatched by hand, so the version already in the manifest cannot be released",
	);
	const trigger = vscodeRelease.slice(
		vscodeRelease.indexOf("\non:"),
		vscodeRelease.indexOf("\npermissions:"),
	);
	assert.ok(
		!/workflow_dispatch:\s*\n\s+inputs:/.test(trigger),
		"the dispatch trigger asks for an input, so a manual run no longer releases the committed version on its own",
	);
});

test("each trigger reads its version from the step that can answer for it", () => {
	// A dispatched run carries no github.event.before to diff against, so the
	// push path's comparison would report no change and skip every job below
	// it. That is the failure this card exists to prevent, and it looks like a
	// working trigger from the outside because the run starts and goes green.
	const job = vscodeRelease.slice(
		vscodeRelease.indexOf("\n  check-version:"),
		vscodeRelease.indexOf("\n  ci:"),
	);
	const diff = job.slice(
		job.indexOf("- name: Read the version before and after this push"),
		job.indexOf("- name: Read the committed version for a manual run"),
	);
	assert.ok(
		diff.includes("if: github.event_name == 'push'"),
		"the push comparison no longer restricts itself to pushes, so a dispatched run would diff against a commit that does not exist",
	);
	const manual = job.slice(
		job.indexOf("- name: Read the committed version for a manual run"),
	);
	assert.ok(
		manual.includes("if: github.event_name == 'workflow_dispatch'"),
		"the manual version read is no longer restricted to dispatched runs",
	);
	assert.ok(
		manual.includes(`node -p "require('./package.json').version"`) &&
			manual.includes('echo "changed=true" >> "$GITHUB_OUTPUT"'),
		"the manual step no longer reads the committed version and declares it releasable",
	);
	// The job's outputs name steps.manual.outputs.version whether or not the
	// step ever writes it, and the create step reads that output for both the
	// release's name and its body. Without this assertion the write can be
	// deleted and the suite stays green while a dispatched run cuts a release
	// titled "Dinah for VS Code " with an empty version in it.
	assert.ok(
		manual.includes('echo "version=$VERSION" >> "$GITHUB_OUTPUT"'),
		"the manual step no longer publishes the version it read, so the release it cuts would be named and described with an empty version",
	);
	assert.ok(
		!/github\.event\.(before|created)/.test(manual),
		"the manual step consults a push-only field, which carries nothing on a dispatched run",
	);
	assert.ok(
		job.includes(
			"changed: ${{ steps.diff.outputs.changed || steps.manual.outputs.changed }}",
		) &&
			job.includes(
				"version: ${{ steps.diff.outputs.version || steps.manual.outputs.version }}",
			),
		"the job's outputs no longer fall back to whichever of the two version steps ran",
	);
});

test("a tag that already carries a release is replaced by hand and refused on a push", () => {
	// softprops/action-gh-release documents that an existing release at the
	// tag is updated with the run's assets rather than refused, so the push
	// half of the operator's 2026-09-06 ruling is a step rather than an
	// absence. Delete that step and a revert walking the manifest back to a
	// released version silently replaces that release's archive.
	const replace = vscodeRelease.slice(
		vscodeRelease.indexOf(
			"- name: Replace a pre-existing release at this tag on a manual run",
		),
		vscodeRelease.indexOf("- name: Refuse to overwrite a pre-existing release on a push"),
	);
	assert.ok(
		replace.includes("if: github.event_name == 'workflow_dispatch'"),
		"the delete-and-recreate step is no longer restricted to dispatched runs, so a push could destroy a published release",
	);
	assert.ok(
		replace.includes('gh api "repos/$REPO/releases"') &&
			replace.includes(`grep -Fxq "$TAG"`) &&
			replace.includes('gh release delete "$TAG" --repo "$REPO" --yes --cleanup-tag'),
		"the manual path no longer decides by testing this tag against the release list before deleting",
	);
	assert.ok(
		replace.includes("::error::the releases on $REPO could not be listed") &&
			replace.includes("exit 1"),
		"the manual path no longer fails when it cannot list the releases, so a lookup that could not answer reads as a tag with nothing on it",
	);
	const refuse = vscodeRelease.slice(
		vscodeRelease.indexOf("- name: Refuse to overwrite a pre-existing release on a push"),
		vscodeRelease.indexOf("- name: Create the GitHub Release"),
	);
	assert.ok(
		refuse.includes("if: github.event_name == 'push'"),
		"the refusal is no longer restricted to pushes, so a manual re-cut would fail on the release it came to replace",
	);
	assert.ok(
		refuse.includes('gh api "repos/$REPO/releases"') &&
			refuse.includes(`grep -Fxq "$TAG"`) &&
			refuse.includes("::error::a release already exists at $TAG") &&
			refuse.includes("exit 1"),
		"a push reaching an already-released version no longer fails loudly",
	);
	// This is the assertion that closes the round-two finding. gh documents
	// exit code 1 for a command that "fails for any reason", so a lookup
	// reading absence off a non-zero exit cannot tell a tag with no release
	// on it from a tag it was unable to ask about, and on the second one this
	// step would wave the run through to an action documented to update an
	// existing release in place.
	assert.ok(
		refuse.includes("::error::the releases on $REPO could not be listed"),
		"the push refusal no longer fails when it cannot list the releases, so an API error, a rate limit or a token problem lets the run overwrite a published release",
	);
	assert.ok(
		!refuse.includes("gh release view") && !replace.includes("gh release view"),
		"a collision step is back to asking gh release view, whose non-zero exit means both 'no such release' and 'could not look'",
	);
	assert.ok(
		!refuse.includes("gh release delete"),
		"the push path deletes a release, which the operator's ruling reserves for a manual run",
	);
	// Both steps have to sit ahead of the create step. Behind it they would
	// delete or refuse the release this run had already made.
	assert.ok(
		vscodeRelease.indexOf(
			"- name: Replace a pre-existing release at this tag on a manual run",
		) < vscodeRelease.indexOf("- name: Create the GitHub Release") &&
			vscodeRelease.indexOf(
				"- name: Refuse to overwrite a pre-existing release on a push",
			) < vscodeRelease.indexOf("- name: Create the GitHub Release"),
		"a pre-existing release is handled after the release is created rather than before it",
	);
	assert.ok(
		vscodeRelease.indexOf("- name: Tag this extension version") <
			vscodeRelease.indexOf(
				"- name: Replace a pre-existing release at this tag on a manual run",
			),
		"the tag these two steps read is computed after they run",
	);
});

test("the release trigger compares the version field rather than the file", () => {
	// The paths filter says the file changed and says nothing about which
	// field. Without this comparison a push that renamed a command or edited
	// the npm scripts cuts a release the marketplace then refuses, because it
	// already carries that version.
	const step = vscodeRelease.slice(
		vscodeRelease.indexOf("- name: Read the version before and after this push"),
		vscodeRelease.indexOf("\n  ci:"),
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
	const ciJob = vscodeRelease.slice(
		vscodeRelease.indexOf("\n  ci:"),
		vscodeRelease.indexOf("\n  package-and-release:"),
	);
	assert.ok(
		ciJob.includes("if: needs.check-version.outputs.changed == 'true'"),
		"the first job downstream of the version check is no longer gated on the version having changed",
	);
});

// A gh read answers over the whole collection, and its --jq filter may select
// and map but may not reduce.
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
// The sweep that found that instance found a second shape of the same class in
// the same file: a read that never asked to paginate and pinned per_page
// instead, which is page-scoped for the same reason and which a guard written
// against the reducing filter cannot see. Both shapes are refused below.
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
	.filter((line) => line.includes("gh api") && !line.trim().startsWith("#"));

test("every gh read asks over the whole collection rather than over one page", () => {
	assert.ok(ghCommands.length > 0, "the workflow no longer calls gh api at all");
	for (const command of ghCommands) {
		// The sweep that produced this guard found the class in two shapes: a
		// reducing --jq filter under --paginate, and a read that never asked
		// to paginate at all and pinned per_page instead. A guard written
		// against the first shape cannot see the second, so both are refused
		// here. Requiring --paginate is what closes the second, because a
		// fixed page is a page-scoped answer however few entries it asks for.
		assert.ok(
			command.includes("--paginate"),
			`a gh api read fetches one page rather than the whole collection: ${command.trim()}`,
		);
		assert.ok(
			!command.includes("per_page"),
			`a gh api read pins per_page, which bounds the answer to a page: ${command.trim()}`,
		);
		if (!command.includes("--jq")) {
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

test("the extension release depends on this commit's CI rather than polling it", () => {
	// D-7: dinah-363 retired polling check runs by SHA and name, and ci.yml's
	// header now states that nothing reads its job names by string. Depending
	// on the nested run keeps the property AC-7 was written for, which is that
	// nothing is packaged or released until this commit's CI has passed.
	assert.ok(
		/\n {2}ci:\n(?: {4}[^\n]*\n)* {4}uses: \.\/\.github\/workflows\/ci\.yml\n/.test(
			vscodeRelease,
		),
		"the extension release no longer calls ci.yml as a reusable workflow",
	);
	assert.ok(
		vscodeRelease.includes("needs: [check-version, ci]"),
		"the packaging job no longer waits for CI",
	);
	assert.ok(
		vscodeRelease.includes(
			"if: needs.check-version.result == 'success' && needs.ci.result == 'success'",
		),
		"the packaging job no longer refuses every CI result that is not the literal success",
	);
	assert.ok(
		!vscodeRelease.includes("check-runs") && !vscodeRelease.includes("check_runs"),
		"the extension release polls the check-runs API again",
	);
	assert.ok(
		!/extension \((?:ubuntu|windows)-latest\)/.test(vscodeRelease),
		"the extension release matches ci.yml's job names by string again",
	);
});

test("the marketplace publish is dormant until a token exists", () => {
	const step = vscodeRelease.slice(vscodeRelease.indexOf("- name: Marketplace publish"));
	assert.ok(
		step.includes("if: env.VSCE_PAT != ''"),
		"the publish step is no longer skipped when the marketplace token is absent",
	);
	// The two triggers publish under identical conditions, which today means
	// never. A manual-only bypass here would publish unattended the moment a
	// token appeared in Actions secrets, which is the operator's call to make
	// rather than this workflow's.
	assert.ok(
		!step.includes("github.event_name"),
		"the publish step distinguishes triggers, so a manual run can publish where a push cannot",
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
