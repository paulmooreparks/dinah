// The two test layers, kept apart.
//
// The unit layer runs on every push, on three platforms, in seconds. It buys
// that by never starting a process and never opening an editor. One unit file
// that spawns a real dinah, or imports the vscode module, turns the fast layer
// into the slow one and does it quietly: the suite still passes, it just costs
// a minute more each time and fails on whichever runner has no Go toolchain.
//
// The integration layer is the opposite bargain and is reachable only through
// `npm run test:integration`, which CI runs on its own. It is never run
// locally, which is a standing instruction on this workbench rather than a
// preference: it launches real editors against real fixture workbenches, and
// dinah's own discovery walk climbs to the drive root from wherever those
// fixtures sit.

import assert from "node:assert/strict";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { test } from "node:test";

// This file is compiled to out/test/unit/, so the extension root is three up.
const extensionRoot = join(__dirname, "..", "..", "..");
const testRoot = join(extensionRoot, "test");

function walk(dir: string): string[] {
	const found: string[] = [];
	for (const entry of readdirSync(dir)) {
		const full = join(dir, entry);
		if (statSync(full).isDirectory()) {
			found.push(...walk(full));
		} else if (entry.endsWith(".ts")) {
			found.push(full);
		}
	}
	return found;
}

function posix(path: string): string {
	return relative(testRoot, path).split("\\").join("/");
}

/**
 * Every file under test/, by which layer's directory holds it.
 *
 * Every check below that filters this roster for offenders asserts the roster
 * itself is non-empty before it reads the offenders, because an offender list
 * computed from no files is empty for the wrong reason. Renaming test/unit is
 * all it takes: the filter then matches nothing, the offender list is empty,
 * and the check reports a clean layer it never looked at.
 */
function layerFiles(prefix: string): string[] {
	return walk(testRoot)
		.map(posix)
		.filter((rel) => rel.startsWith(prefix));
}

/**
 * An import of `mod`, as a value rather than as a type.
 *
 * Anchored at the start of a line and ending at the statement's semicolon,
 * rather than matching the module's name anywhere in the file. A pattern that
 * matched anywhere would flag these very checks, whose own regex literals name
 * the modules they are looking for, and an author faced with a guard that
 * flags itself takes the guard out.
 *
 * A type-only import is deliberately not matched. TypeScript erases it, so it
 * costs the layer nothing and forbidding it would forbid a unit test from
 * naming the shapes it asserts on.
 */
function valueImportOf(mod: string): RegExp {
	return new RegExp(`^import (?!type )[^\\n]*from "${mod}";?$`, "m");
}

test("no unit-test file starts a process", () => {
	// A unit test that shells out to `go build` or to a real dinah is an
	// integration test filed in the wrong directory, and it takes the whole
	// fast layer's runtime with it.
	const files = layerFiles("unit/");
	assert.ok(
		files.length > 0,
		"no unit-test file was scanned at all, so this check proved nothing",
	);
	const offenders = files.filter((rel) => {
		const body = readFileSync(join(testRoot, rel), "utf8");
		return (
			valueImportOf("node:child_process").test(body) ||
			valueImportOf("child_process").test(body) ||
			/require\("(node:)?child_process"\)/.test(body)
		);
	});
	assert.deepEqual(
		offenders,
		[],
		`these unit files start a process: ${offenders.join(", ")}`,
	);
});

test("no unit-test file imports the vscode module", () => {
	// The vscode module exists only inside an extension host. A unit file
	// importing it cannot run under `node --test` at all, so this check is
	// really about the file that imports it transitively through a src module
	// and takes the whole layer down with it.
	const files = layerFiles("unit/");
	assert.ok(
		files.length > 0,
		"no unit-test file was scanned at all, so this check proved nothing",
	);
	const offenders = files.filter((rel) => {
		const body = readFileSync(join(testRoot, rel), "utf8");
		return (
			valueImportOf("vscode").test(body) || /require\("vscode"\)/.test(body)
		);
	});
	assert.deepEqual(
		offenders,
		[],
		`these unit files import vscode: ${offenders.join(", ")}`,
	);
});

test("no module the unit layer reaches imports the vscode module at run time", () => {
	// extension.ts is the one module allowed to, and no unit test imports it.
	// A type-only import elsewhere is fine, because TypeScript erases it; a
	// value import is what would break the layer.
	const srcRoot = join(extensionRoot, "src");
	const offenders: string[] = [];
	let scanned = 0;
	for (const file of walk(srcRoot)) {
		const rel = relative(srcRoot, file).split("\\").join("/");
		if (rel === "extension.ts") {
			continue;
		}
		scanned += 1;
		const body = readFileSync(file, "utf8");
		if (/^import \* as vscode from "vscode";$/m.test(body)) {
			offenders.push(rel);
		}
	}
	// The sibling test below reads extension.ts by name, which says nothing
	// about whether this walk found any other module, so the count is what
	// keeps a moved or renamed src directory from reading as a clean one.
	assert.ok(
		scanned > 0,
		"no module besides extension.ts was scanned at all, so this check proved nothing",
	);
	assert.deepEqual(
		offenders,
		[],
		`these modules import vscode as a value and only extension.ts may: ${offenders.join(", ")}`,
	);
});

test("extension.ts does import it, so the check above is not vacuous", () => {
	const body = readFileSync(join(extensionRoot, "src", "extension.ts"), "utf8");
	assert.ok(body.includes('import * as vscode from "vscode";'));
});

test("every integration file sits under the directory only test:integration runs", () => {
	// The runner names its files by the DINAH_SUITE variable and loads them
	// from out/test/integration/suite/, so a file added anywhere else is
	// either unreachable or is reached by the wrong layer.
	const files = layerFiles("integration/").filter((rel) => rel.endsWith(".test.ts"));
	assert.ok(files.length > 0, "there are no integration test files at all");
	for (const rel of files) {
		assert.ok(
			rel.startsWith("integration/suite/"),
			`${rel} is an integration test outside the directory the runner loads from`,
		);
	}
});

test("the two npm scripts reach the two layers and neither reaches both", () => {
	const manifest = JSON.parse(
		readFileSync(join(extensionRoot, "package.json"), "utf8"),
	) as { scripts: Record<string, string> };
	assert.ok(manifest.scripts["test:unit"].includes("run-unit-tests.mjs"));
	assert.ok(!manifest.scripts["test:unit"].includes("integration"));
	assert.ok(manifest.scripts["test:integration"].includes("integration/runTest"));
});
