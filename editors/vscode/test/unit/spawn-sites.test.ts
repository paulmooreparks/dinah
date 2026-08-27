// One spawn site, enforced rather than reviewed.
//
// dinah-185 says the extension speaks the machine surfaces and never parses
// human output. That holds only while every invocation goes through cli.ts,
// which composes --json itself. A second child_process import anywhere under
// src/ is a route around that, and this is what notices.

import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { test } from "node:test";

const extensionRoot = join(__dirname, "..", "..", "..");
const srcRoot = join(extensionRoot, "src");

/** The one module allowed to start a process, relative to src/. */
const ALLOWED = "spawn.ts";

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

test("only spawn.ts starts a process", () => {
	const offenders: string[] = [];
	for (const file of walk(srcRoot)) {
		const rel = relative(srcRoot, file).split("\\").join("/");
		if (rel === ALLOWED) {
			continue;
		}
		const body = readFileSync(file, "utf8");
		if (/from "node:child_process"|require\("node:child_process"\)|from "child_process"/.test(body)) {
			offenders.push(rel);
		}
	}
	assert.deepEqual(
		offenders,
		[],
		`these modules import child_process and only ${ALLOWED} may: ${offenders.join(", ")}`,
	);
});

test("spawn.ts is the module that does import it, so the check is not vacuous", () => {
	// A check that passes because nothing anywhere spawns would go on passing
	// after the real call site moved, which is the failure mode of every
	// absence assertion written without this companion.
	const body = readFileSync(join(srcRoot, ALLOWED), "utf8");
	assert.ok(body.includes('from "node:child_process"'));
});
