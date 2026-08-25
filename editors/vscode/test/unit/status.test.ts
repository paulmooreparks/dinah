import assert from "node:assert/strict";
import { test } from "node:test";

import type { BinaryState, WorkbenchResolution } from "../../src/api";
import { composeContextKeys, composeStatus } from "../../src/status";
import { AMBIGUOUS_WORKBENCH, NO_WORKBENCH_FOUND } from "../../src/workbench";

const GOOD_BINARY: BinaryState = {
	state: "ok",
	path: "/usr/local/bin/dinah",
	source: "path",
	version: { tool: "v0.1.0-dev.42", profile: "dinah-core/0.4", format: 1 },
};

const INSIDE: WorkbenchResolution = {
	state: "ok",
	root: "/w/.dinah/abc",
	title: "Dinah",
	source: "search",
	profile: "dinah-core/0.4",
	insideWorkspace: true,
};

const OUTSIDE: WorkbenchResolution = { ...INSIDE, insideWorkspace: false };

test("a resolved workbench inside the workspace shows its title and leads with the root", () => {
	const view = composeStatus(GOOD_BINARY, INSIDE, "source");
	assert.equal(view.hidden, false);
	assert.equal(view.text, "$(checklist) Dinah");
	assert.equal(view.tooltip.split("\n")[0], "/w/.dinah/abc");
	assert.ok(view.tooltip.includes("resolved by search"));
	assert.ok(view.tooltip.includes("dinah v0.1.0-dev.42, dinah-core/0.4, format 1"));
	assert.ok(view.tooltip.includes("binary: /usr/local/bin/dinah (path)"));
});

test("a workbench outside the workspace warns and says so first", () => {
	// This is the dinah-241 visibility rule. An extension that showed only the
	// title would pass every other assertion here.
	const view = composeStatus(GOOD_BINARY, OUTSIDE, "source");
	assert.equal(view.text, "$(checklist) Dinah $(warning)");
	assert.equal(
		view.tooltip.split("\n")[0],
		"This workbench is outside your workspace: /w/.dinah/abc",
	);
});

test("an ambiguous refusal warns, names no winner, and lists the candidates", () => {
	const view = composeStatus(
		GOOD_BINARY,
		{
			state: "refused",
			refusal: AMBIGUOUS_WORKBENCH,
			candidates: [
				{ title: "one", path: "/base/.dinah/aaa" },
				{ title: "two", path: "/base/.dinah/bbb" },
			],
		},
		"source",
	);
	assert.equal(view.text, "$(checklist) Dinah $(warning)");
	assert.ok(view.tooltip.includes("Set dinah.workbench to choose one."));
	assert.ok(view.tooltip.includes("/base/.dinah/aaa"));
	assert.ok(view.tooltip.includes("/base/.dinah/bbb"));
});

test("no workbench found hides the item", () => {
	const view = composeStatus(
		GOOD_BINARY,
		{ state: "refused", refusal: NO_WORKBENCH_FOUND },
		"source",
	);
	assert.equal(view.hidden, true);
});

test("no usable binary shows an error and names the two remedies", () => {
	const view = composeStatus({ state: "no-binary" }, undefined, "source");
	assert.equal(view.text, "$(checklist) Dinah $(error)");
	assert.ok(view.tooltip.includes("dinah.path"));
	assert.ok(view.tooltip.includes("github.com/paulmooreparks/dinah"));
});

test("a skewed binary shows an error carrying the gate's own diagnostic", () => {
	const view = composeStatus(
		{
			state: "format-skew",
			path: "dinah",
			detail: "this binary writes storage format 99, and this extension supports 1",
			version: { tool: "x", profile: "dinah-core/0.4", format: 99 },
		},
		INSIDE,
		"source",
	);
	assert.equal(view.text, "$(checklist) Dinah $(error)");
	assert.ok(view.tooltip.includes("storage format 99"));
});

test("the announced demotion reaches the tooltip", () => {
	const view = composeStatus(
		{ ...GOOD_BINARY, source: "carried", demotedFrom: "used the carried one because" },
		INSIDE,
		"source",
	);
	assert.ok(view.tooltip.includes("used the carried one because"));
});

test("the paired release is displayed", () => {
	const view = composeStatus(GOOD_BINARY, INSIDE, "v0.1.0-dev.42");
	assert.ok(view.tooltip.includes("extension paired with dinah v0.1.0-dev.42"));
});

test("the context keys drive every welcome case", () => {
	assert.deepEqual(composeContextKeys({ state: "no-binary" }, undefined), {
		binary: "missing",
		workbench: "unknown",
	});
	// A window with a usable binary and no folder to resolve from. The
	// manifest gives this state a block of its own, so it is composed here
	// rather than left to fall through to the pre-activation text.
	assert.deepEqual(composeContextKeys(GOOD_BINARY, undefined), {
		binary: "ok",
		workbench: "unknown",
	});
	assert.deepEqual(
		composeContextKeys(GOOD_BINARY, { state: "refused", refusal: NO_WORKBENCH_FOUND }),
		{ binary: "ok", workbench: "none" },
	);
	assert.deepEqual(
		composeContextKeys(GOOD_BINARY, { state: "refused", refusal: AMBIGUOUS_WORKBENCH }),
		{ binary: "ok", workbench: "ambiguous" },
	);
	assert.deepEqual(composeContextKeys(GOOD_BINARY, INSIDE), {
		binary: "ok",
		workbench: "ok",
	});
});
