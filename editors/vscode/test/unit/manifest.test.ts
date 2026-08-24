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

// This file is compiled to out/test/unit/, so the extension root is three up.
const extensionRoot = join(__dirname, "..", "..", "..");
const manifest = JSON.parse(
	readFileSync(join(extensionRoot, "package.json"), "utf8"),
) as Record<string, unknown>;

const contributes = manifest.contributes as Record<string, unknown>;

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

test("a welcome block with no when clause covers the interval before activate() returns", () => {
	// All three case-specific blocks are gated on a context key the extension
	// sets after it has activated. Between onView firing and activate()
	// returning, no case matches, and without this block the view is blank in
	// exactly the window the welcome text exists for.
	const welcome = contributes.viewsWelcome as { view: string; when?: string }[];
	const unconditional = welcome.filter(
		(block) => block.view === VIEW_ID && block.when === undefined,
	);
	assert.equal(unconditional.length, 1);
});

test("every conditional welcome block is gated on a key the extension actually sets", () => {
	const welcome = contributes.viewsWelcome as { view: string; when?: string }[];
	const clauses = welcome
		.map((block) => block.when)
		.filter((when): when is string => when !== undefined);
	assert.deepEqual(clauses, [
		"dinah.binary == 'missing'",
		"dinah.workbench == 'none'",
		"dinah.workbench == 'ambiguous'",
	]);
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

test("the version's major and minor come from the repository's VERSION file", () => {
	const base = readFileSync(join(extensionRoot, "..", "..", "VERSION"), "utf8").trim();
	assert.ok(
		String(manifest.version).startsWith(`${base}.`),
		`the extension version ${String(manifest.version)} does not derive from VERSION ${base}`,
	);
});
