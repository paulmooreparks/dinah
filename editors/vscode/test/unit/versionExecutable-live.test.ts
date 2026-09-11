// A shipped binary held to saying where it is.
//
// Every other check of the version report runs inside the go test process or
// over a payload a test wrote, and neither can make this claim. `os.Executable`
// under `go test` reports the test binary's own path, which is absolute and
// exists and so satisfies an assertion that proves nothing about a shipped
// dinah. This file builds one from the working tree, runs `--json version`
// against it, and compares what it said about itself with the path `go build`
// was told to write it to.
//
// The second test carries that same payload through the extension's own decode.
// The raw assertion never reaches `readReport` and the decode's own unit test
// never reaches a binary, so without this join the CLI could stop sending the
// field, or the decode could start dropping it, with both of those green.
//
// This is the second unit file that starts a process, and
// test/unit/layers.test.ts carries the exemption with the reason. The cost is
// one `go build` and two short-lived spawns.

import assert from "node:assert/strict";
import { realpathSync, rmSync } from "node:fs";
import { join } from "node:path";
import { after, test } from "node:test";

import { runDinah } from "../../src/cli";
import { nodeSpawner } from "../../src/spawn";
import { classifyVersion } from "../../src/version";
import type { FixtureRoot } from "../support/fixtures";
import { buildBinary, fixtureEnv } from "../support/fixtures";

const extensionRoot = join(__dirname, "..", "..", "..");
const repoRoot = join(extensionRoot, "..", "..");

/**
 * The binary both tests read, built once.
 *
 * `go build` is the whole cost of this file, so it is paid by the first test
 * that needs it and shared rather than paid twice.
 */
let built: FixtureRoot | undefined;

function binary(): FixtureRoot {
	built ??= buildBinary(repoRoot);
	return built;
}

after(() => {
	if (built !== undefined) {
		rmSync(built.tempRoot, { recursive: true, force: true });
	}
});

/**
 * Where a spawn runs and what it inherits.
 *
 * Inside the run's own temp root with DINAH_HOME under it, so neither the
 * discovery walk nor a configuration write can reach the operator's own
 * workbenches.
 */
function options(root: FixtureRoot): { cwd: string; env: NodeJS.ProcessEnv } {
	return { cwd: root.tempRoot, env: fixtureEnv(root) };
}

/**
 * One documented canonicalisation, applied identically to both operands.
 *
 * Node documents `realpathSync.native` as synchronous `realpath(3)`, and POSIX
 * documents `realpath()` as producing an absolute pathname naming the same
 * file with no `.`, no `..` and no symbolic link left in it. Two spellings that
 * no lesser transformation reconciles are in reach here: a Windows runner's
 * `TEMP` is an 8.3 short name, and a POSIX temporary directory can carry a
 * symlinked component. `resolve()` reaches neither and a case fold reaches
 * neither, so neither is a substitute. A difference that survives this on both
 * sides is a real failure.
 */
function canonical(path: string): string {
	return realpathSync.native(path);
}

test("a built dinah reports the path it was built to", async () => {
	// dinah-424 AC-14's second arm. The process observed here is a shipped
	// dinah rather than the go test binary, which is what no assertion in
	// cmd/dinah can be.
	const root = binary();
	const outcome = await runDinah(
		nodeSpawner,
		root.binary,
		["version"],
		options(root),
	);
	assert.equal(
		outcome.kind,
		"ok",
		`the built binary answered --json version with ${outcome.kind}`,
	);
	if (outcome.kind !== "ok") {
		return;
	}
	const payload = outcome.json as { executable?: unknown };
	assert.equal(
		typeof payload.executable,
		"string",
		`the built binary reported no executable: ${JSON.stringify(outcome.json)}`,
	);
	assert.equal(canonical(payload.executable as string), canonical(root.binary));
});

test("the extension's own decode carries that same answer through", async () => {
	// dinah-424 AC-17, the only assertion on this card whose two operands come
	// from opposite sides of the CLI-to-extension seam. AC-14's arm above
	// reads the raw JSON and never reaches readReport; version.test.ts reaches
	// readReport over a payload it wrote itself and never reaches a binary.
	const root = binary();
	const classification = classifyVersion(
		await runDinah(nodeSpawner, root.binary, ["version"], options(root)),
	);
	assert.equal(classification.kind, "ok");
	if (classification.kind !== "ok") {
		return;
	}
	const reported = classification.version.executable;
	assert.equal(
		typeof reported,
		"string",
		"the decode dropped the executable the binary reported",
	);
	assert.equal(canonical(reported as string), canonical(root.binary));
});
