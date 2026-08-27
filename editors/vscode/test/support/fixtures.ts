// Fixture workbenches, built by the binary this commit produces.
//
// This is the layer that makes "the extension speaks the machine surfaces"
// enforceable rather than asserted. The setup below runs
// `go build ./cmd/dinah` and then `dinah init` per fixture, so the extension
// parses the JSON the binary actually emits in the same run that built it. A
// change to verb.Status or to the refusal envelope turns these tests red
// rather than shipping a client that silently reads a field that moved.
//
// Mocked JSON would keep passing forever after a Go-side field moved, which is
// the whole failure mode this layer exists to catch.

import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { checkedEnv } from "./env";

/** Everything one test run's fixtures share. */
export interface FixtureRoot {
	/** The temp directory holding the binary, the fixtures and DINAH_HOME. */
	readonly tempRoot: string;
	/** The freshly built dinah binary. */
	readonly binary: string;
	/** The user base every child process writes into. */
	readonly home: string;
}

/** The name a built binary takes on this platform. */
export const BINARY_NAME = process.platform === "win32" ? "dinah.exe" : "dinah";

/**
 * Builds the binary under a fresh temp root.
 *
 * The temp root is never inside the checkout. dinah's discovery walk climbs to
 * the drive root, so a fixture created inside the repository would put that
 * walk into the repository's own .dinah, which is the dinah-241 wound.
 */
export function buildBinary(repoRoot: string): FixtureRoot {
	const tempRoot = mkdtempSync(join(tmpdir(), "dinah-vscode-"));
	const binary = join(tempRoot, BINARY_NAME);
	execFileSync("go", ["build", "-o", binary, "./cmd/dinah"], {
		cwd: repoRoot,
		stdio: "inherit",
	});
	const home = join(tempRoot, "home");
	mkdirSync(home, { recursive: true });
	return { tempRoot, binary, home };
}

/** The environment every fixture-building invocation runs with. */
export function fixtureEnv(root: FixtureRoot): NodeJS.ProcessEnv {
	return checkedEnv(root.tempRoot, {
		DINAH_HOME: root.home,
		DINAH_ACTOR: "fixture",
	});
}

/** Runs `dinah init` in `dir`, optionally under a chosen slug. */
export function initBench(root: FixtureRoot, dir: string, slug?: string): void {
	mkdirSync(dir, { recursive: true });
	const args = ["--json", "init"];
	if (slug !== undefined) {
		args.push("--slug", slug);
	}
	execFileSync(root.binary, args, {
		cwd: dir,
		env: fixtureEnv(root),
		stdio: "pipe",
	});
}

/** Writes a workspace-folder settings file pinning the binary this run built. */
export function pinBinary(root: FixtureRoot, folder: string): void {
	const dir = join(folder, ".vscode");
	mkdirSync(dir, { recursive: true });
	writeFileSync(
		join(dir, "settings.json"),
		`${JSON.stringify({ "dinah.path": root.binary }, null, 2)}\n`,
		"utf8",
	);
}

/**
 * Files one card on the workbench in `dir`, returning its reference.
 *
 * The tree suite needs a bench that actually holds cards, and building one
 * through the binary rather than by writing card files keeps that fixture on
 * the same footing as every other one here: the extension reads what this
 * commit's binary emits, not what a fixture author believed it emits.
 */
export function addCard(root: FixtureRoot, dir: string, title: string): string {
	const out = execFileSync(root.binary, ["--json", "add", title], {
		cwd: dir,
		env: fixtureEnv(root),
		stdio: "pipe",
	}).toString();
	const answer = JSON.parse(out) as { card?: { ref?: string } };
	const ref = answer.card?.ref;
	if (ref === undefined || ref === "") {
		throw new Error(`dinah add answered with no reference: ${out}`);
	}
	return ref;
}
