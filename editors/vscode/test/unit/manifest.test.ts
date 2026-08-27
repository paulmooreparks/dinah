// The manifest is a contract too, and nothing else reads it back.
//
// A manifest cannot import a module, so the extension identifier is spelled in
// package.json and in identity.ts and these tests are what keep the two from
// drifting. The identifier is permanent from the first publish, and the
// integration tests reach the extension through it, so a drift would be found
// by a test failing for a reason that looks unrelated.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import {
	EXTENSION_ID,
	EXTENSION_NAME,
	PUBLISHER,
	SETTING_PATH,
	SETTING_WORKBENCH,
	VIEW_CONTAINER_ID,
	VIEW_ID,
} from "../../src/identity";
import { BINARY_KEY_VALUES, WORKBENCH_KEY_VALUES } from "../../src/status";

// This file is compiled to out/test/unit/, so the extension root is three up.
const extensionRoot = join(__dirname, "..", "..", "..");
const manifest = JSON.parse(
	readFileSync(join(extensionRoot, "package.json"), "utf8"),
) as Record<string, unknown>;

const contributes = manifest.contributes as Record<string, unknown>;

interface WelcomeBlock {
	readonly view: string;
	readonly when?: string;
	readonly contents: string;
}

/** The context key values a window can be in, with undefined meaning unset. */
interface KeyState {
	readonly binary?: string;
	readonly workbench?: string;
}

function welcomeBlocks(): WelcomeBlock[] {
	return (contributes.viewsWelcome as WelcomeBlock[]).filter(
		(block) => block.view === VIEW_ID,
	);
}

// The `when` grammar these blocks are held to. Restricting it is what makes
// the exclusivity proof below a proof rather than a sampling: every clause is
// an equality or an inequality against a literal, joined by `and`, so a state
// decides every clause with no evaluation order and no truthiness of an unset
// key involved.
const TERM = /^(\S+) (==|!=) '([^']*)'$/;

function evaluate(when: string | undefined, state: KeyState): boolean {
	if (when === undefined) {
		return true;
	}
	return when.split("&&").every((raw) => {
		const term = TERM.exec(raw.trim());
		assert.ok(term, `welcome clause outside the supported grammar: ${raw.trim()}`);
		const [, key, operator, literal] = term;
		const held =
			key === "dinah.binary"
				? state.binary
				: key === "dinah.workbench"
					? state.workbench
					: assert.fail(`welcome clause reads an unknown context key: ${key}`);
		return operator === "==" ? held === literal : held !== literal;
	});
}

/**
 * Every state a window can be in, unset keys first.
 *
 * The half-set states are the point of this function rather than an edge of
 * it. activate() writes the two keys in two separately awaited commands, so a
 * window holds one key and not the other across an extension-host round trip
 * on every healthy startup, and that interval is where a state with no block
 * of its own hides. The product alone cannot express it, which is how the
 * first repair passed a proof it did not satisfy.
 */
function everyState(): KeyState[] {
	const states: KeyState[] = [{}];
	for (const binary of BINARY_KEY_VALUES) {
		states.push({ binary });
	}
	for (const workbench of WORKBENCH_KEY_VALUES) {
		states.push({ workbench });
	}
	for (const binary of BINARY_KEY_VALUES) {
		for (const workbench of WORKBENCH_KEY_VALUES) {
			states.push({ binary, workbench });
		}
	}
	return states;
}

function describeState(state: KeyState): string {
	return `binary=${state.binary ?? "(unset)"} workbench=${state.workbench ?? "(unset)"}`;
}

test("the manifest and identity.ts spell the same extension identifier", () => {
	assert.equal(manifest.publisher, PUBLISHER);
	assert.equal(manifest.name, EXTENSION_NAME);
	assert.equal(`${String(manifest.publisher)}.${String(manifest.name)}`, EXTENSION_ID);
});

test("activation is the two declared events and nothing else", () => {
	// onStartupFinished and * are both forbidden: a window with no Dinah
	// content must cost nothing, and this is the only mechanical guard against
	// either being added later for convenience.
	assert.deepEqual(manifest.activationEvents, [
		"workspaceContains:**/workbench.md",
		`onView:${VIEW_ID}`,
	]);
});

test("the view container and its single view carry the declared ids", () => {
	const containers = contributes.viewsContainers as {
		activitybar: { id: string }[];
	};
	assert.equal(containers.activitybar.length, 1);
	assert.equal(containers.activitybar[0].id, VIEW_CONTAINER_ID);

	const views = contributes.views as Record<string, { id: string }[]>;
	assert.deepEqual(Object.keys(views), [VIEW_CONTAINER_ID]);
	assert.equal(views[VIEW_CONTAINER_ID].length, 1);
	assert.equal(views[VIEW_CONTAINER_ID][0].id, VIEW_ID);
});

test("exactly one welcome block matches in every state a window can be in", () => {
	// The defect this pins is a state with no block of its own. A window that
	// resolved a binary and a workbench matched none of the case-specific
	// blocks, fell through to an unconditional block, and told a reader whose
	// extension was working perfectly that it was still looking. Nothing
	// noticed, because three of the four states were specified and the fourth
	// was the one that shipped.
	//
	// Exactly one, rather than at least one, is the load-bearing half. The
	// contribution-points documentation does not say whether VS Code renders
	// the first matching block or concatenates every match, so a repair that
	// overlapped two blocks would be correct only by undocumented behaviour.
	// Blocks that cannot overlap never ask the question.
	const blocks = welcomeBlocks();
	for (const state of everyState()) {
		const matched = blocks.filter((block) => evaluate(block.when, state));
		assert.equal(
			matched.length,
			1,
			`${matched.length} welcome blocks match ${describeState(state)}, wanted 1`,
		);
	}
});

test("each resolved state renders its own text, and none renders the still-looking text", () => {
	// The four states the spec names, plus the interval before activate() has
	// set either key. Each of the five reaches different contents, so no
	// resolved window can be shown the text written for a window that has not
	// answered yet. The half-set state where a binary resolved and the
	// workbench key is not written yet is named here too: it is still looking,
	// but it is looking for less than a window that has answered nothing, and
	// telling a reader otherwise is the same lie one degree smaller.
	const blocks = welcomeBlocks();
	const contentsFor = (state: KeyState): string => {
		const matched = blocks.filter((block) => evaluate(block.when, state));
		assert.equal(matched.length, 1, `no single block for ${describeState(state)}`);
		return matched[0].contents;
	};

	const stillLooking = contentsFor({});
	const named: [string, KeyState][] = [
		["no usable binary", { binary: "missing", workbench: "unknown" }],
		["no workbench found", { binary: "ok", workbench: "none" }],
		["several workbenches", { binary: "ok", workbench: "ambiguous" }],
		["workbench-resolved", { binary: "ok", workbench: "ok" }],
		["binary resolved, workbench not written yet", { binary: "ok" }],
	];

	const seen = new Map<string, string>();
	for (const [label, state] of named) {
		const contents = contentsFor(state);
		assert.notEqual(
			contents,
			stillLooking,
			`the ${label} state still renders the pre-activation text`,
		);
		const earlier = seen.get(contents);
		assert.equal(
			earlier,
			undefined,
			`the ${label} state renders the same text as the ${String(earlier)} state`,
		);
		seen.set(contents, label);
	}
});

test("every welcome clause reads only the two keys the extension sets", () => {
	// evaluate() fails on a clause outside the grammar or on an unknown key,
	// so running one state through every block is what checks them.
	const blocks = welcomeBlocks();
	for (const block of blocks) {
		evaluate(block.when, { binary: "ok", workbench: "ok" });
	}
	assert.ok(blocks.length > 0);
});

test("the two settings are contributed with the scopes their subjects need", () => {
	const configuration = contributes.configuration as {
		properties: Record<string, { scope?: string; default?: unknown }>;
	};
	assert.deepEqual(Object.keys(configuration.properties), [
		SETTING_PATH,
		SETTING_WORKBENCH,
	]);
	// A binary path is a property of the machine and must not travel through
	// settings sync to a different one.
	assert.equal(configuration.properties[SETTING_PATH].scope, "machine-overridable");
	// Which workbench a folder uses is a property of the folder.
	assert.equal(configuration.properties[SETTING_WORKBENCH].scope, "resource");
});

test("the extension version is major.minor.patch, which is all the marketplace accepts", () => {
	assert.match(String(manifest.version), /^\d+\.\d+\.\d+$/);
});

test("the unit layer runs through the guard that refuses an empty run", () => {
	// The script this replaced handed a quoted glob to `node --test`, which
	// the pinned CI version could not expand, so the whole layer matched no
	// files on a runner. The guard enumerates the compiled files itself and
	// reads the run's own count back. A revert to any form that describes the
	// files by pattern brings the silent-empty-run failure back with it, and
	// this is the only thing that would notice.
	const scripts = manifest.scripts as Record<string, string>;
	assert.ok(
		scripts["test:unit"].includes("scripts/run-unit-tests.mjs"),
		`test:unit no longer runs through the guard: ${scripts["test:unit"]}`,
	);
	assert.ok(
		!scripts["test:unit"].includes("*"),
		`test:unit describes its files with a pattern again: ${scripts["test:unit"]}`,
	);
});

test("the version's major and minor come from the repository's VERSION file", () => {
	const base = readFileSync(join(extensionRoot, "..", "..", "VERSION"), "utf8").trim();
	assert.ok(
		String(manifest.version).startsWith(`${base}.`),
		`the extension version ${String(manifest.version)} does not derive from VERSION ${base}`,
	);
});
