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

test("the gofmt job is scoped to the Go trees", () => {
	// `gofmt -l .` walks into editors/vscode/node_modules/, so an npm
	// dependency that vendors an unformatted .go file as testdata turns the
	// job red for a reason that has nothing to do with this repository.
	assert.ok(
		ci.includes("gofmt -l cmd internal"),
		"ci.yml no longer scopes gofmt to cmd and internal",
	);
	assert.ok(
		!/gofmt -l \.\s*\)/.test(ci) && !ci.includes("gofmt -l .)"),
		"ci.yml still runs gofmt over the whole tree",
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
