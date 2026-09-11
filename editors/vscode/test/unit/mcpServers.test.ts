// What this window publishes to the editor, and what it refuses to publish.
//
// Every check here reads plain data the test constructed, so none of them
// reaches a binary, a decode or an editor. The two sweeps at the end are the
// exception and say so: they read the text of every file under src/, because a
// claim about a whole tree is produced by a command over the tree rather than
// by reading the module that was open.

import assert from "node:assert/strict";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { test } from "node:test";

import type { BinaryState } from "../../src/api";
import {
	mcpPlansDiffer,
	planMcpServers,
	publishedMcpServers,
} from "../../src/mcpServers";
import type { McpTarget } from "../../src/tree";

// This file is compiled to out/test/unit/, so the extension root is three up.
const extensionRoot = join(__dirname, "..", "..", "..");
const srcRoot = join(extensionRoot, "src");

/**
 * An absolute path standing in for what a binary reported about itself.
 *
 * Absoluteness is a property of the platform the code is running on, not of
 * the string: `node:path`'s `isAbsolute` reads `C:/tools/dinah.exe` as
 * relative under POSIX, so a Windows-shaped literal here passes on a Windows
 * runner and fails the publishing arms on the Linux and macOS ones. The
 * fixture therefore names a path this platform calls absolute. The roots below
 * stay Windows-shaped on every platform on purpose, because the dedup fold is
 * about separators and case rather than about absoluteness and never asks the
 * platform anything.
 */
const EXECUTABLE =
	process.platform === "win32" ? "C:/tools/dinah.exe" : "/usr/local/bin/dinah";

function target(root: string, title = ""): McpTarget {
	return { root, title };
}

/** A resolved binary reporting `executable`, as `--json version` would. */
function resolved(executable?: string): BinaryState {
	return {
		state: "ok",
		path: "dinah",
		source: "path",
		version: {
			tool: "0.1.0",
			profile: "dinah-core/0.4",
			format: 2,
			executable,
		},
	};
}

test("a published plan starts dinah on that workbench and nothing else", () => {
	// AC-3. The whole argv is compared rather than searched for
	// `--workbench`, because an argv carrying the flag after the command word
	// parses differently and a containment check passes on it.
	const plans = planMcpServers([target("C:/w", "Ledger")], EXECUTABLE, "0.9.2", true);
	assert.equal(plans.length, 1);
	assert.deepEqual([...plans[0].args], ["--workbench", "C:/w", "mcp"]);
	// The command is the string this test handed in, verbatim. The function is
	// free to compose a different one, which is what makes this an assertion
	// rather than a value compared with itself.
	assert.equal(plans[0].command, EXECUTABLE);
	assert.equal(plans[0].cwd, "C:/w");
	assert.equal(plans[0].version, "0.9.2");
});

test("one plan per distinct root, folding separators always and case only where told", () => {
	// AC-4. Three inputs, each its own assertion, and the fold under test is
	// planMcpServers's own: mcpTargets returns one target per answered row and
	// folds nothing.
	for (const caseInsensitive of [true, false]) {
		const separators = planMcpServers(
			[target("C:/w"), target("C:\\w")],
			EXECUTABLE,
			"0.1.0",
			caseInsensitive,
		);
		assert.equal(
			separators.length,
			1,
			`two spellings of one directory published ${String(separators.length)} plans with caseInsensitive ${String(caseInsensitive)}`,
		);
	}

	const foldedCase = planMcpServers(
		[target("C:/W"), target("C:/w")],
		EXECUTABLE,
		"0.1.0",
		true,
	);
	assert.equal(foldedCase.length, 1, "case was not folded where it is folded");
	const keptCase = planMcpServers(
		[target("C:/W"), target("C:/w")],
		EXECUTABLE,
		"0.1.0",
		false,
	);
	assert.equal(keptCase.length, 2, "case was folded where it is not folded");

	// The accepting case beside the folding one. A key compared by prefix
	// rather than over the whole string would collapse these two.
	for (const caseInsensitive of [true, false]) {
		const neighbours = planMcpServers(
			[target("C:/w"), target("C:/w-old")],
			EXECUTABLE,
			"0.1.0",
			caseInsensitive,
		);
		assert.equal(
			neighbours.length,
			2,
			`C:/w and C:/w-old were folded together with caseInsensitive ${String(caseInsensitive)}`,
		);
	}
});

test("a label names the workbench, and two that would read alike name their roots", () => {
	// AC-5, six cases: the four the criterion names, plus both halves of the
	// case fold the collision check applies.
	const titled = planMcpServers([target("C:/w", "Ledger")], EXECUTABLE, "0.1.0", true);
	assert.equal(titled[0].label, "Dinah: Ledger");

	// The untitled case never reads `Dinah: Dinah`, which is what it would read
	// if mcpTargets substituted UNTITLED_WORKBENCH the way rootsFor does.
	const untitled = planMcpServers(
		[target("C:/scratch/wt", "")],
		EXECUTABLE,
		"0.1.0",
		true,
	);
	assert.equal(untitled[0].label, "Dinah: wt");

	// Both colliding plans carry their root, not only the second. A reader
	// cannot tell which of two same-titled workbenches got the bare label.
	const colliding = planMcpServers(
		[target("C:/a", "Ledger"), target("C:/b", "Ledger")],
		EXECUTABLE,
		"0.1.0",
		true,
	);
	assert.deepEqual(
		colliding.map((plan) => plan.label),
		["Dinah: Ledger (C:/a)", "Dinah: Ledger (C:/b)"],
	);

	// A workbench genuinely called Dinah reads `Dinah: Dinah` and collides with
	// nothing, because the untitled case above never produces that string.
	const named = planMcpServers([target("C:/w", "Dinah")], EXECUTABLE, "0.1.0", true);
	assert.equal(named[0].label, "Dinah: Dinah");

	// Two titles differing only in case collide where the platform folds case
	// and stand apart where it does not, which is the same rule the dedup
	// above applies to roots. Comparing the composed labels exactly would read
	// the first of these as two distinct labels and publish neither root.
	const foldedTitles = planMcpServers(
		[target("C:/a", "Ledger"), target("C:/b", "ledger")],
		EXECUTABLE,
		"0.1.0",
		true,
	);
	assert.deepEqual(
		foldedTitles.map((plan) => plan.label),
		["Dinah: Ledger (C:/a)", "Dinah: ledger (C:/b)"],
	);
	const distinctTitles = planMcpServers(
		[target("C:/a", "Ledger"), target("C:/b", "ledger")],
		EXECUTABLE,
		"0.1.0",
		false,
	);
	assert.deepEqual(
		distinctTitles.map((plan) => plan.label),
		["Dinah: Ledger", "Dinah: ledger"],
	);
});

test("the reader's switch decides, and so does whether the plan set moved", () => {
	// AC-12. Both arms against one fixture: a decision that always returns
	// nothing fails the true arm, and one that ignores the setting fails the
	// false arm. The BinaryState is built here, so nothing reaches a binary.
	const targets = [target("C:/w", "Ledger")];
	assert.deepEqual(publishedMcpServers(resolved(EXECUTABLE), false, targets, true), []);
	assert.equal(publishedMcpServers(resolved(EXECUTABLE), true, targets, true).length, 1);

	// A checkpoint that resolves the same set must not fire, because the
	// editor reads a change as a reason to prompt the reader to refresh their
	// tools, and firing every poll interval is a real cost to them.
	let fired = 0;
	let published = publishedMcpServers(resolved(EXECUTABLE), true, targets, true);
	for (let round = 0; round < 2; round += 1) {
		const next = publishedMcpServers(resolved(EXECUTABLE), true, targets, true);
		if (mcpPlansDiffer(published, next)) {
			fired += 1;
			published = next;
		}
	}
	assert.equal(fired, 0, "an unchanged fixture reported a change");

	const grown = publishedMcpServers(
		resolved(EXECUTABLE),
		true,
		[...targets, target("C:/other", "Other")],
		true,
	);
	assert.equal(
		mcpPlansDiffer(published, grown) ? 1 : 0,
		1,
		"a fixture that gained a root reported no change",
	);
});

test("nothing is published without an absolute path the binary itself reported", () => {
	// AC-15. Four arms against one fixture, because a decision that always
	// returned an empty array would satisfy any three of them.
	const targets = [target("C:/w", "Ledger")];

	// No binary resolved, so nothing knows where one is.
	assert.deepEqual(publishedMcpServers({ state: "no-binary" }, true, targets, true), []);
	// A binary older than the field reports none, and so does one whose own
	// os.Executable failed. Both publish nothing and neither falls back.
	assert.deepEqual(publishedMcpServers(resolved(undefined), true, targets, true), []);
	// A relative answer no documented path produces, which is what a future
	// regression would produce. It is refused rather than replaced by the bare
	// name.
	assert.deepEqual(publishedMcpServers(resolved("dinah"), true, targets, true), []);

	const published = publishedMcpServers(resolved(EXECUTABLE), true, targets, true);
	assert.equal(published.length, 1);
	assert.equal(published[0].command, EXECUTABLE);
});

/** Every `.ts` file under src/, by its path relative to src/. */
function sourceFiles(): string[] {
	const found: string[] = [];
	const walk = (dir: string): void => {
		for (const entry of readdirSync(dir)) {
			const full = join(dir, entry);
			if (statSync(full).isDirectory()) {
				walk(full);
				continue;
			}
			if (entry.endsWith(".ts")) {
				found.push(relative(srcRoot, full).split("\\").join("/"));
			}
		}
	};
	walk(srcRoot);
	return found;
}

test("no module under src writes a reader's own configuration", () => {
	// AC-7. This is the claim that nothing of the reader's is written, and a
	// claim about a whole tree is produced by a command over the tree. It is
	// weaker than it reads and says so: it catches the spellings somebody
	// would actually write and cannot catch an obfuscated path composition.
	const files = sourceFiles();
	assert.ok(files.length > 0, "no file under src was read at all, so this check proved nothing");
	const offenders: string[] = [];
	for (const rel of files) {
		const body = readFileSync(join(srcRoot, rel), "utf8");
		for (const [at, line] of body.split("\n").entries()) {
			const writes =
				line.includes("mcp.json") ||
				/getConfiguration\([^)]*\)\s*\.\s*update\b/.test(line) ||
				/["'`][^"'`]*\.vscode\//.test(line);
			if (writes) {
				offenders.push(`${rel}:${String(at + 1)}: ${line.trim()}`);
			}
		}
	}
	assert.deepEqual(
		offenders,
		[],
		`these lines reach a reader's own configuration:\n${offenders.join("\n")}`,
	);
});

test("the published labels carry no authored English, so no localiser can reach them", () => {
	// AC-9's first half, held structurally rather than by running the
	// composition under eight tags. planMcpServers takes no Localizer and no
	// `t`, and no module under src/ holds one at module level, so a loop over
	// the tags would call one function eight times with identical arguments
	// and compare eight identical results.
	const body = readFileSync(join(srcRoot, "mcpServers.ts"), "utf8");
	assert.ok(body.length > 0, "mcpServers.ts read as empty, so this check proved nothing");
	assert.equal(
		/^import[^\n]*from "\.\/l10n";?$/m.test(body),
		false,
		"mcpServers.ts imports the localiser, so a published label could change with the editor's display language",
	);
	// A whole-identifier `t`, so this does not fire on `split(` or on any
	// other identifier ending in t.
	assert.equal(/\bt\(/.test(body), false, "mcpServers.ts calls a localiser");
});

test("the absoluteness rung reaches one module and the bare name reaches one other", () => {
	// AC-15's second half. The arms above catch a rung that was moved out of
	// the pure function; neither they nor a plant that moves it can see a copy
	// left in extension.ts beside the original, and that is where the drift
	// starts.
	const files = sourceFiles();
	assert.ok(files.length > 0, "no file under src was read at all, so this check proved nothing");
	const absolute: string[] = [];
	const bareName: string[] = [];
	for (const rel of files) {
		const body = readFileSync(join(srcRoot, rel), "utf8");
		if (/\bisAbsolute\b/.test(body) && rel !== "mcpServers.ts") {
			absolute.push(rel);
		}
		// binary.ts spawns the bare name to probe it, and probing it is what
		// produces the absolute path this card publishes instead.
		if (/\bPATH_NAME\b/.test(body) && rel !== "binary.ts") {
			bareName.push(rel);
		}
	}
	assert.deepEqual(
		absolute,
		[],
		`these modules decide absoluteness and only mcpServers.ts may: ${absolute.join(", ")}`,
	);
	assert.deepEqual(
		bareName,
		[],
		`these modules name the bare dinah and only binary.ts may: ${bareName.join(", ")}`,
	);
});
