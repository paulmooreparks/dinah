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

import { load as loadYaml } from "js-yaml";

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

// The extension release workflow read as a document rather than as text.
//
// Several of the decisions below are about the shape of the file (which paths
// the trigger names, which jobs exist, what each one depends on) and a text
// search answers those only by accident. js-yaml is the parser, and it keeps
// `on` as the string key GitHub means: YAML 1.1 resolved that token as a
// boolean, and js-yaml 4 implements the YAML 1.2 core schema, whose bool type
// resolves only true and false.
//
// What this still cannot do is run the workflow. A file that parses and carries
// the right keys can still fail on a real trigger, and nothing in this
// repository executes a workflow (dinah-401). Each assertion below is a claim
// about the file, never about a run.
interface WorkflowJob {
	needs?: string | string[];
	if?: string;
	uses?: string;
	permissions?: Record<string, string>;
	outputs?: Record<string, string>;
	steps?: {
		name?: string;
		id?: string;
		if?: string;
		uses?: string;
		run?: string;
		with?: Record<string, string>;
		env?: Record<string, string>;
	}[];
}
interface Workflow {
	on: { push?: { branches?: string[]; paths?: string[] }; workflow_dispatch?: unknown };
	jobs: Record<string, WorkflowJob>;
}
const vscodeReleaseDoc = loadYaml(vscodeRelease) as Workflow;

/** Every step of one job, or an empty list when the job runs no steps of its own. */
function stepsOf(job: string): NonNullable<WorkflowJob["steps"]> {
	return vscodeReleaseDoc.jobs[job]?.steps ?? [];
}

/** Every step name in the whole workflow. */
function everyStepName(): string[] {
	return Object.keys(vscodeReleaseDoc.jobs).flatMap((job) =>
		stepsOf(job).map((step) => step.name ?? ""),
	);
}

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

test("the extension release fires on main for anything under the extension", () => {
	// The trigger used to name the manifest alone, because the version was a
	// field somebody typed and a push that did not touch it released nothing.
	// Nobody types it now, so a code, docs or test change under editors/vscode
	// produces an archive whose bytes differ, and the operator's rule is that a
	// new archive means a new release.
	const push = vscodeReleaseDoc.on.push;
	assert.deepEqual(push?.branches, ["main"], "the extension release no longer fires on main");
	assert.deepEqual(
		push?.paths,
		["editors/vscode/**"],
		"the push trigger's paths filter is no longer exactly the extension directory, so it either misses extension changes or fires on pushes outside it",
	);
});

test("the extension release can also be started by hand, with nothing to fill in", () => {
	// The dispatch trigger takes no inputs, because a dispatched run computes
	// its version exactly as a push does rather than asking anybody for one.
	assert.ok(
		"workflow_dispatch" in vscodeReleaseDoc.on,
		"vscode-release.yml can no longer be dispatched by hand",
	);
	const dispatch = vscodeReleaseDoc.on.workflow_dispatch;
	assert.ok(
		dispatch === null || dispatch === undefined || !("inputs" in (dispatch as object)),
		"the dispatch trigger asks for an input, so a manual run no longer cuts a release on its own",
	);
});

test("one job computes the version for both triggers", () => {
	// The workflow used to carry two version-reading steps, one per trigger, and
	// each was gated on github.event_name. That shape is what made the first
	// release impossible to cut from a push, and a dispatched run released
	// whatever the manifest currently said. compute-version answers for both
	// triggers by computing the next patch from the release history, so nothing
	// in it branches on which trigger fired.
	const jobs = Object.keys(vscodeReleaseDoc.jobs);
	assert.ok(
		!jobs.includes("check-version"),
		"the job that compared the manifest across the push is still here",
	);
	assert.ok(jobs.includes("compute-version"), "there is no compute-version job");
	for (const gone of [
		"Read the version before and after this push",
		"Read the committed version for a manual run",
	]) {
		assert.ok(
			!everyStepName().includes(gone),
			`the step "${gone}" survived, so the per-trigger version reading is still here`,
		);
	}
	const compute = vscodeReleaseDoc.jobs["compute-version"];
	for (const step of compute.steps ?? []) {
		assert.equal(
			step.if,
			undefined,
			`compute-version's step "${step.name ?? step.uses ?? "(unnamed)"}" is conditional, so it does not answer for both triggers`,
		);
	}
	assert.deepEqual(
		compute.outputs,
		{
			tag: "${{ steps.version.outputs.tag }}",
			version: "${{ steps.version.outputs.version }}",
		},
		"compute-version no longer publishes both the tag and the version every downstream job reads",
	);
	// Every job below it reads those outputs rather than recomputing anything.
	for (const job of ["ci", "package-and-release", "cleanup-tag"]) {
		const needs = vscodeReleaseDoc.jobs[job].needs;
		const list = typeof needs === "string" ? [needs] : (needs ?? []);
		assert.ok(
			list.includes("compute-version"),
			`${job} no longer waits on compute-version`,
		);
	}
});

test("nothing in the release job reads the version back out of the manifest", () => {
	// This is the trap the redesign has to avoid. package.mjs writes the
	// computed version into package.json, packages, and restores the committed
	// floor in a finally block, so any later step reading package.json's version
	// reads the floor. A run doing that would tag and title every release
	// 1.0.0 for ever while the archive inside carried the real number.
	const release = vscodeReleaseDoc.jobs["package-and-release"];
	for (const step of release.steps ?? []) {
		const text = JSON.stringify(step);
		// The quoting varies with how the read is spelled, so the pattern covers
		// the bare form and both quoted ones rather than one literal substring.
		// What used to sit here was `node -p "require('./package.json').version"`,
		// and a check written against `package.json).version` alone walks past it
		// because of the apostrophe in the middle.
		assert.ok(
			!/package\.json(?:\\?['"])?\)\.version/u.test(text),
			`the step "${step.name ?? "(unnamed)"}" reads package.json's version back out after packaging restored the floor`,
		);
		assert.ok(
			!text.includes("steps.tag.outputs"),
			`the step "${step.name ?? "(unnamed)"}" reads a tag computed inside this job rather than the one compute-version reserved`,
		);
	}
	const create = (release.steps ?? []).find(
		(step) => step.name === "Create the GitHub Release",
	);
	assert.ok(create !== undefined, "the release is no longer created");
	assert.equal(create.with?.tag_name, "${{ needs.compute-version.outputs.tag }}");
	assert.ok(
		create.with?.name?.includes("${{ needs.compute-version.outputs.version }}"),
		"the release's title no longer carries the version compute-version minted",
	);
	const packaging = (release.steps ?? []).find((step) => step.name === "Package the archive");
	assert.ok(
		packaging?.run?.includes(
			'npm run package -- --version "${{ needs.compute-version.outputs.version }}"',
		),
		"the packaging step is no longer handed the version compute-version minted",
	);
});

test("the collision steps that only made sense under a typed version are gone", () => {
	// Both existed because a dispatched run could land on a version a push had
	// already released. compute-version reserves an unused tag before anything
	// is built, so neither can fire. The tagging step went with them, because it
	// read the manifest after packaging had put the floor back.
	for (const gone of [
		"Replace a pre-existing release at this tag on a manual run",
		"Refuse to overwrite a pre-existing release on a push",
		"Tag this extension version",
	]) {
		assert.ok(
			!everyStepName().includes(gone),
			`the step "${gone}" survived, so the redesign was layered on the old mechanism instead of replacing it`,
		);
	}
});

test("the tag is reserved before the build and deleted when the run fails", () => {
	// Two pushes landing close together read the same release list and compute
	// the same next patch. Only the first POST to git/refs succeeds, so the
	// reservation is what makes the race loud rather than silent, and it has to
	// happen before anything is built. A run that reserves and then fails would
	// otherwise burn that number for ever, which is what cleanup-tag prevents.
	// release.yml carries the same pair for the same reason.
	const compute = vscodeReleaseDoc.jobs["compute-version"];
	const reserve = (compute.steps ?? []).find((step) =>
		step.run?.includes('gh api -X POST "repos/$REPO/git/refs"'),
	);
	assert.ok(reserve !== undefined, "compute-version no longer reserves the tag it minted");
	assert.ok(
		reserve.run?.includes('-f ref="refs/tags/$TAG"'),
		"the reservation no longer creates the tag ref this run computed",
	);
	assert.equal(
		compute.permissions?.contents,
		"write",
		"compute-version cannot create a tag ref without contents: write",
	);
	// Reserving inside compute-version is what puts it ahead of the build, since
	// both later jobs wait on this one.
	for (const job of ["ci", "package-and-release"]) {
		const needs = vscodeReleaseDoc.jobs[job].needs;
		const list = typeof needs === "string" ? [needs] : (needs ?? []);
		assert.ok(list.includes("compute-version"), `${job} does not wait for the reservation`);
	}

	const cleanup = vscodeReleaseDoc.jobs["cleanup-tag"];
	assert.ok(cleanup !== undefined, "no job deletes the tag a failed run reserved");
	assert.deepEqual(
		cleanup.needs,
		["compute-version", "ci", "package-and-release"],
		"cleanup-tag no longer watches every job that can leave the reserved tag orphaned",
	);
	assert.equal(
		cleanup.if,
		"always() && needs.compute-version.result == 'success' && (contains(needs.*.result, 'failure') || contains(needs.*.result, 'cancelled'))",
		"cleanup-tag no longer fires exactly when a reservation succeeded and something after it failed or was cancelled",
	);
	const deletion = (cleanup.steps ?? []).find((step) =>
		step.run?.includes('gh api -X DELETE "repos/$REPO/git/refs/tags/$TAG"'),
	);
	assert.ok(deletion !== undefined, "cleanup-tag no longer deletes the tag ref");
	assert.equal(
		deletion.env?.TAG,
		"${{ needs.compute-version.outputs.tag }}",
		"cleanup-tag deletes a tag other than the one compute-version reserved",
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

// A write is not a read, and pagination is meaningless on one. The tag this
// workflow reserves and the tag a failed run deletes are both single-resource
// calls that name their method, so they are held out here rather than being
// made to carry a --paginate flag that would say nothing.
const ghCommands = vscodeRelease
	.replace(/\\\n\s*/g, " ")
	.split("\n")
	.filter(
		(line) =>
			line.includes("gh api") &&
			!line.trim().startsWith("#") &&
			!/-X (?:POST|PATCH|PUT|DELETE)\b/.test(line),
	);

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

test("the workflow consults no field only one of its two triggers carries", () => {
	// The push path used to diff the manifest across github.event.before, which
	// a dispatched run does not carry, and it decided whether there was an
	// earlier commit from github.event.created. Neither field exists on both
	// triggers, and compute-version answers for both, so reading either one
	// again would reintroduce a path that works under one trigger and silently
	// skips everything under the other.
	for (const field of ["github.event.before", "github.event.created"]) {
		assert.ok(
			!vscodeRelease.includes(field),
			`the workflow reads ${field}, which carries nothing on a dispatched run`,
		);
	}
	assert.ok(
		!/0{20,}/.test(vscodeRelease),
		"the workflow compares a SHA against an all-zero sentinel, which appears in no GitHub documentation of the push payload",
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
		step.includes(
			'npm run package -- --version "${{ needs.compute-version.outputs.version }}"',
		),
		"the packaging step no longer packages the version compute-version minted, so it would fall back to the committed floor",
	);
	assert.ok(
		step.includes("npm run verify-package"),
		"the packaging step no longer verifies what it packaged",
	);
});

test("the run fails unless exactly one archive was produced", () => {
	const step = vscodeRelease.slice(
		vscodeRelease.indexOf("- name: Confirm exactly one archive was produced"),
		vscodeRelease.indexOf("- name: Create the GitHub Release"),
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
	const compute = vscodeReleaseDoc.jobs["compute-version"];
	const minted = (compute.steps ?? []).find((step) => step.run?.includes("TAG="));
	assert.ok(
		minted?.run?.includes('TAG="vscode-v$VERSION"'),
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
	assert.deepEqual(
		vscodeReleaseDoc.jobs["package-and-release"].needs,
		["compute-version", "ci"],
		"the packaging job no longer waits for CI",
	);
	assert.equal(
		vscodeReleaseDoc.jobs["package-and-release"].if,
		"needs.compute-version.result == 'success' && needs.ci.result == 'success'",
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
