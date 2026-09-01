// The two workflow files this card's safety sits in, read as text.
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
